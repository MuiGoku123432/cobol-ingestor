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

func setupDatabaseRouter(reader n4j.Reader) *gin.Engine {
	r := gin.New()
	h := &handlers.DatabaseHandler{Reader: reader, Logger: zap.NewNop()}
	r.GET("/api/v1/db/tables", h.ListTables)
	r.GET("/api/v1/db/tables/:name/usage", h.TableUsage)
	return r
}

func TestListDBTables(t *testing.T) {
	mock := &MockReader{
		DBTables: []n4j.DBTableInfo{
			{Name: "CUSTOMER", AccessCount: 5},
			{Name: "ORDERS", AccessCount: 3},
		},
	}
	r := setupDatabaseRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/db/tables", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER")
}

func TestDBTableUsage_Found(t *testing.T) {
	mock := &MockReader{
		TableUsageResult: &n4j.TableUsageInfo{TableName: "CUSTOMER"},
	}
	r := setupDatabaseRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/db/tables/CUSTOMER/usage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CUSTOMER")
}

func TestDBTableUsage_NotFound(t *testing.T) {
	mock := &MockReader{TableUsageResult: nil}
	r := setupDatabaseRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/db/tables/NOPE/usage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
