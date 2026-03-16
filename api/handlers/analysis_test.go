package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cobol-ingestor/api/handlers"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupAnalysisRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.AnalysisHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/programs/:id/impact", h.ImpactAnalysis)
	r.GET("/api/v1/domains", h.ListDomains)
	r.GET("/api/v1/domains/:name", h.GetDomain)
	r.POST("/api/v1/domains/:name/reassign", h.ReassignDomain)
	r.GET("/api/v1/analysis/dead-code-summary", h.DeadCodeSummary)
	r.GET("/api/v1/analysis/migration-sequence", h.MigrationSequence)
	r.GET("/api/v1/analysis/field-impact/:programId/:field", h.FieldImpact)
	r.GET("/api/v1/analysis/shared-data-channels", h.SharedDataChannels)
	r.GET("/api/v1/analysis/file-accessors/:name", h.FileAccessors)
	r.GET("/api/v1/analysis/idms-impact/:record", h.IDMSImpact)
	r.GET("/api/v1/analysis/validation-report", h.ValidationReport)
	return r
}

func TestImpactAnalysis(t *testing.T) {
	mock := &MockReader{
		Impact: &n4j.ImpactResult{
			ProgramID:          "PROG1",
			DownstreamPrograms: []string{"PROG2", "PROG3"},
			UpstreamPrograms:   []string{"MAIN"},
			SharedCopybooks:    []string{"CUSTREC"},
			TotalAffected:      3,
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/programs/PROG1/impact", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG2")
	assert.Contains(t, w.Body.String(), "CUSTREC")
}

func TestListDomains(t *testing.T) {
	mock := &MockReader{
		Domains: []n4j.BusinessDomainSummary{
			{Name: "Customer Management", ProgramCount: 5},
			{Name: "Batch Processing", ProgramCount: 3},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/domains", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Customer Management")
}

func TestGetDomain_Found(t *testing.T) {
	mock := &MockReader{
		DomainDetail: &n4j.BusinessDomainDetail{
			Name:     "Customer Management",
			Programs: []string{"CUSTMAINT", "CUSTINQ"},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/domains/Customer%20Management", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTMAINT")
}

func TestGetDomain_NotFound(t *testing.T) {
	mock := &MockReader{DomainDetail: nil}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/domains/Nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeadCodeSummary(t *testing.T) {
	mock := &MockReader{
		DeadCodeSummaries: []n4j.DeadCodeSummaryInfo{
			{ProgramID: "PROG1", TotalParagraphs: 20, DeadParagraphs: 3},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/dead-code-summary", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PROG1")
}

func TestMigrationSequence(t *testing.T) {
	mock := &MockReader{
		MigrationSteps: []n4j.MigrationStep{
			{ProgramID: "LEAF1", Order: 1},
			{ProgramID: "MIDDLE", Order: 2, BlockedBy: []string{"LEAF1"}},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/migration-sequence", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "LEAF1")
}

func TestFieldImpact(t *testing.T) {
	mock := &MockReader{
		FieldImpacts: []n4j.FieldImpactInfo{
			{ProgramID: "PROG1", FieldName: "WS-CUST-ID"},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/field-impact/PROG1/WS-CUST-ID", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "WS-CUST-ID")
}

func TestSharedDataChannels(t *testing.T) {
	mock := &MockReader{
		SharedChannels: []n4j.SharedDataChannelInfo{
			{Resource: "CUSTFILE", Channel: "FILE", Writers: []string{"PROG1"}, Readers: []string{"PROG2"}},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/shared-data-channels", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTFILE")
}

func TestFileAccessors_Found(t *testing.T) {
	mock := &MockReader{
		FileAccessResult: &n4j.FileAccessInfo{
			FileName: "CUSTFILE",
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/file-accessors/CUSTFILE", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTFILE")
}

func TestFileAccessors_NotFound(t *testing.T) {
	mock := &MockReader{FileAccessResult: nil}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/file-accessors/NOPE", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIDMSImpact_Found(t *testing.T) {
	mock := &MockReader{
		IDMSImpactResult: &n4j.IDMSImpactInfo{
			RecordName: "CUSTOMER-REC",
			Navigators: []string{"PROG1"},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/idms-impact/CUSTOMER-REC", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER-REC")
}

func TestIDMSImpact_NotFound(t *testing.T) {
	mock := &MockReader{IDMSImpactResult: nil}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/idms-impact/NOPE", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestValidationReport_Found(t *testing.T) {
	mock := &MockReader{
		ValidationResult: &n4j.ValidationResult{
			Checks: []n4j.ValidationCheck{
				{Name: "dangling_calls", Count: 2},
			},
		},
	}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/validation-report", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "dangling_calls")
}

func TestValidationReport_NotFound(t *testing.T) {
	mock := &MockReader{ValidationResult: nil}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/analysis/validation-report", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestReassignDomain_NoWriter(t *testing.T) {
	mock := &MockReader{}
	r := setupAnalysisRouter(mock) // no writer set

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"programId":"PROG1"}`)
	req, _ := http.NewRequest("POST", "/api/v1/domains/NewDomain/reassign", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestReassignDomain_MissingBody(t *testing.T) {
	mock := &MockReader{}
	r := setupAnalysisRouter(mock)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest("POST", "/api/v1/domains/NewDomain/reassign", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
