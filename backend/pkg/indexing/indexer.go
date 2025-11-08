package indexing

import (
	"CodeTextor/backend/pkg/embedding"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Indexer is responsible for indexing a single project.
// It runs in its own goroutine and can be safely stopped.
type Indexer struct {
	project         *models.Project
	progress        *models.IndexingProgress
	stopChan        chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	watcher         *fsnotify.Watcher
	semaphore       chan struct{}
	embeddingClient embedding.EmbeddingClient
}

// NewIndexer creates a new indexer for a project.
func NewIndexer(project *models.Project) *Indexer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Indexer{
		project:         project,
		progress:        &models.IndexingProgress{Status: models.IndexingStatusIdle},
		stopChan:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		semaphore:       make(chan struct{}, 10), // Limit to 10 concurrent operations
		embeddingClient: embedding.NewMockEmbeddingClient(1536), // Using a common dimension size
	}
}

// Run starts the indexing process.
// This method is intended to be run in a goroutine.
func (i *Indexer) Run(filePreviews []*models.FilePreview) {
	i.progress.Status = models.IndexingStatusIndexing
	i.progress.TotalFiles = len(filePreviews)
	i.progress.ProcessedFiles = 0
	i.progress.CurrentFile = ""
	i.progress.Error = ""

	log.Printf("Starting indexing for project %s: %d files to process", i.project.Name, i.progress.TotalFiles)

	// --- Initial Indexing Pass ---
	var wg sync.WaitGroup
	for _, file := range filePreviews {
		wg.Add(1)
		go func(file *models.FilePreview) {
			defer wg.Done()

			// Acquire semaphore
			i.semaphore <- struct{}{}
			defer func() { <-i.semaphore }()

			select {
			case <-i.ctx.Done():
				return // Stop processing if context is cancelled
			default:
				// Continue processing
			}

			i.progress.CurrentFile = file.RelativePath

			// Chunk the file
			chunks, err := utils.ChunkFile(file.AbsolutePath, i.project.Config.ChunkSizeMax)
			if err != nil {
				log.Printf("Failed to chunk file %s: %v", file.AbsolutePath, err)
				i.progress.ProcessedFiles++
				return
			}
			// Generate embeddings for chunks
			chunkContents := make([]string, len(chunks))
			for i, chunk := range chunks {
				chunkContents[i] = chunk.Content
			}
			embeddings, err := i.embeddingClient.GenerateEmbeddings(chunkContents)
			if err != nil {
				log.Printf("Failed to generate embeddings for file %s: %v", file.AbsolutePath, err)
				i.progress.ProcessedFiles++
				return
			}
			log.Printf("Generated %d embeddings for file %s", len(embeddings), file.RelativePath)

			// Simulate processing time
			time.Sleep(5 * time.Millisecond)

			i.progress.ProcessedFiles++
		}(file)
	}

	wg.Wait()

	log.Printf("Initial indexing completed for project %s", i.project.Name)

	// --- Continuous Indexing (File Watching) ---
	if i.project.Config.ContinuousIndexing {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Printf("Failed to create file watcher for project %s: %v", i.project.Name, err)
			i.progress.Status = models.IndexingStatusError
			i.progress.Error = fmt.Sprintf("Failed to start file watcher: %v", err)
			return
		}
		i.watcher = watcher
		defer i.watcher.Close()

		// Add all include paths to the watcher
		for _, path := range i.project.Config.IncludePaths {
			// Recursively add directories to watcher
			filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					log.Printf("Error walking path %s for watcher: %v", p, err)
					return nil // Don't stop walk, just skip this path
				}
				if d.IsDir() {
					// Check if directory should be excluded
					for _, excludePattern := range i.project.Config.ExcludePatterns {
						if matched, _ := filepath.Match(excludePattern, p); matched {
							return filepath.SkipDir
						}
					}
					// Check for hidden directories
					if i.project.Config.AutoExcludeHidden && strings.HasPrefix(d.Name(), ".") && len(d.Name()) > 1 {
						return filepath.SkipDir
					}
					log.Printf("Adding path to watcher: %s", p)
					err := i.watcher.Add(p)
					if err != nil {
						log.Printf("Failed to add path %s to watcher: %v", p, err)
					}
				}
				return nil
			})
		}

		i.progress.Status = models.IndexingStatusIdle // Back to idle after initial scan

		for {
			select {
			case <-i.ctx.Done():
				log.Printf("File watcher stopped for project %s", i.project.Name)
				return
			case event, ok := <-i.watcher.Events:
				if !ok {
					log.Printf("File watcher events channel closed for project %s", i.project.Name)
					return
				}
				log.Printf("File system event for project %s: %s %s", i.project.Name, event.Op.String(), event.Name)
				// TODO: Trigger re-indexing for the changed file/directory
				// For now, just update status to show activity
				i.progress.Status = models.IndexingStatusIndexing
				i.progress.CurrentFile = event.Name
				time.AfterFunc(2*time.Second, func(){
					i.progress.Status = models.IndexingStatusIdle
					i.progress.CurrentFile = ""
				})
			case err, ok := <-i.watcher.Errors:
				if !ok {
					log.Printf("File watcher errors channel closed for project %s", i.project.Name)
					return
				}
				log.Printf("File watcher error for project %s: %v", i.project.Name, err)
				i.progress.Status = models.IndexingStatusError
				i.progress.Error = fmt.Sprintf("File watcher error: %v", err)
				return
			}
		}
	} else {
		i.progress.Status = models.IndexingStatusCompleted // If no continuous indexing, just complete
		i.progress.CurrentFile = ""
	}
}

// Stop gracefully stops the indexer.
func (i *Indexer) Stop() {
	if i.watcher != nil {
		i.watcher.Close()
	}
	i.cancel()
}