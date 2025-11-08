package services

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/indexing"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const selectedProjectKey = "selected_project"
const slugCollisionLimit = 10

// ProjectServiceAPI defines the interface for project-related operations.
type ProjectServiceAPI interface {
	CreateProject(req CreateProjectRequest) (*models.Project, error)
	GetProject(projectID string) (*models.Project, error)
	ListProjects() ([]*models.Project, error)
	UpdateProject(req UpdateProjectRequest) (*models.Project, error)
	UpdateProjectConfig(projectID string, config models.ProjectConfig) (*models.Project, error)
	DeleteProject(projectID string) error
	ProjectExists(projectID string) (bool, error)
	SetSelectedProject(projectID string) error
	GetSelectedProject() (*models.Project, error)
	ClearSelectedProject() error
	SetProjectIndexing(projectID string, enabled bool) error
	GetFilePreviews(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error)
	StartIndexing(projectID string) error
	StopIndexing(projectID string) error
	GetIndexingProgress(projectID string) (models.IndexingProgress, error)
	GetGitIgnorePatterns(projectID string) ([]string, error)
	Close() error
}

// ProjectService handles project lifecycle and indexing orchestration.
type ProjectService struct {
	indexesDir     string
	configStore    *store.ConfigStore
	indexerManager *indexing.Manager
	vectorStores   map[string]*store.VectorStore
	mu             sync.Mutex
}

// CreateProjectRequest contains data required to create a new project.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
	RootPath    string `json:"rootPath"`
}

// UpdateProjectRequest describes mutable fields of a project.
type UpdateProjectRequest struct {
	ProjectID   string                `json:"projectId"`
	Name        *string               `json:"name,omitempty"`
	Description *string               `json:"description,omitempty"`
	Config      *models.ProjectConfig `json:"config,omitempty"`
}

// NewProjectService initializes the service.
func NewProjectService() (*ProjectService, error) {
	indexesDir, err := utils.GetIndexesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve indexes directory: %w", err)
	}

	configStore, err := store.NewConfigStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open config store: %w", err)
	}

	return &ProjectService{
		indexesDir:     indexesDir,
		configStore:    configStore,
		indexerManager: indexing.NewManager(),
		vectorStores:   make(map[string]*store.VectorStore),
	}, nil
}

func (s *ProjectService) projectDBPath(projectID string) string {
	return filepath.Join(s.indexesDir, fmt.Sprintf("project-%s.db", projectID))
}

func (s *ProjectService) ensureUniqueProjectID(base string) (string, error) {
	candidate := base
	if candidate == "" {
		candidate = "project"
	}
	for attempts := 0; attempts < slugCollisionLimit; attempts++ {
		if exists, _ := s.ProjectExists(candidate); !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%s", base, uuid.New().String()[:8])
	}
	return "", fmt.Errorf("unable to generate unique project slug for %s", base)
}

func (s *ProjectService) normalizeRootPath(root string) (string, error) {
	cleaned := strings.TrimSpace(root)
	if cleaned == "" {
		return "", fmt.Errorf("project root path cannot be empty")
	}

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to resolve root path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("failed to access root path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root path must be a directory")
	}

	return abs, nil
}

// CreateProject creates a new project with a dedicated database file.
func (s *ProjectService) CreateProject(req CreateProjectRequest) (*models.Project, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	root, err := s.normalizeRootPath(req.RootPath)
	if err != nil {
		return nil, err
	}

	slug := req.Slug
	if slug == "" {
		slug = utils.GenerateSlug(req.Name)
	}
	projectID, err := s.ensureUniqueProjectID(slug)
	if err != nil {
		return nil, err
	}

	project := models.NewProject(projectID, req.Name, req.Description)
	project.Config.RootPath = root
	project.Config.IncludePaths = []string{"."}

	vs, err := store.NewVectorStore(project.ID, project.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create project database: %w", err)
	}
	if err := vs.SaveProjectMetadata(project); err != nil {
		vs.Close()
		return nil, err
	}
	vs.Close()

	return project, nil
}

// GetProject loads a project by id.
func (s *ProjectService) GetProject(projectID string) (*models.Project, error) {
	path := s.projectDBPath(projectID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project not found: %s", projectID)
		}
		return nil, fmt.Errorf("failed to read project database: %w", err)
	}

	if err := store.RunVectorMigrations(path); err != nil {
		return nil, err
	}

	project, err := store.LoadProjectMetadata(path)
	if err != nil {
		return nil, err
	}

	if len(project.Config.IncludePaths) == 0 {
		project.Config.IncludePaths = []string{"."}
	}

	return project, nil
}

// ListProjects returns all configured projects.
func (s *ProjectService) ListProjects() ([]*models.Project, error) {
	dbPaths, err := store.ListProjectDBPaths(s.indexesDir)
	if err != nil {
		return nil, err
	}

	projects := make([]*models.Project, 0, len(dbPaths))
	for _, path := range dbPaths {
		if err := store.RunVectorMigrations(path); err != nil {
			log.Printf("Failed to migrate project database %s: %v", path, err)
			continue
		}

		project, err := store.LoadProjectMetadata(path)
		if err != nil {
			log.Printf("Failed to load metadata from %s: %v", path, err)
			continue
		}
		if len(project.Config.IncludePaths) == 0 {
			project.Config.IncludePaths = []string{"."}
		}
		projects = append(projects, project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreatedAt > projects[j].CreatedAt
	})

	return projects, nil
}

func (s *ProjectService) updateProjectMetadata(project *models.Project) error {
	project.UpdatedAt = time.Now().Unix()
	return store.SaveProjectMetadata(s.projectDBPath(project.ID), project)
}

func (s *ProjectService) applyConfig(project *models.Project, config models.ProjectConfig) error {
	root := config.RootPath
	if strings.TrimSpace(root) != "" {
		normalized, err := s.normalizeRootPath(root)
		if err != nil {
			return err
		}
		config.RootPath = normalized
	} else {
		config.RootPath = project.Config.RootPath
	}

	if len(config.IncludePaths) == 0 {
		config.IncludePaths = []string{"."}
	}

	project.Config = config
	return nil
}

// UpdateProject updates metadata or configuration.
func (s *ProjectService) UpdateProject(req UpdateProjectRequest) (*models.Project, error) {
	project, err := s.GetProject(req.ProjectID)
	if err != nil {
		return nil, err
	}

	updated := false
	if req.Name != nil && *req.Name != project.Name {
		project.Name = *req.Name
		updated = true
	}
	if req.Description != nil && *req.Description != project.Description {
		project.Description = *req.Description
		updated = true
	}
	if req.Config != nil {
		if err := s.applyConfig(project, *req.Config); err != nil {
			return nil, err
		}
		updated = true
	}

	if !updated {
		return project, nil
	}

	if err := s.updateProjectMetadata(project); err != nil {
		return nil, err
	}

	return project, nil
}

// UpdateProjectConfig updates only the stored configuration.
func (s *ProjectService) UpdateProjectConfig(projectID string, config models.ProjectConfig) (*models.Project, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	if err := s.applyConfig(project, config); err != nil {
		return nil, err
	}

	if err := s.updateProjectMetadata(project); err != nil {
		return nil, err
	}

	return project, nil
}

// DeleteProject removes a project database.
func (s *ProjectService) DeleteProject(projectID string) error {
	s.mu.Lock()
	if vs, ok := s.vectorStores[projectID]; ok {
		vs.Close()
		delete(s.vectorStores, projectID)
	}
	s.mu.Unlock()

	path := s.projectDBPath(projectID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove project database: %w", err)
	}

	if err := s.clearSelectedProjectIfMatches(projectID); err != nil {
		log.Printf("Failed to clear selected project: %v", err)
	}

	return nil
}

func (s *ProjectService) clearSelectedProjectIfMatches(projectID string) error {
	current, ok, err := s.configStore.GetValue(selectedProjectKey)
	if err != nil {
		return err
	}
	if ok && current == projectID {
		return s.configStore.DeleteValue(selectedProjectKey)
	}
	return nil
}

// ProjectExists checks if the database file exists for a project.
func (s *ProjectService) ProjectExists(projectID string) (bool, error) {
	path := s.projectDBPath(projectID)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to read project file: %w", err)
}

// SetSelectedProject stores the current selection.
func (s *ProjectService) SetSelectedProject(projectID string) error {
	exists, err := s.ProjectExists(projectID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	if err := s.configStore.SetValue(selectedProjectKey, projectID); err != nil {
		return err
	}
	return nil
}

// GetSelectedProject returns the project that was marked as selected.
func (s *ProjectService) GetSelectedProject() (*models.Project, error) {
	projectID, ok, err := s.configStore.GetValue(selectedProjectKey)
	if err != nil {
		return nil, err
	}
	if !ok || projectID == "" {
		return nil, nil
	}
	return s.GetProject(projectID)
}

// ClearSelectedProject removes any stored selection.
func (s *ProjectService) ClearSelectedProject() error {
	return s.configStore.DeleteValue(selectedProjectKey)
}

// SetProjectIndexing enables or disables continuous indexing for a project.
func (s *ProjectService) SetProjectIndexing(projectID string, enabled bool) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	project.IsIndexing = enabled
	project.Config.ContinuousIndexing = enabled

	if err := s.updateProjectMetadata(project); err != nil {
		return err
	}

	if enabled {
		return s.StartIndexing(projectID)
	}

	s.indexerManager.StopIndexer(projectID)
	return nil
}

// GetVectorStore returns or creates the cached vector store for a project.
func (s *ProjectService) GetVectorStore(projectID string) (*store.VectorStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if vs, ok := s.vectorStores[projectID]; ok {
		return vs, nil
	}

	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	vs, err := store.NewVectorStore(project.ID, project.ID)
	if err != nil {
		return nil, err
	}

	s.vectorStores[projectID] = vs
	return vs, nil
}

// StartIndexing begins indexing files for a project.
func (s *ProjectService) StartIndexing(projectID string) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	files, err := s.GetFilePreviews(projectID, project.Config)
	if err != nil {
		return fmt.Errorf("failed to get file previews for indexing: %w", err)
	}

	s.indexerManager.StartIndexer(project, files)
	return nil
}

// StopIndexing halts the project indexer.
func (s *ProjectService) StopIndexing(projectID string) error {
	s.indexerManager.StopIndexer(projectID)
	return nil
}

// GetIndexingProgress returns the progress for an ongoing run.
func (s *ProjectService) GetIndexingProgress(projectID string) (models.IndexingProgress, error) {
	progress, found := s.indexerManager.GetIndexingProgress(projectID)
	if !found {
		return models.IndexingProgress{Status: models.IndexingStatusIdle}, nil
	}
	return *progress, nil
}

func mergeConfig(base, override models.ProjectConfig) models.ProjectConfig {
	result := base
	if strings.TrimSpace(override.RootPath) != "" {
		result.RootPath = override.RootPath
	}
	if override.ExcludePatterns != nil {
		result.ExcludePatterns = override.ExcludePatterns
	}
	if override.FileExtensions != nil {
		result.FileExtensions = override.FileExtensions
	}
	if override.IncludePaths != nil {
		result.IncludePaths = override.IncludePaths
	}
	result.AutoExcludeHidden = override.AutoExcludeHidden
	result.ContinuousIndexing = override.ContinuousIndexing
	if override.ChunkSizeMin != 0 {
		result.ChunkSizeMin = override.ChunkSizeMin
	}
	if override.ChunkSizeMax != 0 {
		result.ChunkSizeMax = override.ChunkSizeMax
	}
	if override.EmbeddingModel != "" {
		result.EmbeddingModel = override.EmbeddingModel
	}
	if override.MaxResponseBytes != 0 {
		result.MaxResponseBytes = override.MaxResponseBytes
	}
	return result
}

func resolveIncludePaths(root string, includes []string) []string {
	if len(includes) == 0 {
		includes = []string{"."}
	}
	resolved := make([]string, 0, len(includes))
	for _, rel := range includes {
		if rel == "" || rel == "." {
			resolved = append(resolved, root)
			continue
		}
		if filepath.IsAbs(rel) {
			resolved = append(resolved, filepath.Clean(rel))
			continue
		}
		resolved = append(resolved, filepath.Join(root, rel))
	}
	return resolved
}

// GetFilePreviews returns files that match the provided configuration.
func (s *ProjectService) GetFilePreviews(projectID string, config models.ProjectConfig) ([]*models.FilePreview, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	finalConfig := mergeConfig(project.Config, config)
	if finalConfig.RootPath == "" {
		finalConfig.RootPath = project.Config.RootPath
	}
	includePaths := resolveIncludePaths(finalConfig.RootPath, finalConfig.IncludePaths)

	var previews []*models.FilePreview
	seenFiles := make(map[string]bool)
	extensionSet := make(map[string]struct{})
	for _, ext := range finalConfig.FileExtensions {
		extensionSet[ext] = struct{}{}
	}

	for _, includePath := range includePaths {
		err := filepath.WalkDir(includePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if seenFiles[path] {
				return nil
			}
			seenFiles[path] = true

			relativePath, _ := filepath.Rel(includePath, path)
			if relativePath == "." {
				return nil
			}

			isHidden := strings.HasPrefix(d.Name(), ".") && len(d.Name()) > 1
			if finalConfig.AutoExcludeHidden && isHidden {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			for _, pattern := range finalConfig.ExcludePatterns {
				if matched, _ := filepath.Match(pattern, relativePath); matched {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if matched, _ := filepath.Match(pattern, path); matched {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if d.IsDir() {
				return nil
			}

			ext := filepath.Ext(d.Name())
			if len(extensionSet) > 0 {
				if _, ok := extensionSet[ext]; !ok {
					return nil
				}
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			previews = append(previews, &models.FilePreview{
				AbsolutePath: path,
				RelativePath: relativePath,
				Extension:    ext,
				Size:         utils.FormatBytes(info.Size()),
				Hidden:       isHidden,
			})

			return nil
		})

		if err != nil {
			log.Printf("Error walking path %s: %v", includePath, err)
		}
	}

	return previews, nil
}

// GetGitIgnorePatterns returns glob patterns derived from the project's .gitignore.
func (s *ProjectService) GetGitIgnorePatterns(projectID string) ([]string, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	root := project.Config.RootPath
	if strings.TrimSpace(root) == "" {
		return []string{}, nil
	}
	gitignorePath := filepath.Join(root, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read .gitignore: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	patterns := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			// Ignore negation rules for now
			continue
		}
		pattern := line
		pattern = strings.TrimPrefix(pattern, "./")
		pattern = strings.TrimPrefix(pattern, "/")
		pattern = filepath.ToSlash(pattern)
		if !strings.HasPrefix(pattern, "**/") && !strings.Contains(pattern, "/") {
			pattern = "**/" + pattern
		}
		patterns = append(patterns, pattern)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse .gitignore: %w", err)
	}
	return patterns, nil
}

// Close releases vector stores.
func (s *ProjectService) Close() error {
	var firstErr error
	s.mu.Lock()
	for projectID, vs := range s.vectorStores {
		if err := vs.Close(); err != nil && firstErr == nil {
			firstErr = err
			log.Printf("Error closing vector store %s: %v", projectID, err)
		}
	}
	s.vectorStores = make(map[string]*store.VectorStore)
	s.mu.Unlock()
	if s.configStore != nil {
		if err := s.configStore.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
