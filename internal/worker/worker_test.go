// internal/worker/worker_test.go
package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"ocf-worker/internal/storage"
	"ocf-worker/pkg/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspace(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir, err := os.MkdirTemp("", "ocf-worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	jobID := uuid.New()
	workspace, err := NewWorkspace(tempDir, jobID)
	require.NoError(t, err)

	t.Run("Workspace Creation", func(t *testing.T) {
		assert.Contains(t, workspace.GetPath(), jobID.String())
		assert.True(t, workspace.DirExists("."))
	})

	t.Run("File Operations", func(t *testing.T) {
		// Test écriture de fichier
		content := "Hello, OCF Worker!"
		err := workspace.WriteFile("test.txt", strings.NewReader(content))
		assert.NoError(t, err)

		// Test existence du fichier
		assert.True(t, workspace.FileExists("test.txt"))

		// Test lecture du fichier
		reader, err := workspace.ReadFile("test.txt")
		assert.NoError(t, err)

		buf := make([]byte, len(content))
		n, err := reader.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, len(content), n)
		assert.Equal(t, content, string(buf))

		// Test taille du fichier
		size, err := workspace.GetFileSize("test.txt")
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), size)
	})

	t.Run("Directory Operations", func(t *testing.T) {
		// Créer un répertoire
		err := workspace.CreateDirectory("subdir")
		assert.NoError(t, err)
		assert.True(t, workspace.DirExists("subdir"))

		// Écrire un fichier dans le sous-répertoire
		err = workspace.WriteFile("subdir/nested.txt", strings.NewReader("nested content"))
		assert.NoError(t, err)
		assert.True(t, workspace.FileExists("subdir/nested.txt"))
	})

	t.Run("File Listing", func(t *testing.T) {
		// Lister les fichiers à la racine
		files, err := workspace.ListFiles(".")
		assert.NoError(t, err)
		assert.Contains(t, files, "test.txt")

		// Lister récursivement
		allFiles, err := workspace.ListAllFiles(".")
		assert.NoError(t, err)
		assert.Contains(t, allFiles, "test.txt")
		assert.Contains(t, allFiles, "subdir/nested.txt")
	})

	t.Run("Security Validation", func(t *testing.T) {
		// Test protection contre directory traversal
		err := workspace.WriteFile("../malicious.txt", strings.NewReader("bad"))
		assert.Error(t, err)

		err = workspace.WriteFile("/absolute/path.txt", strings.NewReader("bad"))
		assert.Error(t, err)

		assert.False(t, workspace.FileExists("../malicious.txt"))
		assert.False(t, workspace.FileExists("/absolute/path.txt"))
	})

	t.Run("Workspace Info", func(t *testing.T) {
		info := workspace.GetWorkspaceInfo()
		assert.Equal(t, jobID.String(), info.JobID)
		assert.True(t, info.Exists)
		assert.Greater(t, info.SizeBytes, int64(0))
		assert.Greater(t, info.FileCount, 0)
		assert.Contains(t, info.Files, "test.txt")
	})

	t.Run("Cleanup", func(t *testing.T) {
		err := workspace.Cleanup()
		assert.NoError(t, err)
		assert.False(t, workspace.DirExists("."))
	})
}

func TestSlidevRunner(t *testing.T) {
	config := &PoolConfig{
		SlidevCommand: "echo", // Utiliser echo pour simuler Slidev
		JobTimeout:    30 * time.Second,
	}

	runner := NewSlidevRunner(config)

	t.Run("Command Detection", func(t *testing.T) {
		// Test de détection de commande
		cmd := runner.detectSlidevCommand()
		assert.NotEmpty(t, cmd)
	})

	t.Run("Environment Building", func(t *testing.T) {
		env := runner.buildEnvironment()
		assert.NotEmpty(t, env)

		// Vérifier que les variables spécifiques sont présentes
		found := false
		for _, envVar := range env {
			if strings.Contains(envVar, "NODE_ENV=production") {
				found = true
				break
			}
		}
		assert.True(t, found, "NODE_ENV=production should be in environment")
	})

	t.Run("Progress Parsing", func(t *testing.T) {
		tests := []struct {
			input    string
			expected int
		}{
			{"Building... 75%", 75},
			{"Progress: 50%", 50},
			{"[3/10] Processing", 30},
			{"5 of 8 completed", 62}, // 5/8 * 100 = 62.5, arrondi à 62
			{"No progress here", 0},
		}

		for _, test := range tests {
			result := runner.parseProgress(test.input)
			assert.Equal(t, test.expected, result, "Failed for input: %s", test.input)
		}
	})

	t.Run("Dependency Check", func(t *testing.T) {
		// Ce test nécessite un environnement avec Node.js
		// Dans un vrai test, on mockerait ces vérifications
		assert.True(t, runner.commandExists("echo"))
		assert.False(t, runner.commandExists("nonexistent-command-xyz"))
	})
}

func TestWorkerPool(t *testing.T) {
	// Mock job service pour les tests
	mockJobService := &MockJobService{}

	// Créer un vrai storage service avec un backend mock
	mockBackend := &MockStorageBackend{}
	storageService := storage.NewStorageService(mockBackend)

	config := &PoolConfig{
		WorkerCount:   2,
		PollInterval:  100 * time.Millisecond,
		JobTimeout:    5 * time.Second,
		WorkspaceBase: os.TempDir(),
	}

	pool := NewWorkerPool(mockJobService, storageService, config)

	t.Run("Pool Creation", func(t *testing.T) {
		assert.NotNil(t, pool)
		assert.Len(t, pool.workers, 2)
		assert.Equal(t, 2, config.WorkerCount)
	})

	t.Run("Pool Stats", func(t *testing.T) {
		stats := pool.GetStats()
		assert.Equal(t, 2, stats.WorkerCount)
		assert.False(t, stats.Running)
		assert.Len(t, stats.Workers, 2)
	})

	// Note: Les tests de démarrage/arrêt du pool nécessiteraient
	// des mocks plus sophistiqués pour éviter les side effects
}

// MockJobService implémente JobService pour les tests
type MockJobService struct {
	jobs map[uuid.UUID]*models.GenerationJob
}

func (m *MockJobService) CreateJob(ctx context.Context, req *models.GenerationRequest) (*models.GenerationJob, error) {
	if m.jobs == nil {
		m.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}

	job := &models.GenerationJob{
		ID:       req.JobID,
		CourseID: req.CourseID,
		Status:   models.StatusPending,
		Progress: 0,
	}

	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockJobService) GetJob(ctx context.Context, id uuid.UUID) (*models.GenerationJob, error) {
	if m.jobs == nil {
		m.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}

	if job, exists := m.jobs[id]; exists {
		return job, nil
	}

	return nil, fmt.Errorf("job not found")
}

func (m *MockJobService) ListJobs(ctx context.Context, status string, courseID *uuid.UUID) ([]*models.GenerationJob, error) {
	var result []*models.GenerationJob

	for _, job := range m.jobs {
		if status == "" || string(job.Status) == status {
			result = append(result, job)
		}
	}

	return result, nil
}

func (m *MockJobService) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus, progress int, errorMsg string) error {
	if m.jobs == nil {
		m.jobs = make(map[uuid.UUID]*models.GenerationJob)
	}

	if job, exists := m.jobs[id]; exists {
		job.Status = status
		job.Progress = progress
		if errorMsg != "" {
			job.Error = errorMsg
		}
		return nil
	}

	return fmt.Errorf("job not found")
}

func (m *MockJobService) AddJobLog(ctx context.Context, id uuid.UUID, logEntry string) error {
	// Mock implementation
	return nil
}

func (m *MockJobService) CleanupOldJobs(ctx context.Context, maxAge time.Duration) (int64, error) {
	// Mock implementation
	return 0, nil
}

// MockStorageBackend implémente l'interface storage.Storage pour les tests
type MockStorageBackend struct {
	files map[string][]byte
}

func (m *MockStorageBackend) Upload(ctx context.Context, path string, data io.Reader) error {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}

	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	m.files[path] = content
	return nil
}

func (m *MockStorageBackend) Download(ctx context.Context, path string) (io.Reader, error) {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}

	if content, exists := m.files[path]; exists {
		return strings.NewReader(string(content)), nil
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

func (m *MockStorageBackend) Exists(ctx context.Context, path string) (bool, error) {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}

	_, exists := m.files[path]
	return exists, nil
}

func (m *MockStorageBackend) Delete(ctx context.Context, path string) error {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}

	delete(m.files, path)
	return nil
}

func (m *MockStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}

	var files []string
	for path := range m.files {
		if strings.HasPrefix(path, prefix) {
			files = append(files, path)
		}
	}

	return files, nil
}

func (m *MockStorageBackend) GetURL(ctx context.Context, path string) (string, error) {
	return "http://mock-storage/" + path, nil
}

func TestIntegrationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test d'intégration simple qui teste le workflow complet
	// sans exécuter réellement Slidev

	tempDir, err := os.MkdirTemp("", "ocf-integration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	jobID := uuid.New()
	//courseID := uuid.New()

	// Créer un workspace
	workspace, err := NewWorkspace(tempDir, jobID)
	require.NoError(t, err)

	// Simuler des fichiers sources
	err = workspace.WriteFile("slides.md", strings.NewReader(`---
title: Test Slides
---

# Hello OCF Worker

This is a test presentation.
`))
	require.NoError(t, err)

	// Créer un faux répertoire dist avec des résultats
	err = workspace.CreateDirectory("dist")
	require.NoError(t, err)

	err = workspace.WriteFile("dist/index.html", strings.NewReader(`<!DOCTYPE html>
<html>
<head><title>Test Presentation</title></head>
<body><h1>Generated by OCF Worker</h1></body>
</html>`))
	require.NoError(t, err)

	// Vérifier que le workspace est correct
	info := workspace.GetWorkspaceInfo()
	assert.True(t, info.Exists)
	assert.True(t, info.DistExists)
	assert.Greater(t, info.FileCount, 0)
	assert.Greater(t, info.DistFileCount, 0)

	// Vérifier les fichiers de résultat
	distFiles, err := workspace.ListFiles("dist")
	require.NoError(t, err)
	assert.Contains(t, distFiles, "index.html")

	// Tester la validation de sortie
	runner := NewSlidevRunner(&PoolConfig{})
	err = runner.validateOutput(workspace)
	assert.NoError(t, err)

	// Cleanup
	err = workspace.Cleanup()
	assert.NoError(t, err)
}
