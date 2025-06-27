package jobs

import (
	"context"
	"ocf-worker/pkg/models"
	
	"github.com/google/uuid"
)

type JobService interface {
	CreateJob(ctx context.Context, req *models.GenerationRequest) (*models.GenerationJob, error)
	GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error)
	ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error)
	UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error
}

// Implementation simple pour commencer
type jobService struct {
	// TODO: ajouter database repository
}

func NewJobService() JobService {
	return &jobService{}
}

func (s *jobService) CreateJob(ctx context.Context, req *models.GenerationRequest) (*models.GenerationJob, error) {
	// TODO: implémenter avec base de données
	job := &models.GenerationJob{
		ID:          req.JobID,
		CourseID:    req.CourseID,
		Status:      models.StatusPending,
		SourcePath:  req.SourcePath,
		CallbackURL: req.CallbackURL,
		Metadata:    req.Metadata,
	}
	return job, nil
}

func (s *jobService) GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	// TODO: implémenter avec base de données
	return nil, nil
}

func (s *jobService) ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error) {
	// TODO: implémenter avec base de données
	return []*models.GenerationJob{}, nil
}

func (s *jobService) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	// TODO: implémenter avec base de données
	return nil
}
