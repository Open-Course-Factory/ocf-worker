// internal/validation/sanitize_filename_test.go - Tests pour la sanitisation

package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	validator := NewAPIValidator(nil)

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{
			input:    "normal.txt",
			expected: "normal.txt",
			name:     "normal filename",
		},
		{
			input:    "../../../etc/passwd",
			expected: "etc_passwd",
			name:     "path traversal attack",
		},
		{
			input:    "file:with:colons.txt",
			expected: "file_with_colons.txt",
			name:     "colons",
		},
		{
			input:    "file|with|pipes.txt",
			expected: "file_with_pipes.txt",
			name:     "pipes",
		},
		{
			input:    "file?with?questions.txt",
			expected: "file_with_questions.txt",
			name:     "question marks",
		},
		{
			input:    "file<with>brackets.txt",
			expected: "file_with_brackets.txt",
			name:     "angle brackets",
		},
		{
			input:    "file\"with\"quotes.txt",
			expected: "file_with_quotes.txt",
			name:     "quotes",
		},
		{
			input:    "file\\with\\backslashes.txt",
			expected: "file_with_backslashes.txt",
			name:     "backslashes",
		},
		{
			input:    "file/with/slashes.txt",
			expected: "file_with_slashes.txt",
			name:     "forward slashes",
		},
		{
			input:    "file*with*wildcards.txt",
			expected: "file_with_wildcards.txt",
			name:     "wildcards",
		},
		{
			input:    "///..\\\\..//file.txt",
			expected: "file.txt",
			name:     "multiple dangerous chars in sequence",
		},
		{
			input:    "....txt",
			expected: "unnamed_file.txt",
			name:     "multiple dots should be cleaned but preserve extension",
		},
		{
			input:    "______file.txt",
			expected: "file.txt",
			name:     "leading underscores should be trimmed",
		},
		{
			input:    "file.txt______",
			expected: "file.txt",
			name:     "trailing underscores should be trimmed",
		},
		{
			input:    "../../../",
			expected: "unnamed_file",
			name:     "all dangerous chars should fallback to unnamed_file",
		},
		{
			input:    "",
			expected: "unnamed_file",
			name:     "empty filename should fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestSanitizeFilenameLongName(t *testing.T) {
	validator := NewAPIValidator(nil)

	// Créer un nom très long
	longName := strings.Repeat("a", 250) + ".txt"
	result := validator.SanitizeFilename(longName)

	// Vérifier que c'est tronqué à 200 caractères max
	assert.LessOrEqual(t, len(result), 200)

	// Vérifier que l'extension est préservée
	assert.True(t, strings.HasSuffix(result, ".txt"))

	// Vérifier que la base est tronquée correctement
	expectedBase := strings.Repeat("a", 200-4) // 200 - len(".txt")
	expected := expectedBase + ".txt"
	assert.Equal(t, expected, result)
}

func TestSanitizeFilenamePreservesValidChars(t *testing.T) {
	validator := NewAPIValidator(nil)

	// Caractères valides qui doivent être préservés
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_()[]"
	filename := validChars + ".txt"

	result := validator.SanitizeFilename(filename)

	// Tous les caractères valides doivent être préservés
	assert.Equal(t, filename, result)
}

func TestSanitizeFilenameEdgeCases(t *testing.T) {
	validator := NewAPIValidator(nil)

	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{
			input:    ".hidden",
			expected: ".hidden",
			name:     "hidden file (starting with dot)",
		},
		{
			input:    "file.",
			expected: "file",
			name:     "file ending with dot",
		},
		{
			input:    "con.txt",
			expected: "con.txt",
			name:     "reserved name (handled by validation, not sanitization)",
		},
		{
			input:    "file name with spaces.txt",
			expected: "file name with spaces.txt",
			name:     "spaces should be preserved",
		},
		{
			input:    "file-with-dashes_and_underscores.txt",
			expected: "file-with-dashes_and_underscores.txt",
			name:     "dashes and underscores should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}
