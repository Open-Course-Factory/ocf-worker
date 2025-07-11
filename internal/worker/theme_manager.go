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
	"time"
)

// ThemeManager gère l'installation et la détection des thèmes Slidev
type ThemeManager struct {
	workspaceBase string
	npmCommand    string
}

// ThemeInfo contient les informations sur un thème
type ThemeInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
}

// ThemeInstallResult contient le résultat d'une installation
type ThemeInstallResult struct {
	Theme     string        `json:"theme"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Logs      []string      `json:"logs"`
	Installed bool          `json:"installed"`
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
				if !tm.isThemeInstalled(ctx, workspace, theme) {
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
				if theme != "default" && theme != "none" {
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
func (tm *ThemeManager) IsThemeInstalled(ctx context.Context, workspace *Workspace, theme string) bool {
	return tm.isThemeInstalled(ctx, workspace, theme)
}

// isThemeInstalled vérifie si un thème est installé (méthode privée)
func (tm *ThemeManager) isThemeInstalled(ctx context.Context, workspace *Workspace, theme string) bool {
	// Vérifier dans package.json
	if workspace.FileExists("package.json") {
		content, err := workspace.readFileContent("package.json")
		if err == nil {
			var pkg map[string]interface{}
			if json.Unmarshal([]byte(content), &pkg) == nil {
				// Vérifier dependencies et devDependencies
				for _, depType := range []string{"dependencies", "devDependencies"} {
					if deps, ok := pkg[depType].(map[string]interface{}); ok {
						if _, exists := deps[theme]; exists {
							return true
						}
					}
				}
			}
		}
	}

	// Vérifier dans node_modules
	nodeModulesPath := "node_modules/" + theme
	return workspace.DirExists(nodeModulesPath)
}

// InstallTheme installe un thème Slidev
func (tm *ThemeManager) InstallTheme(ctx context.Context, workspace *Workspace, theme string) (*ThemeInstallResult, error) {
	startTime := time.Now()
	result := &ThemeInstallResult{
		Theme:   theme,
		Success: false,
		Logs:    []string{},
	}

	log.Printf("Installing Slidev theme: %s", theme)
	result.Logs = append(result.Logs, fmt.Sprintf("Starting installation of theme: %s", theme))

	// Normaliser le nom du thème
	normalizedTheme := tm.normalizeThemeName(theme)
	result.Theme = normalizedTheme

	// Préparer la commande d'installation
	var cmd *exec.Cmd
	if tm.npmCommand == "yarn" {
		cmd = exec.CommandContext(ctx, "yarn", "add", normalizedTheme)
	} else {
		cmd = exec.CommandContext(ctx, "npm", "install", normalizedTheme, "--save")
	}

	cmd.Dir = workspace.GetPath()
	cmd.Env = tm.buildInstallEnvironment()

	// Configurer les pipes pour capturer la sortie
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create stdout pipe: %v", err)
		return result, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create stderr pipe: %v", err)
		return result, err
	}

	// Configurer stdin pour répondre automatiquement aux prompts
	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create stdin pipe: %v", err)
		return result, err
	}

	// Démarrer la commande
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Sprintf("Failed to start installation command: %v", err)
		return result, err
	}

	// Goroutine pour répondre aux prompts automatiquement
	go func() {
		defer stdin.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				stdin.Write([]byte("y\n")) // Répondre "yes" aux prompts
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Capturer les logs en temps réel
	logsChan := make(chan string, 100)
	go tm.captureOutput(stdout, "STDOUT", logsChan)
	go tm.captureOutput(stderr, "STDERR", logsChan)

	// Collecter les logs
	go func() {
		for logLine := range logsChan {
			result.Logs = append(result.Logs, logLine)
		}
	}()

	// Attendre la fin de la commande avec timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		result.Error = "Installation timeout or cancelled"
		return result, fmt.Errorf("theme installation timeout")

	case err := <-done:
		close(logsChan)

		result.Duration = time.Since(startTime)

		if err != nil {
			result.Error = fmt.Sprintf("Installation failed: %v", err)
			result.Logs = append(result.Logs, fmt.Sprintf("ERROR: %v", err))
			return result, err
		}

		// Vérifier que le thème est maintenant installé
		result.Installed = tm.isThemeInstalled(ctx, workspace, normalizedTheme)
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
}

// InstallMultipleThemes installe plusieurs thèmes
func (tm *ThemeManager) InstallMultipleThemes(ctx context.Context, workspace *Workspace, themes []string) ([]*ThemeInstallResult, error) {
	var results []*ThemeInstallResult

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

// AutoInstallMissingThemes détecte et installe automatiquement les thèmes manquants
func (tm *ThemeManager) AutoInstallMissingThemes(ctx context.Context, workspace *Workspace) ([]*ThemeInstallResult, error) {
	log.Printf("Auto-detecting missing Slidev themes...")

	// Détecter les thèmes manquants
	missingThemes, err := tm.DetectMissingThemes(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to detect missing themes: %w", err)
	}

	if len(missingThemes) == 0 {
		log.Printf("No missing themes detected")
		return []*ThemeInstallResult{}, nil
	}

	log.Printf("Found %d missing themes: %v", len(missingThemes), missingThemes)

	// Installer les thèmes manquants
	return tm.InstallMultipleThemes(ctx, workspace, missingThemes)
}

// ListInstalledThemes liste les thèmes installés
func (tm *ThemeManager) ListInstalledThemes(ctx context.Context, workspace *Workspace) ([]ThemeInfo, error) {
	var themes []ThemeInfo

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
					themes = append(themes, ThemeInfo{
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

// buildInstallEnvironment construit l'environnement pour l'installation
func (tm *ThemeManager) buildInstallEnvironment() []string {
	env := os.Environ()

	// Variables pour éviter les prompts interactifs
	env = append(env, "NPM_CONFIG_YES=true")
	env = append(env, "NPM_CONFIG_AUDIT=false")
	env = append(env, "NPM_CONFIG_FUND=false")
	env = append(env, "NPM_CONFIG_UPDATE_NOTIFIER=false")
	env = append(env, "CI=true")

	return env
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
