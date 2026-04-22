package neo4j

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/parser"

	"go.uber.org/zap"
)

// WriteKshPass1Result writes Korn shell structural analysis results to Neo4j.
func (w *BatchWriter) WriteKshPass1Result(ctx context.Context, result *parser.KshPass1Result) error {
	cb := w.codebase

	// ShellScript node
	scriptRows := []map[string]any{{
		"id":       result.Script.ID,
		"filePath": ScopedKey(cb, result.Script.FilePath),
		"shebang":  result.Script.Shebang,
		"purpose":  result.Script.Purpose,
		"codebase": cb,
	}}
	if err := w.WriteNodes(ctx, "ShellScript", "filePath", scriptRows); err != nil {
		return fmt.Errorf("writing ShellScript: %w", err)
	}

	// ShellFunction nodes
	if len(result.Functions) > 0 {
		rows := make([]map[string]any, len(result.Functions))
		for i, f := range result.Functions {
			rows[i] = map[string]any{
				"id":          f.ID,
				"name":        f.Name,
				"description": f.Description,
				"scriptPath":  result.SourceFile,
				"mergeId":     ScopedKey(cb, f.MergeID),
				"codebase":    cb,
			}
		}
		if err := w.WriteNodes(ctx, "ShellFunction", "mergeId", rows); err != nil {
			return fmt.Errorf("writing ShellFunction: %w", err)
		}
	}

	// Relationships
	if len(result.Relationships) > 0 {
		grouped := groupRelationships(result.Relationships)
		for key, rels := range grouped {
			rows := make([]map[string]any, len(rels))
			for i, r := range rels {
				props := r.Properties
				if props == nil {
					props = map[string]any{}
				}
				fromKey := r.FromKey
				if !isSharedLabel(key.fromLabel) {
					fromKey = ScopedKey(cb, fromKey)
				}
				toKey := r.ToKey
				if !isSharedLabel(key.toLabel) {
					toKey = ScopedKey(cb, toKey)
				}
				rows[i] = map[string]any{"fromKey": fromKey, "toKey": toKey, "props": props}
			}
			if err := w.WriteRelationships(ctx, string(key.relType), key.fromLabel, mergeKeyForLabel(key.fromLabel), key.toLabel, mergeKeyForLabel(key.toLabel), rows); err != nil {
				w.logger.Error("failed to write ksh relationships", zap.String("type", string(key.relType)), zap.Error(err))
			}
		}
	}

	return nil
}
