package api

import (
	"log"
	"net/http"
	"ocf-worker/internal/jobs"
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

// Health check
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "ocf-worker",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Create a new generation job
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

// Get job status
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

// List jobs with optional filtering
func (h *Handlers) ListJobs(c *gin.Context) {
	validator := GetValidator(c)
	if validator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation service unavailable"})
		return
	}

	status := c.Query("status")
	courseIDStr := c.Query("course_id")

	// Valider les paramètres de base
	limit := 100 // valeur par défaut
	offset := 0  // valeur par défaut

	// validation des paramètres de liste
	validationResult := validator.ValidateListParams(status, limit, offset)
	if !validationResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid query parameters",
			"validation_errors": validationResult.Errors,
		})
		return
	}

	log.Printf("Listing jobs with status: %s, course_id: %s", status, courseIDStr)

	var courseID *uuid.UUID
	if courseIDStr != "" {
		// 👈 Utiliser le validator pour course_id aussi
		parsedCourseID, courseValidation := validator.ValidateCourseIDParam(courseIDStr)
		if !courseValidation.Valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":             "Invalid course_id parameter",
				"validation_errors": courseValidation.Errors,
			})
			return
		}
		courseID = &parsedCourseID
	}

	jobs, err := h.jobService.ListJobs(c.Request.Context(), status, courseID)
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
