// internal/worker/slidev.go
package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ocf-worker/pkg/models"
)

// SlidevRunner exécute les commandes Slidev
type SlidevRunner struct {
	config       *PoolConfig
	themeManager *ThemeManager
}

// SlidevResult contient le résultat de l'exécution Slidev
type SlidevResult struct {
	Success    bool
	ExitCode   int
	Logs       []string
	Duration   time.Duration
	OutputPath string
}

// NewSlidevRunner crée un nouveau runner Slidev
func NewSlidevRunner(config *PoolConfig) *SlidevRunner {
	return &SlidevRunner{
		config:       config,
		themeManager: NewThemeManager(config.WorkspaceBase),
	}
}

func (sr *SlidevRunner) InstallMissingThemes(ctx context.Context, workspace *Workspace, job *models.GenerationJob) error {
	log.Printf("Job %s: Checking for missing themes...", job.ID)

	// Auto-installer les thèmes manquants
	results, err := sr.themeManager.AutoInstallMissingThemes(ctx, workspace)
	if err != nil {
		return fmt.Errorf("failed to auto-install themes: %w", err)
	}

	if len(results) == 0 {
		log.Printf("Job %s: No missing themes detected", job.ID)
		return nil
	}

	// Vérifier les résultats
	var failedThemes []string
	var successThemes []string

	for _, result := range results {
		if result.Success {
			successThemes = append(successThemes, result.Theme)
			log.Printf("Job %s: Successfully installed theme: %s", job.ID, result.Theme)
		} else {
			failedThemes = append(failedThemes, result.Theme)
			log.Printf("Job %s: Failed to install theme: %s - %s", job.ID, result.Theme, result.Error)
		}
	}

	if len(successThemes) > 0 {
		log.Printf("Job %s: Installed %d themes: %v", job.ID, len(successThemes), successThemes)
	}

	if len(failedThemes) > 0 {
		return fmt.Errorf("failed to install %d themes: %v", len(failedThemes), failedThemes)
	}

	return nil
}

// Build exécute `slidev build` dans le workspace avec validation améliorée
func (sr *SlidevRunner) Build(ctx context.Context, workspace *Workspace, job *models.GenerationJob) (*SlidevResult, error) {
	startTime := time.Now()
	result := &SlidevResult{
		Success: false,
		Logs:    []string{},
	}

	// Vérifier les prérequis
	if err := sr.checkPrerequisites(ctx, workspace, job); err != nil {
		result.Logs = append(result.Logs, fmt.Sprintf("ERROR: Prerequisites check failed: %v", err))
		return result, fmt.Errorf("prerequisites check failed: %w", err)
	}

	result.Logs = append(result.Logs, "Checking and installing missing themes...")
	if err := sr.InstallMissingThemes(ctx, workspace, job); err != nil {
		result.Logs = append(result.Logs, fmt.Sprintf("WARNING: Theme installation failed: %v", err))
		// On continue quand même, car les thèmes peuvent être optionnels
		log.Printf("Job %s: Theme installation failed but continuing: %v", job.ID, err)
	} else {
		result.Logs = append(result.Logs, "Theme installation completed successfully")
	}

	// Préparer la commande Slidev
	cmd := sr.prepareBuildCommand(ctx, workspace)

	// Configurer la capture des logs
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Démarrer la commande
	log.Printf("Job %s: Starting Slidev build command: %s", job.ID, cmd.String())
	result.Logs = append(result.Logs, fmt.Sprintf("Starting command: %s", cmd.String()))
	result.Logs = append(result.Logs, fmt.Sprintf("Working directory: %s", workspace.GetPath()))

	// Créer un pipe
	reader, writer := io.Pipe()
	cmd.Stdin = reader

	// Goroutine pour alimenter le pipe avec des "y"
	go func() {
		defer writer.Close()
		for {
			_, err := writer.Write([]byte("y\n"))
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("failed to start slidev command: %w", err)
	}

	// Capturer les logs en temps réel
	logChan := make(chan string, 100)
	go sr.captureOutput(stdout, "STDOUT", logChan)
	go sr.captureOutput(stderr, "STDERR", logChan)

	// Collecter les logs
	go func() {
		for logLine := range logChan {
			result.Logs = append(result.Logs, logLine)

			// Optionnel: détecter le progress depuis les logs Slidev
			if progress := sr.parseProgress(logLine); progress > 0 {
				// Ici on pourrait mettre à jour le progress en temps réel
				log.Printf("Job %s: Slidev progress detected: %d%%", job.ID, progress)
			}
		}
	}()

	// Attendre la fin de la commande avec timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Timeout ou annulation
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		result.ExitCode = -1
		result.Logs = append(result.Logs, fmt.Sprintf("ERROR: Command timeout or cancelled after %v", time.Since(startTime)))
		return result, fmt.Errorf("slidev build timeout or cancelled")

	case err := <-done:
		// Commande terminée
		close(logChan)

		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitError.ExitCode()
			} else {
				result.ExitCode = 1
			}
			result.Logs = append(result.Logs, fmt.Sprintf("ERROR: Slidev build failed: %v", err))

			// Afficher le contenu du workspace pour debug
			sr.debugWorkspaceState(workspace, result)

			return result, fmt.Errorf("slidev build failed with exit code %d: %w", result.ExitCode, err)
		}

		result.ExitCode = 0
		result.Success = true
		result.Duration = time.Since(startTime)
		result.OutputPath = workspace.GetDistPath()

		log.Printf("Job %s: Slidev build completed successfully in %v", job.ID, result.Duration)

		// Vérifier que les fichiers de sortie existent
		if err := sr.validateOutput(workspace); err != nil {
			result.Success = false
			result.Logs = append(result.Logs, fmt.Sprintf("ERROR: Output validation failed: %v", err))

			// Debug détaillé en cas d'échec de validation
			sr.debugWorkspaceState(workspace, result)

			return result, fmt.Errorf("slidev output validation failed: %w", err)
		}

		result.Logs = append(result.Logs, fmt.Sprintf("SUCCESS: Slidev build completed in %v", result.Duration))
		return result, nil
	}
}

// checkPrerequisites vérifie que tous les prérequis sont présents
func (sr *SlidevRunner) checkPrerequisites(ctx context.Context, workspace *Workspace, job *models.GenerationJob) error {
	// Vérifier qu'il y a au moins un fichier de slides
	slideFiles := []string{"slides.md", "index.md", "README.md"}
	hasSlideFile := false

	for _, file := range slideFiles {
		if workspace.FileExists(file) {
			hasSlideFile = true
			log.Printf("Job %s: Found slide file: %s", job.ID, file)
			break
		}
	}

	if !hasSlideFile {
		return fmt.Errorf("no slide file found (checked: %v)", slideFiles)
	}

	// Vérifier que Slidev est disponible
	cmd := exec.CommandContext(ctx, "npx", "@slidev/cli", "--version")
	if output, err := cmd.Output(); err != nil {
		return fmt.Errorf("slidev not available: %w", err)
	} else {
		version := strings.TrimSpace(string(output))
		log.Printf("Job %s: Using Slidev version: %s", job.ID, version)
	}

	return nil
}

// debugWorkspaceState affiche l'état détaillé du workspace pour debug
func (sr *SlidevRunner) debugWorkspaceState(workspace *Workspace, result *SlidevResult) {
	result.Logs = append(result.Logs, "=== WORKSPACE DEBUG ===")

	// Lister tous les fichiers
	if files, err := workspace.ListAllFiles("."); err == nil {
		result.Logs = append(result.Logs, fmt.Sprintf("Workspace files (%d total):", len(files)))
		for _, file := range files {
			if size, err := workspace.GetFileSize(file); err == nil {
				result.Logs = append(result.Logs, fmt.Sprintf("  %s (%d bytes)", file, size))
			}
		}
	} else {
		result.Logs = append(result.Logs, fmt.Sprintf("Failed to list workspace files: %v", err))
	}

	// Vérifier les répertoires de sortie possibles
	outputDirs := []string{"dist", "build", "output", "_output", ".slidev", "node_modules"}
	for _, dir := range outputDirs {
		if workspace.DirExists(dir) {
			result.Logs = append(result.Logs, fmt.Sprintf("Found directory: %s/", dir))
			if files, err := workspace.ListFiles(dir); err == nil {
				for _, file := range files[:min(len(files), 10)] { // Limiter à 10 fichiers
					result.Logs = append(result.Logs, fmt.Sprintf("  %s/%s", dir, file))
				}
				if len(files) > 10 {
					result.Logs = append(result.Logs, fmt.Sprintf("  ... and %d more files", len(files)-10))
				}
			}
		}
	}

	result.Logs = append(result.Logs, "=== END DEBUG ===")
}

// min retourne le minimum entre deux entiers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// prepareBuildCommand prépare la commande Slidev build avec le bon répertoire de sortie
func (sr *SlidevRunner) prepareBuildCommand(ctx context.Context, workspace *Workspace) *exec.Cmd {
	// Détecter la commande Slidev à utiliser
	slidevCmd := sr.detectSlidevCommand()

	// Arguments pour la build avec répertoire de sortie explicite
	args := []string{"build", "--out", "./dist", "--theme", "@slidev/theme-default"}

	// Vérifier s'il y a un fichier de configuration spécifique
	if workspace.FileExists("slidev.config.js") || workspace.FileExists("slidev.config.ts") {
		log.Printf("Found Slidev config file in workspace")
	}

	// Créer la commande
	var cmd *exec.Cmd
	if strings.Contains(slidevCmd, " ") {
		// Commande avec arguments (comme "npx @slidev/cli")
		parts := strings.Fields(slidevCmd)
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...)
	} else {
		// Commande simple
		cmd = exec.CommandContext(ctx, slidevCmd, args...)
	}

	// Définir le répertoire de travail
	cmd.Dir = workspace.GetPath()

	// Définir les variables d'environnement
	cmd.Env = sr.buildEnvironment()

	return cmd
}

// detectSlidevCommand détecte la meilleure commande Slidev à utiliser
func (sr *SlidevRunner) detectSlidevCommand() string {
	// Utiliser la configuration si définie
	if sr.config.SlidevCommand != "" {
		log.Printf("Detected Config Slidev command: %s", sr.config.SlidevCommand)
		return sr.config.SlidevCommand
	}

	// Essayer de détecter automatiquement
	commands := []string{
		"slidev",          // Installation globale
		"npx @slidev/cli", // Via npx (recommandé)
		"npm run slidev",  // Via package.json scripts
		"yarn slidev",     // Via yarn
	}

	for _, cmd := range commands {
		if sr.commandExists(strings.Fields(cmd)[0]) {
			log.Printf("Detected Slidev command: %s", cmd)
			return cmd
		}
	}

	// Fallback par défaut
	log.Printf("No Slidev command detected, using default: npx @slidev/cli")
	return "npx @slidev/cli"
}

// commandExists vérifie si une commande existe
func (sr *SlidevRunner) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// buildEnvironment construit l'environnement pour la commande Slidev
func (sr *SlidevRunner) buildEnvironment() []string {
	env := os.Environ()

	// Ajouter des variables spécifiques à Slidev
	env = append(env, "NODE_ENV=production")
	env = append(env, "SLIDEV_BUILD=true")

	// Configurer le cache NPM dans le workspace pour éviter les conflits
	env = append(env, "NPM_CONFIG_CACHE=/tmp/npm-cache")

	return env
}

// captureOutput capture la sortie d'un stream en temps réel
func (sr *SlidevRunner) captureOutput(reader io.Reader, prefix string, logChan chan<- string) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		logLine := fmt.Sprintf("[%s] %s: %s", timestamp, prefix, line)

		select {
		case logChan <- logLine:
		default:
			// Canal plein, ignorer cette ligne de log
		}
	}

	if err := scanner.Err(); err != nil {
		logLine := fmt.Sprintf("[%s] %s: ERROR reading output: %v", time.Now().Format("15:04:05"), prefix, err)
		select {
		case logChan <- logLine:
		default:
		}
	}
}

// parseProgress extrait le pourcentage de progression depuis les logs Slidev
func (sr *SlidevRunner) parseProgress(logLine string) int {
	// Expressions régulières pour détecter le progress dans les logs Slidev
	patterns := []string{
		`(\d+)%`,            // Simple percentage
		`\[(\d+)/(\d+)\]`,   // [current/total] format
		`Building.*(\d+)%`,  // Building 50%
		`Progress.*?(\d+)%`, // Progress: 75%
		`(\d+) of (\d+)`,    // X of Y format
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(logLine)

		if len(matches) >= 2 {
			if len(matches) == 3 && matches[2] != "" {
				// Format [current/total]
				current, err1 := strconv.Atoi(matches[1])
				total, err2 := strconv.Atoi(matches[2])
				if err1 == nil && err2 == nil && total > 0 {
					return int((float64(current) / float64(total)) * 100)
				}
			} else {
				// Simple percentage
				if progress, err := strconv.Atoi(matches[1]); err == nil {
					return progress
				}
			}
		}
	}

	return 0
}

// validateOutput vérifie que les fichiers de sortie ont été générés correctement
func (sr *SlidevRunner) validateOutput(workspace *Workspace) error {
	distPath := workspace.GetDistPath()

	// Vérifier que le répertoire dist existe
	if !workspace.DirExists(distPath) {
		// Lister le contenu du workspace pour debug
		workspaceFiles, _ := workspace.ListAllFiles(".")
		log.Printf("Workspace contents: %v", workspaceFiles)

		// Vérifier les répertoires alternatifs que Slidev pourrait créer
		altPaths := []string{"build", "output", "_output", ".slidev/dist"}
		for _, altPath := range altPaths {
			if workspace.DirExists(altPath) {
				log.Printf("Found alternative output directory: %s", altPath)
				// Copier vers dist/ pour uniformiser
				if err := sr.moveToDistDirectory(workspace, altPath); err != nil {
					log.Printf("Failed to move %s to dist: %v", altPath, err)
				} else {
					log.Printf("Moved %s to dist directory", altPath)
					break
				}
			}
		}

		// Vérifier à nouveau
		if !workspace.DirExists(distPath) {
			return fmt.Errorf("dist directory not found: %s (tried alternatives: %v)", distPath, altPaths)
		}
	}

	// Fichiers obligatoires pour une présentation Slidev
	requiredFiles := []string{
		"index.html",
	}

	for _, file := range requiredFiles {
		filePath := fmt.Sprintf("%s/%s", distPath, file)
		if !workspace.FileExists(filePath) {
			// Lister le contenu de dist pour debug
			distFiles, _ := workspace.ListFiles(distPath)
			log.Printf("Dist directory contents: %v", distFiles)
			return fmt.Errorf("required output file not found: %s", file)
		}
	}

	// Vérifier que index.html n'est pas vide
	indexPath := fmt.Sprintf("%s/index.html", distPath)
	if size, err := workspace.GetFileSize(indexPath); err != nil {
		return fmt.Errorf("failed to check index.html size: %w", err)
	} else if size < 100 {
		return fmt.Errorf("index.html is too small (%d bytes), build may have failed", size)
	}

	log.Printf("Output validation successful - found all required files")
	return nil
}

// moveToDistDirectory déplace un répertoire alternatif vers dist/
func (sr *SlidevRunner) moveToDistDirectory(workspace *Workspace, srcDir string) error {
	// Créer le répertoire dist
	if err := workspace.CreateDirectory("dist"); err != nil {
		return err
	}

	// Lister les fichiers du répertoire source
	files, err := workspace.ListAllFiles(srcDir)
	if err != nil {
		return err
	}

	// Copier chaque fichier
	for _, file := range files {
		srcPath := fmt.Sprintf("%s/%s", srcDir, file)
		dstPath := fmt.Sprintf("dist/%s", file)

		if err := workspace.CopyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
		}
	}

	return nil
}

// GetSlidevVersion retourne la version de Slidev installée
func (sr *SlidevRunner) GetSlidevVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "npx", "@slidev/cli", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Slidev version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// CheckDependencies vérifie que toutes les dépendances nécessaires sont disponibles
func (sr *SlidevRunner) CheckDependencies(ctx context.Context) error {
	// Vérifier Node.js
	if !sr.commandExists("node") {
		return fmt.Errorf("Node.js not found - required for Slidev")
	}

	// Vérifier npm ou yarn
	hasNpm := sr.commandExists("npm")
	hasYarn := sr.commandExists("yarn")

	if !hasNpm && !hasYarn {
		return fmt.Errorf("neither npm nor yarn found - required for Slidev")
	}

	// Vérifier npx si on utilise npx
	if strings.Contains(sr.config.SlidevCommand, "npx") && !sr.commandExists("npx") {
		return fmt.Errorf("npx not found - required for 'npx @slidev/cli'")
	}

	// Tenter d'obtenir la version de Slidev
	version, err := sr.GetSlidevVersion(ctx)
	if err != nil {
		return fmt.Errorf("Slidev not available: %w", err)
	}

	log.Printf("Slidev dependencies check passed - version: %s", version)
	return nil
}

// InstallDependencies installe les dépendances si un package.json est présent
func (sr *SlidevRunner) InstallDependencies(ctx context.Context, workspace *Workspace) error {
	packageJsonPath := filepath.Join(workspace.GetPath(), "package.json")

	// Vérifier si package.json existe
	if !workspace.FileExists(packageJsonPath) {
		log.Printf("No package.json found, skipping dependency installation")
		return nil
	}

	log.Printf("Found package.json, installing dependencies")

	// Choisir la commande d'installation
	var cmd *exec.Cmd
	if sr.commandExists("yarn") && workspace.FileExists("yarn.lock") {
		cmd = exec.CommandContext(ctx, "yarn", "install", "--frozen-lockfile")
	} else {
		cmd = exec.CommandContext(ctx, "npm", "ci")
	}

	cmd.Dir = workspace.GetPath()
	cmd.Env = sr.buildEnvironment()

	// Capturer la sortie
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dependency installation failed: %w\nOutput: %s", err, string(output))
	}

	log.Printf("Dependencies installed successfully")
	return nil
}

// SlidevBuildOptions contient les options pour la build Slidev
type SlidevBuildOptions struct {
	Theme   string            // Thème à utiliser
	Output  string            // Répertoire de sortie (par défaut: dist)
	Base    string            // Base URL
	Options map[string]string // Options additionnelles
	Export  *ExportOptions    // Options d'export (PDF, etc.)
}

// ExportOptions contient les options d'export
type ExportOptions struct {
	Format     string // pdf, png, md
	Output     string // Fichier de sortie
	WithClicks bool   // Inclure les animations de click
	Range      string // Pages à exporter (ex: "1-10")
}

// BuildWithOptions exécute Slidev build avec des options spécifiques
func (sr *SlidevRunner) BuildWithOptions(ctx context.Context, workspace *Workspace, job *models.GenerationJob, options *SlidevBuildOptions) (*SlidevResult, error) {
	if options == nil {
		return sr.Build(ctx, workspace, job)
	}

	// TODO: Implémenter le support des options avancées
	// Pour l'instant, on utilise la build standard
	return sr.Build(ctx, workspace, job)
}

// ExportToPDF exporte la présentation en PDF
func (sr *SlidevRunner) ExportToPDF(ctx context.Context, workspace *Workspace, job *models.GenerationJob, outputFile string) error {
	// Préparer la commande d'export PDF
	args := []string{"export", "--format", "pdf"}

	if outputFile != "" {
		args = append(args, "--output", outputFile)
	}

	slidevCmd := sr.detectSlidevCommand()
	var cmd *exec.Cmd

	if strings.Contains(slidevCmd, " ") {
		parts := strings.Fields(slidevCmd)
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...)
	} else {
		cmd = exec.CommandContext(ctx, slidevCmd, args...)
	}

	cmd.Dir = workspace.GetPath()
	cmd.Env = sr.buildEnvironment()

	// Exécuter la commande
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PDF export failed: %w\nOutput: %s", err, string(output))
	}

	log.Printf("Job %s: PDF export completed successfully", job.ID)
	return nil
}

// GetBuildInfo retourne des informations sur la build
func (sr *SlidevRunner) GetBuildInfo() map[string]interface{} {
	return map[string]interface{}{
		"slidev_command":    sr.config.SlidevCommand,
		"workspace_base":    sr.config.WorkspaceBase,
		"job_timeout":       sr.config.JobTimeout.String(),
		"cleanup_workspace": sr.config.CleanupWorkspace,
		"node_available":    sr.commandExists("node"),
		"npm_available":     sr.commandExists("npm"),
		"yarn_available":    sr.commandExists("yarn"),
		"npx_available":     sr.commandExists("npx"),
	}
}
