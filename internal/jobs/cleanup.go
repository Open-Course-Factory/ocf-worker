package jobs

import (
	"context"
	"log"
	"time"
)

type CleanupService struct {
	jobService JobService
	interval   time.Duration
	maxAge     time.Duration
	stopCh     chan struct{}
}

func NewCleanupService(jobService JobService, interval, maxAge time.Duration) *CleanupService {
	return &CleanupService{
		jobService: jobService,
		interval:   interval,
		maxAge:     maxAge,
		stopCh:     make(chan struct{}),
	}
}

func (c *CleanupService) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	log.Printf("Cleanup service started (interval: %v, max age: %v)", c.interval, c.maxAge)

	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup service stopped due to context cancellation")
			return
		case <-c.stopCh:
			log.Println("Cleanup service stopped")
			return
		case <-ticker.C:
			if deleted, err := c.jobService.CleanupOldJobs(ctx, c.maxAge); err != nil {
				log.Printf("Cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleanup completed: %d jobs removed", deleted)
			}
		}
	}
}

func (c *CleanupService) Stop() {
	close(c.stopCh)
}
