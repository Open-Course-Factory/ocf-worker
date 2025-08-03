// internal/worker/theme_manager.go
package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Open-Course-Factory/ocf-worker/pkg/models"
)

// NpmPackageManager gère l'installation des paquets Npm
type NpmPackageManager struct {
	workspaceBase string
	npmCommand    string
}

// NewNpmPackageManager crée un nouveau gestionnaire de thèmes
func NewNpmPackageManager(workspaceBase string) *NpmPackageManager {
	npmCmd := "npm"

	return &NpmPackageManager{
		workspaceBase: workspaceBase,
		npmCommand:    npmCmd,
	}
}

// InstallNpmPackage installe un paquet NPM
func (tm *NpmPackageManager) InstallNpmPackage(ctx context.Context, workspace *Workspace, npmPackage string) (*models.NpmPackageInstallResult, error) {
	startTime := time.Now()
	result := &models.NpmPackageInstallResult{
		Package: npmPackage,
		Success: false,
		Logs:    []string{},
	}

	// Validation des entrées
	if npmPackage == "" {
		result.Error = "package name cannot be empty"
		return result, fmt.Errorf("package name cannot be empty")
	}

	log.Printf("Installing NPM package: %s", npmPackage)
	result.Logs = append(result.Logs, fmt.Sprintf("Starting installation of package: %s", npmPackage))

	// Créer un contexte avec timeout si pas déjà présent
	installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Préparer la commande d'installation
	cmd := tm.prepareInstallCommand(installCtx, workspace, npmPackage)

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
	result.Installed = true
	result.Success = result.Installed

	if result.Success {
		result.Logs = append(result.Logs, fmt.Sprintf("SUCCESS: Theme %s installed in %v", npmPackage, result.Duration))
		log.Printf("Theme %s installed successfully in %v", npmPackage, result.Duration)
	} else {
		result.Error = "Theme installation completed but theme not detected as installed"
		result.Logs = append(result.Logs, "WARNING: Installation completed but theme not detected")
	}

	return result, nil
}

func (tm *NpmPackageManager) NpmInstall(ctx context.Context, workspace *Workspace) error {
	cmd := exec.CommandContext(ctx, "npm", "install")
	cmd.Dir = workspace.GetPath()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install failed: %v\nOutput: %s", err, output)
	}
	return nil
}

// prepareInstallCommand prépare la commande d'installation
func (tm *NpmPackageManager) prepareInstallCommand(ctx context.Context, workspace *Workspace, npmPackage string) *exec.Cmd {
	var cmd *exec.Cmd
	if tm.npmCommand == "yarn" {
		cmd = exec.CommandContext(ctx, "yarn", "add", npmPackage)
	} else {
		cmd = exec.CommandContext(ctx, "npm", "install", npmPackage, "--save")
	}

	cmd.Dir = workspace.GetPath()
	cmd.Env = tm.buildInstallEnvironment()

	return cmd
}

// setupCommandPipes configure les pipes de manière sécurisée
func (tm *NpmPackageManager) setupCommandPipes(cmd *exec.Cmd, result *models.NpmPackageInstallResult) error {
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
func (tm *NpmPackageManager) handleInstallation(ctx context.Context, cmd *exec.Cmd, result *models.NpmPackageInstallResult) error {
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
			_ = cmd.Process.Kill()
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
func (tm *NpmPackageManager) safeOutputCapture(ctx context.Context, reader io.ReadCloser, prefix string, logChan chan<- string, errChan chan<- error) {
	defer func() {
		// Ne pas logger l'erreur car elle est normale quand le processus se termine
		reader.Close()
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
		case errChan <- err:
		case logChan <- errMsg:
		case <-ctx.Done():
		default:
			// Si tous les channels sont pleins ou fermés, logger au moins
			log.Printf("Warning: %s", errMsg)
		}
	}
}

// safeInputHandler gère stdin de manière sécurisée
func (tm *NpmPackageManager) safeInputHandler(ctx context.Context, stdin io.WriteCloser, errChan chan<- error) {
	defer func() {
		// Ne pas logger cette erreur car elle est normale
		stdin.Close()
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
func (tm *NpmPackageManager) buildInstallEnvironment() []string {
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

// AutoInstallNpmPackages installe automatiquement les thèmes manquants - VERSION ROBUSTE
func (tm *NpmPackageManager) AutoInstallNpmPackages(ctx context.Context, workspace *Workspace) ([]*models.NpmPackageInstallResult, error) {

	// Limiter le nombre de thèmes à installer simultanément
	const maxConcurrent = 3
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	// var mu sync.Mutex
	var results []*models.NpmPackageInstallResult

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Acquérir le semaphore
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		err := tm.NpmInstall(ctx, workspace)

		if err != nil {
			log.Printf("Failed to npm install: %v", err)
		}
	}()

	wg.Wait()
	return results, nil
}
