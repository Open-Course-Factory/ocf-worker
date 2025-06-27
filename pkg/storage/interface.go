package storage

import (
	"context"
	"io"
)

// Storage définit l'interface pour le stockage de fichiers
type Storage interface {
	// Upload un fichier vers le storage
	Upload(ctx context.Context, path string, data io.Reader) error
	
	// Download un fichier depuis le storage
	Download(ctx context.Context, path string) (io.Reader, error)
	
	// Exists vérifie si un fichier existe
	Exists(ctx context.Context, path string) (bool, error)
	
	// Delete supprime un fichier
	Delete(ctx context.Context, path string) error
	
	// List liste les fichiers avec un préfixe donné
	List(ctx context.Context, prefix string) ([]string, error)
	
	// GetURL retourne l'URL d'accès à un fichier (pour les résultats)
	GetURL(ctx context.Context, path string) (string, error)
}

// StorageConfig contient la configuration du storage
type StorageConfig struct {
	Type        string // "filesystem" ou "garage"
	BasePath    string // Pour filesystem
	Endpoint    string // Pour S3/Garage
	AccessKey   string
	SecretKey   string
	Bucket      string
	Region      string
}
