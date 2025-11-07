/*
  File: project_service_test.go
  Purpose: Integration tests for ProjectService.
  Author: CodeTextor project
  Notes: Tests the complete service layer including JSON serialization.
*/

package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"CodeTextor/backend/internal/store"
)

// setupTestService creates a test ProjectService with temporary storage.
func setupTestService(t *testing.T) (*ProjectService, func()) {
	// Create temporary directory
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_projects.db")

	// Create store
	projectStore, err := store.NewProjectStoreWithPath(dbPath)
	if err != nil {
		t.Fatalf("Failed to create project store: %v", err)
	}

	service := &ProjectService{
		store: projectStore,
	}

	cleanup := func() {
		projectStore.Close()
	}

	return service, cleanup
}

// TestListProjectsEmptyReturnsEmptyArray tests that an empty project list
// serializes to [] instead of null, which is critical for frontend compatibility.
func TestListProjectsEmptyReturnsEmptyArray(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// List projects from empty database
	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	// Verify it's not nil
	if projects == nil {
		t.Fatal("Expected non-nil slice, got nil")
	}

	// Verify it's empty
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}

	// Most importantly: verify it serializes to [] not null
	jsonData, err := json.Marshal(projects)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	jsonStr := string(jsonData)
	if jsonStr != "[]" {
		t.Errorf("Expected JSON '[]', got '%s'", jsonStr)
	}
}

// TestCreateAndListProjects tests the complete flow of creating and listing projects.
func TestCreateAndListProjects(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// Create a project
	project1, err := service.CreateProject(CreateProjectRequest{
		Name:        "Test Project 1",
		Description: "First test project",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	if project1.ID == "" {
		t.Error("Expected project ID to be generated, got empty string")
	}

	// Create another project
	_, err = service.CreateProject(CreateProjectRequest{
		Name:        "Test Project 2",
		Description: "Second test project",
	})
	if err != nil {
		t.Fatalf("Failed to create second project: %v", err)
	}

	// List all projects
	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	// Verify JSON serialization works correctly
	jsonData, err := json.Marshal(projects)
	if err != nil {
		t.Fatalf("Failed to marshal projects to JSON: %v", err)
	}

	// Verify we can unmarshal it back
	var unmarshaled []*interface{}
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if len(unmarshaled) != 2 {
		t.Errorf("Expected 2 projects after unmarshal, got %d", len(unmarshaled))
	}
}

// TestUpdateProjectConfig tests updating project configuration.
func TestUpdateProjectConfig(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// Create a project
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        "Config Test",
		Description: "Test project for config updates",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Update config
	newConfig := project.Config
	newConfig.IncludePaths = []string{"/path1", "/path2"}
	newConfig.ContinuousIndexing = true
	newConfig.ChunkSizeMin = 50

	updated, err := service.UpdateProjectConfig(project.ID, newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Verify changes
	if len(updated.Config.IncludePaths) != 2 {
		t.Errorf("Expected 2 include paths, got %d", len(updated.Config.IncludePaths))
	}
	if !updated.Config.ContinuousIndexing {
		t.Error("Expected ContinuousIndexing to be true")
	}
	if updated.Config.ChunkSizeMin != 50 {
		t.Errorf("Expected ChunkSizeMin=50, got %d", updated.Config.ChunkSizeMin)
	}
}

// TestDeleteProject tests project deletion.
func TestDeleteProject(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// Create a project
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        "To Delete",
		Description: "This project will be deleted",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Delete it
	if err := service.DeleteProject(project.ID); err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	// Verify it's gone
	_, err = service.GetProject(project.ID)
	if err == nil {
		t.Error("Expected error when getting deleted project, got nil")
	}

	// List should be empty
	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects after deletion, got %d", len(projects))
	}
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
