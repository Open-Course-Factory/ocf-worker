package models

import "time"

// ArchiveResponse représente la réponse de création d'archive
// @Description Résultat de la création d'une archive de résultats
type ArchiveResponse struct {
	Message     string      `json:"message" example:"Archive created successfully"`
	Archive     ArchiveInfo `json:"archive"`
	CreatedAt   time.Time   `json:"created_at" example:"2025-01-17T10:30:00Z"`
	DownloadURL string      `json:"download_url" example:"/api/v1/storage/courses/550e8400-e29b-41d4-a716-446655440002/results/archive-20250117-103000.zip"`
} // @name ArchiveResponse

// ArchiveRequest contient les paramètres de création d'archive
// @Description Requête de la création d'une archive de résultats
type ArchiveRequest struct {
	Format   ArchiveFormat `json:"format" binding:"required"`
	Include  []string      `json:"include,omitempty"` // Patterns de fichiers à inclure
	Exclude  []string      `json:"exclude,omitempty"` // Patterns de fichiers à exclure
	Compress bool          `json:"compress"`          // Compression activée
} // @name ArchiveRequest

// ArchiveFormat définit les formats d'archive supportés
type ArchiveFormat string

const (
	FormatZIP ArchiveFormat = "zip"
	FormatTAR ArchiveFormat = "tar"
)

// ArchiveInfo contient les informations sur une archive
// @Description Métadonnées d'une archive créée
type ArchiveInfo struct {
	Filename     string  `json:"filename" example:"archive-20250117-103000.zip"`
	Format       string  `json:"format" example:"zip" enums:"zip,tar"`
	FilesCount   int     `json:"files_count" example:"15"`
	Compressed   bool    `json:"compressed" example:"true"`
	SizeBytes    int64   `json:"size_bytes" example:"2097152"`
	SizeMB       float64 `json:"size_mb" example:"2.0"`
	CreationTime string  `json:"creation_time" example:"250ms"`
} // @name ArchiveInfo
