package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cobol-ingestor/internal/graph"
)

// WriteGlossaryTerms merges glossary term nodes into Neo4j.
// Deduplication key is (codebase, termLower).
func (w *BatchWriter) WriteGlossaryTerms(ctx context.Context, codebase, sourceFile string, terms []graph.GlossaryTerm) error {
	if len(terms) == 0 {
		return nil
	}

	nodes := make([]map[string]any, 0, len(terms))
	for _, t := range terms {
		aliasesJSON := "[]"
		if len(t.Aliases) > 0 {
			b, _ := json.Marshal(t.Aliases)
			aliasesJSON = string(b)
		}
		cb := codebase
		if cb == "" {
			cb = "default"
		}
		nodes = append(nodes, map[string]any{
			"termLower":  strings.ToLower(t.Term),
			"term":       t.Term,
			"kind":       t.Kind,
			"definition": t.Definition,
			"aliases":    aliasesJSON,
			"codebase":   cb,
			"sourceFile": sourceFile,
			"updatedAt":  time.Now().UTC().Format(time.RFC3339),
		})
	}

	cypher := `UNWIND $rows AS row
MERGE (g:GlossaryTerm {codebase: row.codebase, termLower: row.termLower})
SET g.term       = row.term,
    g.kind       = row.kind,
    g.definition = row.definition,
    g.aliases    = row.aliases,
    g.sourceFile = row.sourceFile,
    g.updatedAt  = row.updatedAt`

	if err := w.batchUpdate(ctx, cypher, nodes); err != nil {
		return fmt.Errorf("writing GlossaryTerm nodes: %w", err)
	}
	return nil
}
