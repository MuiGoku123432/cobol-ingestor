package handlers_test

import (
	"context"

	n4j "cobol-ingestor/internal/neo4j"
)

// MockReader implements the Reader interface for handler tests.
type MockReader struct {
	Programs       []n4j.ProgramSummary
	ProgramDetail  *n4j.ProgramDetail
	CallChainNodes []n4j.CallChainNode
	DataItems      []n4j.DataItemInfo
	Impact         *n4j.ImpactResult
	Copybooks      []n4j.CopybookSummary
	CopybookUsage  *n4j.CopybookUsage
	Stats          *n4j.DashboardStats
	SearchResults  []n4j.SearchResult
	Domains        []n4j.BusinessDomainSummary
	DomainDetail   *n4j.BusinessDomainDetail
	Err            error
}

func (m *MockReader) ListPrograms(_ context.Context, _ n4j.Filter, page, pageSize int) (*n4j.PagedResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &n4j.PagedResponse{Data: m.Programs, Total: len(m.Programs), Page: page, PageSize: pageSize}, nil
}

func (m *MockReader) GetProgram(_ context.Context, _ string) (*n4j.ProgramDetail, error) {
	return m.ProgramDetail, m.Err
}

func (m *MockReader) GetCallChain(_ context.Context, _ string, _ string, _ int) ([]n4j.CallChainNode, error) {
	return m.CallChainNodes, m.Err
}

func (m *MockReader) GetDataItems(_ context.Context, _ string) ([]n4j.DataItemInfo, error) {
	return m.DataItems, m.Err
}

func (m *MockReader) GetImpactAnalysis(_ context.Context, _ string) (*n4j.ImpactResult, error) {
	return m.Impact, m.Err
}

func (m *MockReader) ListCopybooks(_ context.Context, _ n4j.Filter, page, pageSize int) (*n4j.PagedResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &n4j.PagedResponse{Data: m.Copybooks, Total: len(m.Copybooks), Page: page, PageSize: pageSize}, nil
}

func (m *MockReader) GetCopybookUsage(_ context.Context, _ string) (*n4j.CopybookUsage, error) {
	return m.CopybookUsage, m.Err
}

func (m *MockReader) GetDashboardStats(_ context.Context) (*n4j.DashboardStats, error) {
	return m.Stats, m.Err
}

func (m *MockReader) SearchFullText(_ context.Context, _ string, _ int) ([]n4j.SearchResult, error) {
	return m.SearchResults, m.Err
}

func (m *MockReader) ListBusinessDomains(_ context.Context) ([]n4j.BusinessDomainSummary, error) {
	return m.Domains, m.Err
}

func (m *MockReader) GetBusinessDomain(_ context.Context, _ string) (*n4j.BusinessDomainDetail, error) {
	return m.DomainDetail, m.Err
}
