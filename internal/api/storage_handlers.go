package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"ocf-worker/internal/storage"
	"ocf-worker/internal/validation"
	_ "ocf-worker/pkg/models"
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

// UploadJobSources upload des fichiers sources pour un job
// @Summary Upload des fichiers sources
// @Description Upload des fichiers sources (slides.md, CSS, images, etc.) pour un job de génération
// @Description
// @Description Types de fichiers supportés:
// @Description - `.md` - Fichiers Markdown (slides)
// @Description - `.css` - Feuilles de style
// @Description - `.js` - Scripts JavaScript
// @Description - `.json` - Fichiers de configuration
// @Description - `.png`, `.jpg`, `.gif`, `.svg` - Images
// @Description - `.woff`, `.woff2`, `.ttf` - Polices
// @Tags Storage
// @Accept multipart/form-data
// @Produce json
// @Param job_id path string true "ID du job" Format(uuid)
// @Param files formData file true "Fichiers à uploader (multiple autorisé)"
// @Success 201 {object} models.FileUploadResponse "Fichiers uploadés avec succès"
// @Failure 400 {object} models.ErrorResponse "Erreur de validation (taille, type, etc.)"
// @Failure 413 {object} models.ErrorResponse "Fichier trop volumineux"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/jobs/{job_id}/sources [post]
func (h *StorageHandlers) UploadJobSources(c *gin.Context) {
	// Récupérer les données déjà validées
	jobID := c.MustGet("validated_job_id").(uuid.UUID)
	files := c.MustGet("validated_files").([]*multipart.FileHeader)

	// Se concentrer sur la logique métier : sanitisation et sécurité
	validator := validation.GetValidator(c) // Toujours nécessaire pour la sanitisation
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

		// Validation du contenu de sécurité
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

	// Upload les fichiers
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

// ListJobSources liste les fichiers sources d'un job
// @Summary Lister les fichiers sources
// @Description Liste tous les fichiers sources uploadés pour un job donné
// @Tags Storage
// @Accept json
// @Produce json
// @Param job_id path string true "ID du job" Format(uuid)
// @Success 200 {object} models.FileListResponse "Liste des fichiers sources"
// @Failure 400 {object} models.ErrorResponse "ID du job invalide"
// @Failure 404 {object} models.ErrorResponse "Job non trouvé ou aucun fichier"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/jobs/{job_id}/sources [get]
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

// DownloadJobSource télécharge un fichier source spécifique
// @Summary Télécharger un fichier source
// @Description Télécharge un fichier source spécifique d'un job par son nom
// @Tags Storage
// @Accept json
// @Produce application/octet-stream
// @Produce text/markdown
// @Produce text/css
// @Produce application/javascript
// @Param job_id path string true "ID du job" Format(uuid)
// @Param filename path string true "Nom du fichier à télécharger"
// @Success 200 {file} file "Contenu du fichier"
// @Header 200 {string} Content-Type "Type MIME du fichier"
// @Header 200 {string} Content-Disposition "attachment; filename=..."
// @Failure 400 {object} models.ErrorResponse "Paramètres invalides"
// @Failure 404 {object} models.ErrorResponse "Fichier non trouvé"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/jobs/{job_id}/sources/{filename} [get]
func (h *StorageHandlers) DownloadJobSource(c *gin.Context) {
	// Récupérer les paramètres déjà validés
	jobID := c.MustGet("validated_job_id").(uuid.UUID)
	filename := c.MustGet("validated_filename").(string)

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
// @Summary Télécharger un résultat généré
// @Description Télécharge un fichier spécifique des résultats générés d'un cours
// @Tags Storage
// @Accept json
// @Produce application/octet-stream
// @Produce text/html
// @Produce text/css
// @Produce application/javascript
// @Param course_id path string true "ID du cours" Format(uuid)
// @Param filename path string true "Nom du fichier à télécharger"
// @Success 200 {file} file "Contenu du fichier généré"
// @Header 200 {string} Content-Type "Type MIME du fichier"
// @Failure 400 {object} models.ErrorResponse "Paramètres invalides"
// @Failure 404 {object} models.ErrorResponse "Fichier non trouvé"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/courses/{course_id}/results/{filename} [get]
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
// @Summary Lister les résultats générés
// @Description Liste tous les fichiers générés (HTML, CSS, JS, assets) pour un cours
// @Description
// @Description Les résultats incluent généralement:
// @Description - `index.html` - Page principale de la présentation
// @Description - `assets/` - Ressources (CSS, JS, images)
// @Description - Autres fichiers générés par Slidev
// @Tags Storage
// @Accept json
// @Produce json
// @Param course_id path string true "ID du cours" Format(uuid)
// @Success 200 {object} models.FileListResponse "Liste des fichiers de résultat"
// @Failure 400 {object} models.ErrorResponse "ID du cours invalide"
// @Failure 404 {object} models.ErrorResponse "Cours non trouvé ou aucun résultat"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/courses/{course_id}/results [get]
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

// GetJobLogs récupère les logs d'exécution d'un job
// @Summary Récupérer les logs d'un job
// @Description Récupère les logs détaillés d'exécution d'un job (build Slidev, erreurs, etc.)
// @Tags Storage
// @Accept json
// @Produce text/plain
// @Param job_id path string true "ID du job" Format(uuid)
// @Success 200 {string} string "Logs du job (format texte)"
// @Header 200 {string} Content-Type "text/plain"
// @Failure 400 {object} models.ErrorResponse "ID du job invalide"
// @Failure 404 {object} models.ErrorResponse "Logs non trouvés"
// @Failure 500 {object} models.ErrorResponse "Erreur de stockage"
// @Router /storage/jobs/{job_id}/logs [get]
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

// GetStorageInfo retourne des informations sur le système de stockage
// @Summary Informations sur le stockage
// @Description Retourne les informations de configuration et l'état du système de stockage
// @Tags Storage
// @Accept json
// @Produce json
// @Success 200 {object} models.StorageInfo "Informations sur le stockage"
// @Failure 500 {object} models.ErrorResponse "Erreur interne du serveur"
// @Router /storage/info [get]
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
