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

	// New fields for expanded endpoints
	SQLStatements       []n4j.SQLStatementInfo
	CICSTransactions    []n4j.CICSTransactionInfo
	ParagraphFlowItems  []n4j.ParagraphFlowInfo
	DataFlowItems       []n4j.DataFlowInfo
	DataHierarchyItems  []n4j.DataHierarchyInfo
	DeadParagraphItems  []n4j.DeadParagraphInfo
	ProgramSourceResult *n4j.ProgramSourceInfo
	ProgramJCLResult    *n4j.ProgramJCLInfo
	TableAccessResult   *n4j.ProgramTableAccessInfo
	CrossProgramFlows   []n4j.CrossProgramFlowInfo
	EffortEstimates     []n4j.EffortEstimate
	IDMSRecordItems     []n4j.IDMSRecordInfo
	IDMSSchemaResult    *n4j.IDMSSchemaInfo
	IDMSAreaItems       []n4j.IDMSAreaInfo
	DeadCodeSummaries   []n4j.DeadCodeSummaryInfo
	MigrationSteps      []n4j.MigrationStep
	FieldImpacts        []n4j.FieldImpactInfo
	SharedChannels      []n4j.SharedDataChannelInfo
	FileAccessResult    *n4j.FileAccessInfo
	IDMSImpactResult    *n4j.IDMSImpactInfo
	ValidationResult    *n4j.ValidationResult
	JCLJobs             []n4j.JCLJobInfo
	JCLJobResult        *n4j.JCLJobDetail
	DatasetUsages       []n4j.DatasetUsageInfo
	DBTables            []n4j.DBTableInfo
	TableUsageResult    *n4j.TableUsageInfo
	ExternalDBTables    []n4j.ExternalDBTableInfo
	ExternalDBMapping   *n4j.ExternalDBMappingInfo
	CobolToExtMappings  []n4j.ExternalDBMappingInfo
	GapInfoItems        []n4j.GapInfo
	DataFlowPathItems   []n4j.DataFlowPathInfo

	Err error
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
	return m.SQLStatements, m.Err
}

func (m *MockReader) GetProgramCICS(_ context.Context, _ string) ([]n4j.CICSTransactionInfo, error) {
	return m.CICSTransactions, m.Err
}

func (m *MockReader) GetParagraphFlow(_ context.Context, _ string) ([]n4j.ParagraphFlowInfo, error) {
	return m.ParagraphFlowItems, m.Err
}

func (m *MockReader) GetDataFlow(_ context.Context, _ string) ([]n4j.DataFlowInfo, error) {
	return m.DataFlowItems, m.Err
}

func (m *MockReader) GetDataHierarchy(_ context.Context, _ string) ([]n4j.DataHierarchyInfo, error) {
	return m.DataHierarchyItems, m.Err
}

func (m *MockReader) GetDeadParagraphs(_ context.Context, _ string) ([]n4j.DeadParagraphInfo, error) {
	return m.DeadParagraphItems, m.Err
}

func (m *MockReader) GetDeadCodeSummary(_ context.Context) ([]n4j.DeadCodeSummaryInfo, error) {
	return m.DeadCodeSummaries, m.Err
}

func (m *MockReader) ListJCLJobs(_ context.Context) ([]n4j.JCLJobInfo, error) {
	return m.JCLJobs, m.Err
}

func (m *MockReader) GetJCLJob(_ context.Context, _ string) (*n4j.JCLJobDetail, error) {
	return m.JCLJobResult, m.Err
}

func (m *MockReader) GetProgramJCL(_ context.Context, _ string) (*n4j.ProgramJCLInfo, error) {
	return m.ProgramJCLResult, m.Err
}

func (m *MockReader) GetDatasetUsage(_ context.Context, _ string) ([]n4j.DatasetUsageInfo, error) {
	return m.DatasetUsages, m.Err
}

func (m *MockReader) ListDBTables(_ context.Context) ([]n4j.DBTableInfo, error) {
	return m.DBTables, m.Err
}

func (m *MockReader) GetTableUsage(_ context.Context, _ string) (*n4j.TableUsageInfo, error) {
	return m.TableUsageResult, m.Err
}

func (m *MockReader) GetProgramTableAccess(_ context.Context, _ string) (*n4j.ProgramTableAccessInfo, error) {
	return m.TableAccessResult, m.Err
}

func (m *MockReader) GetCrossProgramDataFlow(_ context.Context, _ string) ([]n4j.CrossProgramFlowInfo, error) {
	return m.CrossProgramFlows, m.Err
}

func (m *MockReader) TraceFieldImpact(_ context.Context, _ string, _ string) ([]n4j.FieldImpactInfo, error) {
	return m.FieldImpacts, m.Err
}

func (m *MockReader) GetSharedDataChannels(_ context.Context) ([]n4j.SharedDataChannelInfo, error) {
	return m.SharedChannels, m.Err
}

func (m *MockReader) GetValidationReport(_ context.Context) (*n4j.ValidationResult, error) {
	return m.ValidationResult, m.Err
}

func (m *MockReader) GetCopybookStructure(_ context.Context, _ string) ([]n4j.DataItemInfo, error) {
	return m.DataItems, m.Err
}

func (m *MockReader) GetProgramSource(_ context.Context, _ string) (*n4j.ProgramSourceInfo, error) {
	return m.ProgramSourceResult, m.Err
}

func (m *MockReader) GetMigrationSequence(_ context.Context) ([]n4j.MigrationStep, error) {
	return m.MigrationSteps, m.Err
}

func (m *MockReader) GetFileAccessors(_ context.Context, _ string) (*n4j.FileAccessInfo, error) {
	return m.FileAccessResult, m.Err
}

func (m *MockReader) GetEffortEstimates(_ context.Context) ([]n4j.EffortEstimate, error) {
	return m.EffortEstimates, m.Err
}

func (m *MockReader) GetIDMSRecords(_ context.Context, _ string) ([]n4j.IDMSRecordInfo, error) {
	return m.IDMSRecordItems, m.Err
}

func (m *MockReader) GetIDMSSchema(_ context.Context, _ string) (*n4j.IDMSSchemaInfo, error) {
	return m.IDMSSchemaResult, m.Err
}

func (m *MockReader) GetIDMSAreas(_ context.Context, _ string) ([]n4j.IDMSAreaInfo, error) {
	return m.IDMSAreaItems, m.Err
}

func (m *MockReader) GetIDMSImpact(_ context.Context, _ string) (*n4j.IDMSImpactInfo, error) {
	return m.IDMSImpactResult, m.Err
}

func (m *MockReader) ListExternalDBTables(_ context.Context) ([]n4j.ExternalDBTableInfo, error) {
	return m.ExternalDBTables, m.Err
}

func (m *MockReader) GetExternalDBMapping(_ context.Context, _ string) (*n4j.ExternalDBMappingInfo, error) {
	return m.ExternalDBMapping, m.Err
}

func (m *MockReader) GetCobolToExternalMappings(_ context.Context, _ string) ([]n4j.ExternalDBMappingInfo, error) {
	return m.CobolToExtMappings, m.Err
}

func (m *MockReader) GetGapAnalysis(_ context.Context) ([]n4j.GapInfo, error) {
	return m.GapInfoItems, m.Err
}

func (m *MockReader) GetDataFlowPaths(_ context.Context, _ string) ([]n4j.DataFlowPathInfo, error) {
	return m.DataFlowPathItems, m.Err
}

func (m *MockReader) ListTargetRepos(_ context.Context) ([]n4j.TargetRepoInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListTargetServices(_ context.Context, _ string) ([]n4j.TargetServiceInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetTargetService(_ context.Context, _ string) (*n4j.TargetServiceDetail, error) {
	return nil, m.Err
}

func (m *MockReader) ListBusinessGaps(_ context.Context, _, _, _ string) ([]n4j.BusinessGapInfo, error) {
	return nil, m.Err
}

func (m *MockReader) ListBusinessRequirements(_ context.Context, _ string) ([]n4j.BusinessRequirementInfo, error) {
	return nil, m.Err
}

func (m *MockReader) GetGapCoverageSummary(_ context.Context) (*n4j.GapCoverageSummary, error) {
	return nil, m.Err
}

func (m *MockReader) GetTargetStackDashboard(_ context.Context) (map[string]any, error) {
	return nil, m.Err
}

func (m *MockReader) GetGlossaryTerm(_ context.Context, _, _ string) (*n4j.GlossaryTermDetail, error) {
	return nil, m.Err
}

func (m *MockReader) SearchGlossaryTerms(_ context.Context, _, _ string, _ int) ([]n4j.GlossaryTermDetail, error) {
	return nil, m.Err
}

func (m *MockReader) ListGlossaryTerms(_ context.Context, _ string, _, _ int) (*n4j.GlossaryTermPage, error) {
	return nil, m.Err
}
