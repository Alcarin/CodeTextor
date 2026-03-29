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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
	debounceMu       sync.Mutex
	debounceTimers   map[string]*time.Timer
	eventEmitter     func(string, interface{})
	embeddingModelID string
	taskChan         chan *embeddingTask // Centralized GPU queue for both initial and live indexing
	
	// Callbacks for external components (e.g. Symbol Linker)
	OnInitialScanComplete func()
	OnFileIndexed         func(filePath string)
}

// embeddingTask holds pre-processed file data ready for GPU embedding.
type embeddingTask struct {
	filePath        string
	absPath         string
	dbChunks        []*models.Chunk
	texts           []string                    // raw texts (fallback for non-ONNX clients)
	tokenizedChunks []*embedding.TokenizedChunk // pre-tokenized (nil if client doesn't support it)
	fileRecord      *models.File
	storeOutline    bool
	wg              *sync.WaitGroup // Optional: WaitGroup to signal when this specific task is fully finalized
}

// pendingItem tracks a single chunk waiting in the GPU basket.
type pendingItem struct {
	task      *embeddingTask
	chunkIdx  int
	text      string                   // raw text (fallback)
	tokenized *embedding.TokenizedChunk // pre-tokenized data (nil = use text)
}

// gpuBatchSize returns the optimal number of chunks to send to the GPU at once.
func (i *Indexer) gpuBatchSize() int {
	if i.embeddingClient == nil {
		return 32 // Safe fallback
	}
	return i.embeddingClient.GetBatchSize()
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

	// Concurrency limit for parallel file processing (CPU parsing/chunking).
	// Keep at 50% to avoid saturating the CPU and freezing the system.
	// increased concurrency to maximize CPU throughput during parsing/tokenization
	concurrencyLimit := runtime.NumCPU()
	if concurrencyLimit < 4 {
		concurrencyLimit = 4
	}
	if concurrencyLimit < 1 {
		concurrencyLimit = 1
	}

	const taskChanBuffer = 128
	taskChan := make(chan *embeddingTask, taskChanBuffer)

	return &Indexer{
		project:          project,
		progress:         &models.IndexingProgress{Status: models.IndexingStatusIdle},
		stopChan:         make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
		semaphore:        make(chan struct{}, concurrencyLimit), // Limit to a percentage of available CPUs
		embeddingClient:  client,
		vectorStore:      vectorStore,
		parser:           chunker.NewParser(chunkConfig),
		semanticChunker:  chunker.NewSemanticChunker(chunkConfig),
		debounceTimers:   make(map[string]*time.Timer),
		eventEmitter:     eventEmitter,
		embeddingModelID: modelID,
		taskChan:         taskChan,
	}, nil
}

// Run starts the indexing process.
// This method is intended to be run in a goroutine.
func (i *Indexer) Run(filePreviews []*models.FilePreview) {
	i.progress.Status = models.IndexingStatusIndexing
	atomic.StoreInt32(&i.progress.TotalFiles, int32(len(filePreviews)))
	atomic.StoreInt32(&i.progress.ProcessedFiles, 0)
	i.progress.CurrentFile = ""
	i.progress.Error = ""

	log.Printf("Starting indexing for project %s: %d files to process", i.project.Name, i.progress.TotalFiles)

	// Clean up artifacts for files that no longer exist.
	i.cleanupRemovedFiles(filePreviews)

	// Start GPU+DB worker goroutine using the centralized taskChan
	var gpuDone sync.WaitGroup
	gpuDone.Add(1)
	go func() {
		defer gpuDone.Done()
		i.gpuWorker(i.taskChan)
	}()

	// Defer closing taskChan so it's only closed when the whole Run goroutine finishes.
	// This keeps the GPU worker alive for the file watcher.
	defer func() {
		close(i.taskChan)
		gpuDone.Wait()
	}()

	// CPU workers: read, parse, and chunk files in parallel
	var cpuWg sync.WaitGroup
	var initialScanWg sync.WaitGroup
	for _, file := range filePreviews {
		initialScanWg.Add(1)
		cpuWg.Add(1)

		// Acquire semaphore BEFORE spawning the goroutine
		i.semaphore <- struct{}{}

		go func(file *models.FilePreview) {
			defer cpuWg.Done()
			defer func() { <-i.semaphore }()

			i.submitFileToIndices(file, &initialScanWg)
		}(file)
	}

	// Wait for all CPU workers to finish parsing
	cpuWg.Wait()
	
	// Wait for GPU worker to finish specifically the INITIAL scans
	initialScanWg.Wait()

	log.Printf("Initial indexing completed for project %s", i.project.Name)
	
	// Trigger the initial scan complete callback (Symbol Linker)
	if i.OnInitialScanComplete != nil {
		i.OnInitialScanComplete()
	}

	// --- Continuous Indexing (File Watching) ---
	if i.project.Config.ContinuousIndexing {
		log.Printf("Entering Continuous Indexing (File Watcher) mode for project %s", i.project.Name)
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
				
				// Standard skip check (handles both directories and files)
				if utils.ShouldSkipPath(i.project.Config.RootPath, p, i.project.Config.ExcludePatterns, i.project.Config.AutoExcludeHidden) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				if d.IsDir() {
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
				log.Printf("File watcher received stop signal (context cancelled) for project %s", i.project.Name)
				return
			case event, ok := <-i.watcher.Events:
				if !ok {
					log.Printf("File watcher events channel closed for project %s", i.project.Name)
					return
				}

				// Process Write and Create events
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					// Check if path is excluded by configuration
					if utils.ShouldSkipPath(i.project.Config.RootPath, event.Name, i.project.Config.ExcludePatterns, i.project.Config.AutoExcludeHidden) {
						continue
					}
					
					// Check if it's a supported file
					if i.parser.IsSupported(event.Name) {
						log.Printf("File changed in project %s: %s", i.project.Name, event.Name)
						i.debounceFileUpdate(event.Name)
					} else {
						// Check if it's a new directory
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							log.Printf("New directory created in project %s: %s, recursively adding to watcher", i.project.Name, event.Name)
							
							// Recursively add to watcher and index existing files in the new directory
							filepath.WalkDir(event.Name, func(p string, d os.DirEntry, err error) error {
								if err != nil {
									return nil
								}
								
								// Skip if matching exclusion patterns
								if utils.ShouldSkipPath(i.project.Config.RootPath, p, i.project.Config.ExcludePatterns, i.project.Config.AutoExcludeHidden) {
									if d.IsDir() {
										return filepath.SkipDir
									}
									return nil
								}

								if d.IsDir() {
									_ = i.watcher.Add(p)
								} else if i.parser.IsSupported(p) {
									// Trigger indexing for files found in the new directory
									i.debounceFileUpdate(p)
								}
								return nil
							})
						}
					}
				} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					// Handle deletion or renaming (which is effectively a deletion from the old path)
					log.Printf("File/Directory removed or renamed in project %s: %s", i.project.Name, event.Name)
					i.handleDeletion(event.Name)
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

// gpuWorker is the GPU pipeline consumer. It reads embeddingTasks from the
// channel, accumulates their chunks into a basket, and processes them in
// optimal batches of gpuBatchSize (32).
//
// Pre-tokenized basket: CPU workers tokenize chunks in parallel using
// TokenizeOne(). The GPU worker receives pre-tokenized data and only needs
// to assemble batch tensors + run GPU inference — zero CPU tokenization overhead.
func (i *Indexer) gpuWorker(taskChan <-chan *embeddingTask) {
	var basket []pendingItem
	taskRemaining := make(map[*embeddingTask]int)
	var saveWg sync.WaitGroup

	// Check if the embedding client supports pre-tokenized batches.
	onnxClient, canUseTokenized := i.embeddingClient.(*embedding.ONNXEmbeddingClient)
	if canUseTokenized {
		log.Printf("GPU Pipeline: Pre-tokenized basket mode (zero CPU in GPU worker)")
	}

	// handleResults maps embeddings back to their tasks and launches async finalization.
	handleResults := func(items []pendingItem, embeddings [][]float32, err error) {
		for j, item := range items {
			if err == nil && j < len(embeddings) {
				item.task.dbChunks[item.chunkIdx].Embedding = embeddings[j]
			}
			taskRemaining[item.task]--
			if taskRemaining[item.task] <= 0 {
				completedTask := item.task
				hasEmb := err == nil
				saveWg.Add(1)
				go func() {
					defer saveWg.Done()
					i.finalizeTask(completedTask, hasEmb)
					if completedTask.wg != nil {
						completedTask.wg.Done()
					}
				}()
				delete(taskRemaining, item.task)
			}
		}
		if err != nil {
			log.Printf("Failed to generate embeddings for batch: %v", err)
		}
	}

	// processBatch sends a batch of items to the GPU.
	// Uses pre-tokenized data if available, otherwise falls back to raw text.
	processBatch := func(items []pendingItem) {
		if len(items) == 0 {
			return
		}

		// Check if all items have pre-tokenized data
		allTokenized := canUseTokenized
		if allTokenized {
			for _, item := range items {
				if item.tokenized == nil {
					allTokenized = false
					break
				}
			}
		}

		if allTokenized {
			// Fast path: assemble pre-tokenized chunks → GPU only
			chunks := make([]*embedding.TokenizedChunk, len(items))
			for j, item := range items {
				chunks[j] = item.tokenized
			}
			embeddings, err := onnxClient.EmbedTokenizedBatch(chunks)
			handleResults(items, embeddings, err)
		} else {
			// Fallback: raw text → tokenize + GPU (for non-ONNX or failed tokenization)
			texts := make([]string, len(items))
			for j, item := range items {
				texts[j] = item.text
			}
			embeddings, err := i.embeddingClient.GenerateEmbeddings(texts)
			handleResults(items, embeddings, err)
		}
	}
	flushTimer := time.NewTimer(2 * time.Second)
	if !flushTimer.Stop() {
		<-flushTimer.C
	}
	defer flushTimer.Stop()

	for {
		terminate := false
		select {
		case <-i.ctx.Done():
			saveWg.Wait()
			return
		case task, ok := <-taskChan:
			if !ok {
				terminate = true
				break
			}

			// Reset flush timer whenever a new task arrives
			if !flushTimer.Stop() {
				select {
				case <-flushTimer.C:
				default:
				}
			}
			flushTimer.Reset(2 * time.Second)
			taskRemaining[task] = len(task.texts)
			for idx, text := range task.texts {
				item := pendingItem{task: task, chunkIdx: idx, text: text}
				if task.tokenizedChunks != nil && idx < len(task.tokenizedChunks) {
					item.tokenized = task.tokenizedChunks[idx]
				}
				basket = append(basket, item)
			}

			log.Printf("[%s] GPU Basket: +%d chunks from %s (basket: %d, queued tasks: %d)",
				i.project.Name, len(task.texts), task.filePath, len(basket), len(taskChan))

			// Process full batches of i.gpuBatchSize()
			for {
				currentBatchSize := i.gpuBatchSize()
				if len(basket) < currentBatchSize {
					break
				}
				batch := make([]pendingItem, currentBatchSize)
				copy(batch, basket[:currentBatchSize])
				basket = basket[currentBatchSize:]
				log.Printf("[%s] GPU Basket: Dispatching full batch of %d (remaining in basket: %d)",
					i.project.Name, currentBatchSize, len(basket))
				processBatch(batch)
			}

		case <-flushTimer.C:
			if len(basket) > 0 {
				log.Printf("[%s] GPU Basket: Flush timeout triggered! Dispatching partial batch of %d chunks", 
					i.project.Name, len(basket))
				processBatch(basket)
				basket = nil
			}
		}

		if terminate {
			break
		}
	}

	// Final flush of remaining chunks (channel closed)
	if len(basket) > 0 {
		log.Printf("[%s] GPU Basket: Final flush of %d chunks (channel closed)", i.project.Name, len(basket))
		processBatch(basket)
	}

	saveWg.Wait()
}

// finalizeTask saves a fully-embedded file's chunks and metadata to the database.
func (i *Indexer) finalizeTask(task *embeddingTask, hasEmbeddings bool) {
	if hasEmbeddings {
		log.Printf("Generated %d embeddings for file %s", len(task.dbChunks), task.filePath)
	}

	for _, dbChunk := range task.dbChunks {
		if err := i.vectorStore.InsertChunk(dbChunk); err != nil {
			log.Printf("Failed to save chunk for file %s: %v", task.filePath, err)
		}
	}

	if task.fileRecord != nil {
		if err := i.vectorStore.InsertFile(task.fileRecord); err != nil {
			log.Printf("Failed to save file metadata for %s: %v", task.filePath, err)
		}
	}

	log.Printf("Saved %d chunks for file %s to database", len(task.dbChunks), task.filePath)

	if task.storeOutline {
		i.storeOutlineForFile(task.absPath)
	}

	i.emitFileUpdate(task.filePath)
	atomic.AddInt32(&i.progress.ProcessedFiles, 1)

	// Trigger incremental linking for this file
	if i.OnFileIndexed != nil {
		i.OnFileIndexed(task.filePath)
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

// Stop gracefully stops the indexer.
func (i *Indexer) Stop() {
	log.Printf("Indexer.Stop() called for project %s", i.project.Name)
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
// This is called by the file watcher (via debounce) when a file is modified.
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

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		log.Printf("Failed to stat file %s: %v", absPath, err)
		return
	}

	preview := &models.FilePreview{
		AbsolutePath: absPath,
		RelativePath: relativePath,
		LastModified: fileInfo.ModTime().Unix(),
		Size:         utils.FormatBytes(fileInfo.Size()),
	}

	i.submitFileToIndices(preview, nil)
}

// submitFileToIndices encapsulates the common logic for parsing a file and sending it to the GPU/DB worker.
func (i *Indexer) submitFileToIndices(file *models.FilePreview, wg *sync.WaitGroup) {
	// Flag to track if the file successfully reached the GPU worker
	sentToGpu := false
	defer func() {
		if !sentToGpu && wg != nil {
			wg.Done()
		}
	}()

	select {
	case <-i.ctx.Done():
		return
	default:
	}

	i.progress.CurrentFile = file.RelativePath

	// Read file content
	source, err := os.ReadFile(file.AbsolutePath)
	if err != nil {
		log.Printf("Failed to read file %s: %v", file.AbsolutePath, err)
		atomic.AddInt32(&i.progress.ProcessedFiles, 1)
		return
	}

	// Check if file has changed since last indexing
	fileHash := utils.ComputeHash(source)
	existingFile, err := i.vectorStore.GetFile(file.RelativePath)
	if err == nil && existingFile != nil {
		if existingFile.Hash == fileHash && existingFile.LastModified == file.LastModified {
			atomic.AddInt32(&i.progress.ProcessedFiles, 1)
			return
		}
	}

	// File is new or has changed, delete existing chunks and re-index
	if err := i.vectorStore.DeleteFileChunks(file.RelativePath); err != nil {
		log.Printf("Failed to delete old chunks for %s: %v", file.RelativePath, err)
	}

	// Parse and chunk the file (CPU-intensive work)
	var chunkContents []string
	var dbChunks []*models.Chunk

	if i.semanticChunker.IsSupported(file.RelativePath) {
		semanticChunks, err := i.semanticChunker.ChunkFile(file.RelativePath, source)
		if err != nil {
			log.Printf("Failed to semantically chunk file %s: %v", file.AbsolutePath, err)
			return
		}

		chunkContents = make([]string, len(semanticChunks))
		dbChunks = make([]*models.Chunk, len(semanticChunks))

		for idx, chunk := range semanticChunks {
			chunkContents[idx] = chunk.Content

			dbChunks[idx] = &models.Chunk{
				ID:               chunk.ID,
				FilePath:         file.RelativePath,
				Content:          chunk.Content,
				LineStart:        int(chunk.StartLine),
				LineEnd:          int(chunk.EndLine),
				CharStart:        int(chunk.StartByte),
				CharEnd:          int(chunk.EndByte),
				Language:         chunk.Language,
				SymbolName:       chunk.SymbolName,
				SymbolKind:       string(chunk.SymbolKind),
				Parent:           chunk.Parent,
				Signature:        chunk.Signature,
				Visibility:       chunk.Visibility,
				PackageName:      chunk.PackageName,
				DocString:        chunk.DocString,
				TokenCount:       chunk.TokenCount,
				IsCollapsed:      chunk.IsCollapsed,
				SourceCode:       chunk.SourceCode,
				EmbeddingModelID: i.embeddingModelID,
			}
		}
		log.Printf("[%s] Created %d semantic chunks for file %s", i.project.Name, len(semanticChunks), file.RelativePath)
	} else {
		simpleChunks, err := utils.ChunkFile(file.AbsolutePath, i.project.Config.ChunkSizeMax)
		if err != nil {
			log.Printf("Failed to chunk file %s: %v", file.AbsolutePath, err)
			return
		}

		chunkContents = make([]string, len(simpleChunks))
		dbChunks = make([]*models.Chunk, len(simpleChunks))

		for idx, chunk := range simpleChunks {
			chunkContents[idx] = chunk.Content

			dbChunks[idx] = &models.Chunk{
				ID:               utils.GenerateSymbolID(file.RelativePath, uint32(chunk.LineStart), uint32(chunk.LineEnd), "chunk", idx+1),
				FilePath:         file.RelativePath,
				Content:          chunk.Content,
				LineStart:        chunk.LineStart,
				LineEnd:          chunk.LineEnd,
				CharStart:        chunk.CharacterStart,
				CharEnd:          chunk.CharacterEnd,
				EmbeddingModelID: i.embeddingModelID,
			}
		}
		log.Printf("[%s] Created %d simple chunks for file %s (unsupported format)", i.project.Name, len(simpleChunks), file.RelativePath)
	}

	if len(chunkContents) == 0 {
		return
	}

	// Pre-tokenize chunks in the CPU worker
	var tokenizedChunks []*embedding.TokenizedChunk
	if onnxClient, ok := i.embeddingClient.(*embedding.ONNXEmbeddingClient); ok {
		tokenizedChunks = make([]*embedding.TokenizedChunk, len(chunkContents))
		for idx, text := range chunkContents {
			tc, err := onnxClient.TokenizeOne(text)
			if err != nil {
				log.Printf("Failed to tokenize chunk %d for file %s: %v", idx, file.RelativePath, err)
				tokenizedChunks = nil // fall back to raw text
				break
			}
			tokenizedChunks[idx] = tc
		}
	}

	// Send task to GPU pipeline
	select {
	case i.taskChan <- &embeddingTask{
		filePath:        file.RelativePath,
		absPath:         file.AbsolutePath,
		dbChunks:        dbChunks,
		texts:           chunkContents,
		tokenizedChunks: tokenizedChunks,
		fileRecord: &models.File{
			Path:         file.RelativePath,
			Hash:         fileHash,
			LastModified: file.LastModified,
			ChunkCount:   len(dbChunks),
		},
		storeOutline: i.project.Config.ContinuousIndexing,
		wg:           wg,
	}:
		sentToGpu = true
		if wg == nil { // Incrementar counter per live updates (Run lo fa solo per initialScan)
			atomic.AddInt32(&i.progress.TotalFiles, 1)
		}
	case <-i.ctx.Done():
		return
	}
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
	roots, allNodes := outline.BuildOutlineNodes(relativePath, result.Symbols)
	if err := i.vectorStore.UpsertFileOutline(relativePath, roots); err != nil {
		log.Printf("Failed to persist outline for %s: %v", absPath, err)
	}

	// Save symbol usages (invocations)
	if len(result.Usages) > 0 {
		for _, u := range result.Usages {
			// Find caller node id from allNodes
			var callerNodeID string
			for _, node := range allNodes {
				// Match by name AND line range (safest)
				if node.Name == u.Caller && u.Line >= node.StartLine && u.Line <= node.EndLine {
					callerNodeID = node.ID
					break
				}
			}

			if callerNodeID != "" {
				usage := &models.SymbolUsage{
					CallerNodeID:    callerNodeID,
					RawTargetName:    u.Name,
					RawTargetContext: u.Context,
					Line:            int(u.Line),
					Column:          int(u.Column),
				}
				if err := i.vectorStore.InsertSymbolUsage(usage); err != nil {
					log.Printf("Failed to insert symbol usage in %s: %v", relativePath, err)
				}
			}
		}
	}

	absKey := filepath.ToSlash(absPath)
	if absKey != relativePath {
		// Remove any legacy absolute-path outline/symbol/chunk records without touching the new relative entry.
		if err := i.vectorStore.RemoveFileAndArtifacts(absKey); err != nil && !strings.Contains(err.Error(), "file not found") {
			log.Printf("Failed to remove legacy outline key %s: %v", absKey, err)
		}
	}

	// Save individual symbols to symbols table
	if len(result.Symbols) > 0 {
		// Delete old symbols for this file
		if err := i.vectorStore.DeleteFileSymbols(relativePath); err != nil {
			log.Printf("Failed to delete old symbols for %s: %v", relativePath, err)
		}

		// Insert new symbols
		idCounters := make(map[string]int)
		for _, parsedSymbol := range result.Symbols {
			idKey := fmt.Sprintf("%s:%d:%d:%s", relativePath, parsedSymbol.StartLine, parsedSymbol.EndLine, parsedSymbol.Name)
			idCounters[idKey]++

			symbol := &models.Symbol{
				ID:        utils.GenerateSymbolID(relativePath, parsedSymbol.StartLine, parsedSymbol.EndLine, parsedSymbol.Name, idCounters[idKey]),
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

// cleanupRemovedFiles deletes stored artifacts for files missing from disk or excluded from index.
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
		
		// If a file is tracked in the database but is NOT in the currentFiles list,
		// it means it was either deleted physically OR it has been excluded via
		// project configuration/gitignore patterns. We should remove its artifacts.
		if err := i.vectorStore.RemoveFileAndArtifacts(path); err != nil {
			log.Printf("Failed to remove stale artifacts for %s: %v", path, err)
			continue
		}
		log.Printf("Removed stale artifacts for missing or excluded file %s", path)
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

// handleDeletion removes a file or directory from the index database and updates progress.
func (i *Indexer) handleDeletion(absPath string) {
	if i.vectorStore == nil {
		return
	}

	// Calculate relative path for database deletion
	relativePath := filepath.ToSlash(absPath)
	if rel, ok := utils.RelativePathWithinRoot(i.project.Config.RootPath, absPath); ok && rel != "" {
		relativePath = rel
	}

	// We don't know if it was a file or directory because it's already gone from disk.
	// So we attempt to remove both. RemoveFileAndArtifacts is specific, 
	// while RemoveDirectoryAndArtifacts uses a prefix match.
	
	// Track if we actually removed anything to update progress counters
	file, _ := i.vectorStore.GetFile(relativePath)
	wasFile := file != nil

	// Remove from DB
	_ = i.vectorStore.RemoveFileAndArtifacts(relativePath)
	_ = i.vectorStore.RemoveDirectoryAndArtifacts(relativePath)

	if wasFile {
		// Update progress counters if a single file was removed
		atomic.AddInt32(&i.progress.TotalFiles, -1)
		atomic.AddInt32(&i.progress.ProcessedFiles, -1)
		
		// Ensure counters don't go negative
		if i.progress.TotalFiles < 0 {
			i.progress.TotalFiles = 0
		}
		if i.progress.ProcessedFiles < 0 {
			i.progress.ProcessedFiles = 0
		}

		log.Printf("[%s] Removed file from index: %s (Progress: %d/%d)", 
			i.project.Name, relativePath, i.progress.ProcessedFiles, i.progress.TotalFiles)
	} else {
		log.Printf("[%s] Attempted to remove path/directory from index: %s. Refreshing project stats is recommended.", 
			i.project.Name, relativePath)
		
		// For directory deletions, we might have removed many files.
		// A full re-count of TotalFiles/ProcessedFiles would be ideal here if performance allows,
		// but for now we just log it. The next app restart or manual re-index will fix it anyway.
	}

	// Trigger UI update
	i.emitFileUpdate(relativePath)
}
