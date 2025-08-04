// internal/worker/workspace.go - Updated with permission handling
package worker

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Workspace représente un espace de travail isolé pour un job
type Workspace struct {
	jobID    uuid.UUID
	basePath string
	path     string
	distPath string
}

// NewWorkspace crée un nouveau workspace pour un job avec gestion des permissions
func NewWorkspace(basePath string, jobID uuid.UUID) (*Workspace, error) {

	// S'assurer que le répertoire de base existe avec les bonnes permissions
	if err := ensureBaseDirectory(basePath); err != nil {
		return nil, fmt.Errorf("failed to ensure base directory: %w", err)
	}

	// Créer un répertoire unique pour ce job
	workspacePath := filepath.Join(basePath, jobID.String())

	// Créer le répertoire s'il n'existe pas
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Créer le répertoire dist pour les résultats
	distPath := filepath.Join(workspacePath, "dist")

	workspace := &Workspace{
		jobID:    jobID,
		basePath: basePath,
		path:     workspacePath,
		distPath: distPath,
	}

	log.Printf("Created workspace for job %s at %s", jobID, workspacePath)
	return workspace, nil
}

// ensureBaseDirectory s'assure que le répertoire de base existe avec les bonnes permissions
func ensureBaseDirectory(basePath string) error {
	// Vérifier si le répertoire existe déjà
	if info, err := os.Stat(basePath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("base path %s exists but is not a directory", basePath)
		}
		// Le répertoire existe, vérifier les permissions
		if info.Mode().Perm()&0200 == 0 {
			return fmt.Errorf("base path %s is not writable", basePath)
		}
		return nil
	}

	// Le répertoire n'existe pas, le créer
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", basePath, err)
	}

	// Tenter de créer un fichier de test pour vérifier les permissions
	testFile := filepath.Join(basePath, ".permission_test")
	if f, err := os.Create(testFile); err != nil {
		return fmt.Errorf("base directory %s is not writable: %w", basePath, err)
	} else {
		f.Close()
		os.Remove(testFile) // Nettoyer le fichier de test
	}

	log.Printf("Base directory %s created and verified", basePath)
	return nil
}

// GetWorkspaceStats retourne des statistiques globales sur les workspaces
func (wm *WorkspaceManager) GetWorkspaceStats() (WorkspaceStats, error) {
	workspaces, err := wm.ListWorkspaces()
	if err != nil {
		return WorkspaceStats{}, err
	}

	stats := WorkspaceStats{
		TotalWorkspaces: len(workspaces),
	}

	for _, ws := range workspaces {
		stats.TotalSizeBytes += ws.SizeBytes
		stats.TotalFileCount += ws.FileCount

		if ws.DistExists {
			stats.WorkspacesWithDist++
		}
	}

	return stats, nil
}

// WorkspaceStats contient des statistiques sur les workspaces
type WorkspaceStats struct {
	TotalWorkspaces    int   `json:"total_workspaces"`
	WorkspacesWithDist int   `json:"workspaces_with_dist"`
	TotalSizeBytes     int64 `json:"total_size_bytes"`
	TotalFileCount     int   `json:"total_file_count"`
}

func (w *Workspace) GetPath() string {
	return w.path
}

// GetDistPath retourne le chemin du répertoire de sortie
func (w *Workspace) GetDistPath() string {
	return "dist" // Chemin relatif au workspace
}

// GetAbsDistPath retourne le chemin absolu du répertoire de sortie
func (w *Workspace) GetAbsDistPath() string {
	return w.distPath
}

// WriteFile écrit un fichier dans le workspace avec gestion d'erreurs améliorée
func (w *Workspace) WriteFile(filename string, reader io.Reader) error {
	// Sécurité: éviter les chemins qui remontent dans l'arborescence
	if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") {
		return fmt.Errorf("invalid filename: %s", filename)
	}

	filePath := filepath.Join(w.path, filename)

	// Créer les répertoires parents si nécessaire
	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories for %s: %w", filePath, err)
	}

	// Créer le fichier
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Copier le contenu
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write file content to %s: %w", filePath, err)
	}

	return nil
}

// ReadFile lit un fichier depuis le workspace
func (w *Workspace) ReadFile(filename string) (io.Reader, error) {
	// Sécurité: éviter les chemins qui remontent dans l'arborescence
	if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") {
		return nil, fmt.Errorf("invalid filename: %s", filename)
	}

	filePath := filepath.Join(w.path, filename)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return file, nil
}

// FileExists vérifie si un fichier existe dans le workspace
func (w *Workspace) FileExists(filename string) bool {
	if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") {
		return false
	}

	filePath := filepath.Join(w.path, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// DirExists vérifie si un répertoire existe dans le workspace
func (w *Workspace) DirExists(dirname string) bool {
	if strings.Contains(dirname, "..") || strings.HasPrefix(dirname, "/") {
		return false
	}

	dirPath := filepath.Join(w.path, dirname)
	info, err := os.Stat(dirPath)
	return err == nil && info.IsDir()
}

// GetFileSize retourne la taille d'un fichier
func (w *Workspace) GetFileSize(filename string) (int64, error) {
	if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") {
		return 0, fmt.Errorf("invalid filename: %s", filename)
	}

	filePath := filepath.Join(w.path, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

// ListFiles liste les fichiers dans un répertoire du workspace
func (w *Workspace) ListFiles(dirname string) ([]string, error) {
	if strings.Contains(dirname, "..") || strings.HasPrefix(dirname, "/") {
		return nil, fmt.Errorf("invalid directory name: %s", dirname)
	}

	dirPath := filepath.Join(w.path, dirname)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ListAllFiles liste récursivement tous les fichiers dans un répertoire
func (w *Workspace) ListAllFiles(dirname string) ([]string, error) {
	if strings.Contains(dirname, "..") || strings.HasPrefix(dirname, "/") {
		return nil, fmt.Errorf("invalid directory name: %s", dirname)
	}

	dirPath := filepath.Join(w.path, dirname)
	var files []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Obtenir le chemin relatif au répertoire de base
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	return files, nil
}

// CreateDirectory crée un répertoire dans le workspace
func (w *Workspace) CreateDirectory(dirname string) error {
	if strings.Contains(dirname, "..") || strings.HasPrefix(dirname, "/") {
		return fmt.Errorf("invalid directory name: %s", dirname)
	}

	dirPath := filepath.Join(w.path, dirname)
	return os.MkdirAll(dirPath, 0755)
}

// CopyFile copie un fichier à l'intérieur du workspace
func (w *Workspace) CopyFile(src, dst string) error {
	// Validation des chemins
	if strings.Contains(src, "..") || strings.HasPrefix(src, "/") ||
		strings.Contains(dst, "..") || strings.HasPrefix(dst, "/") {
		return fmt.Errorf("invalid file paths: %s -> %s", src, dst)
	}

	srcPath := filepath.Join(w.path, src)
	dstPath := filepath.Join(w.path, dst)

	// Ouvrir le fichier source
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Créer les répertoires parents pour la destination
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Créer le fichier de destination
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copier le contenu
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// GetWorkspaceInfo retourne des informations sur le workspace
func (w *Workspace) GetWorkspaceInfo() WorkspaceInfo {
	info := WorkspaceInfo{
		JobID:    w.jobID.String(),
		Path:     w.path,
		DistPath: w.distPath,
		Exists:   w.DirExists("."),
	}

	// Calculer la taille utilisée
	if size, err := w.calculateSize("."); err == nil {
		info.SizeBytes = size
	}

	// Compter les fichiers
	if files, err := w.ListAllFiles("."); err == nil {
		info.FileCount = len(files)
		info.Files = files
	}

	// Vérifier si dist existe
	info.DistExists = w.DirExists("dist")
	if info.DistExists {
		if distFiles, err := w.ListAllFiles("dist"); err == nil {
			info.DistFileCount = len(distFiles)
			info.DistFiles = distFiles
		}
	}

	return info
}

// calculateSize calcule la taille totale d'un répertoire
func (w *Workspace) calculateSize(dirname string) (int64, error) {
	dirPath := filepath.Join(w.path, dirname)
	var size int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// Cleanup supprime le workspace et tous ses fichiers avec vérifications de sécurité
func (w *Workspace) Cleanup() error {
	if w.path == "" || w.path == "/" {
		return fmt.Errorf("invalid workspace path for cleanup: %s", w.path)
	}

	// Vérification de sécurité: le chemin doit contenir l'ID du job
	if !strings.Contains(w.path, w.jobID.String()) {
		return fmt.Errorf("workspace path doesn't contain job ID, refusing cleanup: %s", w.path)
	}

	log.Printf("Cleaning up workspace for job %s at %s", w.jobID, w.path)

	if err := os.RemoveAll(w.path); err != nil {
		return fmt.Errorf("failed to cleanup workspace %s: %w", w.path, err)
	}

	return nil
}

// WorkspaceInfo contient des informations sur un workspace
type WorkspaceInfo struct {
	JobID         string   `json:"job_id"`
	Path          string   `json:"path"`
	DistPath      string   `json:"dist_path"`
	Exists        bool     `json:"exists"`
	SizeBytes     int64    `json:"size_bytes"`
	FileCount     int      `json:"file_count"`
	Files         []string `json:"files,omitempty"`
	DistExists    bool     `json:"dist_exists"`
	DistFileCount int      `json:"dist_file_count"`
	DistFiles     []string `json:"dist_files,omitempty"`
}

// WorkspaceManager gère les workspaces globalement
type WorkspaceManager struct {
	basePath string
}

// NewWorkspaceManager crée un nouveau gestionnaire de workspaces
func NewWorkspaceManager(basePath string) (*WorkspaceManager, error) {
	// S'assurer que le répertoire de base est accessible
	if err := ensureBaseDirectory(basePath); err != nil {
		return nil, fmt.Errorf("failed to initialize workspace manager: %w", err)
	}

	return &WorkspaceManager{
		basePath: basePath,
	}, nil
}

// ListWorkspaces liste tous les workspaces existants
func (wm *WorkspaceManager) ListWorkspaces() ([]WorkspaceInfo, error) {
	entries, err := os.ReadDir(wm.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkspaceInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var workspaces []WorkspaceInfo

	for _, entry := range entries {
		if entry.IsDir() {
			// Vérifier si c'est un UUID valide (workspace de job)
			if jobID, err := uuid.Parse(entry.Name()); err == nil {
				workspace := &Workspace{
					jobID:    jobID,
					basePath: wm.basePath,
					path:     filepath.Join(wm.basePath, entry.Name()),
					distPath: filepath.Join(wm.basePath, entry.Name(), "dist"),
				}

				info := workspace.GetWorkspaceInfo()
				workspaces = append(workspaces, info)
			}
		}
	}

	return workspaces, nil
}

// CleanupOldWorkspaces supprime les workspaces plus anciens qu'une durée donnée
func (wm *WorkspaceManager) CleanupOldWorkspaces(maxAge int64) (int, error) {
	entries, err := os.ReadDir(wm.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read workspace directory: %w", err)
	}

	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			// Vérifier si c'est un UUID valide
			if jobID, err := uuid.Parse(entry.Name()); err == nil {
				workspacePath := filepath.Join(wm.basePath, entry.Name())

				// Vérifier l'âge du workspace
				if info, err := entry.Info(); err == nil {
					if info.ModTime().Unix() < maxAge {
						workspace := &Workspace{
							jobID: jobID,
							path:  workspacePath,
						}

						if err := workspace.Cleanup(); err == nil {
							cleaned++
						}
					}
				}
			}
		}
	}

	return cleaned, nil
}

// Get
