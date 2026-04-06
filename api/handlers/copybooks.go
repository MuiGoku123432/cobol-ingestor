package handlers

import (
	"net/http"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CopybookHandler handles copybook-related API endpoints.
type CopybookHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// List returns a paginated list of copybooks.
// @Summary List copybooks
// @Tags copybooks
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param search query string false "Search filter"
// @Success 200 {object} neo4j.PagedResponse
// @Router /api/v1/copybooks [get]
func (h *CopybookHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	filter := n4j.Filter{Search: c.Query("search"), Codebase: c.Query("codebase")}

	result, err := h.Reader.ListCopybooks(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.Logger.Error("listing copybooks", zap.Error(err))
		middleware.InternalError(c, "failed to list copybooks")
		return
	}

	c.JSON(http.StatusOK, result)
}

// Usage returns which programs include a copybook.
// @Summary Get copybook usage
// @Tags copybooks
// @Param name path string true "Copybook name"
// @Success 200 {object} neo4j.CopybookUsage
// @Router /api/v1/copybooks/{name}/usage [get]
func (h *CopybookHandler) Usage(c *gin.Context) {
	name := c.Param("name")

	usage, err := h.Reader.GetCopybookUsage(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting copybook usage", zap.Error(err))
		middleware.InternalError(c, "failed to get copybook usage")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": usage})
}

// Structure returns the data item structure of a copybook.
func (h *CopybookHandler) Structure(c *gin.Context) {
	name := c.Param("name")
	items, err := h.Reader.GetCopybookStructure(c.Request.Context(), name)
	if err != nil {
		h.Logger.Error("getting copybook structure", zap.Error(err))
		middleware.InternalError(c, "failed to get copybook structure")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
