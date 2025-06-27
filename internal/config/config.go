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
