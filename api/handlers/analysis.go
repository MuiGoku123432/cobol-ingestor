package handlers

import (
	"net/http"
	"strconv"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AnalysisHandler handles impact analysis and domain endpoints.
type AnalysisHandler struct {
	Reader n4j.Reader
	Writer *n4j.BatchWriter
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

// BridgePrograms returns programs that connect multiple business domains.
func (h *AnalysisHandler) BridgePrograms(c *gin.Context) {
	items, err := h.Reader.ListBridgePrograms(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing bridge programs", zap.Error(err))
		middleware.InternalError(c, "failed to list bridge programs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CopybookRisks returns copybooks with risk assessments.
func (h *AnalysisHandler) CopybookRisks(c *gin.Context) {
	items, err := h.Reader.ListCopybookRisks(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing copybook risks", zap.Error(err))
		middleware.InternalError(c, "failed to list copybook risks")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ModernizationCandidates returns programs scored for modernization.
func (h *AnalysisHandler) ModernizationCandidates(c *gin.Context) {
	items, err := h.Reader.ListModernizationCandidates(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing modernization candidates", zap.Error(err))
		middleware.InternalError(c, "failed to list modernization candidates")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// RiskPrograms returns programs above a minimum risk score.
func (h *AnalysisHandler) RiskPrograms(c *gin.Context) {
	minScore, _ := strconv.ParseFloat(c.DefaultQuery("minScore", "0.5"), 64)
	items, err := h.Reader.ListRiskPrograms(c.Request.Context(), minScore)
	if err != nil {
		h.Logger.Error("listing risk programs", zap.Error(err))
		middleware.InternalError(c, "failed to list risk programs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// VolumeEstimates returns transaction volume estimates for all programs.
func (h *AnalysisHandler) VolumeEstimates(c *gin.Context) {
	items, err := h.Reader.ListVolumeEstimates(c.Request.Context())
	if err != nil {
		h.Logger.Error("listing volume estimates", zap.Error(err))
		middleware.InternalError(c, "failed to list volume estimates")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// DeadCodeSummary returns a summary of dead code across all programs.
func (h *AnalysisHandler) DeadCodeSummary(c *gin.Context) {
	items, err := h.Reader.GetDeadCodeSummary(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting dead code summary", zap.Error(err))
		middleware.InternalError(c, "failed to get dead code summary")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// MigrationSequence returns the recommended migration order.
func (h *AnalysisHandler) MigrationSequence(c *gin.Context) {
	items, err := h.Reader.GetMigrationSequence(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting migration sequence", zap.Error(err))
		middleware.InternalError(c, "failed to get migration sequence")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// FieldImpact traces the impact of a specific field across programs.
func (h *AnalysisHandler) FieldImpact(c *gin.Context) {
	programID := c.Param("programId")
	field := c.Param("field")
	items, err := h.Reader.TraceFieldImpact(c.Request.Context(), programID, field)
	if err != nil {
		h.Logger.Error("tracing field impact", zap.Error(err))
		middleware.InternalError(c, "failed to trace field impact")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// SharedDataChannels returns shared data channels between programs.
func (h *AnalysisHandler) SharedDataChannels(c *gin.Context) {
	items, err := h.Reader.GetSharedDataChannels(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting shared data channels", zap.Error(err))
		middleware.InternalError(c, "failed to get shared data channels")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// FileAccessors returns programs that access a specific file.
func (h *AnalysisHandler) FileAccessors(c *gin.Context) {
	name := c.Param("name")
	result, err := h.Reader.GetFileAccessors(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting file accessors", zap.Error(err))
		middleware.InternalError(c, "failed to get file accessors")
		return
	}
	if result == nil {
		middleware.NotFound(c, "file not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// IDMSImpact returns the impact analysis for an IDMS record.
func (h *AnalysisHandler) IDMSImpact(c *gin.Context) {
	record := c.Param("record")
	result, err := h.Reader.GetIDMSImpact(c.Request.Context(), record)
	if err != nil {
		h.Logger.Error("getting IDMS impact", zap.Error(err))
		middleware.InternalError(c, "failed to get IDMS impact")
		return
	}
	if result == nil {
		middleware.NotFound(c, "IDMS record not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ValidationReport returns the graph validation report.
func (h *AnalysisHandler) ValidationReport(c *gin.Context) {
	result, err := h.Reader.GetValidationReport(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting validation report", zap.Error(err))
		middleware.InternalError(c, "failed to get validation report")
		return
	}
	if result == nil {
		middleware.NotFound(c, "validation report not available")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ReassignDomain reassigns a program to a new business domain.
func (h *AnalysisHandler) ReassignDomain(c *gin.Context) {
	newDomain := c.Param("name")

	var body struct {
		ProgramID string `json:"programId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "programId is required"})
		return
	}

	if h.Writer == nil {
		middleware.InternalError(c, "write operations not available")
		return
	}

	if err := h.Writer.ReassignProgramDomain(c.Request.Context(), body.ProgramID, newDomain); err != nil {
		h.Logger.Error("reassigning domain", zap.Error(err))
		middleware.InternalError(c, "failed to reassign domain")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "programId": body.ProgramID, "domain": newDomain})
}
