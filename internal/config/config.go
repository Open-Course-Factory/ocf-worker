// internal/config/config.go - Updated with worker configuration
package config

import (
	"ocf-worker/pkg/storage"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JobTimeout      time.Duration
	CleanupInterval time.Duration
	LogLevel        string
	Environment     string
	Storage         *storage.StorageConfig
	Worker          *WorkerConfig
}

// WorkerConfig contient la configuration spécifique au worker
type WorkerConfig struct {
	WorkerCount      int           // Nombre de workers simultanés
	PollInterval     time.Duration // Intervalle de polling des jobs
	WorkspaceBase    string        // Répertoire de base pour les workspaces
	SlidevCommand    string        // Commande Slidev
	CleanupWorkspace bool          // Nettoyer les workspaces après traitement
	MaxWorkspaceAge  time.Duration // Âge maximum des workspaces avant cleanup
}

func Load() *Config {
	timeout, _ := time.ParseDuration(getEnv("JOB_TIMEOUT", "30m"))
	cleanup, _ := time.ParseDuration(getEnv("CLEANUP_INTERVAL", "1h"))

	return &Config{
		Port:            getEnv("PORT", "8081"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/generation_service?sslmode=disable"),
		JobTimeout:      timeout,
		CleanupInterval: cleanup,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		Storage: &storage.StorageConfig{
			Type:      getEnv("STORAGE_TYPE", "filesystem"),
			BasePath:  getEnv("STORAGE_PATH", "./storage"),
			Endpoint:  getEnv("GARAGE_ENDPOINT", ""),
			AccessKey: getEnv("GARAGE_ACCESS_KEY", ""),
			SecretKey: getEnv("GARAGE_SECRET_KEY", ""),
			Bucket:    getEnv("GARAGE_BUCKET", "ocf-courses"),
			Region:    getEnv("GARAGE_REGION", "us-east-1"),
		},
		Worker: loadWorkerConfig(),
	}
}

// loadWorkerConfig charge la configuration du worker
func loadWorkerConfig() *WorkerConfig {
	pollInterval, _ := time.ParseDuration(getEnv("WORKER_POLL_INTERVAL", "5s"))
	maxWorkspaceAge, _ := time.ParseDuration(getEnv("MAX_WORKSPACE_AGE", "24h"))

	return &WorkerConfig{
		WorkerCount:      getEnvInt("WORKER_COUNT", 3),
		PollInterval:     pollInterval,
		WorkspaceBase:    getEnv("WORKSPACE_BASE", "/tmp/ocf-worker"),
		SlidevCommand:    getEnv("SLIDEV_COMMAND", "npx @slidev/cli"),
		CleanupWorkspace: getEnvBool("CLEANUP_WORKSPACE", true),
		MaxWorkspaceAge:  maxWorkspaceAge,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
