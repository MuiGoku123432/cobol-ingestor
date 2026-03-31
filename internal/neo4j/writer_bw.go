package neo4j

import (
	"context"
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// WriteBWResult writes Businessware extraction results to Neo4j.
func (w *BatchWriter) WriteBWResult(ctx context.Context, result *graph.BWResult) error {
	// 1. MERGE BWFile node
	fileNode := []map[string]any{{
		"path":     result.File.Path,
		"fileType": result.File.FileType,
		"summary":  result.File.Summary,
	}}
	if err := w.WriteNodes(ctx, "BWFile", "path", fileNode); err != nil {
		return fmt.Errorf("writing BWFile node: %w", err)
	}

	// 2. MERGE BWEntity nodes
	if len(result.Entities) > 0 {
		nodes := make([]map[string]any, len(result.Entities))
		for i, e := range result.Entities {
			node := map[string]any{
				"mergeId":     e.MergeID,
				"name":        e.Name,
				"entityType":  e.EntityType,
				"description": e.Description,
				"sourceFile":  e.SourceFile,
			}
			if len(e.Properties) > 0 {
				propsJSON, _ := json.Marshal(e.Properties)
				node["properties"] = string(propsJSON)
			}
			nodes[i] = node
		}
		if err := w.WriteNodes(ctx, "BWEntity", "mergeId", nodes); err != nil {
			return fmt.Errorf("writing BWEntity nodes: %w", err)
		}
	}

	// 3. MERGE BW_CONTAINS relationships (BWFile → BWEntity)
	if len(result.Entities) > 0 {
		rows := make([]map[string]any, len(result.Entities))
		for i, e := range result.Entities {
			rows[i] = map[string]any{
				"fromKey": result.File.Path,
				"toKey":   e.MergeID,
				"props":   map[string]any{},
			}
		}
		if err := w.WriteRelationships(ctx, "BW_CONTAINS", "BWFile", "path", "BWEntity", "mergeId", rows); err != nil {
			return fmt.Errorf("writing BW_CONTAINS relationships: %w", err)
		}
	}

	// 4. MERGE BW_RELATES_TO relationships (BWEntity → BWEntity)
	if len(result.Relationships) > 0 {
		var relRows []map[string]any
		for _, r := range result.Relationships {
			fromMergeID := result.File.Path + "." + r.FromEntity
			toMergeID := result.File.Path + "." + r.ToEntity
			relRows = append(relRows, map[string]any{
				"fromKey": fromMergeID,
				"toKey":   toMergeID,
				"props": map[string]any{
					"relationType": r.RelationType,
					"description":  r.Description,
					"confidence":   r.Confidence,
				},
			})
		}
		if err := w.WriteRelationships(ctx, "BW_RELATES_TO", "BWEntity", "mergeId", "BWEntity", "mergeId", relRows); err != nil {
			w.logger.Warn("failed to write BW_RELATES_TO relationships", zap.Error(err))
		}
	}

	// 5. MATCH+MERGE BW_REFERENCES (BWEntity → Program/Copybook) — batched by target type
	if len(result.CobolReferences) > 0 {
		var programRows, copybookRows []map[string]any
		for _, ref := range result.CobolReferences {
			entityMergeID := result.File.Path + "." + ref.EntityName
			row := map[string]any{
				"mergeId": entityMergeID,
				"target":  ref.TargetName,
				"refType": ref.ReferenceType,
				"desc":    ref.Description,
			}
			if ref.TargetType == "Copybook" {
				copybookRows = append(copybookRows, row)
			} else {
				programRows = append(programRows, row)
			}
		}

		if len(programRows) > 0 {
			cypher := "UNWIND $rows AS row " +
				"MATCH (e:BWEntity {mergeId: row.mergeId}) " +
				"MATCH (t:Program {programId: row.target}) " +
				"MERGE (e)-[r:BW_REFERENCES]->(t) " +
				"SET r.referenceType = row.refType, r.description = row.desc"
			if err := w.batchUpdate(ctx, cypher, programRows); err != nil {
				w.logger.Debug("BW_REFERENCES Program batch had missing targets",
					zap.Int("count", len(programRows)),
					zap.Error(err),
				)
			}
		}

		if len(copybookRows) > 0 {
			cypher := "UNWIND $rows AS row " +
				"MATCH (e:BWEntity {mergeId: row.mergeId}) " +
				"MATCH (t:Copybook {name: row.target}) " +
				"MERGE (e)-[r:BW_REFERENCES]->(t) " +
				"SET r.referenceType = row.refType, r.description = row.desc"
			if err := w.batchUpdate(ctx, cypher, copybookRows); err != nil {
				w.logger.Debug("BW_REFERENCES Copybook batch had missing targets",
					zap.Int("count", len(copybookRows)),
					zap.Error(err),
				)
			}
		}
	}

	w.logger.Info("wrote BW results",
		zap.String("file", result.File.Path),
		zap.Int("entities", len(result.Entities)),
		zap.Int("relationships", len(result.Relationships)),
		zap.Int("cobolRefs", len(result.CobolReferences)),
	)

	return nil
}
