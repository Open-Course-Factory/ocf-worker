// internal/worker/theme_manager_robustness_test.go - Tests de robustesse

package worker

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestThemeManagerConcurrency teste la gestion de la concurrence
func TestThemeManagerConcurrency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "theme-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	// Créer un workspace de test
	workspace, err := NewWorkspace(tempDir, uuid.New())
	require.NoError(t, err)
	defer workspace.Cleanup()

	// Créer un package.json basique
	packageJSON := `{"name": "test", "version": "1.0.0", "dependencies": {}}`
	err = workspace.WriteFile("package.json", strings.NewReader(packageJSON))
	require.NoError(t, err)

	// Test d'installation concurrente
	const numGoroutines = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []*ThemeInstallResult
	var errors []error

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	themes := []string{
		"@slidev/theme-default",
		"@slidev/theme-minimal",
		"@slidev/theme-seriph",
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			theme := themes[id%len(themes)]
			result, err := themeManager.InstallTheme(ctx, workspace, theme)

			mu.Lock()
			results = append(results, result)
			if err != nil {
				errors = append(errors, err)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Vérifier qu'on a tous les résultats
	assert.Len(t, results, numGoroutines)

	// Vérifier qu'il n'y a pas de race conditions
	for _, result := range results {
		assert.NotEmpty(t, result.Theme)
		assert.NotEmpty(t, result.Logs)
		// Les erreurs sont acceptables (npm peut échouer), mais pas de paniques
	}
}

// TestThemeManagerTimeout teste la gestion des timeouts
func TestThemeManagerTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "theme-timeout-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	workspace, err := NewWorkspace(tempDir, uuid.New())
	require.NoError(t, err)
	defer workspace.Cleanup()

	// Créer un package.json
	packageJSON := `{"name": "test", "version": "1.0.0"}`
	err = workspace.WriteFile("package.json", strings.NewReader(packageJSON))
	require.NoError(t, err)

	// Context avec timeout très court pour forcer un timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result, err := themeManager.InstallTheme(ctx, workspace, "@slidev/theme-default")

	// Le timeout devrait être géré proprement
	assert.NotNil(t, result)
	assert.False(t, result.Success)

	// Le processus peut être tué (signal: killed) ou timeout (context deadline exceeded)
	// Les deux sont des comportements acceptables pour un timeout
	assert.True(t,
		strings.Contains(result.Error, "context deadline exceeded") ||
			strings.Contains(result.Error, "signal: killed") ||
			strings.Contains(result.Error, "context canceled"),
		"Error should indicate timeout or process termination, got: %s", result.Error)

	// Pas de panic, et on a une erreur
	assert.Error(t, err)

	// Vérifier que les logs contiennent des informations utiles
	assert.NotEmpty(t, result.Logs)
}

// TestThemeManagerReasonableTimeout teste avec un timeout plus réaliste
func TestThemeManagerReasonableTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "theme-reasonable-timeout-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	workspace, err := NewWorkspace(tempDir, uuid.New())
	require.NoError(t, err)
	defer workspace.Cleanup()

	// Créer un package.json
	packageJSON := `{"name": "test", "version": "1.0.0"}`
	err = workspace.WriteFile("package.json", strings.NewReader(packageJSON))
	require.NoError(t, err)

	// Context avec timeout de 5 secondes (plus réaliste)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := themeManager.InstallTheme(ctx, workspace, "@slidev/theme-nonexistent-xyz")
	duration := time.Since(start)

	// Vérifier que ça s'arrête dans les temps
	assert.True(t, duration < 10*time.Second, "Installation should timeout within reasonable time")

	// Le résultat doit être cohérent
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Logs)

	// Error attendue (thème inexistant ou timeout)
	assert.Error(t, err)
}

// TestThemeManagerErrorHandling teste la gestion d'erreurs
func TestThemeManagerErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "theme-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	workspace, err := NewWorkspace(tempDir, uuid.New())
	require.NoError(t, err)
	defer workspace.Cleanup()

	ctx := context.Background()

	// Test avec thème vide
	result, err := themeManager.InstallTheme(ctx, workspace, "")
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "cannot be empty")

	// Test avec workspace sans package.json
	result, err = themeManager.InstallTheme(ctx, workspace, "nonexistent-theme-xyz")
	assert.NotNil(t, result)
	// Error acceptable, mais pas de panic
}

// TestAutoInstallRobustness teste la robustesse de l'auto-installation
func TestAutoInstallRobustness(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "auto-install-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	workspace, err := NewWorkspace(tempDir, uuid.New())
	require.NoError(t, err)
	defer workspace.Cleanup()

	// Créer un fichier slides.md avec plusieurs thèmes
	slidesContent := `---
theme: nonexistent-theme-1
---

# Test

Content with @slidev/theme-nonexistent-2 reference
`
	err = workspace.WriteFile("slides.md", strings.NewReader(slidesContent))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test d'auto-installation
	results, err := themeManager.AutoInstallMissingThemes(ctx, workspace)

	// Pas d'erreur fatale même si les thèmes n'existent pas
	assert.NoError(t, err)
	assert.NotNil(t, results)

	// Vérifier que tous les résultats ont des logs
	for _, result := range results {
		assert.NotEmpty(t, result.Theme)
		assert.NotEmpty(t, result.Logs)
	}
}

// BenchmarkThemeInstallation benchmark l'installation de thèmes
func BenchmarkThemeInstallation(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "theme-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	themeManager := NewThemeManager(tempDir)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		workspace, err := NewWorkspace(tempDir, uuid.New())
		require.NoError(b, err)

		packageJSON := `{"name": "test", "version": "1.0.0"}`
		err = workspace.WriteFile("package.json", strings.NewReader(packageJSON))
		require.NoError(b, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		// Installation (peut échouer, on mesure juste les performances)
		_, _ = themeManager.InstallTheme(ctx, workspace, "@slidev/theme-default")

		cancel()
		workspace.Cleanup()
	}
}
