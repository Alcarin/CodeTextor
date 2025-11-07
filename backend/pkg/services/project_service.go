/*
  File: project_service.go
  Purpose: Service layer for project management operations exposed to Wails frontend.
  Author: CodeTextor project
  Notes: This service acts as the bridge between the UI and the storage layer.
*/

package services

import (
	"fmt"
	"time"

	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/models"

	"github.com/google/uuid"
)

// ProjectService handles all project-related business logic.
// It provides methods that are exposed to the Wails frontend.
type ProjectService struct {
	store *store.ProjectStore
}

// NewProjectService creates a new ProjectService instance.
// Returns an error if the project store cannot be initialized.
func NewProjectService() (*ProjectService, error) {
	projectStore, err := store.NewProjectStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project store: %w", err)
	}

	return &ProjectService{
		store: projectStore,
	}, nil
}

// CreateProjectRequest represents the data needed to create a new project.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateProject creates a new project with a generated ID.
// Parameters:
//   - req: CreateProjectRequest containing project name and description
//
// Returns the created project with default configuration.
func (s *ProjectService) CreateProject(req CreateProjectRequest) (*models.Project, error) {
	// Generate unique project ID
	projectID := "project-" + uuid.New().String()

	// Create project with default config
	project := models.NewProject(projectID, req.Name, req.Description)

	// Save to store
	if err := s.store.Create(project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
}

// GetProject retrieves a project by its ID.
// Returns nil if the project doesn't exist.
func (s *ProjectService) GetProject(projectID string) (*models.Project, error) {
	project, err := s.store.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	return project, nil
}

// ListProjects returns all projects ordered by creation time (newest first).
func (s *ProjectService) ListProjects() ([]*models.Project, error) {
	projects, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// UpdateProjectRequest represents the data that can be updated in a project.
type UpdateProjectRequest struct {
	ProjectID   string                `json:"projectId"`
	Name        *string               `json:"name,omitempty"`
	Description *string               `json:"description,omitempty"`
	Config      *models.ProjectConfig `json:"config,omitempty"`
}

// UpdateProject updates an existing project's metadata or configuration.
// Only non-nil fields in the request are updated.
func (s *ProjectService) UpdateProject(req UpdateProjectRequest) (*models.Project, error) {
	// Get existing project
	project, err := s.store.Get(req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", req.ProjectID)
	}

	// Update fields if provided
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.Config != nil {
		project.Config = *req.Config
	}

	// Update timestamp
	project.UpdatedAt = time.Now()

	// Save to store
	if err := s.store.Update(project); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return project, nil
}

// DeleteProject removes a project from the database.
// Note: This does NOT delete the project's index database file.
// The index database must be cleaned up separately if needed.
func (s *ProjectService) DeleteProject(projectID string) error {
	if err := s.store.Delete(projectID); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// ProjectExists checks if a project with the given ID exists.
func (s *ProjectService) ProjectExists(projectID string) (bool, error) {
	exists, err := s.store.Exists(projectID)
	if err != nil {
		return false, fmt.Errorf("failed to check project existence: %w", err)
	}
	return exists, nil
}

// UpdateProjectConfig updates only the configuration of a project.
// This is a convenience method for updating indexing settings.
func (s *ProjectService) UpdateProjectConfig(projectID string, config models.ProjectConfig) (*models.Project, error) {
	return s.UpdateProject(UpdateProjectRequest{
		ProjectID: projectID,
		Config:    &config,
	})
}

// SetSelectedProject marks a project as the currently selected one.
// Only one project can be selected at a time.
func (s *ProjectService) SetSelectedProject(projectID string) error {
	if err := s.store.SetSelected(projectID); err != nil {
		return fmt.Errorf("failed to set selected project: %w", err)
	}
	return nil
}

// GetSelectedProject returns the currently selected project.
// Returns nil if no project is selected (e.g., no projects exist).
// Automatically selects the oldest project if none is selected.
func (s *ProjectService) GetSelectedProject() (*models.Project, error) {
	project, err := s.store.GetSelected()
	if err != nil {
		return nil, fmt.Errorf("failed to get selected project: %w", err)
	}
	return project, nil
}

// ClearSelectedProject clears the current project selection.
func (s *ProjectService) ClearSelectedProject() error {
	if err := s.store.ClearSelection(); err != nil {
		return fmt.Errorf("failed to clear project selection: %w", err)
	}
	return nil
}

// Close closes the service and releases resources.
// Should be called when the service is no longer needed.
func (s *ProjectService) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}
