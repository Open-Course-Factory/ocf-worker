package models

import "time"

// SystemMetrics représente les métriques système globales
// @Description Métriques système et de performance globales
type SystemMetrics struct {
	Timestamp   time.Time       `json:"timestamp" example:"2025-01-17T10:30:00Z"`
	Uptime      string          `json:"uptime" example:"24h30m15s"`
	Version     string          `json:"version" example:"2.0.0"`
	Environment string          `json:"environment" example:"production"`
	Storage     StorageMetrics  `json:"storage"`
	Worker      WorkerMetrics   `json:"worker"`
	Database    DatabaseMetrics `json:"database"`
} // @name SystemMetrics

// StorageMetrics contient les métriques de stockage
// @Description Métriques du système de stockage
type StorageMetrics struct {
	Type           string `json:"type" example:"garage" enums:"filesystem,garage"`
	TotalFiles     int64  `json:"total_files" example:"1250"`
	TotalSizeBytes int64  `json:"total_size_bytes" example:"1073741824"`
	TotalSizeMB    int64  `json:"total_size_mb" example:"1024"`
	SourceFiles    int64  `json:"source_files" example:"800"`
	ResultFiles    int64  `json:"result_files" example:"450"`
	Healthy        bool   `json:"healthy" example:"true"`
} // @name StorageMetrics

// WorkerMetrics contient les métriques des workers
// @Description Métriques de performance des workers
type WorkerMetrics struct {
	TotalWorkers       int     `json:"total_workers" example:"3"`
	ActiveWorkers      int     `json:"active_workers" example:"2"`
	JobsProcessedToday int64   `json:"jobs_processed_today" example:"145"`
	JobsPerHour        float64 `json:"jobs_per_hour" example:"12.5"`
	AverageJobTime     string  `json:"average_job_time" example:"2m15s"`
	SuccessRate24h     float64 `json:"success_rate_24h_percent" example:"96.2"`
	QueueSize          int     `json:"queue_size" example:"3"`
	WorkspaceCount     int     `json:"workspace_count" example:"15"`
} // @name WorkerMetrics

// DatabaseMetrics contient les métriques de la base de données
// @Description Métriques de performance de la base de données
type DatabaseMetrics struct {
	Connected       bool    `json:"connected" example:"true"`
	TotalJobs       int64   `json:"total_jobs" example:"2450"`
	JobsToday       int64   `json:"jobs_today" example:"145"`
	PendingJobs     int     `json:"pending_jobs" example:"3"`
	ProcessingJobs  int     `json:"processing_jobs" example:"2"`
	CompletedJobs   int64   `json:"completed_jobs" example:"2300"`
	FailedJobs      int64   `json:"failed_jobs" example:"145"`
	QueryTimeMs     float64 `json:"average_query_time_ms" example:"15.2"`
	ConnectionsUsed int     `json:"connections_used" example:"5"`
	ConnectionsMax  int     `json:"connections_max" example:"20"`
} // @name DatabaseMetrics
