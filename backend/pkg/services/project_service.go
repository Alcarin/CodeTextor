package services

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/indexing"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	GetFileOutline(projectID, path string) ([]*models.OutlineNode, error)
	GetFileChunks(projectID, path string) ([]*models.Chunk, error)
	GetOutlineTimestamps(projectID string) (map[string]int64, error)
	ReadFileContent(projectID, relativePath string) (string, error)
	StartIndexing(projectID string) error
	ResetProjectIndex(projectID string) error
	ReindexProject(projectID string) error
	StopIndexing(projectID string) error
	GetIndexingProgress(projectID string) (models.IndexingProgress, error)
	GetGitIgnorePatterns(projectID string) ([]string, error)
	GetProjectStats(projectID string) (*models.ProjectStats, error)
	GetAllProjectsStats() (*models.ProjectStats, error)
	Close() error
}

// ProjectService handles project lifecycle and indexing orchestration.
type ProjectService struct {
	indexesDir     string
	configStore    *store.ConfigStore
	indexerManager *indexing.Manager
	vectorStores   map[string]*store.VectorStore
	mu             sync.Mutex
	eventEmitter   func(string, interface{})
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
func NewProjectService(ctx context.Context) (*ProjectService, error) {
	indexesDir, err := utils.GetIndexesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve indexes directory: %w", err)
	}

	configStore, err := store.NewConfigStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open config store: %w", err)
	}

	var eventEmitter func(string, interface{})
	if ctx != nil {
		eventEmitter = func(event string, data interface{}) {
			runtime.EventsEmit(ctx, event, data)
		}
	}

	service := &ProjectService{
		indexesDir:     indexesDir,
		configStore:    configStore,
		indexerManager: indexing.NewManager(eventEmitter),
		vectorStores:   make(map[string]*store.VectorStore),
		eventEmitter:   eventEmitter,
	}

	// Auto-start indexing for projects with ContinuousIndexing enabled
	if err := service.initializeAutoIndexing(); err != nil {
		log.Printf("Warning: failed to initialize auto-indexing: %v", err)
	}

	return service, nil
}

// initializeAutoIndexing starts indexing for all projects that have ContinuousIndexing enabled.
func (s *ProjectService) initializeAutoIndexing() error {
	projects, err := s.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	for _, project := range projects {
		if project.Config.ContinuousIndexing {
			log.Printf("Auto-starting indexing for project %s (%s)", project.Name, project.ID)
			if err := s.StartIndexing(project.ID); err != nil {
				log.Printf("Failed to auto-start indexing for project %s: %v", project.ID, err)
			}
		}
	}

	return nil
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

	vectorStore, err := s.GetVectorStore(project.ID)
	if err != nil {
		return fmt.Errorf("failed to open vector store for outlining: %w", err)
	}

	s.indexerManager.StartIndexer(project, files, vectorStore)
	return nil
}

// ResetProjectIndex removes all indexed data for a project without restarting indexing.
func (s *ProjectService) ResetProjectIndex(projectID string) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	// Ensure no indexer is running while we wipe data.
	s.indexerManager.StopIndexer(projectID)

	vectorStore, err := s.GetVectorStore(project.ID)
	if err != nil {
		return fmt.Errorf("failed to open vector store for reset: %w", err)
	}

	if err := vectorStore.ResetProjectData(); err != nil {
		return fmt.Errorf("failed to reset index for %s: %w", projectID, err)
	}

	return nil
}

// ReindexProject clears all indexed data and performs a fresh indexing run.
func (s *ProjectService) ReindexProject(projectID string) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	// Ensure no indexer is running while we wipe data.
	s.indexerManager.StopIndexer(projectID)

	vectorStore, err := s.GetVectorStore(project.ID)
	if err != nil {
		return fmt.Errorf("failed to open vector store for reindexing: %w", err)
	}

	if err := vectorStore.ResetProjectData(); err != nil {
		return fmt.Errorf("failed to reset index for %s: %w", projectID, err)
	}

	files, err := s.GetFilePreviews(projectID, project.Config)
	if err != nil {
		return fmt.Errorf("failed to get file previews for reindexing: %w", err)
	}

	s.indexerManager.StartIndexer(project, files, vectorStore)
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

func isPathWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	if !strings.HasSuffix(root, string(os.PathSeparator)) {
		root += string(os.PathSeparator)
	}
	return strings.HasPrefix(target, root)
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
			relativePath = filepath.ToSlash(relativePath)

			if finalConfig.RootPath != "" {
				if rootRelative, err := filepath.Rel(finalConfig.RootPath, path); err == nil {
					relativePath = filepath.ToSlash(rootRelative)
				}
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
				LastModified: info.ModTime().Unix(),
			})

			return nil
		})

		if err != nil {
			log.Printf("Error walking path %s: %v", includePath, err)
		}
	}

	return previews, nil
}

// GetFileOutline retrieves the stored outline for a single file.
func (s *ProjectService) GetFileOutline(projectID, path string) ([]*models.OutlineNode, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	normalizedRoot := filepath.Clean(project.Config.RootPath)
	if normalizedRoot == "" {
		return nil, fmt.Errorf("project root path is not configured")
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	absPath := trimmed
	if !filepath.IsAbs(trimmed) {
		absPath = filepath.Join(normalizedRoot, trimmed)
	}
	absPath = filepath.Clean(absPath)

	if !isPathWithinRoot(normalizedRoot, absPath) {
		return nil, fmt.Errorf("path %s is outside the project root", trimmed)
	}

	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	absSlash := filepath.ToSlash(absPath)
	key := absSlash
	if rel, ok := utils.RelativePathWithinRoot(normalizedRoot, absPath); ok && rel != "" {
		key = rel
	}

	outline, err := vectorStore.GetFileOutline(key)
	if err != nil {
		return nil, err
	}
	if len(outline) == 0 && key != absSlash {
		outline, err = vectorStore.GetFileOutline(absSlash)
		if err != nil {
			return nil, err
		}
	}
	if len(outline) == 0 {
		return nil, fmt.Errorf("outline is not available for %s; ensure continuous indexing has run", trimmed)
	}

	return outline, nil
}

// GetFileChunks retrieves all semantic chunks for a given file from the database.
func (s *ProjectService) GetFileChunks(projectID, path string) ([]*models.Chunk, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	normalizedRoot := filepath.Clean(project.Config.RootPath)
	if normalizedRoot == "" {
		return nil, fmt.Errorf("project root path is not configured")
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	absPath := trimmed
	if !filepath.IsAbs(trimmed) {
		absPath = filepath.Join(normalizedRoot, trimmed)
	}
	absPath = filepath.Clean(absPath)

	if !isPathWithinRoot(normalizedRoot, absPath) {
		return nil, fmt.Errorf("path %s is outside the project root", trimmed)
	}

	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	// Use relative path as the key (consistent with how we save chunks)
	key := path
	if rel, ok := utils.RelativePathWithinRoot(normalizedRoot, absPath); ok && rel != "" {
		key = rel
	}

	chunks, err := vectorStore.GetFileChunks(key)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found for %s; file may not have been indexed yet", trimmed)
	}

	return chunks, nil
}

// GetOutlineTimestamps retrieves all outline update timestamps for a project.
// Returns a map of relative file paths to their last update timestamps (Unix time).
func (s *ProjectService) GetOutlineTimestamps(projectID string) (map[string]int64, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	timestamps, err := vectorStore.GetAllOutlineTimestamps()
	if err != nil {
		return nil, err
	}

	// Convert absolute paths to relative paths
	normalizedRoot := filepath.Clean(project.Config.RootPath)
	relativeTimestamps := make(map[string]int64)

	for storedPath, timestamp := range timestamps {
		pathKey := filepath.ToSlash(filepath.Clean(storedPath))
		if filepath.IsAbs(pathKey) {
			if rel, ok := utils.RelativePathWithinRoot(normalizedRoot, pathKey); ok && rel != "" {
				pathKey = rel
			}
		}
		if existing, ok := relativeTimestamps[pathKey]; !ok || timestamp > existing {
			relativeTimestamps[pathKey] = timestamp
		}
	}

	return relativeTimestamps, nil
}

// ReadFileContent reads the content of a file within a project.
// The relativePath is relative to the project root.
func (s *ProjectService) ReadFileContent(projectID, relativePath string) (string, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return "", err
	}

	normalizedRoot := filepath.Clean(project.Config.RootPath)
	if normalizedRoot == "" {
		return "", fmt.Errorf("project root path is not configured")
	}

	// Resolve absolute path
	trimmed := strings.TrimPrefix(relativePath, "/")
	trimmed = strings.TrimPrefix(trimmed, "\\")
	absPath := filepath.Join(normalizedRoot, trimmed)
	absPath = filepath.Clean(absPath)

	// Security check: ensure path is within project root
	if !isPathWithinRoot(normalizedRoot, absPath) {
		return "", fmt.Errorf("path %s is outside the project root", trimmed)
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", trimmed, err)
	}

	return string(content), nil
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

// GetProjectStats returns statistics for a specific project.
func (s *ProjectService) GetProjectStats(projectID string) (*models.ProjectStats, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vector store: %w", err)
	}

	stats, err := vectorStore.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Check if the project is currently indexing
	progress, found := s.indexerManager.GetIndexingProgress(projectID)
	if found && progress.Status == models.IndexingStatusIndexing {
		stats.IsIndexing = true
		if progress.TotalFiles > 0 {
			stats.IndexingProgress = float64(progress.ProcessedFiles) / float64(progress.TotalFiles)
		}
	}

	// Add project information
	_ = project // Use project if needed for additional context

	return stats, nil
}

// Close releases vector stores.
// GetAllProjectsStats returns cumulative statistics across all projects.
func (s *ProjectService) GetAllProjectsStats() (*models.ProjectStats, error) {
	projects, err := s.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	cumulativeStats := &models.ProjectStats{
		TotalFiles:   0,
		TotalChunks:  0,
		TotalSymbols: 0,
		DatabaseSize: 0,
	}

	var latestIndexTime *time.Time

	for _, project := range projects {
		vectorStore, err := s.GetVectorStore(project.ID)
		if err != nil {
			log.Printf("Warning: failed to get vector store for project %s: %v", project.ID, err)
			continue
		}

		stats, err := vectorStore.GetStats()
		if err != nil {
			log.Printf("Warning: failed to get stats for project %s: %v", project.ID, err)
			continue
		}

		// Accumulate stats
		cumulativeStats.TotalFiles += stats.TotalFiles
		cumulativeStats.TotalChunks += stats.TotalChunks
		cumulativeStats.TotalSymbols += stats.TotalSymbols
		cumulativeStats.DatabaseSize += stats.DatabaseSize

		// Track the most recent indexing time across all projects
		if stats.LastIndexedAt != nil {
			if latestIndexTime == nil || stats.LastIndexedAt.After(*latestIndexTime) {
				latestIndexTime = stats.LastIndexedAt
			}
		}
	}

	cumulativeStats.LastIndexedAt = latestIndexTime

	// Check if any project is currently indexing
	for _, project := range projects {
		progress, found := s.indexerManager.GetIndexingProgress(project.ID)
		if found && progress.Status == models.IndexingStatusIndexing {
			cumulativeStats.IsIndexing = true
			// Calculate overall indexing progress (weighted average across projects)
			if progress.TotalFiles > 0 {
				projectProgress := float64(progress.ProcessedFiles) / float64(progress.TotalFiles)
				// For simplicity, we'll use the progress of the first indexing project
				cumulativeStats.IndexingProgress = projectProgress
				break
			}
		}
	}

	return cumulativeStats, nil
}

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
