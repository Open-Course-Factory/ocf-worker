// internal/api/archive_handlers.go - Nouvelle fonctionnalité d'archive
package api

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/internal/storage"
	"github.com/Open-Course-Factory/ocf-worker/internal/validation"
	"github.com/Open-Course-Factory/ocf-worker/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ArchiveHandlers gère les endpoints d'archive
type ArchiveHandlers struct {
	storageService *storage.StorageService
}

// NewArchiveHandlers crée un nouveau gestionnaire d'archives
func NewArchiveHandlers(storageService *storage.StorageService) *ArchiveHandlers {
	return &ArchiveHandlers{
		storageService: storageService,
	}
}

// DownloadResultsArchive crée et télécharge une archive des résultats d'un cours
// @Summary Download course results as archive
// @Description Creates and downloads a ZIP archive containing all result files for a course
// @Tags Archive
// @Accept json
// @Produce application/zip
// @Param course_id path string true "Course ID"
// @Param format query string false "Archive format (zip, tar)" default(zip)
// @Param compress query bool false "Enable compression" default(true)
// @Success 200 {file} archive "Archive file"
// @Failure 400 {object} map[string]interface{} "Validation error"
// @Failure 404 {object} map[string]interface{} "Course not found"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Router /api/v1/storage/courses/{course_id}/archive [get]
func (h *ArchiveHandlers) DownloadResultsArchive(c *gin.Context) {
	// Récupérer les paramètres validés
	courseID := c.MustGet("validated_course_id").(uuid.UUID)

	// Paramètres optionnels
	format := models.ArchiveFormat(c.DefaultQuery("format", "zip"))
	compress := c.DefaultQuery("compress", "true") == "true"

	// Validation du format
	if format != models.FormatZIP && format != models.FormatTAR {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported archive format. Supported: zip, tar",
		})
		return
	}

	// Lister les fichiers de résultat
	resultFiles, err := h.storageService.ListResults(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list result files: " + err.Error(),
		})
		return
	}

	if len(resultFiles) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No result files found for this course",
		})
		return
	}

	// Préparer les headers de réponse
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("course-%s-results-%s.%s",
		courseID.String()[:8], timestamp, format)

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("X-Archive-Files-Count", fmt.Sprintf("%d", len(resultFiles)))

	// Créer l'archive en streaming
	if err := h.createArchiveStream(c.Writer, courseID, resultFiles, format, compress); err != nil {
		// Headers déjà envoyés, on ne peut plus renvoyer d'erreur JSON
		c.Header("X-Archive-Error", err.Error())
		return
	}
}

// createArchiveStream crée une archive en streaming directement vers la réponse
func (h *ArchiveHandlers) createArchiveStream(w io.Writer, courseID uuid.UUID, files []string, format models.ArchiveFormat, compress bool) error {
	switch format {
	case models.FormatZIP:
		return h.createZipStream(w, courseID, files, compress)
	case models.FormatTAR:
		return h.createTarStream(w, courseID, files, compress)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// createZipStream crée une archive ZIP en streaming
func (h *ArchiveHandlers) createZipStream(w io.Writer, courseID uuid.UUID, files []string, compress bool) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, filename := range files {
		// Télécharger le fichier depuis le storage
		reader, err := h.storageService.DownloadResult(context.Background(), courseID, filename)
		if err != nil {
			return fmt.Errorf("failed to download file %s: %w", filename, err)
		}

		// Créer l'entrée dans le ZIP
		var zipFileWriter io.Writer
		if compress {
			zipFileWriter, err = zipWriter.Create(filename)
		} else {
			header := &zip.FileHeader{
				Name:   filename,
				Method: zip.Store, // Pas de compression
			}
			zipFileWriter, err = zipWriter.CreateHeader(header)
		}

		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", filename, err)
		}

		// Copier le contenu
		if _, err := io.Copy(zipFileWriter, reader); err != nil {
			return fmt.Errorf("failed to write file %s to archive: %w", filename, err)
		}
	}

	return nil
}

// createTarStream crée une archive TAR en streaming
func (h *ArchiveHandlers) createTarStream(w io.Writer, courseID uuid.UUID, files []string, compress bool) error {
	// TODO: Implémenter le support TAR si nécessaire
	return fmt.Errorf("TAR format not yet implemented")
}

// createArchiveInMemory crée une archive en mémoire
func (h *ArchiveHandlers) createArchiveInMemory(courseID uuid.UUID, files []string, format models.ArchiveFormat, compress bool) (io.Reader, error) {
	// TODO: Implémenter la création d'archive en mémoire pour CreateResultsArchive
	return nil, fmt.Errorf("in-memory archive creation not yet implemented")
}

// filterFiles filtre les fichiers selon les patterns include/exclude
func (h *ArchiveHandlers) filterFiles(files []string, include, exclude []string) []string {
	if len(include) == 0 && len(exclude) == 0 {
		return files
	}

	var filtered []string

	for _, file := range files {
		// Vérifier include patterns
		if len(include) > 0 {
			matched := false
			for _, pattern := range include {
				if matched, _ = filepath.Match(pattern, file); matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Vérifier exclude patterns
		excluded := false
		for _, pattern := range exclude {
			if matched, _ := filepath.Match(pattern, file); matched {
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// Validation pour les paramètres d'archive
func ValidateArchiveParams(c *gin.Context, v *validation.APIValidator) *validation.ValidationResult {
	result := &validation.ValidationResult{Valid: true}

	// Valider le format si présent
	if format := c.Query("format"); format != "" {
		if format != "zip" && format != "tar" {
			result.AddError("format", format, "unsupported archive format (supported: zip, tar)", "INVALID_FORMAT")
		}
	}

	// Valider compress si présent
	if compress := c.Query("compress"); compress != "" {
		if compress != "true" && compress != "false" {
			result.AddError("compress", compress, "compress must be true or false", "INVALID_BOOLEAN")
		}
	}

	return result
}
