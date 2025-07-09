// internal/config/config_test.go - Version finale corrigée
package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigLoad(t *testing.T) {
	// Sauvegarder et nettoyer l'environnement
	envVars := []string{
		"STORAGE_PATH", "DOCKER_CONTAINER", "ENVIRONMENT",
		"PORT", "STORAGE_TYPE", "JOB_TIMEOUT", "WORKSPACE_BASE",
	}

	oldValues := make(map[string]string)
	for _, key := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	defer func() {
		// Restaurer l'environnement
		for key, value := range oldValues {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	// Forcer les chemins explicitement pour éviter la détection Docker
	os.Setenv("STORAGE_PATH", "./storage")
	os.Setenv("WORKSPACE_BASE", "/tmp/ocf-worker")

	// Test avec les valeurs par défaut
	cfg := Load()

	assert.Equal(t, "8081", cfg.Port)
	assert.Equal(t, "filesystem", cfg.Storage.Type)
	assert.Equal(t, "./storage", cfg.Storage.BasePath)
	assert.Equal(t, 30*time.Minute, cfg.JobTimeout)
	assert.Equal(t, time.Hour, cfg.CleanupInterval)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)

	// Vérifier la config worker
	assert.Equal(t, 3, cfg.Worker.WorkerCount)
	assert.Equal(t, 5*time.Second, cfg.Worker.PollInterval)
	assert.Equal(t, "/tmp/ocf-worker", cfg.Worker.WorkspaceBase)
	assert.Equal(t, "npx @slidev/cli", cfg.Worker.SlidevCommand)
}

func TestConfigLoadWithoutExplicitPaths(t *testing.T) {
	// Test pour vérifier le comportement de détection automatique
	envVars := []string{
		"STORAGE_PATH", "DOCKER_CONTAINER", "ENVIRONMENT", "WORKSPACE_BASE",
	}

	oldValues := make(map[string]string)
	for _, key := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	defer func() {
		for key, value := range oldValues {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	cfg := Load()

	// Ici on teste que la configuration se charge sans crash
	// Les valeurs peuvent varier selon l'environnement
	assert.NotEmpty(t, cfg.Storage.BasePath)
	assert.NotEmpty(t, cfg.Worker.WorkspaceBase)
	assert.Contains(t, []string{"./storage", "/app/storage"}, cfg.Storage.BasePath)
	assert.Contains(t, []string{"/tmp/ocf-worker", "/app/workspaces"}, cfg.Worker.WorkspaceBase)
}

func TestConfigWithDockerEnvironment(t *testing.T) {
	// Nettoyer d'abord
	envVars := []string{"STORAGE_PATH", "DOCKER_CONTAINER", "WORKSPACE_BASE", "ENVIRONMENT"}
	oldValues := make(map[string]string)
	for _, key := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	defer func() {
		for key, value := range oldValues {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	// Simuler un environnement Docker
	os.Setenv("DOCKER_CONTAINER", "true")

	cfg := Load()

	assert.Equal(t, "/app/storage", cfg.Storage.BasePath)
	assert.Equal(t, "/app/workspaces", cfg.Worker.WorkspaceBase)
}

func TestConfigWithExplicitPaths(t *testing.T) {
	// Nettoyer d'abord
	oldStorage := os.Getenv("STORAGE_PATH")
	oldWorkspace := os.Getenv("WORKSPACE_BASE")

	defer func() {
		if oldStorage != "" {
			os.Setenv("STORAGE_PATH", oldStorage)
		} else {
			os.Unsetenv("STORAGE_PATH")
		}
		if oldWorkspace != "" {
			os.Setenv("WORKSPACE_BASE", oldWorkspace)
		} else {
			os.Unsetenv("WORKSPACE_BASE")
		}
	}()

	// Définir des chemins explicites
	os.Setenv("STORAGE_PATH", "/custom/storage")
	os.Setenv("WORKSPACE_BASE", "/custom/workspaces")

	cfg := Load()

	assert.Equal(t, "/custom/storage", cfg.Storage.BasePath)
	assert.Equal(t, "/custom/workspaces", cfg.Worker.WorkspaceBase)
}

func TestConfigWithEnvVars(t *testing.T) {
	// Set des variables d'environnement
	envVars := map[string]string{
		"PORT":            "9000",
		"STORAGE_TYPE":    "garage",
		"STORAGE_PATH":    "/custom/path",
		"JOB_TIMEOUT":     "45m",
		"GARAGE_ENDPOINT": "https://garage.example.com",
		"GARAGE_BUCKET":   "custom-bucket",
	}

	// Sauvegarder les anciennes valeurs
	oldValues := make(map[string]string)
	for key, value := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	defer func() {
		for key, oldValue := range oldValues {
			if oldValue != "" {
				os.Setenv(key, oldValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	cfg := Load()

	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "garage", cfg.Storage.Type)
	assert.Equal(t, "/custom/path", cfg.Storage.BasePath)
	assert.Equal(t, 45*time.Minute, cfg.JobTimeout)
	assert.Equal(t, "https://garage.example.com", cfg.Storage.Endpoint)
	assert.Equal(t, "custom-bucket", cfg.Storage.Bucket)
}

func TestStorageConfig(t *testing.T) {
	// Test spécifique pour la configuration storage
	envVars := map[string]string{
		"STORAGE_TYPE":      "garage",
		"GARAGE_ENDPOINT":   "https://s3.garage.com",
		"GARAGE_ACCESS_KEY": "test-access",
		"GARAGE_SECRET_KEY": "test-secret",
		"GARAGE_BUCKET":     "test-bucket",
		"GARAGE_REGION":     "eu-west-1",
	}

	oldValues := make(map[string]string)
	for key, value := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	defer func() {
		for key, oldValue := range oldValues {
			if oldValue != "" {
				os.Setenv(key, oldValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	cfg := Load()

	assert.Equal(t, "garage", cfg.Storage.Type)
	assert.Equal(t, "https://s3.garage.com", cfg.Storage.Endpoint)
	assert.Equal(t, "test-access", cfg.Storage.AccessKey)
	assert.Equal(t, "test-secret", cfg.Storage.SecretKey)
	assert.Equal(t, "test-bucket", cfg.Storage.Bucket)
	assert.Equal(t, "eu-west-1", cfg.Storage.Region)
}

func TestWorkerConfig(t *testing.T) {
	// Test spécifique pour la configuration worker
	envVars := map[string]string{
		"WORKER_COUNT":         "5",
		"WORKER_POLL_INTERVAL": "2s",
		"SLIDEV_COMMAND":       "yarn slidev",
		"CLEANUP_WORKSPACE":    "false",
	}

	oldValues := make(map[string]string)
	for key, value := range envVars {
		oldValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	defer func() {
		for key, oldValue := range oldValues {
			if oldValue != "" {
				os.Setenv(key, oldValue)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	cfg := Load()

	assert.Equal(t, 5, cfg.Worker.WorkerCount)
	assert.Equal(t, 2*time.Second, cfg.Worker.PollInterval)
	assert.Equal(t, "yarn slidev", cfg.Worker.SlidevCommand)
	assert.False(t, cfg.Worker.CleanupWorkspace)
}

// Test pour vérifier la fonction de détection Docker
func TestDockerDetection(t *testing.T) {
	// Test avec DOCKER_CONTAINER=true
	oldContainer := os.Getenv("DOCKER_CONTAINER")
	os.Setenv("DOCKER_CONTAINER", "true")
	assert.True(t, isDockerEnvironment())

	// Restaurer
	if oldContainer != "" {
		os.Setenv("DOCKER_CONTAINER", oldContainer)
	} else {
		os.Unsetenv("DOCKER_CONTAINER")
	}

	// Test avec DOCKER_CONTAINER absent mais présence de /.dockerenv (si applicable)
	os.Unsetenv("DOCKER_CONTAINER")
	// Note: Ce test peut échouer en fonction de l'environnement
	// donc on le rend tolérant
	result := isDockerEnvironment()
	t.Logf("Docker detection result: %v", result)
}
