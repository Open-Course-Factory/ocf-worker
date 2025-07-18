package models

import (
	"time"
)

// WorkspaceInfo structure
// @Description Informations complètes sur un workspace
type WorkspaceInfo struct {
	JobID         string   `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Path          string   `json:"path" example:"/app/workspaces/550e8400-e29b-41d4-a716-446655440001"`
	DistPath      string   `json:"dist_path" example:"/app/workspaces/550e8400-e29b-41d4-a716-446655440001/dist"`
	Exists        bool     `json:"exists" example:"true"`
	SizeBytes     int64    `json:"size_bytes" example:"52428800"`
	FileCount     int      `json:"file_count" example:"25"`
	Files         []string `json:"files,omitempty" example:"slides.md,theme.css,package.json"`
	DistExists    bool     `json:"dist_exists" example:"true"`
	DistFileCount int      `json:"dist_file_count" example:"12"`
	DistFiles     []string `json:"dist_files,omitempty" example:"index.html,assets/style.css,assets/script.js"`
} // @name WorkspaceInfo

// WorkspaceListResponse représente la liste des workspaces
// @Description Liste paginée des workspaces actifs et leurs informations
type WorkspaceListResponse struct {
	Workspaces []WorkspaceInfo   `json:"workspaces"`
	Count      int               `json:"count" example:"15"`
	TotalCount int               `json:"total_count,omitempty" example:"45"`
	Page       int               `json:"page,omitempty" example:"1"`
	PageSize   int               `json:"page_size,omitempty" example:"25"`
	Summary    WorkspacesSummary `json:"summary"`
} // @name WorkspaceListResponse

// WorkspacesSummary contient un résumé des workspaces
// @Description Statistiques globales des workspaces
type WorkspacesSummary struct {
	TotalWorkspaces   int   `json:"total_workspaces" example:"45"`
	ActiveWorkspaces  int   `json:"active_workspaces" example:"12"`
	IdleWorkspaces    int   `json:"idle_workspaces" example:"33"`
	TotalSizeBytes    int64 `json:"total_size_bytes" example:"1073741824"`
	TotalSizeMB       int   `json:"total_size_mb" example:"1024"`
	AverageFilesPerWS int   `json:"average_files_per_workspace" example:"8"`
} // @name WorkspacesSummary

// WorkspaceInfoResponse représente les informations détaillées d'un workspace
// @Description Informations complètes et métriques d'un workspace spécifique
type WorkspaceInfoResponse struct {
	Workspace WorkspaceInfo     `json:"workspace"`
	Usage     WorkspaceUsage    `json:"usage"`
	Activity  WorkspaceActivity `json:"activity"`
} // @name WorkspaceInfoResponse

// WorkspaceUsage contient les métriques d'utilisation
// @Description Métriques d'utilisation détaillées du workspace
type WorkspaceUsage struct {
	DiskUsage        StorageUsage     `json:"disk_usage"`
	FileDistribution FileDistribution `json:"file_distribution"`
	BuildArtifacts   BuildArtifacts   `json:"build_artifacts"`
} // @name WorkspaceUsage

// StorageUsage représente l'utilisation du stockage
// @Description Métriques d'utilisation du stockage
type StorageUsage struct {
	TotalBytes  int64   `json:"total_bytes" example:"52428800"`
	TotalMB     float64 `json:"total_mb" example:"50.0"`
	SourceBytes int64   `json:"source_bytes" example:"2097152"`
	SourceMB    float64 `json:"source_mb" example:"2.0"`
	DistBytes   int64   `json:"dist_bytes,omitempty" example:"20971520"`
	DistMB      float64 `json:"dist_mb,omitempty" example:"20.0"`
	OtherBytes  int64   `json:"other_bytes" example:"29360128"`
	OtherMB     float64 `json:"other_mb" example:"28.0"`
} // @name StorageUsage

// FileDistribution représente la répartition des fichiers
// @Description Répartition des types de fichiers dans le workspace
type FileDistribution struct {
	TotalFiles  int `json:"total_files" example:"25"`
	SourceFiles int `json:"source_files" example:"8"`
	DistFiles   int `json:"dist_files,omitempty" example:"12"`
	ConfigFiles int `json:"config_files" example:"3"`
	AssetFiles  int `json:"asset_files" example:"2"`
} // @name FileDistribution

// BuildArtifacts contient les informations sur les artefacts de build
// @Description Informations sur les fichiers générés par le build
type BuildArtifacts struct {
	HasDist       bool      `json:"has_dist" example:"true"`
	DistCreatedAt time.Time `json:"dist_created_at,omitempty" example:"2025-01-17T10:25:00Z"`
	IndexHtmlSize int64     `json:"index_html_size,omitempty" example:"15432"`
	AssetCount    int       `json:"asset_count,omitempty" example:"8"`
	BuildSuccess  bool      `json:"build_success,omitempty" example:"true"`
} // @name BuildArtifacts

// WorkspaceActivity contient l'activité du workspace
// @Description Informations sur l'activité et l'historique du workspace
type WorkspaceActivity struct {
	Status        string    `json:"status" example:"idle" enums:"active,idle,building,completed,failed"`
	CreatedAt     time.Time `json:"created_at" example:"2025-01-17T10:20:00Z"`
	LastActivity  time.Time `json:"last_activity" example:"2025-01-17T10:25:00Z"`
	AgeDuration   string    `json:"age_duration" example:"10m30s"`
	JobStatus     string    `json:"job_status,omitempty" example:"completed"`
	BuildDuration string    `json:"build_duration,omitempty" example:"2m15s"`
} // @name WorkspaceActivity

// WorkspaceCleanupResponse représente le résultat de nettoyage d'un workspace
// @Description Résultat du nettoyage d'un workspace spécifique
type WorkspaceCleanupResponse struct {
	JobID        string  `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Cleaned      bool    `json:"cleaned" example:"true"`
	SizeFreed    int64   `json:"size_freed_bytes" example:"52428800"`
	SizeFreedMB  float64 `json:"size_freed_mb" example:"50.0"`
	FilesRemoved int     `json:"files_removed" example:"25"`
	CleanupTime  string  `json:"cleanup_time" example:"150ms"`
	Error        string  `json:"error,omitempty"`
} // @name WorkspaceCleanupResponse

// WorkspaceCleanupBatchResponse représente le résultat de nettoyage en lot
// @Description Résultat du nettoyage automatique des anciens workspaces
type WorkspaceCleanupBatchResponse struct {
	CleanedCount      int                        `json:"cleaned_count" example:"12"`
	TotalSizeFreed    int64                      `json:"total_size_freed_bytes" example:"629145600"`
	TotalSizeFreedMB  float64                    `json:"total_size_freed_mb" example:"600.0"`
	TotalFilesRemoved int                        `json:"total_files_removed" example:"300"`
	CleanupDuration   string                     `json:"cleanup_duration" example:"5.2s"`
	Details           []WorkspaceCleanupResponse `json:"details,omitempty"`
	Errors            []string                   `json:"errors,omitempty"`
	Summary           CleanupSummary             `json:"summary"`
} // @name WorkspaceCleanupBatchResponse

// CleanupSummary contient le résumé du nettoyage
// @Description Résumé des opérations de nettoyage
type CleanupSummary struct {
	TotalWorkspaces     int    `json:"total_workspaces" example:"50"`
	EligibleForCleanup  int    `json:"eligible_for_cleanup" example:"15"`
	SuccessfullyCleaned int    `json:"successfully_cleaned" example:"12"`
	FailedToClean       int    `json:"failed_to_clean" example:"3"`
	MaxAgeHours         int    `json:"max_age_hours" example:"24"`
	PerformanceGain     string `json:"performance_gain" example:"High"`
} // @name CleanupSummary

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
