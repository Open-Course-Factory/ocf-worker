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
