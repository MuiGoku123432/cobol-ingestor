package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// globalCB is the sentinel codebase value for universal glossary terms
// that apply across all codebases.
const globalCB = "global"

func (c *Client) GetGlossaryTerm(ctx context.Context, codebase, term string) (*GlossaryTermDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	cb := codebase
	if cb == "" {
		cb = globalCB
	}
	tl := strings.ToLower(term)
	// Return scoped term first; fall back to global.
	result, err := session.Run(ctx,
		"MATCH (g:GlossaryTerm) WHERE g.termLower = $tl AND g.codebase IN [$cb, $global] "+
			"RETURN g.term AS term, g.kind AS kind, g.definition AS definition, "+
			"g.aliases AS aliases, g.codebase AS codebase, g.sourceFile AS sourceFile "+
			"ORDER BY CASE g.codebase WHEN $cb THEN 0 ELSE 1 END LIMIT 1",
		map[string]any{"cb": cb, "tl": tl, "global": globalCB})
	if err != nil {
		return nil, fmt.Errorf("get glossary term: %w", err)
	}
	if !result.Next(ctx) {
		return nil, nil
	}
	return glossaryRecordToDetail(result.Record()), nil
}

func (c *Client) SearchGlossaryTerms(ctx context.Context, codebase, query string, limit int) ([]GlossaryTermDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	if limit <= 0 {
		limit = 20
	}
	cb := codebase
	if cb == "" {
		cb = globalCB
	}

	// Include both scoped terms and global (universal) terms.
	result, err := session.Run(ctx,
		"CALL db.index.fulltext.queryNodes('glossary_fulltext', $query) YIELD node, score "+
			"WHERE node.codebase IN [$cb, $global] "+
			"RETURN node.term AS term, node.kind AS kind, node.definition AS definition, "+
			"node.aliases AS aliases, node.codebase AS codebase, node.sourceFile AS sourceFile "+
			"ORDER BY score DESC LIMIT $limit",
		map[string]any{"query": query, "cb": cb, "global": globalCB, "limit": int64(limit)})
	if err != nil {
		return nil, fmt.Errorf("search glossary terms: %w", err)
	}

	var items []GlossaryTermDetail
	for result.Next(ctx) {
		items = append(items, *glossaryRecordToDetail(result.Record()))
	}
	return items, nil
}

func (c *Client) ListGlossaryTerms(ctx context.Context, codebase string, page, pageSize int) (*GlossaryTermPage, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	cb := codebase
	if cb == "" {
		cb = globalCB
	}
	skip := (page - 1) * pageSize

	// Count includes both scoped and global terms.
	total, err := c.runCountQuery(ctx, session,
		"MATCH (g:GlossaryTerm) WHERE g.codebase IN [$cb, $global] RETURN count(g) AS total",
		map[string]any{"cb": cb, "global": globalCB})
	if err != nil {
		return nil, err
	}

	result, err := session.Run(ctx,
		"MATCH (g:GlossaryTerm) WHERE g.codebase IN [$cb, $global] "+
			"RETURN g.term AS term, g.kind AS kind, g.definition AS definition, "+
			"g.aliases AS aliases, g.codebase AS codebase, g.sourceFile AS sourceFile "+
			"ORDER BY g.termLower SKIP $skip LIMIT $limit",
		map[string]any{"cb": cb, "global": globalCB, "skip": int64(skip), "limit": int64(pageSize)})
	if err != nil {
		return nil, fmt.Errorf("list glossary terms: %w", err)
	}

	var items []GlossaryTermDetail
	for result.Next(ctx) {
		items = append(items, *glossaryRecordToDetail(result.Record()))
	}
	return &GlossaryTermPage{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func glossaryRecordToDetail(rec interface{ Get(string) (any, bool) }) *GlossaryTermDetail {
	d := &GlossaryTermDetail{
		Term:       getStrFromRec(rec, "term"),
		Kind:       getStrFromRec(rec, "kind"),
		Definition: getStrFromRec(rec, "definition"),
		Codebase:   getStrFromRec(rec, "codebase"),
		SourceFile: getStrFromRec(rec, "sourceFile"),
	}
	if raw, ok := rec.Get("aliases"); ok && raw != nil {
		switch v := raw.(type) {
		case string:
			_ = json.Unmarshal([]byte(v), &d.Aliases)
		case []any:
			for _, a := range v {
				if s, ok := a.(string); ok {
					d.Aliases = append(d.Aliases, s)
				}
			}
		}
	}
	return d
}

// getStrFromRec is a helper for glossaryRecordToDetail that works with the interface type.
func getStrFromRec(rec interface{ Get(string) (any, bool) }, key string) string {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}
