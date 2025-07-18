package jobs

import (
	"context"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/pkg/models"

	"github.com/google/uuid"
)

type JobService interface {
	CreateJob(ctx context.Context, req *models.GenerationRequest) (*models.GenerationJob, error)
	GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error)
	ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error)
	UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error
	AddJobLog(ctx context.Context, id uuid.UUID, logEntry string) error
	CleanupOldJobs(ctx context.Context, maxAge time.Duration) (int64, error)
}
