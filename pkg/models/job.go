package models

import (
	"time"
	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusTimeout    JobStatus = "timeout"
)

type GenerationJob struct {
	ID          uuid.UUID            `json:"id" gorm:"type:uuid;primary_key"`
	CourseID    uuid.UUID            `json:"course_id" gorm:"type:uuid;not null"`
	Status      JobStatus            `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	Progress    int                  `json:"progress" gorm:"default:0"`
	SourcePath  string               `json:"source_path" gorm:"type:text"`
	ResultPath  string               `json:"result_path" gorm:"type:text"`
	CallbackURL string               `json:"callback_url" gorm:"type:text"`
	Error       string               `json:"error,omitempty" gorm:"type:text"`
	Logs        []string             `json:"logs" gorm:"type:json"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:json"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	StartedAt   *time.Time           `json:"started_at,omitempty"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
}

type GenerationRequest struct {
	JobID       uuid.UUID            `json:"job_id" binding:"required"`
	CourseID    uuid.UUID            `json:"course_id" binding:"required"`
	SourcePath  string               `json:"source_path" binding:"required"`
	CallbackURL string               `json:"callback_url,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type JobResponse struct {
	ID          uuid.UUID            `json:"id"`
	CourseID    uuid.UUID            `json:"course_id"`
	Status      JobStatus            `json:"status"`
	Progress    int                  `json:"progress"`
	ResultPath  string               `json:"result_path,omitempty"`
	Error       string               `json:"error,omitempty"`
	Logs        []string             `json:"logs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	StartedAt   *time.Time           `json:"started_at,omitempty"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
}

func (j *GenerationJob) ToResponse() *JobResponse {
	return &JobResponse{
		ID:          j.ID,
		CourseID:    j.CourseID,
		Status:      j.Status,
		Progress:    j.Progress,
		ResultPath:  j.ResultPath,
		Error:       j.Error,
		Logs:        j.Logs,
		Metadata:    j.Metadata,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
	}
}
