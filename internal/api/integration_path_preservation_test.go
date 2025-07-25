package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Open-Course-Factory/ocf-worker/internal/worker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileUploadWithDirectoryStructure(t *testing.T) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "ocf-path-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	jobService, storageService := setupTestServices(t)

	// Créer un mock worker pool pour les tests
	mockWorkerPool := createMockWorkerPool(jobService, storageService)

	// Create test router
	router := SetupRouter(jobService, storageService, mockWorkerPool)

	jobID := uuid.New()

	// Test data with directory structure
	testFiles := map[string]string{
		"slides.md":                 "# Main Slides\n\nContent here",
		"assets/css/theme.css":      "body { color: blue; }",
		"assets/images/logo.png":    "fake-png-data",
		"assets/js/custom.js":       "console.log('hello');",
		"config/slidev.config.js":   "export default { theme: 'custom' }",
		"docs/README.md":            "# Documentation",
		"src/components/Header.vue": "<template><header>Test</header></template>",
	}

	t.Run("Upload files with directory structure", func(t *testing.T) {
		// Create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for filePath, content := range testFiles {
			part, err := writer.CreateFormFile("files", filePath)
			require.NoError(t, err)

			_, err = part.Write([]byte(content))
			require.NoError(t, err)
		}

		err := writer.Close()
		require.NoError(t, err)

		// Make request
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/storage/jobs/"+jobID.String()+"/sources", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify response
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(len(testFiles)), response["count"])
		assert.Contains(t, response["message"], "directory structure preserved")
	})

	t.Run("List files with tree structure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/storage/jobs/"+jobID.String()+"/sources?format=tree", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "tree", response["format"])

		tree, ok := response["tree"].(map[string]interface{})
		require.True(t, ok)

		// Verify directory structure
		assert.Contains(t, tree, "root")
		assert.Contains(t, tree, "assets/css")
		assert.Contains(t, tree, "assets/images")
		assert.Contains(t, tree, "assets/js")
		assert.Contains(t, tree, "config")
		assert.Contains(t, tree, "docs")
		assert.Contains(t, tree, "src/components")
	})

	t.Run("Download file with path", func(t *testing.T) {
		// Download a file from a subdirectory
		filePath := "assets/css/"
		fileName := "theme.css"
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/storage/jobs/"+jobID.String()+"/sources/"+fileName+"?filepath="+filePath, nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/css", w.Header().Get("Content-Type"))
		assert.Equal(t, filePath+fileName, w.Header().Get("X-File-Path"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "theme.css")
		assert.Equal(t, testFiles[filePath+fileName], w.Body.String())
	})

	t.Run("Workspace preserves directory structure", func(t *testing.T) {
		// Create a workspace and download sources
		workspace, err := worker.NewWorkspace(tempDir+"/workspaces", jobID)
		require.NoError(t, err)
		defer workspace.Cleanup()

		// Simulate downloading sources to workspace
		sourceFiles, err := storageService.ListJobSources(context.Background(), jobID)
		require.NoError(t, err)

		for _, filePath := range sourceFiles {
			reader, err := storageService.DownloadJobSource(context.Background(), jobID, filePath)
			require.NoError(t, err)

			err = workspace.WriteFile(filePath, reader)
			require.NoError(t, err)
		}

		// Verify workspace structure
		workspaceFiles, err := workspace.ListAllFiles(".")
		require.NoError(t, err)

		// Convert to set for easier comparison
		workspaceFileSet := make(map[string]bool)
		for _, file := range workspaceFiles {
			workspaceFileSet[file] = true
		}

		// Verify all original files are present with correct paths
		for originalPath := range testFiles {
			assert.True(t, workspaceFileSet[originalPath],
				"File %s should be present in workspace", originalPath)
		}

		// Verify directory structure exists
		assert.True(t, workspace.DirExists("assets"))
		assert.True(t, workspace.DirExists("assets/css"))
		assert.True(t, workspace.DirExists("assets/images"))
		assert.True(t, workspace.DirExists("assets/js"))
		assert.True(t, workspace.DirExists("config"))
		assert.True(t, workspace.DirExists("docs"))
		assert.True(t, workspace.DirExists("src"))
		assert.True(t, workspace.DirExists("src/components"))

		// Verify file contents
		reader, err := workspace.ReadFile("assets/css/theme.css")
		require.NoError(t, err)

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, testFiles["assets/css/theme.css"], string(content))
	})
}

func TestPathTraversalSecurity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ocf-security-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	router := setupTestRouter(t)
	jobID := uuid.New()

	// Test malicious paths
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\windows\\system32\\config",
		"/absolute/path/file.txt",
		"normal/../../../malicious.txt",
		"assets/../../bypass.txt",
	}

	for _, maliciousPath := range maliciousPaths {
		t.Run("Block path traversal: "+maliciousPath, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("files", maliciousPath)
			require.NoError(t, err)

			_, err = part.Write([]byte("malicious content"))
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/storage/jobs/"+jobID.String()+"/sources", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should either reject the file or sanitize the path
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusCreated)

			if w.Code == http.StatusCreated {
				// If accepted, verify the path was sanitized
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				files, ok := response["files"].([]interface{})
				require.True(t, ok)

				for _, file := range files {
					filePath := file.(string)
					assert.False(t, strings.Contains(filePath, ".."),
						"Sanitized path should not contain '..'")
					assert.False(t, strings.HasPrefix(filePath, "/"),
						"Sanitized path should not be absolute")
				}
			}
		})
	}
}
