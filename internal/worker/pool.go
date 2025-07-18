// internal/worker/pool.go
package worker

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/internal/jobs"
	"github.com/Open-Course-Factory/ocf-worker/internal/storage"
	"github.com/Open-Course-Factory/ocf-worker/pkg/models"
)

// WorkerPool gère un pool de workers pour traiter les jobs de génération
type WorkerPool struct {
	jobService     jobs.JobService
	storageService *storage.StorageService
	config         *PoolConfig
	workers        []*Worker
	jobQueue       chan *models.GenerationJob
	stopCh         chan struct{}
	wg             sync.WaitGroup
	running        bool
	mu             sync.RWMutex
}

// PoolConfig contient la configuration du pool de workers
type PoolConfig struct {
	WorkerCount      int           // Nombre de workers simultanés
	PollInterval     time.Duration // Intervalle de polling des jobs
	JobTimeout       time.Duration // Timeout par job
	WorkspaceBase    string        // Répertoire de base pour les workspaces
	SlidevCommand    string        // Commande Slidev (par défaut "npx @slidev/cli")
	CleanupWorkspace bool          // Nettoyer les workspaces après traitement
}

// DefaultPoolConfig retourne une configuration par défaut avec chemin sécurisé
func DefaultPoolConfig() *PoolConfig {
	// Détecter l'environnement pour choisir le bon workspace base
	workspaceBase := "/tmp/ocf-worker"
	if os.Getenv("DOCKER_CONTAINER") != "" || os.Getenv("ENVIRONMENT") == "development" {
		workspaceBase = "/app/workspaces"
	}

	return &PoolConfig{
		WorkerCount:      3,
		PollInterval:     5 * time.Second,
		JobTimeout:       30 * time.Minute,
		WorkspaceBase:    workspaceBase,
		SlidevCommand:    "npx @slidev/cli",
		CleanupWorkspace: true,
	}
}

// NewWorkerPool crée un nouveau pool de workers
func NewWorkerPool(
	jobService jobs.JobService,
	storageService *storage.StorageService,
	config *PoolConfig,
) *WorkerPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &WorkerPool{
		jobService:     jobService,
		storageService: storageService,
		config:         config,
		jobQueue:       make(chan *models.GenerationJob, config.WorkerCount*2),
		stopCh:         make(chan struct{}),
	}

	// Créer les workers
	for i := 0; i < config.WorkerCount; i++ {
		worker := NewWorker(i, jobService, storageService, config)
		pool.workers = append(pool.workers, worker)
	}

	return pool
}

// Start démarre le pool de workers
func (p *WorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	log.Printf("Starting worker pool with %d workers", p.config.WorkerCount)

	// Démarrer les workers
	for i, worker := range p.workers {
		p.wg.Add(1)
		go func(workerID int, w *Worker) {
			defer p.wg.Done()
			w.Start(ctx, p.jobQueue)
		}(i, worker)
		log.Printf("Worker %d started", i)
	}

	// Démarrer le job poller
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.runJobPoller(ctx)
	}()

	p.running = true
	log.Printf("Worker pool started successfully")

	return nil
}

// Stop arrête le pool de workers
func (p *WorkerPool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	log.Println("Stopping worker pool...")

	// Signaler l'arrêt
	close(p.stopCh)

	// Fermer la queue des jobs
	close(p.jobQueue)

	// Attendre que tous les workers se terminent
	p.wg.Wait()

	p.running = false
	log.Println("Worker pool stopped")

	return nil
}

// runJobPoller poll régulièrement les jobs pending
func (p *WorkerPool) runJobPoller(ctx context.Context) {
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	log.Printf("Job poller started (interval: %v)", p.config.PollInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Job poller stopped due to context cancellation")
			return
		case <-p.stopCh:
			log.Println("Job poller stopped")
			return
		case <-ticker.C:
			if err := p.pollPendingJobs(ctx); err != nil {
				log.Printf("Error polling jobs: %v", err)
			}
		}
	}
}

// pollPendingJobs récupère les jobs pending et les envoie aux workers
func (p *WorkerPool) pollPendingJobs(ctx context.Context) error {
	// Récupérer les jobs pending
	pendingJobs, err := p.jobService.ListJobs(ctx, string(models.StatusPending), nil)
	if err != nil {
		return err
	}

	if len(pendingJobs) == 0 {
		return nil // Pas de jobs pending
	}

	log.Printf("Found %d pending jobs", len(pendingJobs))

	// Envoyer les jobs aux workers (non-bloquant)
	for _, job := range pendingJobs {
		select {
		case p.jobQueue <- job:
			log.Printf("Job %s queued for processing", job.ID)
		default:
			// Queue pleine, on reessaiera au prochain poll
			log.Printf("Job queue full, job %s will be retried", job.ID)
		}
	}

	return nil
}

// GetStats retourne les statistiques du pool
func (p *WorkerPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PoolStats{
		WorkerCount:   len(p.workers),
		QueueSize:     len(p.jobQueue),
		QueueCapacity: cap(p.jobQueue),
		Running:       p.running,
	}

	// Ajouter les stats des workers individuels
	for i, worker := range p.workers {
		workerStats := worker.GetStats()
		stats.Workers = append(stats.Workers, WorkerStats{
			ID:           i,
			Status:       workerStats.Status,
			CurrentJobID: workerStats.CurrentJobID,
			JobsTotal:    workerStats.JobsTotal,
			JobsSuccess:  workerStats.JobsSuccess,
			JobsFailed:   workerStats.JobsFailed,
		})
	}

	return stats
}

func (p *WorkerPool) GetConfig() *PoolConfig {
	return p.config
}

// PoolStats contient les statistiques du pool
type PoolStats struct {
	WorkerCount   int           `json:"worker_count"`
	QueueSize     int           `json:"queue_size"`
	QueueCapacity int           `json:"queue_capacity"`
	Running       bool          `json:"running"`
	Workers       []WorkerStats `json:"workers"`
}

// WorkerStats contient les statistiques d'un worker
type WorkerStats struct {
	ID           int    `json:"id"`
	Status       string `json:"status"` // idle, busy, stopped
	CurrentJobID string `json:"current_job_id,omitempty"`
	JobsTotal    int64  `json:"jobs_total"`
	JobsSuccess  int64  `json:"jobs_success"`
	JobsFailed   int64  `json:"jobs_failed"`
}
