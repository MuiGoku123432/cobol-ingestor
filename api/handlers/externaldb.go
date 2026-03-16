package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ExternalDBHandler handles external database mapping API endpoints.
type ExternalDBHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// ListTables returns all external database tables.
func (h *ExternalDBHandler) ListTables(c *gin.Context) {
	items, err := h.Reader.ListExternalDBTables(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing external DB tables", zap.Error(err))
		middleware.InternalError(c, "failed to list external DB tables")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Mapping returns COBOL-to-external or external-to-COBOL mappings for a table.
// By default returns COBOL→external mappings. Add ?direction=external for reverse.
func (h *ExternalDBHandler) Mapping(c *gin.Context) {
	table := c.Param("table")
	direction := c.DefaultQuery("direction", "cobol")

	if direction == "external" {
		result, err := h.Reader.GetExternalDBMapping(c.Request.Context(), table)
		if err != nil {
			h.Logger.Error("getting external DB mapping", zap.Error(err))
			middleware.InternalError(c, "failed to get external DB mapping")
			return
		}
		if result == nil {
			middleware.NotFound(c, "external DB mapping not found")
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": result})
		return
	}

	items, err := h.Reader.GetCobolToExternalMappings(c.Request.Context(), table)
	if err != nil {
		h.Logger.Error("getting COBOL to external mappings", zap.Error(err))
		middleware.InternalError(c, "failed to get COBOL to external mappings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GapAnalysis returns the external DB gap analysis.
func (h *ExternalDBHandler) GapAnalysis(c *gin.Context) {
	items, err := h.Reader.GetGapAnalysis(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting gap analysis", zap.Error(err))
		middleware.InternalError(c, "failed to get gap analysis")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// DataFlowPaths returns data flow paths for a table.
func (h *ExternalDBHandler) DataFlowPaths(c *gin.Context) {
	table := c.Query("table")
	if table == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "table query parameter is required"})
		return
	}
	items, err := h.Reader.GetDataFlowPaths(c.Request.Context(), table)
	if err != nil {
		h.Logger.Error("getting data flow paths", zap.Error(err))
		middleware.InternalError(c, "failed to get data flow paths")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
