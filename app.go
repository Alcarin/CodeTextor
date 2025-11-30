package main

import (
	"CodeTextor/backend/pkg/mcp"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/services"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx            context.Context
	projectService services.ProjectServiceAPI
	mcpManager     *mcp.Manager
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
	projectService, err := services.NewProjectService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize project service: %v", err)
	}
	a.projectService = projectService

	mcpManager, err := mcp.NewManager(projectService, func(event string, data interface{}) {
		runtime.EventsEmit(ctx, event, data)
	})
	if err != nil {
		log.Fatalf("Failed to initialize MCP manager: %v", err)
	}
	a.mcpManager = mcpManager

	cfg := mcpManager.GetConfig()
	if cfg.AutoStart {
		if err := mcpManager.Start(a.ctx); err != nil {
			log.Printf("Failed to auto-start MCP server: %v", err)
		}
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.projectService != nil {
		if err := a.projectService.Close(); err != nil {
			log.Printf("Error closing project service: %v", err)
		}
	}
	if a.mcpManager != nil {
		if err := a.mcpManager.Close(); err != nil {
			log.Printf("Error closing MCP manager: %v", err)
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

// ResetProjectIndex removes indexed data for a project without restarting indexing.
func (a *App) ResetProjectIndex(projectID string) error {
	return a.projectService.ResetProjectIndex(projectID)
}

// ReindexProject clears prior index data and starts a fresh indexing run.
func (a *App) ReindexProject(projectID string) error {
	return a.projectService.ReindexProject(projectID)
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

// SelectFile opens a dialog to select a single file.
func (a *App) SelectFile(prompt string, startPath string, pattern string) (string, error) {
	options := runtime.OpenDialogOptions{
		Title: prompt,
	}

	if startPath != "" {
		if info, err := os.Stat(startPath); err == nil && info.IsDir() {
			options.DefaultDirectory = startPath
		} else {
			options.DefaultDirectory = filepath.Dir(startPath)
		}
	}

	if pattern != "" {
		options.Filters = []runtime.FileFilter{
			{
				DisplayName: "Allowed files",
				Pattern:     pattern,
			},
		}
	}

	selectedFile, err := runtime.OpenFileDialog(a.ctx, options)
	if err != nil {
		return "", err
	}
	return selectedFile, nil
}

// GetFilePreviews returns a preview of the files that will be indexed based on project config.
func (a *App) GetFilePreviews(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error) {
	return a.projectService.GetFilePreviews(projectID, config)
}

// GetFileOutline fetches the persisted outline tree for a file.
func (a *App) GetFileOutline(projectID, path string) ([]*models.OutlineNode, error) {
	return a.projectService.GetFileOutline(projectID, path)
}

// GetOutlineTimestamps fetches update timestamps for all outlines in a project.
func (a *App) GetOutlineTimestamps(projectID string) (map[string]int64, error) {
	return a.projectService.GetOutlineTimestamps(projectID)
}

// GetFileChunks retrieves all semantic chunks for a given file from the database.
func (a *App) GetFileChunks(projectID, filePath string) ([]*models.Chunk, error) {
	return a.projectService.GetFileChunks(projectID, filePath)
}

// GetGitignorePatterns returns the glob patterns derived from a project's .gitignore file.
func (a *App) GetGitignorePatterns(projectID string) ([]string, error) {
	return a.projectService.GetGitIgnorePatterns(projectID)
}

// ReadFileContent reads the content of a file within a project.
func (a *App) ReadFileContent(projectID, relativePath string) (string, error) {
	return a.projectService.ReadFileContent(projectID, relativePath)
}

// GetProjectStats returns statistics for a specific project.
// Exposed to frontend as: window.go.main.App.GetProjectStats
func (a *App) GetProjectStats(projectID string) (*models.ProjectStats, error) {
	return a.projectService.GetProjectStats(projectID)
}

// ==================== MCP Server API ====================

// GetMCPConfig returns the persisted MCP server configuration.
func (a *App) GetMCPConfig() (models.MCPServerConfig, error) {
	if a.mcpManager == nil {
		return models.MCPServerConfig{}, fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.GetConfig(), nil
}

// UpdateMCPConfig saves a new MCP configuration.
func (a *App) UpdateMCPConfig(config models.MCPServerConfig) (models.MCPServerConfig, error) {
	if a.mcpManager == nil {
		return models.MCPServerConfig{}, fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.UpdateConfig(config)
}

// StartMCPServer launches the MCP server manually.
func (a *App) StartMCPServer() error {
	if a.mcpManager == nil {
		return fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.Start(a.ctx)
}

// StopMCPServer stops the running MCP server.
func (a *App) StopMCPServer() error {
	if a.mcpManager == nil {
		return fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.Stop(context.Background())
}

// GetMCPStatus returns live metrics from the MCP server.
func (a *App) GetMCPStatus() (models.MCPServerStatus, error) {
	if a.mcpManager == nil {
		return models.MCPServerStatus{}, fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.GetStatus(), nil
}

// GetMCPTools lists all registered MCP tools.
func (a *App) GetMCPTools() ([]models.MCPTool, error) {
	if a.mcpManager == nil {
		return nil, fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.GetTools(), nil
}

// ToggleMCPTool flips the enabled state of a tool.
func (a *App) ToggleMCPTool(name string) error {
	if a.mcpManager == nil {
		return fmt.Errorf("mcp manager not initialized")
	}
	return a.mcpManager.ToggleTool(name)
}

// GetAllProjectsStats returns cumulative statistics across all projects.
// Exposed to frontend as: window.go.main.App.GetAllProjectsStats
func (a *App) GetAllProjectsStats() (*models.ProjectStats, error) {
	return a.projectService.GetAllProjectsStats()
}

// GetEmbeddingCapabilities exposes runtime availability to the frontend.
func (a *App) GetEmbeddingCapabilities() (*models.EmbeddingCapabilities, error) {
	return a.projectService.GetEmbeddingCapabilities()
}

// GetONNXRuntimeSettings returns the persisted ONNX runtime configuration.
func (a *App) GetONNXRuntimeSettings() (*models.ONNXRuntimeSettings, error) {
	return a.projectService.GetONNXRuntimeSettings()
}

// UpdateONNXRuntimeSettings saves a new ONNX runtime path (applied on restart).
func (a *App) UpdateONNXRuntimeSettings(path string) (*models.ONNXRuntimeSettings, error) {
	return a.projectService.UpdateONNXRuntimeSettings(path)
}

// TestONNXRuntimePath performs a lightweight validation of a provided ONNX path.
func (a *App) TestONNXRuntimePath(path string) (*models.ONNXRuntimeTestResult, error) {
	return a.projectService.TestONNXRuntimePath(path)
}

// ListEmbeddingModels returns the embedding model catalog.
func (a *App) ListEmbeddingModels() ([]*models.EmbeddingModelInfo, error) {
	return a.projectService.ListEmbeddingModels()
}

// SaveEmbeddingModel creates or updates an embedding model entry.
func (a *App) SaveEmbeddingModel(model models.EmbeddingModelInfo) (*models.EmbeddingModelInfo, error) {
	return a.projectService.SaveEmbeddingModel(model)
}

// DownloadEmbeddingModel ensures a catalog entry exists locally.
func (a *App) DownloadEmbeddingModel(modelID string) (*models.EmbeddingModelInfo, error) {
	return a.projectService.DownloadEmbeddingModel(modelID)
}

// Search executes semantic search for a project.
func (a *App) Search(projectID, query string, k int) (*models.SearchResponse, error) {
	return a.projectService.Search(projectID, query, k)
}
