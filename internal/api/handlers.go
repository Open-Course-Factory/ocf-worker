package api

import (
	"log"
	"net/http"
	"ocf-worker/internal/jobs"
	"ocf-worker/internal/validation"
	"ocf-worker/pkg/models"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	jobService jobs.JobService
}

func NewHandlers(jobService jobs.JobService) *Handlers {
	return &Handlers{
		jobService: jobService,
	}
}

// Health effectue un health check du service
// @Summary Health check du service
// @Description Vérifie l'état de santé du service OCF Worker
// @Description
// @Description Retourne l'état du service, de la base de données et des composants critiques.
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} models.HealthResponse "Service en bonne santé"
// @Success 503 {object} models.ErrorResponse "Service dégradé ou en panne"
// @Router /health [get]
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "ocf-worker",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// CreateJob crée un nouveau job de génération
// @Summary Créer un job de génération
// @Description Créer un nouveau job de génération de cours Slidev
// @Description
// @Description Le job sera traité de manière asynchrone par le pool de workers.
// @Description Utilisez l'endpoint GET /jobs/{id} pour suivre le progress.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param request body models.GenerationRequest true "Détails du job à créer"
// @Success 201 {object} models.JobResponse "Job créé avec succès"
// @Failure 400 {object} models.ErrorResponse "Erreur de validation"
// @Failure 409 {object} models.ErrorResponse "Job avec cet ID existe déjà"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /generate [post]
func (h *Handlers) CreateJob(c *gin.Context) {
	// Récupérer la requête déjà validée
	req := c.MustGet("validated_request").(models.GenerationRequest)

	log.Printf("Creating job with ID: %s, Course ID: %s", req.JobID, req.CourseID)

	job, err := h.jobService.CreateJob(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Failed to create job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Job created successfully: %s", job.ID)
	c.JSON(http.StatusCreated, job.ToResponse())
}

// GetJobStatus récupère le statut d'un job
// @Summary Récupérer le statut d'un job
// @Description Récupère les détails et le statut actuel d'un job de génération
// @Description
// @Description Les statuts possibles sont:
// @Description - `pending`: Job en attente de traitement
// @Description - `processing`: Job en cours de traitement
// @Description - `completed`: Job terminé avec succès
// @Description - `failed`: Job échoué (voir le champ error)
// @Description - `timeout`: Job interrompu par timeout
// @Tags Jobs
// @Accept json
// @Produce json
// @Param id path string true "ID du job (UUID)" Format(uuid)
// @Success 200 {object} models.JobResponse "Détails du job"
// @Failure 400 {object} models.ErrorResponse "ID du job invalide"
// @Failure 404 {object} models.ErrorResponse "Job non trouvé"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /jobs/{id} [get]
func (h *Handlers) GetJobStatus(c *gin.Context) {
	// Récupérer l'UUID déjà validé et parsé
	jobID := c.MustGet("validated_job_id").(uuid.UUID)

	log.Printf("Retrieving job status for ID: %s", jobID)

	job, err := h.jobService.GetJob(c.Request.Context(), jobID)
	if err != nil {
		log.Printf("Failed to retrieve job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	log.Printf("Job retrieved successfully: %s, status: %s", job.ID, job.Status)
	c.JSON(http.StatusOK, job.ToResponse())
}

// ListJobs liste les jobs avec filtrage optionnel
// @Summary Lister les jobs
// @Description Liste les jobs de génération avec options de filtrage et pagination
// @Description
// @Description Permet de filtrer par statut et par course_id pour retrouver facilement
// @Description les jobs en cours ou terminés.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param status query string false "Filtrer par statut" Enums(pending,processing,completed,failed,timeout)
// @Param course_id query string false "Filtrer par ID de cours" Format(uuid)
// @Param limit query integer false "Nombre maximum de résultats" default(100) minimum(1) maximum(1000)
// @Param offset query integer false "Décalage pour la pagination" default(0) minimum(0)
// @Success 200 {object} models.JobListResponse "Liste des jobs"
// @Failure 400 {object} models.ErrorResponse "Paramètres de requête invalides"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /jobs [get]
func (h *Handlers) ListJobs(c *gin.Context) {
	// Récupérer les valeurs déjà validées
	status := c.GetString("validated_status")
	courseID, _ := c.Get("validated_course_id")
	pagination := c.MustGet("validated_pagination").(validation.PaginationParams)

	// Convertir courseID en bon type (peut être nil)
	var courseIDPtr *uuid.UUID
	if courseID != nil {
		courseIDPtr = courseID.(*uuid.UUID)
	}

	log.Printf("Listing jobs with status: %s, course_id: %v, limit: %d, offset: %d",
		status, courseIDPtr, pagination.Limit, pagination.Offset)

	jobs, err := h.jobService.ListJobs(c.Request.Context(), status, courseIDPtr)
	if err != nil {
		log.Printf("Failed to list jobs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Retrieved %d jobs", len(jobs))

	responses := make([]*models.JobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = job.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"jobs": responses})
}
