package storage

import (
	"fmt"

	"github.com/Open-Course-Factory/ocf-worker/internal/storage/filesystem"
	"github.com/Open-Course-Factory/ocf-worker/internal/storage/garage"
	"github.com/Open-Course-Factory/ocf-worker/pkg/storage"
)

// NewStorage crée une nouvelle instance de storage basée sur la configuration
func NewStorage(config *storage.StorageConfig) (storage.Storage, error) {
	switch config.Type {
	case "filesystem":
		return filesystem.NewFilesystemStorage(config.BasePath)
	case "garage":
		return garage.NewGarageStorage(config)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}
