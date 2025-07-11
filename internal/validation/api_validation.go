// internal/validation/api_validation.go - Validation spécifique à l'API

package validation

import (
	"fmt"
	"mime/multipart"
	"ocf-worker/pkg/models"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
	return av.validationService.ValidateFilename(filename)
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

	// 3. Supprimer les points isolés en début/milieu (mais pas l'extension)
	// Remplacer les points isolés ou multiples par underscore, sauf s'ils sont suivis d'une extension valide
	base = regexp.MustCompile(`^\.+|\.+`).ReplaceAllString(base, "_")

	base = regexp.MustCompile(`_+`).ReplaceAllString(base, "_")

	// 4. Supprimer les underscores en début et fin
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
	for i, b := range content {
		if b < 32 && b != 9 && b != 10 && b != 13 { // Permettre tab, LF, CR
			result.AddError("content", filename,
				fmt.Sprintf("content contains control character at position %d", i),
				"CONTROL_CHARACTERS")
			break // Ne signaler qu'une fois
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
