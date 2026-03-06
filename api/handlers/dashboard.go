package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// Stats returns aggregate counts for the dashboard.
// @Summary Dashboard stats
// @Tags dashboard
// @Success 200 {object} neo4j.DashboardStats
// @Router /api/v1/dashboard/stats [get]
func (h *DashboardHandler) Stats(c *gin.Context) {
	stats, err := h.Reader.GetDashboardStats(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting dashboard stats", zap.Error(err))
		middleware.InternalError(c, "failed to get dashboard stats")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
