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

func setupAnalysisRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.AnalysisHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/programs/:id/impact", h.ImpactAnalysis)
	r.GET("/api/v1/domains", h.ListDomains)
	r.GET("/api/v1/domains/:name", h.GetDomain)
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
