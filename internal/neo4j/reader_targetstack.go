package neo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TargetRepoInfo holds summary info about a connected target repository.
type TargetRepoInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Provider   string `json:"provider"`
	Branch     string `json:"branch"`
	LastCommit string `json:"lastCommit"`
	Language   string `json:"language"`
	Framework  string `json:"framework"`
}

// TargetServiceInfo holds summary info about a target stack service.
type TargetServiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ServiceType string `json:"serviceType"`
	Description string `json:"description"`
	RepoURL     string `json:"repoUrl"`
	Language    string `json:"language"`
	Framework   string `json:"framework"`
}

// TargetServiceDetail extends TargetServiceInfo with related nodes.
type TargetServiceDetail struct {
	TargetServiceInfo
	Endpoints     []map[string]any `json:"endpoints"`
	Rules         []map[string]any `json:"rules"`
	DataModels    []map[string]any `json:"dataModels"`
	Integrations  []map[string]any `json:"integrations"`
	ErrorHandlers []map[string]any `json:"errorHandlers"`
}

// BusinessGapInfo holds a business gap record.
type BusinessGapInfo struct {
	ID           string  `json:"id"`
	GapType      string  `json:"gapType"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	Severity     string  `json:"severity"`
	CobolSource  string  `json:"cobolSource"`
	TargetSource string  `json:"targetSource"`
	Confidence   float64 `json:"confidence"`
}

// BusinessRequirementInfo holds a generated business requirement.
type BusinessRequirementInfo struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	Priority           string `json:"priority"`
	Category           string `json:"category"`
	AcceptanceCriteria string `json:"acceptanceCriteria"`
	EstimatedEffort    string `json:"estimatedEffort"`
	GapID              string `json:"gapId"`
}

// GapCoverageSummary holds aggregate coverage statistics.
type GapCoverageSummary struct {
	TotalCobolDomains    int     `json:"totalCobolDomains"`
	CoveredDomains       int     `json:"coveredDomains"`
	UncoveredDomains     int     `json:"uncoveredDomains"`
	CoveragePercent      float64 `json:"coveragePercent"`
	TotalGaps            int     `json:"totalGaps"`
	CriticalGaps         int     `json:"criticalGaps"`
	HighGaps             int     `json:"highGaps"`
	TotalRequirements    int     `json:"totalRequirements"`
}

// ListTargetRepos returns all connected target repositories.
func (c *Client) ListTargetRepos(ctx context.Context) ([]TargetRepoInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		records, err := tx.Run(ctx,
			"MATCH (r:TargetRepo) RETURN r.id AS id, r.name AS name, r.url AS url, r.provider AS provider, r.branch AS branch, r.lastCommit AS lastCommit, r.language AS language, r.framework AS framework ORDER BY r.name",
			nil)
		if err != nil {
			return nil, err
		}
		var repos []TargetRepoInfo
		for records.Next(ctx) {
			rec := records.Record()
			repos = append(repos, TargetRepoInfo{
				ID:         getStr(rec, "id"),
				Name:       getStr(rec, "name"),
				URL:        getStr(rec, "url"),
				Provider:   getStr(rec, "provider"),
				Branch:     getStr(rec, "branch"),
				LastCommit: getStr(rec, "lastCommit"),
				Language:   getStr(rec, "language"),
				Framework:  getStr(rec, "framework"),
			})
		}
		return repos, records.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]TargetRepoInfo), nil
}

// ListTargetServices returns services, optionally filtered by repo URL.
func (c *Client) ListTargetServices(ctx context.Context, repoURL string) ([]TargetServiceInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		var query string
		var params map[string]any
		if repoURL != "" {
			query = "MATCH (s:TargetService {repoUrl: $repoUrl}) RETURN s.id AS id, s.name AS name, s.serviceType AS serviceType, s.description AS description, s.repoUrl AS repoUrl, s.language AS language, s.framework AS framework ORDER BY s.name"
			params = map[string]any{"repoUrl": repoURL}
		} else {
			query = "MATCH (s:TargetService) RETURN s.id AS id, s.name AS name, s.serviceType AS serviceType, s.description AS description, s.repoUrl AS repoUrl, s.language AS language, s.framework AS framework ORDER BY s.name"
			params = nil
		}
		records, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		var services []TargetServiceInfo
		for records.Next(ctx) {
			rec := records.Record()
			services = append(services, TargetServiceInfo{
				ID:          getStr(rec, "id"),
				Name:        getStr(rec, "name"),
				ServiceType: getStr(rec, "serviceType"),
				Description: getStr(rec, "description"),
				RepoURL:     getStr(rec, "repoUrl"),
				Language:    getStr(rec, "language"),
				Framework:   getStr(rec, "framework"),
			})
		}
		return services, records.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]TargetServiceInfo), nil
}

// GetTargetService returns full details for a service including related nodes.
func (c *Client) GetTargetService(ctx context.Context, serviceID string) (*TargetServiceDetail, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Base service
		svcRec, err := tx.Run(ctx,
			"MATCH (s:TargetService {id: $id}) RETURN s.id AS id, s.name AS name, s.serviceType AS serviceType, s.description AS description, s.repoUrl AS repoUrl, s.language AS language, s.framework AS framework",
			map[string]any{"id": serviceID})
		if err != nil {
			return nil, err
		}
		if !svcRec.Next(ctx) {
			return nil, nil
		}
		rec := svcRec.Record()
		detail := &TargetServiceDetail{
			TargetServiceInfo: TargetServiceInfo{
				ID:          getStr(rec, "id"),
				Name:        getStr(rec, "name"),
				ServiceType: getStr(rec, "serviceType"),
				Description: getStr(rec, "description"),
				RepoURL:     getStr(rec, "repoUrl"),
				Language:    getStr(rec, "language"),
				Framework:   getStr(rec, "framework"),
			},
		}

		// Endpoints
		epRec, err := tx.Run(ctx,
			"MATCH (s:TargetService {id: $id})-[:TS_EXPOSES]->(e:TargetEndpoint) RETURN e.id AS id, e.method AS method, e.path AS path, e.description AS description ORDER BY e.path",
			map[string]any{"id": serviceID})
		if err == nil {
			for epRec.Next(ctx) {
				r := epRec.Record()
				detail.Endpoints = append(detail.Endpoints, map[string]any{
					"id": getStr(r, "id"), "method": getStr(r, "method"),
					"path": getStr(r, "path"), "description": getStr(r, "description"),
				})
			}
		}

		// Rules
		ruleRec, err := tx.Run(ctx,
			"MATCH (s:TargetService {id: $id})-[:TS_ENFORCES]->(r:TargetBusinessRule) RETURN r.id AS id, r.name AS name, r.category AS category, r.description AS description, r.confidence AS confidence ORDER BY r.name",
			map[string]any{"id": serviceID})
		if err == nil {
			for ruleRec.Next(ctx) {
				r := ruleRec.Record()
				detail.Rules = append(detail.Rules, map[string]any{
					"id": getStr(r, "id"), "name": getStr(r, "name"),
					"category": getStr(r, "category"), "description": getStr(r, "description"),
					"confidence": getFloat64(r, "confidence"),
				})
			}
		}

		// Data models
		modelRec, err := tx.Run(ctx,
			"MATCH (s:TargetService {id: $id})-[:TS_MODELS]->(m:TargetDataModel) RETURN m.id AS id, m.name AS name, m.description AS description, m.tableName AS tableName ORDER BY m.name",
			map[string]any{"id": serviceID})
		if err == nil {
			for modelRec.Next(ctx) {
				r := modelRec.Record()
				detail.DataModels = append(detail.DataModels, map[string]any{
					"id": getStr(r, "id"), "name": getStr(r, "name"),
					"description": getStr(r, "description"), "tableName": getStr(r, "tableName"),
				})
			}
		}

		// Integrations
		intRec, err := tx.Run(ctx,
			"MATCH (s:TargetService {id: $id})-[:TS_INTEGRATES]->(i:TargetIntegration) RETURN i.id AS id, i.integrationType AS integrationType, i.target AS target, i.description AS description ORDER BY i.target",
			map[string]any{"id": serviceID})
		if err == nil {
			for intRec.Next(ctx) {
				r := intRec.Record()
				detail.Integrations = append(detail.Integrations, map[string]any{
					"id": getStr(r, "id"), "integrationType": getStr(r, "integrationType"),
					"target": getStr(r, "target"), "description": getStr(r, "description"),
				})
			}
		}

		return detail, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.(*TargetServiceDetail), nil
}

// ListBusinessGaps returns all business gaps, optionally filtered by type/severity/category.
func (c *Client) ListBusinessGaps(ctx context.Context, gapType, severity, category string) ([]BusinessGapInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		where := "WHERE 1=1"
		params := map[string]any{}
		if gapType != "" {
			where += " AND g.gapType = $gapType"
			params["gapType"] = gapType
		}
		if severity != "" {
			where += " AND g.severity = $severity"
			params["severity"] = severity
		}
		if category != "" {
			where += " AND g.category = $category"
			params["category"] = category
		}

		records, err := tx.Run(ctx,
			"MATCH (g:BusinessGap) "+where+" RETURN g.id AS id, g.gapType AS gapType, g.category AS category, g.description AS description, g.severity AS severity, g.cobolSource AS cobolSource, g.targetSource AS targetSource, g.confidence AS confidence ORDER BY g.severity, g.category",
			params)
		if err != nil {
			return nil, err
		}
		var gaps []BusinessGapInfo
		for records.Next(ctx) {
			rec := records.Record()
			gaps = append(gaps, BusinessGapInfo{
				ID:           getStr(rec, "id"),
				GapType:      getStr(rec, "gapType"),
				Category:     getStr(rec, "category"),
				Description:  getStr(rec, "description"),
				Severity:     getStr(rec, "severity"),
				CobolSource:  getStr(rec, "cobolSource"),
				TargetSource: getStr(rec, "targetSource"),
				Confidence:   getFloat64(rec, "confidence"),
			})
		}
		return gaps, records.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]BusinessGapInfo), nil
}

// ListBusinessRequirements returns generated business requirements, optionally filtered by priority.
func (c *Client) ListBusinessRequirements(ctx context.Context, priority string) ([]BusinessRequirementInfo, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		where := ""
		params := map[string]any{}
		if priority != "" {
			where = "WHERE r.priority = $priority "
			params["priority"] = priority
		}
		records, err := tx.Run(ctx,
			"MATCH (r:BusinessRequirement) "+where+"RETURN r.id AS id, r.title AS title, r.description AS description, r.priority AS priority, r.category AS category, r.acceptanceCriteria AS acceptanceCriteria, r.estimatedEffort AS estimatedEffort, r.gapId AS gapId ORDER BY r.priority, r.title",
			params)
		if err != nil {
			return nil, err
		}
		var reqs []BusinessRequirementInfo
		for records.Next(ctx) {
			rec := records.Record()
			reqs = append(reqs, BusinessRequirementInfo{
				ID:                 getStr(rec, "id"),
				Title:              getStr(rec, "title"),
				Description:        getStr(rec, "description"),
				Priority:           getStr(rec, "priority"),
				Category:           getStr(rec, "category"),
				AcceptanceCriteria: getStr(rec, "acceptanceCriteria"),
				EstimatedEffort:    getStr(rec, "estimatedEffort"),
				GapID:              getStr(rec, "gapId"),
			})
		}
		return reqs, records.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]BusinessRequirementInfo), nil
}

// GetGapCoverageSummary returns aggregate gap coverage statistics.
func (c *Client) GetGapCoverageSummary(ctx context.Context) (*GapCoverageSummary, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		summary := &GapCoverageSummary{}

		// Total COBOL domains
		rec, err := tx.Run(ctx, "MATCH (d:BusinessDomain) RETURN count(d) AS cnt", nil)
		if err == nil && rec.Next(ctx) {
			summary.TotalCobolDomains = int(getInt64(rec.Record(), "cnt"))
		}

		// Covered domains (at least one TS_MAPS_TO_COBOL relationship)
		rec2, err2 := tx.Run(ctx,
			"MATCH (d:BusinessDomain)<-[:BELONGS_TO]-(p:Program)<-[:TS_MAPS_TO_COBOL]-(s:TargetService) RETURN count(DISTINCT d) AS cnt",
			nil)
		if err2 == nil && rec2.Next(ctx) {
			summary.CoveredDomains = int(getInt64(rec2.Record(), "cnt"))
		}
		summary.UncoveredDomains = summary.TotalCobolDomains - summary.CoveredDomains
		if summary.TotalCobolDomains > 0 {
			summary.CoveragePercent = float64(summary.CoveredDomains) / float64(summary.TotalCobolDomains) * 100
		}

		// Gap counts
		rec3, err3 := tx.Run(ctx, "MATCH (g:BusinessGap) RETURN count(g) AS total, sum(CASE WHEN g.severity='CRITICAL' THEN 1 ELSE 0 END) AS critical, sum(CASE WHEN g.severity='HIGH' THEN 1 ELSE 0 END) AS high", nil)
		if err3 == nil && rec3.Next(ctx) {
			r := rec3.Record()
			summary.TotalGaps = int(getInt64(r, "total"))
			summary.CriticalGaps = int(getInt64(r, "critical"))
			summary.HighGaps = int(getInt64(r, "high"))
		}

		// Requirements count
		rec4, err4 := tx.Run(ctx, "MATCH (r:BusinessRequirement) RETURN count(r) AS cnt", nil)
		if err4 == nil && rec4.Next(ctx) {
			summary.TotalRequirements = int(getInt64(rec4.Record(), "cnt"))
		}

		return summary, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*GapCoverageSummary), nil
}

// GetTargetStackDashboard returns aggregate statistics about the connected target stack.
func (c *Client) GetTargetStackDashboard(ctx context.Context) (map[string]any, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		stats := map[string]any{}

		queries := map[string]string{
			"repos":        "MATCH (n:TargetRepo) RETURN count(n) AS cnt",
			"services":     "MATCH (n:TargetService) RETURN count(n) AS cnt",
			"endpoints":    "MATCH (n:TargetEndpoint) RETURN count(n) AS cnt",
			"rules":        "MATCH (n:TargetBusinessRule) RETURN count(n) AS cnt",
			"dataModels":   "MATCH (n:TargetDataModel) RETURN count(n) AS cnt",
			"integrations": "MATCH (n:TargetIntegration) RETURN count(n) AS cnt",
			"gaps":         "MATCH (n:BusinessGap) RETURN count(n) AS cnt",
			"requirements": "MATCH (n:BusinessRequirement) RETURN count(n) AS cnt",
		}

		for key, query := range queries {
			rec, err := tx.Run(ctx, query, nil)
			if err == nil && rec.Next(ctx) {
				stats[key] = getInt64(rec.Record(), "cnt")
			}
		}
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(map[string]any), nil
}

