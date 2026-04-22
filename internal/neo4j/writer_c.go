package neo4j

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/parser"

	"go.uber.org/zap"
)

// WriteCPass1Result writes C structural analysis results to Neo4j.
func (w *BatchWriter) WriteCPass1Result(ctx context.Context, result *parser.CPass1Result) error {
	cb := w.codebase

	// CProgram node
	programRows := []map[string]any{{
		"id":       result.Program.ID,
		"filePath": ScopedKey(cb, result.Program.FilePath),
		"language": result.Program.Language,
		"codebase": cb,
	}}
	if err := w.WriteNodes(ctx, "CProgram", "filePath", programRows); err != nil {
		return fmt.Errorf("writing CProgram: %w", err)
	}

	// CFunction nodes
	if len(result.Functions) > 0 {
		rows := make([]map[string]any, len(result.Functions))
		for i, f := range result.Functions {
			rows[i] = map[string]any{
				"id":       f.ID,
				"name":     f.Name,
				"retType":  f.RetType,
				"isStatic": f.IsStatic,
				"filePath": result.SourceFile,
				"mergeId":  ScopedKey(cb, f.MergeID),
				"codebase": cb,
			}
		}
		if err := w.WriteNodes(ctx, "CFunction", "mergeId", rows); err != nil {
			return fmt.Errorf("writing CFunction: %w", err)
		}
	}

	// CStruct nodes
	if len(result.Structs) > 0 {
		rows := make([]map[string]any, len(result.Structs))
		for i, s := range result.Structs {
			rows[i] = map[string]any{
				"id":       s.ID,
				"name":     s.Name,
				"filePath": result.SourceFile,
				"mergeId":  ScopedKey(cb, s.MergeID),
				"codebase": cb,
			}
		}
		if err := w.WriteNodes(ctx, "CStruct", "mergeId", rows); err != nil {
			return fmt.Errorf("writing CStruct: %w", err)
		}
	}

	// CTypedef nodes
	if len(result.Typedefs) > 0 {
		rows := make([]map[string]any, len(result.Typedefs))
		for i, t := range result.Typedefs {
			rows[i] = map[string]any{
				"id":         t.ID,
				"alias":      t.Alias,
				"underlying": t.Underlying,
				"filePath":   result.SourceFile,
				"mergeId":    ScopedKey(cb, t.MergeID),
				"codebase":   cb,
			}
		}
		if err := w.WriteNodes(ctx, "CTypedef", "mergeId", rows); err != nil {
			return fmt.Errorf("writing CTypedef: %w", err)
		}
	}

	// CHeader nodes (shared across codebases)
	if len(result.Headers) > 0 {
		rows := make([]map[string]any, len(result.Headers))
		for i, h := range result.Headers {
			rows[i] = map[string]any{
				"id":      h.ID,
				"name":    h.Name,
				"mergeId": h.MergeID,
			}
		}
		if err := w.WriteNodes(ctx, "CHeader", "mergeId", rows); err != nil {
			return fmt.Errorf("writing CHeader: %w", err)
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
				w.logger.Error("failed to write C relationships", zap.String("type", string(key.relType)), zap.Error(err))
			}
		}
	}

	return nil
}
