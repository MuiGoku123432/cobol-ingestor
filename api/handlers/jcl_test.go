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

func setupJCLRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.JCLHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/jcl/jobs", h.ListJobs)
	r.GET("/api/v1/jcl/jobs/:id", h.GetJob)
	r.GET("/api/v1/jcl/datasets/:name/usage", h.DatasetUsage)
	return r
}

func TestListJCLJobs(t *testing.T) {
	mock := &MockReader{
		JCLJobs: []n4j.JCLJobInfo{
			{JobName: "NIGHTBAT"},
			{JobName: "DAYBATCH"},
		},
	}
	r := setupJCLRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jcl/jobs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NIGHTBAT")
}

func TestGetJCLJob_Found(t *testing.T) {
	mock := &MockReader{
		JCLJobResult: &n4j.JCLJobDetail{JobName: "NIGHTBAT"},
	}
	r := setupJCLRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jcl/jobs/NIGHTBAT", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NIGHTBAT")
}

func TestGetJCLJob_NotFound(t *testing.T) {
	mock := &MockReader{JCLJobResult: nil}
	r := setupJCLRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jcl/jobs/NOPE", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDatasetUsage(t *testing.T) {
	mock := &MockReader{
		DatasetUsages: []n4j.DatasetUsageInfo{
			{DSName: "CUST.MASTER", Jobs: []string{"NIGHTBAT"}, IsInput: true},
		},
	}
	r := setupJCLRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jcl/datasets/CUST.MASTER/usage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUST.MASTER")
}
