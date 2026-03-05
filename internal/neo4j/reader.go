package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Reader defines read-only Neo4j queries for the API layer.
type Reader interface {
	ListPrograms(ctx context.Context, filter Filter, page, pageSize int) (*PagedResponse, error)
	GetProgram(ctx context.Context, programID string) (*ProgramDetail, error)
	GetCallChain(ctx context.Context, programID, direction string, depth int) ([]CallChainNode, error)
	GetDataItems(ctx context.Context, programID string) ([]DataItemInfo, error)
	GetImpactAnalysis(ctx context.Context, programID string) (*ImpactResult, error)
	ListCopybooks(ctx context.Context, filter Filter, page, pageSize int) (*PagedResponse, error)
	GetCopybookUsage(ctx context.Context, name string) (*CopybookUsage, error)
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)
	SearchFullText(ctx context.Context, query string, limit int) ([]SearchResult, error)
	ListBusinessDomains(ctx context.Context) ([]BusinessDomainSummary, error)
	GetBusinessDomain(ctx context.Context, name string) (*BusinessDomainDetail, error)
}

// Ensure Client implements Reader.
var _ Reader = (*Client)(nil)

func (c *Client) ListPrograms(ctx context.Context, filter Filter, page, pageSize int) (*PagedResponse, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	skip := (page - 1) * pageSize

	// Count query
	countCypher := "MATCH (p:Program) RETURN count(p) AS total"
	listCypher := "MATCH (p:Program) " +
		"OPTIONAL MATCH (p)-[:CALLS]->(callee:Program) " +
		"RETURN p.programId AS programId, p.filePath AS filePath, p.language AS language, " +
		"p.deadCode AS deadCode, count(callee) AS callCount " +
		"ORDER BY p.programId SKIP $skip LIMIT $limit"

	if filter.Search != "" {
		countCypher = "MATCH (p:Program) WHERE p.programId CONTAINS $search RETURN count(p) AS total"
		listCypher = "MATCH (p:Program) WHERE p.programId CONTAINS $search " +
			"OPTIONAL MATCH (p)-[:CALLS]->(callee:Program) " +
			"RETURN p.programId AS programId, p.filePath AS filePath, p.language AS language, " +
			"p.deadCode AS deadCode, count(callee) AS callCount " +
			"ORDER BY p.programId SKIP $skip LIMIT $limit"
	}

	params := map[string]any{"skip": int64(skip), "limit": int64(pageSize), "search": filter.Search}

	total, err := c.runCountQuery(ctx, session, countCypher, params)
	if err != nil {
		return nil, err
	}

	result, err := session.Run(ctx, listCypher, params)
	if err != nil {
		return nil, fmt.Errorf("listing programs: %w", err)
	}

	var programs []ProgramSummary
	for result.Next(ctx) {
		rec := result.Record()
		p := ProgramSummary{
			ProgramID: getStr(rec, "programId"),
			FilePath:  getStr(rec, "filePath"),
			Language:  getStr(rec, "language"),
			CallCount: int(getInt64(rec, "callCount")),
			DeadCode:  getBool(rec, "deadCode"),
		}
		programs = append(programs, p)
	}

	return &PagedResponse{Data: programs, Total: total, Page: page, PageSize: pageSize}, nil
}

func (c *Client) GetProgram(ctx context.Context, programID string) (*ProgramDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"id": programID}

	// Check program exists
	res, err := session.Run(ctx,
		"MATCH (p:Program {programId: $id}) RETURN p.filePath AS filePath, p.language AS language, "+
			"p.deadCode AS deadCode, p.riskScore AS riskScore, p.riskType AS riskType",
		params)
	if err != nil {
		return nil, fmt.Errorf("getting program: %w", err)
	}
	if !res.Next(ctx) {
		return nil, nil
	}

	rec := res.Record()
	detail := &ProgramDetail{
		ProgramID: programID,
		FilePath:  getStr(rec, "filePath"),
		Language:  getStr(rec, "language"),
		DeadCode:  getBool(rec, "deadCode"),
		RiskScore: getFloat64(rec, "riskScore"),
		RiskType:  getStr(rec, "riskType"),
	}

	// Callers
	detail.Callers, _ = c.queryStringList(ctx, session,
		"MATCH (caller:Program)-[:CALLS]->(p:Program {programId: $id}) RETURN caller.programId AS val", params)

	// Callees
	detail.Callees, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:CALLS]->(callee:Program) RETURN callee.programId AS val", params)

	// Copybooks
	detail.Copybooks, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:INCLUDES]->(cb:Copybook) RETURN cb.name AS val", params)

	// Paragraphs
	paraRes, err := session.Run(ctx,
		"MATCH (para:Paragraph)-[:BELONGS_TO]->(p:Program {programId: $id}) "+
			"RETURN para.name AS name, para.description AS description, para.category AS category", params)
	if err == nil {
		for paraRes.Next(ctx) {
			r := paraRes.Record()
			detail.Paragraphs = append(detail.Paragraphs, ParagraphInfo{
				Name:        getStr(r, "name"),
				Description: getStr(r, "description"),
				Category:    getStr(r, "category"),
			})
		}
	}

	// Sections
	detail.Sections, _ = c.queryStringList(ctx, session,
		"MATCH (s:Section)-[:BELONGS_TO]->(p:Program {programId: $id}) RETURN s.name AS val", params)

	// File defs
	detail.FileDefs, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:READS|WRITES]->(f:File) RETURN DISTINCT f.name AS val", params)

	// Domains
	detail.Domains, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:BELONGS_TO]->(d:BusinessDomain) RETURN d.name AS val", params)

	return detail, nil
}

func (c *Client) GetCallChain(ctx context.Context, programID, direction string, depth int) ([]CallChainNode, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	if depth < 1 {
		depth = 3
	}
	if depth > 10 {
		depth = 10
	}

	var cypher string
	if direction == "upstream" {
		cypher = fmt.Sprintf(
			"MATCH path = (caller:Program)-[:CALLS*1..%d]->(p:Program {programId: $id}) "+
				"UNWIND nodes(path) AS n "+
				"WITH DISTINCT n, length(shortestPath((n)-[:CALLS*]->(p))) AS d "+
				"MATCH (p:Program {programId: $id}) "+
				"RETURN n.programId AS programId, d AS depth ORDER BY d",
			depth)
	} else {
		cypher = fmt.Sprintf(
			"MATCH path = (p:Program {programId: $id})-[:CALLS*1..%d]->(callee:Program) "+
				"UNWIND nodes(path) AS n "+
				"WITH DISTINCT n, length(shortestPath((p)-[:CALLS*]->(n))) AS d "+
				"MATCH (p:Program {programId: $id}) "+
				"RETURN n.programId AS programId, d AS depth ORDER BY d",
			depth)
	}

	result, err := session.Run(ctx, cypher, map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("call chain query: %w", err)
	}

	var nodes []CallChainNode
	for result.Next(ctx) {
		rec := result.Record()
		pid := getStr(rec, "programId")
		if pid == programID {
			continue
		}
		nodes = append(nodes, CallChainNode{
			ProgramID: pid,
			Depth:     int(getInt64(rec, "depth")),
		})
	}

	return nodes, nil
}

func (c *Client) GetDataItems(ctx context.Context, programID string) ([]DataItemInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (d:DataItem {programId: $id}) RETURN d.name AS name, d.level AS level, d.fqn AS fqn, d.picture AS picture ORDER BY d.level, d.name",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("data items query: %w", err)
	}

	var items []DataItemInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DataItemInfo{
			Name:    getStr(rec, "name"),
			Level:   int(getInt64(rec, "level")),
			FQN:     getStr(rec, "fqn"),
			Picture: getStr(rec, "picture"),
		})
	}

	return items, nil
}

func (c *Client) GetImpactAnalysis(ctx context.Context, programID string) (*ImpactResult, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"id": programID}
	impact := &ImpactResult{ProgramID: programID}

	// Downstream programs (programs this program calls, transitively)
	impact.DownstreamPrograms, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:CALLS*1..5]->(d:Program) RETURN DISTINCT d.programId AS val", params)

	// Upstream programs (programs that call this one, transitively)
	impact.UpstreamPrograms, _ = c.queryStringList(ctx, session,
		"MATCH (u:Program)-[:CALLS*1..5]->(p:Program {programId: $id}) RETURN DISTINCT u.programId AS val", params)

	// Shared copybooks
	impact.SharedCopybooks, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:INCLUDES]->(cb:Copybook)<-[:INCLUDES]-(other:Program) "+
			"WHERE other.programId <> $id RETURN DISTINCT cb.name AS val", params)

	// Shared files
	impact.SharedFiles, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program {programId: $id})-[:READS|WRITES]->(f:File)<-[:READS|WRITES]-(other:Program) "+
			"WHERE other.programId <> $id RETURN DISTINCT f.name AS val", params)

	impact.TotalAffected = len(impact.DownstreamPrograms) + len(impact.UpstreamPrograms)

	return impact, nil
}

func (c *Client) ListCopybooks(ctx context.Context, filter Filter, page, pageSize int) (*PagedResponse, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	skip := (page - 1) * pageSize
	params := map[string]any{"skip": int64(skip), "limit": int64(pageSize), "search": filter.Search}

	countCypher := "MATCH (cb:Copybook) RETURN count(cb) AS total"
	listCypher := "MATCH (cb:Copybook) " +
		"OPTIONAL MATCH (p:Program)-[:INCLUDES]->(cb) " +
		"RETURN cb.name AS name, count(p) AS usageCount " +
		"ORDER BY cb.name SKIP $skip LIMIT $limit"

	if filter.Search != "" {
		countCypher = "MATCH (cb:Copybook) WHERE cb.name CONTAINS $search RETURN count(cb) AS total"
		listCypher = "MATCH (cb:Copybook) WHERE cb.name CONTAINS $search " +
			"OPTIONAL MATCH (p:Program)-[:INCLUDES]->(cb) " +
			"RETURN cb.name AS name, count(p) AS usageCount " +
			"ORDER BY cb.name SKIP $skip LIMIT $limit"
	}

	total, err := c.runCountQuery(ctx, session, countCypher, params)
	if err != nil {
		return nil, err
	}

	result, err := session.Run(ctx, listCypher, params)
	if err != nil {
		return nil, fmt.Errorf("listing copybooks: %w", err)
	}

	var copybooks []CopybookSummary
	for result.Next(ctx) {
		rec := result.Record()
		copybooks = append(copybooks, CopybookSummary{
			Name:       getStr(rec, "name"),
			UsageCount: int(getInt64(rec, "usageCount")),
		})
	}

	return &PagedResponse{Data: copybooks, Total: total, Page: page, PageSize: pageSize}, nil
}

func (c *Client) GetCopybookUsage(ctx context.Context, name string) (*CopybookUsage, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	programs, err := c.queryStringList(ctx, session,
		"MATCH (p:Program)-[:INCLUDES]->(cb:Copybook {name: $name}) RETURN p.programId AS val",
		map[string]any{"name": name})
	if err != nil {
		return nil, err
	}

	return &CopybookUsage{Name: name, Programs: programs}, nil
}

func (c *Client) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	stats := &DashboardStats{}

	queries := []struct {
		cypher string
		dest   *int
	}{
		{"MATCH (p:Program) RETURN count(p) AS c", &stats.ProgramCount},
		{"MATCH (cb:Copybook) RETURN count(cb) AS c", &stats.CopybookCount},
		{"MATCH (p:Paragraph) RETURN count(p) AS c", &stats.ParagraphCount},
		{"MATCH (d:DataItem) RETURN count(d) AS c", &stats.DataItemCount},
		{"MATCH ()-[r]->() RETURN count(r) AS c", &stats.RelationshipCount},
		{"MATCH (p:Program) WHERE NOT ()-[:CALLS]->(p) RETURN count(p) AS c", &stats.OrphanCount},
		{"MATCH (d:BusinessDomain) RETURN count(d) AS c", &stats.DomainCount},
	}

	for _, q := range queries {
		result, err := session.Run(ctx, q.cypher, nil)
		if err != nil {
			continue
		}
		if result.Next(ctx) {
			*q.dest = int(getInt64(result.Record(), "c"))
		}
	}

	return stats, nil
}

func (c *Client) SearchFullText(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	if limit <= 0 {
		limit = 20
	}

	var results []SearchResult

	// Search programs
	programRes, err := session.Run(ctx,
		"CALL db.index.fulltext.queryNodes('search_programs', $query) YIELD node, score "+
			"RETURN node.programId AS name, score, node.filePath AS path, 'Program' AS label "+
			"LIMIT $limit",
		map[string]any{"query": query, "limit": int64(limit)})
	if err == nil {
		for programRes.Next(ctx) {
			rec := programRes.Record()
			results = append(results, SearchResult{
				Label: getStr(rec, "label"),
				Name:  getStr(rec, "name"),
				Score: getFloat64(rec, "score"),
				Path:  getStr(rec, "path"),
			})
		}
	}

	// Search paragraphs
	paraRes, err := session.Run(ctx,
		"CALL db.index.fulltext.queryNodes('search_paragraphs', $query) YIELD node, score "+
			"RETURN node.name AS name, score, 'Paragraph' AS label "+
			"LIMIT $limit",
		map[string]any{"query": query, "limit": int64(limit)})
	if err == nil {
		for paraRes.Next(ctx) {
			rec := paraRes.Record()
			results = append(results, SearchResult{
				Label: getStr(rec, "label"),
				Name:  getStr(rec, "name"),
				Score: getFloat64(rec, "score"),
			})
		}
	}

	return results, nil
}

func (c *Client) ListBusinessDomains(ctx context.Context) ([]BusinessDomainSummary, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (d:BusinessDomain) "+
			"OPTIONAL MATCH (p:Program)-[:BELONGS_TO]->(d) "+
			"RETURN d.name AS name, d.description AS description, count(p) AS programCount "+
			"ORDER BY d.name", nil)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}

	var domains []BusinessDomainSummary
	for result.Next(ctx) {
		rec := result.Record()
		domains = append(domains, BusinessDomainSummary{
			Name:         getStr(rec, "name"),
			Description:  getStr(rec, "description"),
			ProgramCount: int(getInt64(rec, "programCount")),
		})
	}

	return domains, nil
}

func (c *Client) GetBusinessDomain(ctx context.Context, name string) (*BusinessDomainDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"name": name}

	// Check domain exists
	res, err := session.Run(ctx,
		"MATCH (d:BusinessDomain {name: $name}) RETURN d.description AS description", params)
	if err != nil {
		return nil, fmt.Errorf("getting domain: %w", err)
	}
	if !res.Next(ctx) {
		return nil, nil
	}

	detail := &BusinessDomainDetail{
		Name:        name,
		Description: getStr(res.Record(), "description"),
	}

	detail.Programs, _ = c.queryStringList(ctx, session,
		"MATCH (p:Program)-[:BELONGS_TO]->(d:BusinessDomain {name: $name}) RETURN p.programId AS val", params)

	return detail, nil
}

// Helper methods

func (c *Client) runCountQuery(ctx context.Context, session neo4j.SessionWithContext, cypher string, params map[string]any) (int, error) {
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return 0, fmt.Errorf("count query: %w", err)
	}
	if result.Next(ctx) {
		return int(getInt64(result.Record(), "total")), nil
	}
	return 0, nil
}

func (c *Client) queryStringList(ctx context.Context, session neo4j.SessionWithContext, cypher string, params map[string]any) ([]string, error) {
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var vals []string
	for result.Next(ctx) {
		if val, ok := result.Record().Get("val"); ok && val != nil {
			if s, ok := val.(string); ok {
				vals = append(vals, s)
			}
		}
	}
	return vals, nil
}

func getStr(rec *neo4j.Record, key string) string {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func getInt64(rec *neo4j.Record, key string) int64 {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return 0
}

func getFloat64(rec *neo4j.Record, key string) float64 {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	return 0
}

func getBool(rec *neo4j.Record, key string) bool {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}
