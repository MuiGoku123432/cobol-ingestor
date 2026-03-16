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
	r.GET("/api/v1/programs/:id/sql", h.SQL)
	r.GET("/api/v1/programs/:id/cics", h.CICS)
	r.GET("/api/v1/programs/:id/paragraph-flow", h.ParagraphFlow)
	r.GET("/api/v1/programs/:id/data-flow", h.DataFlow)
	r.GET("/api/v1/programs/:id/data-hierarchy", h.DataHierarchy)
	r.GET("/api/v1/programs/:id/dead-paragraphs", h.DeadParagraphs)
	r.GET("/api/v1/programs/:id/source", h.Source)
	r.GET("/api/v1/programs/:id/jcl", h.JCL)
	r.GET("/api/v1/programs/:id/table-access", h.TableAccess)
	r.GET("/api/v1/programs/:id/cross-program-flow", h.CrossProgramFlow)
	r.GET("/api/v1/programs/:id/effort-estimate", h.EffortEstimate)
	r.GET("/api/v1/programs/:id/idms/records", h.IDMSRecords)
	r.GET("/api/v1/programs/:id/idms/schema", h.IDMSSchema)
	r.GET("/api/v1/programs/:id/idms/areas", h.IDMSAreas)
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

func TestSQL(t *testing.T) {
	mock := &MockReader{
		SQLStatements: []n4j.SQLStatementInfo{
			{ID: "sql1", Text: "SELECT * FROM CUSTOMER", Type: "SELECT"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/sql", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SELECT * FROM CUSTOMER")
}

func TestCICS(t *testing.T) {
	mock := &MockReader{
		CICSTransactions: []n4j.CICSTransactionInfo{
			{ID: "cics1", Command: "SEND MAP"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/cics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SEND MAP")
}

func TestParagraphFlow(t *testing.T) {
	mock := &MockReader{
		ParagraphFlowItems: []n4j.ParagraphFlowInfo{
			{FromParagraph: "MAIN-LOGIC", ToParagraph: "PROCESS-RECORD", Type: "PERFORM"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/paragraph-flow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "MAIN-LOGIC")
}

func TestDataFlow(t *testing.T) {
	mock := &MockReader{
		DataFlowItems: []n4j.DataFlowInfo{
			{FromItem: "WS-INPUT", ToItem: "WS-OUTPUT"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/data-flow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "WS-INPUT")
}

func TestDataHierarchy(t *testing.T) {
	mock := &MockReader{
		DataHierarchyItems: []n4j.DataHierarchyInfo{
			{Name: "WS-RECORD"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/data-hierarchy", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "WS-RECORD")
}

func TestDeadParagraphs(t *testing.T) {
	mock := &MockReader{
		DeadParagraphItems: []n4j.DeadParagraphInfo{
			{Name: "UNUSED-PARA", ProgramID: "PROG1", Reason: "never performed"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/dead-paragraphs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "UNUSED-PARA")
}

func TestSource_Found(t *testing.T) {
	mock := &MockReader{
		ProgramSourceResult: &n4j.ProgramSourceInfo{
			ProgramID: "PROG1",
			FilePath:  "/src/PROG1.cbl",
			Source:    "IDENTIFICATION DIVISION.",
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/source", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "IDENTIFICATION DIVISION")
}

func TestSource_NotFound(t *testing.T) {
	mock := &MockReader{ProgramSourceResult: nil}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NOPE/source", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestJCL_Found(t *testing.T) {
	mock := &MockReader{
		ProgramJCLResult: &n4j.ProgramJCLInfo{ProgramID: "PROG1"},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/jcl", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG1")
}

func TestJCL_NotFound(t *testing.T) {
	mock := &MockReader{ProgramJCLResult: nil}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NOPE/jcl", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTableAccess_Found(t *testing.T) {
	mock := &MockReader{
		TableAccessResult: &n4j.ProgramTableAccessInfo{ProgramID: "PROG1"},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/table-access", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTableAccess_NotFound(t *testing.T) {
	mock := &MockReader{TableAccessResult: nil}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NOPE/table-access", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCrossProgramFlow(t *testing.T) {
	mock := &MockReader{
		CrossProgramFlows: []n4j.CrossProgramFlowInfo{
			{ProgramID: "PROG1", OtherProgram: "PROG2", Channel: "LINKAGE", Direction: "outbound"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/cross-program-flow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG2")
}

func TestEffortEstimate_Found(t *testing.T) {
	mock := &MockReader{
		EffortEstimates: []n4j.EffortEstimate{
			{ProgramID: "PROG1", ParagraphCount: 10},
			{ProgramID: "PROG2", ParagraphCount: 5},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/effort-estimate", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG1")
}

func TestEffortEstimate_NotFound(t *testing.T) {
	mock := &MockReader{
		EffortEstimates: []n4j.EffortEstimate{
			{ProgramID: "OTHER"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NOPE/effort-estimate", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIDMSRecords(t *testing.T) {
	mock := &MockReader{
		IDMSRecordItems: []n4j.IDMSRecordInfo{
			{Name: "CUSTOMER-REC", ProgramID: "PROG1"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/idms/records", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER-REC")
}

func TestIDMSSchema_Found(t *testing.T) {
	mock := &MockReader{
		IDMSSchemaResult: &n4j.IDMSSchemaInfo{SchemaName: "CUSTDB"},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/idms/schema", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTDB")
}

func TestIDMSSchema_NotFound(t *testing.T) {
	mock := &MockReader{IDMSSchemaResult: nil}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/NOPE/idms/schema", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIDMSAreas(t *testing.T) {
	mock := &MockReader{
		IDMSAreaItems: []n4j.IDMSAreaInfo{
			{Name: "CUST-AREA"},
		},
	}
	r := setupProgramRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/idms/areas", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUST-AREA")
}
