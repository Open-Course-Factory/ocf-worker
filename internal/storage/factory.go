package storage

import (
	"fmt"
	
	"ocf-worker/internal/storage/filesystem"
	"ocf-worker/pkg/storage"
)

// NewStorage crée une nouvelle instance de storage basée sur la configuration
func NewStorage(config *storage.StorageConfig) (storage.Storage, error) {
	switch config.Type {
	case "filesystem":
		return filesystem.NewFilesystemStorage(config.BasePath)
	case "garage":
		// TODO: Implémenter Garage storage
		return nil, fmt.Errorf("garage storage not implemented yet")
	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}
