package models

import "time"

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

// WorkerStatsResponse représente les statistiques détaillées du worker
// @Description Statistiques complètes du pool de workers et de leurs performances
type WorkerStatsResponse struct {
	WorkerPool WorkerPoolStats `json:"worker_pool"`
	Timestamp  time.Time       `json:"timestamp" example:"2025-01-17T10:30:00Z"`
} // @name WorkerStatsResponse

// WorkerPoolStats contient les statistiques du pool de workers
// @Description Métriques détaillées du pool de workers
type WorkerPoolStats struct {
	WorkerCount   int               `json:"worker_count" example:"3"`
	QueueSize     int               `json:"queue_size" example:"5"`
	QueueCapacity int               `json:"queue_capacity" example:"20"`
	QueueUsage    float64           `json:"queue_usage_percent" example:"25.0"`
	Running       bool              `json:"running" example:"true"`
	Workers       []WorkerInfo      `json:"workers"`
	Performance   WorkerPerformance `json:"performance"`
} // @name WorkerPoolStats

// WorkerPerformance contient les métriques de performance
// @Description Métriques de performance globales du pool de workers
type WorkerPerformance struct {
	JobsPerMinute       float64 `json:"jobs_per_minute" example:"12.5"`
	AverageJobDuration  string  `json:"average_job_duration" example:"2m15s"`
	SuccessRate         float64 `json:"success_rate_percent" example:"95.2"`
	TotalJobsProcessed  int64   `json:"total_jobs_processed" example:"1250"`
	TotalJobsSuccessful int64   `json:"total_jobs_successful" example:"1190"`
	TotalJobsFailed     int64   `json:"total_jobs_failed" example:"60"`
} // @name WorkerPerformance

// WorkerHealthResponse représente l'état de santé du système de workers
// @Description État de santé détaillé du système de workers
type WorkerHealthResponse struct {
	Status     string           `json:"status" example:"healthy" enums:"healthy,degraded,unhealthy"`
	WorkerPool WorkerPoolHealth `json:"worker_pool"`
	Issues     []string         `json:"issues,omitempty" example:"1 worker is overloaded"`
	Timestamp  time.Time        `json:"timestamp" example:"2025-01-17T10:30:00Z"`
	Uptime     string           `json:"uptime" example:"24h30m15s"`
} // @name WorkerHealthResponse

// WorkerPoolHealth contient les métriques de santé du pool
// @Description Indicateurs de santé du pool de workers
type WorkerPoolHealth struct {
	Running       bool    `json:"running" example:"true"`
	WorkerCount   int     `json:"worker_count" example:"3"`
	ActiveWorkers int     `json:"active_workers" example:"2"`
	IdleWorkers   int     `json:"idle_workers" example:"1"`
	QueueSize     int     `json:"queue_size" example:"3"`
	QueueUsage    float64 `json:"queue_usage_percent" example:"15.0"`
	OverloadRisk  bool    `json:"overload_risk" example:"false"`
} // @name WorkerPoolHealth
