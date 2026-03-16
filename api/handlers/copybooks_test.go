package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cobol-ingestor/api/handlers"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupCopybookRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.CopybookHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/copybooks/:name/structure", h.Structure)
	return r
}

func TestCopybookStructure(t *testing.T) {
	mock := &MockReader{
		DataItems: []n4j.DataItemInfo{
			{Name: "CUST-REC", Level: 1, FQN: "CUSTREC.01.CUST-REC"},
		},
	}
	r := setupCopybookRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/copybooks/CUSTREC/structure", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUST-REC")
}
