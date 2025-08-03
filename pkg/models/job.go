package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusTimeout    JobStatus = "timeout"
)

// JSON type for PostgreSQL compatibility
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSON", value)
	}

	if len(bytes) == 0 {
		*j = make(map[string]interface{})
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// StringSlice type for PostgreSQL JSON arrays
type StringSlice []string

func (ss StringSlice) Value() (driver.Value, error) {
	if ss == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(ss)
}

func (ss *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*ss = []string{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into StringSlice", value)
	}

	if len(bytes) == 0 {
		*ss = []string{}
		return nil
	}

	return json.Unmarshal(bytes, ss)
}

// GenerationJob est le modèle principal pour la base de données
type GenerationJob struct {
	ID          uuid.UUID   `json:"id" gorm:"type:uuid;primary_key"`
	CourseID    uuid.UUID   `json:"course_id" gorm:"type:uuid;not null;index"`
	Status      JobStatus   `json:"status" gorm:"type:varchar(20);not null;default:'pending';index"`
	Progress    int         `json:"progress" gorm:"default:0;check:progress >= 0 AND progress <= 100"`
	SourcePath  string      `json:"source_path" gorm:"type:text;not null"`
	ResultPath  string      `json:"result_path" gorm:"type:text"`
	CallbackURL string      `json:"callback_url" gorm:"type:text"`
	Error       string      `json:"error,omitempty" gorm:"type:text"`
	Logs        StringSlice `json:"logs" gorm:"type:jsonb;default:'[]'"`
	Metadata    JSON        `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time   `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time   `json:"updated_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty" gorm:"index"`
	CompletedAt *time.Time  `json:"completed_at,omitempty" gorm:"index"`
}

// TableName spécifie le nom de la table
func (GenerationJob) TableName() string {
	return "generation_jobs"
}

// BeforeCreate hook GORM pour initialiser l'ID et les timestamps
func (j *GenerationJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now()
	j.CreatedAt = now
	j.UpdatedAt = now

	// Initialiser les slices si nil
	if j.Logs == nil {
		j.Logs = StringSlice{}
	}
	if j.Metadata == nil {
		j.Metadata = JSON{}
	}

	return nil
}

// BeforeUpdate hook GORM pour mettre à jour le timestamp
func (j *GenerationJob) BeforeUpdate(tx *gorm.DB) error {
	j.UpdatedAt = time.Now()
	return nil
}

// GenerationRequest représente une demande de génération de cours
// @Description Requête pour créer un nouveau job de génération
type GenerationRequest struct {
	JobID       uuid.UUID              `json:"job_id" binding:"required"`
	CourseID    uuid.UUID              `json:"course_id" binding:"required"`
	SourcePath  string                 `json:"source_path" binding:"required"`
	CallbackURL string                 `json:"callback_url,omitempty"`
	Packages    []string               `json:"packages,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
} // @name GenerationRequest

// JobResponse représente la réponse contenant les détails d'un job
// @Description Détails complets d'un job de génération
type JobResponse struct {
	ID          uuid.UUID              `json:"id"`
	CourseID    uuid.UUID              `json:"course_id"`
	Status      JobStatus              `json:"status"`
	Progress    int                    `json:"progress"`
	SourcePath  string                 `json:"source_path"`
	ResultPath  string                 `json:"result_path,omitempty"`
	CallbackURL string                 `json:"callback_url,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
} // @name JobResponse

// ToResponse convertit un GenerationJob en JobResponse
func (j *GenerationJob) ToResponse() *JobResponse {
	// Convertir les types personnalisés en types standard
	logs := []string(j.Logs)
	metadata := map[string]interface{}(j.Metadata)

	return &JobResponse{
		ID:          j.ID,
		CourseID:    j.CourseID,
		Status:      j.Status,
		Progress:    j.Progress,
		SourcePath:  j.SourcePath,
		ResultPath:  j.ResultPath,
		CallbackURL: j.CallbackURL,
		Error:       j.Error,
		Logs:        logs,
		Metadata:    metadata,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
	}
}

// JobListResponse représente une liste de jobs
// @Description Liste paginée de jobs
type JobListResponse struct {
	Jobs       []JobResponse `json:"jobs"`
	Count      int           `json:"count" example:"25"`
	TotalCount int           `json:"total_count,omitempty" example:"150"`
	Page       int           `json:"page,omitempty" example:"1"`
	PageSize   int           `json:"page_size,omitempty" example:"25"`
} // @name JobListResponse

// IsTerminal retourne true si le job est dans un état final
func (j *GenerationJob) IsTerminal() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusTimeout
}

// IsActive retourne true si le job est en cours de traitement
func (j *GenerationJob) IsActive() bool {
	return j.Status == StatusPending || j.Status == StatusProcessing
}

// SetStatus met à jour le statut avec les timestamps appropriés
func (j *GenerationJob) SetStatus(status JobStatus) {
	j.Status = status
	j.UpdatedAt = time.Now()

	if status == StatusProcessing && j.StartedAt == nil {
		now := time.Now()
		j.StartedAt = &now
	}

	if j.IsTerminal() && j.CompletedAt == nil {
		now := time.Now()
		j.CompletedAt = &now
	}
}

// AddLog ajoute un log avec timestamp
func (j *GenerationJob) AddLog(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)
	j.Logs = append(j.Logs, logEntry)
	j.UpdatedAt = time.Now()
}
