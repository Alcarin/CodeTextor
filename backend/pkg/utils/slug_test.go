package utils

import "testing"

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "My Project",
			expected: "my-project",
		},
		{
			name:     "with special characters",
			input:    "My Awesome Project!",
			expected: "my-awesome-project",
		},
		{
			name:     "with numbers and dot",
			input:    "Project 2.0",
			expected: "project-20",
		},
		{
			name:     "with underscores",
			input:    "Code_Textor",
			expected: "code-textor",
		},
		{
			name:     "multiple consecutive spaces",
			input:    "My    Project",
			expected: "my-project",
		},
		{
			name:     "multiple consecutive underscores",
			input:    "Test___Project",
			expected: "test-project",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  My Project  ",
			expected: "my-project",
		},
		{
			name:     "leading and trailing hyphens",
			input:    "-My-Project-",
			expected: "my-project",
		},
		{
			name:     "only special characters",
			input:    "!!!",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "mixed case",
			input:    "MixedCaseProject",
			expected: "mixedcaseproject",
		},
		{
			name:     "unicode characters",
			input:    "Проект Test",
			expected: "test",
		},
		{
			name:     "already valid slug",
			input:    "my-valid-slug",
			expected: "my-valid-slug",
		},
		{
			name:     "complex real-world example",
			input:    "My Awesome Project (v2.0) - Production!",
			expected: "my-awesome-project-v20-production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
