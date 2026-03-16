package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"cobol-ingestor/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// BatchWriter writes graph data to Neo4j in batches.
// All write methods are safe for concurrent use.
type BatchWriter struct {
	mu        sync.Mutex
	client    *Client
	batchSize int
	logger    *zap.Logger
}

// NewBatchWriter creates a new batch writer.
func NewBatchWriter(client *Client, batchSize int, logger *zap.Logger) *BatchWriter {
	return &BatchWriter{
		client:    client,
		batchSize: batchSize,
		logger:    logger,
	}
}

// WriteNodes merges nodes of a given label using UNWIND.
// mergeKey is the property used in the MERGE clause (e.g. "programId" for Program).
func (w *BatchWriter) WriteNodes(ctx context.Context, label, mergeKey string, nodes []map[string]any) error {
	if len(nodes) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	cypher := fmt.Sprintf(
		"UNWIND $rows AS row MERGE (n:%s {%s: row.%s}) SET n += row",
		label, mergeKey, mergeKey,
	)

	for i := 0; i < len(nodes); i += w.batchSize {
		end := i + w.batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		batch := nodes[i:end]

		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, cypher, map[string]any{"rows": batch})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			return fmt.Errorf("writing %s nodes: %w", label, err)
		}
	}

	w.logger.Debug("wrote nodes", zap.String("label", label), zap.Int("count", len(nodes)))
	return nil
}

// WriteRelationships merges relationships using UNWIND + MATCH + MERGE.
func (w *BatchWriter) WriteRelationships(ctx context.Context, relType, fromLabel, fromKey, toLabel, toKey string, rels []map[string]any) error {
	if len(rels) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	cypher := fmt.Sprintf(
		"UNWIND $rows AS row "+
			"MATCH (a:%s {%s: row.fromKey}) "+
			"MATCH (b:%s {%s: row.toKey}) "+
			"MERGE (a)-[r:%s]->(b) SET r += row.props",
		fromLabel, fromKey, toLabel, toKey, relType,
	)

	for i := 0; i < len(rels); i += w.batchSize {
		end := i + w.batchSize
		if end > len(rels) {
			end = len(rels)
		}
		batch := rels[i:end]

		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, cypher, map[string]any{"rows": batch})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			return fmt.Errorf("writing %s relationships: %w", relType, err)
		}
	}

	w.logger.Debug("wrote relationships", zap.String("type", relType), zap.Int("count", len(rels)))
	return nil
}

// batchUpdate runs a Cypher statement with UNWIND against batches of rows.
// This replaces N+1 individual session.ExecuteWrite calls with batched UNWIND queries.
func (w *BatchWriter) batchUpdate(ctx context.Context, cypher string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for i := 0; i < len(rows); i += w.batchSize {
		end := i + w.batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]

		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, cypher, map[string]any{"rows": batch})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			return fmt.Errorf("batch update: %w", err)
		}
	}
	return nil
}

// mergeKeyForLabel returns the MERGE property key for a given node label.
func mergeKeyForLabel(label string) string {
	switch label {
	case "Program":
		return "programId"
	case "Copybook":
		return "name"
	case "DataItem":
		return "fqn"
	case "Paragraph":
		return "mergeId"
	case "Section":
		return "mergeId"
	case "File":
		return "name"
	case "SQLStatement":
		return "id"
	case "CICSTransaction":
		return "id"
	case "JCLJob":
		return "jobName"
	case "JCLStep":
		return "id"
	case "BusinessDomain":
		return "name"
	case "Condition":
		return "fqn"
	case "Parameter":
		return "fqn"
	case "ExternalInterface":
		return "id"
	case "DDCard":
		return "id"
	case "DBTable":
		return "name"
	case "IDMSRecord":
		return "name"
	case "IDMSSchema":
		return "id"
	case "IDMSArea":
		return "name"
	case "IDMSSet":
		return "name"
	case "ExternalDatabase":
		return "name"
	case "ExternalDBTable":
		return "name"
	default:
		return "id"
	}
}

// WritePass1Result converts a Pass1Result to maps and writes nodes then relationships.
func (w *BatchWriter) WritePass1Result(ctx context.Context, result *graph.Pass1Result) error {
	// Count CALLS targets per program for callTargetCount property
	callTargetCounts := make(map[string]int)
	for _, r := range result.Relationships {
		if r.Type == graph.RelCalls {
			callTargetCounts[r.FromKey]++
		}
	}

	// Write Programs
	if len(result.Programs) > 0 {
		nodes := make([]map[string]any, len(result.Programs))
		for i, p := range result.Programs {
			nodes[i] = map[string]any{
				"id":               p.ID,
				"programId":        p.ProgramID,
				"filePath":         p.FilePath,
				"language":         p.Language,
				"lineCount":        p.LineCount,
				"executionMode":    p.ExecutionMode,
				"callTargetCount":  callTargetCounts[p.ProgramID],
			}
		}
		if err := w.WriteNodes(ctx, "Program", "programId", nodes); err != nil {
			return err
		}
	}

	// Write Paragraphs
	if len(result.Paragraphs) > 0 {
		nodes := make([]map[string]any, len(result.Paragraphs))
		for i, p := range result.Paragraphs {
			nodes[i] = map[string]any{
				"id":        p.ID,
				"name":      p.Name,
				"programId": p.ProgramID,
				"mergeId":   p.ProgramID + "." + p.Name,
			}
		}
		if err := w.WriteNodes(ctx, "Paragraph", "mergeId", nodes); err != nil {
			return err
		}
	}

	// Write Sections
	if len(result.Sections) > 0 {
		nodes := make([]map[string]any, len(result.Sections))
		for i, s := range result.Sections {
			nodes[i] = map[string]any{
				"id":        s.ID,
				"name":      s.Name,
				"programId": s.ProgramID,
				"mergeId":   s.ProgramID + "." + s.Name,
			}
		}
		if err := w.WriteNodes(ctx, "Section", "mergeId", nodes); err != nil {
			return err
		}
	}

	// Write Copybooks
	if len(result.Copybooks) > 0 {
		nodes := make([]map[string]any, len(result.Copybooks))
		for i, c := range result.Copybooks {
			nodes[i] = map[string]any{
				"id":   c.ID,
				"name": c.Name,
			}
		}
		if err := w.WriteNodes(ctx, "Copybook", "name", nodes); err != nil {
			return err
		}
	}

	// Write DataItems
	if len(result.DataItems) > 0 {
		nodes := make([]map[string]any, len(result.DataItems))
		for i, d := range result.DataItems {
			nodes[i] = map[string]any{
				"id":        d.ID,
				"name":      d.Name,
				"level":     d.Level,
				"programId": d.ProgramID,
				"fqn":       d.FQN,
				"picture":   d.Picture,
				"usage":     d.Usage,
			}
		}
		if err := w.WriteNodes(ctx, "DataItem", "fqn", nodes); err != nil {
			return err
		}
	}

	// Write Conditions (88-level)
	if len(result.Conditions) > 0 {
		nodes := make([]map[string]any, len(result.Conditions))
		for i, c := range result.Conditions {
			nodes[i] = map[string]any{
				"id":        c.ID,
				"name":      c.Name,
				"parent":    c.Parent,
				"value":     c.Value,
				"programId": c.ProgramID,
				"fqn":       c.FQN,
			}
		}
		if err := w.WriteNodes(ctx, "Condition", "fqn", nodes); err != nil {
			return err
		}
	}

	// Write Parameters (LINKAGE SECTION)
	if len(result.Parameters) > 0 {
		nodes := make([]map[string]any, len(result.Parameters))
		for i, p := range result.Parameters {
			nodes[i] = map[string]any{
				"id":        p.ID,
				"name":      p.Name,
				"level":     p.Level,
				"direction": p.Direction,
				"programId": p.ProgramID,
				"fqn":       p.FQN,
			}
		}
		if err := w.WriteNodes(ctx, "Parameter", "fqn", nodes); err != nil {
			return err
		}
	}

	// Write FileDefinitions
	if len(result.FileDefs) > 0 {
		nodes := make([]map[string]any, len(result.FileDefs))
		for i, f := range result.FileDefs {
			nodes[i] = map[string]any{
				"id":            f.ID,
				"name":          f.Name,
				"programId":     f.ProgramID,
				"organization":  f.Organization,
				"vsamType":      f.VSAMType,
				"dataStoreType": f.DataStoreType,
			}
		}
		if err := w.WriteNodes(ctx, "File", "name", nodes); err != nil {
			return err
		}
	}

	// Write SQLStatements
	if len(result.SQLStatements) > 0 {
		nodes := make([]map[string]any, len(result.SQLStatements))
		for i, s := range result.SQLStatements {
			nodes[i] = map[string]any{
				"id":          s.ID,
				"text":        s.Text,
				"programId":   s.ProgramID,
				"type":        s.Type,
				"targetTable": s.TargetTable,
			}
		}
		if err := w.WriteNodes(ctx, "SQLStatement", "id", nodes); err != nil {
			return err
		}
	}

	// Write CICSTransactions
	if len(result.CICSTxns) > 0 {
		nodes := make([]map[string]any, len(result.CICSTxns))
		for i, c := range result.CICSTxns {
			nodes[i] = map[string]any{
				"id":        c.ID,
				"command":   c.Command,
				"programId": c.ProgramID,
			}
		}
		if err := w.WriteNodes(ctx, "CICSTransaction", "id", nodes); err != nil {
			return err
		}
	}

	// Write ExternalInterfaces
	if len(result.ExternalInterfaces) > 0 {
		nodes := make([]map[string]any, len(result.ExternalInterfaces))
		for i, e := range result.ExternalInterfaces {
			nodes[i] = map[string]any{
				"id":        e.ID,
				"type":      e.Type,
				"details":   e.Details,
				"paragraph": e.Paragraph,
				"programId": e.ProgramID,
			}
		}
		if err := w.WriteNodes(ctx, "ExternalInterface", "id", nodes); err != nil {
			return err
		}
	}

	// Write DBTables + ACCESSES relationships
	if len(result.DBTables) > 0 {
		nodes := make([]map[string]any, len(result.DBTables))
		for i, t := range result.DBTables {
			nodes[i] = map[string]any{
				"id":     t.ID,
				"name":   t.Name,
				"schema": t.Schema,
			}
		}
		if err := w.WriteNodes(ctx, "DBTable", "name", nodes); err != nil {
			return err
		}

		// Write ACCESSES relationships for each DBTable
		programID := ""
		if len(result.Programs) > 0 {
			programID = result.Programs[0].ProgramID
		}
		if programID != "" {
			var accessRows []map[string]any
			for _, t := range result.DBTables {
				accessRows = append(accessRows, map[string]any{
					"fromKey": programID,
					"toKey":   t.Name,
					"props": map[string]any{
						"operations": t.Operations,
						"columns":    t.Columns,
					},
				})
			}
			if err := w.WriteRelationships(ctx, "ACCESSES", "Program", "programId", "DBTable", "name", accessRows); err != nil {
				return err
			}
		}
	}

	// Write IDMSSchema nodes
	if len(result.IDMSSchemas) > 0 {
		nodes := make([]map[string]any, len(result.IDMSSchemas))
		for i, s := range result.IDMSSchemas {
			nodes[i] = map[string]any{
				"id":            s.ID,
				"schemaName":    s.SchemaName,
				"subschemaName": s.SubschemaName,
				"programId":     s.ProgramID,
				"protocolMode":  s.ProtocolMode,
			}
		}
		if err := w.WriteNodes(ctx, "IDMSSchema", "id", nodes); err != nil {
			return err
		}
	}

	// Write IDMSRecord nodes
	if len(result.IDMSRecords) > 0 {
		nodes := make([]map[string]any, len(result.IDMSRecords))
		for i, r := range result.IDMSRecords {
			nodes[i] = map[string]any{
				"id":        r.ID,
				"name":      r.Name,
				"area":      r.Area,
				"schema":    r.Schema,
				"programId": r.ProgramID,
			}
		}
		if err := w.WriteNodes(ctx, "IDMSRecord", "name", nodes); err != nil {
			return err
		}
	}

	// Write IDMSArea nodes
	if len(result.IDMSAreas) > 0 {
		nodes := make([]map[string]any, len(result.IDMSAreas))
		for i, a := range result.IDMSAreas {
			nodes[i] = map[string]any{
				"id":        a.ID,
				"name":      a.Name,
				"usageMode": a.UsageMode,
				"schema":    a.Schema,
			}
		}
		if err := w.WriteNodes(ctx, "IDMSArea", "name", nodes); err != nil {
			return err
		}
	}

	// Write IDMSSet nodes
	if len(result.IDMSSets) > 0 {
		nodes := make([]map[string]any, len(result.IDMSSets))
		for i, s := range result.IDMSSets {
			nodes[i] = map[string]any{
				"id":           s.ID,
				"name":         s.Name,
				"ownerRecord":  s.OwnerRecord,
				"memberRecord": s.MemberRecord,
				"schema":       s.Schema,
			}
		}
		if err := w.WriteNodes(ctx, "IDMSSet", "name", nodes); err != nil {
			return err
		}
	}

	// Create stub Program nodes for CALLS targets not in the scan set
	knownPrograms := make(map[string]bool, len(result.Programs))
	for _, p := range result.Programs {
		knownPrograms[p.ProgramID] = true
	}
	var stubNodes []map[string]any
	seen := make(map[string]bool)
	for _, r := range result.Relationships {
		if r.Type == graph.RelCalls && r.ToLabel == "Program" {
			if !knownPrograms[r.ToKey] && !seen[r.ToKey] {
				seen[r.ToKey] = true
				stubNodes = append(stubNodes, map[string]any{
					"programId": r.ToKey,
					"id":        r.ToKey,
					"language":  "UNKNOWN",
				})
			}
		}
	}
	if len(stubNodes) > 0 {
		if err := w.WriteNodes(ctx, "Program", "programId", stubNodes); err != nil {
			w.logger.Warn("failed to write stub Program nodes for CALLS targets", zap.Error(err))
		}
	}

	// Write Relationships
	grouped := groupRelationships(result.Relationships)
	for key, rels := range grouped {
		rows := make([]map[string]any, len(rels))
		for i, r := range rels {
			props := r.Properties
			if props == nil {
				props = map[string]any{}
			}
			rows[i] = map[string]any{
				"fromKey": r.FromKey,
				"toKey":   r.ToKey,
				"props":   props,
			}
		}
		if err := w.WriteRelationships(ctx, string(key.relType), key.fromLabel, mergeKeyForLabel(key.fromLabel), key.toLabel, mergeKeyForLabel(key.toLabel), rows); err != nil {
			return err
		}
	}

	return nil
}

type relGroupKey struct {
	relType   graph.RelType
	fromLabel string
	toLabel   string
}

func groupRelationships(rels []graph.Relationship) map[relGroupKey][]graph.Relationship {
	groups := make(map[relGroupKey][]graph.Relationship)
	for _, r := range rels {
		key := relGroupKey{relType: r.Type, fromLabel: r.FromLabel, toLabel: r.ToLabel}
		groups[key] = append(groups[key], r)
	}
	return groups
}

// WritePass2Result writes Pass 2 deep semantic analysis results to Neo4j.
func (w *BatchWriter) WritePass2Result(ctx context.Context, result *graph.Pass2Result) error {
	programID := result.ProgramID
	var rels []graph.Relationship

	// PERFORMS relationships — use composite mergeId for Paragraph nodes
	for _, p := range result.Performs {
		rel := graph.Relationship{
			Type:      graph.RelPerforms,
			FromLabel: "Paragraph",
			FromKey:   programID + "." + p.FromParagraph,
			ToLabel:   "Paragraph",
			ToKey:     programID + "." + p.ToParagraph,
			Properties: map[string]any{
				"isLoop":    p.IsLoop,
				"condition": p.Condition,
			},
		}
		rels = append(rels, rel)

		if p.ThruParagraph != "" {
			rels = append(rels, graph.Relationship{
				Type:      graph.RelPerformsThru,
				FromLabel: "Paragraph",
				FromKey:   programID + "." + p.FromParagraph,
				ToLabel:   "Paragraph",
				ToKey:     programID + "." + p.ThruParagraph,
			})
		}
	}

	// File operation relationships (READS/WRITES)
	for _, f := range result.FileOps {
		var relType graph.RelType
		switch f.Operation {
		case "READ", "START":
			relType = graph.RelReads
		default:
			relType = graph.RelWrites
		}
		rels = append(rels, graph.Relationship{
			Type:      relType,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "File",
			ToKey:     f.FileName,
			Properties: map[string]any{
				"operation": f.Operation,
				"paragraph": f.Paragraph,
			},
		})
	}

	// Write PERFORMS/PERFORMS_THRU and file op relationships using grouped pattern
	grouped := groupRelationships(rels)
	for key, groupRels := range grouped {
		rows := make([]map[string]any, len(groupRels))
		for i, r := range groupRels {
			props := r.Properties
			if props == nil {
				props = map[string]any{}
			}
			rows[i] = map[string]any{
				"fromKey": r.FromKey,
				"toKey":   r.ToKey,
				"props":   props,
			}
		}
		if err := w.WriteRelationships(ctx, string(key.relType), key.fromLabel, mergeKeyForLabel(key.fromLabel), key.toLabel, mergeKeyForLabel(key.toLabel), rows); err != nil {
			return err
		}
	}

	// MOVES_TO (data flow) — custom Cypher matching on {name, programId}
	if len(result.DataFlows) > 0 {
		var flowRows []map[string]any
		for _, d := range result.DataFlows {
			flowRows = append(flowRows, map[string]any{
				"fromName": d.FromItem,
				"toName":   d.ToItem,
				"pid":      programID,
				"context":  d.Context,
			})
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row "+
				"MATCH (src:DataItem {name: row.fromName, programId: row.pid}) "+
				"MATCH (dst:DataItem {name: row.toName, programId: row.pid}) "+
				"MERGE (src)-[r:MOVES_TO]->(dst) SET r.context = row.context",
			flowRows); err != nil {
			w.logger.Warn("failed to write MOVES_TO relationships", zap.Error(err))
		}
	}

	// Ensure DataItem nodes exist for all hierarchy items (Pass 1 only creates 01/77-level)
	if len(result.DataHierarchy) > 0 {
		nodes := make([]map[string]any, len(result.DataHierarchy))
		for i, d := range result.DataHierarchy {
			fqn := fmt.Sprintf("%s.%02d.%s", programID, d.Level, d.Name)
			nodes[i] = map[string]any{
				"name":      d.Name,
				"level":     d.Level,
				"programId": programID,
				"fqn":       fqn,
				"picture":   d.Picture,
				"copybook":  d.Copybook,
			}
		}
		if err := w.WriteNodes(ctx, "DataItem", "fqn", nodes); err != nil {
			w.logger.Warn("failed to write DataItem nodes from hierarchy", zap.Error(err))
		}
	}

	// Data hierarchy — CHILD_OF relationships — FQN-based matching for accuracy
	if len(result.DataHierarchy) > 0 {
		// Build a level lookup from the hierarchy for FQN construction
		levelByName := make(map[string]int)
		for _, d := range result.DataHierarchy {
			levelByName[d.Name] = d.Level
		}

		var childRows []map[string]any
		for _, d := range result.DataHierarchy {
			if d.Parent != "" {
				childFQN := fmt.Sprintf("%s.%02d.%s", programID, d.Level, d.Name)
				parentLevel := findParentLevel(result.DataHierarchy, d.Parent, d.Name)
				if parentLevel > 0 {
					parentFQN := fmt.Sprintf("%s.%02d.%s", programID, parentLevel, d.Parent)
					childRows = append(childRows, map[string]any{
						"childFQN":  childFQN,
						"parentFQN": parentFQN,
					})
				} else {
					// Fallback to name-based matching if parent level unknown
					childRows = append(childRows, map[string]any{
						"childFQN":  childFQN,
						"parentFQN": "",
						"parentName": d.Parent,
						"pid":        programID,
					})
				}
			}
		}

		// Split into FQN-matched and fallback rows
		var fqnRows, fallbackRows []map[string]any
		for _, row := range childRows {
			if row["parentFQN"] != "" {
				fqnRows = append(fqnRows, row)
			} else {
				fallbackRows = append(fallbackRows, row)
			}
		}

		if len(fqnRows) > 0 {
			if err := w.batchUpdate(ctx,
				"UNWIND $rows AS row "+
					"MATCH (child:DataItem {fqn: row.childFQN}) "+
					"MATCH (parent:DataItem {fqn: row.parentFQN}) "+
					"MERGE (child)-[:CHILD_OF]->(parent)",
				fqnRows); err != nil {
				w.logger.Warn("failed to write CHILD_OF relationships (FQN)", zap.Error(err))
			}
		}
		if len(fallbackRows) > 0 {
			if err := w.batchUpdate(ctx,
				"UNWIND $rows AS row "+
					"MATCH (child:DataItem {fqn: row.childFQN}) "+
					"MATCH (parent:DataItem {name: row.parentName, programId: row.pid}) "+
					"MERGE (child)-[:CHILD_OF]->(parent)",
				fallbackRows); err != nil {
				w.logger.Warn("failed to write CHILD_OF relationships (fallback)", zap.Error(err))
			}
		}
	}

	// REDEFINES relationships — FQN-based matching where possible
	if len(result.Redefines) > 0 {
		// Build level lookup from hierarchy
		levelByName := make(map[string]int)
		for _, d := range result.DataHierarchy {
			levelByName[d.Name] = d.Level
		}

		var fqnRows, fallbackRows []map[string]any
		for _, r := range result.Redefines {
			itemLevel, itemOk := levelByName[r.Item]
			targetLevel, targetOk := levelByName[r.Redefines]
			if itemOk && targetOk {
				fqnRows = append(fqnRows, map[string]any{
					"itemFQN":   fmt.Sprintf("%s.%02d.%s", programID, itemLevel, r.Item),
					"targetFQN": fmt.Sprintf("%s.%02d.%s", programID, targetLevel, r.Redefines),
				})
			} else {
				fallbackRows = append(fallbackRows, map[string]any{
					"itemName":   r.Item,
					"targetName": r.Redefines,
					"pid":        programID,
				})
			}
		}

		if len(fqnRows) > 0 {
			if err := w.batchUpdate(ctx,
				"UNWIND $rows AS row "+
					"MATCH (item:DataItem {fqn: row.itemFQN}) "+
					"MATCH (target:DataItem {fqn: row.targetFQN}) "+
					"MERGE (item)-[:REDEFINES]->(target)",
				fqnRows); err != nil {
				w.logger.Warn("failed to write REDEFINES relationships (FQN)", zap.Error(err))
			}
		}
		if len(fallbackRows) > 0 {
			if err := w.batchUpdate(ctx,
				"UNWIND $rows AS row "+
					"MATCH (item:DataItem {name: row.itemName, programId: row.pid}) "+
					"MATCH (target:DataItem {name: row.targetName, programId: row.pid}) "+
					"MERGE (item)-[:REDEFINES]->(target)",
				fallbackRows); err != nil {
				w.logger.Warn("failed to write REDEFINES relationships (fallback)", zap.Error(err))
			}
		}
	}

	// DEFINED_IN (copybook definitions) — custom Cypher matching DataItem on {name, programId}
	if len(result.CopybookDefs) > 0 {
		var defRows []map[string]any
		for _, c := range result.CopybookDefs {
			defRows = append(defRows, map[string]any{
				"itemName": c.DataItem,
				"cbName":   c.Copybook,
				"pid":      programID,
			})
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row "+
				"MATCH (item:DataItem {name: row.itemName, programId: row.pid}) "+
				"MATCH (cb:Copybook {name: row.cbName}) "+
				"MERGE (item)-[:DEFINED_IN]->(cb)",
			defRows); err != nil {
			w.logger.Warn("failed to write DEFINED_IN relationships", zap.Error(err))
		}
	}

	// Update Paragraph nodes with conditional logic (batched)
	if len(result.ConditionalLogic) > 0 {
		var clRows []map[string]any
		for _, cl := range result.ConditionalLogic {
			if cl.Paragraph == "" {
				continue
			}
			clRows = append(clRows, map[string]any{
				"name":  cl.Paragraph,
				"pid":   programID,
				"entry": fmt.Sprintf("[%s] %s", cl.Type, cl.Condition),
			})
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Paragraph {name: row.name, programId: row.pid}) "+
				"SET p.conditionalLogic = coalesce(p.conditionalLogic, []) + [row.entry]",
			clRows); err != nil {
			w.logger.Warn("failed to update conditional logic", zap.Error(err))
		}
	}

	// Write dynamic call resolutions as CALLS relationships with resolved targets
	for _, dc := range result.DynamicCallResolutions {
		for _, target := range dc.ResolvedTargets {
			rels = append(rels, graph.Relationship{
				Type:      graph.RelCalls,
				FromLabel: "Program",
				FromKey:   programID,
				ToLabel:   "Program",
				ToKey:     target,
				Properties: map[string]any{
					"isDynamic":     true,
					"resolvedFrom":  dc.Variable,
					"fromParagraph": dc.Paragraph,
				},
			})
		}
	}

	// Update Paragraph nodes with error handling patterns (batched)
	if len(result.ErrorHandlers) > 0 {
		var ehRows []map[string]any
		for _, eh := range result.ErrorHandlers {
			if eh.Paragraph == "" {
				continue
			}
			ehRows = append(ehRows, map[string]any{
				"name":    eh.Paragraph,
				"pid":     programID,
				"pattern": eh.Pattern,
				"details": eh.Details,
			})
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Paragraph {name: row.name, programId: row.pid}) "+
				"SET p.errorPattern = row.pattern, p.errorDetails = row.details",
			ehRows); err != nil {
			w.logger.Warn("failed to update error handling", zap.Error(err))
		}
	}

	// Write IDMS operation relationships
	if len(result.IDMSOperations) > 0 {
		var idmsRels []graph.Relationship
		for _, op := range result.IDMSOperations {
			props := map[string]any{
				"verb":       op.Verb,
				"paragraph":  op.Paragraph,
				"navigation": op.Navigation,
				"calcKey":    op.CalcKey,
				"usageMode":  op.UsageMode,
			}

			switch op.Verb {
			case "OBTAIN", "FIND", "GET":
				if op.Record != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelNavigates,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSRecord",
						ToKey:      op.Record,
						Properties: props,
					})
				}
			case "STORE":
				if op.Record != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelStoresIn,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSRecord",
						ToKey:      op.Record,
						Properties: props,
					})
				}
			case "MODIFY":
				if op.Record != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelModifiesRec,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSRecord",
						ToKey:      op.Record,
						Properties: props,
					})
				}
			case "ERASE":
				if op.Record != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelErasesRec,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSRecord",
						ToKey:      op.Record,
						Properties: props,
					})
				}
			case "CONNECT":
				if op.Set != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelConnectsSet,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSSet",
						ToKey:      op.Set,
						Properties: props,
					})
				}
			case "DISCONNECT":
				if op.Set != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelDisconnectsSet,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSSet",
						ToKey:      op.Set,
						Properties: props,
					})
				}
			case "READY":
				if op.Area != "" {
					idmsRels = append(idmsRels, graph.Relationship{
						Type:       graph.RelReadyArea,
						FromLabel:  "Program",
						FromKey:    programID,
						ToLabel:    "IDMSArea",
						ToKey:      op.Area,
						Properties: props,
					})
				}
			}
		}

		if len(idmsRels) > 0 {
			idmsGrouped := groupRelationships(idmsRels)
			for key, groupRels := range idmsGrouped {
				rows := make([]map[string]any, len(groupRels))
				for i, r := range groupRels {
					p := r.Properties
					if p == nil {
						p = map[string]any{}
					}
					rows[i] = map[string]any{
						"fromKey": r.FromKey,
						"toKey":   r.ToKey,
						"props":   p,
					}
				}
				if err := w.WriteRelationships(ctx, string(key.relType), key.fromLabel, mergeKeyForLabel(key.fromLabel), key.toLabel, mergeKeyForLabel(key.toLabel), rows); err != nil {
					w.logger.Warn("failed to write IDMS relationships", zap.String("type", string(key.relType)), zap.Error(err))
				}
			}
		}
	}

	// Update Paragraph nodes with annotations (batched)
	if len(result.Annotations) > 0 {
		var annRows []map[string]any
		for _, a := range result.Annotations {
			if a.Paragraph == "" {
				continue
			}
			annRows = append(annRows, map[string]any{
				"name": a.Paragraph,
				"pid":  programID,
				"desc": a.Description,
				"cat":  a.Category,
			})
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Paragraph {name: row.name, programId: row.pid}) "+
				"SET p.description = row.desc, p.category = row.cat",
			annRows); err != nil {
			w.logger.Warn("failed to update paragraph annotations", zap.Error(err))
		}
	}

	return nil
}

// WritePass3Result writes Pass 3 cross-cutting analysis results to Neo4j.
func (w *BatchWriter) WritePass3Result(ctx context.Context, result *graph.Pass3Result) error {
	// MERGE BusinessDomain nodes
	if len(result.BusinessDomains) > 0 {
		nodes := make([]map[string]any, len(result.BusinessDomains))
		for i, d := range result.BusinessDomains {
			nodes[i] = map[string]any{
				"id":          d.ID,
				"name":        d.Name,
				"description": d.Description,
			}
		}
		if err := w.WriteNodes(ctx, "BusinessDomain", "name", nodes); err != nil {
			return fmt.Errorf("writing BusinessDomain nodes: %w", err)
		}
	}

	// Write BELONGS_TO relationships (Program → BusinessDomain)
	if len(result.DomainMembers) > 0 {
		rows := make([]map[string]any, len(result.DomainMembers))
		for i, m := range result.DomainMembers {
			rows[i] = map[string]any{
				"fromKey": m.ProgramID,
				"toKey":   m.DomainName,
				"props":   map[string]any{"confidence": m.Confidence},
			}
		}
		if err := w.WriteRelationships(ctx, "BELONGS_TO", "Program", "programId", "BusinessDomain", "name", rows); err != nil {
			return fmt.Errorf("writing BELONGS_TO relationships: %w", err)
		}
	}

	// Set deadCode flags on programs (batched)
	if len(result.DeadCodeFlags) > 0 {
		rows := make([]map[string]any, len(result.DeadCodeFlags))
		for i, dc := range result.DeadCodeFlags {
			rows[i] = map[string]any{"pid": dc.ProgramID, "reason": dc.Reason}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Program {programId: row.pid}) "+
				"SET p.deadCode = true, p.deadCodeReason = row.reason",
			rows); err != nil {
			w.logger.Warn("failed to set dead code flags", zap.Error(err))
		}
	}

	// Set risk flags on programs (batched)
	if len(result.RiskFlags) > 0 {
		rows := make([]map[string]any, len(result.RiskFlags))
		for i, rf := range result.RiskFlags {
			rows[i] = map[string]any{
				"pid":      rf.ProgramID,
				"score":    rf.Score,
				"riskType": rf.RiskType,
				"details":  rf.Details,
			}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Program {programId: row.pid}) "+
				"SET p.riskScore = row.score, p.riskType = row.riskType, p.riskDetails = row.details",
			rows); err != nil {
			w.logger.Warn("failed to set risk flags", zap.Error(err))
		}
	}

	// Set bridge program flags (batched)
	if len(result.BridgePrograms) > 0 {
		rows := make([]map[string]any, len(result.BridgePrograms))
		for i, bp := range result.BridgePrograms {
			rows[i] = map[string]any{
				"pid":     bp.ProgramID,
				"domains": bp.Domains,
				"reason":  bp.Reason,
			}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Program {programId: row.pid}) "+
				"SET p.isBridge = true, p.bridgeDomains = row.domains, p.bridgeReason = row.reason",
			rows); err != nil {
			w.logger.Warn("failed to set bridge program flags", zap.Error(err))
		}
	}

	// Set copybook risk flags (batched)
	if len(result.CopybookRisks) > 0 {
		rows := make([]map[string]any, len(result.CopybookRisks))
		for i, cr := range result.CopybookRisks {
			rows[i] = map[string]any{
				"name":   cr.Copybook,
				"risk":   cr.RiskLevel,
				"count":  cr.ProgramCount,
				"reason": cr.Reason,
			}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (c:Copybook {name: row.name}) "+
				"SET c.riskLevel = row.risk, c.programCount = row.count, c.riskReason = row.reason",
			rows); err != nil {
			w.logger.Warn("failed to set copybook risks", zap.Error(err))
		}
	}

	// Set modernization candidate flags (batched)
	if len(result.ModernizationCandidates) > 0 {
		rows := make([]map[string]any, len(result.ModernizationCandidates))
		for i, mc := range result.ModernizationCandidates {
			rows[i] = map[string]any{
				"pid":      mc.ProgramID,
				"score":    mc.Score,
				"reason":   mc.Reason,
				"approach": mc.Approach,
			}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Program {programId: row.pid}) "+
				"SET p.modernizationScore = row.score, p.modernizationReason = row.reason, p.modernizationApproach = row.approach",
			rows); err != nil {
			w.logger.Warn("failed to set modernization candidates", zap.Error(err))
		}
	}

	// Set volume estimates on programs (batched)
	if len(result.VolumeEstimates) > 0 {
		rows := make([]map[string]any, len(result.VolumeEstimates))
		for i, ve := range result.VolumeEstimates {
			rows[i] = map[string]any{
				"pid":      ve.ProgramID,
				"estimate": ve.Estimate,
				"reason":   ve.Reason,
			}
		}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (p:Program {programId: row.pid}) "+
				"SET p.volumeEstimate = row.estimate, p.volumeReason = row.reason",
			rows); err != nil {
			w.logger.Warn("failed to set volume estimates", zap.Error(err))
		}
	}

	w.logger.Info("wrote pass 3 results",
		zap.Int("domains", len(result.BusinessDomains)),
		zap.Int("members", len(result.DomainMembers)),
		zap.Int("deadCode", len(result.DeadCodeFlags)),
		zap.Int("riskFlags", len(result.RiskFlags)),
		zap.Int("bridgePrograms", len(result.BridgePrograms)),
		zap.Int("copybookRisks", len(result.CopybookRisks)),
		zap.Int("modernizationCandidates", len(result.ModernizationCandidates)),
		zap.Int("volumeEstimates", len(result.VolumeEstimates)),
	)

	return nil
}

// WriteJCLResult writes JCL analysis results to Neo4j.
func (w *BatchWriter) WriteJCLResult(ctx context.Context, result *graph.JCLAnalysisResult) error {
	// Write JCLJob nodes
	if len(result.Jobs) > 0 {
		nodes := make([]map[string]any, len(result.Jobs))
		for i, j := range result.Jobs {
			nodes[i] = map[string]any{
				"id":       j.ID,
				"jobName":  j.JobName,
				"class":    j.Class,
				"msgclass": j.MsgClass,
				"region":   j.Region,
				"cond":     j.Cond,
			}
		}
		if err := w.WriteNodes(ctx, "JCLJob", "jobName", nodes); err != nil {
			return fmt.Errorf("writing JCLJob nodes: %w", err)
		}
	}

	// Write JCLStep nodes
	if len(result.Steps) > 0 {
		nodes := make([]map[string]any, len(result.Steps))
		for i, s := range result.Steps {
			nodes[i] = map[string]any{
				"id":       s.ID,
				"stepName": s.StepName,
				"program":  s.Program,
				"proc":     s.Proc,
				"cond":     s.Cond,
				"jobName":  s.JobName,
				"order":    s.Order,
			}
		}
		if err := w.WriteNodes(ctx, "JCLStep", "id", nodes); err != nil {
			return fmt.Errorf("writing JCLStep nodes: %w", err)
		}
	}

	// Write DDCard nodes
	if len(result.DDCards) > 0 {
		nodes := make([]map[string]any, len(result.DDCards))
		for i, dd := range result.DDCards {
			nodes[i] = map[string]any{
				"id":       dd.ID,
				"ddName":   dd.DDName,
				"dsname":   dd.DSName,
				"disp":     dd.Disp,
				"isInput":  dd.IsInput,
				"isOutput": dd.IsOutput,
				"jobName":  dd.JobName,
				"stepName": dd.StepName,
			}
		}
		if err := w.WriteNodes(ctx, "DDCard", "id", nodes); err != nil {
			return fmt.Errorf("writing DDCard nodes: %w", err)
		}
	}

	// Write relationships
	grouped := groupRelationships(result.Relationships)
	for key, rels := range grouped {
		rows := make([]map[string]any, len(rels))
		for i, r := range rels {
			props := r.Properties
			if props == nil {
				props = map[string]any{}
			}
			rows[i] = map[string]any{
				"fromKey": r.FromKey,
				"toKey":   r.ToKey,
				"props":   props,
			}
		}
		if err := w.WriteRelationships(ctx, string(key.relType), key.fromLabel, mergeKeyForLabel(key.fromLabel), key.toLabel, mergeKeyForLabel(key.toLabel), rows); err != nil {
			return fmt.Errorf("writing JCL relationship %s: %w", key.relType, err)
		}
	}

	w.logger.Info("wrote JCL results",
		zap.Int("jobs", len(result.Jobs)),
		zap.Int("steps", len(result.Steps)),
		zap.Int("ddCards", len(result.DDCards)),
	)

	return nil
}

// findParentLevel looks up the COBOL level number for a parent data item in the hierarchy.
// Returns 0 if not found.
func findParentLevel(hierarchy []graph.DataHierarchyItem, parentName, childName string) int {
	for _, d := range hierarchy {
		if d.Name == parentName {
			return d.Level
		}
	}
	return 0
}

// WriteExternalDBResult writes external DB gap analysis results to Neo4j.
func (w *BatchWriter) WriteExternalDBResult(ctx context.Context, result *graph.ExternalDBResult) error {
	// Write ExternalDatabase node
	dbNode := []map[string]any{{
		"id":             result.Database.ID,
		"name":           result.Database.Name,
		"databaseType":   result.Database.DatabaseType,
		"connectionInfo": result.Database.ConnectionInfo,
	}}
	if err := w.WriteNodes(ctx, "ExternalDatabase", "name", dbNode); err != nil {
		return fmt.Errorf("writing ExternalDatabase node: %w", err)
	}

	// Write ExternalDBTable nodes
	if len(result.Tables) > 0 {
		nodes := make([]map[string]any, len(result.Tables))
		for i, t := range result.Tables {
			colJSON, _ := json.Marshal(t.Columns)
			nodes[i] = map[string]any{
				"id":           t.ID,
				"name":         t.Name,
				"schema":       t.Schema,
				"databaseName": t.DatabaseName,
				"databaseType": t.DatabaseType,
				"columns":      string(colJSON),
			}
		}
		if err := w.WriteNodes(ctx, "ExternalDBTable", "name", nodes); err != nil {
			return fmt.Errorf("writing ExternalDBTable nodes: %w", err)
		}

		// Write HOSTED_IN relationships (ExternalDBTable → ExternalDatabase)
		hostedRows := make([]map[string]any, len(result.Tables))
		for i, t := range result.Tables {
			hostedRows[i] = map[string]any{
				"fromKey": t.Name,
				"toKey":   result.Database.Name,
				"props":   map[string]any{},
			}
		}
		if err := w.WriteRelationships(ctx, "HOSTED_IN", "ExternalDBTable", "name", "ExternalDatabase", "name", hostedRows); err != nil {
			return fmt.Errorf("writing HOSTED_IN relationships: %w", err)
		}
	}

	// Write MAPS_TO_EXT_DB relationships (DBTable → ExternalDBTable)
	if len(result.Mappings) > 0 {
		var mappingRows []map[string]any
		for _, m := range result.Mappings {
			colMapJSON, _ := json.Marshal(m.ColumnMappings)
			mappingRows = append(mappingRows, map[string]any{
				"fromKey": m.CobolDBTable,
				"toKey":   m.ExternalTable,
				"props": map[string]any{
					"confidence":     m.Confidence,
					"reason":         m.Reason,
					"columnMappings": string(colMapJSON),
				},
			})
		}
		if err := w.WriteRelationships(ctx, "MAPS_TO_EXT_DB", "DBTable", "name", "ExternalDBTable", "name", mappingRows); err != nil {
			w.logger.Warn("failed to write MAPS_TO_EXT_DB relationships", zap.Error(err))
		}
	}

	// Store gaps as JSON property on ExternalDatabase node
	if len(result.Gaps) > 0 {
		gapJSON, _ := json.Marshal(result.Gaps)
		gapRows := []map[string]any{{
			"name": result.Database.Name,
			"gaps": string(gapJSON),
		}}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (n:ExternalDatabase {name: row.name}) SET n.gaps = row.gaps",
			gapRows); err != nil {
			w.logger.Warn("failed to write gap analysis", zap.Error(err))
		}
	}

	// Store data flows as JSON property on ExternalDatabase node
	if len(result.Flows) > 0 {
		flowJSON, _ := json.Marshal(result.Flows)
		flowRows := []map[string]any{{
			"name":  result.Database.Name,
			"flows": string(flowJSON),
		}}
		if err := w.batchUpdate(ctx,
			"UNWIND $rows AS row MATCH (n:ExternalDatabase {name: row.name}) SET n.dataFlows = row.flows",
			flowRows); err != nil {
			w.logger.Warn("failed to write data flows", zap.Error(err))
		}
	}

	w.logger.Info("wrote external DB results",
		zap.String("database", result.Database.Name),
		zap.Int("tables", len(result.Tables)),
		zap.Int("mappings", len(result.Mappings)),
		zap.Int("gaps", len(result.Gaps)),
		zap.Int("flows", len(result.Flows)),
	)

	return nil
}

// ReassignProgramDomain moves a program to a different business domain.
func (w *BatchWriter) ReassignProgramDomain(ctx context.Context, programID, newDomain string) error {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Delete existing BELONGS_TO edges for the program
		_, err := tx.Run(ctx,
			"MATCH (p:Program {programId: $pid})-[r:BELONGS_TO]->(:BusinessDomain) DELETE r",
			map[string]any{"pid": programID})
		if err != nil {
			return nil, err
		}

		// Ensure the target BusinessDomain node exists
		_, err = tx.Run(ctx,
			"MERGE (d:BusinessDomain {name: $name})",
			map[string]any{"name": newDomain})
		if err != nil {
			return nil, err
		}

		// Create new BELONGS_TO edge
		_, err = tx.Run(ctx,
			"MATCH (p:Program {programId: $pid}) "+
				"MATCH (d:BusinessDomain {name: $name}) "+
				"MERGE (p)-[r:BELONGS_TO]->(d) "+
				"SET r.confidence = 1.0, r.source = 'manual_override'",
			map[string]any{"pid": programID, "name": newDomain})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("reassigning program domain: %w", err)
	}

	w.logger.Info("reassigned program domain",
		zap.String("program", programID),
		zap.String("domain", newDomain))
	return nil
}
