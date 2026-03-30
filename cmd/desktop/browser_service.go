package main

import (
	"context"
	"fmt"

	n4j "cobol-ingestor/internal/neo4j"
)

// BrowserService exposes read-only Neo4j data to the desktop frontend.
// Every method delegates to the existing Reader interface — zero new query logic.
type BrowserService struct {
	app *App
}

func (s *BrowserService) reader() (n4j.Reader, error) {
	r := s.app.Neo4jService.reader
	if r == nil {
		return nil, fmt.Errorf("not connected to Neo4j")
	}
	return r, nil
}

// ──────────────────────────────────────────────
// Dashboard
// ──────────────────────────────────────────────

func (s *BrowserService) GetDashboardStats() (*n4j.DashboardStats, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDashboardStats(context.Background())
}

// ──────────────────────────────────────────────
// Program Browser
// ──────────────────────────────────────────────

func (s *BrowserService) ListPrograms(search, codebase string, page, pageSize int) (*n4j.PagedResponse, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	return r.ListPrograms(context.Background(), n4j.Filter{Search: search, Codebase: codebase}, page, pageSize)
}

func (s *BrowserService) GetProgram(programID string) (*n4j.ProgramDetail, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetProgram(context.Background(), programID)
}

func (s *BrowserService) GetCallChain(programID, direction string, depth int) ([]n4j.CallChainNode, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if depth < 1 {
		depth = 3
	}
	return r.GetCallChain(context.Background(), programID, direction, depth)
}

func (s *BrowserService) GetEffortEstimate(programID string) (*n4j.EffortEstimate, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	estimates, err := r.GetEffortEstimates(context.Background())
	if err != nil {
		return nil, err
	}
	for _, e := range estimates {
		if e.ProgramID == programID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("no effort estimate found for %s", programID)
}

func (s *BrowserService) GetImpactAnalysis(programID string) (*n4j.ImpactResult, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetImpactAnalysis(context.Background(), programID)
}

// ──────────────────────────────────────────────
// Global Search
// ──────────────────────────────────────────────

func (s *BrowserService) SearchFullText(query string, limit int) ([]n4j.SearchResult, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 20
	}
	return r.SearchFullText(context.Background(), query, limit)
}

// ──────────────────────────────────────────────
// Analysis Reports
// ──────────────────────────────────────────────

func (s *BrowserService) ListModernizationCandidates() ([]n4j.ModernizationCandidateInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListModernizationCandidates(context.Background())
}

func (s *BrowserService) ListRiskPrograms(minScore float64) ([]n4j.RiskProgramInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListRiskPrograms(context.Background(), minScore)
}

func (s *BrowserService) GetMigrationSequence() ([]n4j.MigrationStep, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetMigrationSequence(context.Background())
}

func (s *BrowserService) GetDeadCodeSummary() ([]n4j.DeadCodeSummaryInfo, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetDeadCodeSummary(context.Background())
}

func (s *BrowserService) GetEffortEstimates() ([]n4j.EffortEstimate, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetEffortEstimates(context.Background())
}

// ──────────────────────────────────────────────
// Domain Browser
// ──────────────────────────────────────────────

func (s *BrowserService) ListBusinessDomains() ([]n4j.BusinessDomainSummary, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.ListBusinessDomains(context.Background())
}

func (s *BrowserService) GetBusinessDomain(name string) (*n4j.BusinessDomainDetail, error) {
	r, err := s.reader()
	if err != nil {
		return nil, err
	}
	return r.GetBusinessDomain(context.Background(), name)
}
