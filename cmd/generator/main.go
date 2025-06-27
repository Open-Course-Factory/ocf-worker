package main

import (
	"log"
	"ocf-worker/internal/api"
	"ocf-worker/internal/config"
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

	// Initialize services
	jobService := jobs.NewJobService()

	// Setup router
	router := api.SetupRouter(jobService)

	// Start server
	log.Printf("Starting generation service on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
