package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"cobol-ingestor/api/middleware"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/pipeline"
	"cobol-ingestor/internal/scanner"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// JobHandler handles async ingestion job endpoints.
type JobHandler struct {
	Reader   n4j.Reader
	Logger   *zap.Logger
	Pipeline *pipeline.Pipeline
	jobs     sync.Map // map[string]*n4j.JobStatus
}

// TriggerRequest is the request body for triggering an ingestion job.
type TriggerRequest struct {
	Dir  string `json:"dir" binding:"required"`
	Pass int    `json:"pass"`
}

// Trigger starts an async ingestion job.
// @Summary Trigger ingestion job
// @Tags jobs
// @Param body body TriggerRequest true "Job parameters"
// @Success 202 {object} neo4j.JobStatus
// @Router /api/v1/jobs/ingest [post]
func (h *JobHandler) Trigger(c *gin.Context) {
	var req TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	if h.Pipeline == nil {
		middleware.InternalError(c, "pipeline not configured")
		return
	}

	jobID := uuid.New().String()
	status := &n4j.JobStatus{
		ID:        jobID,
		Status:    "pending",
		StartedAt: time.Now(),
	}
	h.jobs.Store(jobID, status)

	go h.runJob(jobID, req)

	c.JSON(http.StatusAccepted, status)
}

// Status returns the current state of a job.
// @Summary Get job status
// @Tags jobs
// @Param id path string true "Job ID"
// @Success 200 {object} neo4j.JobStatus
// @Router /api/v1/jobs/{id} [get]
func (h *JobHandler) Status(c *gin.Context) {
	jobID := c.Param("id")

	val, ok := h.jobs.Load(jobID)
	if !ok {
		middleware.NotFound(c, "job not found")
		return
	}

	c.JSON(http.StatusOK, val)
}

func (h *JobHandler) runJob(jobID string, req TriggerRequest) {
	val, _ := h.jobs.Load(jobID)
	status := val.(*n4j.JobStatus)
	status.Status = "running"

	ctx := context.Background()

	scanResult, err := scanner.Scan(ctx, req.Dir, h.Logger)
	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
		return
	}

	if err := h.Pipeline.Run(ctx, scanResult, req.Pass); err != nil {
		status.Status = "failed"
		status.Error = err.Error()
		return
	}

	status.Status = "completed"
}
