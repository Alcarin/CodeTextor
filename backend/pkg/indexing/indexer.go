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
	semanticChunker *chunker.SemanticChunker
	// Debounce map: tracks pending file updates
	debounceMu     sync.Mutex
	debounceTimers map[string]*time.Timer
	eventEmitter   func(string, interface{})
	embeddingModelID string
}

// NewIndexer creates a new indexer for a project.
func NewIndexer(project *models.Project, vectorStore *store.VectorStore, eventEmitter func(string, interface{}), client embedding.EmbeddingClient) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create chunk config from project settings
	chunkConfig := chunker.ChunkConfig{
		MaxChunkSize:      project.Config.ChunkSizeMax,
		MinChunkSize:      project.Config.ChunkSizeMin,
		CollapseThreshold: 500, // Default threshold for collapsing
		MergeSmallChunks:  true,
		IncludeComments:   true,
	}

	if client == nil {
		cancel()
		return nil, fmt.Errorf("embedding client is required for project %s", project.ID)
	}

	modelID := strings.TrimSpace(project.Config.EmbeddingModel)
	if modelID == "" && project.Config.EmbeddingModelInfo != nil {
		modelID = project.Config.EmbeddingModelInfo.ID
	}
	if modelID == "" {
		modelID = "unknown"
	}

	return &Indexer{
		project:         project,
		progress:        &models.IndexingProgress{Status: models.IndexingStatusIdle},
		stopChan:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		semaphore:       make(chan struct{}, 10), // Limit to 10 concurrent operations
		embeddingClient: client,
		vectorStore:     vectorStore,
		parser:          chunker.NewParser(chunkConfig),
		semanticChunker: chunker.NewSemanticChunker(chunkConfig),
		debounceTimers:  make(map[string]*time.Timer),
		eventEmitter:    eventEmitter,
		embeddingModelID: modelID,
	}, nil
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

	// Clean up artifacts for files that no longer exist.
	i.cleanupRemovedFiles(filePreviews)

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

			// Read file content
			source, err := os.ReadFile(file.AbsolutePath)
			if err != nil {
				log.Printf("Failed to read file %s: %v", file.AbsolutePath, err)
				i.progress.ProcessedFiles++
				return
			}

			// Check if file has changed since last indexing
			fileHash := utils.ComputeHash(source)
			existingFile, err := i.vectorStore.GetFile(file.RelativePath)
			if err == nil && existingFile != nil {
				// File exists in database, check if it changed
				if existingFile.Hash == fileHash && existingFile.LastModified == file.LastModified {
					// File hasn't changed, skip re-indexing
					log.Printf("Skipping unchanged file %s", file.RelativePath)
					i.progress.ProcessedFiles++
					return
				}
			}

			// File is new or has changed, delete existing chunks and re-index
			if err := i.vectorStore.DeleteFileChunks(file.RelativePath); err != nil {
				log.Printf("Failed to delete old chunks for %s: %v", file.RelativePath, err)
			}

			// Check if file is supported for semantic chunking
			var chunkContents []string
			var dbChunks []*models.Chunk

			if i.semanticChunker.IsSupported(file.RelativePath) {
				// Use semantic chunking for supported files
				semanticChunks, err := i.semanticChunker.ChunkFile(file.RelativePath, source)
				if err != nil {
					log.Printf("Failed to semantically chunk file %s: %v", file.AbsolutePath, err)
					i.progress.ProcessedFiles++
					return
				}

				// Extract enriched content for embedding and prepare DB chunks
				chunkContents = make([]string, len(semanticChunks))
				dbChunks = make([]*models.Chunk, len(semanticChunks))

				for idx, chunk := range semanticChunks {
					chunkContents[idx] = chunk.Content // Use enriched content for embedding

					// Prepare chunk for database storage
					dbChunks[idx] = &models.Chunk{
						FilePath:    file.RelativePath,
						Content:     chunk.Content,
						LineStart:   int(chunk.StartLine),
						LineEnd:     int(chunk.EndLine),
						CharStart:   int(chunk.StartByte),
						CharEnd:     int(chunk.EndByte),
						Language:    chunk.Language,
						SymbolName:  chunk.SymbolName,
						SymbolKind:  string(chunk.SymbolKind),
						Parent:      chunk.Parent,
						Signature:   chunk.Signature,
						Visibility:  chunk.Visibility,
						PackageName: chunk.PackageName,
						DocString:   chunk.DocString,
						TokenCount:  chunk.TokenCount,
						IsCollapsed: chunk.IsCollapsed,
						SourceCode:  chunk.SourceCode,
						EmbeddingModelID: i.embeddingModelID,
					}
				}
				log.Printf("Created %d semantic chunks for file %s", len(semanticChunks), file.RelativePath)
			} else {
				// Fallback to simple line-based chunking for unsupported files
				simpleChunks, err := utils.ChunkFile(file.AbsolutePath, i.project.Config.ChunkSizeMax)
				if err != nil {
					log.Printf("Failed to chunk file %s: %v", file.AbsolutePath, err)
					i.progress.ProcessedFiles++
					return
				}

				chunkContents = make([]string, len(simpleChunks))
				dbChunks = make([]*models.Chunk, len(simpleChunks))

				for idx, chunk := range simpleChunks {
					chunkContents[idx] = chunk.Content

					// Prepare simple chunk for database
					dbChunks[idx] = &models.Chunk{
						FilePath:  file.RelativePath,
						Content:   chunk.Content,
						LineStart: chunk.LineStart,
						LineEnd:   chunk.LineEnd,
						CharStart: chunk.CharacterStart,
						CharEnd:   chunk.CharacterEnd,
						EmbeddingModelID: i.embeddingModelID,
					}
				}
				log.Printf("Created %d simple chunks for file %s (unsupported format)", len(simpleChunks), file.RelativePath)
			}

			// Generate embeddings for chunks
			embeddings, err := i.embeddingClient.GenerateEmbeddings(chunkContents)
			if err != nil {
				log.Printf("Failed to generate embeddings for file %s: %v", file.AbsolutePath, err)
				i.progress.ProcessedFiles++
				return
			}
			log.Printf("Generated %d embeddings for file %s", len(embeddings), file.RelativePath)

			// Save chunks to database with embeddings
			for idx, dbChunk := range dbChunks {
				if idx < len(embeddings) {
					dbChunk.Embedding = embeddings[idx]
				}

				if err := i.vectorStore.InsertChunk(dbChunk); err != nil {
					log.Printf("Failed to save chunk %d for file %s: %v", idx, file.RelativePath, err)
				}
			}

			// Save file metadata
			fileRecord := &models.File{
				Path:         file.RelativePath,
				Hash:         fileHash,
				LastModified: file.LastModified,
				ChunkCount:   len(dbChunks),
			}
			if err := i.vectorStore.InsertFile(fileRecord); err != nil {
				log.Printf("Failed to save file metadata for %s: %v", file.RelativePath, err)
			}

			log.Printf("Saved %d chunks for file %s to database", len(dbChunks), file.RelativePath)

			if i.project.Config.ContinuousIndexing {
				i.storeOutlineForFile(file.AbsolutePath)
			}

			i.emitFileUpdate(file.RelativePath)

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

// debounceFileUpdate schedules a file index update (chunks + outline) with debouncing.
// Multiple rapid changes to the same file will be coalesced into a single update.
func (i *Indexer) debounceFileUpdate(filePath string) {
	const debounceDelay = 2 * time.Second

	i.debounceMu.Lock()
	defer i.debounceMu.Unlock()

	// Cancel existing timer for this file if any
	if timer, exists := i.debounceTimers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer that will trigger full index update
	i.debounceTimers[filePath] = time.AfterFunc(debounceDelay, func() {
		log.Printf("Processing index update for %s (after debounce)", filePath)
		i.updateFileIndex(filePath)

		// Clean up the timer
		i.debounceMu.Lock()
		delete(i.debounceTimers, filePath)
		i.debounceMu.Unlock()
	})
}

// updateFileIndex re-indexes a single file (chunks + outline) when it changes.
// This is called by the file watcher when a file is modified.
func (i *Indexer) updateFileIndex(filePath string) {
	if i.vectorStore == nil || i.parser == nil || i.semanticChunker == nil {
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

	// Get relative path for storage
	relativePath := filepath.ToSlash(absPath)
	if rel, ok := utils.RelativePathWithinRoot(i.project.Config.RootPath, absPath); ok && rel != "" {
		relativePath = rel
	}

	// Read file content
	source, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("Failed to read file for re-indexing %s: %v", absPath, err)
		return
	}

	// Get file info for last modified timestamp
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		log.Printf("Failed to stat file %s: %v", absPath, err)
		return
	}

	// Check if file has changed
	fileHash := utils.ComputeHash(source)
	existingFile, err := i.vectorStore.GetFile(relativePath)
	if err == nil && existingFile != nil {
		if existingFile.Hash == fileHash && existingFile.LastModified == fileInfo.ModTime().Unix() {
			log.Printf("Skipping unchanged file %s", relativePath)
			return
		}
	}

	log.Printf("Re-indexing changed file: %s", relativePath)

	// Delete existing chunks
	if err := i.vectorStore.DeleteFileChunks(relativePath); err != nil {
		log.Printf("Failed to delete old chunks for %s: %v", relativePath, err)
	}

	// Check if file is supported for semantic chunking
	var chunkContents []string
	var dbChunks []*models.Chunk

	if i.semanticChunker.IsSupported(relativePath) {
		// Use semantic chunking for supported files
		semanticChunks, err := i.semanticChunker.ChunkFile(relativePath, source)
		if err != nil {
			log.Printf("Failed to semantically chunk file %s: %v", absPath, err)
			return
		}

		// Extract enriched content for embedding and prepare DB chunks
		chunkContents = make([]string, len(semanticChunks))
		dbChunks = make([]*models.Chunk, len(semanticChunks))

		for idx, chunk := range semanticChunks {
			chunkContents[idx] = chunk.Content

			dbChunks[idx] = &models.Chunk{
				FilePath:    relativePath,
				Content:     chunk.Content,
				LineStart:   int(chunk.StartLine),
				LineEnd:     int(chunk.EndLine),
				CharStart:   int(chunk.StartByte),
				CharEnd:     int(chunk.EndByte),
				Language:    chunk.Language,
				SymbolName:  chunk.SymbolName,
				SymbolKind:  string(chunk.SymbolKind),
				Parent:      chunk.Parent,
				Signature:   chunk.Signature,
				Visibility:  chunk.Visibility,
				PackageName: chunk.PackageName,
				DocString:   chunk.DocString,
				TokenCount:  chunk.TokenCount,
				IsCollapsed: chunk.IsCollapsed,
				SourceCode:  chunk.SourceCode,
				EmbeddingModelID: i.embeddingModelID,
			}
		}
		log.Printf("Created %d semantic chunks for file %s", len(semanticChunks), relativePath)
	} else {
		// Fallback to simple line-based chunking
		simpleChunks, err := utils.ChunkFile(absPath, i.project.Config.ChunkSizeMax)
		if err != nil {
			log.Printf("Failed to chunk file %s: %v", absPath, err)
			return
		}

		chunkContents = make([]string, len(simpleChunks))
		dbChunks = make([]*models.Chunk, len(simpleChunks))

		for idx, chunk := range simpleChunks {
			chunkContents[idx] = chunk.Content

		dbChunks[idx] = &models.Chunk{
			FilePath:  relativePath,
			Content:   chunk.Content,
			LineStart: chunk.LineStart,
			LineEnd:   chunk.LineEnd,
			CharStart: chunk.CharacterStart,
			CharEnd:   chunk.CharacterEnd,
			EmbeddingModelID: i.embeddingModelID,
		}
	}
		log.Printf("Created %d simple chunks for file %s", len(simpleChunks), relativePath)
	}

	// Generate embeddings for chunks
	embeddings, err := i.embeddingClient.GenerateEmbeddings(chunkContents)
	if err != nil {
		log.Printf("Failed to generate embeddings for file %s: %v", absPath, err)
		return
	}

	// Save chunks to database with embeddings
	for idx, dbChunk := range dbChunks {
		if idx < len(embeddings) {
			dbChunk.Embedding = embeddings[idx]
		}

		if err := i.vectorStore.InsertChunk(dbChunk); err != nil {
			log.Printf("Failed to save chunk %d for file %s: %v", idx, relativePath, err)
		}
	}

	// Save file metadata
	fileRecord := &models.File{
		Path:         relativePath,
		Hash:         fileHash,
		LastModified: fileInfo.ModTime().Unix(),
		ChunkCount:   len(dbChunks),
	}
	if err := i.vectorStore.InsertFile(fileRecord); err != nil {
		log.Printf("Failed to save file metadata for %s: %v", relativePath, err)
	}

	log.Printf("Updated %d chunks for file %s", len(dbChunks), relativePath)

	// Also update the outline
	i.storeOutlineForFile(absPath)
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

	// Save outline nodes
	nodes := outline.BuildOutlineNodes(relativePath, result.Symbols)
	if len(nodes) > 0 {
		if err := i.vectorStore.UpsertFileOutline(relativePath, nodes); err != nil {
			log.Printf("Failed to persist outline for %s: %v", absPath, err)
		}

		absKey := filepath.ToSlash(absPath)
		if absKey != relativePath {
			// Remove any legacy absolute-path outline/symbol/chunk records without touching the new relative entry.
			if err := i.vectorStore.RemoveFileAndArtifacts(absKey); err != nil && !strings.Contains(err.Error(), "file not found") {
				log.Printf("Failed to remove legacy outline key %s: %v", absKey, err)
			}
		}
	}

	// Save individual symbols to symbols table
	if len(result.Symbols) > 0 {
		// Delete old symbols for this file
		if err := i.vectorStore.DeleteFileSymbols(relativePath); err != nil {
			log.Printf("Failed to delete old symbols for %s: %v", relativePath, err)
		}

		// Insert new symbols
		for _, parsedSymbol := range result.Symbols {
			symbol := &models.Symbol{
				FilePath:  relativePath,
				Name:      parsedSymbol.Name,
				Kind:      string(parsedSymbol.Kind),
				Line:      int(parsedSymbol.StartLine),
				Character: 0, // We don't have character position from parser
			}
			if err := i.vectorStore.InsertSymbol(symbol); err != nil {
				log.Printf("Failed to insert symbol %s for file %s: %v", parsedSymbol.Name, relativePath, err)
			}
		}
		log.Printf("Saved %d symbols for file %s", len(result.Symbols), relativePath)
	}

	if err := i.vectorStore.RebuildChunkSymbolLinks(relativePath); err != nil {
		log.Printf("Failed to rebuild chunk-symbol links for %s: %v", relativePath, err)
	}

	i.emitFileUpdate(relativePath)
}

// cleanupRemovedFiles deletes stored artifacts for files missing from disk.
func (i *Indexer) cleanupRemovedFiles(currentFiles []*models.FilePreview) {
	if i.vectorStore == nil {
		return
	}
	current := make(map[string]struct{}, len(currentFiles))
	for _, f := range currentFiles {
		current[filepath.ToSlash(f.RelativePath)] = struct{}{}
	}

	tracked, err := i.vectorStore.ListAllFilePaths()
	if err != nil {
		log.Printf("Failed to list tracked files for cleanup: %v", err)
		return
	}

	for _, path := range tracked {
		if _, ok := current[path]; ok {
			continue
		}
		abs := filepath.Join(i.project.Config.RootPath, path)
		if _, err := os.Stat(abs); err == nil {
			// File still exists but not in current scope; skip removal.
			continue
		}
		if err := i.vectorStore.RemoveFileAndArtifacts(path); err != nil {
			log.Printf("Failed to remove stale artifacts for %s: %v", path, err)
			continue
		}
		log.Printf("Removed stale artifacts for missing file %s", path)
	}
}

func (i *Indexer) emitFileUpdate(filePath string) {
	if i.eventEmitter == nil {
		return
	}
	payload := map[string]interface{}{
		"projectId": i.project.ID,
		"filePath":  filePath,
		"timestamp": time.Now().Unix(),
	}
	i.eventEmitter("project:fileIndexed", payload)
}
