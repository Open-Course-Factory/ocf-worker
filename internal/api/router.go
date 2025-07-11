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

	// Handlers
	jobHandlers := NewHandlers(jobService, apiValidator)
	storageHandlers := NewStorageHandlers(storageService)
	workerHandlers := NewWorkerHandlers(workerPool)
	themeHandlers := NewThemeHandlers(storageService, workerPool.GetConfig().WorkspaceBase)

	// Routes principales
	r.GET("/health", jobHandlers.Health)

	api := r.Group("/api/v1")
	{
		// Routes des jobs
		api.POST("/generate", jobHandlers.CreateJob)
		api.GET("/jobs/:id", jobHandlers.GetJobStatus)
		api.GET("/jobs", jobHandlers.ListJobs)

		// Routes du storage
		storage := api.Group("/storage")
		{
			// Info storage
			storage.GET("/info", storageHandlers.GetStorageInfo)

			// Sources des jobs
			storage.POST("/jobs/:job_id/sources", storageHandlers.UploadJobSources)
			storage.GET("/jobs/:job_id/sources", storageHandlers.ListJobSources)
			storage.GET("/jobs/:job_id/sources/:filename", storageHandlers.DownloadJobSource)

			// Résultats des cours
			storage.GET("/courses/:course_id/results", storageHandlers.ListResults)
			storage.GET("/courses/:course_id/results/:filename", storageHandlers.DownloadResult)

			// Logs des jobs
			storage.GET("/jobs/:job_id/logs", storageHandlers.GetJobLogs)
		}

		workerAPI := api.Group("/worker")
		{
			workerAPI.GET("/stats", workerHandlers.GetWorkerStats)
			workerAPI.GET("/health", workerHandlers.GetWorkerHealth)
			workerAPI.GET("/workspaces", workerHandlers.ListWorkspaces)
			workerAPI.GET("/workspaces/:job_id", workerHandlers.GetWorkspaceInfo)
			workerAPI.DELETE("/workspaces/:job_id", workerHandlers.CleanupWorkspace)
			workerAPI.POST("/workspaces/cleanup", workerHandlers.CleanupOldWorkspaces)
		}

		themeAPI := api.Group("/themes")
		{
			themeAPI.GET("/available", themeHandlers.ListAvailableThemes)
			themeAPI.GET("/jobs/:job_id/detect", themeHandlers.DetectThemes)
		}
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
	if status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if status == "degraded" {
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
	jobID := c.Param("job_id")

	// Validation basique de l'UUID
	if len(jobID) != 36 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID format"})
		return
	}

	// Pour l'instant, retourner une réponse placeholder
	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID,
		"message": "Workspace info not yet implemented",
	})
}

// CleanupWorkspace nettoie un workspace spécifique
func (h *WorkerHandlers) CleanupWorkspace(c *gin.Context) {
	jobID := c.Param("job_id")

	// Validation basique de l'UUID
	if len(jobID) != 36 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID format"})
		return
	}

	// Pour l'instant, retourner une réponse de succès simulée
	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID,
		"message": "Workspace cleanup simulated (not yet implemented)",
		"cleaned": true,
	})
}

// CleanupOldWorkspaces nettoie les anciens workspaces
func (h *WorkerHandlers) CleanupOldWorkspaces(c *gin.Context) {
	// Paramètres optionnels
	maxAge := c.DefaultQuery("max_age_hours", "24")

	// Pour l'instant, retourner une réponse de succès simulée
	c.JSON(http.StatusOK, gin.H{
		"message":       "Old workspace cleanup simulated (not yet implemented)",
		"max_age_hours": maxAge,
		"cleaned_count": 0,
	})
}
