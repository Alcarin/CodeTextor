/*
  File: project_store_test.go
  Purpose: Unit tests for ProjectStore functionality.
  Author: CodeTextor project
  Notes: Tests CRUD operations and project isolation.
*/

package store

import (
	"path/filepath"
	"testing"

	"CodeTextor/backend/pkg/models"
)

// setupTestStore creates a temporary ProjectStore for testing.
// Returns the store and a cleanup function.
func setupTestStore(t *testing.T) (*ProjectStore, func()) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "projects.db")

	store, err := NewProjectStoreWithPath(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

// TestCreateProject tests creating a new project.
func TestCreateProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	project := models.NewProject("test-project-1", "Test Project", "A test project")
	project.Config.IncludePaths = []string{"/test/path1", "/test/path2"}

	err := store.Create(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Verify project was created
	retrieved, err := store.Get("test-project-1")
	if err != nil {
		t.Fatalf("Failed to retrieve project: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Project not found after creation")
	}

	// Verify fields
	if retrieved.Name != "Test Project" {
		t.Errorf("Expected name 'Test Project', got '%s'", retrieved.Name)
	}
	if len(retrieved.Config.IncludePaths) != 2 {
		t.Errorf("Expected 2 include paths, got %d", len(retrieved.Config.IncludePaths))
	}
}

// TestCreateDuplicateProject tests that creating a duplicate project fails.
func TestCreateDuplicateProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	project := models.NewProject("test-project-1", "Test Project", "A test project")

	// Create first time - should succeed
	err := store.Create(project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create second time - should fail
	err = store.Create(project)
	if err == nil {
		t.Fatal("Expected error when creating duplicate project, got nil")
	}
}

// TestGetNonexistentProject tests retrieving a project that doesn't exist.
func TestGetNonexistentProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	project, err := store.Get("nonexistent-project")
	if err != nil {
		t.Fatalf("Expected nil error, got: %v", err)
	}
	if project != nil {
		t.Fatal("Expected nil project, got non-nil")
	}
}

// TestListProjects tests listing all projects.
func TestListProjects(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create multiple projects
	projects := []*models.Project{
		models.NewProject("project-1", "Project 1", "First project"),
		models.NewProject("project-2", "Project 2", "Second project"),
		models.NewProject("project-3", "Project 3", "Third project"),
	}

	for _, p := range projects {
		if err := store.Create(p); err != nil {
			t.Fatalf("Failed to create project: %v", err)
		}
	}

	// List all projects
	retrieved, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(retrieved))
	}

	// Verify projects are ordered by creation time (newest first)
	if len(retrieved) > 0 && retrieved[0].ID != "project-3" {
		t.Errorf("Expected first project to be 'project-3', got '%s'", retrieved[0].ID)
	}
}

// TestListProjectsEmpty tests that listing projects when none exist returns an empty array, not nil.
func TestListProjectsEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// List projects from empty database
	retrieved, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	// Verify it returns an empty slice, not nil
	if retrieved == nil {
		t.Error("Expected non-nil slice, got nil")
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(retrieved))
	}
}

// TestUpdateProject tests updating an existing project.
func TestUpdateProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create project
	project := models.NewProject("test-project-1", "Original Name", "Original description")
	if err := store.Create(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Update project
	project.Name = "Updated Name"
	project.Description = "Updated description"
	project.Config.ContinuousIndexing = true

	if err := store.Update(project); err != nil {
		t.Fatalf("Failed to update project: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.Get("test-project-1")
	if err != nil {
		t.Fatalf("Failed to retrieve project: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if retrieved.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", retrieved.Description)
	}
	if !retrieved.Config.ContinuousIndexing {
		t.Error("Expected ContinuousIndexing to be true")
	}
}

// TestUpdateNonexistentProject tests updating a project that doesn't exist.
func TestUpdateNonexistentProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	project := models.NewProject("nonexistent-project", "Test", "Test")
	err := store.Update(project)
	if err == nil {
		t.Fatal("Expected error when updating nonexistent project, got nil")
	}
}

// TestDeleteProject tests deleting a project.
func TestDeleteProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create project
	project := models.NewProject("test-project-1", "Test Project", "A test project")
	if err := store.Create(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Delete project
	if err := store.Delete("test-project-1"); err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	// Verify project is gone
	retrieved, err := store.Get("test-project-1")
	if err != nil {
		t.Fatalf("Error retrieving deleted project: %v", err)
	}
	if retrieved != nil {
		t.Fatal("Project should not exist after deletion")
	}
}

// TestDeleteNonexistentProject tests deleting a project that doesn't exist.
func TestDeleteNonexistentProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.Delete("nonexistent-project")
	if err == nil {
		t.Fatal("Expected error when deleting nonexistent project, got nil")
	}
}

// TestExists tests the Exists method.
func TestExists(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Check nonexistent project
	exists, err := store.Exists("nonexistent-project")
	if err != nil {
		t.Fatalf("Error checking existence: %v", err)
	}
	if exists {
		t.Error("Expected project to not exist")
	}

	// Create project
	project := models.NewProject("test-project-1", "Test Project", "A test project")
	if err := store.Create(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Check existing project
	exists, err = store.Exists("test-project-1")
	if err != nil {
		t.Fatalf("Error checking existence: %v", err)
	}
	if !exists {
		t.Error("Expected project to exist")
	}
}

// TestProjectValidation tests that invalid projects are rejected.
func TestProjectValidation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	tests := []struct {
		name        string
		project     *models.Project
		shouldError bool
	}{
		{
			name:        "Empty ID",
			project:     models.NewProject("", "Test", "Test"),
			shouldError: true,
		},
		{
			name:        "Empty Name",
			project:     models.NewProject("test-1", "", "Test"),
			shouldError: true,
		},
		{
			name: "Invalid ChunkSizeMin",
			project: func() *models.Project {
				p := models.NewProject("test-1", "Test", "Test")
				p.Config.ChunkSizeMin = 5
				return p
			}(),
			shouldError: true,
		},
		{
			name: "ChunkSizeMax < ChunkSizeMin",
			project: func() *models.Project {
				p := models.NewProject("test-1", "Test", "Test")
				p.Config.ChunkSizeMin = 500
				p.Config.ChunkSizeMax = 100
				return p
			}(),
			shouldError: true,
		},
		{
			name:        "Valid Project",
			project:     models.NewProject("test-1", "Test", "Test"),
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Create(tt.project)
			if tt.shouldError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestConfigPersistence tests that all configuration fields are persisted correctly.
func TestConfigPersistence(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create project with custom config
	project := models.NewProject("test-project-1", "Test Project", "A test project")
	project.Config.IncludePaths = []string{"/path1", "/path2", "/path3"}
	project.Config.ExcludePatterns = []string{"*.tmp", "build/"}
	project.Config.FileExtensions = []string{".go", ".ts", ".py"}
	project.Config.AutoExcludeHidden = false
	project.Config.ContinuousIndexing = true
	project.Config.ChunkSizeMin = 50
	project.Config.ChunkSizeMax = 1000
	project.Config.EmbeddingModel = "custom-model"
	project.Config.MaxResponseBytes = 200000

	if err := store.Create(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Retrieve and verify all fields
	retrieved, err := store.Get("test-project-1")
	if err != nil {
		t.Fatalf("Failed to retrieve project: %v", err)
	}

	config := retrieved.Config

	if len(config.IncludePaths) != 3 || config.IncludePaths[0] != "/path1" {
		t.Errorf("IncludePaths not persisted correctly: %v", config.IncludePaths)
	}
	if len(config.ExcludePatterns) != 2 || config.ExcludePatterns[0] != "*.tmp" {
		t.Errorf("ExcludePatterns not persisted correctly: %v", config.ExcludePatterns)
	}
	if len(config.FileExtensions) != 3 || config.FileExtensions[0] != ".go" {
		t.Errorf("FileExtensions not persisted correctly: %v", config.FileExtensions)
	}
	if config.AutoExcludeHidden != false {
		t.Error("AutoExcludeHidden not persisted correctly")
	}
	if config.ContinuousIndexing != true {
		t.Error("ContinuousIndexing not persisted correctly")
	}
	if config.ChunkSizeMin != 50 {
		t.Errorf("ChunkSizeMin not persisted correctly: %d", config.ChunkSizeMin)
	}
	if config.ChunkSizeMax != 1000 {
		t.Errorf("ChunkSizeMax not persisted correctly: %d", config.ChunkSizeMax)
	}
	if config.EmbeddingModel != "custom-model" {
		t.Errorf("EmbeddingModel not persisted correctly: %s", config.EmbeddingModel)
	}
	if config.MaxResponseBytes != 200000 {
		t.Errorf("MaxResponseBytes not persisted correctly: %d", config.MaxResponseBytes)
	}
}
