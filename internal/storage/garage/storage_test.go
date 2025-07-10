// internal/storage/garage/storage_test.go
package garage

import (
	"context"
	"os"
	"strings"
	"testing"

	"ocf-worker/pkg/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestConfig() *storage.StorageConfig {
	return &storage.StorageConfig{
		Type:      "garage",
		Endpoint:  getEnvOrDefault("TEST_GARAGE_ENDPOINT", "http://localhost:9000"),
		AccessKey: getEnvOrDefault("TEST_GARAGE_ACCESS_KEY", "minioadmin"),
		SecretKey: getEnvOrDefault("TEST_GARAGE_SECRET_KEY", "minioadmin"),
		Bucket:    getEnvOrDefault("TEST_GARAGE_BUCKET", "ocf-test"),
		Region:    getEnvOrDefault("TEST_GARAGE_REGION", "us-east-1"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func TestGarageStorageIntegration(t *testing.T) {

	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration tests")
	}

	cfg := getTestConfig()

	storage, err := NewGarageStorage(cfg)
	if err != nil {
		t.Skipf("Cannot connect to test Garage/MinIO server: %v", err)
	}

	ctx := context.Background()

	// Nettoyer le bucket de test
	t.Cleanup(func() {
		// Lister et supprimer tous les objets de test
		objects, _ := storage.List(ctx, "test/")
		for _, obj := range objects {
			storage.Delete(ctx, obj)
		}
	})

	t.Run("Upload and Download", func(t *testing.T) {
		testData := "Hello, Garage Storage!"
		testPath := "test/hello.txt"

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
		require.NotNil(t, reader)

		// Lire le contenu
		buf := make([]byte, len(testData))
		n, err := reader.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, testData, string(buf))
	})

	t.Run("List files with prefix", func(t *testing.T) {
		// Upload plusieurs fichiers
		files := map[string]string{
			"test/sources/job1/slides.md":     "# Test Slides 1",
			"test/sources/job1/theme.css":     "body { color: blue; }",
			"test/sources/job2/slides.md":     "# Test Slides 2",
			"test/results/course1/index.html": "<html><body>Test</body></html>",
		}

		for path, content := range files {
			err := storage.Upload(ctx, path, strings.NewReader(content))
			assert.NoError(t, err)
		}

		// Lister les fichiers sources
		sourceFiles, err := storage.List(ctx, "test/sources/")
		assert.NoError(t, err)
		assert.Len(t, sourceFiles, 3)

		// Lister les fichiers pour job1
		job1Files, err := storage.List(ctx, "test/sources/job1/")
		assert.NoError(t, err)
		assert.Len(t, job1Files, 2)

		// Vérifier que les chemins sont corrects
		for _, file := range job1Files {
			assert.True(t, strings.HasPrefix(file, "test/sources/job1/"))
		}
	})

	t.Run("Delete file", func(t *testing.T) {
		testPath := "test/to-delete.txt"

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

	t.Run("Get presigned URL", func(t *testing.T) {
		testPath := "test/url-test.txt"
		testData := "URL test content"

		// Upload un fichier
		err := storage.Upload(ctx, testPath, strings.NewReader(testData))
		assert.NoError(t, err)

		// Obtenir l'URL
		url, err := storage.GetURL(ctx, testPath)
		assert.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, cfg.Bucket)
		assert.Contains(t, url, "url-test.txt")
	})

	t.Run("Non-existent file operations", func(t *testing.T) {
		// Download d'un fichier inexistant
		_, err := storage.Download(ctx, "test/non-existent.txt")
		assert.Error(t, err)

		// Exists pour un fichier inexistant
		exists, err := storage.Exists(ctx, "test/non-existent.txt")
		assert.NoError(t, err)
		assert.False(t, exists)

		// Delete d'un fichier inexistant (devrait réussir)
		err = storage.Delete(ctx, "test/non-existent.txt")
		assert.NoError(t, err)
	})

	t.Run("Content type detection", func(t *testing.T) {
		// Test que les content-types sont correctement définis
		tests := []struct {
			filename   string
			expectedCT string
		}{
			{"test.md", "text/markdown"},
			{"test.css", "text/css"},
			{"test.js", "application/javascript"},
			{"test.json", "application/json"},
			{"test.html", "text/html"},
			{"test.png", "image/png"},
			{"test.jpg", "image/jpeg"},
			{"test.svg", "image/svg+xml"},
			{"test.unknown", "application/octet-stream"},
		}

		for _, tt := range tests {
			actualCT := getContentType(tt.filename)
			assert.Equal(t, tt.expectedCT, actualCT, "Content type for %s", tt.filename)
		}
	})
}

func TestGarageStorageConfig(t *testing.T) {
	t.Run("Missing required config", func(t *testing.T) {
		tests := []struct {
			name   string
			config *storage.StorageConfig
		}{
			{
				name: "missing endpoint",
				config: &storage.StorageConfig{
					Type:      "garage",
					AccessKey: "test",
					SecretKey: "test",
					Bucket:    "test",
				},
			},
			{
				name: "missing access key",
				config: &storage.StorageConfig{
					Type:      "garage",
					Endpoint:  "http://localhost:9000",
					SecretKey: "test",
					Bucket:    "test",
				},
			},
			{
				name: "missing secret key",
				config: &storage.StorageConfig{
					Type:      "garage",
					Endpoint:  "http://localhost:9000",
					AccessKey: "test",
					Bucket:    "test",
				},
			},
			{
				name: "missing bucket",
				config: &storage.StorageConfig{
					Type:      "garage",
					Endpoint:  "http://localhost:9000",
					AccessKey: "test",
					SecretKey: "test",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewGarageStorage(tt.config)
				assert.Error(t, err)
			})
		}
	})

	t.Run("Valid config", func(t *testing.T) {
		cfg := &storage.StorageConfig{
			Type:      "garage",
			Endpoint:  "http://localhost:9000",
			AccessKey: "test",
			SecretKey: "test",
			Bucket:    "test",
			Region:    "us-east-1",
		}

		// Cette fonction devrait au moins créer le client sans erreur
		// même si la connexion échoue
		storage, err := NewGarageStorage(cfg)

		// Si on n'arrive pas à se connecter au serveur, c'est normal en test unitaire
		if err != nil && strings.Contains(err.Error(), "failed to ensure bucket exists") {
			t.Skip("Cannot connect to test server, but config validation passed")
		}

		assert.NotNil(t, storage)
	})
}
