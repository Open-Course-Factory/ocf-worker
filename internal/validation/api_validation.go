// internal/validation/api_validation.go - Validation spécifique à l'API

package validation

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-Course-Factory/ocf-worker/pkg/models"

	"github.com/google/uuid"
)

// APIValidator gère la validation des requêtes API
type APIValidator struct {
	validationService *ValidationService
}

// PaginationParams contient les paramètres de pagination validés
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ListJobsParams struct {
	Status     string           `json:"status"`
	CourseID   *uuid.UUID       `json:"course_id,omitempty"`
	Pagination PaginationParams `json:"pagination"`
}

// WorkspaceListParams contient les paramètres validés pour lister les workspaces
type WorkspaceListParams struct {
	Status     string           `json:"status"`
	Pagination PaginationParams `json:"pagination"`
}

// WorkspaceCleanupParams contient les paramètres validés pour le nettoyage
type WorkspaceCleanupParams struct {
	MaxAgeHours int `json:"max_age_hours"`
}

// NewAPIValidator crée un nouveau validateur d'API
func NewAPIValidator(config *ValidationConfig) *APIValidator {
	return &APIValidator{
		validationService: NewValidationService(config),
	}
}

// ValidateGenerationRequest valide une requête de génération
func (av *APIValidator) ValidateGenerationRequest(req *models.GenerationRequest) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Valider Job ID
	jobIDResult := av.validationService.ValidateJobID(req.JobID.String())
	if !jobIDResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, jobIDResult.Errors...)
	}

	// Valider Course ID
	courseIDResult := av.validationService.ValidateCourseID(req.CourseID.String())
	if !courseIDResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, courseIDResult.Errors...)
	}

	// Valider Source Path
	sourcePathResult := av.validationService.ValidateSourcePath(req.SourcePath)
	if !sourcePathResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, sourcePathResult.Errors...)
	}

	// Valider Callback URL
	callbackResult := av.validationService.ValidateCallbackURL(req.CallbackURL)
	if !callbackResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, callbackResult.Errors...)
	}

	// Valider Metadata
	metadataResult := av.validationService.ValidateMetadata(req.Metadata)
	if !metadataResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, metadataResult.Errors...)
	}

	return result
}

// ValidateFileUpload valide un upload de fichiers
func (av *APIValidator) ValidateFileUpload(files []*multipart.FileHeader) *ValidationResult {
	return av.validationService.ValidateFiles(files)
}

// ValidateJobIDParam valide un paramètre job_id depuis l'URL
func (av *APIValidator) ValidateJobIDParam(jobIDStr string) (uuid.UUID, *ValidationResult) {
	result := av.validationService.ValidateJobID(jobIDStr)

	if !result.Valid {
		return uuid.Nil, result
	}

	// Parse the UUID (we know it's valid from validation above)
	jobID, _ := uuid.Parse(jobIDStr)
	return jobID, result
}

// ValidateCourseIDParam valide un paramètre course_id depuis l'URL
func (av *APIValidator) ValidateCourseIDParam(courseIDStr string) (uuid.UUID, *ValidationResult) {
	result := av.validationService.ValidateCourseID(courseIDStr)

	if !result.Valid {
		return uuid.Nil, result
	}

	// Parse the UUID (we know it's valid from validation above)
	courseID, _ := uuid.Parse(courseIDStr)
	return courseID, result
}

// ValidateFilenameParam valide un paramètre filename depuis l'URL
func (av *APIValidator) ValidateFilenameParam(filename string) *ValidationResult {
	return av.validationService.ValidateFilename(filename, false)
}

// ValidateListParams valide les paramètres de listage (status, limit, offset)
func (av *APIValidator) ValidateListParams(status string, limit, offset int) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Valider le status s'il est fourni
	if status != "" {
		validStatuses := []string{"pending", "processing", "completed", "failed", "timeout"}
		isValid := false
		for _, validStatus := range validStatuses {
			if status == validStatus {
				isValid = true
				break
			}
		}

		if !isValid {
			result.AddError("status", status,
				"invalid status (must be: pending, processing, completed, failed, timeout)",
				"INVALID_STATUS")
		}
	}

	// Valider limit
	if limit < 0 {
		result.AddError("limit", string(rune(limit)), "limit cannot be negative", "NEGATIVE_LIMIT")
	} else if limit > 1000 {
		result.AddError("limit", string(rune(limit)), "limit too large (max 1000)", "LIMIT_TOO_LARGE")
	}

	// Valider offset
	if offset < 0 {
		result.AddError("offset", string(rune(offset)), "offset cannot be negative", "NEGATIVE_OFFSET")
	}

	return result
}

// SanitizeFilename nettoie un nom de fichier en supprimant les caractères dangereux
func (av *APIValidator) SanitizeFilename(filename string) string {
	// Séparer l'extension du nom de base pour la protéger
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	if base == "" && ext != "" {
		base = "hidden_file" // cas des fichiers cachés
	}

	// Nettoyer le nom de base
	// 1. Remplacer les séquences .. par un underscore
	base = regexp.MustCompile(`\.\.+`).ReplaceAllString(base, "_")

	// 2. Remplacer les autres caractères dangereux
	dangerous := regexp.MustCompile(`[\/\\:*?"<>|]+`)
	base = dangerous.ReplaceAllString(base, "_")

	base = regexp.MustCompile(`_+`).ReplaceAllString(base, "_")

	// 3. Supprimer les underscores en début et fin
	base = strings.Trim(base, "_")

	// traitements sur l'extension
	ext = strings.Trim(ext, "_")

	if len(ext) < 2 {
		ext = ""
	}

	if base == "" {
		base = "unnamed_file"
	}

	if base == "hidden_file" {
		base = ""
	}

	// Reconstruire le nom complet
	sanitized := base + ext

	// Limiter la longueur totale
	if len(sanitized) > 200 {
		if len(ext) < 200 {
			maxBaseLen := 200 - len(ext)
			if len(base) > maxBaseLen {
				base = base[:maxBaseLen]
			}
			sanitized = base + ext
		} else {
			// Extension trop longue (cas très rare)
			sanitized = sanitized[:200]
		}
	}

	return sanitized
}

// ValidateContentSafety effectue une validation de sécurité du contenu
func (av *APIValidator) ValidateContentSafety(content []byte, filename string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Vérifier la taille
	if len(content) > 50*1024*1024 { // 50MB max
		result.AddError("content", filename, "content too large", "CONTENT_TOO_LARGE")
	}

	// Vérifications spécifiques par type de fichier
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".js":
		// Vérifier les scripts potentiellement malveillants
		contentStr := string(content)
		if strings.Contains(contentStr, "eval(") ||
			strings.Contains(contentStr, "Function(") ||
			strings.Contains(contentStr, "setTimeout(") ||
			strings.Contains(contentStr, "setInterval(") {
			result.AddError("content", filename,
				"JavaScript content contains potentially dangerous functions",
				"DANGEROUS_JS_CONTENT")
		}

	case ".html":
		// Vérifier les balises script
		contentStr := string(content)
		if strings.Contains(strings.ToLower(contentStr), "<script") {
			result.AddError("content", filename,
				"HTML content contains script tags",
				"SCRIPT_TAGS_NOT_ALLOWED")
		}

	case ".md":
		// Vérifier les liens externes suspects
		contentStr := string(content)
		if strings.Contains(contentStr, "javascript:") {
			result.AddError("content", filename,
				"Markdown content contains javascript: links",
				"JAVASCRIPT_LINKS_NOT_ALLOWED")
		}
	}

	// Vérifier qu'il n'y a pas de caractères de contrôle dangereux
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		for i, b := range content {
			if b < 32 && b != 9 && b != 10 && b != 13 { // Permettre tab, LF, CR
				result.AddError("content", filename,
					fmt.Sprintf("content contains control character at position %d", i),
					"CONTROL_CHARACTERS")
				break // Ne signaler qu'une fois
			}
		}
	}

	return result
}

// ValidateStatusParam valide un paramètre status de job (optionnel)
func (av *APIValidator) ValidateStatusParam(status string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Si status est vide, c'est valide (paramètre optionnel)
	if status == "" {
		return result
	}

	// Valider que le status est dans la liste autorisée
	validStatuses := []string{"pending", "processing", "completed", "failed", "timeout"}
	isValid := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		result.AddError("status", status,
			"invalid status (must be: pending, processing, completed, failed, timeout)",
			"INVALID_STATUS")
	}

	return result
}

// ValidatePaginationParams valide les paramètres de pagination avec valeurs par défaut
func (av *APIValidator) ValidatePaginationParams(limitStr, offsetStr string) (*PaginationParams, *ValidationResult) {
	result := &ValidationResult{Valid: true}

	// Valeurs par défaut
	limit := 100
	offset := 0

	// Valider limit si fourni
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err != nil {
			result.AddError("limit", limitStr, "limit must be a valid integer", "INVALID_LIMIT")
		} else if parsedLimit < 0 {
			result.AddError("limit", limitStr, "limit cannot be negative", "NEGATIVE_LIMIT")
		} else if parsedLimit > 1000 {
			result.AddError("limit", limitStr, "limit too large (max 1000)", "LIMIT_TOO_LARGE")
		} else {
			limit = parsedLimit
		}
	}

	// Valider offset si fourni
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err != nil {
			result.AddError("offset", offsetStr, "offset must be a valid integer", "INVALID_OFFSET")
		} else if parsedOffset < 0 {
			result.AddError("offset", offsetStr, "offset cannot be negative", "NEGATIVE_OFFSET")
		} else {
			offset = parsedOffset
		}
	}

	pagination := &PaginationParams{
		Limit:  limit,
		Offset: offset,
	}

	return pagination, result
}

// ValidateListJobsParams valide tous les paramètres pour ListJobs
func (av *APIValidator) ValidateListJobsParams(statusParam, courseIDParam, limitParam, offsetParam string) (*ListJobsParams, *ValidationResult) {
	result := &ValidationResult{Valid: true}

	// Valider le status
	statusResult := av.ValidateStatusParam(statusParam)
	if !statusResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, statusResult.Errors...)
	}

	// Valider course_id si fourni
	var courseID *uuid.UUID
	if courseIDParam != "" {
		parsedCourseID, courseValidation := av.ValidateCourseIDParam(courseIDParam)
		if !courseValidation.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, courseValidation.Errors...)
		} else {
			courseID = &parsedCourseID
		}
	}

	// Valider la pagination
	pagination, paginationResult := av.ValidatePaginationParams(limitParam, offsetParam)
	if !paginationResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, paginationResult.Errors...)
	}

	params := &ListJobsParams{
		Status:     statusParam,
		CourseID:   courseID,
		Pagination: *pagination,
	}

	return params, result
}

// ValidateWorkspaceListParams valide les paramètres de listing des workspaces
func (av *APIValidator) ValidateWorkspaceListParams(statusParam, limitParam, offsetParam string) (*WorkspaceListParams, *ValidationResult) {
	result := &ValidationResult{Valid: true}

	// Valider la pagination (réutiliser la logique existante)
	pagination, paginationResult := av.ValidatePaginationParams(limitParam, offsetParam)
	if !paginationResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, paginationResult.Errors...)
	}

	// Valider le status optionnel (peut être "active", "idle", "stopped", etc.)
	if statusParam != "" {
		validStatuses := []string{"active", "idle", "stopped", "busy"}
		isValid := false
		for _, validStatus := range validStatuses {
			if statusParam == validStatus {
				isValid = true
				break
			}
		}

		if !isValid {
			result.AddError("status", statusParam,
				"invalid workspace status (must be: active, idle, stopped, busy)",
				"INVALID_WORKSPACE_STATUS")
		}
	}

	params := &WorkspaceListParams{
		Status:     statusParam,
		Pagination: *pagination,
	}

	return params, result
}

// ValidateWorkspaceCleanupParams valide les paramètres de nettoyage des workspaces
func (av *APIValidator) ValidateWorkspaceCleanupParams(maxAgeParam string) (*WorkspaceCleanupParams, *ValidationResult) {
	result := &ValidationResult{Valid: true}

	// Valeur par défaut
	maxAgeHours := 24

	if maxAgeParam != "" {
		if parsed, err := strconv.Atoi(maxAgeParam); err != nil {
			result.AddError("max_age_hours", maxAgeParam,
				"max_age_hours must be a valid integer", "INVALID_MAX_AGE")
		} else if parsed < 1 {
			result.AddError("max_age_hours", maxAgeParam,
				"max_age_hours must be at least 1", "MIN_MAX_AGE")
		} else if parsed > 8760 { // 1 an max
			result.AddError("max_age_hours", maxAgeParam,
				"max_age_hours too large (max 8760 hours = 1 year)", "MAX_AGE_TOO_LARGE")
		} else {
			maxAgeHours = parsed
		}
	}

	params := &WorkspaceCleanupParams{
		MaxAgeHours: maxAgeHours,
	}

	return params, result
}

// SanitizeFilePath nettoie un chemin de fichier en préservant la structure de dossiers
func (av *APIValidator) SanitizeFilePath(filePath string) string {
	if filePath == "" {
		return "unnamed_file"
	}

	// Normaliser les séparateurs de chemin
	filePath = filepath.ToSlash(filePath)

	// Supprimer les chemins absolus et path traversal
	filePath = strings.TrimPrefix(filePath, "/")

	// Diviser en segments de chemin
	segments := strings.Split(filePath, "/")
	var cleanSegments []string

	for _, segment := range segments {
		// Ignorer les segments vides et les path traversal
		if segment == "" || segment == "." || segment == ".." {
			continue
		}

		// Nettoyer chaque segment individuellement
		cleanSegment := av.sanitizeSegment(segment)
		if cleanSegment != "" {
			cleanSegments = append(cleanSegments, cleanSegment)
		}
	}

	if len(cleanSegments) == 0 {
		return "unnamed_file"
	}

	// Reconstruire le chemin
	cleanPath := strings.Join(cleanSegments, "/")

	// Limiter la profondeur des dossiers
	const maxDepth = 10
	if len(cleanSegments) > maxDepth {
		cleanSegments = cleanSegments[len(cleanSegments)-maxDepth:]
		cleanPath = strings.Join(cleanSegments, "/")
	}

	return cleanPath
}

// sanitizeSegment nettoie un segment de chemin individuel
func (av *APIValidator) sanitizeSegment(segment string) string {
	// Utiliser la logique existante de SanitizeFilename pour chaque segment
	return av.SanitizeFilename(segment)
}

// ValidateFilePath valide un chemin de fichier complet
func (av *APIValidator) ValidateFilePath(filePath string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if filePath == "" {
		result.AddError("file_path", "", "file path is required", "REQUIRED")
		return result
	}

	// Vérifier la longueur totale
	if len(filePath) > 1000 {
		result.AddError("file_path", filePath, "file path too long (max 1000 characters)", "PATH_TOO_LONG")
	}

	// Normaliser et diviser le chemin
	normalizedPath := filepath.ToSlash(filePath)
	segments := strings.Split(strings.TrimPrefix(normalizedPath, "/"), "/")

	// Vérifier chaque segment
	directory := true
	for i, segment := range segments {
		if segment == "" && i != len(segments)-1 {
			continue // Permettre les segments vides sauf le dernier
		}

		if i == len(segments)-1 {
			directory = false
		}

		// Valider chaque segment comme un nom de fichier/dossier
		segmentResult := av.validationService.ValidateFilename(segment, directory)
		if !segmentResult.Valid {
			result.Valid = false
			for _, err := range segmentResult.Errors {
				err.Field = fmt.Sprintf("file_path[%d]", i)
				result.Errors = append(result.Errors, err)
			}
		}
	}

	// Vérifier la profondeur
	if len(segments) > 10 {
		result.AddError("file_path", filePath, "path too deep (max 10 levels)", "PATH_TOO_DEEP")
	}

	return result
}

// ExtractFilePathFromMultipart extrait le chemin complet depuis un header multipart
func (av *APIValidator) ExtractFilePathFromMultipart(fileHeader *multipart.FileHeader) string {
	filename := fileHeader.Filename

	// Vérifier les headers customisés pour le chemin
	if contentDisp := fileHeader.Header.Get("Content-Disposition"); contentDisp != "" {
		// Chercher un paramètre 'filename' customisé
		if matches := regexp.MustCompile(`filename="([^"]+)"`).FindStringSubmatch(contentDisp); len(matches) > 1 {
			return matches[1]
		}
	}

	// Fallback sur le filename standard
	return filename
}
