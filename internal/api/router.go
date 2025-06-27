package api

import (
	"ocf-worker/internal/jobs"
	
	"github.com/gin-gonic/gin"
)

func SetupRouter(jobService jobs.JobService) *gin.Engine {
	r := gin.Default()
	
	// Middleware pour CORS et logs
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	handlers := NewHandlers(jobService)

	// Routes
	r.GET("/health", handlers.Health)
	
	api := r.Group("/api/v1")
	{
		api.POST("/generate", handlers.CreateJob)
		api.GET("/jobs/:id", handlers.GetJobStatus)
		api.GET("/jobs", handlers.ListJobs)
	}

	return r
}
