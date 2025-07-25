package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/Open-Course-Factory/ocf-worker/pkg/storage"
	"github.com/google/uuid"
)

type StorageService struct {
	storage storage.Storage
}

func NewStorageService(storage storage.Storage) *StorageService {
	return &StorageService{
		storage: storage,
	}
}

// UploadJobSources upload les fichiers source pour un job
func (s *StorageService) UploadJobSources(ctx context.Context, jobID uuid.UUID, files []*multipart.FileHeader) error {
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Extraire le chemin complet du fichier (peut inclure des dossiers)
		filePath := fileHeader.Filename

		// Construire le chemin complet: sources/{job_id}/{filepath}
		// Note: filePath peut maintenant contenir des dossiers comme "assets/images/logo.png"
		storagePath := fmt.Sprintf("sources/%s/%s", jobID.String(), filePath)

		if err := s.storage.Upload(ctx, storagePath, file); err != nil {
			return fmt.Errorf("failed to upload file %s: %w", filePath, err)
		}
	}

	return nil
}

// UploadJobSourceWithPath upload un fichier source avec un chemin explicite
func (s *StorageService) UploadJobSourceWithPath(ctx context.Context, jobID uuid.UUID, filePath string, content io.Reader) error {
	storagePath := fmt.Sprintf("sources/%s/%s", jobID.String(), filePath)
	return s.storage.Upload(ctx, storagePath, content)
}

// UploadJobSource upload un fichier source unique
func (s *StorageService) UploadJobSource(ctx context.Context, jobID uuid.UUID, filename string, content io.Reader) error {
	path := fmt.Sprintf("sources/%s/%s", jobID.String(), filename)
	return s.storage.Upload(ctx, path, content)
}

// DownloadJobSource télécharge un fichier source
func (s *StorageService) DownloadJobSource(ctx context.Context, jobID uuid.UUID, filename string) (io.Reader, error) {
	path := fmt.Sprintf("sources/%s/%s", jobID.String(), filename)
	return s.storage.Download(ctx, path)
}

// ListJobSources liste les fichiers source d'un job avec leurs chemins complets
func (s *StorageService) ListJobSources(ctx context.Context, jobID uuid.UUID) ([]string, error) {
	prefix := fmt.Sprintf("sources/%s/", jobID.String())
	files, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// Retourner les chemins relatifs (en préservant la structure de dossiers)
	var filePaths []string
	for _, file := range files {
		if after, ok := strings.CutPrefix(file, prefix); ok {
			relativePath := after
			if relativePath != "" {
				filePaths = append(filePaths, relativePath)
			}
		}
	}

	return filePaths, nil
}

// GetJobSourceTree retourne l'arbre des fichiers sources organisé par dossiers
func (s *StorageService) GetJobSourceTree(ctx context.Context, jobID uuid.UUID) (map[string][]string, error) {
	files, err := s.ListJobSources(ctx, jobID)
	if err != nil {
		return nil, err
	}

	tree := make(map[string][]string)

	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == "." {
			dir = "root"
		}

		tree[dir] = append(tree[dir], filepath.Base(file))
	}

	return tree, nil
}

// UploadResult upload le résultat généré pour un cours
func (s *StorageService) UploadResult(ctx context.Context, courseID uuid.UUID, filename string, content io.Reader) error {
	path := fmt.Sprintf("results/%s/%s", courseID.String(), filename)
	return s.storage.Upload(ctx, path, content)
}

// DownloadResult télécharge un résultat généré
func (s *StorageService) DownloadResult(ctx context.Context, courseID uuid.UUID, filename string) (io.Reader, error) {
	path := fmt.Sprintf("results/%s/%s", courseID.String(), filename)
	return s.storage.Download(ctx, path)
}

// GetResultURL retourne l'URL d'accès à un résultat
func (s *StorageService) GetResultURL(ctx context.Context, courseID uuid.UUID, filename string) (string, error) {
	path := fmt.Sprintf("results/%s/%s", courseID.String(), filename)
	return s.storage.GetURL(ctx, path)
}

// ListResults liste les fichiers de résultat d'un cours
func (s *StorageService) ListResults(ctx context.Context, courseID uuid.UUID) ([]string, error) {
	prefix := fmt.Sprintf("results/%s/", courseID.String())
	files, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var filenames []string
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			filename := strings.TrimPrefix(file, prefix)
			if filename != "" {
				filenames = append(filenames, filename)
			}
		}
	}

	return filenames, nil
}

// SaveJobLog sauvegarde les logs d'un job
func (s *StorageService) SaveJobLog(ctx context.Context, jobID uuid.UUID, logContent string) error {
	path := fmt.Sprintf("logs/%s/generation.log", jobID.String())
	return s.storage.Upload(ctx, path, strings.NewReader(logContent))
}

// GetJobLog récupère les logs d'un job
func (s *StorageService) GetJobLog(ctx context.Context, jobID uuid.UUID) (string, error) {
	path := fmt.Sprintf("logs/%s/generation.log", jobID.String())

	reader, err := s.storage.Download(ctx, path)
	if err != nil {
		return "", err
	}

	buf := make([]byte, 1024*1024) // 1MB max pour les logs
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}

// CleanupJob supprime tous les fichiers liés à un job
func (s *StorageService) CleanupJob(ctx context.Context, jobID uuid.UUID) error {
	// Lister et supprimer les sources
	sources, err := s.ListJobSources(ctx, jobID)
	if err == nil {
		for _, filename := range sources {
			path := fmt.Sprintf("sources/%s/%s", jobID.String(), filename)
			s.storage.Delete(ctx, path) // Ignorer les erreurs de suppression
		}
	}

	// Supprimer les logs
	logPath := fmt.Sprintf("logs/%s/generation.log", jobID.String())
	s.storage.Delete(ctx, logPath) // Ignorer les erreurs

	return nil
}

// ValidateFile valide un fichier uploadé
func (s *StorageService) ValidateFile(filePath string) error {
	// Normaliser le chemin
	normalizedPath := filepath.ToSlash(filePath)

	// Vérifier les path traversal
	if strings.Contains(normalizedPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", filePath)
	}

	// Vérifier la profondeur
	segments := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	if len(segments) > 10 {
		return fmt.Errorf("path too deep (max 10 levels): %s", filePath)
	}

	// Valider chaque segment
	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Vérifier l'extension du fichier final
		if strings.Contains(segment, ".") {
			ext := strings.ToLower(filepath.Ext(segment))
			allowedExts := map[string]bool{
				".md": true, ".css": true, ".js": true, ".json": true,
				".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
				".svg": true, ".woff": true, ".woff2": true, ".ttf": true,
				".eot": true, ".ico": true, ".txt": true, ".yml": true,
				".yaml": true, ".html": true, ".vue": true, ".ts": true,
			}

			if ext != "" && !allowedExts[ext] {
				return fmt.Errorf("file extension not allowed: %s", ext)
			}
		}

		// Vérifier les caractères interdits dans les noms de dossiers/fichiers
		if strings.ContainsAny(segment, ":*?\"<>|") {
			return fmt.Errorf("invalid characters in path segment: %s", segment)
		}
	}

	return nil
}
