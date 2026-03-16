package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JCLHandler handles JCL-related API endpoints.
type JCLHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// ListJobs returns all JCL jobs.
func (h *JCLHandler) ListJobs(c *gin.Context) {
	items, err := h.Reader.ListJCLJobs(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing JCL jobs", zap.Error(err))
		middleware.InternalError(c, "failed to list JCL jobs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetJob returns details for a specific JCL job.
func (h *JCLHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	result, err := h.Reader.GetJCLJob(c.Request.Context(), id)
	if err != nil {
		h.Logger.Error("getting JCL job", zap.Error(err))
		middleware.InternalError(c, "failed to get JCL job")
		return
	}
	if result == nil {
		middleware.NotFound(c, "JCL job not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// DatasetUsage returns usage information for a dataset.
func (h *JCLHandler) DatasetUsage(c *gin.Context) {
	name := c.Param("name")
	items, err := h.Reader.GetDatasetUsage(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting dataset usage", zap.Error(err))
		middleware.InternalError(c, "failed to get dataset usage")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
