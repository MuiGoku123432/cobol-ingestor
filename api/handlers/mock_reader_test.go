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

func (m *MockReader) GetProgramConditions(_ context.Context, _ string) ([]n4j.ConditionInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramParameters(_ context.Context, _ string) ([]n4j.ParameterInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramConditionalLogic(_ context.Context, _ string) ([]n4j.ConditionalLogicInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramErrorHandlers(_ context.Context, _ string) ([]n4j.ErrorHandlerInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramExternalInterfaces(_ context.Context, _ string) ([]n4j.ExternalInterfaceInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListBridgePrograms(_ context.Context) ([]n4j.BridgeProgramInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListCopybookRisks(_ context.Context) ([]n4j.CopybookRiskInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListModernizationCandidates(_ context.Context) ([]n4j.ModernizationCandidateInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListRiskPrograms(_ context.Context, _ float64) ([]n4j.RiskProgramInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListVolumeEstimates(_ context.Context) ([]n4j.VolumeEstimateInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramSQL(_ context.Context, _ string) ([]n4j.SQLStatementInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramCICS(_ context.Context, _ string) ([]n4j.CICSTransactionInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetParagraphFlow(_ context.Context, _ string) ([]n4j.ParagraphFlowInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetDataFlow(_ context.Context, _ string) ([]n4j.DataFlowInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetDataHierarchy(_ context.Context, _ string) ([]n4j.DataHierarchyInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetDeadParagraphs(_ context.Context, _ string) ([]n4j.DeadParagraphInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetDeadCodeSummary(_ context.Context) ([]n4j.DeadCodeSummaryInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListJCLJobs(_ context.Context) ([]n4j.JCLJobInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetJCLJob(_ context.Context, _ string) (*n4j.JCLJobDetail, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramJCL(_ context.Context, _ string) (*n4j.ProgramJCLInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetDatasetUsage(_ context.Context, _ string) ([]n4j.DatasetUsageInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListDBTables(_ context.Context) ([]n4j.DBTableInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetTableUsage(_ context.Context, _ string) (*n4j.TableUsageInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetProgramTableAccess(_ context.Context, _ string) (*n4j.ProgramTableAccessInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetCrossProgramDataFlow(_ context.Context, _ string) ([]n4j.CrossProgramFlowInfo, error) {
	return nil, m.Err
}

func (m *MockReader) TraceFieldImpact(_ context.Context, _ string, _ string) ([]n4j.FieldImpactInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetSharedDataChannels(_ context.Context) ([]n4j.SharedDataChannelInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetValidationReport(_ context.Context) (*n4j.ValidationResult, error) {
	return nil, m.Err
}
