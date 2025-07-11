// internal/validation/middleware.go
package validation

import (
	"ocf-worker/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestValidator définit une fonction de validation pour une requête
type RequestValidator func(*gin.Context, *APIValidator) *ValidationResult

// ValidateRequest est le middleware principal qui exécute une liste de validators
func ValidateRequest(validators ...RequestValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		validator := GetValidator(c)
		if validator == nil {
			c.JSON(500, gin.H{"error": "Validation service unavailable"})
			c.Abort()
			return
		}

		// Exécuter toutes les validations dans l'ordre
		for _, validate := range validators {
			if result := validate(c, validator); !result.Valid {
				c.JSON(400, gin.H{
					"error":             "Validation failed",
					"validation_errors": result.Errors,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// GetValidator helper pour récupérer le validator du contexte (déjà existant dans middleware.go)
func GetValidator(c *gin.Context) *APIValidator {
	if validator, exists := c.Get("validator"); exists {
		if apiValidator, ok := validator.(*APIValidator); ok {
			return apiValidator
		}
	}
	return nil
}

func ValidateJobIDParam(paramName string) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		jobIDStr := c.Param(paramName)

		// Utiliser votre méthode existante
		_, result := v.ValidateJobIDParam(jobIDStr)

		// Optionnel : stocker l'UUID parsé pour éviter de re-parser dans le handler
		if result.Valid {
			if jobID, err := uuid.Parse(jobIDStr); err == nil {
				c.Set("validated_job_id", jobID)
			}
		}

		return result
	}
}

func ValidateGenerationRequest(c *gin.Context, v *APIValidator) *ValidationResult {
	// Récupérer la requête déjà parsée par Gin
	req, exists := c.Get("parsed_request")
	if !exists {
		// Si pas encore parsée, on la parse ici (fallback)
		var parsedReq models.GenerationRequest
		if err := c.ShouldBindJSON(&parsedReq); err != nil {
			// Laisser Gin gérer cette erreur - on ne devrait pas arriver ici
			return &ValidationResult{Valid: false, Errors: []*ValidationError{{
				Field: "json", Value: "", Message: "JSON parsing failed", Code: "JSON_PARSE_ERROR",
			}}}
		}
		req = parsedReq
	}

	// Cast vers le bon type
	generationReq := req.(models.GenerationRequest)

	// Faire seulement la validation métier
	result := v.ValidateGenerationRequest(&generationReq)

	// Stocker pour le handler
	if result.Valid {
		c.Set("validated_request", generationReq)
	}

	return result
}

func ParseJSONRequest[T any]() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req T
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{
				"error":   "Invalid JSON format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		c.Set("parsed_request", req)
		c.Next()
	}
}

// Version spécialisée pour GenerationRequest
func ParseGenerationRequest() gin.HandlerFunc {
	return ParseJSONRequest[models.GenerationRequest]()
}

// CombineValidators combine plusieurs validators (tous doivent passer)
func CombineValidators(validators ...RequestValidator) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		for _, validator := range validators {
			if result := validator(c, v); !result.Valid {
				return result // Arrêter à la première erreur
			}
		}
		return &ValidationResult{Valid: true}
	}
}

// ValidateFilenameParam valide un nom de fichier depuis l'URL
func ValidateFilenameParam(paramName string) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		filename := c.Param(paramName)
		result := v.ValidateFilenameParam(filename)

		if result.Valid {
			// Sanitiser et stocker
			sanitized := v.SanitizeFilename(filename)
			c.Set("validated_filename", sanitized)
		}

		return result
	}
}

// ValidateCourseIDParam valide un paramètre course_id depuis l'URL
func ValidateCourseIDParam(paramName string) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		courseIDStr := c.Param(paramName)
		courseID, result := v.ValidateCourseIDParam(courseIDStr)

		if result.Valid {
			c.Set("validated_course_id", courseID)
		}

		return result
	}
}

// ValidateFileUpload valide un upload de fichiers multipart
func ValidateFileUpload(c *gin.Context, v *APIValidator) *ValidationResult {
	form, err := c.MultipartForm()
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []*ValidationError{{
				Field:   "files",
				Value:   "",
				Message: "Failed to parse multipart form: " + err.Error(),
				Code:    "MULTIPART_PARSE_ERROR",
			}},
		}
	}

	files := form.File["files"]
	if len(files) == 0 {
		return &ValidationResult{
			Valid: false,
			Errors: []*ValidationError{{
				Field:   "files",
				Value:   "",
				Message: "No files provided",
				Code:    "NO_FILES",
			}},
		}
	}

	result := v.ValidateFileUpload(files)

	if result.Valid {
		c.Set("validated_files", files)
	}

	return result
}

// ValidateStatusParam valide un paramètre status depuis l'URL ou query
func ValidateStatusParam(paramName string) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		status := c.Query(paramName)
		result := v.ValidateStatusParam(status)

		if result.Valid {
			// Stocker le status validé (peut être vide)
			c.Set("validated_status", status)
		}

		return result
	}
}

// ValidatePaginationParams valide les paramètres de pagination depuis les query params
func ValidatePaginationParams(c *gin.Context, v *APIValidator) *ValidationResult {
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	pagination, result := v.ValidatePaginationParams(limitStr, offsetStr)

	if result.Valid {
		c.Set("validated_pagination", *pagination)
	}

	return result
}

// ValidateOptionalCourseIDParam valide un paramètre course_id optionnel depuis les query params
func ValidateOptionalCourseIDParam(paramName string) RequestValidator {
	return func(c *gin.Context, v *APIValidator) *ValidationResult {
		courseIDStr := c.Query(paramName)

		// Si pas de course_id, c'est valide
		if courseIDStr == "" {
			c.Set("validated_course_id", (*uuid.UUID)(nil))
			return &ValidationResult{Valid: true}
		}

		// Sinon valider normalement
		courseID, result := v.ValidateCourseIDParam(courseIDStr)

		if result.Valid {
			c.Set("validated_course_id", &courseID)
		}

		return result
	}
}

// ValidateListJobsParams valide tous les paramètres pour l'endpoint ListJobs
func ValidateListJobsParams(c *gin.Context, v *APIValidator) *ValidationResult {
	statusParam := c.Query("status")
	courseIDParam := c.Query("course_id")
	limitParam := c.Query("limit")
	offsetParam := c.Query("offset")

	// Utiliser la méthode du validator API
	params, result := v.ValidateListJobsParams(statusParam, courseIDParam, limitParam, offsetParam)

	if result.Valid {
		// Stocker les paramètres validés individuellement pour compatibilité
		c.Set("validated_status", params.Status)
		c.Set("validated_course_id", params.CourseID)
		c.Set("validated_pagination", params.Pagination)

		// Stocker aussi l'objet complet
		c.Set("validated_list_params", *params)
	}

	return result
}

// ValidateWorkspaceListParams valide les paramètres de listing des workspaces
func ValidateWorkspaceListParams(c *gin.Context, v *APIValidator) *ValidationResult {
	statusParam := c.Query("status")
	limitParam := c.Query("limit")
	offsetParam := c.Query("offset")

	params, result := v.ValidateWorkspaceListParams(statusParam, limitParam, offsetParam)

	if result.Valid {
		c.Set("validated_workspace_list_params", *params)
	}

	return result
}

// ValidateWorkspaceCleanupParams valide les paramètres de nettoyage des workspaces
func ValidateWorkspaceCleanupParams(c *gin.Context, v *APIValidator) *ValidationResult {
	maxAgeParam := c.Query("max_age_hours")

	params, result := v.ValidateWorkspaceCleanupParams(maxAgeParam)

	if result.Valid {
		c.Set("validated_workspace_cleanup_params", *params)
	}

	return result
}
