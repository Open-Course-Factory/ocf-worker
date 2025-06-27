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

	job := &models.GenerationJob{
		ID:          req.JobID,
		CourseID:    req.CourseID,
		Status:      models.StatusPending,
		Progress:    0,
		SourcePath:  req.SourcePath,
		CallbackURL: req.CallbackURL,
		Metadata:    req.Metadata,
		Logs:        []string{},
	}

	if err := s.repo.Create(ctx, job); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.Printf("Job created: %s for course %s", job.ID, job.CourseID)
	return job, nil
}

func (s *jobServiceImpl) GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.GetJob")
	defer span.End()

	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get job %s: %w", id, err)
	}

	return job, nil
}

func (s *jobServiceImpl) ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error) {
	ctx, span := s.tracer.Start(ctx, "JobService.ListJobs")
	defer span.End()

	filters := JobFilters{
		Status:   status,
		CourseID: courseID,
		Limit:    100, // Default limit
	}

	jobs, err := s.repo.List(ctx, filters)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return jobs, nil
}

func (s *jobServiceImpl) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	ctx, span := s.tracer.Start(ctx, "JobService.UpdateJobStatus")
	defer span.End()

	if err := s.repo.UpdateStatus(ctx, id, status, progress, errorMsg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("Job %s status updated to %s (progress: %d%%)", id, status, progress)
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
