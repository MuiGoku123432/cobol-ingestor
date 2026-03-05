package handlers

import (
	"net/http"
	"strconv"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SearchHandler handles full-text search endpoints.
type SearchHandler struct {
	Reader n4j.Reader
	Logger *zap.Logger
}

// Search performs a full-text search across programs and paragraphs.
// @Summary Full-text search
// @Tags search
// @Param q query string true "Search query"
// @Param limit query int false "Max results" default(20)
// @Success 200 {array} neo4j.SearchResult
// @Router /api/v1/search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		middleware.BadRequest(c, "query parameter 'q' is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	results, err := h.Reader.SearchFullText(c.Request.Context(), query, limit)
	if err != nil {
		h.Logger.Error("search failed", zap.Error(err))
		middleware.InternalError(c, "search failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results, "query": query})
}
