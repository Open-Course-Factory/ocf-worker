package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"ocf-worker/internal/jobs"
	"ocf-worker/internal/storage"
	"ocf-worker/internal/storage/filesystem"
	"ocf-worker/internal/worker"
	"ocf-worker/pkg/models"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestRouter(t *testing.T) *gin.Engine {
	jobService, storageService := setupTestServices(t)

	// Créer un mock worker pool pour les tests
	mockWorkerPool := createMockWorkerPool(jobService, storageService)

	// 👈 Utiliser SetupRouterWithWorker au lieu de SetupRouter
	return SetupRouter(jobService, storageService, mockWorkerPool)
}

// Helper pour créer un mock worker pool
func createMockWorkerPool(jobService jobs.JobService, storageService *storage.StorageService) *worker.WorkerPool {
	config := &worker.PoolConfig{
		WorkerCount:   1,
		PollInterval:  1 * time.Second,
		JobTimeout:    30 * time.Second,
		WorkspaceBase: os.TempDir(),
	}
	return worker.NewWorkerPool(jobService, storageService, config)
}

func setupTestServices(t *testing.T) (jobs.JobService, *storage.StorageService) {
	// Créer un répertoire temporaire pour le storage de test
	tempDir, err := os.MkdirTemp("", "ocf-test-storage-*")
	require.NoError(t, err)

	// Cleanup après le test
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Créer le storage service
	storageBackend, err := filesystem.NewFilesystemStorage(tempDir)
	require.NoError(t, err)
	storageService := storage.NewStorageService(storageBackend)

	// Pour les tests, on utilise un mock du job repository
	mockRepo := &mockJobRepository{}
	jobService := jobs.NewJobServiceImpl(mockRepo)

	return jobService, storageService
}

// Mock simple du JobRepository pour les tests
type mockJobRepository struct {
	jobs map[uuid.UUID]*models.GenerationJob
}

func (r *mockJobRepository) Create(ctx context.Context, job *models.GenerationJob) error {
	if r.jobs == nil {
		r.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}
	r.jobs[job.ID] = job
	return nil
}

func (r *mockJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	if r.jobs == nil {
		r.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}
	job, exists := r.jobs[id]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	return job, nil
}

func (r *mockJobRepository) List(ctx context.Context, filters jobs.JobFilters) ([]*models.GenerationJob, error) {
	var result []*models.GenerationJob
	for _, job := range r.jobs {
		if filters.Status == "" || string(job.Status) == filters.Status {
			if filters.CourseID == nil || job.CourseID == *filters.CourseID {
				result = append(result, job)
			}
		}
	}
	return result, nil
}

func (r *mockJobRepository) Update(ctx context.Context, job *models.GenerationJob) error {
	if r.jobs == nil {
		r.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}
	r.jobs[job.ID] = job
	return nil
}

func (r *mockJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	if r.jobs == nil {
		r.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}
	job, exists := r.jobs[id]
	if !exists {
		return gorm.ErrRecordNotFound
	}
	job.Status = status
	job.Progress = progress
	if errorMsg != "" {
		job.Error = errorMsg
	}
	return nil
}

func (r *mockJobRepository) DeleteOldJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	// Pour les tests, on ne supprime rien
	return 0, nil
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter(t)

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
	router := setupTestRouter(t)

	jobID := uuid.New()
	courseID := uuid.New()

	reqBody := models.GenerationRequest{
		JobID:       jobID,
		CourseID:    courseID,
		SourcePath:  "courses/pending/" + jobID.String(),
		CallbackURL: "https://api.example.com/webhook/" + jobID.String(),
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

func TestGetJobStatus(t *testing.T) {
	router := setupTestRouter(t)

	jobID := uuid.New()
	courseID := uuid.New()

	// Créer d'abord un job
	reqBody := models.GenerationRequest{
		JobID:       jobID,
		CourseID:    courseID,
		SourcePath:  "test/path",
		CallbackURL: "https://api.example.com/webhook",
	}

	jsonBody, _ := json.Marshal(reqBody)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/v1/generate", bytes.NewBuffer(jsonBody))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	assert.Equal(t, 201, w1.Code)

	// Maintenant récupérer le job
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/jobs/"+jobID.String(), nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, 200, w2.Code)

	var response models.JobResponse
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, courseID, response.CourseID)
}

func TestInvalidJobID(t *testing.T) {
	router := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/invalid-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestStorageInfoEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/storage/info", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "configured", response["storage_type"])

	// Vérifier que les endpoints sont présents
	endpoints, ok := response["endpoints"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, endpoints, "upload_sources")
	assert.Contains(t, endpoints, "download_source")
}

func TestListJobs(t *testing.T) {
	router := setupTestRouter(t)

	// Créer quelques jobs de test
	for i := 0; i < 3; i++ {
		jobID := uuid.New()
		courseID := uuid.New()

		reqBody := models.GenerationRequest{
			JobID:      jobID,
			CourseID:   courseID,
			SourcePath: "test/path",
		}

		jsonBody, _ := json.Marshal(reqBody)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/generate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 201, w.Code)
	}

	// Lister les jobs
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	jobs, ok := response["jobs"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, jobs, 3)
}

func TestCreateJobValidation(t *testing.T) {
	router := setupTestRouter(t)

	tests := []struct {
		name                string
		requestBody         interface{}
		expectedStatus      int
		expectedError       string
		hasValidationErrors bool
	}{
		{
			name: "Valid request",
			requestBody: map[string]interface{}{
				"job_id":       uuid.New().String(),
				"course_id":    uuid.New().String(),
				"source_path":  "test/path",
				"callback_url": "https://api.example.com/webhook",
			},
			expectedStatus:      201,
			expectedError:       "",
			hasValidationErrors: false,
		},
		{
			name: "Missing course ID - Gin validation",
			requestBody: map[string]interface{}{
				"job_id":      uuid.New().String(),
				"source_path": "test/path",
			},
			expectedStatus:      400,
			expectedError:       "Invalid JSON format", // 👈 Gin validation
			hasValidationErrors: false,
		},
		{
			name: "Empty source path - Gin validation",
			requestBody: map[string]interface{}{
				"job_id":      uuid.New().String(),
				"course_id":   uuid.New().String(),
				"source_path": "", // 👈 Échoue au niveau Gin à cause de required
			},
			expectedStatus:      400,
			expectedError:       "Invalid JSON format", // 👈 Gin validation
			hasValidationErrors: false,
		},
		{
			name: "Invalid source path with path traversal - Validation",
			requestBody: map[string]interface{}{
				"job_id":       uuid.New().String(),
				"course_id":    uuid.New().String(),
				"source_path":  "../../../etc/passwd", // 👈 Passe Gin mais échoue notre validation
				"callback_url": "https://api.example.com/webhook",
			},
			expectedStatus:      400,
			expectedError:       "Validation failed", // 👈 Notre validation
			hasValidationErrors: true,
		},
		{
			name: "Invalid callback URL with localhost - Validation",
			requestBody: map[string]interface{}{
				"job_id":       uuid.New().String(),
				"course_id":    uuid.New().String(),
				"source_path":  "test/path",
				"callback_url": "http://localhost:3000/webhook", // 👈 Notre validation
			},
			expectedStatus:      400,
			expectedError:       "Validation failed", // 👈 Notre validation
			hasValidationErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tt.requestBody)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/generate", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus != 201 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)

				// Vérifier validation_errors seulement pour notre validation
				if tt.hasValidationErrors {
					assert.Contains(t, response, "validation_errors")
				} else {
					assert.NotContains(t, response, "validation_errors")
				}
			}
		})
	}
}

func TestGetJobStatusValidation(t *testing.T) {
	router := setupTestRouter(t)

	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
	}{
		{
			name:           "Invalid UUID format",
			jobID:          "invalid-uuid",
			expectedStatus: 400,
		},
		{
			name:           "Empty job ID - Router redirect to list jobs",
			jobID:          "",
			expectedStatus: 301, // Gin redirects /jobs/ to /jobs
		},
		{
			name:           "Valid UUID but job not found",
			jobID:          uuid.New().String(),
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/jobs/"+tt.jobID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == 400 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "validation_errors")
			}
		})
	}
}

func TestListJobsValidation(t *testing.T) {
	router := setupTestRouter(t)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "Invalid status",
			queryParams:    "?status=invalid_status",
			expectedStatus: 400,
		},
		{
			name:           "Invalid course_id",
			queryParams:    "?course_id=invalid-uuid",
			expectedStatus: 400,
		},
		{
			name:           "Valid parameters",
			queryParams:    "?status=pending",
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/jobs"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
