package main

import (
	"context"
	"fmt"

	n4j "cobol-ingestor/internal/neo4j"
)

// GraphService wraps the Neo4j Reader for frontend graph queries.
type GraphService struct {
	app *App
}

func (s *GraphService) reader() (n4j.Reader, error) {
	r := s.app.Neo4jService.reader
	if r == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	return r, nil
}

func (s *GraphService) GetDashboardStats() (*n4j.DashboardStats, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDashboardStats(context.Background())
}

func (s *GraphService) ListPrograms(search string, page, pageSize int) (*n4j.PagedResponse, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return r.ListPrograms(context.Background(), n4j.Filter{Search: search}, page, pageSize)
}

func (s *GraphService) GetProgram(programID string) (*n4j.ProgramDetail, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetProgram(context.Background(), programID)
}

func (s *GraphService) GetCallChain(programID, direction string, depth int) ([]n4j.CallChainNode, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if depth < 1 {
		depth = 3
	}
	return r.GetCallChain(context.Background(), programID, direction, depth)
}

func (s *GraphService) GetImpactAnalysis(programID string) (*n4j.ImpactResult, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetImpactAnalysis(context.Background(), programID)
}

func (s *GraphService) ListCopybooks(search string, page, pageSize int) (*n4j.PagedResponse, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return r.ListCopybooks(context.Background(), n4j.Filter{Search: search}, page, pageSize)
}

func (s *GraphService) GetCopybookUsage(name string) (*n4j.CopybookUsage, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetCopybookUsage(context.Background(), name)
}

func (s *GraphService) ListBusinessDomains() ([]n4j.BusinessDomainSummary, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListBusinessDomains(context.Background())
}

func (s *GraphService) GetBusinessDomain(name string) (*n4j.BusinessDomainDetail, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetBusinessDomain(context.Background(), name)
}

func (s *GraphService) SearchFullText(query string, limit int) ([]n4j.SearchResult, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 20
	}
	return r.SearchFullText(context.Background(), query, limit)
}

func (s *GraphService) ListModernizationCandidates() ([]n4j.ModernizationCandidateInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListModernizationCandidates(context.Background())
}

func (s *GraphService) GetDeadCodeSummary() ([]n4j.DeadCodeSummaryInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDeadCodeSummary(context.Background())
}

func (s *GraphService) GetDataItems(programID string) ([]n4j.DataItemInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDataItems(context.Background(), programID)
}

func (s *GraphService) GetParagraphFlow(programID string) ([]n4j.ParagraphFlowInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetParagraphFlow(context.Background(), programID)
}

func (s *GraphService) GetDataFlow(programID string) ([]n4j.DataFlowInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDataFlow(context.Background(), programID)
}

func (s *GraphService) ListJCLJobs() ([]n4j.JCLJobInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListJCLJobs(context.Background())
}

func (s *GraphService) ListRiskPrograms(minScore float64) ([]n4j.RiskProgramInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListRiskPrograms(context.Background(), minScore)
}

func (s *GraphService) GetMigrationSequence() ([]n4j.MigrationStep, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetMigrationSequence(context.Background())
}

func (s *GraphService) GetEffortEstimates() ([]n4j.EffortEstimate, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetEffortEstimates(context.Background())
}

func (s *GraphService) GetValidationReport() (*n4j.ValidationResult, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetValidationReport(context.Background())
}

func (s *GraphService) GetProgramSource(programID string) (*n4j.ProgramSourceInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetProgramSource(context.Background(), programID)
}
