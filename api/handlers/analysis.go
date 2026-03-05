package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AnalysisHandler handles impact analysis and domain endpoints.
type AnalysisHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// ImpactAnalysis returns the blast radius for a program.
// @Summary Impact analysis
// @Tags analysis
// @Param id path string true "Program ID"
// @Success 200 {object} neo4j.ImpactResult
// @Router /api/v1/programs/{id}/impact [get]
func (h *AnalysisHandler) ImpactAnalysis(c *gin.Context) {
	programID := c.Param("id")

	result, err := h.Reader.GetImpactAnalysis(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("impact analysis", zap.Error(err))
		middleware.InternalError(c, "failed to get impact analysis")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ListDomains returns all business domains with program counts.
// @Summary List business domains
// @Tags analysis
// @Success 200 {array} neo4j.BusinessDomainSummary
// @Router /api/v1/domains [get]
func (h *AnalysisHandler) ListDomains(c *gin.Context) {
	domains, err := h.Reader.ListBusinessDomains(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing domains", zap.Error(err))
		middleware.InternalError(c, "failed to list domains")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": domains})
}

// GetDomain returns a business domain with its member programs.
// @Summary Get business domain
// @Tags analysis
// @Param name path string true "Domain name"
// @Success 200 {object} neo4j.BusinessDomainDetail
// @Router /api/v1/domains/{name} [get]
func (h *AnalysisHandler) GetDomain(c *gin.Context) {
	name := c.Param("name")

	detail, err := h.Reader.GetBusinessDomain(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting domain", zap.Error(err))
		middleware.InternalError(c, "failed to get domain")
		return
	}
	if detail == nil {
		middleware.NotFound(c, "domain not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}
