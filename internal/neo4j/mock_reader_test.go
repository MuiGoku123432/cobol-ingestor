package neo4j_test

import (
	"context"

	n4j "cobol-ingestor/internal/neo4j"
)

// MockReader implements the Reader interface for testing.
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

var _ n4j.Reader = (*MockReader)(nil)

func (m *MockReader) ListPrograms(_ context.Context, _ n4j.Filter, page, pageSize int) (*n4j.PagedResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &n4j.PagedResponse{Data: m.Programs, Total: len(m.Programs), Page: page, PageSize: pageSize}, nil
}

func (m *MockReader) GetProgram(_ context.Context, _ string) (*n4j.ProgramDetail, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ProgramDetail, nil
}

func (m *MockReader) GetCallChain(_ context.Context, _ string, _ string, _ int) ([]n4j.CallChainNode, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.CallChainNodes, nil
}

func (m *MockReader) GetDataItems(_ context.Context, _ string) ([]n4j.DataItemInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.DataItems, nil
}

func (m *MockReader) GetImpactAnalysis(_ context.Context, _ string) (*n4j.ImpactResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Impact, nil
}

func (m *MockReader) ListCopybooks(_ context.Context, _ n4j.Filter, page, pageSize int) (*n4j.PagedResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &n4j.PagedResponse{Data: m.Copybooks, Total: len(m.Copybooks), Page: page, PageSize: pageSize}, nil
}

func (m *MockReader) GetCopybookUsage(_ context.Context, _ string) (*n4j.CopybookUsage, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.CopybookUsage, nil
}

func (m *MockReader) GetDashboardStats(_ context.Context) (*n4j.DashboardStats, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Stats, nil
}

func (m *MockReader) SearchFullText(_ context.Context, _ string, _ int) ([]n4j.SearchResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.SearchResults, nil
}

func (m *MockReader) ListBusinessDomains(_ context.Context) ([]n4j.BusinessDomainSummary, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Domains, nil
}

func (m *MockReader) GetBusinessDomain(_ context.Context, _ string) (*n4j.BusinessDomainDetail, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.DomainDetail, nil
}
