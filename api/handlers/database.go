package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DatabaseHandler handles database-related API endpoints.
type DatabaseHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// ListTables returns all database tables.
func (h *DatabaseHandler) ListTables(c *gin.Context) {
	items, err := h.Reader.ListDBTables(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing DB tables", zap.Error(err))
		middleware.InternalError(c, "failed to list DB tables")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// TableUsage returns usage information for a specific table.
func (h *DatabaseHandler) TableUsage(c *gin.Context) {
	name := c.Param("name")
	result, err := h.Reader.GetTableUsage(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting table usage", zap.Error(err))
		middleware.InternalError(c, "failed to get table usage")
		return
	}
	if result == nil {
		middleware.NotFound(c, "table not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
