// internal/worker/theme_manager.go
package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/pkg/models"
)

// ThemeManager gère l'installation et la détection des thèmes Slidev
type ThemeManager struct {
	workspaceBase string
	npmCommand    string
	mu            sync.RWMutex
}

// NewThemeManager crée un nouveau gestionnaire de thèmes
func NewThemeManager(workspaceBase string) *ThemeManager {
	npmCmd := "npm"

	return &ThemeManager{
		workspaceBase: workspaceBase,
		npmCommand:    npmCmd,
	}
}

// DetectMissingThemes analyse le contenu Slidev et détecte les thèmes manquants
func (tm *ThemeManager) DetectMissingThemes(ctx context.Context, workspace *Workspace) ([]string, error) {
	var missingThemes []string

	// Lire le fichier slides.md ou autres fichiers de slides
	slideFiles := []string{"slides.md", "index.md", "README.md"}

	for _, slideFile := range slideFiles {
		if workspace.FileExists(slideFile) {
			content, err := workspace.readFileContent(slideFile)
			if err != nil {
				continue
			}

			// Extraire les thèmes du frontmatter
			themes := tm.extractThemesFromContent(content)

			for _, theme := range themes {
				if !tm.isThemeInstalled(workspace, theme) {
					missingThemes = append(missingThemes, theme)
				}
			}
		}
	}

	// Supprimer les doublons
	return tm.removeDuplicates(missingThemes), nil
}

// extractThemesFromContent extrait les thèmes du contenu Slidev
func (tm *ThemeManager) extractThemesFromContent(content string) []string {
	var themes []string

	// Regex pour détecter les thèmes dans le frontmatter
	patterns := []string{
		`theme:\s*['"]*([^'"\s]+)['"]*`,                   // theme: default
		`@slidev/theme-([^'"\s]+)`,                        // @slidev/theme-seriph
		`slidev-theme-([^'"\s]+)`,                         // slidev-theme-custom
		`import.*from.*['""]@slidev/theme-([^'""]+)['""]`, // import from theme
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			if len(match) > 1 {
				theme := match[1]
				if theme != "none" {
					themes = append(themes, tm.normalizeThemeName(theme))
				}
			}
		}
	}

	return themes
}

// normalizeThemeName normalise le nom du thème
func (tm *ThemeManager) normalizeThemeName(theme string) string {
	// Convertir en format npm package
	if !strings.HasPrefix(theme, "@slidev/theme-") && !strings.HasPrefix(theme, "slidev-theme-") {
		if strings.Contains(theme, "/") {
			return theme // Déjà un package npm complet
		}
		return "@slidev/theme-" + theme
	}
	return theme
}

// IsThemeInstalled vérifie si un thème est installé (méthode publique)
func (tm *ThemeManager) IsThemeInstalled(workspace *Workspace, theme string) bool {
	return tm.isThemeInstalled(workspace, theme)
}

// isThemeInstalled vérifie si un thème est installé (méthode privée)
func (tm *ThemeManager) isThemeInstalled(workspace *Workspace, theme string) bool {
	// Vérifier dans node_modules
	nodeModulesPath := "node_modules/" + theme
	return workspace.DirExists(nodeModulesPath)
}

// InstallMultipleThemes installe plusieurs thèmes
func (tm *ThemeManager) InstallMultipleThemes(ctx context.Context, workspace *Workspace, themes []string) ([]*models.ThemeInstallResult, error) {
	var results []*models.ThemeInstallResult

	for _, theme := range themes {
		log.Printf("Installing theme %s (%d/%d)", theme, len(results)+1, len(themes))

		result, err := tm.InstallTheme(ctx, workspace, theme)
		results = append(results, result)

		if err != nil {
			log.Printf("Failed to install theme %s: %v", theme, err)
			// Continuer avec les autres thèmes même si un échoue
		}

		// Petite pause entre les installations
		time.Sleep(1 * time.Second)
	}

	return results, nil
}

// ListInstalledThemes liste les thèmes installés
func (tm *ThemeManager) ListInstalledThemes(ctx context.Context, workspace *Workspace) ([]models.ThemeInfo, error) {
	var themes []models.ThemeInfo

	// Lire package.json
	if !workspace.FileExists("package.json") {
		return themes, nil
	}

	content, err := workspace.readFileContent("package.json")
	if err != nil {
		return nil, err
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil, err
	}

	// Extraire les thèmes des dépendances
	for _, depType := range []string{"dependencies", "devDependencies"} {
		if deps, ok := pkg[depType].(map[string]interface{}); ok {
			for name, version := range deps {
				if strings.Contains(name, "slidev-theme-") || strings.Contains(name, "@slidev/theme-") {
					themes = append(themes, models.ThemeInfo{
						Name:      name,
						Version:   fmt.Sprintf("%v", version),
						Installed: true,
					})
				}
			}
		}
	}

	return themes, nil
}

// captureOutput capture la sortie d'un stream
func (tm *ThemeManager) captureOutput(reader io.Reader, prefix string, logChan chan<- string) {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		logLine := fmt.Sprintf("[%s] %s: %s", timestamp, prefix, line)

		select {
		case logChan <- logLine:
		default:
			// Canal plein, ignorer cette ligne
		}
	}
}

// removeDuplicates supprime les doublons d'une slice de strings
func (tm *ThemeManager) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// Helper pour lire le contenu d'un fichier dans le workspace
func (w *Workspace) readFileContent(filename string) (string, error) {
	reader, err := w.ReadFile(filename)
	if err != nil {
		return "", err
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// InstallTheme installe un thème Slidev
func (tm *ThemeManager) InstallTheme(ctx context.Context, workspace *Workspace, theme string) (*models.ThemeInstallResult, error) {
	startTime := time.Now()
	result := &models.ThemeInstallResult{
		Theme:   theme,
		Success: false,
		Logs:    []string{},
	}

	// Validation des entrées
	if theme == "" {
		result.Error = "theme name cannot be empty"
		return result, fmt.Errorf("theme name cannot be empty")
	}

	// Normaliser le nom du thème
	normalizedTheme := tm.normalizeThemeName(theme)
	result.Theme = normalizedTheme

	log.Printf("Installing Slidev theme: %s", normalizedTheme)
	result.Logs = append(result.Logs, fmt.Sprintf("Starting installation of theme: %s", normalizedTheme))

	// Créer un contexte avec timeout si pas déjà présent
	installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Préparer la commande d'installation
	cmd := tm.prepareInstallCommand(installCtx, workspace, normalizedTheme)

	// Configurer la gestion des erreurs et des pipes
	if err := tm.setupCommandPipes(cmd, result); err != nil {
		result.Error = fmt.Sprintf("Failed to setup command pipes: %v", err)
		return result, err
	}

	// Démarrer la commande
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Sprintf("Failed to start installation command: %v", err)
		return result, err
	}

	// Gérer l'installation de manière robuste
	if err := tm.handleInstallation(installCtx, cmd, result); err != nil {
		// La commande a échoué, mais on a des logs utiles
		result.Duration = int64(time.Since(startTime))
		return result, err
	}

	// Finaliser l'installation
	result.Duration = int64(time.Since(startTime))
	result.Installed = tm.isThemeInstalled(workspace, normalizedTheme)
	result.Success = result.Installed

	if result.Success {
		result.Logs = append(result.Logs, fmt.Sprintf("SUCCESS: Theme %s installed in %v", normalizedTheme, result.Duration))
		log.Printf("Theme %s installed successfully in %v", normalizedTheme, result.Duration)
	} else {
		result.Error = "Theme installation completed but theme not detected as installed"
		result.Logs = append(result.Logs, "WARNING: Installation completed but theme not detected")
	}

	return result, nil
}

// prepareInstallCommand prépare la commande d'installation
func (tm *ThemeManager) prepareInstallCommand(ctx context.Context, workspace *Workspace, theme string) *exec.Cmd {
	var cmd *exec.Cmd
	if tm.npmCommand == "yarn" {
		cmd = exec.CommandContext(ctx, "yarn", "add", theme)
	} else {
		cmd = exec.CommandContext(ctx, "npm", "install", theme, "--save")
	}

	cmd.Dir = workspace.GetPath()
	cmd.Env = tm.buildInstallEnvironment()

	return cmd
}

// setupCommandPipes configure les pipes de manière sécurisée
func (tm *ThemeManager) setupCommandPipes(cmd *exec.Cmd, result *models.ThemeInstallResult) error {
	// Configurer stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Configurer stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Configurer stdin de manière sécurisée
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Stocker les pipes pour la gestion
	result.Pipes = &models.InstallPipes{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  stdin,
	}

	return nil
}

// handleInstallation gère l'installation de manière robuste
func (tm *ThemeManager) handleInstallation(ctx context.Context, cmd *exec.Cmd, result *models.ThemeInstallResult) error {
	// Channels pour la coordination
	logsChan := make(chan string, 100)
	errChan := make(chan error, 3)
	done := make(chan struct{})
	captureCtx, captureCancel := context.WithCancel(ctx)

	// WaitGroup pour attendre que toutes les goroutines se terminent
	var wg sync.WaitGroup

	// Démarrer la capture des logs
	wg.Add(2)
	go func() {
		defer wg.Done()
		tm.safeOutputCapture(captureCtx, result.Pipes.Stdout, "STDOUT", logsChan, errChan)
	}()
	go func() {
		defer wg.Done()
		tm.safeOutputCapture(captureCtx, result.Pipes.Stderr, "STDERR", logsChan, errChan)
	}()

	// Gérer stdin de manière sécurisée
	wg.Add(1)
	go func() {
		defer wg.Done()
		tm.safeInputHandler(captureCtx, result.Pipes.Stdin, errChan)
	}()

	// Collecter les logs
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for {
			select {
			case <-captureCtx.Done():
				// Vider le channel restant
				for {
					select {
					case logLine := <-logsChan:
						result.Logs = append(result.Logs, logLine)
					default:
						return
					}
				}
			case logLine, ok := <-logsChan:
				if !ok {
					return
				}
				result.Logs = append(result.Logs, logLine)
			}
		}
	}()

	// Attendre la fin de la commande avec gestion du contexte
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	var cmdErr error
	select {
	case <-ctx.Done():
		// Context annulé - tuer le processus
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// Attendre que la commande se termine
		<-cmdDone
		cmdErr = ctx.Err()
		result.Logs = append(result.Logs, fmt.Sprintf("Installation cancelled: %v", ctx.Err()))
	case cmdErr = <-cmdDone:
		// Commande terminée normalement
	}

	// Arrêter toutes les goroutines de capture
	captureCancel()

	// Attendre que toutes les goroutines se terminent avant de fermer le channel
	go func() {
		wg.Wait()
		close(logsChan)
	}()

	// Attendre la fin de la collecte des logs
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("Warning: Log collection timeout")
	}

	// Analyser le résultat
	if cmdErr != nil {
		// Gestion spécifique des erreurs de contexte
		if cmdErr == context.DeadlineExceeded {
			result.Error = "Installation timeout: context deadline exceeded"
			result.ExitCode = -2
		} else if cmdErr == context.Canceled {
			result.Error = "Installation cancelled: context canceled"
			result.ExitCode = -3
		} else if exitError, ok := cmdErr.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Error = fmt.Sprintf("Installation failed: %v", cmdErr)
		} else {
			result.ExitCode = 1
			result.Error = fmt.Sprintf("Installation failed: %v", cmdErr)
		}

		result.Logs = append(result.Logs, fmt.Sprintf("ERROR: %v", cmdErr))
		return cmdErr
	}

	result.ExitCode = 0
	return nil
}

// safeOutputCapture capture la sortie de manière sécurisée
func (tm *ThemeManager) safeOutputCapture(ctx context.Context, reader io.ReadCloser, prefix string, logChan chan<- string, errChan chan<- error) {
	defer func() {
		if err := reader.Close(); err != nil {
			// Ne plus logger cette erreur car elle est normale quand le processus se termine
		}
	}()

	scanner := bufio.NewScanner(reader)

	// Limiter la taille des lignes pour éviter les attaques DoS
	const maxLineSize = 64 * 1024 // 64KB par ligne max
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)

	for scanner.Scan() {
		// Vérifier si le contexte est annulé
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		logLine := fmt.Sprintf("[%s] %s: %s", timestamp, prefix, line)

		// Essayer d'envoyer le log, mais s'arrêter si le contexte est annulé
		select {
		case logChan <- logLine:
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Canal plein, ignorer cette ligne pour éviter le blocage
		}
	}

	if err := scanner.Err(); err != nil {
		errMsg := fmt.Sprintf("Error reading %s output: %v", prefix, err)
		select {
		case errChan <- fmt.Errorf(errMsg):
		case logChan <- errMsg:
		case <-ctx.Done():
		default:
			// Si tous les channels sont pleins ou fermés, logger au moins
			log.Printf("Warning: %s", errMsg)
		}
	}
}

// safeInputHandler gère stdin de manière sécurisée
func (tm *ThemeManager) safeInputHandler(ctx context.Context, stdin io.WriteCloser, errChan chan<- error) {
	defer func() {
		if err := stdin.Close(); err != nil {
			// Ne plus logger cette erreur car elle est normale
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Essayer d'écrire, mais gérer les erreurs silencieusement
			if _, err := stdin.Write([]byte("y\n")); err != nil {
				// Stdin fermé ou erreur - arrêter silencieusement
				return
			}
		}
	}
}

// buildInstallEnvironment construit l'environnement pour l'installation - VERSION SÉCURISÉE
func (tm *ThemeManager) buildInstallEnvironment() []string {
	env := os.Environ()

	// Variables pour éviter les prompts interactifs
	secureEnvVars := []string{
		"NPM_CONFIG_YES=true",
		"NPM_CONFIG_AUDIT=false",
		"NPM_CONFIG_FUND=false",
		"NPM_CONFIG_UPDATE_NOTIFIER=false",
		"NPM_CONFIG_PROGRESS=false",
		"CI=true",
		"DEBIAN_FRONTEND=noninteractive",
		// Limiter les ressources
		"NPM_CONFIG_MAXSOCKETS=5",
		"NPM_CONFIG_TIMEOUT=300000", // 5 minutes
	}

	return append(env, secureEnvVars...)
}

// AutoInstallMissingThemes installe automatiquement les thèmes manquants - VERSION ROBUSTE
func (tm *ThemeManager) AutoInstallMissingThemes(ctx context.Context, workspace *Workspace) ([]*models.ThemeInstallResult, error) {
	log.Printf("Auto-detecting missing Slidev themes...")

	// Détecter les thèmes manquants
	missingThemes, err := tm.DetectMissingThemes(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to detect missing themes: %w", err)
	}

	if len(missingThemes) == 0 {
		log.Printf("No missing themes detected")
		return []*models.ThemeInstallResult{}, nil
	}

	log.Printf("Found %d missing themes: %v", len(missingThemes), missingThemes)

	// Limiter le nombre de thèmes à installer simultanément
	const maxConcurrent = 3
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []*models.ThemeInstallResult

	// Installer les thèmes avec limite de concurrence
	for _, theme := range missingThemes {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()

			// Acquérir le semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			log.Printf("Installing theme: %s", t)
			result, err := tm.InstallTheme(ctx, workspace, t)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if err != nil {
				log.Printf("Failed to install theme %s: %v", t, err)
			}
		}(theme)
	}

	wg.Wait()
	return results, nil
}
