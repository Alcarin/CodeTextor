package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileContent(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file.\n"
	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test service
	service, err := NewProjectService()
	if err != nil {
		t.Fatalf("Failed to create project service: %v", err)
	}
	defer service.Close()

	// Create a test project
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        "Test Project",
		Description: "Test description",
		Slug:        "",
		RootPath:    tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test reading file content
	content, err := service.ReadFileContent(project.ID, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, content)
	}
}

func TestReadFileContent_SecurityCheck(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a file outside project root
	outsideDir := t.TempDir()
	outsideFilePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFilePath, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Create test service
	service, err := NewProjectService()
	if err != nil {
		t.Fatalf("Failed to create project service: %v", err)
	}
	defer service.Close()

	// Create a test project
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        "Test Project",
		Description: "Test description",
		Slug:        "",
		RootPath:    tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Try to read file outside project root using path traversal
	relativePath := filepath.Join("..", "..", filepath.Base(outsideDir), "secret.txt")
	_, err = service.ReadFileContent(project.ID, relativePath)
	if err == nil {
		t.Error("Expected error when trying to read file outside project root, got nil")
	}
}

func TestReadFileContent_NonExistentFile(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create test service
	service, err := NewProjectService()
	if err != nil {
		t.Fatalf("Failed to create project service: %v", err)
	}
	defer service.Close()

	// Create a test project
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        "Test Project",
		Description: "Test description",
		Slug:        "",
		RootPath:    tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Try to read non-existent file
	_, err = service.ReadFileContent(project.ID, "nonexistent.txt")
	if err == nil {
		t.Error("Expected error when reading non-existent file, got nil")
	}
}
