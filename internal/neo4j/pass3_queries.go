package neo4j

import (
	"context"
	"fmt"
	"strings"
)

// ProgramSlice holds a program's summary for Pass 3 classification.
type ProgramSlice struct {
	ProgramID          string
	Callers            []string
	Callees            []string
	Copybooks          []string
	Paragraphs         []string
	FileDefs           []string
	SQLTables          []string // DB2 tables accessed
	CICSCommands       []string // CICS commands used
	ExternalInterfaces []string // MQ, CICS_LINK, etc.
	ExecutionMode      string   // BATCH, CICS, etc.
	IsExternal         bool     // true if filePath IS NULL
}

// QueryOrphanPrograms returns programs with no incoming CALLS (excludes externals).
func (c *Client) QueryOrphanPrograms(ctx context.Context) ([]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.filePath IS NOT NULL AND NOT ()-[:CALLS]->(p) RETURN p.programId AS pid", nil)
	if err != nil {
		return nil, fmt.Errorf("querying orphan programs: %w", err)
	}

	var orphans []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("pid"); ok {
			if s, ok := val.(string); ok {
				orphans = append(orphans, s)
			}
		}
	}
	return orphans, nil
}

// QueryHubPrograms returns programs called by >= threshold other programs (excludes externals).
func (c *Client) QueryHubPrograms(ctx context.Context, threshold int) ([]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (caller:Program)-[:CALLS]->(p:Program) "+
			"WHERE p.filePath IS NOT NULL "+
			"WITH p, count(DISTINCT caller) AS callerCount "+
			"WHERE callerCount >= $threshold "+
			"RETURN p.programId AS pid",
		map[string]any{"threshold": int64(threshold)})
	if err != nil {
		return nil, fmt.Errorf("querying hub programs: %w", err)
	}

	var hubs []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("pid"); ok {
			if s, ok := val.(string); ok {
				hubs = append(hubs, s)
			}
		}
	}
	return hubs, nil
}

// QueryProgramClusters returns groups of programs that share copybooks.
func (c *Client) QueryProgramClusters(ctx context.Context) ([][]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p1:Program)-[:INCLUDES]->(cb:Copybook)<-[:INCLUDES]-(p2:Program) "+
			"WHERE p1.programId < p2.programId "+
			"RETURN cb.name AS copybook, collect(DISTINCT p1.programId) + collect(DISTINCT p2.programId) AS programs", nil)
	if err != nil {
		return nil, fmt.Errorf("querying program clusters: %w", err)
	}

	var clusters [][]string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("programs"); ok {
			if arr, ok := val.([]any); ok {
				var cluster []string
				for _, v := range arr {
					if s, ok := v.(string); ok {
						cluster = append(cluster, s)
					}
				}
				clusters = append(clusters, cluster)
			}
		}
	}
	return clusters, nil
}

// QueryCallGraphSlice returns paginated program summaries for domain classification.
// Excludes external programs (filePath IS NULL) from classification.
func (c *Client) QueryCallGraphSlice(ctx context.Context, batchSize, offset int) ([]ProgramSlice, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.filePath IS NOT NULL RETURN p.programId AS pid, p.executionMode AS execMode ORDER BY p.programId SKIP $offset LIMIT $limit",
		map[string]any{"offset": int64(offset), "limit": int64(batchSize)})
	if err != nil {
		return nil, fmt.Errorf("querying call graph slice: %w", err)
	}

	type programInfo struct {
		pid      string
		execMode string
	}
	var programs []programInfo
	for result.Next(ctx) {
		rec := result.Record()
		pid := getStr(rec, "pid")
		execMode := getStr(rec, "execMode")
		if pid != "" {
			programs = append(programs, programInfo{pid: pid, execMode: execMode})
		}
	}

	var slices []ProgramSlice
	for _, p := range programs {
		slice, err := c.buildProgramSlice(ctx, p.pid, p.execMode)
		if err != nil {
			continue
		}
		slices = append(slices, *slice)
	}

	return slices, nil
}

// QueryCallGraphSliceForPrograms returns ProgramSlice data for specific program IDs.
func (c *Client) QueryCallGraphSliceForPrograms(ctx context.Context, programIDs []string) ([]ProgramSlice, error) {
	var slices []ProgramSlice
	for _, pid := range programIDs {
		// Look up execution mode
		session := c.NewSession(ctx)
		res, err := session.Run(ctx,
			"MATCH (p:Program {programId: $id}) RETURN p.executionMode AS execMode",
			map[string]any{"id": pid})
		execMode := ""
		if err == nil && res.Next(ctx) {
			execMode = getStr(res.Record(), "execMode")
		}
		session.Close(ctx)

		slice, err := c.buildProgramSlice(ctx, pid, execMode)
		if err != nil {
			continue
		}
		slices = append(slices, *slice)
	}
	return slices, nil
}

// buildProgramSlice constructs a fully enriched ProgramSlice for a single program.
func (c *Client) buildProgramSlice(ctx context.Context, pid, execMode string) (*ProgramSlice, error) {
	pc, err := c.QueryProgramContext(ctx, pid)
	if err != nil {
		return nil, err
	}

	slice := &ProgramSlice{
		ProgramID:          pid,
		Callers:            pc.Callers,
		Callees:            pc.Callees,
		Copybooks:          pc.Copybooks,
		Paragraphs:         pc.Paragraphs,
		FileDefs:           pc.FileDefs,
		ExternalInterfaces: pc.ExternalInterfaces,
		ExecutionMode:      execMode,
	}

	// Enrich with SQL tables
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.Run(ctx,
		"MATCH (p:Program {programId: $id})-[:ACCESSES]->(t:DBTable) RETURN t.name AS val",
		map[string]any{"id": pid})
	if err == nil {
		for res.Next(ctx) {
			if v, ok := res.Record().Get("val"); ok {
				if s, ok := v.(string); ok {
					slice.SQLTables = append(slice.SQLTables, s)
				}
			}
		}
	}

	// Enrich with CICS commands
	res, err = session.Run(ctx,
		"MATCH (c:CICSTransaction {programId: $id}) RETURN DISTINCT c.command AS val",
		map[string]any{"id": pid})
	if err == nil {
		for res.Next(ctx) {
			if v, ok := res.Record().Get("val"); ok {
				if s, ok := v.(string); ok {
					slice.CICSCommands = append(slice.CICSCommands, s)
				}
			}
		}
	}

	return slice, nil
}

// QueryExistingDomainNames returns all BusinessDomain names currently in the graph.
func (c *Client) QueryExistingDomainNames(ctx context.Context) ([]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (d:BusinessDomain) RETURN d.name AS name ORDER BY d.name", nil)
	if err != nil {
		return nil, fmt.Errorf("querying existing domain names: %w", err)
	}

	var names []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("name"); ok {
			if s, ok := val.(string); ok {
				names = append(names, s)
			}
		}
	}
	return names, nil
}

// FormatGraphSlice renders a batch of ProgramSlice as text for the Claude prompt.
func FormatGraphSlice(slices []ProgramSlice, orphans, hubs []string) string {
	var b strings.Builder
	b.WriteString("=== PROGRAM GRAPH DATA ===\n\n")

	for _, s := range slices {
		header := s.ProgramID
		if s.ExecutionMode != "" {
			header += " (" + s.ExecutionMode + ")"
		}
		if s.IsExternal {
			header += " [EXTERNAL]"
		}
		b.WriteString(fmt.Sprintf("Program: %s\n", header))
		if len(s.Callers) > 0 {
			b.WriteString(fmt.Sprintf("  Called by: %s\n", strings.Join(s.Callers, ", ")))
		}
		if len(s.Callees) > 0 {
			b.WriteString(fmt.Sprintf("  Calls: %s\n", strings.Join(s.Callees, ", ")))
		}
		if len(s.Copybooks) > 0 {
			b.WriteString(fmt.Sprintf("  Copybooks: %s\n", strings.Join(s.Copybooks, ", ")))
		}
		if len(s.Paragraphs) > 0 {
			b.WriteString(fmt.Sprintf("  Paragraphs: %s\n", strings.Join(s.Paragraphs, ", ")))
		}
		if len(s.FileDefs) > 0 {
			b.WriteString(fmt.Sprintf("  Files: %s\n", strings.Join(s.FileDefs, ", ")))
		}
		if len(s.SQLTables) > 0 {
			b.WriteString(fmt.Sprintf("  SQL Tables: %s\n", strings.Join(s.SQLTables, ", ")))
		}
		if len(s.CICSCommands) > 0 {
			b.WriteString(fmt.Sprintf("  CICS Commands: %s\n", strings.Join(s.CICSCommands, ", ")))
		}
		if len(s.ExternalInterfaces) > 0 {
			b.WriteString(fmt.Sprintf("  External Interfaces: %s\n", strings.Join(s.ExternalInterfaces, ", ")))
		}
		b.WriteString("\n")
	}

	if len(orphans) > 0 {
		b.WriteString(fmt.Sprintf("=== ORPHAN PROGRAMS (no callers) ===\n%s\n\n", strings.Join(orphans, ", ")))
	}
	if len(hubs) > 0 {
		b.WriteString(fmt.Sprintf("=== HUB PROGRAMS (heavily called) ===\n%s\n\n", strings.Join(hubs, ", ")))
	}

	b.WriteString("=== END PROGRAM GRAPH DATA ===")
	return b.String()
}
