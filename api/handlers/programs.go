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
