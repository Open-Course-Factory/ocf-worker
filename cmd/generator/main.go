package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ocf-worker/internal/api"
	"ocf-worker/internal/config"
	"ocf-worker/internal/database"
	"ocf-worker/internal/jobs"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Load configuration
	cfg := config.Load()

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

	// Start cleanup service
	cleanupService := jobs.NewCleanupService(jobService, cfg.CleanupInterval, 24*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cleanupService.Start(ctx)

	// Setup router
	router := api.SetupRouter(jobService)

	// Start server in goroutine
	log.Printf("Starting ocf-worker on port %s", cfg.Port)
	log.Printf("Database: connected")
	log.Printf("Storage type: %s", cfg.Storage.Type)

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
		cancel() // Stop cleanup service
		log.Println("Cleanup service stopped")
		log.Println("Server shutdown complete")
	}
}
