package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"ocf-worker/internal/storage"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StorageHandlers struct {
	storageService *storage.StorageService
}

func NewStorageHandlers(storageService *storage.StorageService) *StorageHandlers {
	return &StorageHandlers{
		storageService: storageService,
	}
}

// UploadJobSources upload des fichiers source pour un job
func (h *StorageHandlers) UploadJobSources(c *gin.Context) {
	validator := GetValidator(c)
	if validator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation service unavailable"})
		return
	}
	jobIDStr := c.Param("job_id")

	jobID, validationResult := validator.ValidateJobIDParam(jobIDStr)
	if !validationResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid job ID",
			"validation_errors": validationResult.Errors,
		})
		return
	}

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files provided"})
		return
	}

	// Valider les fichiers
	fileValidation := validator.ValidateFileUpload(files)
	if !fileValidation.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "File validation failed",
			"validation_errors": fileValidation.Errors,
		})
		return
	}

	// 👈 Sanitiser les noms de fichiers et valider le contenu
	var processedFiles []*multipart.FileHeader
	for _, fileHeader := range files {
		// Sanitiser le nom de fichier
		sanitizedName := validator.SanitizeFilename(fileHeader.Filename)
		if sanitizedName != fileHeader.Filename {
			// Créer une nouvelle structure avec le nom sanitisé
			newHeader := *fileHeader
			newHeader.Filename = sanitizedName
			processedFiles = append(processedFiles, &newHeader)
		} else {
			processedFiles = append(processedFiles, fileHeader)
		}

		// 👈 Validation du contenu de sécurité
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open file: " + fileHeader.Filename})
			return
		}

		// Lire le contenu pour la validation (attention à la mémoire)
		content := make([]byte, min(fileHeader.Size, 1024*1024)) // Max 1MB pour validation
		n, _ := file.Read(content)
		file.Close()

		contentValidation := validator.ValidateContentSafety(content[:n], sanitizedName)
		if !contentValidation.Valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":             "Content validation failed for file: " + fileHeader.Filename,
				"validation_errors": contentValidation.Errors,
			})
			return
		}
	}

	// Upload les fichiers (utiliser les fichiers traités)
	if err := h.storageService.UploadJobSources(c.Request.Context(), jobID, processedFiles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "files uploaded successfully",
		"job_id":  jobID,
		"count":   len(processedFiles),
	})
}

// ListJobSources liste les fichiers source d'un job
func (h *StorageHandlers) ListJobSources(c *gin.Context) {
	validator := GetValidator(c)
	if validator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation service unavailable"})
		return
	}
	jobIDStr := c.Param("job_id")

	jobID, validationResult := validator.ValidateJobIDParam(jobIDStr)
	if !validationResult.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid job ID",
			"validation_errors": validationResult.Errors,
		})
		return
	}

	files, err := h.storageService.ListJobSources(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id": jobID,
		"files":  files,
	})
}

// DownloadJobSource télécharge un fichier source
func (h *StorageHandlers) DownloadJobSource(c *gin.Context) {
	validator := GetValidator(c)
	if validator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation service unavailable"})
		return
	}
	jobIDStr := c.Param("job_id")

	jobID, jobValidation := validator.ValidateJobIDParam(jobIDStr)
	if !jobValidation.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid job ID",
			"validation_errors": jobValidation.Errors,
		})
		return
	}

	filename := c.Param("filename")
	filenameValidation := validator.ValidateFilenameParam(filename)
	if !filenameValidation.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Invalid filename",
			"validation_errors": filenameValidation.Errors,
		})
		return
	}

	reader, err := h.storageService.DownloadJobSource(c.Request.Context(), jobID, filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// Déterminer le content type
	contentType := "application/octet-stream"
	ext := filepath.Ext(filename)
	switch ext {
	case ".md":
		contentType = "text/markdown"
	case ".css":
		contentType = "text/css"
	case ".js":
		contentType = "application/javascript"
	case ".json":
		contentType = "application/json"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

// DownloadResult télécharge un fichier de résultat
func (h *StorageHandlers) DownloadResult(c *gin.Context) {
	courseIDStr := c.Param("course_id")
	courseID, err := uuid.Parse(courseIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid course ID"})
		return
	}

	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename required"})
		return
	}

	reader, err := h.storageService.DownloadResult(c.Request.Context(), courseID, filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	contentType := "application/octet-stream"
	ext := filepath.Ext(filename)
	switch ext {
	case ".html":
		contentType = "text/html"
	case ".css":
		contentType = "text/css"
	case ".js":
		contentType = "application/javascript"
	}

	c.Header("Content-Type", contentType)
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

// ListResults liste les fichiers de résultat d'un cours
func (h *StorageHandlers) ListResults(c *gin.Context) {
	courseIDStr := c.Param("course_id")
	courseID, err := uuid.Parse(courseIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid course ID"})
		return
	}

	files, err := h.storageService.ListResults(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"course_id": courseID,
		"files":     files,
	})
}

// GetJobLogs récupère les logs d'un job
func (h *StorageHandlers) GetJobLogs(c *gin.Context) {
	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	logs, err := h.storageService.GetJobLog(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "logs not found"})
		return
	}

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, logs)
}

// GetStorageInfo retourne des informations sur le storage
func (h *StorageHandlers) GetStorageInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"storage_type": "configured",
		"endpoints": gin.H{
			"upload_sources":  "/api/v1/storage/jobs/{job_id}/sources",
			"list_sources":    "/api/v1/storage/jobs/{job_id}/sources",
			"download_source": "/api/v1/storage/jobs/{job_id}/sources/{filename}",
			"list_results":    "/api/v1/storage/courses/{course_id}/results",
			"download_result": "/api/v1/storage/courses/{course_id}/results/{filename}",
			"get_logs":        "/api/v1/storage/jobs/{job_id}/logs",
		},
	})
}
