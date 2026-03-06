package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cobol-ingestor/api/handlers"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupProgramRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.ProgramHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/programs", h.List)
	r.GET("/api/v1/programs/:id", h.Get)
	r.GET("/api/v1/programs/:id/call-chain", h.CallChain)
	r.GET("/api/v1/programs/:id/data-items", h.DataItems)
	return r
}

func TestListPrograms(t *testing.T) {
	mock := &MockReader{
		Programs: []n4j.ProgramSummary{
			{ProgramID: "PROG1", FilePath: "/src/PROG1.cbl", Language: "COBOL", CallCount: 3},
			{ProgramID: "PROG2", FilePath: "/src/PROG2.cbl", Language: "COBOL", CallCount: 1},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs?page=1&pageSize=10", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp n4j.PagedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, 1, resp.Page)
}

func TestGetProgram_Found(t *testing.T) {
	mock := &MockReader{
		ProgramDetail: &n4j.ProgramDetail{
			ProgramID: "CUSTMAINT",
			FilePath:  "/src/CUSTMAINT.cbl",
			Language:  "COBOL",
			Callers:   []n4j.CallInfo{{ProgramID: "MAIN"}},
			Callees:   []n4j.CallInfo{{ProgramID: "CUSTRPT"}},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/CUSTMAINT", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTMAINT")
}

func TestGetProgram_NotFound(t *testing.T) {
	mock := &MockReader{ProgramDetail: nil}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NONEXIST", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCallChain(t *testing.T) {
	mock := &MockReader{
		CallChainNodes: []n4j.CallChainNode{
			{ProgramID: "SUBRTN1", Depth: 1},
			{ProgramID: "SUBRTN2", Depth: 2},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/MAIN/call-chain?direction=downstream&depth=3", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SUBRTN1")
}

func TestDataItems(t *testing.T) {
	mock := &MockReader{
		DataItems: []n4j.DataItemInfo{
			{Name: "WS-CUSTOMER-ID", Level: 1, FQN: "PROG1.01.WS-CUSTOMER-ID"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/data-items", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "WS-CUSTOMER-ID")
}

func TestPagination_Defaults(t *testing.T) {
	mock := &MockReader{Programs: []n4j.ProgramSummary{}}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp n4j.PagedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
}
