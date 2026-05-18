package neo4j

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
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
	GetProgramConditions(ctx context.Context, programID string) ([]ConditionInfo, error)
	GetProgramParameters(ctx context.Context, programID string) ([]ParameterInfo, error)
	GetProgramConditionalLogic(ctx context.Context, programID string) ([]ConditionalLogicInfo, error)
	GetProgramErrorHandlers(ctx context.Context, programID string) ([]ErrorHandlerInfo, error)
	GetProgramExternalInterfaces(ctx context.Context, programID string) ([]ExternalInterfaceInfo, error)
	ListBridgePrograms(ctx context.Context) ([]BridgeProgramInfo, error)
	ListCopybookRisks(ctx context.Context) ([]CopybookRiskInfo, error)
	ListModernizationCandidates(ctx context.Context) ([]ModernizationCandidateInfo, error)
	ListRiskPrograms(ctx context.Context, minScore float64) ([]RiskProgramInfo, error)
	ListVolumeEstimates(ctx context.Context) ([]VolumeEstimateInfo, error)
	GetProgramSQL(ctx context.Context, programID string) ([]SQLStatementInfo, error)
	GetProgramCICS(ctx context.Context, programID string) ([]CICSTransactionInfo, error)
	GetParagraphFlow(ctx context.Context, programID string) ([]ParagraphFlowInfo, error)
	GetDataFlow(ctx context.Context, programID string) ([]DataFlowInfo, error)
	GetDataHierarchy(ctx context.Context, programID string) ([]DataHierarchyInfo, error)
	// Phase 1: Dead paragraph detection
	GetDeadParagraphs(ctx context.Context, programID string) ([]DeadParagraphInfo, error)
	GetDeadCodeSummary(ctx context.Context) ([]DeadCodeSummaryInfo, error)
	// Phase 2: JCL analysis
	ListJCLJobs(ctx context.Context) ([]JCLJobInfo, error)
	GetJCLJob(ctx context.Context, jobName string) (*JCLJobDetail, error)
	GetProgramJCL(ctx context.Context, programID string) (*ProgramJCLInfo, error)
	GetDatasetUsage(ctx context.Context, dsname string) ([]DatasetUsageInfo, error)
	// Phase 3: DB table access
	ListDBTables(ctx context.Context) ([]DBTableInfo, error)
	GetTableUsage(ctx context.Context, tableName string) (*TableUsageInfo, error)
	GetProgramTableAccess(ctx context.Context, programID string) (*ProgramTableAccessInfo, error)
	// Phase 4: Cross-program data flow
	GetCrossProgramDataFlow(ctx context.Context, programID string) ([]CrossProgramFlowInfo, error)
	TraceFieldImpact(ctx context.Context, programID, fieldName string) ([]FieldImpactInfo, error)
	GetSharedDataChannels(ctx context.Context) ([]SharedDataChannelInfo, error)
	// Phase 5: Validation report
	GetValidationReport(ctx context.Context) (*ValidationResult, error)
	// Copybook structure
	GetCopybookStructure(ctx context.Context, copybookName string) ([]DataItemInfo, error)
	// Source code retrieval
	GetProgramSource(ctx context.Context, programID string) (*ProgramSourceInfo, error)
	// Migration dependency ordering
	GetMigrationSequence(ctx context.Context) ([]MigrationStep, error)
	// File accessor drill-down
	GetFileAccessors(ctx context.Context, fileName string) (*FileAccessInfo, error)
	// Effort estimation
	GetEffortEstimates(ctx context.Context) ([]EffortEstimate, error)
	// IDMS support
	GetIDMSRecords(ctx context.Context, programID string) ([]IDMSRecordInfo, error)
	GetIDMSSchema(ctx context.Context, programID string) (*IDMSSchemaInfo, error)
	GetIDMSAreas(ctx context.Context, programID string) ([]IDMSAreaInfo, error)
	GetIDMSImpact(ctx context.Context, recordName string) (*IDMSImpactInfo, error)
	// External DB gap analysis
	ListExternalDBTables(ctx context.Context) ([]ExternalDBTableInfo, error)
	GetExternalDBMapping(ctx context.Context, tableName string) (*ExternalDBMappingInfo, error)
	GetCobolToExternalMappings(ctx context.Context, cobolTable string) ([]ExternalDBMappingInfo, error)
	GetGapAnalysis(ctx context.Context) ([]GapInfo, error)
	GetDataFlowPaths(ctx context.Context, tableName string) ([]DataFlowPathInfo, error)
	// Target stack gap analysis
	ListTargetRepos(ctx context.Context) ([]TargetRepoInfo, error)
	ListTargetServices(ctx context.Context, repoURL string) ([]TargetServiceInfo, error)
	GetTargetService(ctx context.Context, serviceID string) (*TargetServiceDetail, error)
	ListBusinessGaps(ctx context.Context, gapType, severity, category string) ([]BusinessGapInfo, error)
	ListBusinessRequirements(ctx context.Context, priority string) ([]BusinessRequirementInfo, error)
	GetGapCoverageSummary(ctx context.Context) (*GapCoverageSummary, error)
	GetTargetStackDashboard(ctx context.Context) (map[string]any, error)
}

// Ensure Client implements Reader.
var _ Reader = (*Client)(nil)

func (c *Client) ListPrograms(ctx context.Context, filter Filter, page, pageSize int) (*PagedResponse, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	skip := (page - 1) * pageSize
	cbFilter := codebaseOrFilter(filter.Codebase, c.codebase)

	// Count query
	countCypher := "MATCH (p:Program)" + codebaseWhere("p", cbFilter) + " RETURN count(p) AS total"
	listCypher := "MATCH (p:Program)" + codebaseWhere("p", cbFilter) + " " +
		"OPTIONAL MATCH (p)-[:CALLS]->(callee:Program) " +
		"RETURN p.programId AS programId, p.filePath AS filePath, p.language AS language, " +
		"p.deadCode AS deadCode, p.executionMode AS executionMode, p.lineCount AS lineCount, count(callee) AS callCount " +
		"ORDER BY p.programId SKIP $skip LIMIT $limit"

	if filter.Search != "" {
		countCypher = "MATCH (p:Program) WHERE p.programId CONTAINS $search" + codebaseWhereAnd("p", cbFilter) + " RETURN count(p) AS total"
		listCypher = "MATCH (p:Program) WHERE p.programId CONTAINS $search" + codebaseWhereAnd("p", cbFilter) + " " +
			"OPTIONAL MATCH (p)-[:CALLS]->(callee:Program) " +
			"RETURN p.programId AS programId, p.filePath AS filePath, p.language AS language, " +
			"p.deadCode AS deadCode, p.executionMode AS executionMode, p.lineCount AS lineCount, count(callee) AS callCount " +
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
			ProgramID:     getStr(rec, "programId"),
			FilePath:      getStr(rec, "filePath"),
			Language:      getStr(rec, "language"),
			CallCount:     int(getInt64(rec, "callCount")),
			DeadCode:      getBool(rec, "deadCode"),
			ExecutionMode: getStr(rec, "executionMode"),
			LineCount:     int(getInt64(rec, "lineCount")),
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
			"p.lineCount AS lineCount, p.executionMode AS executionMode, "+
			"p.deadCode AS deadCode, p.deadCodeReason AS deadCodeReason, "+
			"p.riskScore AS riskScore, p.riskType AS riskType, p.riskDetails AS riskDetails",
		params)
	if err != nil {
		return nil, fmt.Errorf("getting program: %w", err)
	}
	if !res.Next(ctx) {
		return nil, nil
	}

	rec := res.Record()
	detail := &ProgramDetail{
		ProgramID:      programID,
		FilePath:       getStr(rec, "filePath"),
		Language:       getStr(rec, "language"),
		LineCount:      int(getInt64(rec, "lineCount")),
		ExecutionMode:  getStr(rec, "executionMode"),
		DeadCode:       getBool(rec, "deadCode"),
		DeadCodeReason: getStr(rec, "deadCodeReason"),
		RiskScore:      getFloat64(rec, "riskScore"),
		RiskType:       getStr(rec, "riskType"),
		RiskDetails:    getStr(rec, "riskDetails"),
	}

	// Callers
	detail.Callers, _ = c.queryCallInfoList(ctx, session,
		"MATCH (caller:Program)-[r:CALLS]->(p:Program {programId: $id}) "+
			"RETURN caller.programId AS programId, r.isDynamic AS isDynamic, r.resolvedFrom AS resolvedFrom, r.fromParagraph AS fromParagraph", params)

	// Callees
	detail.Callees, _ = c.queryCallInfoList(ctx, session,
		"MATCH (p:Program {programId: $id})-[r:CALLS]->(callee:Program) "+
			"RETURN callee.programId AS programId, r.isDynamic AS isDynamic, r.resolvedFrom AS resolvedFrom, r.fromParagraph AS fromParagraph", params)

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
	fileRes, err := session.Run(ctx,
		"MATCH (p:Program {programId: $id})-[r:READS|WRITES]->(f:File) "+
			"RETURN DISTINCT f.name AS name, type(r) AS accessType, f.organization AS organization, f.vsamType AS vsamType, f.dataStoreType AS dataStoreType", params)
	if err == nil {
		for fileRes.Next(ctx) {
			r := fileRes.Record()
			detail.FileDefs = append(detail.FileDefs, FileDefInfo{
				Name:          getStr(r, "name"),
				AccessType:    getStr(r, "accessType"),
				Organization:  getStr(r, "organization"),
				VSAMType:      getStr(r, "vsamType"),
				DataStoreType: getStr(r, "dataStoreType"),
			})
		}
	}

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
				"WITH p, nodes(path) AS ns, length(path) AS pathLen "+
				"UNWIND range(0, pathLen - 1) AS idx "+
				"WITH DISTINCT ns[idx] AS n, pathLen - idx AS d "+
				"RETURN n.programId AS programId, min(d) AS depth "+
				"ORDER BY depth",
			depth)
	} else {
		cypher = fmt.Sprintf(
			"MATCH path = (p:Program {programId: $id})-[:CALLS*1..%d]->(callee:Program) "+
				"WITH p, nodes(path) AS ns, length(path) AS pathLen "+
				"UNWIND range(1, pathLen) AS idx "+
				"WITH DISTINCT ns[idx] AS n, idx AS d "+
				"RETURN n.programId AS programId, min(d) AS depth "+
				"ORDER BY depth",
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
		"MATCH (d:DataItem {programId: $id}) RETURN d.name AS name, d.level AS level, d.fqn AS fqn, d.picture AS picture, d.usage AS usage, d.copybook AS copybook ORDER BY d.level, d.name",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("data items query: %w", err)
	}

	var items []DataItemInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DataItemInfo{
			Name:     getStr(rec, "name"),
			Level:    int(getInt64(rec, "level")),
			FQN:      getStr(rec, "fqn"),
			Picture:  getStr(rec, "picture"),
			Usage:    getStr(rec, "usage"),
			Copybook: getStr(rec, "copybook"),
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
	cbFilter := codebaseOrFilter(filter.Codebase, c.codebase)
	params := map[string]any{"skip": int64(skip), "limit": int64(pageSize), "search": filter.Search}

	countCypher := "MATCH (cb:Copybook) WHERE 1=1" + codebasesWhereAnd("cb", cbFilter) + " RETURN count(cb) AS total"
	listCypher := "MATCH (cb:Copybook) WHERE 1=1" + codebasesWhereAnd("cb", cbFilter) + " " +
		"OPTIONAL MATCH (p:Program)-[:INCLUDES]->(cb) " +
		"RETURN cb.name AS name, count(p) AS usageCount " +
		"ORDER BY cb.name SKIP $skip LIMIT $limit"

	if filter.Search != "" {
		countCypher = "MATCH (cb:Copybook) WHERE cb.name CONTAINS $search" + codebasesWhereAnd("cb", cbFilter) + " RETURN count(cb) AS total"
		listCypher = "MATCH (cb:Copybook) WHERE cb.name CONTAINS $search" + codebasesWhereAnd("cb", cbFilter) + " " +
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
		{"MATCH (p:Program)" + codebaseWhere("p", c.codebase) + " RETURN count(p) AS c", &stats.ProgramCount},
		{"MATCH (cb:Copybook) WHERE 1=1" + codebasesWhereAnd("cb", c.codebase) + " RETURN count(cb) AS c", &stats.CopybookCount},
		{"MATCH (p:Paragraph)" + codebaseWhere("p", c.codebase) + " RETURN count(p) AS c", &stats.ParagraphCount},
		{"MATCH (s:Section)" + codebaseWhere("s", c.codebase) + " RETURN count(s) AS c", &stats.SectionCount},
		{"MATCH (d:DataItem)" + codebaseWhere("d", c.codebase) + " RETURN count(d) AS c", &stats.DataItemCount},
		{"MATCH (f:File) WHERE 1=1" + codebasesWhereAnd("f", c.codebase) + " RETURN count(f) AS c", &stats.FileCount},
		{"MATCH (s:SQLStatement)" + codebaseWhere("s", c.codebase) + " RETURN count(s) AS c", &stats.SQLStatementCount},
		{"MATCH (c:CICSTransaction)" + codebaseWhere("c", c.codebase) + " RETURN count(c) AS c", &stats.CICSTransactionCount},
		{"MATCH (e:ExternalInterface)" + codebaseWhere("e", c.codebase) + " RETURN count(e) AS c", &stats.ExternalInterfaceCount},
		{"MATCH (c:Condition)" + codebaseWhere("c", c.codebase) + " RETURN count(c) AS c", &stats.ConditionCount},
		{"MATCH (p:Parameter)" + codebaseWhere("p", c.codebase) + " RETURN count(p) AS c", &stats.ParameterCount},
		{"MATCH (p:Program)" + codebaseWhere("p", c.codebase) + " WITH p MATCH (p)-[r]->() RETURN count(r) AS c", &stats.RelationshipCount},
		{"MATCH (p:Program) WHERE NOT ()-[:CALLS]->(p)" + codebaseWhereAnd("p", c.codebase) + " RETURN count(p) AS c", &stats.OrphanCount},
		{"MATCH (d:BusinessDomain) WHERE 1=1" + codebasesWhereAnd("d", c.codebase) + " RETURN count(d) AS c", &stats.DomainCount},
		{"MATCH (dd:DDCard)" + codebaseWhere("dd", c.codebase) + " RETURN count(dd) AS c", &stats.DDCardCount},
		{"MATCH (t:DBTable) WHERE 1=1" + codebasesWhereAnd("t", c.codebase) + " RETURN count(t) AS c", &stats.DBTableCount},
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
		"MATCH (d:BusinessDomain) WHERE 1=1"+codebasesWhereAnd("d", c.codebase)+" "+
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

func (c *Client) GetProgramConditions(ctx context.Context, programID string) ([]ConditionInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (cond:Condition {programId: $pid}) RETURN cond.name AS name, cond.parent AS parent, cond.value AS value, cond.fqn AS fqn, cond.programId AS programId",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("conditions query: %w", err)
	}

	var items []ConditionInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, ConditionInfo{
			Name:      getStr(rec, "name"),
			Parent:    getStr(rec, "parent"),
			Value:     getStr(rec, "value"),
			FQN:       getStr(rec, "fqn"),
			ProgramID: getStr(rec, "programId"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramParameters(ctx context.Context, programID string) ([]ParameterInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Parameter {programId: $pid}) RETURN p.name AS name, p.level AS level, p.direction AS direction, p.fqn AS fqn, p.programId AS programId",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("parameters query: %w", err)
	}

	var items []ParameterInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, ParameterInfo{
			Name:      getStr(rec, "name"),
			Level:     int(getInt64(rec, "level")),
			Direction: getStr(rec, "direction"),
			FQN:       getStr(rec, "fqn"),
			ProgramID: getStr(rec, "programId"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramConditionalLogic(ctx context.Context, programID string) ([]ConditionalLogicInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Paragraph {programId: $pid}) WHERE p.conditionalLogic IS NOT NULL RETURN p.name AS name, p.conditionalLogic AS conditionalLogic",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("conditional logic query: %w", err)
	}

	var items []ConditionalLogicInfo
	for result.Next(ctx) {
		rec := result.Record()
		var logic []string
		if val, ok := rec.Get("conditionalLogic"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						logic = append(logic, s)
					}
				}
			}
		}
		items = append(items, ConditionalLogicInfo{
			Paragraph:        getStr(rec, "name"),
			ConditionalLogic: logic,
		})
	}
	return items, nil
}

func (c *Client) GetProgramErrorHandlers(ctx context.Context, programID string) ([]ErrorHandlerInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Paragraph {programId: $pid}) WHERE p.errorPattern IS NOT NULL RETURN p.name AS name, p.errorPattern AS errorPattern, p.errorDetails AS errorDetails",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("error handlers query: %w", err)
	}

	var items []ErrorHandlerInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, ErrorHandlerInfo{
			Paragraph:    getStr(rec, "name"),
			ErrorPattern: getStr(rec, "errorPattern"),
			ErrorDetails: getStr(rec, "errorDetails"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramExternalInterfaces(ctx context.Context, programID string) ([]ExternalInterfaceInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (e:ExternalInterface {programId: $pid}) RETURN e.id AS id, e.type AS type, e.details AS details, e.paragraph AS paragraph, e.programId AS programId",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("external interfaces query: %w", err)
	}

	var items []ExternalInterfaceInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, ExternalInterfaceInfo{
			ID:        getStr(rec, "id"),
			Type:      getStr(rec, "type"),
			Details:   getStr(rec, "details"),
			Paragraph: getStr(rec, "paragraph"),
			ProgramID: getStr(rec, "programId"),
		})
	}
	return items, nil
}

func (c *Client) ListBridgePrograms(ctx context.Context) ([]BridgeProgramInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.isBridge = true"+codebaseWhereAnd("p", c.codebase)+" RETURN p.programId AS programId, p.bridgeDomains AS domains, p.bridgeReason AS reason", nil)
	if err != nil {
		return nil, fmt.Errorf("bridge programs query: %w", err)
	}

	var items []BridgeProgramInfo
	for result.Next(ctx) {
		rec := result.Record()
		var domains []string
		if val, ok := rec.Get("domains"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						domains = append(domains, s)
					}
				}
			}
		}
		items = append(items, BridgeProgramInfo{
			ProgramID:    getStr(rec, "programId"),
			Domains:      domains,
			BridgeReason: getStr(rec, "reason"),
		})
	}
	return items, nil
}

func (c *Client) ListCopybookRisks(ctx context.Context) ([]CopybookRiskInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (cb:Copybook) WHERE cb.riskLevel IS NOT NULL"+codebasesWhereAnd("cb", c.codebase)+" RETURN cb.name AS name, cb.riskLevel AS riskLevel, cb.programCount AS programCount, cb.riskReason AS riskReason ORDER BY cb.riskLevel", nil)
	if err != nil {
		return nil, fmt.Errorf("copybook risks query: %w", err)
	}

	var items []CopybookRiskInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, CopybookRiskInfo{
			Name:         getStr(rec, "name"),
			RiskLevel:    getStr(rec, "riskLevel"),
			ProgramCount: int(getInt64(rec, "programCount")),
			RiskReason:   getStr(rec, "riskReason"),
		})
	}
	return items, nil
}

func (c *Client) ListModernizationCandidates(ctx context.Context) ([]ModernizationCandidateInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.modernizationScore IS NOT NULL AND p.filePath IS NOT NULL"+codebaseWhereAnd("p", c.codebase)+" RETURN p.programId AS programId, p.modernizationScore AS score, p.modernizationReason AS reason, p.modernizationApproach AS approach ORDER BY p.modernizationScore DESC", nil)
	if err != nil {
		return nil, fmt.Errorf("modernization candidates query: %w", err)
	}

	var items []ModernizationCandidateInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, ModernizationCandidateInfo{
			ProgramID: getStr(rec, "programId"),
			Score:     getFloat64(rec, "score"),
			Reason:    getStr(rec, "reason"),
			Approach:  getStr(rec, "approach"),
		})
	}
	return items, nil
}

func (c *Client) ListRiskPrograms(ctx context.Context, minScore float64) ([]RiskProgramInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.riskScore >= $min AND p.filePath IS NOT NULL"+codebaseWhereAnd("p", c.codebase)+" RETURN p.programId AS programId, p.riskScore AS riskScore, p.riskType AS riskType, p.riskDetails AS riskDetails ORDER BY p.riskScore DESC",
		map[string]any{"min": minScore})
	if err != nil {
		return nil, fmt.Errorf("risk programs query: %w", err)
	}

	var items []RiskProgramInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, RiskProgramInfo{
			ProgramID:   getStr(rec, "programId"),
			RiskScore:   getFloat64(rec, "riskScore"),
			RiskType:    getStr(rec, "riskType"),
			RiskDetails: getStr(rec, "riskDetails"),
		})
	}
	return items, nil
}

func (c *Client) ListVolumeEstimates(ctx context.Context) ([]VolumeEstimateInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.volumeEstimate IS NOT NULL"+codebaseWhereAnd("p", c.codebase)+" RETURN p.programId AS programId, p.volumeEstimate AS estimate, p.volumeReason AS reason ORDER BY p.programId", nil)
	if err != nil {
		return nil, fmt.Errorf("volume estimates query: %w", err)
	}

	var items []VolumeEstimateInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, VolumeEstimateInfo{
			ProgramID:    getStr(rec, "programId"),
			Estimate:     getStr(rec, "estimate"),
			VolumeReason: getStr(rec, "reason"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramSQL(ctx context.Context, programID string) ([]SQLStatementInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (s:SQLStatement {programId: $id}) RETURN s.id AS id, s.text AS text, s.type AS type, s.targetTable AS targetTable ORDER BY s.type, s.id",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("sql statements query: %w", err)
	}

	var items []SQLStatementInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, SQLStatementInfo{
			ID:          getStr(rec, "id"),
			Text:        getStr(rec, "text"),
			Type:        getStr(rec, "type"),
			TargetTable: getStr(rec, "targetTable"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramCICS(ctx context.Context, programID string) ([]CICSTransactionInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (t:CICSTransaction {programId: $id}) RETURN t.id AS id, t.command AS command ORDER BY t.command",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("cics transactions query: %w", err)
	}

	var items []CICSTransactionInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, CICSTransactionInfo{
			ID:      getStr(rec, "id"),
			Command: getStr(rec, "command"),
		})
	}
	return items, nil
}

func (c *Client) GetParagraphFlow(ctx context.Context, programID string) ([]ParagraphFlowInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	var items []ParagraphFlowInfo

	// PERFORMS relationships
	performsRes, err := session.Run(ctx,
		"MATCH (from:Paragraph {programId: $id})-[r:PERFORMS]->(to:Paragraph {programId: $id}) "+
			"RETURN from.name AS fromParagraph, to.name AS toParagraph, r.isLoop AS isLoop, r.condition AS condition",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("paragraph flow query: %w", err)
	}
	for performsRes.Next(ctx) {
		rec := performsRes.Record()
		items = append(items, ParagraphFlowInfo{
			FromParagraph: getStr(rec, "fromParagraph"),
			ToParagraph:   getStr(rec, "toParagraph"),
			Type:          "PERFORMS",
			IsLoop:        getBool(rec, "isLoop"),
			Condition:     getStr(rec, "condition"),
		})
	}

	// PERFORMS_THRU relationships
	thruRes, err := session.Run(ctx,
		"MATCH (from:Paragraph {programId: $id})-[r:PERFORMS_THRU]->(to:Paragraph {programId: $id}) "+
			"RETURN from.name AS fromParagraph, to.name AS toParagraph",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("paragraph thru query: %w", err)
	}
	for thruRes.Next(ctx) {
		rec := thruRes.Record()
		items = append(items, ParagraphFlowInfo{
			FromParagraph: getStr(rec, "fromParagraph"),
			ToParagraph:   getStr(rec, "toParagraph"),
			Type:          "PERFORMS_THRU",
		})
	}

	return items, nil
}

func (c *Client) GetDataFlow(ctx context.Context, programID string) ([]DataFlowInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (from:DataItem {programId: $id})-[r:MOVES_TO]->(to:DataItem {programId: $id}) "+
			"RETURN from.name AS fromItem, to.name AS toItem, r.context AS context",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("data flow query: %w", err)
	}

	var items []DataFlowInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DataFlowInfo{
			FromItem: getStr(rec, "fromItem"),
			ToItem:   getStr(rec, "toItem"),
			Context:  getStr(rec, "context"),
		})
	}
	return items, nil
}

func (c *Client) GetDataHierarchy(ctx context.Context, programID string) ([]DataHierarchyInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	var items []DataHierarchyInfo

	// CHILD_OF relationships
	childRes, err := session.Run(ctx,
		"MATCH (child:DataItem {programId: $id})-[:CHILD_OF]->(parent:DataItem {programId: $id}) "+
			"RETURN child.name AS name, child.level AS level, parent.name AS parent, parent.level AS parentLevel",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("data hierarchy query: %w", err)
	}
	for childRes.Next(ctx) {
		rec := childRes.Record()
		items = append(items, DataHierarchyInfo{
			Name:        getStr(rec, "name"),
			Level:       int(getInt64(rec, "level")),
			Parent:      getStr(rec, "parent"),
			ParentLevel: int(getInt64(rec, "parentLevel")),
			Relation:    "CHILD_OF",
		})
	}

	// REDEFINES relationships
	redefRes, err := session.Run(ctx,
		"MATCH (item:DataItem {programId: $id})-[:REDEFINES]->(target:DataItem {programId: $id}) "+
			"RETURN item.name AS name, target.name AS redefines",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("redefines query: %w", err)
	}
	for redefRes.Next(ctx) {
		rec := redefRes.Record()
		items = append(items, DataHierarchyInfo{
			Name:      getStr(rec, "name"),
			Redefines: getStr(rec, "redefines"),
			Relation:  "REDEFINES",
		})
	}

	return items, nil
}

func (c *Client) GetValidationReport(ctx context.Context) (*ValidationResult, error) {
	w := NewBatchWriter(c, 500, "default", zap.NewNop())
	return w.RunValidation(ctx)
}

func (c *Client) GetCopybookStructure(ctx context.Context, copybookName string) ([]DataItemInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (d:DataItem)-[:DEFINED_IN]->(c:Copybook {name: $name}) "+
			"RETURN d.name AS name, d.level AS level, d.fqn AS fqn, "+
			"d.picture AS picture, d.usage AS usage, d.copybook AS copybook "+
			"ORDER BY d.level, d.name",
		map[string]any{"name": copybookName})
	if err != nil {
		return nil, fmt.Errorf("copybook structure query: %w", err)
	}

	var items []DataItemInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, DataItemInfo{
			Name:     getStr(rec, "name"),
			Level:    int(getInt64(rec, "level")),
			FQN:      getStr(rec, "fqn"),
			Picture:  getStr(rec, "picture"),
			Usage:    getStr(rec, "usage"),
			Copybook: getStr(rec, "copybook"),
		})
	}
	return items, nil
}

func (c *Client) GetProgramSource(ctx context.Context, programID string) (*ProgramSourceInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program {programId: $id}) RETURN p.filePath AS filePath",
		map[string]any{"id": programID})
	if err != nil {
		return nil, fmt.Errorf("querying program source: %w", err)
	}
	if !result.Next(ctx) {
		return nil, nil
	}

	filePath := getStr(result.Record(), "filePath")
	if filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading source file %s: %w", filePath, err)
	}

	source := string(data)
	lineCount := strings.Count(source, "\n") + 1

	return &ProgramSourceInfo{
		ProgramID: programID,
		FilePath:  filePath,
		Source:    source,
		LineCount: lineCount,
	}, nil
}

func (c *Client) GetMigrationSequence(ctx context.Context) ([]MigrationStep, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	// Get all modernization candidates with their call targets (also candidates)
	result, err := session.Run(ctx,
		"MATCH (p:Program) WHERE p.modernizationScore IS NOT NULL"+codebaseWhereAnd("p", c.codebase)+" "+
			"OPTIONAL MATCH (p)-[:CALLS]->(callee:Program) WHERE callee.modernizationScore IS NOT NULL "+
			"OPTIONAL MATCH (p)-[:BELONGS_TO]->(d:BusinessDomain) "+
			"RETURN p.programId AS programId, p.modernizationScore AS score, "+
			"p.modernizationApproach AS approach, "+
			"collect(DISTINCT callee.programId) AS blockedBy, "+
			"head(collect(DISTINCT d.name)) AS domain",
		nil)
	if err != nil {
		return nil, fmt.Errorf("migration sequence query: %w", err)
	}

	type candidate struct {
		programID string
		score     float64
		approach  string
		domain    string
		blockedBy []string
	}

	candidateSet := make(map[string]bool)
	var candidates []candidate

	for result.Next(ctx) {
		rec := result.Record()
		pid := getStr(rec, "programId")
		candidateSet[pid] = true

		var blockedBy []string
		if val, ok := rec.Get("blockedBy"); ok && val != nil {
			if arr, ok := val.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						blockedBy = append(blockedBy, s)
					}
				}
			}
		}

		candidates = append(candidates, candidate{
			programID: pid,
			score:     getFloat64(rec, "score"),
			approach:  getStr(rec, "approach"),
			domain:    getStr(rec, "domain"),
			blockedBy: blockedBy,
		})
	}

	// Filter blockedBy to only include actual candidates
	for i := range candidates {
		var filtered []string
		for _, b := range candidates[i].blockedBy {
			if candidateSet[b] {
				filtered = append(filtered, b)
			}
		}
		candidates[i].blockedBy = filtered
	}

	// Build adjacency for topological sort (Kahn's algorithm)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // key blocks values
	candidateMap := make(map[string]*candidate)

	for i := range candidates {
		c := &candidates[i]
		candidateMap[c.programID] = c
		if _, ok := inDegree[c.programID]; !ok {
			inDegree[c.programID] = 0
		}
		for _, dep := range c.blockedBy {
			inDegree[c.programID]++
			dependents[dep] = append(dependents[dep], c.programID)
		}
	}

	// Kahn's algorithm
	var queue []string
	for pid, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, pid)
		}
	}
	sort.Strings(queue) // deterministic ordering

	var ordered []string
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		ordered = append(ordered, pid)
		for _, dep := range dependents[pid] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}

	// Add any remaining (cycles) at the end
	if len(ordered) < len(candidates) {
		for _, c := range candidates {
			found := false
			for _, o := range ordered {
				if o == c.programID {
					found = true
					break
				}
			}
			if !found {
				ordered = append(ordered, c.programID)
			}
		}
	}

	// Classify tiers and build result
	// LEAF = no outbound deps, ROOT = no inbound, MIDDLE = both
	hasOutbound := make(map[string]bool)
	hasInbound := make(map[string]bool)
	for _, c := range candidates {
		if len(c.blockedBy) > 0 {
			hasOutbound[c.programID] = true
			for _, b := range c.blockedBy {
				hasInbound[b] = true
			}
		}
	}

	var steps []MigrationStep
	for i, pid := range ordered {
		c := candidateMap[pid]
		tier := "MIDDLE"
		if !hasOutbound[pid] {
			tier = "LEAF"
		} else if !hasInbound[pid] {
			tier = "ROOT"
		}

		steps = append(steps, MigrationStep{
			ProgramID: pid,
			Order:     i + 1,
			BlockedBy: c.blockedBy,
			Domain:    c.domain,
			Approach:  c.approach,
			Score:     c.score,
			Tier:      tier,
		})
	}

	return steps, nil
}

func (c *Client) GetFileAccessors(ctx context.Context, fileName string) (*FileAccessInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	params := map[string]any{"name": fileName}
	info := &FileAccessInfo{FileName: fileName}

	// Query programs that read/write this file
	result, err := session.Run(ctx,
		"MATCH (p:Program)-[r:READS|WRITES]->(f:File {name: $name}) "+
			"RETURN p.programId AS programId, type(r) AS accessType",
		params)
	if err != nil {
		return nil, fmt.Errorf("file accessors query: %w", err)
	}

	// Track per-program access types for dedup into READS_WRITES
	accessMap := make(map[string]map[string]bool)
	for result.Next(ctx) {
		rec := result.Record()
		pid := getStr(rec, "programId")
		atype := getStr(rec, "accessType")
		if accessMap[pid] == nil {
			accessMap[pid] = make(map[string]bool)
		}
		accessMap[pid][atype] = true
	}

	for pid, types := range accessMap {
		accessType := "READS"
		if types["READS"] && types["WRITES"] {
			accessType = "READS_WRITES"
		} else if types["WRITES"] {
			accessType = "WRITES"
		}
		info.Accessors = append(info.Accessors, FileAccessorInfo{
			ProgramID:  pid,
			AccessType: accessType,
		})
	}

	// Query DD card mappings
	ddResult, err := session.Run(ctx,
		"MATCH (dd:DDCard)-[:MAPS_TO_FILE]->(f:File {name: $name}) "+
			"RETURN dd.ddName AS ddName, dd.dsname AS dsname, dd.jobName AS jobName, "+
			"dd.stepName AS stepName, dd.isInput AS isInput, dd.isOutput AS isOutput",
		params)
	if err == nil {
		for ddResult.Next(ctx) {
			rec := ddResult.Record()
			info.DDCards = append(info.DDCards, FileAccessDDInfo{
				DDName:   getStr(rec, "ddName"),
				DSName:   getStr(rec, "dsname"),
				JobName:  getStr(rec, "jobName"),
				StepName: getStr(rec, "stepName"),
				IsInput:  getBool(rec, "isInput"),
				IsOutput: getBool(rec, "isOutput"),
			})
		}
	}

	return info, nil
}

func (c *Client) GetEffortEstimates(ctx context.Context) ([]EffortEstimate, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program)"+codebaseWhere("p", c.codebase)+" "+
			"OPTIONAL MATCH (para:Paragraph)-[:BELONGS_TO]->(p) "+
			"OPTIONAL MATCH (p)-[:INCLUDES]->(cb:Copybook) "+
			"OPTIONAL MATCH (d:DataItem {programId: p.programId}) "+
			"OPTIONAL MATCH (e:ExternalInterface {programId: p.programId}) "+
			"OPTIONAL MATCH (s:SQLStatement {programId: p.programId}) "+
			"OPTIONAL MATCH (c:CICSTransaction {programId: p.programId}) "+
			"RETURN p.programId AS programId, p.lineCount AS lineCount, "+
			"p.modernizationApproach AS approach, "+
			"count(DISTINCT para) AS paragraphs, count(DISTINCT cb) AS copybooks, "+
			"count(DISTINCT d) AS dataItems, count(DISTINCT e) AS externals, "+
			"count(DISTINCT s) AS sqls, count(DISTINCT c) AS cics "+
			"ORDER BY p.programId",
		nil)
	if err != nil {
		return nil, fmt.Errorf("effort estimates query: %w", err)
	}

	var items []EffortEstimate
	for result.Next(ctx) {
		rec := result.Record()
		lineCount := int(getInt64(rec, "lineCount"))
		paragraphs := int(getInt64(rec, "paragraphs"))
		copybooks := int(getInt64(rec, "copybooks"))
		dataItemCount := int(getInt64(rec, "dataItems"))
		externals := int(getInt64(rec, "externals"))
		sqls := int(getInt64(rec, "sqls"))
		cics := int(getInt64(rec, "cics"))

		score := paragraphs*2 + copybooks*3 + externals*5 + sqls*3 + cics*4 + lineCount/500

		tshirt := "S"
		if score > 120 {
			tshirt = "XL"
		} else if score > 60 {
			tshirt = "L"
		} else if score > 20 {
			tshirt = "M"
		}

		items = append(items, EffortEstimate{
			ProgramID:       getStr(rec, "programId"),
			ParagraphCount:  paragraphs,
			CopybookCount:   copybooks,
			DataItemCount:   dataItemCount,
			ExternalCount:   externals,
			SQLCount:        sqls,
			CICSCount:       cics,
			LineCount:       lineCount,
			TShirtSize:      tshirt,
			ComplexityScore: score,
			Approach:        getStr(rec, "approach"),
		})
	}

	return items, nil
}

// Helper methods

func (c *Client) queryCallInfoList(ctx context.Context, session neo4j.SessionWithContext, cypher string, params map[string]any) ([]CallInfo, error) {
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var items []CallInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, CallInfo{
			ProgramID:     getStr(rec, "programId"),
			IsDynamic:     getBool(rec, "isDynamic"),
			ResolvedFrom:  getStr(rec, "resolvedFrom"),
			FromParagraph: getStr(rec, "fromParagraph"),
		})
	}
	return items, nil
}

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

// GetStr extracts a string value from a Neo4j record by key.
func GetStr(rec *neo4j.Record, key string) string {
	return getStr(rec, key)
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

// GetIDMSRecords returns IDMS records associated with a program.
func (c *Client) GetIDMSRecords(ctx context.Context, programID string) ([]IDMSRecordInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (r:IDMSRecord {programId: $pid}) RETURN r.name AS name, r.area AS area, r.programId AS programId ORDER BY r.name",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("IDMS records query: %w", err)
	}

	var items []IDMSRecordInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, IDMSRecordInfo{
			Name:      getStr(rec, "name"),
			Area:      getStr(rec, "area"),
			ProgramID: getStr(rec, "programId"),
		})
	}
	return items, nil
}

// GetIDMSSchema returns the IDMS schema binding for a program.
func (c *Client) GetIDMSSchema(ctx context.Context, programID string) (*IDMSSchemaInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program {programId: $pid})-[:BINDS_TO]->(s:IDMSSchema) "+
			"RETURN s.schemaName AS schemaName, s.subschemaName AS subschemaName, s.protocolMode AS protocolMode, s.programId AS programId LIMIT 1",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("IDMS schema query: %w", err)
	}

	if !result.Next(ctx) {
		return nil, nil
	}
	rec := result.Record()
	return &IDMSSchemaInfo{
		SchemaName:    getStr(rec, "schemaName"),
		SubschemaName: getStr(rec, "subschemaName"),
		ProtocolMode:  getStr(rec, "protocolMode"),
		ProgramID:     getStr(rec, "programId"),
	}, nil
}

// GetIDMSAreas returns IDMS areas readied by a program.
func (c *Client) GetIDMSAreas(ctx context.Context, programID string) ([]IDMSAreaInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (p:Program {programId: $pid})-[r:READIES]->(a:IDMSArea) "+
			"RETURN a.name AS name, r.usageMode AS usageMode ORDER BY a.name",
		map[string]any{"pid": programID})
	if err != nil {
		return nil, fmt.Errorf("IDMS areas query: %w", err)
	}

	var items []IDMSAreaInfo
	for result.Next(ctx) {
		rec := result.Record()
		items = append(items, IDMSAreaInfo{
			Name:      getStr(rec, "name"),
			UsageMode: getStr(rec, "usageMode"),
		})
	}
	return items, nil
}

// GetIDMSImpact returns which programs navigate, store, modify, or erase a given IDMS record.
func (c *Client) GetIDMSImpact(ctx context.Context, recordName string) (*IDMSImpactInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	info := &IDMSImpactInfo{RecordName: recordName}

	for _, pair := range []struct {
		rel   string
		field *[]string
	}{
		{"NAVIGATES", &info.Navigators},
		{"STORES_IN", &info.Storers},
		{"MODIFIES", &info.Modifiers},
		{"ERASES", &info.Erasers},
	} {
		result, err := session.Run(ctx,
			fmt.Sprintf("MATCH (p:Program)-[:%s]->(r:IDMSRecord {name: $name}) RETURN DISTINCT p.programId AS pid ORDER BY pid", pair.rel),
			map[string]any{"name": recordName})
		if err != nil {
			return nil, fmt.Errorf("IDMS impact query (%s): %w", pair.rel, err)
		}
		for result.Next(ctx) {
			*pair.field = append(*pair.field, getStr(result.Record(), "pid"))
		}
	}

	return info, nil
}

// ListCodebases returns all distinct codebase identifiers in the graph.
func (c *Client) ListCodebases(ctx context.Context) ([]string, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (n) WHERE n.codebase IS NOT NULL RETURN DISTINCT n.codebase AS cb ORDER BY cb", nil)
	if err != nil {
		return nil, fmt.Errorf("listing codebases: %w", err)
	}

	var codebases []string
	for result.Next(ctx) {
		if val := getStr(result.Record(), "cb"); val != "" {
			codebases = append(codebases, val)
		}
	}
	return codebases, nil
}

// GetCrossCodebaseCalls finds CALLS relationships between programs in different codebases.
func (c *Client) GetCrossCodebaseCalls(ctx context.Context) ([]CrossCodebaseCall, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (a:Program)-[:CALLS]->(b:Program) "+
			"WHERE a.codebase IS NOT NULL AND b.codebase IS NOT NULL AND a.codebase <> b.codebase "+
			"RETURN a.programId AS callerId, a.codebase AS callerCB, b.programId AS calleeId, b.codebase AS calleeCB", nil)
	if err != nil {
		return nil, fmt.Errorf("cross-codebase calls query: %w", err)
	}

	var calls []CrossCodebaseCall
	for result.Next(ctx) {
		rec := result.Record()
		calls = append(calls, CrossCodebaseCall{
			CallerID:       getStr(rec, "callerId"),
			CallerCodebase: getStr(rec, "callerCB"),
			CalleeID:       getStr(rec, "calleeId"),
			CalleeCodebase: getStr(rec, "calleeCB"),
		})
	}
	return calls, nil
}
