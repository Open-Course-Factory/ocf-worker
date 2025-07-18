// internal/api/router.go - Enhanced version with worker routes
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/internal/jobs"
	"github.com/Open-Course-Factory/ocf-worker/internal/storage"
	"github.com/Open-Course-Factory/ocf-worker/internal/validation"
	"github.com/Open-Course-Factory/ocf-worker/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	_ "github.com/Open-Course-Factory/ocf-worker/pkg/models"
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

	api := r.Group("/api/v1")
	{
		// Routes principales
		api.GET("/health", jobHandlers.Health)
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

	// Configuration Swagger
	SetupSwagger(r)

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

// GetWorkerStats retourne les statistiques détaillées du pool de workers
// @Summary Statistiques du pool de workers
// @Description Retourne des informations détaillées sur l'état du pool de workers
// @Description
// @Description Inclut le nombre de workers actifs, la taille de la queue des jobs,
// @Description les statistiques individuelles de chaque worker, et les métriques de performance.
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} models.WorkerStatsResponse "Statistiques du pool de workers"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /worker/stats [get]
func (h *WorkerHandlers) GetWorkerStats(c *gin.Context) {
	stats := h.workerPool.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"worker_pool": stats,
		"timestamp": gin.H{
			"unix": gin.H{
				"seconds": gin.H{
					"now": time.Now().Unix(),
				},
			},
		},
	})
}

// GetWorkerHealth vérifie l'état de santé du système de workers
// @Summary Santé du système de workers
// @Description Effectue un health check complet du système de workers
// @Description
// @Description Vérifie que le pool de workers fonctionne correctement,
// @Description que les workers ne sont pas bloqués, et que la queue n'est pas saturée.
// @Tags Worker
// @Accept json
// @Produce json
// @Success 200 {object} models.WorkerHealthResponse "Système de workers en bonne santé"
// @Success 503 {object} models.WorkerHealthResponse "Système de workers dégradé ou en panne"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /worker/health [get]
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

// ListWorkspaces liste tous les workspaces actifs
// @Summary Lister les workspaces actifs
// @Description Liste tous les workspaces de jobs en cours ou récents
// @Description
// @Description Permet de surveiller l'utilisation des ressources et identifier
// @Description les workspaces qui peuvent nécessiter un nettoyage.
// @Tags Worker
// @Accept json
// @Produce json
// @Param status query string false "Filtrer par statut" Enums(active,idle,completed)
// @Param limit query integer false "Nombre maximum de résultats" default(50) minimum(1) maximum(500)
// @Param offset query integer false "Décalage pour la pagination" default(0) minimum(0)
// @Success 200 {object} models.WorkspaceListResponse "Liste des workspaces"
// @Failure 400 {object} models.ErrorResponse "Paramètres de requête invalides"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /worker/workspaces [get]
func (h *WorkerHandlers) ListWorkspaces(c *gin.Context) {
	// Cette fonctionnalité nécessiterait un workspace manager global
	// Pour l'instant, on retourne une réponse basique
	c.JSON(http.StatusOK, gin.H{
		"message":    "Workspace listing not yet implemented",
		"workspaces": []gin.H{},
	})
}

// GetWorkspaceInfo retourne les informations détaillées d'un workspace
// @Summary Informations détaillées d'un workspace
// @Description Retourne les informations complètes d'un workspace spécifique
// @Description
// @Description Inclut la taille utilisée, le nombre de fichiers, l'état du répertoire dist,
// @Description et autres métadonnées utiles pour le debug et le monitoring.
// @Tags Worker
// @Accept json
// @Produce json
// @Param job_id path string true "ID du job associé au workspace" Format(uuid)
// @Success 200 {object} models.WorkspaceInfoResponse "Informations du workspace"
// @Failure 400 {object} models.ErrorResponse "ID du job invalide"
// @Failure 404 {object} models.ErrorResponse "Workspace non trouvé"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /worker/workspaces/{job_id} [get]
func (h *WorkerHandlers) GetWorkspaceInfo(c *gin.Context) {
	// Récupérer l'ID déjà validé
	jobID := c.MustGet("validated_job_id").(uuid.UUID)

	// Plus de validation inline - directement la logique métier
	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID.String(),
		"message": "Workspace info not yet implemented",
	})
}

// CleanupWorkspace supprime un workspace spécifique
// @Summary Supprimer un workspace
// @Description Supprime complètement un workspace et tous ses fichiers
// @Description
// @Description ⚠️ **Opération destructive** : tous les fichiers du workspace seront perdus.
// @Description Utilisez cette opération uniquement pour les jobs terminés ou échoués.
// @Tags Worker
// @Accept json
// @Produce json
// @Param job_id path string true "ID du job associé au workspace" Format(uuid)
// @Success 200 {object} models.WorkspaceCleanupResponse "Workspace supprimé avec succès"
// @Failure 400 {object} models.ErrorResponse "ID du job invalide"
// @Failure 404 {object} models.ErrorResponse "Workspace non trouvé"
// @Failure 500 {object} models.ErrorResponse "Erreur de suppression"
// @Router /worker/workspaces/{job_id} [delete]
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

// CleanupOldWorkspaces supprime les workspaces anciens
// @Summary Nettoyage automatique des anciens workspaces
// @Description Supprime tous les workspaces plus anciens que l'âge spécifié
// @Description
// @Description Opération de maintenance pour libérer l'espace disque.
// @Description Par défaut, supprime les workspaces de plus de 24 heures.
// @Tags Worker
// @Accept json
// @Produce json
// @Param max_age_hours query integer false "Âge maximum en heures" default(24) minimum(1) maximum(8760)
// @Success 200 {object} models.WorkspaceCleanupBatchResponse "Nettoyage terminé"
// @Failure 400 {object} models.ErrorResponse "Paramètres invalides"
// @Failure 500 {object} models.ErrorResponse "Erreur de nettoyage"
// @Router /worker/workspaces/cleanup [post]
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
