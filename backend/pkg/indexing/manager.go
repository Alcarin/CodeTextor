package indexing

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/embedding"
	"CodeTextor/backend/pkg/models"
	"sync"
	"sync/atomic"
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
func (m *Manager) StartIndexer(project *models.Project, files []*models.FilePreview, vectorStore *store.VectorStore, client embedding.EmbeddingClient, onComplete func(models.IndexingStatus), onInitialScanComplete func(), onFileIndexed func(string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If an indexer is already running, check if it's already busy with the same work
	if existingIndexer, exists := m.projectIndexers[project.ID]; exists {
		// If it's already indexing or in idle/watcher mode, don't stop it and don't restart it
		// This prevents frontend "refresh" calls from killing a running indexer.
		// Only stop if we specifically want to re-index (which is handled by StopIndexer calls before StartIndexer in ReindexProject).
		if existingIndexer.progress.Status == models.IndexingStatusIndexing || existingIndexer.progress.Status == models.IndexingStatusIdle {
			return nil
		}
		// Otherwise, stop it normally (e.g. if it's in Error or somehow stuck)
		existingIndexer.Stop()
		delete(m.projectIndexers, project.ID)
	}

	// Create and register the new indexer
	newIndexer, err := NewIndexer(project, vectorStore, client, m.eventEmitter)
	if err != nil {
		return err
	}
	newIndexer.OnInitialScanComplete = onInitialScanComplete
	newIndexer.OnFileIndexed = onFileIndexed
	m.projectIndexers[project.ID] = newIndexer
	m.progressMap.Store(project.ID, newIndexer.progress)

	// Start the indexer in a goroutine
	go func() {
		newIndexer.Run(files)
		if onComplete != nil {
			onComplete(newIndexer.progress.Status)
		}

		// Clean up when done
		m.mu.Lock()
		// Only delete if this indexer is still the registered one
		// (it might have been replaced by another StartIndexer call)
		if currentIndexer, exists := m.projectIndexers[project.ID]; exists && currentIndexer == newIndexer {
			delete(m.projectIndexers, project.ID)
		}
		m.mu.Unlock()
	}()

	return nil
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

// ClearProgress removes the progress tracking for a given project.
func (m *Manager) ClearProgress(projectID string) {
	m.progressMap.Delete(projectID)
}

// GetIndexingProgress retrieves the current indexing progress for a project.
func (m *Manager) GetIndexingProgress(projectID string) (models.IndexingProgress, bool) {
	m.mu.Lock()
	indexer, exists := m.projectIndexers[projectID]
	m.mu.Unlock()

	if exists {
		return indexer.GetProgress(), true
	}

	progress, found := m.progressMap.Load(projectID)
	if !found {
		return models.IndexingProgress{}, false
	}
	
	// If the indexer is gone, we still have the last pointer in progressMap.
	// Since no one is writing to it anymore (indexer is dead), it's safe-ish,
	// but let's be consistent and return a copy.
	p := progress.(*models.IndexingProgress)
	return models.IndexingProgress{
		TotalFiles:     atomic.LoadInt32(&p.TotalFiles),
		ProcessedFiles: atomic.LoadInt32(&p.ProcessedFiles),
		CurrentFile:    p.CurrentFile,
		Status:         p.Status,
		Error:          p.Error,
	}, true
}

// IsProjectIndexing returns true if an indexer is currently active for the given project.
func (m *Manager) IsProjectIndexing(projectID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.projectIndexers[projectID]
	return exists
}
