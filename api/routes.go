package api

import (
	"cobol-ingestor/api/handlers"
	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/pipeline"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RegisterRoutes sets up all API routes on the Gin engine.
func RegisterRoutes(r *gin.Engine, reader n4j.Reader, writer *n4j.BatchWriter, pipe *pipeline.Pipeline, logger *zap.Logger) {
	programH := &handlers.ProgramHandler{Reader: reader, Logger: logger}
	copybookH := &handlers.CopybookHandler{Reader: reader, Logger: logger}
	analysisH := &handlers.AnalysisHandler{Reader: reader, Writer: writer, Logger: logger}
	searchH := &handlers.SearchHandler{Reader: reader, Logger: logger}
	dashboardH := &handlers.DashboardHandler{Reader: reader, Logger: logger}
	jobH := &handlers.JobHandler{Reader: reader, Logger: logger, Pipeline: pipe}
	jclH := &handlers.JCLHandler{Reader: reader, Logger: logger}
	dbH := &handlers.DatabaseHandler{Reader: reader, Logger: logger}
	extDBH := &handlers.ExternalDBHandler{Reader: reader, Logger: logger}

	r.Use(middleware.ZapLogger(logger))

	v1 := r.Group("/api/v1")
	{
		// Programs
		v1.GET("/programs", programH.List)
		v1.GET("/programs/:id", programH.Get)
		v1.GET("/programs/:id/call-chain", programH.CallChain)
		v1.GET("/programs/:id/data-items", programH.DataItems)
		v1.GET("/programs/:id/conditions", programH.Conditions)
		v1.GET("/programs/:id/parameters", programH.Parameters)
		v1.GET("/programs/:id/conditional-logic", programH.ConditionalLogic)
		v1.GET("/programs/:id/error-handlers", programH.ErrorHandlers)
		v1.GET("/programs/:id/external-interfaces", programH.ExternalInterfaces)
		v1.GET("/programs/:id/impact", analysisH.ImpactAnalysis)
		v1.GET("/programs/:id/sql", programH.SQL)
		v1.GET("/programs/:id/cics", programH.CICS)
		v1.GET("/programs/:id/paragraph-flow", programH.ParagraphFlow)
		v1.GET("/programs/:id/data-flow", programH.DataFlow)
		v1.GET("/programs/:id/data-hierarchy", programH.DataHierarchy)
		v1.GET("/programs/:id/dead-paragraphs", programH.DeadParagraphs)
		v1.GET("/programs/:id/source", programH.Source)
		v1.GET("/programs/:id/jcl", programH.JCL)
		v1.GET("/programs/:id/table-access", programH.TableAccess)
		v1.GET("/programs/:id/cross-program-flow", programH.CrossProgramFlow)
		v1.GET("/programs/:id/effort-estimate", programH.EffortEstimate)
		v1.GET("/programs/:id/idms/records", programH.IDMSRecords)
		v1.GET("/programs/:id/idms/schema", programH.IDMSSchema)
		v1.GET("/programs/:id/idms/areas", programH.IDMSAreas)

		// Copybooks
		v1.GET("/copybooks", copybookH.List)
		v1.GET("/copybooks/:name/usage", copybookH.Usage)
		v1.GET("/copybooks/:name/structure", copybookH.Structure)

		// Domains
		v1.GET("/domains", analysisH.ListDomains)
		v1.GET("/domains/:name", analysisH.GetDomain)
		v1.POST("/domains/:name/reassign", analysisH.ReassignDomain)

		// Analysis
		v1.GET("/analysis/bridge-programs", analysisH.BridgePrograms)
		v1.GET("/analysis/copybook-risks", analysisH.CopybookRisks)
		v1.GET("/analysis/modernization-candidates", analysisH.ModernizationCandidates)
		v1.GET("/analysis/risk-programs", analysisH.RiskPrograms)
		v1.GET("/analysis/volume-estimates", analysisH.VolumeEstimates)
		v1.GET("/analysis/dead-code-summary", analysisH.DeadCodeSummary)
		v1.GET("/analysis/migration-sequence", analysisH.MigrationSequence)
		v1.GET("/analysis/field-impact/:programId/:field", analysisH.FieldImpact)
		v1.GET("/analysis/shared-data-channels", analysisH.SharedDataChannels)
		v1.GET("/analysis/file-accessors/:name", analysisH.FileAccessors)
		v1.GET("/analysis/idms-impact/:record", analysisH.IDMSImpact)
		v1.GET("/analysis/validation-report", analysisH.ValidationReport)

		// Search
		v1.GET("/search", searchH.Search)

		// Dashboard
		v1.GET("/dashboard/stats", dashboardH.Stats)

		// Jobs
		v1.POST("/jobs/ingest", jobH.Trigger)
		v1.GET("/jobs/:id", jobH.Status)

		// JCL
		v1.GET("/jcl/jobs", jclH.ListJobs)
		v1.GET("/jcl/jobs/:id", jclH.GetJob)
		v1.GET("/jcl/datasets/:name/usage", jclH.DatasetUsage)

		// Database
		v1.GET("/db/tables", dbH.ListTables)
		v1.GET("/db/tables/:name/usage", dbH.TableUsage)

		// External DB
		v1.GET("/external-db/tables", extDBH.ListTables)
		v1.GET("/external-db/mappings/:table", extDBH.Mapping)
		v1.GET("/external-db/gap-analysis", extDBH.GapAnalysis)
		v1.GET("/external-db/data-flow-paths", extDBH.DataFlowPaths)
	}
}
