package neo4j

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/graph"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// BatchWriter writes graph data to Neo4j in batches.
type BatchWriter struct {
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
		return "name"
	case "Section":
		return "name"
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
		return "name"
	case "Parameter":
		return "name"
	default:
		return "id"
	}
}

// WritePass1Result converts a Pass1Result to maps and writes nodes then relationships.
func (w *BatchWriter) WritePass1Result(ctx context.Context, result *graph.Pass1Result) error {
	// Write Programs
	if len(result.Programs) > 0 {
		nodes := make([]map[string]any, len(result.Programs))
		for i, p := range result.Programs {
			nodes[i] = map[string]any{
				"id":        p.ID,
				"programId": p.ProgramID,
				"filePath":  p.FilePath,
				"language":  p.Language,
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
			}
		}
		if err := w.WriteNodes(ctx, "Paragraph", "name", nodes); err != nil {
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
			}
		}
		if err := w.WriteNodes(ctx, "Section", "name", nodes); err != nil {
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
			}
		}
		if err := w.WriteNodes(ctx, "Condition", "name", nodes); err != nil {
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
			}
		}
		if err := w.WriteNodes(ctx, "Parameter", "name", nodes); err != nil {
			return err
		}
	}

	// Write FileDefinitions
	if len(result.FileDefs) > 0 {
		nodes := make([]map[string]any, len(result.FileDefs))
		for i, f := range result.FileDefs {
			nodes[i] = map[string]any{
				"id":           f.ID,
				"name":         f.Name,
				"programId":    f.ProgramID,
				"organization": f.Organization,
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
				"id":        s.ID,
				"text":      s.Text,
				"programId": s.ProgramID,
				"type":      s.Type,
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

	// PERFORMS relationships
	for _, p := range result.Performs {
		rel := graph.Relationship{
			Type:      graph.RelPerforms,
			FromLabel: "Paragraph",
			FromKey:   p.FromParagraph,
			ToLabel:   "Paragraph",
			ToKey:     p.ToParagraph,
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
				FromKey:   p.FromParagraph,
				ToLabel:   "Paragraph",
				ToKey:     p.ThruParagraph,
			})
		}
	}

	// MOVES_TO (data flow) relationships
	for _, d := range result.DataFlows {
		rels = append(rels, graph.Relationship{
			Type:      graph.RelMovesTo,
			FromLabel: "DataItem",
			FromKey:   d.FromItem,
			ToLabel:   "DataItem",
			ToKey:     d.ToItem,
			Properties: map[string]any{
				"context": d.Context,
			},
		})
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

	// Data hierarchy — CHILD_OF relationships
	for _, d := range result.DataHierarchy {
		if d.Parent != "" {
			rels = append(rels, graph.Relationship{
				Type:      graph.RelChildOf,
				FromLabel: "DataItem",
				FromKey:   d.Name,
				ToLabel:   "DataItem",
				ToKey:     d.Parent,
			})
		}
	}

	// REDEFINES relationships
	for _, r := range result.Redefines {
		rels = append(rels, graph.Relationship{
			Type:      graph.RelRedefines,
			FromLabel: "DataItem",
			FromKey:   r.Item,
			ToLabel:   "DataItem",
			ToKey:     r.Redefines,
		})
	}

	// DEFINED_IN (copybook definitions)
	for _, c := range result.CopybookDefs {
		rels = append(rels, graph.Relationship{
			Type:      graph.RelDefinedIn,
			FromLabel: "DataItem",
			FromKey:   c.DataItem,
			ToLabel:   "Copybook",
			ToKey:     c.Copybook,
		})
	}

	// Write relationships using existing grouped pattern
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

	// Update Paragraph nodes with conditional logic
	for _, cl := range result.ConditionalLogic {
		if cl.Paragraph == "" {
			continue
		}
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Paragraph {name: $name}) "+
					"SET p.conditionalLogic = coalesce(p.conditionalLogic, []) + [$entry]",
				map[string]any{
					"name":  cl.Paragraph,
					"entry": fmt.Sprintf("[%s] %s", cl.Type, cl.Condition),
				},
			)
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to update conditional logic",
				zap.String("paragraph", cl.Paragraph), zap.Error(err))
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

	// Update Paragraph nodes with error handling patterns
	for _, eh := range result.ErrorHandlers {
		if eh.Paragraph == "" {
			continue
		}
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Paragraph {name: $name}) SET p.errorPattern = $pattern, p.errorDetails = $details",
				map[string]any{
					"name":    eh.Paragraph,
					"pattern": eh.Pattern,
					"details": eh.Details,
				},
			)
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to update error handling",
				zap.String("paragraph", eh.Paragraph), zap.Error(err))
		}
	}

	// Update Paragraph nodes with annotations (description + category)
	if len(result.Annotations) > 0 {
		for _, a := range result.Annotations {
			if a.Paragraph == "" {
				continue
			}
			session := w.client.NewSession(ctx)
			_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
				_, err := tx.Run(ctx,
					"MATCH (p:Paragraph {name: $name}) SET p.description = $desc, p.category = $cat",
					map[string]any{
						"name": a.Paragraph,
						"desc": a.Description,
						"cat":  a.Category,
					},
				)
				return nil, err
			})
			session.Close(ctx)
			if err != nil {
				w.logger.Warn("failed to update paragraph annotation",
					zap.String("paragraph", a.Paragraph),
					zap.Error(err),
				)
			}
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

	// Set deadCode flags on programs
	for _, dc := range result.DeadCodeFlags {
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Program {programId: $pid}) SET p.deadCode = true, p.deadCodeReason = $reason",
				map[string]any{"pid": dc.ProgramID, "reason": dc.Reason})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to set dead code flag",
				zap.String("program", dc.ProgramID), zap.Error(err))
		}
	}

	// Set risk flags on programs
	for _, rf := range result.RiskFlags {
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Program {programId: $pid}) SET p.riskScore = $score, p.riskType = $riskType, p.riskDetails = $details",
				map[string]any{
					"pid":      rf.ProgramID,
					"score":    rf.Score,
					"riskType": rf.RiskType,
					"details":  rf.Details,
				})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to set risk flag",
				zap.String("program", rf.ProgramID), zap.Error(err))
		}
	}

	// Set bridge program flags
	for _, bp := range result.BridgePrograms {
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Program {programId: $pid}) SET p.isBridge = true, p.bridgeDomains = $domains, p.bridgeReason = $reason",
				map[string]any{
					"pid":     bp.ProgramID,
					"domains": bp.Domains,
					"reason":  bp.Reason,
				})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to set bridge program flag",
				zap.String("program", bp.ProgramID), zap.Error(err))
		}
	}

	// Set copybook risk flags
	for _, cr := range result.CopybookRisks {
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (c:Copybook {name: $name}) SET c.riskLevel = $risk, c.programCount = $count, c.riskReason = $reason",
				map[string]any{
					"name":   cr.Copybook,
					"risk":   cr.RiskLevel,
					"count":  cr.ProgramCount,
					"reason": cr.Reason,
				})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to set copybook risk",
				zap.String("copybook", cr.Copybook), zap.Error(err))
		}
	}

	// Set modernization candidate flags
	for _, mc := range result.ModernizationCandidates {
		session := w.client.NewSession(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx,
				"MATCH (p:Program {programId: $pid}) SET p.modernizationScore = $score, p.modernizationReason = $reason, p.modernizationApproach = $approach",
				map[string]any{
					"pid":      mc.ProgramID,
					"score":    mc.Score,
					"reason":   mc.Reason,
					"approach": mc.Approach,
				})
			return nil, err
		})
		session.Close(ctx)
		if err != nil {
			w.logger.Warn("failed to set modernization candidate",
				zap.String("program", mc.ProgramID), zap.Error(err))
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
	)

	return nil
}
