package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	StorageType     string // "filesystem" or "garage"
	StoragePath     string
	GarageEndpoint  string
	GarageAccessKey string
	GarageSecretKey string
	GarageBucket    string
	JobTimeout      time.Duration
	CleanupInterval time.Duration
	LogLevel        string
	Environment     string
}

func Load() *Config {
	timeout, _ := time.ParseDuration(getEnv("JOB_TIMEOUT", "30m"))
	cleanup, _ := time.ParseDuration(getEnv("CLEANUP_INTERVAL", "1h"))

	return &Config{
		Port:            getEnv("PORT", "8081"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/generation_service?sslmode=disable"),
		StorageType:     getEnv("STORAGE_TYPE", "filesystem"),
		StoragePath:     getEnv("STORAGE_PATH", "./storage"),
		GarageEndpoint:  getEnv("GARAGE_ENDPOINT", ""),
		GarageAccessKey: getEnv("GARAGE_ACCESS_KEY", ""),
		GarageSecretKey: getEnv("GARAGE_SECRET_KEY", ""),
		GarageBucket:    getEnv("GARAGE_BUCKET", "ocf-courses"),
		JobTimeout:      timeout,
		CleanupInterval: cleanup,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Environment:     getEnv("ENVIRONMENT", "development"),
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
