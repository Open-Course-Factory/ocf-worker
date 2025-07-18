// internal/api/archive_handlers_test.go
package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-Course-Factory/ocf-worker/internal/storage"

	"github.com/stretchr/testify/assert"
)

func TestArchiveHandlers_filterFiles(t *testing.T) {
	// Setup: créer un ArchiveHandlers pour les tests
	mockStorageService := &storage.StorageService{} // On n'a pas besoin du storage pour filterFiles
	handler := NewArchiveHandlers(mockStorageService)

	// Fichiers de test
	testFiles := []string{
		"index.html",
		"styles.css",
		"app.js",
		"config.json",
		"images/logo.png",
		"images/background.jpg",
		"fonts/roboto.woff2",
		"docs/readme.md",
		"backup/old.html",
	}

	tests := []struct {
		name     string
		files    []string
		include  []string
		exclude  []string
		expected []string
	}{
		{
			name:     "no filters - returns all files",
			files:    testFiles,
			include:  []string{},
			exclude:  []string{},
			expected: testFiles,
		},
		{
			name:     "nil filters - returns all files",
			files:    testFiles,
			include:  nil,
			exclude:  nil,
			expected: testFiles,
		},
		{
			name:    "include CSS files only",
			files:   testFiles,
			include: []string{"*.css"},
			exclude: []string{},
			expected: []string{
				"styles.css",
			},
		},
		{
			name:    "include multiple patterns",
			files:   testFiles,
			include: []string{"*.css", "*.js"},
			exclude: []string{},
			expected: []string{
				"styles.css",
				"app.js",
			},
		},
		{
			name:    "include with directory pattern",
			files:   testFiles,
			include: []string{"images/*"},
			exclude: []string{},
			expected: []string{
				"images/logo.png",
				"images/background.jpg",
			},
		},
		{
			name:    "exclude HTML files",
			files:   testFiles,
			include: []string{},
			exclude: []string{"*.html", "*/*.html"},
			expected: []string{
				"styles.css",
				"app.js",
				"config.json",
				"images/logo.png",
				"images/background.jpg",
				"fonts/roboto.woff2",
				"docs/readme.md",
			},
		},
		{
			name:    "exclude multiple patterns",
			files:   testFiles,
			include: []string{},
			exclude: []string{"*.html", "backup/*"},
			expected: []string{
				"styles.css",
				"app.js",
				"config.json",
				"images/logo.png",
				"images/background.jpg",
				"fonts/roboto.woff2",
				"docs/readme.md",
			},
		},
		{
			name:    "combine include and exclude",
			files:   testFiles,
			include: []string{"images/*", "*.css", "*.js"},
			exclude: []string{"*.jpg", "*/*.jpg"},
			expected: []string{
				"styles.css",
				"app.js",
				"images/logo.png",
			},
		},
		{
			name:     "include pattern that matches nothing",
			files:    testFiles,
			include:  []string{"*.xyz"},
			exclude:  []string{},
			expected: []string{},
		},
		{
			name:     "exclude pattern that matches nothing",
			files:    testFiles,
			include:  []string{},
			exclude:  []string{"*.xyz"},
			expected: testFiles,
		},
		{
			name:     "empty file list",
			files:    []string{},
			include:  []string{"*.css"},
			exclude:  []string{},
			expected: []string{},
		},
		{
			name:    "complex pattern matching",
			files:   testFiles,
			include: []string{"*.*", "*/*.*"}, // Tous les fichiers avec extension
			exclude: []string{"backup/*", "*/*.md"},
			expected: []string{
				"index.html",
				"styles.css",
				"app.js",
				"config.json",
				"images/logo.png",
				"images/background.jpg",
				"fonts/roboto.woff2",
			},
		},
		{
			name:    "specific file inclusion",
			files:   testFiles,
			include: []string{"index.html", "app.js"},
			exclude: []string{},
			expected: []string{
				"index.html",
				"app.js",
			},
		},
		{
			name:    "include all but exclude specific",
			files:   testFiles,
			include: []string{"*", "*/*"},
			exclude: []string{"config.json", "docs/*"},
			expected: []string{
				"index.html",
				"styles.css",
				"app.js",
				"images/logo.png",
				"images/background.jpg",
				"fonts/roboto.woff2",
				"backup/old.html",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.filterFiles(tt.files, tt.include, tt.exclude)

			// Utiliser ElementsMatch pour ignorer l'ordre
			assert.ElementsMatch(t, tt.expected, result,
				"filterFiles() result doesn't match expected files")

			// Vérifier aussi la longueur pour être sûr
			assert.Len(t, result, len(tt.expected),
				"filterFiles() returned wrong number of files")
		})
	}
}

func TestArchiveHandlers_filterFiles_EdgeCases(t *testing.T) {
	handler := NewArchiveHandlers(&storage.StorageService{})

	t.Run("nil file list", func(t *testing.T) {
		result := handler.filterFiles(nil, []string{"*.css"}, []string{})
		assert.Empty(t, result, "filterFiles() should handle nil file list gracefully")
	})

	t.Run("invalid pattern handling", func(t *testing.T) {
		files := []string{"test.css", "test.js"}

		// filepath.Match handles most patterns gracefully, but let's test edge cases
		result := handler.filterFiles(files, []string{"["}, []string{})
		// Invalid pattern in Go's filepath.Match typically returns false for matching
		// So we expect empty result when include pattern is invalid
		assert.Empty(t, result, "filterFiles() should handle invalid include patterns")
	})

	t.Run("very long file names", func(t *testing.T) {
		longFileName := "very-" + string(make([]byte, 200)) + "-long-filename.css"
		files := []string{longFileName, "normal.css"}

		result := handler.filterFiles(files, []string{"*.css"}, []string{})
		assert.Len(t, result, 2, "filterFiles() should handle long filenames")
		assert.Contains(t, result, longFileName)
		assert.Contains(t, result, "normal.css")
	})

	t.Run("special characters in filenames", func(t *testing.T) {
		files := []string{
			"file with spaces.css",
			"file-with-dashes.css",
			"file_with_underscores.css",
			"file(with)parentheses.css",
			"file[with]brackets.css",
		}

		result := handler.filterFiles(files, []string{"*.css"}, []string{})
		assert.Len(t, result, 5, "filterFiles() should handle special characters in filenames")
		assert.ElementsMatch(t, files, result)
	})
}

func TestArchiveHandlers_filterFiles_Performance(t *testing.T) {
	handler := NewArchiveHandlers(&storage.StorageService{})

	// Test avec un grand nombre de fichiers
	t.Run("large file list performance", func(t *testing.T) {
		// Générer 10000 fichiers de test
		largeFileList := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			switch i % 3 {
			case 0:
				largeFileList[i] = fmt.Sprintf("file%d.css", i)
			case 1:
				largeFileList[i] = fmt.Sprintf("file%d.js", i)
			default:
				largeFileList[i] = fmt.Sprintf("file%d.html", i)
			}
		}

		// Test include avec pattern simple
		result := handler.filterFiles(largeFileList, []string{"*.css"}, []string{})

		// Vérifier que le résultat est correct
		expectedCount := 0
		for _, file := range largeFileList {
			if strings.HasSuffix(file, ".css") {
				expectedCount++
			}
		}

		assert.Len(t, result, expectedCount,
			"filterFiles() should correctly filter large file lists")

		// Vérifier que tous les résultats sont des fichiers CSS
		for _, file := range result {
			assert.True(t, strings.HasSuffix(file, ".css"),
				"All filtered files should match the pattern")
		}
	})
}
