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
	eventEmitter    func(string, interface{})
}

// NewManager creates a new IndexerManager.
func NewManager(eventEmitter func(string, interface{})) *Manager {
	return &Manager{
		projectIndexers: make(map[string]*Indexer),
		eventEmitter:    eventEmitter,
	}
}

// StartIndexer starts a new indexing job for a given project.
// If an indexer is already running for the project, the existing one will be stopped first.
// This method ensures that only one indexer runs per project at a time.
func (m *Manager) StartIndexer(project *models.Project, files []*models.FilePreview, vectorStore *store.VectorStore) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If an indexer is already running, stop it first
	if existingIndexer, exists := m.projectIndexers[project.ID]; exists {
		// Stop the existing indexer (this will cancel its context)
		existingIndexer.Stop()
		// Remove it from the map immediately to prevent race conditions
		delete(m.projectIndexers, project.ID)
		// Note: The goroutine will still try to delete from map when it finishes,
		// but that's safe since we're holding the lock and it's already deleted
	}

	// Create and register the new indexer
	newIndexer := NewIndexer(project, vectorStore, m.eventEmitter)
	m.projectIndexers[project.ID] = newIndexer
	m.progressMap.Store(project.ID, newIndexer.progress)

	// Start the indexer in a goroutine
	go func() {
		newIndexer.Run(files)

		// Clean up when done
		m.mu.Lock()
		// Only delete if this indexer is still the registered one
		// (it might have been replaced by another StartIndexer call)
		if currentIndexer, exists := m.projectIndexers[project.ID]; exists && currentIndexer == newIndexer {
			delete(m.projectIndexers, project.ID)
		}
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
