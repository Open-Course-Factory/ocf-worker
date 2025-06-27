// internal/worker/worker.go
package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ocf-worker/internal/jobs"
	"ocf-worker/internal/storage"
	"ocf-worker/pkg/models"

	"github.com/google/uuid"
)

// Worker représente un worker individuel qui traite les jobs
type Worker struct {
	id             int
	jobService     jobs.JobService
	storageService *storage.StorageService
	config         *PoolConfig
	processor      *JobProcessor

	// Statistiques
	status       string
	currentJobID uuid.UUID
	jobsTotal    int64
	jobsSuccess  int64
	jobsFailed   int64
	mu           sync.RWMutex
}

// NewWorker crée un nouveau worker
func NewWorker(
	id int,
	jobService jobs.JobService,
	storageService *storage.StorageService,
	config *PoolConfig,
) *Worker {
	return &Worker{
		id:             id,
		jobService:     jobService,
		storageService: storageService,
		config:         config,
		processor:      NewJobProcessor(jobService, storageService, config),
		status:         "idle",
	}
}

// Start démarre le worker et écoute la queue des jobs
func (w *Worker) Start(ctx context.Context, jobQueue <-chan *models.GenerationJob) {
	log.Printf("Worker %d starting", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopped due to context cancellation", w.id)
			w.setStatus("stopped")
			return
		case job, ok := <-jobQueue:
			if !ok {
				log.Printf("Worker %d stopped - job queue closed", w.id)
				w.setStatus("stopped")
				return
			}

			w.processJob(ctx, job)
		}
	}
}

// processJob traite un job individuel
func (w *Worker) processJob(ctx context.Context, job *models.GenerationJob) {
	w.setCurrentJob(job.ID)
	w.setStatus("busy")
	atomic.AddInt64(&w.jobsTotal, 1)

	log.Printf("Worker %d processing job %s (course: %s)", w.id, job.ID, job.CourseID)

	// Créer un contexte avec timeout pour le job
	jobCtx, cancel := context.WithTimeout(ctx, w.config.JobTimeout)
	defer cancel()

	// Traiter le job
	result := w.processor.ProcessJob(jobCtx, job)

	// Mettre à jour les statistiques
	if result.Success {
		atomic.AddInt64(&w.jobsSuccess, 1)
		log.Printf("Worker %d completed job %s successfully", w.id, job.ID)
	} else {
		atomic.AddInt64(&w.jobsFailed, 1)
		log.Printf("Worker %d failed job %s: %v", w.id, job.ID, result.Error)
	}

	// Nettoyer l'état du worker
	w.setCurrentJob(uuid.Nil)
	w.setStatus("idle")
}

// setStatus met à jour le statut du worker
func (w *Worker) setStatus(status string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

// setCurrentJob met à jour le job actuel
func (w *Worker) setCurrentJob(jobID uuid.UUID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentJobID = jobID
}

// GetStats retourne les statistiques du worker
func (w *Worker) GetStats() WorkerStatsInternal {
	w.mu.RLock()
	defer w.mu.RUnlock()

	currentJobIDStr := ""
	if w.currentJobID != uuid.Nil {
		currentJobIDStr = w.currentJobID.String()
	}

	return WorkerStatsInternal{
		Status:       w.status,
		CurrentJobID: currentJobIDStr,
		JobsTotal:    atomic.LoadInt64(&w.jobsTotal),
		JobsSuccess:  atomic.LoadInt64(&w.jobsSuccess),
		JobsFailed:   atomic.LoadInt64(&w.jobsFailed),
	}
}

// WorkerStatsInternal structure interne pour les stats du worker
type WorkerStatsInternal struct {
	Status       string
	CurrentJobID string
	JobsTotal    int64
	JobsSuccess  int64
	JobsFailed   int64
}

// JobResult contient le résultat du traitement d'un job
type JobResult struct {
	Success   bool
	Error     error
	Duration  time.Duration
	Progress  int
	LogOutput []string
}

// JobProcessor traite les jobs de génération
type JobProcessor struct {
	jobService     jobs.JobService
	storageService *storage.StorageService
	config         *PoolConfig
	slidevRunner   *SlidevRunner
}

// NewJobProcessor crée un nouveau processeur de jobs
func NewJobProcessor(
	jobService jobs.JobService,
	storageService *storage.StorageService,
	config *PoolConfig,
) *JobProcessor {
	return &JobProcessor{
		jobService:     jobService,
		storageService: storageService,
		config:         config,
		slidevRunner:   NewSlidevRunner(config),
	}
}

// ProcessJob traite un job de génération complet avec debug amélioré
func (p *JobProcessor) ProcessJob(ctx context.Context, job *models.GenerationJob) *JobResult {
	startTime := time.Now()
	result := &JobResult{
		Success:  false,
		Progress: 0,
	}

	// Créer un workspace isolé pour ce job
	workspace, err := NewWorkspace(p.config.WorkspaceBase, job.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to create workspace: %w", err)
		p.updateJobStatus(ctx, job.ID, models.StatusFailed, 0, result.Error.Error())
		return result
	}

	// Nettoyage du workspace à la fin
	if p.config.CleanupWorkspace {
		defer func() {
			if cleanupErr := workspace.Cleanup(); cleanupErr != nil {
				log.Printf("Failed to cleanup workspace for job %s: %v", job.ID, cleanupErr)
			}
		}()
	}

	// Marquer le job comme en cours de traitement
	if err := p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 10, ""); err != nil {
		result.Error = fmt.Errorf("failed to update job status: %w", err)
		return result
	}

	// Étape 1: Télécharger les sources
	log.Printf("Job %s: Downloading sources", job.ID)
	if err := p.downloadSources(ctx, job, workspace); err != nil {
		result.Error = fmt.Errorf("failed to download sources: %w", err)
		p.updateJobStatus(ctx, job.ID, models.StatusFailed, 20, result.Error.Error())
		return result
	}

	p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 30, "Sources downloaded")
	result.Progress = 30

	// Étape 2: Préparer l'environnement Slidev
	log.Printf("Job %s: Preparing Slidev environment", job.ID)
	if err := p.prepareSlidevEnvironment(ctx, job, workspace); err != nil {
		log.Printf("Job %s: Slidev preparation failed (non-fatal): %v", job.ID, err)
	}

	p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 40, "Environment prepared")

	// Étape 3: Exécuter Slidev build
	log.Printf("Job %s: Running Slidev build", job.ID)
	slidevResult, err := p.slidevRunner.Build(ctx, workspace, job)
	if err != nil {
		result.Error = fmt.Errorf("slidev build failed: %w", err)
		p.updateJobStatus(ctx, job.ID, models.StatusFailed, 50, result.Error.Error())

		// Sauvegarder les logs même en cas d'échec
		if len(slidevResult.Logs) > 0 {
			p.saveJobLogs(ctx, job.ID, slidevResult.Logs)
		}

		// Debug: lister le contenu du workspace
		p.debugWorkspaceContents(workspace, job.ID)
		return result
	}

	p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 70, "Slidev build completed")
	result.Progress = 70
	result.LogOutput = slidevResult.Logs

	// Étape 4: Upload des résultats
	log.Printf("Job %s: Uploading results", job.ID)
	if err := p.uploadResults(ctx, job, workspace); err != nil {
		result.Error = fmt.Errorf("failed to upload results: %w", err)
		p.updateJobStatus(ctx, job.ID, models.StatusFailed, 80, result.Error.Error())

		// Debug: lister le contenu du workspace
		p.debugWorkspaceContents(workspace, job.ID)
		return result
	}

	p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 90, "Results uploaded")

	// Étape 5: Sauvegarder les logs
	if len(result.LogOutput) > 0 {
		if err := p.saveJobLogs(ctx, job.ID, result.LogOutput); err != nil {
			log.Printf("Failed to save logs for job %s: %v", job.ID, err)
		}
	}

	// Marquer le job comme terminé
	if err := p.updateJobStatus(ctx, job.ID, models.StatusCompleted, 100, ""); err != nil {
		log.Printf("Failed to update final job status for %s: %v", job.ID, err)
	}

	result.Success = true
	result.Progress = 100
	result.Duration = time.Since(startTime)

	log.Printf("Job %s completed successfully in %v", job.ID, result.Duration)
	return result
}

// prepareSlidevEnvironment prépare l'environnement Slidev dans le workspace
func (p *JobProcessor) prepareSlidevEnvironment(ctx context.Context, job *models.GenerationJob, workspace *Workspace) error {
	// S'il n'y a pas de package.json, en créer un basique
	if !workspace.FileExists("package.json") {
		log.Printf("Job %s: Creating basic package.json", job.ID)
		packageJSON := `{
  "name": "ocf-generated-course",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "slidev",
    "build": "slidev build",
    "export": "slidev export"
  }
}`
		if err := workspace.WriteFile("package.json", strings.NewReader(packageJSON)); err != nil {
			return fmt.Errorf("failed to create package.json: %w", err)
		}
	}

	// Vérifier qu'il y a un fichier de slides principal
	slideFiles := []string{"slides.md", "index.md", "README.md"}
	hasSlideFile := false
	for _, file := range slideFiles {
		if workspace.FileExists(file) {
			hasSlideFile = true
			break
		}
	}

	if !hasSlideFile {
		log.Printf("Job %s: No slide file found, creating basic slides.md", job.ID)
		basicSlides := `---
theme: default
title: OCF Generated Course
---

# OCF Generated Course

Welcome to your generated course!

---

# Content

This course was generated by OCF Worker.
`
		if err := workspace.WriteFile("slides.md", strings.NewReader(basicSlides)); err != nil {
			return fmt.Errorf("failed to create basic slides.md: %w", err)
		}
	}

	return nil
}

// debugWorkspaceContents affiche le contenu du workspace pour debug
func (p *JobProcessor) debugWorkspaceContents(workspace *Workspace, jobID uuid.UUID) {
	log.Printf("Job %s: DEBUG - Workspace contents:", jobID)

	if files, err := workspace.ListAllFiles("."); err == nil {
		for _, file := range files {
			if size, err := workspace.GetFileSize(file); err == nil {
				log.Printf("  %s (%d bytes)", file, size)
			} else {
				log.Printf("  %s (size unknown)", file)
			}
		}
	} else {
		log.Printf("  Failed to list workspace files: %v", err)
	}

	// Vérifier spécifiquement les répertoires de sortie possibles
	outputDirs := []string{"dist", "build", "output", "_output", ".slidev"}
	for _, dir := range outputDirs {
		if workspace.DirExists(dir) {
			log.Printf("  Found output directory: %s", dir)
			if files, err := workspace.ListAllFiles(dir); err == nil {
				for _, file := range files {
					log.Printf("    %s/%s", dir, file)
				}
			}
		}
	}
}

// downloadSources télécharge les fichiers sources dans le workspace
func (p *JobProcessor) downloadSources(ctx context.Context, job *models.GenerationJob, workspace *Workspace) error {
	// Lister les fichiers sources
	sourceFiles, err := p.storageService.ListJobSources(ctx, job.ID)
	if err != nil {
		return fmt.Errorf("failed to list source files: %w", err)
	}

	if len(sourceFiles) == 0 {
		return fmt.Errorf("no source files found for job %s", job.ID)
	}

	log.Printf("Job %s: Found %d source files", job.ID, len(sourceFiles))

	// Télécharger chaque fichier
	for _, filename := range sourceFiles {
		reader, err := p.storageService.DownloadJobSource(ctx, job.ID, filename)
		if err != nil {
			return fmt.Errorf("failed to download source file %s: %w", filename, err)
		}

		if err := workspace.WriteFile(filename, reader); err != nil {
			return fmt.Errorf("failed to write source file %s: %w", filename, err)
		}

		log.Printf("Job %s: Downloaded source file %s", job.ID, filename)
	}

	return nil
}

// uploadResults upload les résultats générés vers le storage
func (p *JobProcessor) uploadResults(ctx context.Context, job *models.GenerationJob, workspace *Workspace) error {
	distPath := workspace.GetDistPath()

	// Lister les fichiers générés
	resultFiles, err := workspace.ListFiles(distPath)
	if err != nil {
		return fmt.Errorf("failed to list result files: %w", err)
	}

	if len(resultFiles) == 0 {
		return fmt.Errorf("no result files generated")
	}

	log.Printf("Job %s: Found %d result files", job.ID, len(resultFiles))

	// Upload chaque fichier de résultat
	for _, filename := range resultFiles {
		reader, err := workspace.ReadFile(fmt.Sprintf("%s/%s", distPath, filename))
		if err != nil {
			return fmt.Errorf("failed to read result file %s: %w", filename, err)
		}

		if err := p.storageService.UploadResult(ctx, job.CourseID, filename, reader); err != nil {
			return fmt.Errorf("failed to upload result file %s: %w", filename, err)
		}

		log.Printf("Job %s: Uploaded result file %s", job.ID, filename)
	}

	return nil
}

// saveJobLogs sauvegarde les logs du job
func (p *JobProcessor) saveJobLogs(ctx context.Context, jobID uuid.UUID, logs []string) error {
	logContent := ""
	for _, line := range logs {
		logContent += line + "\n"
	}

	return p.storageService.SaveJobLog(ctx, jobID, logContent)
}

// updateJobStatus met à jour le statut d'un job
func (p *JobProcessor) updateJobStatus(ctx context.Context, jobID uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	return p.jobService.UpdateJobStatus(ctx, jobID, status, progress, errorMsg)
}
