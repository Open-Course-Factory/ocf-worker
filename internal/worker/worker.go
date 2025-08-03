// internal/worker/worker.go
package worker

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/internal/jobs"
	"github.com/Open-Course-Factory/ocf-worker/internal/storage"
	"github.com/Open-Course-Factory/ocf-worker/pkg/models"

	"github.com/google/uuid"
)

// Worker représente un worker individuel qui traite les jobs
type Worker struct {
	id             int
	jobService     jobs.JobService
	storageService *storage.StorageService
	config         *PoolConfig
	processor      *JobProcessor

	// État du worker - protégé par mutex
	mu           sync.RWMutex
	status       string
	currentJobID uuid.UUID

	// Statistiques - utiliser atomic pour éviter les locks
	jobsTotal   int64
	jobsSuccess int64
	jobsFailed  int64
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
		currentJobID:   uuid.Nil,
	}
}

// setState met à jour l'état du worker de manière atomique
func (w *Worker) setState(status string, jobID uuid.UUID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.status = status
	w.currentJobID = jobID
}

// getState retourne l'état actuel du worker
func (w *Worker) getState() (string, uuid.UUID) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.status, w.currentJobID
}

// processJob traite un job individuel - VERSION CORRIGÉE
func (w *Worker) processJob(ctx context.Context, job *models.GenerationJob) {
	// Mise à jour atomique de l'état
	w.setState("busy", job.ID)
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

	// Nettoyer l'état du worker - atomique
	w.setState("idle", uuid.Nil)
}

// GetStats retourne les statistiques du worker - VERSION CORRIGÉE
func (w *Worker) GetStats() WorkerStatsInternal {
	// Récupérer l'état de manière thread-safe
	status, currentJobID := w.getState()

	currentJobIDStr := ""
	if currentJobID != uuid.Nil {
		currentJobIDStr = currentJobID.String()
	}

	return WorkerStatsInternal{
		Status:       status,
		CurrentJobID: currentJobIDStr,
		JobsTotal:    atomic.LoadInt64(&w.jobsTotal),
		JobsSuccess:  atomic.LoadInt64(&w.jobsSuccess),
		JobsFailed:   atomic.LoadInt64(&w.jobsFailed),
	}
}

// Start démarre le worker et écoute la queue des jobs
func (w *Worker) Start(ctx context.Context, jobQueue <-chan *models.GenerationJob) {
	log.Printf("Worker %d starting", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopped due to context cancellation", w.id)
			w.setState("stopped", uuid.Nil)
			return
		case job, ok := <-jobQueue:
			if !ok {
				log.Printf("Worker %d stopped - job queue closed", w.id)
				w.setState("stopped", uuid.Nil)
				return
			}

			w.processJob(ctx, job)
		}
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
		if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusFailed, 0, result.Error.Error()); errUpdate != nil {
			log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
		}
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
		if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusFailed, 20, result.Error.Error()); errUpdate != nil {
			log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
		}
		return result
	}

	if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 30, "Sources downloaded"); errUpdate != nil {
		log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
	}
	result.Progress = 30

	// Étape 2: Préparer l'environnement Slidev
	log.Printf("Job %s: Preparing Slidev environment", job.ID)
	if err := p.prepareSlidevEnvironment(ctx, job, workspace); err != nil {
		log.Printf("Job %s: Slidev preparation failed (non-fatal): %v", job.ID, err)
	}

	if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 40, "Environment prepared"); errUpdate != nil {
		log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
	}

	//

	// Étape 3: Exécuter Slidev build
	log.Printf("Job %s: Running Slidev build", job.ID)
	slidevResult, err := p.slidevRunner.Build(ctx, workspace, job)
	if err != nil {
		result.Error = fmt.Errorf("slidev build failed: %w", err)
		if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusFailed, 50, result.Error.Error()); errUpdate != nil {
			log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
		}

		// Sauvegarder les logs même en cas d'échec
		if len(slidevResult.Logs) > 0 {
			errSave := p.saveJobLogs(ctx, job.ID, slidevResult.Logs)
			log.Printf("Failed to save logs for job %s: %v", job.ID, errSave)
		}

		// Debug: lister le contenu du workspace
		p.debugWorkspaceContents(workspace, job.ID)
		return result
	}

	if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 70, "Slidev build completed"); errUpdate != nil {
		log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
	}
	result.Progress = 70
	result.LogOutput = slidevResult.Logs

	// Étape 4: Upload des résultats
	log.Printf("Job %s: Uploading results", job.ID)
	if err := p.uploadResults(ctx, job, workspace); err != nil {
		result.Error = fmt.Errorf("failed to upload results: %w", err)
		if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusFailed, 80, result.Error.Error()); errUpdate != nil {
			log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
		}

		// Debug: lister le contenu du workspace
		p.debugWorkspaceContents(workspace, job.ID)
		return result
	}

	if errUpdate := p.updateJobStatus(ctx, job.ID, models.StatusProcessing, 90, "Results uploaded"); errUpdate != nil {
		log.Printf("Job %s: update failed: %v", job.ID, errUpdate)
	}

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
	// Lister les fichiers sources (peut inclure des chemins avec dossiers)
	sourceFiles, err := p.storageService.ListJobSources(ctx, job.ID)
	if err != nil {
		return fmt.Errorf("failed to list source files: %w", err)
	}

	if len(sourceFiles) == 0 {
		return fmt.Errorf("no source files found for job %s", job.ID)
	}

	log.Printf("Job %s: Found %d source files with paths", job.ID, len(sourceFiles))

	// Organiser les fichiers par dossier pour un meilleur logging
	dirMap := make(map[string][]string)
	for _, filePath := range sourceFiles {
		dir := filepath.Dir(filePath)
		if dir == "." {
			dir = "root"
		}
		dirMap[dir] = append(dirMap[dir], filepath.Base(filePath))
	}

	// Logger la structure détectée
	for dir, files := range dirMap {
		log.Printf("Job %s: Directory '%s' contains %d files: %v", job.ID, dir, len(files), files)
	}

	// Télécharger chaque fichier en préservant la structure
	for _, filePath := range sourceFiles {
		reader, err := p.storageService.DownloadJobSource(ctx, job.ID, filePath)
		if err != nil {
			return fmt.Errorf("failed to download source file %s: %w", filePath, err)
		}

		// WriteFile va automatiquement créer les dossiers parents
		if err := workspace.WriteFile(filePath, reader); err != nil {
			return fmt.Errorf("failed to write source file %s to workspace: %w", filePath, err)
		}

		log.Printf("Job %s: Downloaded and placed source file %s", job.ID, filePath)
	}

	// Vérifier la structure créée dans le workspace
	if err := p.verifyWorkspaceStructure(workspace, job.ID); err != nil {
		log.Printf("Job %s: Warning - workspace structure verification failed: %v", job.ID, err)
		// Ne pas échouer le job pour cela, juste logger un warning
	}

	return nil
}

// verifyWorkspaceStructure vérifie que la structure de dossiers a été correctement créée
func (p *JobProcessor) verifyWorkspaceStructure(workspace *Workspace, jobID uuid.UUID) error {
	// Lister tous les fichiers dans le workspace
	allFiles, err := workspace.ListAllFiles(".")
	if err != nil {
		return fmt.Errorf("failed to list workspace files: %w", err)
	}

	log.Printf("Job %s: Workspace structure verification - found %d files:", jobID, len(allFiles))

	// Organiser et logger la structure
	dirStructure := make(map[string][]string)
	for _, file := range allFiles {
		dir := filepath.Dir(file)
		if dir == "." {
			dir = "root"
		}
		dirStructure[dir] = append(dirStructure[dir], filepath.Base(file))
	}

	for dir, files := range dirStructure {
		log.Printf("Job %s: Workspace directory '%s': %v", jobID, dir, files)
	}

	return nil
}

// uploadResults upload les résultats générés vers le storage
func (p *JobProcessor) uploadResults(ctx context.Context, job *models.GenerationJob, workspace *Workspace) error {
	distPath := workspace.GetDistPath()

	// Lister tous les fichiers générés (y compris dans les sous-dossiers)
	resultFiles, err := workspace.ListAllFiles(distPath)
	if err != nil {
		return fmt.Errorf("failed to list result files: %w", err)
	}

	if len(resultFiles) == 0 {
		return fmt.Errorf("no result files generated")
	}

	log.Printf("Job %s: Found %d result files with structure", job.ID, len(resultFiles))

	// Organiser les résultats par dossier pour le logging
	dirMap := make(map[string][]string)
	for _, filePath := range resultFiles {
		dir := filepath.Dir(filePath)
		if dir == "." {
			dir = "root"
		}
		dirMap[dir] = append(dirMap[dir], filepath.Base(filePath))
	}

	for dir, files := range dirMap {
		log.Printf("Job %s: Result directory '%s' contains %d files: %v", job.ID, dir, len(files), files)
	}

	// Upload chaque fichier de résultat en préservant la structure
	for _, relativePath := range resultFiles {
		fullPath := fmt.Sprintf("%s/%s", distPath, relativePath)
		reader, err := workspace.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read result file %s: %w", relativePath, err)
		}

		// UploadResult va maintenant préserver la structure de dossiers
		if err := p.storageService.UploadResult(ctx, job.CourseID, relativePath, reader); err != nil {
			return fmt.Errorf("failed to upload result file %s: %w", relativePath, err)
		}

		log.Printf("Job %s: Uploaded result file %s", job.ID, relativePath)
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
