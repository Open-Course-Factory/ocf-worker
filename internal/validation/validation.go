// internal/validation/validation.go - Service de validation robuste

package validation

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ValidationConfig contient la configuration de validation
type ValidationConfig struct {
	MaxFileSize       int64           // Taille max par fichier (bytes)
	MaxTotalSize      int64           // Taille max totale (bytes)
	MaxFiles          int             // Nombre max de fichiers
	AllowedExtensions map[string]bool // Extensions autorisées
	MaxFilenameLength int             // Longueur max du nom de fichier
	AllowedMimeTypes  map[string]bool // Types MIME autorisés
}

// DefaultValidationConfig retourne une configuration par défaut sécurisée
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxFileSize:       10 * 1024 * 1024, // 10MB par fichier
		MaxTotalSize:      50 * 1024 * 1024, // 50MB total
		MaxFiles:          20,               // 20 fichiers max
		MaxFilenameLength: 255,              // 255 caractères max
		AllowedExtensions: map[string]bool{
			".md":    true, // Markdown
			".css":   true, // Styles
			".js":    true, // JavaScript
			".json":  true, // Configuration
			".png":   true, // Images
			".jpg":   true,
			".jpeg":  true,
			".gif":   true,
			".svg":   true,
			".woff":  true, // Fonts
			".woff2": true,
			".ttf":   true,
			".eot":   true,
			".ico":   true, // Icon
			".txt":   true, // Texte
			".yml":   true, // YAML
			".yaml":  true,
		},
		AllowedMimeTypes: map[string]bool{
			"text/plain":               true,
			"text/markdown":            true,
			"text/css":                 true,
			"application/javascript":   true,
			"application/json":         true,
			"image/png":                true,
			"image/jpeg":               true,
			"image/gif":                true,
			"image/svg+xml":            true,
			"font/woff":                true,
			"font/woff2":               true,
			"application/font-woff":    true,
			"application/x-font-woff":  true,
			"application/octet-stream": true, // Pour les fonts
		},
	}
}

// ValidationService gère la validation des entrées
type ValidationService struct {
	config *ValidationConfig
}

// NewValidationService crée un nouveau service de validation
func NewValidationService(config *ValidationConfig) *ValidationService {
	if config == nil {
		config = DefaultValidationConfig()
	}

	return &ValidationService{
		config: config,
	}
}

// ValidationError représente une erreur de validation avec détails
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// ValidationResult contient le résultat de validation
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
}

// AddError ajoute une erreur de validation
func (vr *ValidationResult) AddError(field, value, message, code string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

// ValidateJobID valide un ID de job
func (vs *ValidationService) ValidateJobID(jobID string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if jobID == "" {
		result.AddError("job_id", "", "job ID is required", "REQUIRED")
		return result
	}

	// Vérifier que c'est un UUID valide
	if _, err := uuid.Parse(jobID); err != nil {
		result.AddError("job_id", jobID, "job ID must be a valid UUID", "INVALID_UUID")
	}

	return result
}

// ValidateCourseID valide un ID de cours
func (vs *ValidationService) ValidateCourseID(courseID string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if courseID == "" {
		result.AddError("course_id", "", "course ID is required", "REQUIRED")
		return result
	}

	// Vérifier que c'est un UUID valide
	if _, err := uuid.Parse(courseID); err != nil {
		result.AddError("course_id", courseID, "course ID must be a valid UUID", "INVALID_UUID")
	}

	return result
}

// ValidateFilename valide un nom de fichier de manière robuste
func (vs *ValidationService) ValidateFilename(filename string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if filename == "" {
		result.AddError("filename", "", "filename is required", "REQUIRED")
		return result
	}

	// Vérifier la longueur
	if len(filename) > vs.config.MaxFilenameLength {
		result.AddError("filename", filename,
			fmt.Sprintf("filename too long (max %d characters)", vs.config.MaxFilenameLength),
			"TOO_LONG")
	}

	// Vérifier que c'est un UTF-8 valide
	if !utf8.ValidString(filename) {
		result.AddError("filename", filename, "filename must be valid UTF-8", "INVALID_ENCODING")
	}

	// Vérifier les caractères interdits
	forbiddenChars := []string{
		"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|",
		"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07",
		"\x08", "\x09", "\x0a", "\x0b", "\x0c", "\x0d", "\x0e", "\x0f",
	}

	for _, char := range forbiddenChars {
		if strings.Contains(filename, char) {
			result.AddError("filename", filename,
				fmt.Sprintf("filename contains forbidden character: %q", char),
				"FORBIDDEN_CHAR")
		}
	}

	// Vérifier les noms réservés (Windows)
	reservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	baseName := strings.ToUpper(strings.TrimSuffix(filename, filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if baseName == reserved {
			result.AddError("filename", filename,
				fmt.Sprintf("filename uses reserved name: %s", reserved),
				"RESERVED_NAME")
		}
	}

	// Vérifier que le nom ne commence/finit pas par un espace ou un point
	if strings.HasPrefix(filename, " ") || strings.HasSuffix(filename, " ") ||
		strings.HasPrefix(filename, ".") || strings.HasSuffix(filename, ".") {
		result.AddError("filename", filename,
			"filename cannot start or end with space or dot",
			"INVALID_FORMAT")
	}

	// Vérifier l'extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		result.AddError("filename", filename, "filename must have an extension", "NO_EXTENSION")
	} else if !vs.config.AllowedExtensions[ext] {
		result.AddError("filename", filename,
			fmt.Sprintf("file extension %s not allowed", ext),
			"FORBIDDEN_EXTENSION")
	}

	return result
}

// ValidateFileHeader valide un header de fichier multipart
func (vs *ValidationService) ValidateFileHeader(header *multipart.FileHeader) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Valider le nom de fichier
	filenameResult := vs.ValidateFilename(header.Filename)
	if !filenameResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, filenameResult.Errors...)
	}

	// Vérifier la taille
	if header.Size > vs.config.MaxFileSize {
		result.AddError("file_size", fmt.Sprintf("%d", header.Size),
			fmt.Sprintf("file too large (max %d bytes)", vs.config.MaxFileSize),
			"FILE_TOO_LARGE")
	}

	if header.Size == 0 {
		result.AddError("file_size", "0", "file is empty", "EMPTY_FILE")
	}

	// Vérifier le type MIME si disponible
	if len(header.Header["Content-Type"]) > 0 {
		contentType := header.Header["Content-Type"][0]
		// Extraire le type principal (avant les paramètres)
		mainType := strings.Split(contentType, ";")[0]
		if !vs.config.AllowedMimeTypes[mainType] {
			result.AddError("content_type", contentType,
				fmt.Sprintf("content type %s not allowed", mainType),
				"FORBIDDEN_MIME_TYPE")
		}
	}

	return result
}

// ValidateFiles valide un ensemble de fichiers
func (vs *ValidationService) ValidateFiles(files []*multipart.FileHeader) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Vérifier le nombre de fichiers
	if len(files) == 0 {
		result.AddError("files", "", "no files provided", "NO_FILES")
		return result
	}

	if len(files) > vs.config.MaxFiles {
		result.AddError("files", fmt.Sprintf("%d files", len(files)),
			fmt.Sprintf("too many files (max %d)", vs.config.MaxFiles),
			"TOO_MANY_FILES")
	}

	// Vérifier la taille totale et valider chaque fichier
	var totalSize int64
	filenames := make(map[string]bool) // Détecter les doublons

	for i, file := range files {
		// Valider le fichier individuel
		fileResult := vs.ValidateFileHeader(file)
		if !fileResult.Valid {
			result.Valid = false
			// Préfixer les erreurs avec l'index du fichier
			for _, err := range fileResult.Errors {
				err.Field = fmt.Sprintf("files[%d].%s", i, err.Field)
			}
			result.Errors = append(result.Errors, fileResult.Errors...)
		}

		// Vérifier les doublons
		if filenames[file.Filename] {
			result.AddError(fmt.Sprintf("files[%d].filename", i), file.Filename,
				"duplicate filename", "DUPLICATE_FILENAME")
		}
		filenames[file.Filename] = true

		totalSize += file.Size
	}

	// Vérifier la taille totale
	if totalSize > vs.config.MaxTotalSize {
		result.AddError("total_size", fmt.Sprintf("%d", totalSize),
			fmt.Sprintf("total size too large (max %d bytes)", vs.config.MaxTotalSize),
			"TOTAL_SIZE_TOO_LARGE")
	}

	return result
}

// ValidateCallbackURL valide une URL de callback
func (vs *ValidationService) ValidateCallbackURL(url string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if url == "" {
		// URL de callback optionnelle
		return result
	}

	// Regex plus stricte pour les URLs HTTP/HTTPS
	urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(?::[0-9]+)?(?:/[a-zA-Z0-9._~:/?#[\]@!$&'()*+,;=%-]*)?$`)
	if !urlRegex.MatchString(url) {
		result.AddError("callback_url", url, "invalid URL format", "INVALID_URL")
	}

	// Vérifier la longueur
	if len(url) > 2048 {
		result.AddError("callback_url", url, "URL too long (max 2048 characters)", "URL_TOO_LONG")
	}

	// Interdire les URLs localhost/127.0.0.1 en production
	if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") || strings.Contains(url, "0.0.0.0") {
		result.AddError("callback_url", url, "localhost URLs not allowed", "LOCALHOST_NOT_ALLOWED")
	}

	return result
}

// ValidateSourcePath valide un chemin source
func (vs *ValidationService) ValidateSourcePath(path string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if path == "" {
		result.AddError("source_path", "", "source path is required", "REQUIRED")
		return result
	}

	// Vérifier les caractères dangereux
	if strings.Contains(path, "..") {
		result.AddError("source_path", path, "path traversal not allowed", "PATH_TRAVERSAL")
	}

	// Vérifier la longueur
	if len(path) > 500 {
		result.AddError("source_path", path, "path too long (max 500 characters)", "PATH_TOO_LONG")
	}

	return result
}

// ValidateMetadata valide les métadonnées
func (vs *ValidationService) ValidateMetadata(metadata map[string]interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if metadata == nil {
		return result // Métadonnées optionnelles
	}

	// Limiter le nombre de clés
	if len(metadata) > 50 {
		result.AddError("metadata", fmt.Sprintf("%d keys", len(metadata)),
			"too many metadata keys (max 50)", "TOO_MANY_KEYS")
	}

	// Valider chaque clé et valeur
	for key, value := range metadata {
		// Valider la clé
		if len(key) > 100 {
			result.AddError("metadata", key,
				fmt.Sprintf("metadata key too long (max 100 characters): %s", key),
				"KEY_TOO_LONG")
		}

		// Valider la valeur (convertir en string pour la validation)
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 1000 {
			result.AddError("metadata", key,
				fmt.Sprintf("metadata value too long (max 1000 characters) for key: %s", key),
				"VALUE_TOO_LONG")
		}
	}

	return result
}
