package neo4j

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/parser"

	"go.uber.org/zap"
)

// WritePLSQLPass1Result writes PL/SQL structural analysis results to Neo4j.
func (w *BatchWriter) WritePLSQLPass1Result(ctx context.Context, result *parser.PLSQLPass1Result) error {
	cb := w.codebase

	// PLSQLPackage node (shared — Oracle packages exist once across codebases)
	if result.Package.Name != "" {
		pkgRows := []map[string]any{{
			"id":         result.Package.ID,
			"name":       result.Package.Name,
			"objectType": result.Package.ObjectType,
			"schemaName": result.Package.SchemaName,
			"filePath":   result.Package.FilePath,
			"codebase":   cb,
		}}
		if err := w.WriteSharedNodes(ctx, "PLSQLPackage", "name", pkgRows); err != nil {
			return fmt.Errorf("writing PLSQLPackage: %w", err)
		}
	}

	// PLSQLProcedure nodes (scoped)
	if len(result.Procedures) > 0 {
		rows := make([]map[string]any, len(result.Procedures))
		for i, p := range result.Procedures {
			rows[i] = map[string]any{
				"id":          p.ID,
				"name":        p.Name,
				"packageName": p.PackageName,
				"filePath":    p.FilePath,
				"mergeId":     ScopedKey(cb, p.MergeID),
				"codebase":    cb,
			}
		}
		if err := w.WriteNodes(ctx, "PLSQLProcedure", "mergeId", rows); err != nil {
			return fmt.Errorf("writing PLSQLProcedure: %w", err)
		}
	}

	// PLSQLFunction nodes (scoped)
	if len(result.Functions) > 0 {
		rows := make([]map[string]any, len(result.Functions))
		for i, f := range result.Functions {
			rows[i] = map[string]any{
				"id":          f.ID,
				"name":        f.Name,
				"returnType":  f.ReturnType,
				"packageName": f.PackageName,
				"filePath":    f.FilePath,
				"mergeId":     ScopedKey(cb, f.MergeID),
				"codebase":    cb,
			}
		}
		if err := w.WriteNodes(ctx, "PLSQLFunction", "mergeId", rows); err != nil {
			return fmt.Errorf("writing PLSQLFunction: %w", err)
		}
	}

	// PLSQLTrigger nodes (scoped)
	if len(result.Triggers) > 0 {
		rows := make([]map[string]any, len(result.Triggers))
		for i, t := range result.Triggers {
			rows[i] = map[string]any{
				"id":        t.ID,
				"name":      t.Name,
				"tableName": t.TableName,
				"event":     t.Event,
				"timing":    t.Timing,
				"filePath":  t.FilePath,
				"mergeId":   ScopedKey(cb, t.MergeID),
				"codebase":  cb,
			}
		}
		if err := w.WriteNodes(ctx, "PLSQLTrigger", "mergeId", rows); err != nil {
			return fmt.Errorf("writing PLSQLTrigger: %w", err)
		}
	}

	// PLSQLCursor nodes (scoped)
	if len(result.Cursors) > 0 {
		rows := make([]map[string]any, len(result.Cursors))
		for i, c := range result.Cursors {
			rows[i] = map[string]any{
				"id":          c.ID,
				"name":        c.Name,
				"query":       c.Query,
				"packageName": c.PackageName,
				"mergeId":     ScopedKey(cb, c.MergeID),
				"codebase":    cb,
			}
		}
		if err := w.WriteNodes(ctx, "PLSQLCursor", "mergeId", rows); err != nil {
			return fmt.Errorf("writing PLSQLCursor: %w", err)
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
				w.logger.Error("failed to write PL/SQL relationships", zap.String("type", string(key.relType)), zap.Error(err))
			}
		}
	}

	return nil
}
