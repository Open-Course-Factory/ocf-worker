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
	validator  *validation.APIValidator
}

func NewHandlers(jobService jobs.JobService, apiValidator *validation.APIValidator) *Handlers {
	return &Handlers{
		jobService: jobService,
		validator:  apiValidator,
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
	var req models.GenerationRequest

	// Validation JSON basique
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// validation avec le système structuré
	validationResult := h.validator.ValidateGenerationRequest(&req)
	if !validationResult.Valid {
		// Formater les erreurs de validation pour l'API
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Validation failed",
			"validation_errors": validationResult.Errors,
		})
		return
	}

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
	jobIDStr := c.Param("id")
	log.Printf("Retrieving job status for ID: %s", jobIDStr)

	jobID, validationResult := h.validator.ValidateJobIDParam(jobIDStr)
	if !validationResult.Valid {
		log.Printf("Invalid job ID format: %s", jobIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid job ID",
			"validation_errors": validationResult.Errors,
		})
		return
	}

	log.Printf("Parsed job ID: %s", jobID)

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
	status := c.Query("status")
	courseIDStr := c.Query("course_id")

	// Valider les paramètres de base
	limit := 100 // valeur par défaut
	offset := 0  // valeur par défaut

	// validation des paramètres de liste
	validationResult := h.validator.ValidateListParams(status, limit, offset)
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
		parsedCourseID, courseValidation := h.validator.ValidateCourseIDParam(courseIDStr)
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
