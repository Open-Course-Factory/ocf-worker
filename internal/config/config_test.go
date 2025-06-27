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
	assert.Equal(t, "filesystem", cfg.StorageType)
	assert.Equal(t, "./storage", cfg.StoragePath)
	assert.Equal(t, 30*time.Minute, cfg.JobTimeout)
	assert.Equal(t, time.Hour, cfg.CleanupInterval)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
}

func TestConfigWithEnvVars(t *testing.T) {
	// Set des variables d'environnement
	os.Setenv("PORT", "9000")
	os.Setenv("STORAGE_TYPE", "garage")
	os.Setenv("JOB_TIMEOUT", "45m")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("STORAGE_TYPE")
		os.Unsetenv("JOB_TIMEOUT")
	}()

	cfg := Load()
	
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "garage", cfg.StorageType)
	assert.Equal(t, 45*time.Minute, cfg.JobTimeout)
}
