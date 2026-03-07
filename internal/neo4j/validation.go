package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// ValidationResult holds the results of all validation checks.
type ValidationResult struct {
	Checks []ValidationCheck `json:"checks"`
}

// ValidationCheck represents a single validation gap check.
type ValidationCheck struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"` // WARN or INFO
	Count       int      `json:"count"`
	Samples     []string `json:"samples"`   // up to 10 sample IDs
	AutoFixed   bool     `json:"autoFixed"`
	FixMethod   string   `json:"fixMethod"` // GRAPH_ONLY, LLM_REPAIR, RERUN_PASS3
}

// RunValidation runs all 9 validation checks and returns gaps found.
func (w *BatchWriter) RunValidation(ctx context.Context) (*ValidationResult, error) {
	result := &ValidationResult{}

	checks := []struct {
		name        string
		description string
		severity    string
		fixMethod   string
		cypher      string
		idField     string
	}{
		{
			name:        "missing_pass3",
			description: "Programs missing Pass 3 analysis (no riskScore/domains)",
			severity:    "WARN",
			fixMethod:   "RERUN_PASS3",
			cypher:      "MATCH (p:Program) WHERE p.riskScore IS NULL RETURN p.programId AS id",
			idField:     "id",
		},
		{
			name:        "missing_child_of",
			description: "Programs with DataItems but zero CHILD_OF relationships",
			severity:    "WARN",
			fixMethod:   "LLM_REPAIR",
			cypher: "MATCH (p:Program) WHERE EXISTS { MATCH (d:DataItem {programId: p.programId}) } " +
				"AND NOT EXISTS { MATCH (d1:DataItem {programId: p.programId})-[:CHILD_OF]->(:DataItem) } " +
				"RETURN p.programId AS id",
			idField: "id",
		},
		{
			name:        "missing_moves_to",
			description: "Programs with DataItems but zero MOVES_TO relationships",
			severity:    "WARN",
			fixMethod:   "LLM_REPAIR",
			cypher: "MATCH (p:Program) WHERE EXISTS { MATCH (d:DataItem {programId: p.programId}) } " +
				"AND NOT EXISTS { MATCH (d1:DataItem {programId: p.programId})-[:MOVES_TO]->(:DataItem) } " +
				"RETURN p.programId AS id",
			idField: "id",
		},
		{
			name:        "missing_calls",
			description: "Programs expected to have CALLS but have none",
			severity:    "WARN",
			fixMethod:   "LLM_REPAIR",
			cypher:      "MATCH (p:Program) WHERE p.callCount > 0 AND NOT (p)-[:CALLS]->() RETURN p.programId AS id",
			idField:     "id",
		},
		{
			name:        "unannotated_paragraphs",
			description: "Programs with unannotated paragraphs (no description)",
			severity:    "INFO",
			fixMethod:   "LLM_REPAIR",
			cypher: "MATCH (para:Paragraph) WHERE para.description IS NULL " +
				"WITH para.programId AS pid, count(*) AS cnt WHERE cnt > 0 " +
				"RETURN pid AS id",
			idField: "id",
		},
		{
			name:        "unlinked_ddcards",
			description: "DD cards not linked to File nodes",
			severity:    "INFO",
			fixMethod:   "GRAPH_ONLY",
			cypher:      "MATCH (dd:DDCard) WHERE NOT (dd)-[:MAPS_TO_FILE]->() RETURN dd.id AS id",
			idField:     "id",
		},
		{
			name:        "dangling_calls",
			description: "CALLS targets with no source file (external/system programs)",
			severity:    "INFO",
			fixMethod:   "GRAPH_ONLY",
			cypher:      "MATCH (a:Program)-[:CALLS]->(b:Program) WHERE b.filePath IS NULL RETURN DISTINCT b.programId AS id",
			idField:     "id",
		},
		{
			name:        "orphan_data_items",
			description: "DataItems with no programId",
			severity:    "INFO",
			fixMethod:   "GRAPH_ONLY",
			cypher:      "MATCH (d:DataItem) WHERE d.programId IS NULL RETURN d.fqn AS id",
			idField:     "id",
		},
		{
			name:        "missing_dashboard_counts",
			description: "DashboardStats missing DDCard and DBTable counts",
			severity:    "INFO",
			fixMethod:   "GRAPH_ONLY",
			cypher:      "RETURN 'check' AS id", // always runs as a marker
			idField:     "id",
		},
	}

	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	for _, chk := range checks {
		vc := ValidationCheck{
			Name:        chk.name,
			Description: chk.description,
			Severity:    chk.severity,
			FixMethod:   chk.fixMethod,
		}

		res, err := session.Run(ctx, chk.cypher, nil)
		if err != nil {
			w.logger.Warn("validation check failed", zap.String("check", chk.name), zap.Error(err))
			result.Checks = append(result.Checks, vc)
			continue
		}

		for res.Next(ctx) {
			vc.Count++
			if len(vc.Samples) < 10 {
				if val, ok := res.Record().Get(chk.idField); ok && val != nil {
					if s, ok := val.(string); ok {
						vc.Samples = append(vc.Samples, s)
					}
				}
			}
		}

		result.Checks = append(result.Checks, vc)
	}

	return result, nil
}

// MergeDuplicateDomains collapses BusinessDomain nodes that share >80% of their programs.
// Returns the number of domains merged.
func (w *BatchWriter) MergeDuplicateDomains(ctx context.Context) (int, error) {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	// Find domain pairs with high program overlap
	res, err := session.Run(ctx,
		"MATCH (d1:BusinessDomain)<-[:BELONGS_TO]-(p:Program)-[:BELONGS_TO]->(d2:BusinessDomain) "+
			"WHERE id(d1) < id(d2) "+
			"WITH d1, d2, count(p) AS shared, "+
			"size([(p1:Program)-[:BELONGS_TO]->(d1) | p1]) AS size1, "+
			"size([(p2:Program)-[:BELONGS_TO]->(d2) | p2]) AS size2 "+
			"WHERE shared > 0.8 * toFloat(CASE WHEN size1 < size2 THEN size1 ELSE size2 END) "+
			"RETURN d1.name AS name1, d2.name AS name2, shared, size1, size2",
		nil)
	if err != nil {
		return 0, fmt.Errorf("querying duplicate domains: %w", err)
	}

	type mergePair struct {
		keepName  string
		mergeName string
	}
	var pairs []mergePair

	for res.Next(ctx) {
		rec := res.Record()
		name1 := getStr(rec, "name1")
		name2 := getStr(rec, "name2")
		size1Val, _ := rec.Get("size1")
		size2Val, _ := rec.Get("size2")
		s1, _ := size1Val.(int64)
		s2, _ := size2Val.(int64)

		// Keep the domain with more members; shorter name as tiebreaker
		keep, merge := name1, name2
		if s2 > s1 || (s1 == s2 && len(name2) < len(name1)) {
			keep, merge = name2, name1
		}

		w.logger.Info("merging duplicate domains",
			zap.String("keep", keep),
			zap.String("merge", merge),
			zap.Int64("size1", s1),
			zap.Int64("size2", s2),
		)

		pairs = append(pairs, mergePair{keepName: keep, mergeName: merge})
	}

	merged := 0
	for _, pair := range pairs {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			// Reassign programs not already in the kept domain
			_, err := tx.Run(ctx,
				"MATCH (p:Program)-[r:BELONGS_TO]->(d:BusinessDomain {name: $mergeName}) "+
					"WHERE NOT (p)-[:BELONGS_TO]->(:BusinessDomain {name: $keepName}) "+
					"WITH p, r "+
					"MATCH (keep:BusinessDomain {name: $keepName}) "+
					"MERGE (p)-[:BELONGS_TO {confidence: r.confidence}]->(keep) "+
					"DELETE r",
				map[string]any{"mergeName": pair.mergeName, "keepName": pair.keepName})
			if err != nil {
				return nil, err
			}

			// Delete remaining duplicate edges
			_, err = tx.Run(ctx,
				"MATCH (:Program)-[r:BELONGS_TO]->(d:BusinessDomain {name: $mergeName}) DELETE r",
				map[string]any{"mergeName": pair.mergeName})
			if err != nil {
				return nil, err
			}

			// Delete the merged domain node
			_, err = tx.Run(ctx,
				"MATCH (d:BusinessDomain {name: $mergeName}) DELETE d",
				map[string]any{"mergeName": pair.mergeName})
			if err != nil {
				return nil, err
			}

			// Update any BridgeProgram references
			_, err = tx.Run(ctx,
				"MATCH (bp:Program) WHERE bp.bridgeDomains IS NOT NULL "+
					"AND $mergeName IN bp.bridgeDomains "+
					"SET bp.bridgeDomains = [d IN bp.bridgeDomains WHERE d <> $mergeName] + "+
					"CASE WHEN $keepName IN bp.bridgeDomains THEN [] ELSE [$keepName] END",
				map[string]any{"mergeName": pair.mergeName, "keepName": pair.keepName})
			return nil, err
		})
		if err != nil {
			w.logger.Warn("failed to merge domain pair",
				zap.String("keep", pair.keepName),
				zap.String("merge", pair.mergeName),
				zap.Error(err))
			continue
		}
		merged++
	}

	return merged, nil
}

// QueryProgramsMissingPass3 returns program IDs that lack riskScore (never processed by Pass 3).
// All programs processed by Pass 3 receive a riskScore (even low-risk ones),
// so riskScore IS NULL reliably indicates programs that were skipped or failed.
func (c *Client) QueryProgramsMissingPass3(ctx context.Context) ([]string, error) {
	return c.queryIDList(ctx, "MATCH (p:Program) WHERE p.riskScore IS NULL RETURN p.programId AS id")
}

// QueryProgramsMissingChildOf returns program IDs that have DataItems but no CHILD_OF relationships.
func (c *Client) QueryProgramsMissingChildOf(ctx context.Context) ([]string, error) {
	return c.queryIDList(ctx,
		"MATCH (p:Program) WHERE EXISTS { MATCH (d:DataItem {programId: p.programId}) } "+
			"AND NOT EXISTS { MATCH (d1:DataItem {programId: p.programId})-[:CHILD_OF]->(:DataItem) } "+
			"RETURN p.programId AS id")
}

// QueryProgramsMissingMovesTo returns program IDs that have DataItems but no MOVES_TO relationships.
func (c *Client) QueryProgramsMissingMovesTo(ctx context.Context) ([]string, error) {
	return c.queryIDList(ctx,
		"MATCH (p:Program) WHERE EXISTS { MATCH (d:DataItem {programId: p.programId}) } "+
			"AND NOT EXISTS { MATCH (d1:DataItem {programId: p.programId})-[:MOVES_TO]->(:DataItem) } "+
			"RETURN p.programId AS id")
}

// QueryProgramsMissingCalls returns program IDs expected to have CALLS but have none.
func (c *Client) QueryProgramsMissingCalls(ctx context.Context) ([]string, error) {
	return c.queryIDList(ctx,
		"MATCH (p:Program) WHERE p.callCount > 0 AND NOT (p)-[:CALLS]->() RETURN p.programId AS id")
}

// QueryUnannotatedParagraphs returns a map of programID → paragraph names that lack descriptions.
func (c *Client) QueryUnannotatedParagraphs(ctx context.Context) (map[string][]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (para:Paragraph) WHERE para.description IS NULL "+
			"RETURN para.programId AS pid, para.name AS name",
		nil)
	if err != nil {
		return nil, fmt.Errorf("querying unannotated paragraphs: %w", err)
	}

	pMap := make(map[string][]string)
	for result.Next(ctx) {
		rec := result.Record()
		pid := getStr(rec, "pid")
		name := getStr(rec, "name")
		if pid != "" && name != "" {
			pMap[pid] = append(pMap[pid], name)
		}
	}
	return pMap, nil
}

// FixUnlinkedDDCards applies extended heuristic linking between DDCards and File nodes.
func (w *BatchWriter) FixUnlinkedDDCards(ctx context.Context) (int, error) {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	// Heuristic 1: Match DDCard.ddName to File.name (case-insensitive)
	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (dd:DDCard) WHERE NOT (dd)-[:MAPS_TO_FILE]->() "+
				"MATCH (f:File) WHERE toUpper(dd.ddName) = toUpper(f.name) "+
				"MERGE (dd)-[:MAPS_TO_FILE {source: 'pass5_heuristic', method: 'ddname_match'}]->(f) "+
				"RETURN count(*) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, fmt.Errorf("fixing unlinked DD cards (heuristic 1): %w", err)
	}
	count1 := int(res.(int64))

	// Heuristic 2: Match DD dsname suffix to File name
	res2, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (dd:DDCard) WHERE NOT (dd)-[:MAPS_TO_FILE]->() AND dd.dsname IS NOT NULL "+
				"MATCH (f:File) "+
				"WHERE dd.dsname ENDS WITH '.' + f.name OR dd.dsname CONTAINS '.' + f.name + '.' "+
				"MERGE (dd)-[:MAPS_TO_FILE {source: 'pass5_heuristic', method: 'dsname_suffix'}]->(f) "+
				"RETURN count(*) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return count1, fmt.Errorf("fixing unlinked DD cards (heuristic 2): %w", err)
	}
	count2 := int(res2.(int64))

	total := count1 + count2
	if total > 0 {
		w.logger.Info("fixed unlinked DD cards", zap.Int("ddnameMatch", count1), zap.Int("dsnameMatch", count2))
	}
	return total, nil
}

// FixDanglingCalls marks stub Program nodes (no filePath) as external/system programs.
func (w *BatchWriter) FixDanglingCalls(ctx context.Context) (int, error) {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	res, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx,
			"MATCH (b:Program) WHERE b.filePath IS NULL AND EXISTS { MATCH ()-[:CALLS]->(b) } "+
				"SET b.isExternal = true, b.language = COALESCE(b.language, 'EXTERNAL') "+
				"RETURN count(b) AS cnt",
			nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if val, ok := result.Record().Get("cnt"); ok {
				if n, ok := val.(int64); ok {
					return n, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, fmt.Errorf("fixing dangling calls: %w", err)
	}

	count := int(res.(int64))
	if count > 0 {
		w.logger.Info("marked external programs", zap.Int("count", count))
	}
	return count, nil
}

// WriteParagraphAnnotations updates Paragraph nodes with descriptions and categories.
func (w *BatchWriter) WriteParagraphAnnotations(ctx context.Context, programID string, annotations []map[string]any) error {
	if len(annotations) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	cypher := "UNWIND $rows AS row " +
		"MATCH (p:Paragraph {programId: $pid, name: row.name}) " +
		"SET p.description = row.description, p.category = row.category"

	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypher, map[string]any{
			"pid":  programID,
			"rows": annotations,
		})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("writing paragraph annotations: %w", err)
	}

	w.logger.Debug("wrote paragraph annotations", zap.String("program", programID), zap.Int("count", len(annotations)))
	return nil
}

// WriteRepairRelationships writes repair-generated relationships to Neo4j using MERGE.
func (w *BatchWriter) WriteRepairRelationships(ctx context.Context, relType, fromLabel, fromKey, toLabel, toKey string, rels []map[string]any) error {
	return w.WriteRelationships(ctx, relType, fromLabel, fromKey, toLabel, toKey, rels)
}

// queryIDList runs a Cypher query and collects string "id" values.
func (c *Client) queryIDList(ctx context.Context, cypher string) ([]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, nil)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var ids []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("id"); ok && val != nil {
			if s, ok := val.(string); ok {
				ids = append(ids, s)
			}
		}
	}
	return ids, nil
}

// GetProgramFilePath returns the file path for a given program ID.
func (c *Client) GetProgramFilePath(ctx context.Context, programID string) (string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program {programId: $id}) RETURN p.filePath AS path",
		map[string]any{"id": programID})
	if err != nil {
		return "", fmt.Errorf("querying program file path: %w", err)
	}

	if result.Next(ctx) {
		return getStr(result.Record(), "path"), nil
	}
	return "", nil
}
