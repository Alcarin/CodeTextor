package main

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/services"
	"context"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx            context.Context
	projectService services.ProjectServiceAPI
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize project service
	projectService, err := services.NewProjectService()
	if err != nil {
		log.Fatalf("Failed to initialize project service: %v", err)
	}
	a.projectService = projectService
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.projectService != nil {
		if err := a.projectService.Close(); err != nil {
			log.Printf("Error closing project service: %v", err)
		}
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ==================== Project Management API ====================

// CreateProject creates a new project.
// Exposed to frontend as: window.go.main.App.CreateProject
func (a *App) CreateProject(name, description, slug, rootPath string) (*models.Project, error) {
	return a.projectService.CreateProject(services.CreateProjectRequest{
		Name:        name,
		Description: description,
		Slug:        slug,
		RootPath:    rootPath,
	})
}

// GetProject retrieves a project by ID.
// Exposed to frontend as: window.go.main.App.GetProject
func (a *App) GetProject(projectID string) (*models.Project, error) {
	return a.projectService.GetProject(projectID)
}

// ListProjects returns all projects.
// Exposed to frontend as: window.go.main.App.ListProjects
func (a *App) ListProjects() ([]*models.Project, error) {
	return a.projectService.ListProjects()
}

// UpdateProject updates a project's basic information.
// Exposed to frontend as: window.go.main.App.UpdateProject
func (a *App) UpdateProject(projectID, name, description string) (*models.Project, error) {
	return a.projectService.UpdateProject(services.UpdateProjectRequest{
		ProjectID:   projectID,
		Name:        &name,
		Description: &description,
	})
}

// UpdateProjectConfig updates a project's configuration.
// Exposed to frontend as: window.go.main.App.UpdateProjectConfig
func (a *App) UpdateProjectConfig(projectID string, config models.ProjectConfig) (*models.Project, error) {
	return a.projectService.UpdateProjectConfig(projectID, config)
}

// DeleteProject deletes a project.
// Exposed to frontend as: window.go.main.App.DeleteProject
func (a *App) DeleteProject(projectID string) error {
	return a.projectService.DeleteProject(projectID)
}

// ProjectExists checks if a project exists.
// Exposed to frontend as: window.go.main.App.ProjectExists
func (a *App) ProjectExists(projectID string) (bool, error) {
	return a.projectService.ProjectExists(projectID)
}

// SetSelectedProject sets the currently selected project.
// Exposed to frontend as: window.go.main.App.SetSelectedProject
func (a *App) SetSelectedProject(projectID string) error {
	return a.projectService.SetSelectedProject(projectID)
}

// GetSelectedProject gets the currently selected project.
// Exposed to frontend as: window.go.main.App.GetSelectedProject
func (a *App) GetSelectedProject() (*models.Project, error) {
	return a.projectService.GetSelectedProject()
}

// ClearSelectedProject clears the currently selected project.
// Exposed to frontend as: window.go.main.App.ClearSelectedProject
func (a *App) ClearSelectedProject() error {
	return a.projectService.ClearSelectedProject()
}

// SetProjectIndexing enables or disables continuous indexing for a project.
// Exposed to frontend as: window.go.main.App.SetProjectIndexing
func (a *App) SetProjectIndexing(projectID string, enabled bool) error {
	return a.projectService.SetProjectIndexing(projectID, enabled)
}

// StartIndexing initiates the indexing process for a given project.
func (a *App) StartIndexing(projectID string) error {
	return a.projectService.StartIndexing(projectID)
}

// StopIndexing halts the indexing process for a given project.
func (a *App) StopIndexing(projectID string) error {
	return a.projectService.StopIndexing(projectID)
}

// GetIndexingProgress returns the current indexing progress for a given project.
func (a *App) GetIndexingProgress(projectID string) (models.IndexingProgress, error) {
	return a.projectService.GetIndexingProgress(projectID)
}

// SelectDirectory opens a dialog to select a directory.
func (a *App) SelectDirectory(prompt string, startPath string) (string, error) {
	selectedDir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            prompt,
		DefaultDirectory: startPath,
	})
	if err != nil {
		return "", err
	}
	return selectedDir, nil
}

// GetFilePreviews returns a preview of the files that will be indexed based on project config.
func (a *App) GetFilePreviews(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error) {
	return a.projectService.GetFilePreviews(projectID, config)
}

// GetGitignorePatterns returns the glob patterns derived from a project's .gitignore file.
func (a *App) GetGitignorePatterns(projectID string) ([]string, error) {
	return a.projectService.GetGitIgnorePatterns(projectID)
}
