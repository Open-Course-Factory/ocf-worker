package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	
	"ocf-worker/pkg/storage"
)

type filesystemStorage struct {
	basePath string
}

// NewFilesystemStorage crée une nouvelle instance de storage filesystem
func NewFilesystemStorage(basePath string) (storage.Storage, error) {
	// Créer le répertoire de base s'il n'existe pas
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", basePath, err)
	}
	
	return &filesystemStorage{
		basePath: basePath,
	}, nil
}

func (fs *filesystemStorage) Upload(ctx context.Context, path string, data io.Reader) error {
	fullPath := filepath.Join(fs.basePath, path)
	
	// Créer les répertoires parents si nécessaire
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories for %s: %w", fullPath, err)
	}
	
	// Créer le fichier
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", fullPath, err)
	}
	defer file.Close()
	
	// Copier les données
	if _, err := io.Copy(file, data); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", fullPath, err)
	}
	
	return nil
}

func (fs *filesystemStorage) Download(ctx context.Context, path string) (io.Reader, error) {
	fullPath := filepath.Join(fs.basePath, path)
	
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file %s: %w", fullPath, err)
	}
	
	return file, nil
}

func (fs *filesystemStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(fs.basePath, path)
	
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence %s: %w", fullPath, err)
	}
	
	return true, nil
}

func (fs *filesystemStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(fs.basePath, path)
	
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, no error
		}
		return fmt.Errorf("failed to delete file %s: %w", fullPath, err)
	}
	
	return nil
}

func (fs *filesystemStorage) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := filepath.Join(fs.basePath, prefix)
	
	var files []string
	
	err := filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasPrefix(path, fullPrefix) {
			// Retourner le chemin relatif au basePath
			relPath, err := filepath.Rel(fs.basePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list files with prefix %s: %w", prefix, err)
	}
	
	return files, nil
}

func (fs *filesystemStorage) GetURL(ctx context.Context, path string) (string, error) {
	// Pour filesystem, on retourne juste le chemin relatif
	// Dans un vrai environnement, ceci pourrait être une URL HTTP
	return path, nil
}
