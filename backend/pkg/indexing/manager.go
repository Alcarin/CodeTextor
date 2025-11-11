package indexing

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/models"
	"sync"
)

// Manager manages the lifecycle of indexing jobs for all projects.
type Manager struct {
	projectIndexers map[string]*Indexer
	progressMap     sync.Map // Safely stores map[string]*models.IndexingProgress
	mu              sync.Mutex
}

// NewManager creates a new IndexerManager.
func NewManager() *Manager {
	return &Manager{
		projectIndexers: make(map[string]*Indexer),
	}
}

// StartIndexer starts a new indexing job for a given project.
// If an indexer is already running for the project, it will be stopped first.
func (m *Manager) StartIndexer(project *models.Project, files []*models.FilePreview, vectorStore *store.VectorStore) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If an indexer is already running, stop it before starting a new one.
	if indexer, exists := m.projectIndexers[project.ID]; exists {
		indexer.Stop()
	}

	// Create and run a new indexer.
	newIndexer := NewIndexer(project, vectorStore)
	m.projectIndexers[project.ID] = newIndexer
	m.progressMap.Store(project.ID, newIndexer.progress)

	go func() {
		newIndexer.Run(files)
		// Once the indexer is finished, remove it from the active list.
		m.mu.Lock()
		delete(m.projectIndexers, project.ID)
		m.mu.Unlock()
	}()
}

// StopIndexer stops the indexing job for a given project.
func (m *Manager) StopIndexer(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if indexer, exists := m.projectIndexers[projectID]; exists {
		indexer.Stop()
		delete(m.projectIndexers, projectID)
	}
}

// GetIndexingProgress retrieves the current indexing progress for a project.
func (m *Manager) GetIndexingProgress(projectID string) (*models.IndexingProgress, bool) {
	progress, found := m.progressMap.Load(projectID)
	if !found {
		return nil, false
	}
	return progress.(*models.IndexingProgress), true
}
