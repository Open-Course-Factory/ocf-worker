package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	
	"github.com/google/uuid"
	"ocf-worker/pkg/storage"
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
		
		// Construire le chemin: sources/{job_id}/{filename}
		path := fmt.Sprintf("sources/%s/%s", jobID.String(), fileHeader.Filename)
		
		if err := s.storage.Upload(ctx, path, file); err != nil {
			return fmt.Errorf("failed to upload file %s: %w", fileHeader.Filename, err)
		}
	}
	
	return nil
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

// ListJobSources liste les fichiers source d'un job
func (s *StorageService) ListJobSources(ctx context.Context, jobID uuid.UUID) ([]string, error) {
	prefix := fmt.Sprintf("sources/%s/", jobID.String())
	files, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	
	// Retourner seulement les noms de fichiers (sans le préfixe)
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
func (s *StorageService) ValidateFile(filename string) error {
	// Vérifier l'extension
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".md":   true,
		".css":  true,
		".js":   true,
		".json": true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".svg":  true,
		".woff": true,
		".woff2": true,
		".ttf":  true,
		".eot":  true,
	}
	
	if !allowedExts[ext] {
		return fmt.Errorf("file type not allowed: %s", ext)
	}
	
	// Vérifier le nom de fichier
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	
	return nil
}
