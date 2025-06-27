package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigLoad(t *testing.T) {
	// Test avec les valeurs par défaut
	cfg := Load()

	assert.Equal(t, "8081", cfg.Port)
	assert.Equal(t, "filesystem", cfg.Storage.Type)
	assert.Equal(t, "./storage", cfg.Storage.BasePath)
	assert.Equal(t, 30*time.Minute, cfg.JobTimeout)
	assert.Equal(t, time.Hour, cfg.CleanupInterval)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
}

func TestConfigWithEnvVars(t *testing.T) {
	// Set des variables d'environnement
	os.Setenv("PORT", "9000")
	os.Setenv("STORAGE_TYPE", "garage")
	os.Setenv("STORAGE_PATH", "/custom/path")
	os.Setenv("JOB_TIMEOUT", "45m")
	os.Setenv("GARAGE_ENDPOINT", "https://garage.example.com")
	os.Setenv("GARAGE_BUCKET", "custom-bucket")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("STORAGE_TYPE")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("JOB_TIMEOUT")
		os.Unsetenv("GARAGE_ENDPOINT")
		os.Unsetenv("GARAGE_BUCKET")
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
	os.Setenv("STORAGE_TYPE", "garage")
	os.Setenv("GARAGE_ENDPOINT", "https://s3.garage.com")
	os.Setenv("GARAGE_ACCESS_KEY", "test-access")
	os.Setenv("GARAGE_SECRET_KEY", "test-secret")
	os.Setenv("GARAGE_BUCKET", "test-bucket")
	os.Setenv("GARAGE_REGION", "eu-west-1")
	defer func() {
		os.Unsetenv("STORAGE_TYPE")
		os.Unsetenv("GARAGE_ENDPOINT")
		os.Unsetenv("GARAGE_ACCESS_KEY")
		os.Unsetenv("GARAGE_SECRET_KEY")
		os.Unsetenv("GARAGE_BUCKET")
		os.Unsetenv("GARAGE_REGION")
	}()

	cfg := Load()

	assert.Equal(t, "garage", cfg.Storage.Type)
	assert.Equal(t, "https://s3.garage.com", cfg.Storage.Endpoint)
	assert.Equal(t, "test-access", cfg.Storage.AccessKey)
	assert.Equal(t, "test-secret", cfg.Storage.SecretKey)
	assert.Equal(t, "test-bucket", cfg.Storage.Bucket)
	assert.Equal(t, "eu-west-1", cfg.Storage.Region)
}
