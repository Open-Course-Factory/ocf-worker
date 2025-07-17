// pkg/models/swagger.go
package models

import (
	"time"
)

// ErrorResponse représente une réponse d'erreur standard
// @Description Réponse d'erreur standard de l'API
type ErrorResponse struct {
	Error            string            `json:"error" example:"Validation failed"`
	Message          string            `json:"message,omitempty" example:"Detailed error message"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
	Timestamp        time.Time         `json:"timestamp" example:"2025-01-17T10:30:00Z"`
	Path             string            `json:"path,omitempty" example:"/api/v1/generate"`
	RequestID        string            `json:"request_id,omitempty" example:"req-123456"`
} // @name ErrorResponse

// ValidationError représente une erreur de validation spécifique
// @Description Détail d'une erreur de validation
type ValidationError struct {
	Field   string `json:"field" example:"job_id"`
	Value   string `json:"value" example:"invalid-uuid"`
	Message string `json:"message" example:"job ID must be a valid UUID"`
	Code    string `json:"code" example:"INVALID_UUID"`
} // @name ValidationError

// SuccessResponse représente une réponse de succès générique
// @Description Réponse de succès standard
type SuccessResponse struct {
	Message   string      `json:"message" example:"Operation completed successfully"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp" example:"2025-01-17T10:30:00Z"`
} // @name SuccessResponse

// HealthResponse représente la réponse du health check
// @Description Statut de santé du service
type HealthResponse struct {
	Status      string    `json:"status" example:"healthy" enums:"healthy,degraded,unhealthy"`
	Service     string    `json:"service" example:"ocf-worker"`
	Version     string    `json:"version" example:"2.0.0"`
	Timestamp   time.Time `json:"timestamp" example:"2025-01-17T10:30:00Z"`
	Uptime      string    `json:"uptime,omitempty" example:"24h30m15s"`
	Environment string    `json:"environment,omitempty" example:"development"`
} // @name HealthResponse

// WorkerStats représente les statistiques du pool de workers
// @Description Statistiques détaillées du pool de workers
type WorkerStats struct {
	WorkerCount   int          `json:"worker_count" example:"3"`
	QueueSize     int          `json:"queue_size" example:"5"`
	QueueCapacity int          `json:"queue_capacity" example:"20"`
	Running       bool         `json:"running" example:"true"`
	Workers       []WorkerInfo `json:"workers"`
} // @name WorkerStats

// WorkerInfo représente les informations d'un worker individuel
// @Description Informations sur un worker spécifique
type WorkerInfo struct {
	ID           int    `json:"id" example:"1"`
	Status       string `json:"status" example:"busy" enums:"idle,busy,stopped"`
	CurrentJobID string `json:"current_job_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	JobsTotal    int64  `json:"jobs_total" example:"150"`
	JobsSuccess  int64  `json:"jobs_success" example:"145"`
	JobsFailed   int64  `json:"jobs_failed" example:"5"`
} // @name WorkerInfo

// StorageInfo représente les informations du système de stockage
// @Description Informations sur la configuration du stockage
type StorageInfo struct {
	StorageType string            `json:"storage_type" example:"garage" enums:"filesystem,garage"`
	Endpoints   map[string]string `json:"endpoints"`
	Status      string            `json:"status" example:"healthy"`
	Capacity    *StorageCapacity  `json:"capacity,omitempty"`
} // @name StorageInfo

// StorageCapacity représente la capacité de stockage
// @Description Informations sur la capacité de stockage
type StorageCapacity struct {
	Total     int64   `json:"total_bytes" example:"1073741824"`    // 1GB
	Used      int64   `json:"used_bytes" example:"268435456"`      // 256MB
	Available int64   `json:"available_bytes" example:"805306368"` // 768MB
	Usage     float64 `json:"usage_percent" example:"25.0"`
} // @name StorageCapacity

// FileUploadResponse représente la réponse d'upload de fichiers
// @Description Réponse après upload de fichiers
type FileUploadResponse struct {
	Message string   `json:"message" example:"files uploaded successfully"`
	JobID   string   `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Count   int      `json:"count" example:"3"`
	Files   []string `json:"files,omitempty" example:"slides.md,theme.css,config.json"`
} // @name FileUploadResponse

// FileListResponse représente la liste de fichiers
// @Description Liste de fichiers dans le stockage
type FileListResponse struct {
	JobID    string   `json:"job_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	CourseID string   `json:"course_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440002"`
	Files    []string `json:"files" example:"index.html,assets/style.css,assets/script.js"`
	Count    int      `json:"count" example:"3"`
} // @name FileListResponse

// ThemeInfo représente les informations d'un thème Slidev
// @Description Informations sur un thème Slidev
type ThemeInfo struct {
	Name        string `json:"name" example:"@slidev/theme-seriph"`
	Version     string `json:"version" example:"0.22.7"`
	Installed   bool   `json:"installed" example:"true"`
	Description string `json:"description" example:"Seriph theme with elegant typography"`
	Homepage    string `json:"homepage" example:"https://github.com/slidevjs/themes"`
} // @name ThemeInfo

// ThemeInstallRequest représente une demande d'installation de thème
// @Description Requête pour installer un thème Slidev
type ThemeInstallRequest struct {
	Theme string `json:"theme" example:"@slidev/theme-seriph" binding:"required"`
} // @name ThemeInstallRequest

// ThemeInstallResponse représente la réponse d'installation de thème
// @Description Réponse après installation d'un thème
type ThemeInstallResponse struct {
	Theme     string `json:"theme" example:"@slidev/theme-seriph"`
	Success   bool   `json:"success" example:"true"`
	Installed bool   `json:"installed" example:"true"`
	Message   string `json:"message,omitempty" example:"Theme installed successfully"`
	Error     string `json:"error,omitempty"`
} // @name ThemeInstallResponse
