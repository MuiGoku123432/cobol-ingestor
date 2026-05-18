package targetstack

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/neo4j"
)

// WriteResult writes a complete TargetStackResult (one repo) to Neo4j.
func WriteResult(ctx context.Context, w *neo4j.BatchWriter, repo graph.TargetRepo, result *graph.TargetStackResult) error {
	// 1. TargetRepo node
	repoNode := []map[string]any{{
		"id":         repo.ID,
		"name":       repo.Name,
		"url":        repo.URL,
		"provider":   repo.Provider,
		"branch":     repo.Branch,
		"lastCommit": repo.LastCommit,
		"localPath":  repo.LocalPath,
		"language":   repo.Language,
		"framework":  repo.Framework,
	}}
	if err := w.WriteNodes(ctx, "TargetRepo", "id", repoNode); err != nil {
		return fmt.Errorf("writing TargetRepo: %w", err)
	}

	// 2. TargetService nodes + TS_CONTAINS relationships
	if len(result.Services) > 0 {
		nodes := make([]map[string]any, len(result.Services))
		rels := make([]map[string]any, len(result.Services))
		for i, s := range result.Services {
			nodes[i] = map[string]any{
				"id":          s.ID,
				"name":        s.Name,
				"serviceType": s.ServiceType,
				"description": s.Description,
				"repoUrl":     s.RepoURL,
				"basePath":    s.BasePath,
				"language":    s.Language,
				"framework":   s.Framework,
			}
			rels[i] = map[string]any{
				"fromKey": repo.ID,
				"toKey":   s.ID,
				"props":   map[string]any{},
			}
		}
		if err := w.WriteNodes(ctx, "TargetService", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetService nodes: %w", err)
		}
		if err := w.WriteRelationships(ctx, "TS_CONTAINS", "TargetRepo", "id", "TargetService", "id", rels); err != nil {
			return fmt.Errorf("writing TS_CONTAINS: %w", err)
		}
	}

	// 3. TargetEndpoint nodes + TS_EXPOSES relationships
	if len(result.Endpoints) > 0 {
		nodes := make([]map[string]any, len(result.Endpoints))
		rels := make([]map[string]any, 0, len(result.Endpoints))
		for i, e := range result.Endpoints {
			nodes[i] = map[string]any{
				"id":          e.ID,
				"method":      e.Method,
				"path":        e.Path,
				"description": e.Description,
				"serviceName": e.ServiceName,
				"parameters":  e.Parameters,
			}
			if e.ServiceName != "" {
				rels = append(rels, map[string]any{
					"fromKey": repo.URL + "::" + e.ServiceName,
					"toKey":   e.ID,
					"props":   map[string]any{},
				})
			}
		}
		if err := w.WriteNodes(ctx, "TargetEndpoint", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetEndpoint nodes: %w", err)
		}
		if len(rels) > 0 {
			if err := w.WriteRelationships(ctx, "TS_EXPOSES", "TargetService", "id", "TargetEndpoint", "id", rels); err != nil {
				return fmt.Errorf("writing TS_EXPOSES: %w", err)
			}
		}
	}

	// 4. TargetBusinessRule nodes + TS_ENFORCES relationships
	if len(result.Rules) > 0 {
		nodes := make([]map[string]any, len(result.Rules))
		rels := make([]map[string]any, 0, len(result.Rules))
		for i, r := range result.Rules {
			nodes[i] = map[string]any{
				"id":          r.ID,
				"name":        r.Name,
				"description": r.Description,
				"category":    r.Category,
				"serviceName": r.ServiceName,
				"sourceFile":  r.SourceFile,
				"confidence":  r.Confidence,
			}
			if r.ServiceName != "" {
				rels = append(rels, map[string]any{
					"fromKey": repo.URL + "::" + r.ServiceName,
					"toKey":   r.ID,
					"props":   map[string]any{},
				})
			}
		}
		if err := w.WriteNodes(ctx, "TargetBusinessRule", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetBusinessRule nodes: %w", err)
		}
		if len(rels) > 0 {
			if err := w.WriteRelationships(ctx, "TS_ENFORCES", "TargetService", "id", "TargetBusinessRule", "id", rels); err != nil {
				return fmt.Errorf("writing TS_ENFORCES: %w", err)
			}
		}
	}

	// 5. TargetDataModel nodes + TS_MODELS relationships
	if len(result.DataModels) > 0 {
		nodes := make([]map[string]any, len(result.DataModels))
		rels := make([]map[string]any, 0, len(result.DataModels))
		for i, m := range result.DataModels {
			nodes[i] = map[string]any{
				"id":          m.ID,
				"name":        m.Name,
				"description": m.Description,
				"serviceName": m.ServiceName,
				"sourceFile":  m.SourceFile,
				"fields":      m.Fields,
				"tableName":   m.TableName,
			}
			if m.ServiceName != "" {
				rels = append(rels, map[string]any{
					"fromKey": repo.URL + "::" + m.ServiceName,
					"toKey":   m.ID,
					"props":   map[string]any{},
				})
			}
		}
		if err := w.WriteNodes(ctx, "TargetDataModel", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetDataModel nodes: %w", err)
		}
		if len(rels) > 0 {
			if err := w.WriteRelationships(ctx, "TS_MODELS", "TargetService", "id", "TargetDataModel", "id", rels); err != nil {
				return fmt.Errorf("writing TS_MODELS: %w", err)
			}
		}
	}

	// 6. TargetIntegration nodes + TS_INTEGRATES relationships
	if len(result.Integrations) > 0 {
		nodes := make([]map[string]any, len(result.Integrations))
		rels := make([]map[string]any, 0, len(result.Integrations))
		for i, it := range result.Integrations {
			nodes[i] = map[string]any{
				"id":              it.ID,
				"integrationType": it.IntegrationType,
				"target":          it.Target,
				"description":     it.Description,
				"serviceName":     it.ServiceName,
			}
			if it.ServiceName != "" {
				rels = append(rels, map[string]any{
					"fromKey": repo.URL + "::" + it.ServiceName,
					"toKey":   it.ID,
					"props":   map[string]any{},
				})
			}
		}
		if err := w.WriteNodes(ctx, "TargetIntegration", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetIntegration nodes: %w", err)
		}
		if len(rels) > 0 {
			if err := w.WriteRelationships(ctx, "TS_INTEGRATES", "TargetService", "id", "TargetIntegration", "id", rels); err != nil {
				return fmt.Errorf("writing TS_INTEGRATES: %w", err)
			}
		}
	}

	// 7. TargetErrorHandler nodes + TS_HANDLES_ERROR relationships
	if len(result.ErrorHandlers) > 0 {
		nodes := make([]map[string]any, len(result.ErrorHandlers))
		rels := make([]map[string]any, 0, len(result.ErrorHandlers))
		for i, eh := range result.ErrorHandlers {
			nodes[i] = map[string]any{
				"id":          eh.ID,
				"pattern":     eh.Pattern,
				"description": eh.Description,
				"serviceName": eh.ServiceName,
				"sourceFile":  eh.SourceFile,
			}
			if eh.ServiceName != "" {
				rels = append(rels, map[string]any{
					"fromKey": repo.URL + "::" + eh.ServiceName,
					"toKey":   eh.ID,
					"props":   map[string]any{},
				})
			}
		}
		if err := w.WriteNodes(ctx, "TargetErrorHandler", "id", nodes); err != nil {
			return fmt.Errorf("writing TargetErrorHandler nodes: %w", err)
		}
		if len(rels) > 0 {
			if err := w.WriteRelationships(ctx, "TS_HANDLES_ERROR", "TargetService", "id", "TargetErrorHandler", "id", rels); err != nil {
				return fmt.Errorf("writing TS_HANDLES_ERROR: %w", err)
			}
		}
	}

	return nil
}

// WriteGaps writes BusinessGap nodes with GAP_FROM/GAP_TO relationships.
func WriteGaps(ctx context.Context, w *neo4j.BatchWriter, gaps []graph.BusinessGap) error {
	if len(gaps) == 0 {
		return nil
	}

	nodes := make([]map[string]any, len(gaps))
	for i, g := range gaps {
		nodes[i] = map[string]any{
			"id":           g.ID,
			"gapType":      g.GapType,
			"category":     g.Category,
			"description":  g.Description,
			"severity":     g.Severity,
			"cobolSource":  g.CobolSource,
			"targetSource": g.TargetSource,
			"confidence":   g.Confidence,
		}
	}
	return w.WriteNodes(ctx, "BusinessGap", "id", nodes)
}

// WriteRequirements writes BusinessRequirement nodes with REQUIREMENT_FOR relationships.
func WriteRequirements(ctx context.Context, w *neo4j.BatchWriter, reqs []graph.BusinessRequirement) error {
	if len(reqs) == 0 {
		return nil
	}

	nodes := make([]map[string]any, len(reqs))
	rels := make([]map[string]any, 0, len(reqs))
	for i, r := range reqs {
		nodes[i] = map[string]any{
			"id":                 r.ID,
			"title":              r.Title,
			"description":        r.Description,
			"priority":           r.Priority,
			"category":           r.Category,
			"acceptanceCriteria": r.AcceptanceCriteria,
			"estimatedEffort":    r.EstimatedEffort,
			"gapId":              r.GapID,
		}
		if r.GapID != "" {
			rels = append(rels, map[string]any{
				"fromKey": r.ID,
				"toKey":   r.GapID,
				"props":   map[string]any{},
			})
		}
	}

	if err := w.WriteNodes(ctx, "BusinessRequirement", "id", nodes); err != nil {
		return fmt.Errorf("writing BusinessRequirement nodes: %w", err)
	}
	if len(rels) > 0 {
		if err := w.WriteRelationships(ctx, "REQUIREMENT_FOR", "BusinessRequirement", "id", "BusinessGap", "id", rels); err != nil {
			return fmt.Errorf("writing REQUIREMENT_FOR: %w", err)
		}
	}
	return nil
}
