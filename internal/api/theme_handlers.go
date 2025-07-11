// internal/api/theme_handlers.go
package api

import (
	"context"
	"net/http"
	"ocf-worker/internal/storage"
	"ocf-worker/internal/worker"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ThemeHandlers struct {
	themeManager   *worker.ThemeManager
	storageService *storage.StorageService
}

func NewThemeHandlers(storageService *storage.StorageService, workspaceBase string) *ThemeHandlers {
	return &ThemeHandlers{
		themeManager:   worker.NewThemeManager(workspaceBase),
		storageService: storageService,
	}
}

// InstallTheme installe un thème spécifique
func (h *ThemeHandlers) InstallTheme(c *gin.Context) {
	var req struct {
		Theme string `json:"theme" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Créer un workspace temporaire pour l'installation
	tempWorkspace, err := worker.NewWorkspace("/tmp/theme-install", uuid.New())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary workspace"})
		return
	}
	defer tempWorkspace.Cleanup()

	// Créer un package.json basique s'il n'existe pas
	if !tempWorkspace.FileExists("package.json") {
		packageJSON := `{
  "name": "theme-install-temp",
  "version": "1.0.0",
  "dependencies": {}
}`
		if err := tempWorkspace.WriteFile("package.json", strings.NewReader(packageJSON)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create package.json"})
			return
		}
	}

	// Installer le thème avec timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	result, err := h.themeManager.InstallTheme(ctx, tempWorkspace, req.Theme)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Theme installation failed",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DetectThemes détecte les thèmes dans les fichiers sources d'un job
func (h *ThemeHandlers) DetectThemes(c *gin.Context) {
	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	// Créer un workspace temporaire et télécharger les sources
	tempWorkspace, err := worker.NewWorkspace("/tmp/theme-detect", jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}
	defer tempWorkspace.Cleanup()

	// Télécharger les sources du job depuis le storage
	sourceFiles, err := h.storageService.ListJobSources(c, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list job sources: " + err.Error()})
		return
	}

	if len(sourceFiles) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No source files found for this job"})
		return
	}

	// Télécharger chaque fichier source
	for _, filename := range sourceFiles {
		reader, err := h.storageService.DownloadJobSource(c, jobID, filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download source file: " + filename})
			return
		}

		if err := tempWorkspace.WriteFile(filename, reader); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write source file: " + filename})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	missingThemes, err := h.themeManager.DetectMissingThemes(ctx, tempWorkspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	installedThemes, err := h.themeManager.ListInstalledThemes(ctx, tempWorkspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":           jobID,
		"missing_themes":   missingThemes,
		"installed_themes": installedThemes,
	})
}

// InstallJobThemes installe automatiquement les thèmes manquants pour un job
func (h *ThemeHandlers) InstallJobThemes(c *gin.Context) {
	jobIDStr := c.Param("job_id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	// Créer un workspace temporaire
	tempWorkspace, err := worker.NewWorkspace("/tmp/theme-install-job", jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}
	defer tempWorkspace.Cleanup()

	// Télécharger les sources du job depuis le storage
	sourceFiles, err := h.storageService.ListJobSources(c, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list job sources: " + err.Error()})
		return
	}

	if len(sourceFiles) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No source files found for this job"})
		return
	}

	// Télécharger chaque fichier source
	for _, filename := range sourceFiles {
		reader, err := h.storageService.DownloadJobSource(c, jobID, filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download source file: " + filename})
			return
		}

		if err := tempWorkspace.WriteFile(filename, reader); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write source file: " + filename})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	results, err := h.themeManager.AutoInstallMissingThemes(ctx, tempWorkspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID,
		"results": results,
	})
}

// ListAvailableThemes liste les thèmes Slidev populaires avec leur statut d'installation
func (h *ThemeHandlers) ListAvailableThemes(c *gin.Context) {
	// Créer un workspace temporaire pour vérifier les installations
	tempWorkspace, err := worker.NewWorkspace("/tmp/theme-check", uuid.New())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary workspace"})
		return
	}
	defer tempWorkspace.Cleanup()

	// Créer un package.json basique pour la vérification
	packageJSON := `{
  "name": "theme-check-temp",
  "version": "1.0.0",
  "dependencies": {},
  "devDependencies": {}
}`
	if err := tempWorkspace.WriteFile("package.json", strings.NewReader(packageJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create package.json"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Liste des thèmes Slidev populaires/officiels
	themeList := []struct {
		name        string
		description string
		homepage    string
	}{
		{"@slidev/theme-default", "Default Slidev theme", "https://github.com/slidevjs/slidev"},
		{"@slidev/theme-seriph", "Seriph theme with elegant typography", "https://github.com/slidevjs/themes/tree/main/packages/theme-seriph"},
		{"@slidev/theme-academic", "Academic presentation theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-academic"},
		{"@slidev/theme-apple-basic", "Apple-style basic theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-apple-basic"},
		{"@slidev/theme-bricks", "Brick-style theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-bricks"},
		{"@slidev/theme-eloc", "Eloc theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-eloc"},
		{"@slidev/theme-geist", "Geist design theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-geist"},
		{"@slidev/theme-metropolis", "Metropolis theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-metropolis"},
		{"@slidev/theme-shibainu", "Shiba Inu theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-shibainu"},
		{"@slidev/theme-starter", "Starter theme template", "https://github.com/slidevjs/themes/tree/main/packages/theme-starter"},
		{"@slidev/theme-purplin", "Purple gradient theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-purplin"},
		{"@slidev/theme-penguin", "Penguin theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-penguin"},
		{"@slidev/theme-minimal", "Minimal clean theme", "https://github.com/slidevjs/themes/tree/main/packages/theme-minimal"},
	}

	var themes []worker.ThemeInfo

	// Vérifier le statut d'installation pour chaque thème
	for _, theme := range themeList {
		installed := h.themeManager.IsThemeInstalled(ctx, tempWorkspace, theme.name)

		// Obtenir la version si installé
		version := "latest"
		if installed {
			if installedThemes, err := h.themeManager.ListInstalledThemes(ctx, tempWorkspace); err == nil {
				for _, installedTheme := range installedThemes {
					if installedTheme.Name == theme.name {
						version = installedTheme.Version
						break
					}
				}
			}
		}

		themes = append(themes, worker.ThemeInfo{
			Name:        theme.name,
			Version:     version,
			Installed:   installed,
			Description: theme.description,
			Homepage:    theme.homepage,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"themes": themes,
		"count":  len(themes),
		"summary": gin.H{
			"total":     len(themes),
			"installed": countInstalledThemes(themes),
			"available": len(themes) - countInstalledThemes(themes),
		},
	})
}

// countInstalledThemes compte le nombre de thèmes installés
func countInstalledThemes(themes []worker.ThemeInfo) int {
	count := 0
	for _, theme := range themes {
		if theme.Installed {
			count++
		}
	}
	return count
}
