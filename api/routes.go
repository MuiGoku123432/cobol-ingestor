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
func RegisterRoutes(r *gin.Engine, reader n4j.Reader, pipe *pipeline.Pipeline, logger *zap.Logger) {
	programH := &handlers.ProgramHandler{Reader: reader, Logger: logger}
	copybookH := &handlers.CopybookHandler{Reader: reader, Logger: logger}
	analysisH := &handlers.AnalysisHandler{Reader: reader, Logger: logger}
	searchH := &handlers.SearchHandler{Reader: reader, Logger: logger}
	dashboardH := &handlers.DashboardHandler{Reader: reader, Logger: logger}
	jobH := &handlers.JobHandler{Reader: reader, Logger: logger, Pipeline: pipe}

	r.Use(middleware.ZapLogger(logger))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/programs", programH.List)
		v1.GET("/programs/:id", programH.Get)
		v1.GET("/programs/:id/call-chain", programH.CallChain)
		v1.GET("/programs/:id/data-items", programH.DataItems)
		v1.GET("/programs/:id/impact", analysisH.ImpactAnalysis)

		v1.GET("/copybooks", copybookH.List)
		v1.GET("/copybooks/:name/usage", copybookH.Usage)

		v1.GET("/domains", analysisH.ListDomains)
		v1.GET("/domains/:name", analysisH.GetDomain)

		v1.GET("/search", searchH.Search)

		v1.GET("/dashboard/stats", dashboardH.Stats)

		v1.POST("/jobs/ingest", jobH.Trigger)
		v1.GET("/jobs/:id", jobH.Status)
	}
}
