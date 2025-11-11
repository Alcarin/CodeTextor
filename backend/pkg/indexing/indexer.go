package indexing

import (
	"CodeTextor/backend/internal/chunker"
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/embedding"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/outline"
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
	vectorStore     *store.VectorStore
	parser          *chunker.Parser
	// Debounce map: tracks pending file updates
	debounceMu     sync.Mutex
	debounceTimers map[string]*time.Timer
}

// NewIndexer creates a new indexer for a project.
func NewIndexer(project *models.Project, vectorStore *store.VectorStore) *Indexer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Indexer{
		project:         project,
		progress:        &models.IndexingProgress{Status: models.IndexingStatusIdle},
		stopChan:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		semaphore:       make(chan struct{}, 10),                // Limit to 10 concurrent operations
		embeddingClient: embedding.NewMockEmbeddingClient(1536), // Using a common dimension size
		vectorStore:     vectorStore,
		parser:          chunker.NewParser(chunker.DefaultChunkConfig()),
		debounceTimers:  make(map[string]*time.Timer),
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

			if i.project.Config.ContinuousIndexing {
				i.storeOutlineForFile(file.AbsolutePath)
			}

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

		// Resolve include paths to absolute directories so the watcher
		// follows the actual folders on disk (not the relative entries stored in config).
		includePaths := resolveIncludePaths(i.project.Config.RootPath, i.project.Config.IncludePaths)

		// Add all include paths to the watcher
		for _, path := range includePaths {
			// Recursively add directories to watcher
			includeRoot := path
			filepath.WalkDir(includeRoot, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					log.Printf("Error walking path %s for watcher: %v", p, err)
					return nil // Don't stop walk, just skip this path
				}
				if d.IsDir() {
					// Check if directory should be excluded using relative + absolute patterns
					if shouldSkipDir(includeRoot, p, i.project.Config.ExcludePatterns) {
						return filepath.SkipDir
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

				// Only process Write and Create events for files
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					// Check if it's a supported file
					if i.parser.IsSupported(event.Name) {
						log.Printf("File changed in project %s: %s", i.project.Name, event.Name)
						i.debounceFileUpdate(event.Name)
					}
				}
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

// resolveIncludePaths mirrors the logic used in the project service
// to ensure watcher paths are absolute and anchored to the configured root.
func resolveIncludePaths(root string, includes []string) []string {
	root = strings.TrimSpace(root)
	if root != "" && !filepath.IsAbs(root) {
		if absRoot, err := filepath.Abs(root); err == nil {
			root = absRoot
		}
	}

	var cwd string
	if wd, err := os.Getwd(); err == nil {
		cwd = wd
	}

	if len(includes) == 0 {
		includes = []string{"."}
	}

	var resolved []string
	for _, rel := range includes {
		switch {
		case rel == "", rel == ".":
			switch {
			case root != "":
				resolved = append(resolved, filepath.Clean(root))
			case cwd != "":
				resolved = append(resolved, filepath.Clean(cwd))
			}
		case filepath.IsAbs(rel):
			resolved = append(resolved, filepath.Clean(rel))
		default:
			base := root
			if base == "" {
				base = cwd
			}
			if base != "" {
				resolved = append(resolved, filepath.Clean(filepath.Join(base, rel)))
			} else if abs, err := filepath.Abs(rel); err == nil {
				resolved = append(resolved, filepath.Clean(abs))
			}
		}
	}

	return resolved
}

func shouldSkipDir(root, dir string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	absPath := filepath.Clean(dir)
	relPath := absPath
	if root != "" {
		if rel, err := filepath.Rel(root, absPath); err == nil {
			relPath = rel
		}
	}

	absSlash := filepath.ToSlash(absPath)
	relSlash := filepath.ToSlash(relPath)
	base := filepath.Base(absPath)

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if matched, _ := filepath.Match(pattern, relSlash); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, absSlash); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// Stop gracefully stops the indexer.
func (i *Indexer) Stop() {
	// Cancel all pending debounce timers
	i.debounceMu.Lock()
	for _, timer := range i.debounceTimers {
		timer.Stop()
	}
	i.debounceTimers = make(map[string]*time.Timer)
	i.debounceMu.Unlock()

	if i.watcher != nil {
		i.watcher.Close()
	}
	i.cancel()
}

// debounceFileUpdate schedules a file outline update with debouncing.
// Multiple rapid changes to the same file will be coalesced into a single update.
func (i *Indexer) debounceFileUpdate(filePath string) {
	const debounceDelay = 10 * time.Second

	i.debounceMu.Lock()
	defer i.debounceMu.Unlock()

	// Cancel existing timer for this file if any
	if timer, exists := i.debounceTimers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer that will trigger outline update
	i.debounceTimers[filePath] = time.AfterFunc(debounceDelay, func() {
		log.Printf("Processing outline update for %s (after debounce)", filePath)
		i.storeOutlineForFile(filePath)

		// Clean up the timer
		i.debounceMu.Lock()
		delete(i.debounceTimers, filePath)
		i.debounceMu.Unlock()
	})
}

func (i *Indexer) storeOutlineForFile(filePath string) {
	if i.vectorStore == nil || i.parser == nil {
		return
	}
	if filePath == "" {
		return
	}

	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		if resolved, err := filepath.Abs(absPath); err == nil {
			absPath = resolved
		}
	}

	if !i.parser.IsSupported(absPath) {
		return
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("Failed to read file for outline %s: %v", absPath, err)
		return
	}

	result, err := i.parser.ParseFile(absPath, source)
	if err != nil {
		log.Printf("Failed to parse outline for %s: %v", absPath, err)
		return
	}

	relativePath := filepath.ToSlash(absPath)
	if rel, ok := utils.RelativePathWithinRoot(i.project.Config.RootPath, absPath); ok && rel != "" {
		relativePath = rel
	}

	nodes := outline.BuildOutlineNodes(relativePath, result.Symbols)
	if len(nodes) == 0 {
		return
	}

	if err := i.vectorStore.UpsertFileOutline(relativePath, nodes); err != nil {
		log.Printf("Failed to persist outline for %s: %v", absPath, err)
	}

	absKey := filepath.ToSlash(absPath)
	if absKey != relativePath {
		if err := i.vectorStore.DeleteFileOutline(absKey); err != nil {
			log.Printf("Failed to remove legacy outline key %s: %v", absKey, err)
		}
	}
}
