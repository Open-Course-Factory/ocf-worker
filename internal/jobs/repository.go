package jobs

import (
	"context"
	"time"
	"ocf-worker/pkg/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobRepository interface {
	Create(ctx context.Context, job *models.GenerationJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error)
	List(ctx context.Context, filters JobFilters) ([]*models.GenerationJob, error)
	Update(ctx context.Context, job *models.GenerationJob) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error
	DeleteOldJobs(ctx context.Context, olderThan time.Time) (int64, error)
}

type JobFilters struct {
	Status   string
	CourseID *uuid.UUID
	Limit    int
	Offset   int
}

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, job *models.GenerationJob) error {
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *jobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	var job models.GenerationJob
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) List(ctx context.Context, filters JobFilters) ([]*models.GenerationJob, error) {
	var jobs []*models.GenerationJob
	
	query := r.db.WithContext(ctx).Model(&models.GenerationJob{})
	
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	
	if filters.CourseID != nil {
		query = query.Where("course_id = ?", *filters.CourseID)
	}
	
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}
	
	query = query.Order("created_at DESC")
	
	err := query.Find(&jobs).Error
	return jobs, err
}

func (r *jobRepository) Update(ctx context.Context, job *models.GenerationJob) error {
	job.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": time.Now(),
	}
	
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	
	if status == models.StatusProcessing {
		updates["started_at"] = time.Now()
	}
	
	if status == models.StatusCompleted || status == models.StatusFailed || status == models.StatusTimeout {
		updates["completed_at"] = time.Now()
	}
	
	return r.db.WithContext(ctx).Model(&models.GenerationJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *jobRepository) DeleteOldJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("created_at < ? AND status IN ?", olderThan, 
		[]models.JobStatus{models.StatusCompleted, models.StatusFailed, models.StatusTimeout}).
		Delete(&models.GenerationJob{})
	
	return result.RowsAffected, result.Error
}
