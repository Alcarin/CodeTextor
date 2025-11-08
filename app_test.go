package main

import (
	"context"
	"testing"

	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/services"
	"CodeTextor/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

// MockProjectServiceAPI for testing App methods
type MockProjectServiceAPI struct {
	CreateProjectFunc        func(req services.CreateProjectRequest) (*models.Project, error)
	GetProjectFunc           func(projectID string) (*models.Project, error)
	ListProjectsFunc         func() ([]*models.Project, error)
	UpdateProjectFunc        func(req services.UpdateProjectRequest) (*models.Project, error)
	UpdateProjectConfigFunc  func(projectID string, config models.ProjectConfig) (*models.Project, error)
	DeleteProjectFunc        func(projectID string) error
	ProjectExistsFunc        func(projectID string) (bool, error)
	SetSelectedProjectFunc   func(projectID string) error
	GetSelectedProjectFunc   func() (*models.Project, error)
	ClearSelectedProjectFunc func() error
	SetProjectIndexingFunc   func(projectID string, enabled bool) error
	GetFilePreviewsFunc      func(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error)
	StartIndexingFunc        func(projectID string) error
	StopIndexingFunc         func(projectID string) error
	GetIndexingProgressFunc  func(projectID string) (models.IndexingProgress, error)
	GetGitIgnorePatternsFunc func(projectID string) ([]string, error)
	CloseFunc                func() error
}

func (m *MockProjectServiceAPI) CreateProject(req services.CreateProjectRequest) (*models.Project, error) {
	if m.CreateProjectFunc != nil {
		return m.CreateProjectFunc(req)
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) GetProject(projectID string) (*models.Project, error) {
	if m.GetProjectFunc != nil {
		return m.GetProjectFunc(projectID)
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) ListProjects() ([]*models.Project, error) {
	if m.ListProjectsFunc != nil {
		return m.ListProjectsFunc()
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) UpdateProject(req services.UpdateProjectRequest) (*models.Project, error) {
	if m.UpdateProjectFunc != nil {
		return m.UpdateProjectFunc(req)
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) UpdateProjectConfig(projectID string, config models.ProjectConfig) (*models.Project, error) {
	if m.UpdateProjectConfigFunc != nil {
		return m.UpdateProjectConfigFunc(projectID, config)
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) DeleteProject(projectID string) error {
	if m.DeleteProjectFunc != nil {
		return m.DeleteProjectFunc(projectID)
	}
	return nil
}
func (m *MockProjectServiceAPI) ProjectExists(projectID string) (bool, error) {
	if m.ProjectExistsFunc != nil {
		return m.ProjectExistsFunc(projectID)
	}
	return false, nil
}
func (m *MockProjectServiceAPI) SetSelectedProject(projectID string) error {
	if m.SetSelectedProjectFunc != nil {
		return m.SetSelectedProjectFunc(projectID)
	}
	return nil
}
func (m *MockProjectServiceAPI) GetSelectedProject() (*models.Project, error) {
	if m.GetSelectedProjectFunc != nil {
		return m.GetSelectedProjectFunc()
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) ClearSelectedProject() error {
	if m.ClearSelectedProjectFunc != nil {
		return m.ClearSelectedProjectFunc()
	}
	return nil
}
func (m *MockProjectServiceAPI) SetProjectIndexing(projectID string, enabled bool) error {
	if m.SetProjectIndexingFunc != nil {
		return m.SetProjectIndexingFunc(projectID, enabled)
	}
	return nil
}
func (m *MockProjectServiceAPI) GetFilePreviews(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error) {
	if m.GetFilePreviewsFunc != nil {
		return m.GetFilePreviewsFunc(projectID, config)
	}
	return nil, nil
}
func (m *MockProjectServiceAPI) StartIndexing(projectID string) error {
	if m.StartIndexingFunc != nil {
		return m.StartIndexingFunc(projectID)
	}
	return nil
}
func (m *MockProjectServiceAPI) StopIndexing(projectID string) error {
	if m.StopIndexingFunc != nil {
		return m.StopIndexingFunc(projectID)
	}
	return nil
}
func (m *MockProjectServiceAPI) GetIndexingProgress(projectID string) (models.IndexingProgress, error) {
	if m.GetIndexingProgressFunc != nil {
		return m.GetIndexingProgressFunc(projectID)
	}
	return models.IndexingProgress{}, nil
}
func (m *MockProjectServiceAPI) GetGitIgnorePatterns(projectID string) ([]string, error) {
	if m.GetGitIgnorePatternsFunc != nil {
		return m.GetGitIgnorePatternsFunc(projectID)
	}
	return []string{}, nil
}
func (m *MockProjectServiceAPI) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestApp_UpdateProjectConfig(t *testing.T) {
	assert := assert.New(t)

	mockProject := models.NewProject("test-id", "Test Project", "A project for testing")
	mockProject.Config.AutoExcludeHidden = false
	mockProject.Config.IncludePaths = []string{"/path/to/src"}

	mockService := &MockProjectServiceAPI{
		UpdateProjectConfigFunc: func(projectID string, config models.ProjectConfig) (*models.Project, error) {
			assert.Equal("test-id", projectID)
			assert.Equal(false, config.AutoExcludeHidden)
			assert.Contains(config.IncludePaths, "/path/to/src")
			return mockProject, nil
		},
	}

	app := &App{
		ctx:            context.Background(),
		projectService: mockService,
	}

	newConfig := models.ProjectConfig{
		AutoExcludeHidden: false,
		IncludePaths:      []string{"/path/to/src"},
	}

	updatedProject, err := app.UpdateProjectConfig("test-id", newConfig)
	assert.NoError(err)
	assert.NotNil(updatedProject)
	assert.Equal(mockProject.ID, updatedProject.ID)
}

func TestApp_StartIndexing(t *testing.T) {
	assert := assert.New(t)

	mockService := &MockProjectServiceAPI{
		StartIndexingFunc: func(projectID string) error {
			assert.Equal("project-123", projectID)
			return nil
		},
	}

	app := &App{
		ctx:            context.Background(),
		projectService: mockService,
	}

	err := app.StartIndexing("project-123")
	assert.NoError(err)
}

func TestApp_StopIndexing(t *testing.T) {
	assert := assert.New(t)

	mockService := &MockProjectServiceAPI{
		StopIndexingFunc: func(projectID string) error {
			assert.Equal("project-123", projectID)
			return nil
		},
	}

	app := &App{
		ctx:            context.Background(),
		projectService: mockService,
	}

	err := app.StopIndexing("project-123")
	assert.NoError(err)
}

func TestApp_GetIndexingProgress(t *testing.T) {
	assert := assert.New(t)

	expectedProgress := models.IndexingProgress{
		TotalFiles:     100,
		ProcessedFiles: 50,
		CurrentFile:    "src/main.go",
		Status:         models.IndexingStatusIndexing,
	}

	mockService := &MockProjectServiceAPI{
		GetIndexingProgressFunc: func(projectID string) (models.IndexingProgress, error) {
			assert.Equal("project-123", projectID)
			return expectedProgress, nil
		},
	}

	app := &App{
		ctx:            context.Background(),
		projectService: mockService,
	}

	progress, err := app.GetIndexingProgress("project-123")
	assert.NoError(err)
	assert.Equal(expectedProgress.TotalFiles, progress.TotalFiles)
	assert.Equal(expectedProgress.ProcessedFiles, progress.ProcessedFiles)
	assert.Equal(expectedProgress.CurrentFile, progress.CurrentFile)
	assert.Equal(expectedProgress.Status, progress.Status)
}

func Test_FormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"bytes", 500, "500 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"kilobytes_fraction", 1536, "1.5 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"terabytes", 1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{"petabytes", 1024 * 1024 * 1024 * 1024 * 1024, "1.0 PB"},
		{"exabytes", 1024 * 1024 * 1024 * 1024 * 1024 * 1024, "1.0 EB"},
		{"large value", 1234567890123456789, "1.1 EB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, utils.FormatBytes(tt.bytes))
		})
	}
}

func TestApp_GetFilePreviews(t *testing.T) {
	assert := assert.New(t)

	// Setup expected file previews
	expectedPreviews := []*models.FilePreview{
		{
			AbsolutePath: "/test/project/src/main.go",
			RelativePath: "main.go",
			Extension:    ".go",
			Size:         "1.2 KB",
			Hidden:       false,
		},
		{
			AbsolutePath: "/test/project/src/helper.go",
			RelativePath: "helper.go",
			Extension:    ".go",
			Size:         "856 B",
			Hidden:       false,
		},
		{
			AbsolutePath: "/test/project/docs/guide.md",
			RelativePath: "guide.md",
			Extension:    ".md",
			Size:         "2.4 KB",
			Hidden:       false,
		},
	}

	mockConfig := models.ProjectConfig{
		IncludePaths:      []string{"/test/project/src", "/test/project/docs"},
		ExcludePatterns:   []string{"**/node_modules"},
		AutoExcludeHidden: true,
		FileExtensions:    []string{".go", ".md"},
	}

	mockService := &MockProjectServiceAPI{
		GetFilePreviewsFunc: func(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error) {
			assert.Equal("test-id", projectID)
			assert.Equal(mockConfig.IncludePaths, config.IncludePaths)
			assert.Equal(mockConfig.ExcludePatterns, config.ExcludePatterns)
			assert.Equal(mockConfig.AutoExcludeHidden, config.AutoExcludeHidden)
			return expectedPreviews, nil
		},
	}

	app := &App{
		ctx:            context.Background(),
		projectService: mockService,
	}

	// Call GetFilePreviews
	previews, err := app.GetFilePreviews("test-id", mockConfig)
	assert.NoError(err)
	assert.NotNil(previews)
	assert.Len(previews, 3)

	// Verify returned previews match expected
	for i, preview := range previews {
		assert.Equal(expectedPreviews[i].AbsolutePath, preview.AbsolutePath)
		assert.Equal(expectedPreviews[i].RelativePath, preview.RelativePath)
		assert.Equal(expectedPreviews[i].Extension, preview.Extension)
		assert.Equal(expectedPreviews[i].Size, preview.Size)
		assert.Equal(expectedPreviews[i].Hidden, preview.Hidden)
	}
}
