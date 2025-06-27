package jobs

import (
	"context"
	"fmt"
	"log"
	"time"
	"ocf-worker/pkg/models"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type jobServiceImpl struct {
	repo   JobRepository
	tracer trace.Tracer
}

func NewJobServiceImpl(repo JobRepository) JobService {
	return &jobServiceImpl{
		repo:   repo,
		tracer: otel.Tracer("ocf-worker/jobs"),
	}
}

func (s *jobServiceImpl) CreateJob(ctx context.Context, req *models.GenerationRequest) (*models.GenerationJob, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.CreateJob")
	defer span.End()

	log.Printf("JobService.CreateJob: Creating job with ID %s", req.JobID)

	// Convertir les metadata en type JSON personnalisé
	metadata := models.JSON{}
	if req.Metadata != nil {
		metadata = models.JSON(req.Metadata)
	}

	job := &models.GenerationJob{
		ID:          req.JobID,
		CourseID:    req.CourseID,
		Status:      models.StatusPending,
		Progress:    0,
		SourcePath:  req.SourcePath,
		CallbackURL: req.CallbackURL,
		Metadata:    metadata,
		Logs:        models.StringSlice{}, // Initialiser avec un slice vide
	}

	if err := s.repo.Create(ctx, job); err != nil {
		span.RecordError(err)
		log.Printf("JobService.CreateJob: Failed to create job %s: %v", req.JobID, err)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.Printf("JobService.CreateJob: Job %s created successfully for course %s", job.ID, job.CourseID)
	return job, nil
}

func (s *jobServiceImpl) GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.GetJob")
	defer span.End()

	log.Printf("JobService.GetJob: Retrieving job with ID %s", id)

	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		log.Printf("JobService.GetJob: Failed to get job %s: %v", id, err)
		return nil, fmt.Errorf("failed to get job %s: %w", id, err)
	}

	log.Printf("JobService.GetJob: Job %s retrieved successfully, status: %s", job.ID, job.Status)
	return job, nil
}

func (s *jobServiceImpl) ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.ListJobs")
	defer span.End()

	log.Printf("JobService.ListJobs: Listing jobs with status=%s, courseID=%v", status, courseID)

	filters := JobFilters{
		Status:   status,
		CourseID: courseID,
		Limit:    100, // Default limit
	}

	jobs, err := s.repo.List(ctx, filters)
	if err != nil {
		span.RecordError(err)
		log.Printf("JobService.ListJobs: Failed to list jobs: %v", err)
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	log.Printf("JobService.ListJobs: Retrieved %d jobs", len(jobs))
	return jobs, nil
}

func (s *jobServiceImpl) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	ctx, span := s.tracer.Start(ctx, "JobService.UpdateJobStatus")
	defer span.End()

	log.Printf("JobService.UpdateJobStatus: Updating job %s to status %s (progress: %d%%)", id, status, progress)

	if err := s.repo.UpdateStatus(ctx, id, status, progress, errorMsg); err != nil {
		span.RecordError(err)
		log.Printf("JobService.UpdateJobStatus: Failed to update job status: %v", err)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("JobService.UpdateJobStatus: Job %s status updated successfully", id)
	return nil
}

func (s *jobServiceImpl) AddJobLog(ctx context.Context, id uuid.UUID, logEntry string) error {
	ctx, span := s.tracer.Start(ctx, "JobService.AddJobLog")
	defer span.End()

	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get job for logging: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logWithTimestamp := fmt.Sprintf("[%s] %s", timestamp, logEntry)
	
	// Ajouter le log en utilisant le type StringSlice
	job.Logs = append(job.Logs, logWithTimestamp)
	job.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, job); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update job logs: %w", err)
	}

	return nil
}

func (s *jobServiceImpl) CleanupOldJobs(ctx context.Context, maxAge time.Duration) (int64, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.CleanupOldJobs")
	defer span.End()

	cutoffTime := time.Now().Add(-maxAge)
	deleted, err := s.repo.DeleteOldJobs(ctx, cutoffTime)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to cleanup old jobs: %w", err)
	}

	if deleted > 0 {
		log.Printf("Cleaned up %d old jobs", deleted)
	}

	return deleted, nil
}
