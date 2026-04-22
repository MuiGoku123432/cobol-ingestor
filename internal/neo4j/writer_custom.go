package neo4j

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/parser"

	"go.uber.org/zap"
)

// WriteCustomExtractResult writes generic custom-format extraction results to Neo4j.
func (w *BatchWriter) WriteCustomExtractResult(ctx context.Context, result *parser.CustomExtractResult) error {
	cb := w.codebase

	// CustomEntity nodes
	if len(result.Entities) > 0 {
		rows := make([]map[string]any, len(result.Entities))
		for i, e := range result.Entities {
			row := map[string]any{
				"id":          e.ID,
				"name":        e.Name,
				"entityType":  e.EntityType,
				"description": e.Description,
				"sourceFile":  e.SourceFile,
				"extension":   e.Extension,
				"mergeId":     ScopedKey(cb, e.MergeID),
				"codebase":    cb,
			}
			// Merge in LLM-extracted properties
			for k, v := range e.Properties {
				if _, exists := row[k]; !exists {
					row[k] = v
				}
			}
			rows[i] = row
		}
		if err := w.WriteNodes(ctx, "CustomEntity", "mergeId", rows); err != nil {
			return fmt.Errorf("writing CustomEntity: %w", err)
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
				w.logger.Error("failed to write custom relationships", zap.String("type", string(key.relType)), zap.Error(err))
			}
		}
	}

	return nil
}
