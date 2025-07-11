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
// List jobs with optional filtering
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
