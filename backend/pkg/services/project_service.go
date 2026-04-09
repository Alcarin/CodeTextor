package services

import (
	"CodeTextor/backend/internal/chunker"
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/embedding"
	"CodeTextor/backend/pkg/indexing"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/outline"
	"CodeTextor/backend/pkg/utils"
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	stdruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const selectedProjectKey = "selected_project"
const slugCollisionLimit = 10
const (
	defaultFastEmbedModelID = "fastembed/bge-small-en-v1.5"
	defaultOnnxModelID      = "baai/bge-small-en-v1.5"
	onnxRuntimePathKey      = "onnx_runtime_path"
)

var (
	supportedEmbeddingModelIDs = buildSupportedModelSet()
	loggedONNXWarning          sync.Once
)

func buildSupportedModelSet() map[string]struct{} {
	set := make(map[string]struct{})
	for _, entry := range models.DefaultEmbeddingModels() {
		if entry == nil || strings.TrimSpace(entry.ID) == "" {
			continue
		}
		set[strings.ToLower(entry.ID)] = struct{}{}
	}
	return set
}

func detectONNXRuntimeAvailability() bool {
	available, err := embedding.CheckONNXRuntimeAvailability()
	if err != nil {
		log.Printf("ONNX Runtime unavailable: %v", err)
		return false
	}
	if !available {
		log.Printf("ONNX Runtime unavailable (initialization failed)")
		return false
	}

	log.Printf("ONNX Runtime initialized successfully")
	return true
}

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
	GetProjectStructure(projectID string, subPath string, depth int) ([]*models.FilePreview, error)
	GetFileOutline(projectID, path string) ([]*models.OutlineNode, error)
	GetFileChunks(projectID, path string) ([]*models.Chunk, error)
	GetChunkByID(projectID, chunkID string) (*models.Chunk, error)
	GetChunksByIDFuzzy(projectID, chunkID string) ([]*models.Chunk, error)
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
	ListEmbeddingModels() ([]*models.EmbeddingModelInfo, error)
	SaveEmbeddingModel(model models.EmbeddingModelInfo) (*models.EmbeddingModelInfo, error)
	DownloadEmbeddingModel(modelID string) (*models.EmbeddingModelInfo, error)
	GetEmbeddingCapabilities() (*models.EmbeddingCapabilities, error)
	GetONNXRuntimeSettings() (*models.ONNXRuntimeSettings, error)
	UpdateONNXRuntimeSettings(path string) (*models.ONNXRuntimeSettings, error)
	TestONNXRuntimePath(path string) (*models.ONNXRuntimeTestResult, error)
	DownloadONNXRuntime() error
	Search(projectID string, query string, k int) (*models.SearchResponse, error)
	GrepSearch(projectID string, query string, isRegex bool, subPath string, limit int) (*models.GrepSearchResponse, error)
	GetRecentChanges(projectID string, limit int) (*models.RecentChangesResponse, error)
	GetProjectSummary(projectID string) (*models.ProjectSummary, error)
	FindReferences(projectID, nodeID, symbolName, path string) (*models.SymbolReferencesResponse, error)
	GetCallGraph(projectID, nodeID, symbolName, path, direction string, depth int) (*models.CallGraphResponse, error)
	FindImplementations(projectID, nodeID string) (*models.FindImplementationsResponse, error)
	FindTodos(projectID string) (*models.FindTodosResponse, error)
	GetPackageGraph(projectID string, depth int) (models.PackageGraphResponse, error)
	GetSupportedExtensions() []string
	GetIndexedFiles(projectID string) ([]*models.IndexedFile, error)
	Close() error
}

const defaultEmbeddingModelID = defaultOnnxModelID
const defaultFastEmbedModel = "fastembed/bge-small-en-v1.5"

// ProjectService handles project lifecycle and indexing orchestration.
type ProjectService struct {
	indexesDir        string
	configStore       *store.ConfigStore
	indexerManager    *indexing.Manager
	vectorStores      map[string]*store.VectorStore
	mu                sync.Mutex
	eventEmitter      func(string, interface{})
	modelDownloader   *embedding.Downloader
	embeddingClients  map[string]embedding.EmbeddingClient
	loadingClients    map[string]chan struct{} // ModelID -> Channel that closes when loading is done
	clientsMu         sync.Mutex
	enableONNXRuntime bool
	onnxRuntimePath   string
	activeONNXPath    string
	heavyTasks        map[string]bool // ProjectID -> is running a heavy task (reindex)
	initialSystemVRAM int             // VRAM baseline (system + non-app) in MB
	linkerService     *LinkerService
	extensionsCache   []string
	extensionsMu      sync.Mutex
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
		indexesDir:        indexesDir,
		configStore:       configStore,
		indexerManager:    indexing.NewManager(eventEmitter),
		vectorStores:      make(map[string]*store.VectorStore),
		eventEmitter:      eventEmitter,
		modelDownloader:   embedding.NewDownloader(),
		embeddingClients:  make(map[string]embedding.EmbeddingClient),
		loadingClients:    make(map[string]chan struct{}),
		heavyTasks:        make(map[string]bool),
		linkerService:     NewLinkerService(),
		enableONNXRuntime: false,
	}

	// Load persisted ONNX runtime path before detection so initialization uses it.
	if path, ok, err := configStore.GetValue(onnxRuntimePathKey); err == nil && ok {
		service.onnxRuntimePath = strings.TrimSpace(path)
	} else if err != nil {
		log.Printf("Warning: failed to read ONNX runtime path: %v", err)
	}

	embedding.ConfigureSharedLibraryPath(service.onnxRuntimePath)
	service.enableONNXRuntime = detectONNXRuntimeAvailability()
	service.activeONNXPath = embedding.ActiveSharedLibraryPath()

	if err := service.ensureDefaultEmbeddingModels(); err != nil {
		log.Printf("Warning: failed to seed embedding model catalog: %v", err)
	}

	// Auto-start indexing for projects with ContinuousIndexing enabled
	go func() {
		if err := service.initializeAutoIndexing(); err != nil {
			log.Printf("Warning: failed to initialize auto-indexing: %v", err)
		}
	}()

	// Inizializza la baseline VRAM del sistema (OS + altre app già aperte)
	service.initialSystemVRAM = embedding.GetTotalVRAMUsage()
	log.Printf("VRAM Initial Baseline: %d MB", service.initialSystemVRAM)

	// Start Eco-Mode cleaner to free VRAM during inactivity
	go service.startEcoModeCleaner()

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
	if err := s.ensureEmbeddingModelSnapshot(&project.Config); err != nil {
		return nil, err
	}

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
	if err := s.ensureEmbeddingModelSnapshot(&project.Config); err != nil {
		return nil, err
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
		if err := s.ensureEmbeddingModelSnapshot(&project.Config); err != nil {
			log.Printf("Failed to attach embedding model to %s: %v", project.ID, err)
			continue
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

	if strings.TrimSpace(config.EmbeddingModel) == "" {
		config.EmbeddingModel = project.Config.EmbeddingModel
		config.EmbeddingModelInfo = project.Config.EmbeddingModelInfo
	}

	if err := s.ensureEmbeddingModelSnapshot(&config); err != nil {
		return err
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
	s.indexerManager.StopIndexer(projectID)
	s.indexerManager.ClearProgress(projectID)

	s.mu.Lock()
	if vs, ok := s.vectorStores[projectID]; ok {
		vs.Close()
		delete(s.vectorStores, projectID)
	}
	s.mu.Unlock()

	path := s.projectDBPath(projectID)
	// Retry removal a few times to give the indexer time to release file handles
	var removeErr error
	for i := 0; i < 5; i++ {
		removeErr = os.Remove(path)
		if removeErr == nil || os.IsNotExist(removeErr) {
			// Also try to remove WAL and SHM files
			_ = os.Remove(path + "-wal")
			_ = os.Remove(path + "-shm")
			removeErr = nil
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if removeErr != nil {
		return fmt.Errorf("failed to remove project database after retries: %w", removeErr)
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

// GetPackageGraph aggregates file-level dependencies into a folder-level matrix.
func (s *ProjectService) GetPackageGraph(projectID string, depth int) (models.PackageGraphResponse, error) {
	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	paths, err := vs.GetSymbolUsagePaths()
	if err != nil {
		return nil, err
	}

	graph := make(models.PackageGraphResponse)

	for _, up := range paths {
		sourcePkg := s.pathToPackage(up.SourcePath, depth)
		targetPkg := s.pathToPackage(up.TargetPath, depth)

		// Skip self-dependencies at the same package level
		if sourcePkg == targetPkg {
			continue
		}

		if _, ok := graph[sourcePkg]; !ok {
			graph[sourcePkg] = make(map[string]int)
		}
		graph[sourcePkg][targetPkg]++
	}

	return graph, nil
}

func (s *ProjectService) pathToPackage(path string, depth int) string {
	// Normalize path
	path = filepath.ToSlash(path)

	// Virtual symbols often start with @external/
	isExternal := strings.HasPrefix(path, "@external/")

	dir := ""
	if isExternal {
		dir = path
	} else {
		dir = filepath.ToSlash(filepath.Dir(path))
	}

	if dir == "." || dir == "/" {
		if isExternal {
			return "@external"
		}
		return "root"
	}

	parts := strings.Split(dir, "/")

	// If depth is specified, truncate components
	if depth > 0 && len(parts) > depth {
		parts = parts[:depth]
	}

	return strings.Join(parts, "/")
}

// SetProjectIndexing enables or disables continuous indexing for a project.
func (s *ProjectService) SetProjectIndexing(projectID string, enabled bool) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	if enabled {
		vectorStore, err := s.GetVectorStore(projectID)
		if err != nil {
			return err
		}
		match, err := s.embeddingUsageMatchesSelection(project, vectorStore)
		if err != nil {
			return fmt.Errorf("failed to verify embedding model consistency: %w", err)
		}
		if !match {
			if err := vectorStore.ResetProjectData(); err != nil {
				return fmt.Errorf("failed to reset index for model change: %w", err)
			}
		}
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

	// 1. Resolve the project first to get the canonical ID (slug).
	// This prevents multiple VectorStore instances for the same project due to casing.
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	canonicalID := project.ID

	if vs, ok := s.vectorStores[canonicalID]; ok {
		return vs, nil
	}

	vs, err := store.NewVectorStore(canonicalID, canonicalID)
	if err != nil {
		return nil, err
	}

	s.vectorStores[canonicalID] = vs
	return vs, nil
}

func (s *ProjectService) embeddingUsageMatchesSelection(project *models.Project, vectorStore *store.VectorStore) (bool, error) {
	if project == nil || vectorStore == nil {
		return true, nil
	}

	stats, err := vectorStore.GetStats()
	if err != nil {
		return false, err
	}
	if len(stats.EmbeddingModels) == 0 {
		return true, nil
	}

	selectedID := strings.TrimSpace(project.Config.EmbeddingModel)
	if selectedID == "" && project.Config.EmbeddingModelInfo != nil {
		selectedID = strings.TrimSpace(project.Config.EmbeddingModelInfo.ID)
	}
	normalizedSelected := strings.ToLower(selectedID)

	for _, usage := range stats.EmbeddingModels {
		usageID := strings.TrimSpace(usage.ModelID)
		if usageID == "" {
			if normalizedSelected != "" {
				return false, nil
			}
			continue
		}
		if normalizedSelected == "" {
			normalizedSelected = strings.ToLower(usageID)
			selectedID = usage.ModelID
			continue
		}
		if !strings.EqualFold(usageID, selectedID) {
			return false, nil
		}
	}
	return true, nil
}

// StartIndexing begins indexing files for a project.
func (s *ProjectService) StartIndexing(projectID string) error {
	project, err := s.GetProject(projectID)
	if err != nil {
		return err
	}

	// Use effective config with gitignore patterns merged if enabled
	effectiveConfig := project.Config
	if effectiveConfig.UseGitIgnore {
		if giPatterns, err := s.GetGitIgnorePatterns(projectID); err == nil {
			effectiveConfig.ExcludePatterns = append(effectiveConfig.ExcludePatterns, giPatterns...)
		}
	}

	files, err := s.GetFilePreviews(projectID, effectiveConfig)
	if err != nil {
		return fmt.Errorf("failed to get file previews for indexing: %w", err)
	}

	// Update the project snapshot for the indexer so it uses effective patterns for watching
	projectCopy := *project
	projectCopy.Config = effectiveConfig
	project = &projectCopy

	vectorStore, err := s.GetVectorStore(project.ID)
	if err != nil {
		return fmt.Errorf("failed to open vector store for outlining: %w", err)
	}

	client, err := s.getEmbeddingClient(project)
	if err != nil {
		return fmt.Errorf("failed to initialize embedding model: %w", err)
	}

	onComplete := func(status models.IndexingStatus) {
		// General completion (e.g. for overall progress status)
	}

	onInitialScanComplete := func() {
		if err := s.linkerService.ResolveUsages(project.ID, vectorStore); err != nil {
			log.Printf("Warning: linker failed for project %s: %v", project.ID, err)
		}
	}

	onFileIndexed := func(filePath string) {
		if err := s.linkerService.ResolveFileUsages(project.ID, filePath, vectorStore); err != nil {
			log.Printf("Warning: incremental linker failed for file %s: %v", filePath, err)
		}
	}

	if err := s.indexerManager.StartIndexer(project, files, vectorStore, client, onComplete, onInitialScanComplete, onFileIndexed); err != nil {
		return fmt.Errorf("failed to start indexer: %w", err)
	}
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

	client, err := s.getEmbeddingClient(project)
	if err != nil {
		return fmt.Errorf("failed to initialize embedding model: %w", err)
	}

	// Activate heavy task mode for this project to boost VRAM budget
	s.clientsMu.Lock()
	s.heavyTasks[projectID] = true
	s.clientsMu.Unlock()
	s.rebalanceBatchSizes()

	onComplete := func(status models.IndexingStatus) {
		s.clientsMu.Lock()
		delete(s.heavyTasks, projectID)
		s.clientsMu.Unlock()
		s.rebalanceBatchSizes()

		log.Printf("Reindexing lifecycle update for project %s (Status: %v).", projectID, status)
	}

	onInitialScanComplete := func() {
		if err := s.linkerService.ResolveUsages(projectID, vectorStore); err != nil {
			log.Printf("Warning: linker failed for project %s: %v", projectID, err)
		}
		log.Printf("Bulk linking completed for reindexed project %s", projectID)
	}

	onFileIndexed := func(filePath string) {
		if err := s.linkerService.ResolveFileUsages(projectID, filePath, vectorStore); err != nil {
			log.Printf("Warning: incremental linker failed for file %s: %v", filePath, err)
		}
	}

	if err := s.indexerManager.StartIndexer(project, files, vectorStore, client, onComplete, onInitialScanComplete, onFileIndexed); err != nil {
		// If indexing failed to even start, ensure heavyTasks is cleaned up
		s.clientsMu.Lock()
		delete(s.heavyTasks, projectID)
		s.clientsMu.Unlock()
		s.rebalanceBatchSizes()
		return fmt.Errorf("failed to start indexer: %w", err)
	}
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
	return progress, nil
}

func (s *ProjectService) ensureDefaultEmbeddingModels() error {
	defaults := models.DefaultEmbeddingModels()
	for _, entry := range defaults {
		if entry == nil {
			continue
		}
		if entry.DownloadStatus == "" {
			entry.DownloadStatus = "unknown"
		}
		if err := s.configStore.UpsertEmbeddingModel(entry.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProjectService) ensureEmbeddingModelSnapshot(config *models.ProjectConfig) error {
	if config == nil {
		return fmt.Errorf("project config cannot be nil")
	}

	if strings.TrimSpace(config.EmbeddingModel) == "" {
		config.EmbeddingModel = defaultFastEmbedModelID
	}

	if _, ok := supportedEmbeddingModelIDs[strings.ToLower(config.EmbeddingModel)]; !ok {
		log.Printf("Embedding model %s unsupported, falling back to default", config.EmbeddingModel)
		config.EmbeddingModel = defaultFastEmbedModelID
	}

	if config.EmbeddingModelInfo != nil && config.EmbeddingModelInfo.ID == config.EmbeddingModel {
		return nil
	}

	meta, err := s.configStore.GetEmbeddingModel(config.EmbeddingModel)
	if err != nil {
		if config.EmbeddingModel != defaultFastEmbedModelID {
			config.EmbeddingModel = defaultFastEmbedModelID
			meta, err = s.configStore.GetEmbeddingModel(config.EmbeddingModel)
		}
		if err != nil {
			return fmt.Errorf("failed to resolve embedding model %s: %w", config.EmbeddingModel, err)
		}
	}

	if meta.Backend == "onnx" && !s.enableONNXRuntime {
		log.Printf("ONNX Runtime unavailable: %s cannot be used, falling back to %s", meta.ID, defaultFastEmbedModelID)
		config.EmbeddingModel = defaultFastEmbedModelID
		return s.ensureEmbeddingModelSnapshot(config)
	}

	refreshModelLocalStatus(meta)
	if err := s.configStore.UpsertEmbeddingModel(meta.Clone()); err != nil {
		return err
	}

	config.EmbeddingModelInfo = meta.Clone()
	config.EmbeddingBackend = meta.Backend
	return nil
}

func (s *ProjectService) getEmbeddingClient(project *models.Project) (embedding.EmbeddingClient, error) {
	if project.Config.EmbeddingModelInfo == nil {
		if err := s.ensureEmbeddingModelSnapshot(&project.Config); err != nil {
			return nil, err
		}
		if err := s.updateProjectMetadata(project); err != nil {
			log.Printf("Failed to persist embedding metadata for %s: %v", project.ID, err)
		}
	}

	meta := project.Config.EmbeddingModelInfo
	if meta == nil {
		return nil, fmt.Errorf("project %s missing embedding model", project.ID)
	}

	switch strings.ToLower(meta.Backend) {
	case "fastembed", "":
		if !s.enableONNXRuntime {
			return nil, fmt.Errorf("FastEmbed models require ONNX Runtime. Install the shared library, set its path in Settings → Projects, and restart CodeTextor to enable %s.", meta.ID)
		}

		// 1. Check if already cached OR if it's currently loading
		s.clientsMu.Lock()
		client, ok := s.embeddingClients[meta.ID]
		if ok {
			s.clientsMu.Unlock()
			log.Printf("GPU Cache: Reusing existing FastEmbed client for %s", meta.ID)
			return client, nil
		}

		waitChan, isLoading := s.loadingClients[meta.ID]
		if isLoading {
			// Another thread is loading this model. Wait for it.
			s.clientsMu.Unlock()
			log.Printf("GPU Cache: Waiting for concurrent FastEmbed initialization of %s...", meta.ID)
			<-waitChan

			// Try again, now it should be in cache
			return s.getEmbeddingClient(project)
		}

		// 2. Not cached and not loading, mark as "loading" and proceed
		waitChan = make(chan struct{})
		s.loadingClients[meta.ID] = waitChan
		s.clientsMu.Unlock()

		// Cleanup "loading" state and notify waiters
		defer func() {
			s.clientsMu.Lock()
			delete(s.loadingClients, meta.ID)
			close(waitChan)
			s.clientsMu.Unlock()
		}()

		log.Printf("GPU Cache: Initializing NEW FastEmbed client for %s (VRAM will increase)", meta.ID)
		newClient, err := embedding.NewFastEmbedClient(meta)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize FastEmbed client for %s: %w", meta.ID, err)
		}

		s.clientsMu.Lock()
		s.embeddingClients[meta.ID] = newClient
		s.clientsMu.Unlock()

		log.Printf("GPU Cache: Successfully initialized %s, triggering rebalance", meta.ID)
		s.rebalanceBatchSizes()
		return newClient, nil
	case "onnx":
		log.Printf("GPU Cache: Requesting ONNX client for %s", meta.ID)
		if !s.enableONNXRuntime {
			loggedONNXWarning.Do(func() {
				log.Printf("ONNX Runtime not detected: install the onnxruntime library, configure its path in Settings → Projects, and restart CodeTextor to enable FastEmbed/ONNX models.")
			})
			return nil, fmt.Errorf("ONNX Runtime not detected: set the shared library path in Settings → Projects and restart CodeTextor to enable %s", meta.ID)
		}

		if strings.TrimSpace(meta.LocalPath) == "" || !strings.EqualFold(meta.DownloadStatus, "ready") {
			updated, err := s.DownloadEmbeddingModel(meta.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to download ONNX model %s: %w", meta.ID, err)
			}
			meta = updated.Clone()
			project.Config.EmbeddingModelInfo = meta.Clone()
			if err := s.updateProjectMetadata(project); err != nil {
				log.Printf("Failed to update embedding metadata for %s: %v", project.ID, err)
			}
		}

		// 1. Check if already cached OR if it's currently loading
		s.clientsMu.Lock()
		client, ok := s.embeddingClients[meta.ID]
		if ok {
			if client.IsClosed() {
				log.Printf("GPU Cache: Detected closed zombie client for %s, removing from cache", meta.ID)
				delete(s.embeddingClients, meta.ID)
			} else {
				s.clientsMu.Unlock()
				log.Printf("GPU Cache: Reusing existing ONNX client for %s", meta.ID)
				return client, nil
			}
		}

		waitChan, isLoading := s.loadingClients[meta.ID]
		if isLoading {
			// Another thread is loading this model. Wait for it with a timeout.
			s.clientsMu.Unlock()
			log.Printf("GPU Cache: Waiting for concurrent ONNX initialization of %s...", meta.ID)
			
			select {
			case <-waitChan:
				// Success, continue below
			case <-time.After(60 * time.Second):
				log.Printf("GPU Cache: Timeout waiting for ONNX initialization of %s", meta.ID)
				return nil, fmt.Errorf("timeout waiting for embedding model initialization")
			}

			// Try again, now it should be in cache
			return s.getEmbeddingClient(project)
		}

		// 2. Not cached and not loading, mark as "loading" and proceed
		waitChan = make(chan struct{})
		s.loadingClients[meta.ID] = waitChan
		s.clientsMu.Unlock()

		// Cleanup "loading" state and notify waiters
		defer func() {
			s.clientsMu.Lock()
			delete(s.loadingClients, meta.ID)
			close(waitChan)
			s.clientsMu.Unlock()
		}()

		log.Printf("GPU Cache: Initializing NEW ONNX client for %s (VRAM will increase)", meta.ID)
		newClient, err := embedding.NewONNXEmbeddingClient(meta)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX model %s: %w", meta.ID, err)
		}
		s.clientsMu.Lock()
		s.embeddingClients[meta.ID] = newClient
		s.clientsMu.Unlock()

		s.rebalanceBatchSizes()
		return newClient, nil
	default:
		return nil, fmt.Errorf("embedding backend %s is not supported", meta.Backend)
	}
}

func refreshModelLocalStatus(meta *models.EmbeddingModelInfo) {
	if meta == nil {
		return
	}
	if strings.EqualFold(meta.Backend, "fastembed") {
		dir, err := embedding.ResolveFastEmbedDir(meta)
		if err != nil {
			meta.DownloadStatus = "pending"
			return
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			meta.LocalPath = dir
			meta.DownloadStatus = "ready"
		} else if meta.DownloadStatus == "" {
			meta.DownloadStatus = "pending"
		} else {
			meta.DownloadStatus = "missing"
		}
		return
	}
	targetPath := strings.TrimSpace(meta.LocalPath)
	if targetPath == "" {
		if resolved, err := embedding.ResolveModelPath(meta); err == nil {
			targetPath = resolved
		}
	}
	if targetPath == "" {
		if meta.DownloadStatus == "" {
			meta.DownloadStatus = "pending"
		}
		return
	}
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		meta.LocalPath = targetPath
		meta.DownloadStatus = "ready"
		// Ensure tokenizer path is also refreshed if missing
		if strings.TrimSpace(meta.TokenizerLocalPath) == "" {
			tokenizerPath := filepath.Join(filepath.Dir(targetPath), "tokenizer.json")
			if tInfo, tErr := os.Stat(tokenizerPath); tErr == nil && !tInfo.IsDir() {
				meta.TokenizerLocalPath = tokenizerPath
			}
		}
	} else if meta.DownloadStatus == "" {
		meta.DownloadStatus = "pending"
	} else {
		meta.DownloadStatus = "missing"
	}
}

func (s *ProjectService) makeDownloadProgressEmitter() embedding.DownloadProgressCallback {
	if s.eventEmitter == nil {
		return nil
	}
	return func(update embedding.DownloadProgress) {
		s.eventEmitter("embedding:download-progress", update)
	}
}

// ListEmbeddingModels returns the catalog entries stored in the config DB.
func (s *ProjectService) ListEmbeddingModels() ([]*models.EmbeddingModelInfo, error) {
	entries, err := s.configStore.ListEmbeddingModels()
	if err != nil {
		return nil, err
	}

	result := make([]*models.EmbeddingModelInfo, 0, len(entries))
	for _, entry := range entries {
		refreshModelLocalStatus(entry)
		result = append(result, entry.Clone())
	}
	return result, nil
}

// SaveEmbeddingModel creates or updates a catalog entry (used by the frontend modal).
func (s *ProjectService) SaveEmbeddingModel(model models.EmbeddingModelInfo) (*models.EmbeddingModelInfo, error) {
	sanitized := model.Clone()
	if sanitized == nil {
		return nil, fmt.Errorf("embedding model payload cannot be empty")
	}

	sanitized.ID = strings.TrimSpace(sanitized.ID)
	if sanitized.ID == "" {
		sanitized.ID = utils.GenerateSlug(sanitized.DisplayName)
	}
	if sanitized.ID == "" {
		return nil, fmt.Errorf("embedding model id cannot be empty")
	}
	if sanitized.Dimension <= 0 {
		return nil, fmt.Errorf("embedding model dimension must be greater than zero")
	}
	if sanitized.SourceType == "" {
		sanitized.SourceType = "custom"
	}
	if sanitized.DownloadStatus == "" {
		sanitized.DownloadStatus = "unknown"
	}
	if sanitized.DisplayName == "" {
		sanitized.DisplayName = sanitized.ID
	}
	if sanitized.DownloadStatus == "" {
		sanitized.DownloadStatus = "pending"
	}

	if err := s.configStore.UpsertEmbeddingModel(sanitized.Clone()); err != nil {
		return nil, err
	}

	return sanitized, nil
}

// DownloadEmbeddingModel ensures the specified model is downloaded locally.
func (s *ProjectService) DownloadEmbeddingModel(modelID string) (*models.EmbeddingModelInfo, error) {
	meta, err := s.configStore.GetEmbeddingModel(modelID)
	if err != nil {
		return nil, err
	}

	metaClone := meta.Clone()
	metaClone.DownloadStatus = "downloading"
	// Force redownload by clearing local paths if we are explicitly calling DownloadEmbeddingModel
	metaClone.LocalPath = ""
	metaClone.TokenizerLocalPath = ""

	if err := s.configStore.UpsertEmbeddingModel(metaClone); err != nil {
		return nil, err
	}

	updated, err := s.modelDownloader.EnsureLocal(metaClone, s.makeDownloadProgressEmitter())
	if err != nil {
		metaClone.DownloadStatus = "error"
		metaClone.Notes = strings.TrimSpace(fmt.Sprintf("%s\nDownload error: %v", metaClone.Notes, err))
		_ = s.configStore.UpsertEmbeddingModel(metaClone)
		return nil, err
	}

	cloned := updated.Clone()
	if err := s.configStore.UpsertEmbeddingModel(cloned); err != nil {
		return nil, err
	}

	s.clientsMu.Lock()
	if client, ok := s.embeddingClients[modelID]; ok {
		client.Close()
		delete(s.embeddingClients, modelID)
	}
	s.clientsMu.Unlock()

	return cloned, nil
}

// DownloadONNXRuntime downloads and installs the ONNX Runtime library.
func (s *ProjectService) DownloadONNXRuntime() error {
	log.Printf("[ProjectService] Starting ONNX Runtime download...")
	progressCb := func(p embedding.DownloadProgress) {
		if s.eventEmitter != nil {
			s.eventEmitter("runtime_download_progress", p)
		}
	}

	if err := s.modelDownloader.DownloadONNXRuntime(progressCb); err != nil {
		log.Printf("[ProjectService] Error downloading ONNX Runtime: %v", err)
		return err
	}

	log.Printf("[ProjectService] ONNX Runtime download and extraction completed successfully.")

	// Update the path in settings to the newly downloaded location
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return err
	}

	var libName string
	switch stdruntime.GOOS {
	case "windows":
		libName = "onnxruntime.dll"
	case "darwin":
		libName = "libonnxruntime.1.24.4.dylib"
	default:
		libName = "libonnxruntime.so.1.24.4"
	}

	newPath := filepath.Join(configDir, "bin", libName)
	_, err = s.UpdateONNXRuntimeSettings(newPath)
	return err
}

// Search performs a semantic search across a project's index.
func (s *ProjectService) Search(projectID string, query string, k int) (*models.SearchResponse, error) {
	start := time.Now()
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	client, err := s.getEmbeddingClient(project)
	if err != nil {
		return nil, err
	}

	vecs, err := client.GenerateEmbeddings([]string{trimmed})
	if err != nil || len(vecs) == 0 {
		if err != nil {
			return nil, fmt.Errorf("failed to embed query: %w", err)
		}
		return nil, fmt.Errorf("embedding client returned no vector")
	}

	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	results, err := vectorStore.SearchSimilarChunks(vecs[0], project.Config.EmbeddingModelInfo.ID, k)
	if err != nil {
		return nil, err
	}

	for _, c := range results {
		c.ProjectID = projectID
		// Drop embeddings to avoid large payloads but keep a non-nil slice so MCP schema validation
		// (expects an array) does not see a null value.
		c.Embedding = []float32{}
	}

	resp := &models.SearchResponse{
		Chunks:       results,
		TotalResults: len(results),
		QueryTimeMs:  time.Since(start).Milliseconds(),
	}
	return resp, nil
}

// GetRecentChanges returns both recently indexed files and current VCS modifications.
func (s *ProjectService) GetRecentChanges(projectID string, limit int) (*models.RecentChangesResponse, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	res := &models.RecentChangesResponse{
		Indexed:     make([]models.RecentIndexedFile, 0),
		WorkingCopy: make([]models.VCSFile, 0),
	}

	// 1. Get indexed files from DB
	vs, err := s.GetVectorStore(projectID)
	if err == nil {
		dbFiles, err := vs.GetRecentFiles(limit)
		if err == nil {
			for _, f := range dbFiles {
				res.Indexed = append(res.Indexed, models.RecentIndexedFile{
					Path: f.Path,
					Time: time.Unix(f.UpdatedAt, 0).Format(time.RFC3339),
				})
			}
		}
	}

	// 2. Detect VCS and get working copy changes
	root := project.Config.RootPath
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		res.VCSType = "git"
		s.fillGitChanges(root, res)
	} else if _, err := os.Stat(filepath.Join(root, ".svn")); err == nil {
		res.VCSType = "svn"
		s.fillSvnChanges(root, res)
	}

	return res, nil
}

func (s *ProjectService) fillGitChanges(root string, res *models.RecentChangesResponse) {
	cmd := exec.Command("git", "status", "--porcelain")
	utils.SetHideWindow(cmd)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		if path != "" {
			res.WorkingCopy = append(res.WorkingCopy, models.VCSFile{
				Path:   path,
				Status: status,
			})
		}
	}
}

func (s *ProjectService) fillSvnChanges(root string, res *models.RecentChangesResponse) {
	cmd := exec.Command("svn", "status")
	utils.SetHideWindow(cmd)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 8 {
			continue
		}
		status := string(line[0])
		path := strings.TrimSpace(line[8:])
		if path != "" {
			res.WorkingCopy = append(res.WorkingCopy, models.VCSFile{
				Path:   path,
				Status: status,
			})
		}
	}
}

// GetEmbeddingCapabilities reports which embedding backends are currently available.
func (s *ProjectService) GetEmbeddingCapabilities() (*models.EmbeddingCapabilities, error) {
	return &models.EmbeddingCapabilities{
		OnnxRuntimeAvailable:    s.enableONNXRuntime,
		ActiveExecutionProvider: embedding.GetActiveExecutionProvider(),
	}, nil
}

// GetONNXRuntimeSettings returns the persisted runtime path plus current status.
func (s *ProjectService) GetONNXRuntimeSettings() (*models.ONNXRuntimeSettings, error) {
	return s.buildONNXRuntimeSettings(), nil
}

// UpdateONNXRuntimeSettings saves the ONNX runtime path for future startups.
func (s *ProjectService) UpdateONNXRuntimeSettings(path string) (*models.ONNXRuntimeSettings, error) {
	sanitized := strings.TrimSpace(path)
	if sanitized == "" {
		if err := s.configStore.DeleteValue(onnxRuntimePathKey); err != nil {
			return nil, err
		}
	} else {
		if err := s.configStore.SetValue(onnxRuntimePathKey, sanitized); err != nil {
			return nil, err
		}
	}
	s.onnxRuntimePath = sanitized
	return s.buildONNXRuntimeSettings(), nil
}

// TestONNXRuntimePath performs a lightweight validation of the provided path.
func (s *ProjectService) TestONNXRuntimePath(path string) (*models.ONNXRuntimeTestResult, error) {
	sanitized := strings.TrimSpace(path)
	if sanitized == "" {
		return &models.ONNXRuntimeTestResult{
			Success: false,
			Message: "Please provide a path to the ONNX runtime library.",
		}, nil
	}
	info, err := os.Stat(sanitized)
	if err != nil {
		return &models.ONNXRuntimeTestResult{
			Success: false,
			Message: "Unable to access the provided path.",
			Error:   err.Error(),
		}, nil
	}
	if info.IsDir() {
		ext := ".so"
		if stdruntime.GOOS == "windows" {
			ext = ".dll"
		} else if stdruntime.GOOS == "darwin" {
			ext = ".dylib"
		}
		return &models.ONNXRuntimeTestResult{
			Success: false,
			Message: fmt.Sprintf("The provided path points to a directory. Select the shared library file (e.g., libonnxruntime%s or onnxruntime%s).", ext, ext),
		}, nil
	}

	result := &models.ONNXRuntimeTestResult{
		Success: true,
		Message: "Library found. Save and restart CodeTextor to apply this path.",
	}
	// If runtime is already initialized, remind about restart.
	if s.enableONNXRuntime && s.activeONNXPath != "" && !strings.EqualFold(s.activeONNXPath, sanitized) {
		result.Message = "Library found. Restart CodeTextor to switch to this path."
	}
	return result, nil
}

func (s *ProjectService) buildONNXRuntimeSettings() *models.ONNXRuntimeSettings {
	expected := strings.TrimSpace(s.onnxRuntimePath)
	active := strings.TrimSpace(s.activeONNXPath)
	return &models.ONNXRuntimeSettings{
		SharedLibraryPath:       expected,
		ActivePath:              active,
		RuntimeAvailable:        s.enableONNXRuntime,
		RequiresRestart:         !strings.EqualFold(expected, active),
		ActiveExecutionProvider: embedding.GetActiveExecutionProvider(),
	}
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
	result.UseGitIgnore = override.UseGitIgnore
	if override.ChunkSizeMin != 0 {
		result.ChunkSizeMin = override.ChunkSizeMin
	}
	if override.ChunkSizeMax != 0 {
		result.ChunkSizeMax = override.ChunkSizeMax
	}
	if override.EmbeddingModel != "" {
		result.EmbeddingModel = override.EmbeddingModel
		if override.EmbeddingModelInfo != nil {
			result.EmbeddingModelInfo = override.EmbeddingModelInfo
		}
	} else if override.EmbeddingModelInfo != nil {
		result.EmbeddingModelInfo = override.EmbeddingModelInfo
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

	// Merge gitignore patterns if enabled
	if finalConfig.UseGitIgnore {
		if giPatterns, err := s.GetGitIgnorePatterns(projectID); err == nil {
			finalConfig.ExcludePatterns = append(finalConfig.ExcludePatterns, giPatterns...)
		}
	}

	includePaths := resolveIncludePaths(finalConfig.RootPath, finalConfig.IncludePaths)

	var previews []*models.FilePreview
	seenFiles := make(map[string]bool)
	extensionSet := make(map[string]struct{})
	
	// We ignore the saved FileExtensions from the database to ensure we always 
	// use the latest supported extensions from the TOML configurations.
	exts := s.GetSupportedExtensions()
	
	for _, ext := range exts {
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

			if utils.ShouldSkipPath(finalConfig.RootPath, path, finalConfig.ExcludePatterns, finalConfig.AutoExcludeHidden) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
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

			isHidden := strings.HasPrefix(d.Name(), ".") && len(d.Name()) > 1
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

// GetProjectStructure returns a richer structure of files and directories with semantic metadata.
// It uses Depth to limit recursion and fetches symbol/line statistics from the database.
func (s *ProjectService) GetProjectStructure(projectID string, subPath string, depth int) ([]*models.FilePreview, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	finalConfig := project.Config
	if finalConfig.RootPath == "" {
		finalConfig.RootPath = project.Config.RootPath
	}

	// Merge gitignore patterns if enabled
	if finalConfig.UseGitIgnore {
		if giPatterns, err := s.GetGitIgnorePatterns(projectID); err == nil {
			finalConfig.ExcludePatterns = append(finalConfig.ExcludePatterns, giPatterns...)
		}
	}

	root := finalConfig.RootPath
	cleanSub := filepath.ToSlash(filepath.Clean(subPath))
	if cleanSub == "." || cleanSub == "/" {
		cleanSub = ""
	}
	
	absStart := filepath.Join(root, cleanSub)

	var entries []*models.FilePreview
	dirEntries := make(map[string]*models.FilePreview)
	var filePaths []string

	err = filepath.WalkDir(absStart, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == absStart {
			return nil
		}

		// Calculate relative path to the start directory and project root
		relToStart, _ := filepath.Rel(absStart, path)
		relToStart = filepath.ToSlash(relToStart)
		
		relToRoot, _ := filepath.Rel(root, path)
		relToRoot = filepath.ToSlash(relToRoot)

		// Calculate depth relative to subPath
		currentDepth := len(strings.Split(relToStart, "/"))

		// Skip if excluded
		if utils.ShouldSkipPath(root, path, finalConfig.ExcludePatterns, finalConfig.AutoExcludeHidden) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Handle depth limit
		if depth > 0 && currentDepth > depth {
			// Increment item count for parent if it's within the visible tree
			parentRel := filepath.ToSlash(filepath.Dir(relToRoot))
			if p, ok := dirEntries[parentRel]; ok {
				p.ItemCount++
			}

			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		isHidden := strings.HasPrefix(d.Name(), ".") && len(d.Name()) > 1
		info, _ := d.Info()
		
		preview := &models.FilePreview{
			AbsolutePath: path,
			RelativePath: relToRoot,
			IsDir:        d.IsDir(),
			Hidden:       isHidden,
		}
		
		if info != nil {
			preview.LastModified = info.ModTime().Unix()
			if !d.IsDir() {
				preview.Size = utils.FormatBytes(info.Size())
				preview.Extension = filepath.Ext(d.Name())
				filePaths = append(filePaths, relToRoot)
			}
		}

		entries = append(entries, preview)
		if d.IsDir() {
			dirEntries[relToRoot] = preview
		}
		
		// If it's a child of a visible directory, increment parent's item count
		parentRel := filepath.ToSlash(filepath.Dir(relToRoot))
		if p, ok := dirEntries[parentRel]; ok {
			p.ItemCount++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Fetch semantic stats from DB for files in batch
	if len(filePaths) > 0 {
		vs, err := s.GetVectorStore(projectID)
		if err == nil {
			stats, err := vs.GetFileSemanticStats(filePaths)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir {
						if s, ok := stats[entry.RelativePath]; ok {
							entry.Lines = s.Lines
							entry.Symbols = s.Symbols
							entry.Languages = s.Languages
						}
					}
				}
			}
		}
	}

	return entries, nil
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
		outline, err = s.buildAndStoreOutline(project, absPath, key, vectorStore)
		if err != nil {
			return nil, err
		}
	}

	if outline == nil {
		outline = []*models.OutlineNode{}
	}

	return outline, nil
}

// FindReferences finds all locations where a symbol is used.
// It supports resolution by nodeID or symbolName + optional path.
func (s *ProjectService) FindReferences(projectID, nodeID, symbolName, path string) (*models.SymbolReferencesResponse, error) {
	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	var candidates []*models.OutlineNode

	if nodeID != "" {
		nodes, err := vs.GetOutlineNodes([]string{nodeID})
		if err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			return nil, fmt.Errorf("node with ID '%s' not found", nodeID)
		}
		candidates = nodes
	} else if symbolName != "" {
		candidates, err = vs.FindSymbolNodesByName(symbolName)
		if err != nil {
			return nil, err
		}
		
		if len(candidates) == 0 {
			return nil, fmt.Errorf("symbol '%s' not found", symbolName)
		}
		
		// If path is provided, filter candidates strictly
		if path != "" {
			var filtered []*models.OutlineNode
			for _, c := range candidates {
				if strings.Contains(c.FilePath, path) {
					filtered = append(filtered, c)
				}
			}
			if len(filtered) > 0 {
				candidates = filtered
			}
		}
	} else {
		return nil, fmt.Errorf("either nodeID or symbolName must be provided")
	}

	response := &models.SymbolReferencesResponse{
		Targets: make([]models.TargetUsages, 0, len(candidates)),
	}

	// Cache for file lines to avoid re-reading for multiple references in same file
	fileLinesCache := make(map[string][]string)

	for _, candidate := range candidates {
		usages, err := vs.GetSymbolUsages(candidate.ID)
		if err != nil {
			continue // Skip on error, though it shouldn't happen often
		}

		// Tabular format for this target: [File, Line, Caller, Kind, Content]
		results := [][]any{
			{"File", "Line", "Caller", "Kind", "Content"},
		}

		for _, u := range usages {
			lines, ok := fileLinesCache[u.FilePath]
			if !ok {
				content, err := s.ReadFileContent(projectID, u.FilePath)
				if err == nil {
					lines = strings.Split(content, "\n")
					fileLinesCache[u.FilePath] = lines
				} else {
					fileLinesCache[u.FilePath] = nil // Mark as failed
				}
			}

			contentSnippet := ""
			if lines != nil && u.Line > 0 && u.Line <= len(lines) {
				contentSnippet = strings.TrimSpace(lines[u.Line-1])
			}

			caller := u.CallerName
			kind := u.CallerKind
			if caller == "" {
				caller = "root"
				kind = "file"
			}

			results = append(results, []any{
				u.FilePath,
				u.Line,
				caller,
				kind,
				contentSnippet,
			})
		}

		response.Targets = append(response.Targets, models.TargetUsages{
			TargetID:   candidate.ID,
			TargetPath: candidate.FilePath,
			TargetLine: int(candidate.StartLine),
			Results:    results,
		})
	}

	return response, nil
}

// FindImplementations searches the index for classes or interfaces that implement the given interface nodeID.
func (s *ProjectService) FindImplementations(projectID, nodeID string) (*models.FindImplementationsResponse, error) {
	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	nodes, err := vs.GetOutlineNodes([]string{nodeID})
	if err != nil {
		return nil, fmt.Errorf("failed to get target node: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("symbol node '%s' not found", nodeID)
	}

	targetNode := nodes[0]
	interfaceName := targetNode.Name

	impls, err := vs.GetSymbolImplementations(interfaceName)
	if err != nil {
		return nil, err
	}

	response := &models.FindImplementationsResponse{
		Implementations: impls,
	}

	// Go uses implicit interfaces, which we cannot reliably resolve statically via explicit statements.
	if strings.HasSuffix(targetNode.FilePath, ".go") || strings.HasSuffix(targetNode.FilePath, ".mod") {
		response.Warning = "Go uses implicit interfaces which are evaluated at compile time. This static analysis tool relies on explicit declarations (like 'implements' in TypeScript/PHP/Java) and currently cannot detect Go implementations. Use Find References on the interface methods instead."
	}

	return response, nil
}

// GetCallGraph returns the call relationships for a specific function.
func (s *ProjectService) GetCallGraph(projectID, nodeID, symbolName, path string, direction string, depth int) (*models.CallGraphResponse, error) {
	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	targetID := nodeID
	
	// Reuse identification logic
	if targetID == "" && symbolName != "" {
		candidates, err := vs.FindSymbolNodesByName(symbolName)
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("symbol '%s' not found", symbolName)
		}
		if path != "" {
			for _, c := range candidates {
				if strings.Contains(c.FilePath, path) {
					targetID = c.ID
					break
				}
			}
		}
		if targetID == "" {
			if len(candidates) > 1 {
				return nil, fmt.Errorf("symbol '%s' is ambiguous, please provide a path", symbolName)
			}
			targetID = candidates[0].ID
		}
	}

	if targetID == "" {
		return nil, fmt.Errorf("target symbol not identified")
	}

	// Preparation
	fileCache := make(map[string][]string)
	getLineContent := func(projectID, filePath string, line int) string {
		lines, ok := fileCache[filePath]
		if !ok {
			content, err := s.ReadFileContent(projectID, filePath)
			if err != nil {
				return ""
			}
			lines = strings.Split(content, "\n")
			fileCache[filePath] = lines
		}
		if line > 0 && line <= len(lines) {
			return strings.TrimSpace(lines[line-1])
		}
		return ""
	}

	getNodeLoc := func(nodeID string) (string, string, int) {
		nodes, _ := vs.GetOutlineNodes([]string{nodeID})
		if len(nodes) > 0 {
			n := nodes[0]
			return n.Name, n.FilePath, int(n.StartLine)
		}
		return "unknown", "external", 0
	}

	rootName, rootPath, rootLine := getNodeLoc(targetID)
	resp := &models.CallGraphResponse{
		Direction: direction,
		Root: models.CallDetails{
			Symbol:   rootName,
			Location: fmt.Sprintf("%s:%d", rootPath, rootLine),
		},
	}

	visited := make(map[string]bool)
	var buildTree func(nodeID string, d int) []models.CallDetails
	buildTree = func(nodeID string, d int) []models.CallDetails {
		if d <= 0 || visited[nodeID] {
			return nil
		}
		visited[nodeID] = true
		defer func() { visited[nodeID] = false }() // For tree structure, we might want to revisit in different branches? 
		// Actually, to avoid infinite trees, we keep visited per branch or globally. 
		// For a call graph tree, global visited is safer to avoid OOM on cycles.

		var calls []models.CallDetails

		if direction == "outgoing" || direction == "both" {
			usages, _ := vs.GetOutgoingCalls(nodeID)
			for _, u := range usages {
				name, path, line := getNodeLoc(u.TargetNodeID)
				call := models.CallDetails{
					Symbol:   name,
					Location: fmt.Sprintf("%s:%d", path, line),
					Content:  getLineContent(projectID, path, u.Line), // Note: snippet is from caller file if outgoing? 
					// Wait, snippet should be where the call happens.
				}
				
				// Snippet for outgoing call is in the CURRENT node's file
				_, callerPath, _ := getNodeLoc(nodeID)
				call.Content = getLineContent(projectID, callerPath, u.Line)
				
				call.Calls = buildTree(u.TargetNodeID, d-1)
				calls = append(calls, call)
			}
		}

		if direction == "incoming" || direction == "both" {
			usages, _ := vs.GetSymbolUsages(nodeID)
			for _, u := range usages {
				if u.CallerNodeID == "" {
					continue
				}
				name, path, line := getNodeLoc(u.CallerNodeID)
				call := models.CallDetails{
					Symbol:   name,
					Location: fmt.Sprintf("%s:%d", path, line),
					Content:  getLineContent(projectID, path, u.Line),
					Calls:    buildTree(u.CallerNodeID, d-1) ,
				}
				calls = append(calls, call)
			}
		}

		return calls
	}

	resp.Root.Calls = buildTree(targetID, depth)

	return resp, nil
}

// buildAndStoreOutline parses the file to generate and persist an outline when none exists yet.
func (s *ProjectService) buildAndStoreOutline(
	project *models.Project,
	absPath string,
	storageKey string,
	vectorStore *store.VectorStore,
) ([]*models.OutlineNode, error) {
	if project == nil {
		return nil, fmt.Errorf("project is required to build outline")
	}

	chunkConfig := chunker.ChunkConfig{
		MaxChunkSize:      project.Config.ChunkSizeMax,
		MinChunkSize:      project.Config.ChunkSizeMin,
		CollapseThreshold: 500,
		MergeSmallChunks:  true,
		IncludeComments:   true,
	}
	parser := chunker.NewParser(chunkConfig)
	if !parser.IsSupported(absPath) {
		return nil, fmt.Errorf("outline is not supported for %s", storageKey)
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", storageKey, err)
	}

	result, err := parser.ParseFile(absPath, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse outline for %s: %w", storageKey, err)
	}

	nodes, _ := outline.BuildOutlineNodes(storageKey, result.Symbols)
	if err := vectorStore.UpsertFileOutline(storageKey, nodes); err != nil {
		return nil, fmt.Errorf("failed to persist outline for %s: %w", storageKey, err)
	}

	return nodes, nil
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

// GetChunkByID retrieves a single chunk using its identifier.
func (s *ProjectService) GetChunkByID(projectID, chunkID string) (*models.Chunk, error) {
	chunks, err := s.GetChunksByIDFuzzy(projectID, chunkID)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("chunk not found: %s", chunkID)
	}
	// Return the first one for existing single-chunk API compatibility
	return chunks[0], nil
}

// GetChunksByIDFuzzy retrieves one or more chunks using an exact ID match or a fuzzy fallback
// based on the components of a "talking" ID (path|lines|name).
func (s *ProjectService) GetChunksByIDFuzzy(projectID, chunkID string) ([]*models.Chunk, error) {
	if strings.TrimSpace(chunkID) == "" {
		return nil, fmt.Errorf("chunk id cannot be empty")
	}

	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	vectorStore, err := s.GetVectorStore(project.ID)
	if err != nil {
		return nil, err
	}

	// 1. Try exact match first
	chunk, err := vectorStore.GetChunkByID(chunkID)
	if err == nil {
		chunk.ProjectID = project.ID
		chunk.Embedding = []float32{}
		return []*models.Chunk{chunk}, nil
	}

	// 2. Fallback to fuzzy parsing
	filePath, startLine, endLine, symbolName := parseTalkingID(chunkID)
	if filePath == "" {
		return nil, fmt.Errorf("chunk not found and ID format is not parsable: %s", chunkID)
	}

	fuzzyResults, err := vectorStore.FindChunksFuzzy(filePath, startLine, endLine, symbolName)
	if err != nil {
		return nil, err
	}

	// If still nothing found, but we have a file path, maybe the file doesn't exist or is not indexed
	if len(fuzzyResults) == 0 {
		return nil, fmt.Errorf("no chunks found for ID '%s' (fuzzy: path=%s, lines=%d-%d, name=%s)", 
			chunkID, filePath, startLine, endLine, symbolName)
	}

	for _, c := range fuzzyResults {
		c.ProjectID = project.ID
		c.Embedding = []float32{}
	}

	return fuzzyResults, nil
}

// parseTalkingID attempts to extract components from a "talking" ID.
// Format: path|Lstart-Lend|name or path|Lstart|name or path|name
func parseTalkingID(id string) (path string, start, end int, name string) {
	parts := strings.Split(id, "|")
	if len(parts) < 2 {
		return "", 0, 0, ""
	}

	path = parts[0]
	
	// Check if the second part is a line range (starts with 'L')
	idx := 1
	if strings.HasPrefix(parts[idx], "L") {
		linePart := parts[idx][1:] // Remove 'L'
		if strings.Contains(linePart, "-") {
			rangeParts := strings.Split(linePart, "-")
			fmt.Sscanf(rangeParts[0], "%d", &start)
			fmt.Sscanf(rangeParts[1], "%d", &end)
		} else {
			fmt.Sscanf(linePart, "%d", &start)
			end = start
		}
		idx++
	}

	// The remaining part (if any) is the symbol name
	if idx < len(parts) {
		name = parts[idx]
	}

	return path, start, end, name
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

	resolveModelInfo := func(modelID string) *models.EmbeddingModelInfo {
		trimmed := strings.TrimSpace(modelID)
		if trimmed == "" {
			return nil
		}
		if project.Config.EmbeddingModelInfo != nil && strings.EqualFold(project.Config.EmbeddingModelInfo.ID, trimmed) {
			return project.Config.EmbeddingModelInfo.Clone()
		}
		if meta, err := s.configStore.GetEmbeddingModel(trimmed); err == nil && meta != nil {
			return meta.Clone()
		}
		return &models.EmbeddingModelInfo{
			ID:          trimmed,
			DisplayName: trimmed,
		}
	}

	for idx := range stats.EmbeddingModels {
		stats.EmbeddingModels[idx].ModelInfo = resolveModelInfo(stats.EmbeddingModels[idx].ModelID)
	}

	if len(stats.EmbeddingModels) > 0 {
		if stats.EmbeddingModels[0].ModelInfo != nil {
			stats.LastEmbeddingModel = stats.EmbeddingModels[0].ModelInfo.Clone()
		} else if stats.EmbeddingModels[0].ModelID != "" {
			stats.LastEmbeddingModel = &models.EmbeddingModelInfo{
				ID:          stats.EmbeddingModels[0].ModelID,
				DisplayName: stats.EmbeddingModels[0].ModelID,
			}
		}
	}

	return stats, nil
}

// GetProjectSummary returns structural information about the project.
func (s *ProjectService) GetProjectSummary(projectID string) (*models.ProjectSummary, error) {
	vectorStore, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vector store: %w", err)
	}

	return vectorStore.GetProjectSummary()
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

	if latestIndexTime != nil {
		cumulativeStats.LastIndexedAt = latestIndexTime
		cumulativeStats.LastIndexedAtUnix = latestIndexTime.Unix()
	}

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

// GetSupportedExtensions returns a list of all file extensions supported by the system's parsers.
func (s *ProjectService) GetSupportedExtensions() []string {
	s.extensionsMu.Lock()
	defer s.extensionsMu.Unlock()

	// Return cached version if available to avoid re-initializing the parser (and spamming logs)
	if len(s.extensionsCache) > 0 {
		return s.extensionsCache
	}

	// We can create a temporary parser to get extensions, they are static and embedded.
	p := chunker.NewParser(chunker.ChunkConfig{})
	s.extensionsCache = p.GetSupportedExtensions()
	return s.extensionsCache
}

// GetIndexedFiles returns detailed metadata for all files indexed in a project.
func (s *ProjectService) GetIndexedFiles(projectID string) ([]*models.IndexedFile, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	paths, err := vs.ListPhysicalFilePaths()
	if err != nil {
		return nil, err
	}

	stats, err := vs.GetFileSemanticStats(paths)
	if err != nil {
		return nil, err
	}

	// Load recent files metadata (size, mod time)
	recentFiles, err := vs.GetRecentFiles(len(paths))
	if err != nil {
		return nil, err
	}
	recentMap := make(map[string]*models.File)
	for _, f := range recentFiles {
		recentMap[f.Path] = f
	}

	results := make([]*models.IndexedFile, 0, len(paths))
	for _, p := range paths {
		fStats := stats[p]
		fileMeta := recentMap[p]
		linesCount := fStats.Lines
		sizeStr := "—"
		var modTime int64
		
		absPath := p
		if !filepath.IsAbs(p) {
			absPath = filepath.Join(project.Config.RootPath, p)
		}

		// If metadata is missing or clearly wrong, try to fill it from disk
		if fileMeta != nil {
			modTime = fileMeta.LastModified
			
			if modTime == 0 || fileMeta.SizeBytes == 0 || linesCount <= 0 {
				if info, err := os.Stat(absPath); err == nil {
					if modTime == 0 {
						modTime = info.ModTime().Unix()
					}
					if fileMeta.SizeBytes == 0 {
						sizeStr = utils.FormatBytes(info.Size())
					}
					if linesCount <= 0 {
						// Fallback: count lines from file
						if content, err := os.ReadFile(absPath); err == nil {
							linesCount = strings.Count(string(content), "\n") + 1
						}
					}
				} else {
					log.Printf("[DEBUG] Dashboard Stat failed for %s (Root: %s): %v", p, project.Config.RootPath, err)
				}
			}

			// If we still don't have a size string, use the stored SizeBytes or fallback to ChunkCount
			if sizeStr == "—" {
				if fileMeta.SizeBytes > 0 {
					sizeStr = utils.FormatBytes(fileMeta.SizeBytes)
				} else if fileMeta.ChunkCount > 0 {
					sizeStr = utils.FormatBytes(int64(fileMeta.ChunkCount * 500))
				} else {
					sizeStr = "0 B"
				}
			}
		} else {
			// No record in DB yet, but let's at least show disk info if we can
			if info, err := os.Stat(absPath); err == nil {
				modTime = info.ModTime().Unix()
				sizeStr = utils.FormatBytes(info.Size())
				if content, err := os.ReadFile(absPath); err == nil {
					linesCount = strings.Count(string(content), "\n") + 1
				}
			}
		}

		results = append(results, &models.IndexedFile{
			Path:         p,
			Symbols:      fStats.Symbols,
			Languages:    fStats.Languages,
			Lines:        linesCount,
			LastModified: modTime,
			Size:         sizeStr,
		})
	}

	return results, nil
}

// FindTodos retrieves all TODO comments from the project's index.
func (s *ProjectService) FindTodos(projectID string) (*models.FindTodosResponse, error) {
	vs, err := s.GetVectorStore(projectID)
	if err != nil {
		return nil, err
	}

	categories, err := vs.GetTodos()
	if err != nil {
		return nil, err
	}

	stats := models.TodoStats{
		ByCategory: make(map[string]int),
	}

	for cat, ids := range categories {
		count := len(ids)
		stats.Total += count
		stats.ByCategory[cat] = count
	}

	return &models.FindTodosResponse{
		Categories: categories,
		Stats:      stats,
	}, nil
}

func (s *ProjectService) Close() error {
	var firstErr error
	s.mu.Lock()
	// Stop all active indexers first
	for projectID := range s.vectorStores {
		s.indexerManager.StopIndexer(projectID)
	}
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
	s.clientsMu.Lock()
	for id, client := range s.embeddingClients {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(s.embeddingClients, id)
	}
	s.clientsMu.Unlock()
	return firstErr
}

// GrepSearch performs a literal or regex search across project files.
func (s *ProjectService) GrepSearch(projectID string, query string, isRegex bool, subPath string, limit int) (*models.GrepSearchResponse, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	root := project.Config.RootPath
	searchPath := root
	if subPath != "" {
		absSubPath := filepath.Join(root, subPath)
		if isPathWithinRoot(root, absSubPath) {
			searchPath = absSubPath
		}
	}

	var re *regexp.Regexp
	if isRegex {
		re, err = regexp.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	res := &models.GrepSearchResponse{
		Results: [][]any{{"File", "Line", "Content"}},
	}
	startTime := time.Now()

	patterns := project.Config.ExcludePatterns
	if project.Config.UseGitIgnore {
		if giPatterns, err := s.GetGitIgnorePatterns(projectID); err == nil {
			patterns = append(patterns, giPatterns...)
		}
	}

	totalMatches := 0
	err = filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if totalMatches >= limit {
			return filepath.SkipDir
		}

		if utils.ShouldSkipPath(root, path, patterns, project.Config.AutoExcludeHidden) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Perform search in file
		fileMatches, searchErr := s.searchInFile(path, root, query, isRegex, re)
		if searchErr != nil {
			return nil
		}

		if len(fileMatches) > 0 {
			res.Results = append(res.Results, fileMatches...)
			totalMatches += len(fileMatches)
		}

		if totalMatches >= limit {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll && err != filepath.SkipDir {
		return nil, err
	}

	res.TotalMatches = totalMatches
	res.QueryTimeMs = time.Since(startTime).Milliseconds()

	return res, nil
}

func (s *ProjectService) searchInFile(path, root, query string, isRegex bool, re *regexp.Regexp) ([][]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	relPath, _ := utils.RelativePathWithinRoot(root, path)
	matches := [][]any{}

	scanner := bufio.NewScanner(f)
	// Limit line length to prevent issues with huge files/binary
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	count := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		matched := false
		if isRegex {
			matched = re.MatchString(line)
		} else {
			matched = strings.Contains(strings.ToLower(line), strings.ToLower(query))
		}

		if matched {
			count++
			// Token optimization: truncate long lines
			content := strings.TrimSpace(line)
			if len(content) > 500 {
				content = content[:497] + "..."
			}
			matches = append(matches, []any{
				relPath,
				lineNum,
				content,
			})
		}

		if count >= 50 { // Max 50 matches per file to avoid huge responses
			break
		}
	}

	return matches, scanner.Err()
}

// startEcoModeCleaner periodically checks for idle embedding clients and unloads them
// to free up VRAM/RAM after 5 minutes of inactivity.
func (s *ProjectService) startEcoModeCleaner() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.clientsMu.Lock()
		now := time.Now()
		
		var toUnload []embedding.EmbeddingClient
		for id, client := range s.embeddingClients {
			// If the model hasn't been used for more than 5 minutes, close it.
			if now.Sub(client.LastActivity()) > 5*time.Minute {
				log.Printf("Eco-Mode: Marking idle embedding model %s for unloading", id)
				toUnload = append(toUnload, client)
				delete(s.embeddingClients, id)
			}
		}

		if len(s.embeddingClients) == 0 && len(toUnload) > 0 {
			// Ricalbriamo la baseline se stiamo scaricando gli ultimi modelli
			s.initialSystemVRAM = embedding.GetTotalVRAMUsage()
			log.Printf("Eco-Mode: Recalibrated system VRAM baseline to %d MB", s.initialSystemVRAM)
		}
		s.clientsMu.Unlock()

		// Eseguiamo la chiusura effettiva fuori dal lock per evitare deadlock
		if len(toUnload) > 0 {
			for _, client := range toUnload {
				if err := client.Close(); err != nil {
					log.Printf("Error closing idle embedding client: %v", err)
				}
			}
			s.rebalanceBatchSizes()
		}
	}
}

func (s *ProjectService) refreshVRAMBudget() int {
	// Base batch is 128 (max), but we scale down if VRAM is tight.
	totalGPU := embedding.DetectGPUVRAM()
	if totalGPU <= 0 {
		return 16 // Fallback
	}

	// Calcoliamo quanto CodeTextor può occupare realmente
	// Capacità = Totale - (Baseline di Sistema + Margine Sicurezza)
	safetyBuffer := 256
	availableForApp := totalGPU - s.initialSystemVRAM - safetyBuffer

	// Minimo vitale: almeno 1GB se possibile, o 512MB
	if availableForApp < 512 {
		availableForApp = 512
	}

	vramGB := float64(availableForApp) / 1024.0
	// 12 batches per ogni GB "dedicato" all'app
	budget := int(vramGB * 12.0)

	log.Printf("VRAM Budget Analysis: GPU Total=%dMB, System Baseline=%dMB -> Dedicated App Capacity=%dMB -> Budget=%d",
		totalGPU, s.initialSystemVRAM, availableForApp, budget)

	// Cap globale di sicurezza per evitare frammentazione eccessiva
	if budget > 128 {
		budget = 128
	}
	if budget < 16 {
		budget = 16
	}
	return budget
}

// rebalanceBatchSizes redistributes the global VRAM budget among all active clients.
func (s *ProjectService) rebalanceBatchSizes() {
	// 1. Gather all necessary data OUTSIDE the client lock to avoid deadlocks with other locks (like m.mu or s.mu)
	projects, _ := s.ListProjects()
	
	indexerStatuses := make(map[string]bool)
	for _, p := range projects {
		indexerStatuses[p.ID] = s.indexerManager.IsProjectIndexing(p.ID)
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	activeCount := len(s.embeddingClients)
	if activeCount == 0 {
		return
	}

	budget := s.refreshVRAMBudget()
	now := time.Now()

	// 2. Identify priority models and active models based on pre-gathered data
	modelIsHeavy := make(map[string]bool)
	modelHasIndexerWork := make(map[string]bool)
	modelProjectCount := make(map[string]int)

	for _, p := range projects {
		if p.Config.EmbeddingModelInfo == nil {
			continue
		}
		mID := p.Config.EmbeddingModelInfo.ID
		modelProjectCount[mID]++
		if s.heavyTasks[p.ID] {
			modelIsHeavy[mID] = true
		}
		if indexerStatuses[p.ID] {
			modelHasIndexerWork[mID] = true
		}
	}

	type modelStatus struct {
		isHeavy      bool
		isActive     bool
		projectCount int
		client       embedding.EmbeddingClient
	}

	statuses := make(map[string]modelStatus)
	heavyCount := 0
	activeCountTotal := 0

	for id, client := range s.embeddingClients {
		isHeavy := modelIsHeavy[id]

		// Un modello è attivo se: prova
		// - Ha avuto attività negli ultimi 30 secondi
		// - Ha dei chunk in attesa nel basket GPU (PendingWork)
		// - Ha un indicizzatore che gli sta mandando file (modelHasIndexerWork)
		// - È marcato come Heavy (reindex manuale)

		hasGPUWork := client.PendingWork() > 0
		hasIndexerWork := modelHasIndexerWork[id]
		recentActivity := now.Sub(client.LastActivity()) < 30*time.Second

		isActive := isHeavy || hasGPUWork || hasIndexerWork || recentActivity

		statuses[id] = modelStatus{isHeavy, isActive, modelProjectCount[id], client}
		if isHeavy {
			heavyCount++
		}
		if isActive {
			activeCountTotal++
		}
	}

	if activeCountTotal == 0 {
		activeCountTotal = len(s.embeddingClients)
		// Se nessuno ha lavoro pendente né attività recente, li consideriamo tutti pronti
		for id, status := range statuses {
			status.isActive = true
			statuses[id] = status
		}
	}

	// 2. Calcola i batch size con priorità
	// Regola: Heavy > Active > Idle
	// Minimo assoluto: 2.

	for id, status := range statuses {
		targetBatch := 2 // Fallback minimo

		if status.isHeavy {
			// Il modello heavy cerca di prendersi la potenza di 2 massima che lasci almeno 2 a tutti gli altri
			remainingBudget := budget - (len(s.embeddingClients)-1)*2
			targetBatch = s.maxPowerOf2(remainingBudget)
		} else if status.isActive && heavyCount == 0 {
			// Se non ci sono heavy, i modelli attivi si dividono il budget
			remainingBudget := budget / activeCountTotal
			targetBatch = s.maxPowerOf2(remainingBudget)
		} else {
			// Idle o deprioritizzato da un heavy
			targetBatch = 2
		}

		// Cap per singolo modello (evitiamo batch > 128 nel normale utilizzo se non richiesto)
		if targetBatch > 128 {
			targetBatch = 128
		}
		if targetBatch < 2 {
			targetBatch = 2
		}

		oldBatch := status.client.GetBatchSize()
		if oldBatch != targetBatch {
			log.Printf("VRAM Budgeting: Modello %s -> Batch %d (era %d)", id, targetBatch, oldBatch)
			status.client.SetBatchSize(targetBatch)
		}
	}

	// 3. Log di riepilogo consolidato per trasparenza
	log.Printf("VRAM Distribution Summary (Total Budget: %d):", budget)
	for id, status := range statuses {
		stateStr := "IDLE"
		if status.isHeavy {
			stateStr = "HEAVY"
		} else if status.isActive {
			stateStr = "ACTIVE"
		}
		log.Printf("  - %-30s: Batch %-3d [%s] (%d projects)", id, status.client.GetBatchSize(), stateStr, status.projectCount)
	}
}

// maxPowerOf2 returns the largest power of 2 less than or equal to n.
func (s *ProjectService) maxPowerOf2(n int) int {
	if n < 2 {
		return 2
	}
	p := 1
	for p*2 <= n {
		p *= 2
	}
	return p
}
