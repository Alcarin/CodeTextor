package services

import (
	"encoding/json"
	"testing"

	"CodeTextor/backend/pkg/models"
)

func setupTestService(t *testing.T) (*ProjectService, func()) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

    service, err := NewProjectService(nil)
	if err != nil {
		t.Fatalf("failed to create project service: %v", err)
	}

	cleanup := func() {
		service.Close()
	}

	return service, cleanup
}

func createProject(t *testing.T, service *ProjectService, name string) *models.Project {
	root := t.TempDir()
	project, err := service.CreateProject(CreateProjectRequest{
		Name:        name,
		Description: "test project",
		RootPath:    root,
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	return project
}

func TestListProjectsEmptyReturnsEmptyArray(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	if projects == nil {
		t.Fatal("Expected non-nil slice, got nil")
	}
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}

	serialized, err := json.Marshal(projects)
	if err != nil {
		t.Fatalf("Failed to marshal projects: %v", err)
	}
	if string(serialized) != "[]" {
		t.Errorf("Expected JSON [], got %s", string(serialized))
	}
}

func TestCreateAndListProjects(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	project1 := createProject(t, service, "Test Project 1")
	if project1.ID == "" {
		t.Error("Expected generated project ID")
	}

	createProject(t, service, "Test Project 2")

	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	serialized, err := json.Marshal(projects)
	if err != nil {
		t.Fatalf("Failed to marshal projects: %v", err)
	}
	var unmarshaled []*interface{}
	if err := json.Unmarshal(serialized, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(unmarshaled) != 2 {
		t.Errorf("Expected 2 projects after unmarshal, got %d", len(unmarshaled))
	}
}

func TestUpdateProjectConfig(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	project := createProject(t, service, "Config Project")
	newConfig := project.Config
	newConfig.IncludePaths = []string{"src", "backend"}
	newConfig.ChunkSizeMin = 50

	updated, err := service.UpdateProjectConfig(project.ID, newConfig)
	if err != nil {
		t.Fatalf("Failed to update project config: %v", err)
	}
	if len(updated.Config.IncludePaths) != 2 {
		t.Errorf("Expected 2 include paths, got %d", len(updated.Config.IncludePaths))
	}
	if updated.Config.ChunkSizeMin != 50 {
		t.Errorf("ChunkSizeMin mismatch, got %d", updated.Config.ChunkSizeMin)
	}
}

func TestDeleteProject(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	project := createProject(t, service, "ToDelete")

	if err := service.DeleteProject(project.ID); err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	if _, err := service.GetProject(project.ID); err == nil {
		t.Error("Expected error when fetching deleted project")
	}

	projects, err := service.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}
