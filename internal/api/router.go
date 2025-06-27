package api

import (
	"ocf-worker/internal/jobs"
	"ocf-worker/internal/storage"
	
	"github.com/gin-gonic/gin"
)

func SetupRouter(jobService jobs.JobService, storageService *storage.StorageService) *gin.Engine {
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

	// Handlers
	jobHandlers := NewHandlers(jobService)
	storageHandlers := NewStorageHandlers(storageService)

	// Routes principales
	r.GET("/health", jobHandlers.Health)
	
	api := r.Group("/api/v1")
	{
		// Routes des jobs
		api.POST("/generate", jobHandlers.CreateJob)
		api.GET("/jobs/:id", jobHandlers.GetJobStatus)
		api.GET("/jobs", jobHandlers.ListJobs)
		
		// Routes du storage
		storage := api.Group("/storage")
		{
			// Info storage
			storage.GET("/info", storageHandlers.GetStorageInfo)
			
			// Sources des jobs
			storage.POST("/jobs/:job_id/sources", storageHandlers.UploadJobSources)
			storage.GET("/jobs/:job_id/sources", storageHandlers.ListJobSources)
			storage.GET("/jobs/:job_id/sources/:filename", storageHandlers.DownloadJobSource)
			
			// Résultats des cours
			storage.GET("/courses/:course_id/results", storageHandlers.ListResults)
			storage.GET("/courses/:course_id/results/:filename", storageHandlers.DownloadResult)
			
			// Logs des jobs
			storage.GET("/jobs/:job_id/logs", storageHandlers.GetJobLogs)
		}
	}

	return r
}
