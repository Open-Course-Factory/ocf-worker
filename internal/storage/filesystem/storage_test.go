package filesystem

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystemStorage(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir, err := os.MkdirTemp("", "ocf-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storage, err := NewFilesystemStorage(tempDir)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Upload and Download", func(t *testing.T) {
		testData := "Hello, World!"
		testPath := "test/file.txt"

		// Upload
		err := storage.Upload(ctx, testPath, strings.NewReader(testData))
		assert.NoError(t, err)

		// Vérifier que le fichier existe
		exists, err := storage.Exists(ctx, testPath)
		assert.NoError(t, err)
		assert.True(t, exists)

		// Download
		reader, err := storage.Download(ctx, testPath)
		assert.NoError(t, err)

		// Lire le contenu
		buf := make([]byte, len(testData))
		n, err := reader.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, testData, string(buf))
	})

	t.Run("List files", func(t *testing.T) {
		// Upload plusieurs fichiers
		files := map[string]string{
			"sources/job1/slides.md":     "# Slides 1",
			"sources/job1/theme.css":     "body { color: red; }",
			"sources/job2/slides.md":     "# Slides 2",
			"results/course1/index.html": "<html></html>",
		}

		for path, content := range files {
			err := storage.Upload(ctx, path, strings.NewReader(content))
			assert.NoError(t, err)
		}

		// Lister les fichiers sources
		sourceFiles, err := storage.List(ctx, "sources/")
		assert.NoError(t, err)
		assert.Len(t, sourceFiles, 3)

		// Lister les fichiers pour job1
		job1Files, err := storage.List(ctx, "sources/job1/")
		assert.NoError(t, err)
		assert.Len(t, job1Files, 2)
	})

	t.Run("Delete file", func(t *testing.T) {
		testPath := "to-delete.txt"

		// Upload puis delete
		err := storage.Upload(ctx, testPath, strings.NewReader("delete me"))
		assert.NoError(t, err)

		exists, err := storage.Exists(ctx, testPath)
		assert.NoError(t, err)
		assert.True(t, exists)

		err = storage.Delete(ctx, testPath)
		assert.NoError(t, err)

		exists, err = storage.Exists(ctx, testPath)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Non-existent file", func(t *testing.T) {
		_, err := storage.Download(ctx, "non-existent.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")

		exists, err := storage.Exists(ctx, "non-existent.txt")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}
