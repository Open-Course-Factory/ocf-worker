// internal/api/router.go - Enhanced version with worker routes
package api

import (
	"fmt"
	"net/http"
	"ocf-worker/internal/jobs"
	"ocf-worker/internal/storage"
	"ocf-worker/internal/validation"
	"ocf-worker/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SetupRouter configure le routeur standard (rétrocompatibilité)
func SetupRouter(jobService jobs.JobService, storageService *storage.StorageService, workerPool *worker.WorkerPool) *gin.Engine {
	r := gin.Default()

	// Middleware pour CORS et logs
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	validationConfig := validation.DefaultValidationConfig()
	apiValidator := validation.NewAPIValidator(validationConfig)

	r.Use(SecurityHeadersMiddleware())
	r.Use(ValidationMiddleware(apiValidator))
	r.Use(ValidationErrorLogger())
	r.Use(RateLimitMiddleware(60))

	// Handlers
	jobHandlers := NewHandlers(jobService)
	storageHandlers := NewStorageHandlers(storageService)
	workerHandlers := NewWorkerHandlers(workerPool)
	themeHandlers := NewThemeHandlers(storageService, workerPool.GetConfig().WorkspaceBase)
	archiveHandlers := NewArchiveHandlers(storageService)

	// Routes principales
	r.GET("/health", jobHandlers.Health)

	api := r.Group("/api/v1")
	{
		// Routes des jobs
		api.POST("/generate",
			validation.ParseGenerationRequest(),
			validation.ValidateRequest(validation.ValidateGenerationRequest),
			jobHandlers.CreateJob)
		api.GET("/jobs/:id",
			validation.ValidateRequest(validation.ValidateJobIDParam("id")),
			jobHandlers.GetJobStatus)
		api.GET("/jobs",
			validation.ValidateRequest(validation.ValidateListJobsParams),
			jobHandlers.ListJobs)

		// Routes du storage
		storage := api.Group("/storage")
		{
			storage.GET("/info", storageHandlers.GetStorageInfo)

			storage.POST("/jobs/:job_id/sources",
				validation.ValidateRequest(
					validation.ValidateJobIDParam("job_id"),
					validation.ValidateFileUpload,
				),
				storageHandlers.UploadJobSources)

			storage.GET("/jobs/:job_id/sources",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				storageHandlers.ListJobSources)

			storage.GET("/jobs/:job_id/sources/:filename",
				validation.ValidateRequest(
					validation.ValidateJobIDParam("job_id"),
					validation.ValidateFilenameParam("filename"),
				),
				storageHandlers.DownloadJobSource)

			storage.GET("/courses/:course_id/results",
				validation.ValidateRequest(validation.ValidateCourseIDParam("course_id")),
				storageHandlers.ListResults)

			storage.GET("/courses/:course_id/results/:filename",
				validation.ValidateRequest(
					validation.ValidateCourseIDParam("course_id"),
					validation.ValidateFilenameParam("filename"),
				),
				storageHandlers.DownloadResult)

			storage.GET("/jobs/:job_id/logs",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				storageHandlers.GetJobLogs)
		}

		workerAPI := api.Group("/worker")
		{
			workerAPI.GET("/stats", workerHandlers.GetWorkerStats)
			workerAPI.GET("/health", workerHandlers.GetWorkerHealth)

			// Routes avec validation
			workerAPI.GET("/workspaces",
				validation.ValidateRequest(validation.ValidateWorkspaceListParams),
				workerHandlers.ListWorkspaces)

			workerAPI.GET("/workspaces/:job_id",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				workerHandlers.GetWorkspaceInfo)

			workerAPI.DELETE("/workspaces/:job_id",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				workerHandlers.CleanupWorkspace)

			workerAPI.POST("/workspaces/cleanup",
				validation.ValidateRequest(validation.ValidateWorkspaceCleanupParams),
				workerHandlers.CleanupOldWorkspaces)
		}

		themeAPI := api.Group("/themes")
		{
			themeAPI.GET("/available", themeHandlers.ListAvailableThemes)

			themeAPI.POST("/install",
				validation.ValidateRequest(validation.ValidateThemeInstallRequest),
				themeHandlers.InstallTheme)

			themeAPI.GET("/jobs/:job_id/detect",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				themeHandlers.DetectThemes)

			themeAPI.POST("/jobs/:job_id/install",
				validation.ValidateRequest(validation.ValidateJobIDParam("job_id")),
				themeHandlers.InstallJobThemes)
		}

		storage.GET("/courses/:course_id/archive",
			validation.ValidateRequest(
				validation.ValidateCourseIDParam("course_id"),
				ValidateArchiveParams,
			),
			archiveHandlers.DownloadResultsArchive)

		storage.POST("/courses/:course_id/archive",
			validation.ValidateRequest(validation.ValidateCourseIDParam("course_id")),
			archiveHandlers.CreateResultsArchive)
	}

	return r
}

// WorkerHandlers gère les endpoints liés au worker
type WorkerHandlers struct {
	workerPool *worker.WorkerPool
}

// NewWorkerHandlers crée un nouveau gestionnaire pour les workers
func NewWorkerHandlers(workerPool *worker.WorkerPool) *WorkerHandlers {
	return &WorkerHandlers{
		workerPool: workerPool,
	}
}

// GetWorkerStats retourne les statistiques du pool de workers
func (h *WorkerHandlers) GetWorkerStats(c *gin.Context) {
	stats := h.workerPool.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"worker_pool": stats,
		"timestamp": gin.H{
			"unix": gin.H{
				"seconds": gin.H{
					"now": "placeholder", // Sera remplacé par time.Now().Unix()
				},
			},
		},
	})
}

// GetWorkerHealth vérifie la santé du worker
func (h *WorkerHandlers) GetWorkerHealth(c *gin.Context) {
	stats := h.workerPool.GetStats()

	status := "healthy"
	issues := []string{}

	if !stats.Running {
		status = "unhealthy"
		issues = append(issues, "worker pool not running")
	}

	if stats.QueueSize >= stats.QueueCapacity {
		status = "degraded"
		issues = append(issues, "job queue is full")
	}

	// Vérifier si des workers sont bloqués
	stuckWorkers := 0
	for _, worker := range stats.Workers {
		if worker.Status == "stopped" {
			stuckWorkers++
		}
	}

	if stuckWorkers > 0 {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("%d workers stopped", stuckWorkers))
	}

	response := gin.H{
		"status": status,
		"worker_pool": gin.H{
			"running":      stats.Running,
			"worker_count": stats.WorkerCount,
			"queue_size":   stats.QueueSize,
			"queue_usage":  float64(stats.QueueSize) / float64(stats.QueueCapacity) * 100,
		},
	}

	if len(issues) > 0 {
		response["issues"] = issues
	}

	statusCode := http.StatusOK
	switch status {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "degraded":
		statusCode = http.StatusOK // Toujours 200 mais avec des warnings
	}

	c.JSON(statusCode, response)
}

// ListWorkspaces liste tous les workspaces
func (h *WorkerHandlers) ListWorkspaces(c *gin.Context) {
	// Cette fonctionnalité nécessiterait un workspace manager global
	// Pour l'instant, on retourne une réponse basique
	c.JSON(http.StatusOK, gin.H{
		"message":    "Workspace listing not yet implemented",
		"workspaces": []gin.H{},
	})
}

// GetWorkspaceInfo retourne les informations d'un workspace spécifique
func (h *WorkerHandlers) GetWorkspaceInfo(c *gin.Context) {
	// Récupérer l'ID déjà validé
	jobID := c.MustGet("validated_job_id").(uuid.UUID)

	// Plus de validation inline - directement la logique métier
	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID.String(),
		"message": "Workspace info not yet implemented",
	})
}

// CleanupWorkspace nettoie un workspace spécifique
func (h *WorkerHandlers) CleanupWorkspace(c *gin.Context) {
	// Récupérer l'ID déjà validé
	jobID := c.MustGet("validated_job_id").(uuid.UUID)

	// Plus de validation inline - directement la logique métier
	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID.String(),
		"message": "Workspace cleanup simulated (not yet implemented)",
		"cleaned": true,
	})
}

// CleanupOldWorkspaces nettoie les anciens workspaces
func (h *WorkerHandlers) CleanupOldWorkspaces(c *gin.Context) {
	// Récupérer les paramètres déjà validés
	params := c.MustGet("validated_workspace_cleanup_params").(validation.WorkspaceCleanupParams)

	// Plus de validation inline - directement la logique métier
	c.JSON(http.StatusOK, gin.H{
		"message":       "Old workspace cleanup simulated (not yet implemented)",
		"max_age_hours": params.MaxAgeHours,
		"cleaned_count": 0,
	})
}
