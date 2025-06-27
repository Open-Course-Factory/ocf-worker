package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"ocf-worker/internal/jobs"
	"ocf-worker/pkg/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	jobService := jobs.NewJobService()
	router := SetupRouter(jobService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "ocf-worker", response["service"])
}

func TestCreateJobEndpoint(t *testing.T) {
	jobService := jobs.NewJobService()
	router := SetupRouter(jobService)

	jobID := uuid.New()
	courseID := uuid.New()
	
	reqBody := models.GenerationRequest{
		JobID:      jobID,
		CourseID:   courseID,
		SourcePath: "courses/pending/" + jobID.String(),
		CallbackURL: "http://localhost:8080/api/v1/generations/" + jobID.String() + "/status",
	}

	jsonBody, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	
	var response models.JobResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, courseID, response.CourseID)
	assert.Equal(t, models.StatusPending, response.Status)
}

func TestInvalidJobID(t *testing.T) {
	jobService := jobs.NewJobService()
	router := SetupRouter(jobService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/invalid-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}
