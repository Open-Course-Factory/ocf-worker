// cmd/generator/main.go - Updated version with worker integration
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"ocf-worker/internal/api"
	"ocf-worker/internal/config"
	"ocf-worker/internal/database"
	"ocf-worker/internal/jobs"
	"ocf-worker/internal/storage"
	"ocf-worker/internal/worker"

	"github.com/lpernett/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Info: .env file not found, using environment variables: %v", err)
	}

	// Load configuration
	cfg := config.Load()

	// Initialize storage
	storageBackend, err := storage.NewStorage(cfg.Storage)
	if err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}
	storageService := storage.NewStorageService(storageBackend)

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize services
	jobRepo := jobs.NewJobRepository(db.DB)
	jobService := jobs.NewJobServiceImpl(jobRepo)

	// Initialize worker pool
	workerConfig := &worker.PoolConfig{
		WorkerCount:      getWorkerCount(cfg),
		PollInterval:     5 * time.Second,
		JobTimeout:       cfg.JobTimeout,
		WorkspaceBase:    getWorkspaceBase(cfg),
		SlidevCommand:    getSlidevCommand(cfg),
		CleanupWorkspace: true,
	}

	workerPool := worker.NewWorkerPool(jobService, storageService, workerConfig)

	// Start cleanup service
	cleanupService := jobs.NewCleanupService(jobService, cfg.CleanupInterval, 24*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cleanupService.Start(ctx)

	// Start worker pool
	log.Printf("Starting worker pool...")
	if err := workerPool.Start(ctx); err != nil {
		log.Fatal("Failed to start worker pool:", err)
	}

	// Setup router with enhanced worker stats
	router := api.SetupRouterWithWorker(jobService, storageService, workerPool)

	// Start server in goroutine
	log.Printf("Starting ocf-worker on port %s", cfg.Port)
	log.Printf("Database: connected")
	log.Printf("Storage type: %s", cfg.Storage.Type)
	log.Printf("Worker pool: %d workers", workerConfig.WorkerCount)
	log.Printf("Workspace base: %s", workerConfig.WorkspaceBase)
	log.Printf("Job timeout: %v", workerConfig.JobTimeout)

	switch cfg.Storage.Type {
	case "filesystem":
		log.Printf("Storage path: %s", cfg.Storage.BasePath)
	case "garage":
		log.Printf("Garage endpoint: %s", cfg.Storage.Endpoint)
		log.Printf("Garage bucket: %s", cfg.Storage.Bucket)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- router.Run(":" + cfg.Port)
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatal("Server failed to start:", err)
	case sig := <-quit:
		log.Printf("Received signal %v, shutting down...", sig)

		// Graceful shutdown
		log.Println("Stopping worker pool...")
		if err := workerPool.Stop(); err != nil {
			log.Printf("Error stopping worker pool: %v", err)
		}

		log.Println("Stopping cleanup service...")
		cancel() // Stop cleanup service

		log.Println("Shutdown complete")
	}
}

// getWorkerCount retourne le nombre de workers à partir de la configuration
func getWorkerCount(cfg *config.Config) int {
	if count := os.Getenv("WORKER_COUNT"); count != "" {
		if parsed, err := parseIntEnv(count); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 3 // Par défaut
}

// getWorkspaceBase retourne le répertoire de base pour les workspaces
func getWorkspaceBase(cfg *config.Config) string {
	if base := os.Getenv("WORKSPACE_BASE"); base != "" {
		return base
	}
	// Utiliser /app/workspaces dans le container pour éviter les problèmes de permissions
	if cfg.Environment == "development" || os.Getenv("DOCKER_CONTAINER") != "" {
		return "/app/workspaces"
	}
	return "/tmp/ocf-worker" // Par défaut pour environnement local
}

// getSlidevCommand retourne la commande Slidev à utiliser
func getSlidevCommand(cfg *config.Config) string {
	if cmd := os.Getenv("SLIDEV_COMMAND"); cmd != "" {
		return cmd
	}
	return "npx @slidev/cli" // Par défaut
}

// parseIntEnv parse une variable d'environnement en entier
func parseIntEnv(value string) (int, error) {
	return strconv.Atoi(value)
}
