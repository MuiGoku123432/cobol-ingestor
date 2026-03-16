package handlers

import (
	"net/http"
	"strconv"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProgramHandler handles program-related API endpoints.
type ProgramHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// List returns a paginated list of programs.
// @Summary List programs
// @Tags programs
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param search query string false "Search filter"
// @Success 200 {object} neo4j.PagedResponse
// @Router /api/v1/programs [get]
func (h *ProgramHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	filter := n4j.Filter{Search: c.Query("search")}

	result, err := h.Reader.ListPrograms(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.Logger.Error("listing programs", zap.Error(err))
		middleware.InternalError(c, "failed to list programs")
		return
	}

	c.JSON(http.StatusOK, result)
}

// Get returns a program's full details.
// @Summary Get program details
// @Tags programs
// @Param id path string true "Program ID"
// @Success 200 {object} neo4j.ProgramDetail
// @Router /api/v1/programs/{id} [get]
func (h *ProgramHandler) Get(c *gin.Context) {
	programID := c.Param("id")

	detail, err := h.Reader.GetProgram(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting program", zap.Error(err))
		middleware.InternalError(c, "failed to get program")
		return
	}
	if detail == nil {
		middleware.NotFound(c, "program not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// CallChain returns the call chain for a program.
// @Summary Get call chain
// @Tags programs
// @Param id path string true "Program ID"
// @Param direction query string false "Direction: upstream or downstream" default(downstream)
// @Param depth query int false "Max depth" default(3)
// @Success 200 {array} neo4j.CallChainNode
// @Router /api/v1/programs/{id}/call-chain [get]
func (h *ProgramHandler) CallChain(c *gin.Context) {
	programID := c.Param("id")
	direction := c.DefaultQuery("direction", "downstream")
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "3"))

	nodes, err := h.Reader.GetCallChain(c.Request.Context(), programID, direction, depth)
	if err != nil {
		h.Logger.Error("getting call chain", zap.Error(err))
		middleware.InternalError(c, "failed to get call chain")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": nodes, "programId": programID, "direction": direction})
}

// DataItems returns data items for a program.
// @Summary Get program data items
// @Tags programs
// @Param id path string true "Program ID"
// @Success 200 {array} neo4j.DataItemInfo
// @Router /api/v1/programs/{id}/data-items [get]
func (h *ProgramHandler) DataItems(c *gin.Context) {
	programID := c.Param("id")

	items, err := h.Reader.GetDataItems(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting data items", zap.Error(err))
		middleware.InternalError(c, "failed to get data items")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Conditions returns 88-level conditions for a program.
func (h *ProgramHandler) Conditions(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramConditions(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting conditions", zap.Error(err))
		middleware.InternalError(c, "failed to get conditions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Parameters returns LINKAGE SECTION parameters for a program.
func (h *ProgramHandler) Parameters(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramParameters(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting parameters", zap.Error(err))
		middleware.InternalError(c, "failed to get parameters")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ConditionalLogic returns conditional logic from paragraph nodes.
func (h *ProgramHandler) ConditionalLogic(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramConditionalLogic(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting conditional logic", zap.Error(err))
		middleware.InternalError(c, "failed to get conditional logic")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ErrorHandlers returns error handling patterns from paragraph nodes.
func (h *ProgramHandler) ErrorHandlers(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramErrorHandlers(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting error handlers", zap.Error(err))
		middleware.InternalError(c, "failed to get error handlers")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ExternalInterfaces returns external interface points for a program.
func (h *ProgramHandler) ExternalInterfaces(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramExternalInterfaces(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting external interfaces", zap.Error(err))
		middleware.InternalError(c, "failed to get external interfaces")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// SQL returns SQL statements for a program.
func (h *ProgramHandler) SQL(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramSQL(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting SQL statements", zap.Error(err))
		middleware.InternalError(c, "failed to get SQL statements")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CICS returns CICS transactions for a program.
func (h *ProgramHandler) CICS(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetProgramCICS(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting CICS transactions", zap.Error(err))
		middleware.InternalError(c, "failed to get CICS transactions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ParagraphFlow returns paragraph execution flow for a program.
func (h *ProgramHandler) ParagraphFlow(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetParagraphFlow(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting paragraph flow", zap.Error(err))
		middleware.InternalError(c, "failed to get paragraph flow")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// DataFlow returns data flow information for a program.
func (h *ProgramHandler) DataFlow(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetDataFlow(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting data flow", zap.Error(err))
		middleware.InternalError(c, "failed to get data flow")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// DataHierarchy returns data hierarchy for a program.
func (h *ProgramHandler) DataHierarchy(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetDataHierarchy(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting data hierarchy", zap.Error(err))
		middleware.InternalError(c, "failed to get data hierarchy")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// DeadParagraphs returns dead paragraphs for a program.
func (h *ProgramHandler) DeadParagraphs(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetDeadParagraphs(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting dead paragraphs", zap.Error(err))
		middleware.InternalError(c, "failed to get dead paragraphs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Source returns the source code for a program.
func (h *ProgramHandler) Source(c *gin.Context) {
	programID := c.Param("id")
	result, err := h.Reader.GetProgramSource(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting program source", zap.Error(err))
		middleware.InternalError(c, "failed to get program source")
		return
	}
	if result == nil {
		middleware.NotFound(c, "program source not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// JCL returns JCL information for a program.
func (h *ProgramHandler) JCL(c *gin.Context) {
	programID := c.Param("id")
	result, err := h.Reader.GetProgramJCL(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting program JCL", zap.Error(err))
		middleware.InternalError(c, "failed to get program JCL")
		return
	}
	if result == nil {
		middleware.NotFound(c, "program JCL not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// TableAccess returns table access information for a program.
func (h *ProgramHandler) TableAccess(c *gin.Context) {
	programID := c.Param("id")
	result, err := h.Reader.GetProgramTableAccess(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting table access", zap.Error(err))
		middleware.InternalError(c, "failed to get table access")
		return
	}
	if result == nil {
		middleware.NotFound(c, "table access not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// CrossProgramFlow returns cross-program data flow for a program.
func (h *ProgramHandler) CrossProgramFlow(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetCrossProgramDataFlow(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting cross-program data flow", zap.Error(err))
		middleware.InternalError(c, "failed to get cross-program data flow")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// EffortEstimate returns the effort estimate for a specific program.
func (h *ProgramHandler) EffortEstimate(c *gin.Context) {
	programID := c.Param("id")
	all, err := h.Reader.GetEffortEstimates(c.Request.Context())
	if err != nil {
		h.Logger.Error("getting effort estimates", zap.Error(err))
		middleware.InternalError(c, "failed to get effort estimates")
		return
	}
	for _, e := range all {
		if e.ProgramID == programID {
			c.JSON(http.StatusOK, gin.H{"data": e})
			return
		}
	}
	middleware.NotFound(c, "effort estimate not found")
}

// IDMSRecords returns IDMS records for a program.
func (h *ProgramHandler) IDMSRecords(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetIDMSRecords(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting IDMS records", zap.Error(err))
		middleware.InternalError(c, "failed to get IDMS records")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// IDMSSchema returns the IDMS schema for a program.
func (h *ProgramHandler) IDMSSchema(c *gin.Context) {
	programID := c.Param("id")
	result, err := h.Reader.GetIDMSSchema(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting IDMS schema", zap.Error(err))
		middleware.InternalError(c, "failed to get IDMS schema")
		return
	}
	if result == nil {
		middleware.NotFound(c, "IDMS schema not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// IDMSAreas returns IDMS areas for a program.
func (h *ProgramHandler) IDMSAreas(c *gin.Context) {
	programID := c.Param("id")
	items, err := h.Reader.GetIDMSAreas(c.Request.Context(), programID)
	if err != nil {
		h.Logger.Error("getting IDMS areas", zap.Error(err))
		middleware.InternalError(c, "failed to get IDMS areas")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func parsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}
