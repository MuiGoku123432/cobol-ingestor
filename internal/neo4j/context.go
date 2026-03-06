package neo4j

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// ProgramContext holds Pass 1 graph data used as context for Pass 2 analysis.
type ProgramContext struct {
	ProgramID          string
	Callers            []string
	Callees            []string
	Copybooks          []string
	Paragraphs         []string
	Sections           []string
	FileDefs           []string
	DataItems          []string
	Conditions         []string
	Parameters         []string
	ExternalInterfaces []string
}

// QueryProgramContext retrieves Pass 1 graph context for a program.
func (c *Client) QueryProgramContext(ctx context.Context, programID string) (*ProgramContext, error) {
	pc := &ProgramContext{ProgramID: programID}

	session := c.NewSession(ctx)
	defer session.Close(ctx)

	queries := []struct {
		cypher string
		field  string
		dest   *[]string
	}{
		{
			cypher: "MATCH (caller:Program)-[:CALLS]->(p:Program {programId: $id}) RETURN caller.programId AS val",
			field:  "val",
			dest:   &pc.Callers,
		},
		{
			cypher: "MATCH (p:Program {programId: $id})-[:CALLS]->(callee:Program) RETURN callee.programId AS val",
			field:  "val",
			dest:   &pc.Callees,
		},
		{
			cypher: "MATCH (p:Program {programId: $id})-[:INCLUDES]->(cb:Copybook) RETURN cb.name AS val",
			field:  "val",
			dest:   &pc.Copybooks,
		},
		{
			cypher: "MATCH (para:Paragraph)-[:BELONGS_TO]->(p:Program {programId: $id}) RETURN para.name AS val",
			field:  "val",
			dest:   &pc.Paragraphs,
		},
		{
			cypher: "MATCH (sec:Section)-[:BELONGS_TO]->(p:Program {programId: $id}) RETURN sec.name AS val",
			field:  "val",
			dest:   &pc.Sections,
		},
		{
			cypher: "MATCH (f:File)<-[:READS|WRITES]-(p:Program {programId: $id}) RETURN DISTINCT f.name AS val",
			field:  "val",
			dest:   &pc.FileDefs,
		},
		{
			cypher: "MATCH (d:DataItem {programId: $id}) RETURN d.name AS val LIMIT 50",
			field:  "val",
			dest:   &pc.DataItems,
		},
		{
			cypher: "MATCH (c:Condition {programId: $id}) RETURN c.name AS val",
			field:  "val",
			dest:   &pc.Conditions,
		},
		{
			cypher: "MATCH (p:Parameter {programId: $id}) RETURN p.name AS val",
			field:  "val",
			dest:   &pc.Parameters,
		},
		{
			cypher: "MATCH (e:ExternalInterface {programId: $id}) RETURN e.type + ': ' + e.details AS val",
			field:  "val",
			dest:   &pc.ExternalInterfaces,
		},
	}

	params := map[string]any{"id": programID}

	for _, q := range queries {
		result, err := session.Run(ctx, q.cypher, params)
		if err != nil {
			return nil, fmt.Errorf("querying %s: %w", q.cypher[:40], err)
		}
		for result.Next(ctx) {
			val, ok := result.Record().Get(q.field)
			if ok && val != nil {
				if s, ok := val.(string); ok {
					*q.dest = append(*q.dest, s)
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, fmt.Errorf("iterating results: %w", err)
		}
	}

	return pc, nil
}

// FormatContextPreamble renders ProgramContext as a structured text block.
func FormatContextPreamble(pc *ProgramContext) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("=== GRAPH CONTEXT (from Pass 1) ===\n")
	b.WriteString(fmt.Sprintf("Program: %s\n", pc.ProgramID))

	if len(pc.Callers) > 0 {
		b.WriteString(fmt.Sprintf("Called by: %s\n", strings.Join(pc.Callers, ", ")))
	}
	if len(pc.Callees) > 0 {
		b.WriteString(fmt.Sprintf("Calls: %s\n", strings.Join(pc.Callees, ", ")))
	}
	if len(pc.Copybooks) > 0 {
		b.WriteString(fmt.Sprintf("Copybooks: %s\n", strings.Join(pc.Copybooks, ", ")))
	}
	if len(pc.Paragraphs) > 0 {
		b.WriteString(fmt.Sprintf("Paragraphs: %s\n", strings.Join(pc.Paragraphs, ", ")))
	}
	if len(pc.Sections) > 0 {
		b.WriteString(fmt.Sprintf("Sections: %s\n", strings.Join(pc.Sections, ", ")))
	}
	if len(pc.FileDefs) > 0 {
		b.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(pc.FileDefs, ", ")))
	}
	if len(pc.DataItems) > 0 {
		b.WriteString(fmt.Sprintf("Data Items: %s\n", strings.Join(pc.DataItems, ", ")))
	}
	if len(pc.Conditions) > 0 {
		b.WriteString(fmt.Sprintf("Conditions: %s\n", strings.Join(pc.Conditions, ", ")))
	}
	if len(pc.Parameters) > 0 {
		b.WriteString(fmt.Sprintf("Parameters: %s\n", strings.Join(pc.Parameters, ", ")))
	}
	if len(pc.ExternalInterfaces) > 0 {
		b.WriteString(fmt.Sprintf("External Interfaces: %s\n", strings.Join(pc.ExternalInterfaces, ", ")))
	}

	b.WriteString("=== END GRAPH CONTEXT ===")
	return b.String()
}

// RunReadQuery is a helper used by QueryProgramContext.
func runReadQuery(session neo4j.SessionWithContext, ctx context.Context, cypher string, params map[string]any, field string) ([]string, error) {
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var vals []string
	for result.Next(ctx) {
		val, ok := result.Record().Get(field)
		if ok && val != nil {
			if s, ok := val.(string); ok {
				vals = append(vals, s)
			}
		}
	}
	return vals, result.Err()
}
