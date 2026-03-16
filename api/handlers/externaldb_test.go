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

func setupExternalDBRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.ExternalDBHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/external-db/tables", h.ListTables)
	r.GET("/api/v1/external-db/mappings/:table", h.Mapping)
	r.GET("/api/v1/external-db/gap-analysis", h.GapAnalysis)
	r.GET("/api/v1/external-db/data-flow-paths", h.DataFlowPaths)
	return r
}

func TestListExternalDBTables(t *testing.T) {
	mock := &MockReader{
		ExternalDBTables: []n4j.ExternalDBTableInfo{
			{Name: "customers", DatabaseName: "postgres"},
		},
	}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/tables", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "customers")
}

func TestMapping_CobolDirection(t *testing.T) {
	mock := &MockReader{
		CobolToExtMappings: []n4j.ExternalDBMappingInfo{
			{CobolTable: "CUSTOMER", ExternalTable: "customers", Confidence: 0.95},
		},
	}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/mappings/CUSTOMER", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "customers")
}

func TestMapping_ExternalDirection(t *testing.T) {
	mock := &MockReader{
		ExternalDBMapping: &n4j.ExternalDBMappingInfo{
			CobolTable: "CUSTOMER", ExternalTable: "customers", Confidence: 0.95,
		},
	}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/mappings/customers?direction=external", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER")
}

func TestMapping_ExternalDirection_NotFound(t *testing.T) {
	mock := &MockReader{ExternalDBMapping: nil}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/mappings/nope?direction=external", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGapAnalysis(t *testing.T) {
	mock := &MockReader{
		GapInfoItems: []n4j.GapInfo{
			{Side: "cobol", TableName: "CUSTOMER", Description: "no external mapping"},
		},
	}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/gap-analysis", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER")
}

func TestDataFlowPaths(t *testing.T) {
	mock := &MockReader{
		DataFlowPathItems: []n4j.DataFlowPathInfo{
			{CobolProgram: "PROG1", Operation: "SELECT", DB2Table: "CUSTOMER"},
		},
	}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/data-flow-paths?table=CUSTOMER", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG1")
}

func TestDataFlowPaths_MissingTable(t *testing.T) {
	mock := &MockReader{}
	r := setupExternalDBRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/external-db/data-flow-paths", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
