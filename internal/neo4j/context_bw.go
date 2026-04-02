package neo4j

import (
	"context"
	"fmt"
	"strings"

	"cobol-ingestor/internal/chunker"
)

// BWGraphContext holds tiered COBOL graph context for enriched BW prompts.
type BWGraphContext struct {
	InterfacePrograms []InterfaceProgramInfo // Tier 1: programs with external interfaces
	BusinessDomains   []DomainInfo           // Tier 2: business domains with member programs
	FileIOPrograms    []FileIOProgramInfo    // Tier 3: programs with file I/O
	RemainingPrograms []string               // Tier 4: all other program IDs
}

// InterfaceProgramInfo holds a program's external interface summary.
type InterfaceProgramInfo struct {
	ProgramID      string
	ExecutionMode  string
	InterfaceTypes []string
}

// DomainInfo holds a business domain with its member programs.
type DomainInfo struct {
	Name     string
	Programs []string
}

// FileIOProgramInfo holds a program's file I/O summary.
type FileIOProgramInfo struct {
	ProgramID string
	FileNames []string
}

// QueryBWGraphContext runs tiered queries to build enriched context for BW prompts.
func (c *Client) QueryBWGraphContext(ctx context.Context) (*BWGraphContext, error) {
	result := &BWGraphContext{}

	session := c.NewSession(ctx)
	defer session.Close(ctx)

	// Tier 1: Programs with external interfaces (CICS, MQ, etc.)
	tier1Res, err := session.Run(ctx,
		"MATCH (p:Program) "+
			"WHERE EXISTS { MATCH (e:ExternalInterface {programId: p.programId}) } "+
			"OPTIONAL MATCH (e:ExternalInterface {programId: p.programId}) "+
			"WITH p.programId AS pid, p.executionMode AS mode, collect(DISTINCT e.type) AS types "+
			"RETURN pid, mode, types ORDER BY pid",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying interface programs: %w", err)
	}
	tier1Set := make(map[string]bool)
	for tier1Res.Next(ctx) {
		rec := tier1Res.Record()
		pid := getStr(rec, "pid")
		tier1Set[pid] = true
		var types []string
		if val, ok := rec.Get("types"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						types = append(types, s)
					}
				}
			}
		}
		result.InterfacePrograms = append(result.InterfacePrograms, InterfaceProgramInfo{
			ProgramID:      pid,
			ExecutionMode:  getStr(rec, "mode"),
			InterfaceTypes: types,
		})
	}

	// Tier 2: Business domains with member programs
	tier2Res, err := session.Run(ctx,
		"MATCH (d:BusinessDomain)<-[:BELONGS_TO]-(p:Program) "+
			"WITH d.name AS domain, collect(p.programId) AS programs "+
			"RETURN domain, programs ORDER BY domain",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying business domains: %w", err)
	}
	for tier2Res.Next(ctx) {
		rec := tier2Res.Record()
		result.BusinessDomains = append(result.BusinessDomains, DomainInfo{
			Name:     getStr(rec, "domain"),
			Programs: getStringArray(rec, "programs"),
		})
	}

	// Tier 3: Programs with file I/O
	tier3Res, err := session.Run(ctx,
		"MATCH (p:Program)-[:READS|WRITES]->(f:File) "+
			"WITH p.programId AS pid, collect(DISTINCT f.name) AS files "+
			"RETURN pid, files ORDER BY pid",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying file I/O programs: %w", err)
	}
	tier3Set := make(map[string]bool)
	for tier3Res.Next(ctx) {
		rec := tier3Res.Record()
		pid := getStr(rec, "pid")
		tier3Set[pid] = true
		result.FileIOPrograms = append(result.FileIOPrograms, FileIOProgramInfo{
			ProgramID: pid,
			FileNames: getStringArray(rec, "files"),
		})
	}

	// Tier 4: Remaining program IDs not in tiers 1 or 3
	allRes, err := session.Run(ctx,
		"MATCH (p:Program) RETURN p.programId AS pid ORDER BY pid", nil)
	if err != nil {
		return nil, fmt.Errorf("querying remaining programs: %w", err)
	}
	for allRes.Next(ctx) {
		pid := getStr(allRes.Record(), "pid")
		if !tier1Set[pid] && !tier3Set[pid] {
			result.RemainingPrograms = append(result.RemainingPrograms, pid)
		}
	}

	return result, nil
}

// FormatBWGraphContext renders the tiered context as structured text with token budgeting.
func FormatBWGraphContext(ctx *BWGraphContext, maxTokens int) (interfaceProgs, domains, fileIOProgs, remaining string) {
	// Tier 1: ~3000 tokens budget
	var sb1 strings.Builder
	tokens := 0
	for _, p := range ctx.InterfacePrograms {
		line := fmt.Sprintf("%s (%s) — interfaces: %s\n", p.ProgramID, p.ExecutionMode, strings.Join(p.InterfaceTypes, ", "))
		lineTokens := chunker.EstimateTokens(line)
		if tokens+lineTokens > 3000 {
			fmt.Fprintf(&sb1, "... and %d more interface programs\n", len(ctx.InterfacePrograms)-tokens/20)
			break
		}
		sb1.WriteString(line)
		tokens += lineTokens
	}
	interfaceProgs = sb1.String()

	// Tier 2: ~2000 tokens budget
	var sb2 strings.Builder
	tokens = 0
	for _, d := range ctx.BusinessDomains {
		progs := strings.Join(d.Programs, ", ")
		if len(progs) > 200 {
			progs = progs[:200] + "..."
		}
		line := fmt.Sprintf("**%s**: %s\n", d.Name, progs)
		lineTokens := chunker.EstimateTokens(line)
		if tokens+lineTokens > 2000 {
			break
		}
		sb2.WriteString(line)
		tokens += lineTokens
	}
	domains = sb2.String()

	// Tier 3: ~1500 tokens budget
	var sb3 strings.Builder
	tokens = 0
	for _, p := range ctx.FileIOPrograms {
		line := fmt.Sprintf("%s — files: %s\n", p.ProgramID, strings.Join(p.FileNames, ", "))
		lineTokens := chunker.EstimateTokens(line)
		if tokens+lineTokens > 1500 {
			break
		}
		sb3.WriteString(line)
		tokens += lineTokens
	}
	fileIOProgs = sb3.String()

	// Tier 4: remaining IDs (flat list, ~1500 tokens)
	remaining = capToTokens(ctx.RemainingPrograms, 1500)

	return
}

// capToTokens joins strings with commas until the token budget is reached.
func capToTokens(items []string, maxTokens int) string {
	var sb strings.Builder
	tokens := 0
	for i, item := range items {
		entry := item
		if i > 0 {
			entry = ", " + item
		}
		entryTokens := chunker.EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			remaining := len(items) - i
			fmt.Fprintf(&sb, " ... and %d more programs", remaining)
			break
		}
		sb.WriteString(entry)
		tokens += entryTokens
	}
	return sb.String()
}
