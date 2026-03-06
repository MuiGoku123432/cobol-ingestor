package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// DetectDeadParagraphs runs a graph-only reachability analysis on the PERFORMS graph.
// For each program, it finds entry paragraphs (no incoming PERFORMS within that program),
// traverses PERFORMS/PERFORMS_THRU edges, and marks unreachable paragraphs.
func (w *BatchWriter) DetectDeadParagraphs(ctx context.Context) error {
	w.logger.Info("detecting dead paragraphs via PERFORMS reachability")

	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	// First, reset all paragraphs to reachable
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			"MATCH (p:Paragraph) SET p.isReachable = true, p.deadCodeReason = null", nil)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("resetting paragraph reachability: %w", err)
	}

	// Find all programs with more than one paragraph (single-paragraph programs are trivially reachable)
	result, err := session.Run(ctx,
		"MATCH (para:Paragraph)-[:BELONGS_TO]->(prog:Program) "+
			"WITH prog.programId AS pid, count(para) AS cnt "+
			"WHERE cnt > 1 "+
			"RETURN pid", nil)
	if err != nil {
		return fmt.Errorf("querying programs with paragraphs: %w", err)
	}

	var programIDs []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("pid"); ok {
			if s, ok := val.(string); ok {
				programIDs = append(programIDs, s)
			}
		}
	}

	if len(programIDs) == 0 {
		w.logger.Info("no programs with multiple paragraphs found")
		return nil
	}

	// For each program, compute reachability and mark dead paragraphs
	deadCount := 0
	for _, pid := range programIDs {
		count, err := w.detectDeadForProgram(ctx, pid)
		if err != nil {
			w.logger.Warn("failed to detect dead paragraphs for program",
				zap.String("programId", pid), zap.Error(err))
			continue
		}
		deadCount += count
	}

	w.logger.Info("dead paragraph detection complete",
		zap.Int("programsAnalyzed", len(programIDs)),
		zap.Int("deadParagraphs", deadCount))

	return nil
}

func (w *BatchWriter) detectDeadForProgram(ctx context.Context, programID string) (int, error) {
	session := w.client.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"pid": programID}

	// Step 1: Find entry paragraphs (no incoming PERFORMS within same program)
	// Step 2: Traverse PERFORMS/PERFORMS_THRU to find all reachable paragraphs
	// Step 3: Mark unreachable paragraphs
	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx,
			// Find entry paragraphs: paragraphs with no incoming PERFORMS from same program
			"MATCH (para:Paragraph)-[:BELONGS_TO]->(prog:Program {programId: $pid}) "+
				"WHERE NOT EXISTS { "+
				"  MATCH (caller:Paragraph)-[:PERFORMS]->(para) "+
				"  WHERE caller.programId = $pid "+
				"} "+
				"WITH prog, collect(para) AS entries "+
				// Traverse from all entries
				"UNWIND entries AS entry "+
				"MATCH path = (entry)-[:PERFORMS|PERFORMS_THRU*0..50]->(reachable:Paragraph) "+
				"WHERE reachable.programId = $pid "+
				"WITH prog, collect(DISTINCT reachable.mergeId) AS reachableIds "+
				// Mark unreachable
				"MATCH (dead:Paragraph)-[:BELONGS_TO]->(prog) "+
				"WHERE NOT dead.mergeId IN reachableIds "+
				"SET dead.isReachable = false, "+
				"    dead.deadCodeReason = 'Not reachable from entry paragraph' "+
				"RETURN count(dead) AS deadCount",
			params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			return getInt64(res.Record(), "deadCount"), nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	count := int(result.(int64))
	return count, nil
}

// GetDeadParagraphs returns unreachable paragraphs for a program.
func (c *Client) GetDeadParagraphs(ctx context.Context, programID string) ([]DeadParagraphInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Paragraph {programId: $pid}) WHERE p.isReachable = false "+
			"RETURN p.name AS name, p.deadCodeReason AS reason, p.description AS description, p.category AS category "+
			"ORDER BY p.name",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("dead paragraphs query: %w", err)
	}

	var items []DeadParagraphInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DeadParagraphInfo{
			Name:        getStr(rec, "name"),
			ProgramID:   programID,
			Reason:      getStr(rec, "reason"),
			Description: getStr(rec, "description"),
			Category:    getStr(rec, "category"),
		})
	}
	return items, nil
}

// GetDeadCodeSummary returns aggregate dead paragraph counts per program.
func (c *Client) GetDeadCodeSummary(ctx context.Context) ([]DeadCodeSummaryInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Paragraph)-[:BELONGS_TO]->(prog:Program) "+
			"WITH prog.programId AS programId, "+
			"     count(p) AS totalParagraphs, "+
			"     sum(CASE WHEN p.isReachable = false THEN 1 ELSE 0 END) AS deadParagraphs "+
			"WHERE deadParagraphs > 0 "+
			"RETURN programId, totalParagraphs, deadParagraphs "+
			"ORDER BY deadParagraphs DESC", nil)
	if err != nil {
		return nil, fmt.Errorf("dead code summary query: %w", err)
	}

	var items []DeadCodeSummaryInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DeadCodeSummaryInfo{
			ProgramID:       getStr(rec, "programId"),
			TotalParagraphs: int(getInt64(rec, "totalParagraphs")),
			DeadParagraphs:  int(getInt64(rec, "deadParagraphs")),
		})
	}
	return items, nil
}
