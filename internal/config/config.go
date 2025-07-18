// internal/config/config.go - Version corrigée
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/pkg/storage"
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

type WorkerConfig struct {
	WorkerCount      int
	PollInterval     time.Duration
	WorkspaceBase    string
	SlidevCommand    string
	CleanupWorkspace bool
	MaxWorkspaceAge  time.Duration
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
			BasePath:  getStorageBasePath(),
			Endpoint:  getEnv("GARAGE_ENDPOINT", ""),
			AccessKey: getEnv("GARAGE_ACCESS_KEY", ""),
			SecretKey: getEnv("GARAGE_SECRET_KEY", ""),
			Bucket:    getEnv("GARAGE_BUCKET", "ocf-courses"),
			Region:    getEnv("GARAGE_REGION", "us-east-1"),
		},
		Worker: loadWorkerConfig(),
	}
}

// getStorageBasePath détermine le chemin de base pour le storage
func getStorageBasePath() string {
	// Si explicitement défini, l'utiliser
	if path := os.Getenv("STORAGE_PATH"); path != "" {
		return path
	}

	// Sinon, détecter l'environnement
	if isDockerEnvironment() {
		return "/app/storage"
	}

	return "./storage"
}

// isDockerEnvironment détecte si on est dans un container Docker
func isDockerEnvironment() bool {
	// Vérifier les indicateurs d'environnement Docker
	if os.Getenv("DOCKER_CONTAINER") != "" {
		return true
	}

	if os.Getenv("ENVIRONMENT") == "development" && dockerFileExists() {
		return true
	}

	// Vérifier si on est dans un container (présence de /.dockerenv)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	return false
}

// dockerFileExists vérifie si des fichiers Docker sont présents
func dockerFileExists() bool {
	dockerFiles := []string{"docker-compose.yml", "Dockerfile", "deployments/docker/Dockerfile"}
	for _, file := range dockerFiles {
		if _, err := os.Stat(file); err == nil {
			return true
		}
	}
	return false
}

func loadWorkerConfig() *WorkerConfig {
	pollInterval, _ := time.ParseDuration(getEnv("WORKER_POLL_INTERVAL", "5s"))
	maxWorkspaceAge, _ := time.ParseDuration(getEnv("MAX_WORKSPACE_AGE", "24h"))

	return &WorkerConfig{
		WorkerCount:      getEnvInt("WORKER_COUNT", 3),
		PollInterval:     pollInterval,
		WorkspaceBase:    getWorkspaceBasePath(),
		SlidevCommand:    getEnv("SLIDEV_COMMAND", "npx @slidev/cli"),
		CleanupWorkspace: getEnvBool("CLEANUP_WORKSPACE", true),
		MaxWorkspaceAge:  maxWorkspaceAge,
	}
}

// getWorkspaceBasePath détermine le répertoire de base pour les workspaces
func getWorkspaceBasePath() string {
	// Si explicitement défini, l'utiliser
	if path := os.Getenv("WORKSPACE_BASE"); path != "" {
		return path
	}

	// Sinon, détecter l'environnement
	if isDockerEnvironment() {
		return "/app/workspaces"
	}

	return "/tmp/ocf-worker"
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
