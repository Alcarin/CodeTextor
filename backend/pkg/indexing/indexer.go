package indexing

import (
	"CodeTextor/backend/internal/chunker"
	"CodeTextor/backend/pkg/outline"
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/embedding"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Indexer is responsible for indexing a single project.
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
	debounceMu       sync.Mutex
	debounceTimers   map[string]*time.Timer
	eventEmitter     func(string, interface{})
	embeddingModelID string
	isInitialScan    bool
	
	// Callbacks for external components
	OnInitialScanComplete func()
	OnFileIndexed         func(filePath string)

	dbWriteMu   sync.Mutex // Serializes writes to the single-writer SQLite DB
	progressMu  sync.RWMutex // Protects access to non-atomic progress fields
}

// embeddingTask holds pre-processed file data ready for GPU embedding.
type embeddingTask struct {
	filePath     string
	absPath      string
	dbChunks     []*models.Chunk
	dbSymbols    []*models.Symbol
	dbUsages     []*models.SymbolUsage
	outlineNodes []*models.OutlineNode
	rawChunks    []string
	fileRecord   *models.File
	storeOutline bool
	isNewFile    bool
	wg           *sync.WaitGroup
}

// NewIndexer creates a new Indexer for a project.
func NewIndexer(project *models.Project, vectorStore *store.VectorStore, client embedding.EmbeddingClient, eventEmitter func(string, interface{})) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	chunkConfig := chunker.ChunkConfig{
		MaxChunkSize:      project.Config.ChunkSizeMax,
		MinChunkSize:      project.Config.ChunkSizeMin,
		CollapseThreshold: 500,
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

	concurrencyLimit := runtime.NumCPU()
	if concurrencyLimit < 2 {
		concurrencyLimit = 2
	}

	return &Indexer{
		project:          project,
		progress:         &models.IndexingProgress{Status: models.IndexingStatusIdle},
		stopChan:         make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
		semaphore:        make(chan struct{}, concurrencyLimit),
		embeddingClient:  client,
		vectorStore:      vectorStore,
		parser:           chunker.NewParser(chunkConfig),
		semanticChunker:  chunker.NewSemanticChunker(chunkConfig),
		debounceTimers:   make(map[string]*time.Timer),
		eventEmitter:     eventEmitter,
		embeddingModelID: modelID,
	}, nil
}

func (i *Indexer) setProgressStatus(status models.IndexingStatus) {
	i.progressMu.Lock()
	defer i.progressMu.Unlock()
	i.progress.Status = status
}

func (i *Indexer) setProgressCurrentFile(filePath string) {
	i.progressMu.Lock()
	defer i.progressMu.Unlock()
	i.progress.CurrentFile = filePath
}

func (i *Indexer) setProgressError(err string) {
	i.progressMu.Lock()
	defer i.progressMu.Unlock()
	i.progress.Error = err
}

func (i *Indexer) GetProgress() models.IndexingProgress {
	i.progressMu.RLock()
	defer i.progressMu.RUnlock()
	
	return models.IndexingProgress{
		TotalFiles:     atomic.LoadInt32(&i.progress.TotalFiles),
		ProcessedFiles: atomic.LoadInt32(&i.progress.ProcessedFiles),
		CurrentFile:    i.progress.CurrentFile,
		Status:         i.progress.Status,
		Error:          i.progress.Error,
	}
}

// Run starts the indexing process.
func (i *Indexer) Run(filePreviews []*models.FilePreview) {
	i.isInitialScan = true
	i.setProgressStatus(models.IndexingStatusIndexing)
	atomic.StoreInt32(&i.progress.TotalFiles, int32(len(filePreviews)))
	atomic.StoreInt32(&i.progress.ProcessedFiles, 0)
	i.setProgressCurrentFile("")
	i.setProgressError("")

	log.Printf("Starting indexing for project %s: %d files to process", i.project.Name, len(filePreviews))

	// Clean up artifacts for files that no longer exist.
	i.cleanupRemovedFiles(filePreviews)

	var cpuWg sync.WaitGroup
	var initialScanWg sync.WaitGroup
	var globalWorkerID int32
	
	for _, file := range filePreviews {
		initialScanWg.Add(1)
		cpuWg.Add(1)

		i.semaphore <- struct{}{}
		
		workerID := atomic.AddInt32(&globalWorkerID, 1)

		go func(file *models.FilePreview, wid int32) {
			defer cpuWg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[%s] CRITICAL PANIC during CPU stage for %s: %v", i.project.Name, file.RelativePath, r)
					initialScanWg.Done()
					atomic.AddInt32(&i.progress.ProcessedFiles, 1)
				}
			}()
			
			// 1. CPU Stage: Parser & Tokenizer
			log.Printf("[%s] Starting CPU stage: %s", i.project.Name, file.RelativePath)
			task := i.submitFileToIndices(file, wid)
			
			// 2. Release CPU semaphore early!
			<-i.semaphore

			// 3. GPU + DB Stage: Inference & Persistence (Non-blocking for CPU workers)
			if task != nil {
				go i.processTask(task, &initialScanWg)
			} else {
				initialScanWg.Done()
				atomic.AddInt32(&i.progress.ProcessedFiles, 1) // Increment for skipped files too
			}
		}(file, workerID)
	}

	initialScanWg.Wait()
	i.isInitialScan = false

	log.Printf("[%s] Initial scan complete. Processed: %d, Errors: %v", i.project.Name, i.progress.ProcessedFiles, i.getProgressError())
	
	if i.OnInitialScanComplete != nil {
		i.OnInitialScanComplete()
	}

	if i.project.Config.ContinuousIndexing {
		log.Printf("[%s] Starting file watcher for continuous indexing...", i.project.Name)
		i.startWatcher()
	} else {
		i.setProgressStatus(models.IndexingStatusCompleted)
	}
}

func (i *Indexer) startWatcher() {
	log.Printf("Entering Continuous Indexing (File Watcher) mode for project %s", i.project.Name)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create file watcher: %v", err)
		return
	}
	i.watcher = watcher
	defer i.watcher.Close()

	includePaths := resolveIncludePaths(i.project.Config.RootPath, i.project.Config.IncludePaths)
	for _, path := range includePaths {
		filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				if !utils.ShouldSkipPath(i.project.Config.RootPath, p, i.project.Config.ExcludePatterns, i.project.Config.AutoExcludeHidden) {
					_ = i.watcher.Add(p)
				} else {
					return filepath.SkipDir
				}
			}
			return nil
		})
	}

	i.setProgressStatus(models.IndexingStatusIdle)

	for {
		select {
		case <-i.ctx.Done():
			return
		case event, ok := <-i.watcher.Events:
			if !ok { return }
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				if !utils.ShouldSkipPath(i.project.Config.RootPath, event.Name, i.project.Config.ExcludePatterns, i.project.Config.AutoExcludeHidden) {
					if i.parser.IsSupported(event.Name) {
						i.debounceFileUpdate(event.Name)
					} else if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						i.watcher.Add(event.Name)
						// Add recursion if needed...
					}
				}
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				i.handleDeletion(event.Name)
			}
		case err, ok := <-i.watcher.Errors:
			if !ok { return }
			log.Printf("Watcher error: %v", err)
		}
	}
}

// processTask handles the GPU and DB stages for a file task.
func (i *Indexer) processTask(task *embeddingTask, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] CRITICAL PANIC during GPU/DB stage for %s: %v", i.project.Name, task.filePath, r)
			atomic.AddInt32(&i.progress.ProcessedFiles, 1)
		}
	}()

	if i.ctx.Err() != nil {
		atomic.AddInt32(&i.progress.ProcessedFiles, 1) // Count it as processed (cancelled) to allow 100%
		return
	}

	// 1. GPU Stage: Generate Embeddings
	i.setProgressCurrentFile(task.filePath)
	log.Printf("[%s] Starting GPU stage: %s", i.project.Name, task.filePath)
	embeddings, err := i.embeddingClient.GenerateEmbeddings(task.rawChunks)
	if err != nil {
		log.Printf("[%s] Failed to generate embeddings for %s: %v", i.project.Name, task.filePath, err)
		atomic.AddInt32(&i.progress.ProcessedFiles, 1)
		return
	}

	for idx, emb := range embeddings {
		if idx < len(task.dbChunks) {
			task.dbChunks[idx].Embedding = emb
		}
	}

	// 2. DB Stage: Atomic Transaction
	i.dbWriteMu.Lock()
	isNewActual, err := i.vectorStore.InsertFileTasksInTransaction(
		task.fileRecord,
		task.dbChunks,
		task.dbSymbols,
		task.outlineNodes,
		task.dbUsages,
	)
	i.dbWriteMu.Unlock()

	if err != nil {
		log.Printf("[%s] Failed to persist file %s: %v", i.project.Name, task.filePath, err)
		return
	}

	processed := atomic.LoadInt32(&i.progress.ProcessedFiles)
	if isNewActual || i.isInitialScan {
		processed = atomic.AddInt32(&i.progress.ProcessedFiles, 1)
	}
	total := atomic.LoadInt32(&i.progress.TotalFiles)
	
	if i.OnFileIndexed != nil && !i.isInitialScan {
		i.OnFileIndexed(task.filePath)
	}
	i.emitFileUpdate(task.filePath)
	
	percent := 0.0
	if total > 0 {
		percent = float64(processed) / float64(total) * 100
	}
	i.setProgressCurrentFile(task.filePath)
	log.Printf("[%s] Indexed (%d/%d, %.1f%%): %s", i.project.Name, processed, total, percent, task.filePath)
}

func (i *Indexer) getProgressError() string {
	i.progressMu.RLock()
	defer i.progressMu.RUnlock()
	return i.progress.Error
}

func (i *Indexer) submitFileToIndices(file *models.FilePreview, workerID int32) *embeddingTask {
	i.setProgressCurrentFile(file.RelativePath)
	
	


	sourceRaw, err := os.ReadFile(file.AbsolutePath)
	if err != nil {
		log.Printf("[%s] Error reading %s: %v", i.project.Name, file.AbsolutePath, err)
		return nil
	}
	
	source := sourceRaw

	fileHash := utils.ComputeHash(source)
	existingFile, err := i.vectorStore.GetFile(file.RelativePath)
	if err == nil && existingFile != nil {
		if existingFile.Hash == fileHash && existingFile.LastModified == file.LastModified {
			return nil // Don't increment here, let Run handle it
		}
	}

	var dbChunks []*models.Chunk
	var dbSymbols []*models.Symbol
	var dbUsages []*models.SymbolUsage
	var outlineNodes []*models.OutlineNode

	if i.semanticChunker.IsSupported(file.RelativePath) {
		semanticChunks, parseResult, err := i.semanticChunker.ChunkFileWithResult(file.RelativePath, source)
		if err != nil {
			log.Printf("[%s] Error chunking %s: %v", i.project.Name, file.RelativePath, err)
			return nil // Incremented by Run
		}

		dbChunks = make([]*models.Chunk, len(semanticChunks))
		for idx, chunk := range semanticChunks {
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

		dbSymbols = make([]*models.Symbol, 0, len(parseResult.Symbols))
		idCounters := make(map[string]int)
		for _, sym := range parseResult.Symbols {
			idKey := fmt.Sprintf("%s:%d:%d:%s", file.RelativePath, sym.StartLine, sym.EndLine, sym.Name)
			idCounters[idKey]++
			dbSymbols = append(dbSymbols, &models.Symbol{
				ID:         utils.GenerateSymbolID(file.RelativePath, sym.StartLine, sym.EndLine, sym.Name, idCounters[idKey]),
				FilePath:   file.RelativePath,
				Name:       sym.Name,
				Kind:       string(sym.Kind),
				Line:       int(sym.StartLine),
				Character:  int(sym.StartByte),
				Parent:     sym.Parent,
				Implements: sym.Implements,
				Language:   sym.Language,
			})
		}

		outlineNodes, _ = outline.BuildOutlineNodes(file.RelativePath, parseResult.Symbols)
		
		symbolLookup := make(map[string][]*models.Symbol)
		for _, ds := range dbSymbols {
			symbolLookup[ds.Name] = append(symbolLookup[ds.Name], ds)
		}

		// Sort candidates by line to allow faster lookup
		for _, candidates := range symbolLookup {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].Line < candidates[j].Line
			})
		}

		for _, u := range parseResult.Usages {
			var callerID string
			if candidates, ok := symbolLookup[u.Caller]; ok {
				// Find the best candidate (the one starting closest but before the usage line)
				for _, cand := range candidates {
					if cand.Line <= int(u.Line) {
						callerID = cand.ID
					} else {
						// Since candidates are sorted, we can stop here
						break
					}
				}
			}
			if callerID == "" {
				callerID = utils.GenerateSymbolID(file.RelativePath, 0, 0, "root", 1)
			}
			
			var targetID string
			if candidates, ok := symbolLookup[u.Name]; ok && len(candidates) == 1 {
				targetID = candidates[0].ID
			}

			dbUsages = append(dbUsages, &models.SymbolUsage{
				FilePath:         file.RelativePath,
				CallerNodeID:     callerID,
				TargetNodeID:     targetID,
				RawTargetName:    u.Name,
				RawTargetContext: u.Context,
				Line:             int(u.Line),
				Column:           int(u.Column),
			})
		}

	} else {
		simpleChunks, _ := utils.ChunkFile(file.AbsolutePath, i.project.Config.ChunkSizeMax)
		dbChunks = make([]*models.Chunk, len(simpleChunks))
		for idx, chunk := range simpleChunks {
			dbChunks[idx] = &models.Chunk{
				ID:               utils.GenerateSymbolID(file.RelativePath, uint32(chunk.LineStart), uint32(chunk.LineEnd), "chunk", idx+1),
				FilePath:         file.RelativePath,
				Content:          chunk.Content,
				LineStart:        chunk.LineStart,
				LineEnd:          chunk.LineEnd,
				Language:         "text",
				EmbeddingModelID: i.embeddingModelID,
			}
		}
	}



	rawChunks := make([]string, len(dbChunks))
	for idx, chunk := range dbChunks {
		rawChunks[idx] = chunk.Content
	}

	return &embeddingTask{
		filePath:     file.RelativePath,
		absPath:      file.AbsolutePath,
		dbChunks:     dbChunks,
		dbSymbols:    dbSymbols,
		dbUsages:     dbUsages,
		outlineNodes: outlineNodes,
		rawChunks:    rawChunks,
		fileRecord: &models.File{
			ID:           utils.NormalizeRelativePath(file.RelativePath),
			Path:         file.RelativePath,
			Hash:         fileHash,
			LastModified: file.LastModified,
			SizeBytes:    int64(len(source)),
			ChunkCount:   len(dbChunks),
		},
		storeOutline: true,
		isNewFile:    existingFile == nil,
	}
}

func (i *Indexer) Stop() {
	i.debounceMu.Lock()
	for _, timer := range i.debounceTimers { timer.Stop() }
	i.debounceTimers = make(map[string]*time.Timer)
	i.debounceMu.Unlock()
	if i.watcher != nil { i.watcher.Close() }
	i.cancel()
}

func (i *Indexer) debounceFileUpdate(filePath string) {
	i.debounceMu.Lock()
	defer i.debounceMu.Unlock()
	if timer, exists := i.debounceTimers[filePath]; exists { timer.Stop() }
	i.debounceTimers[filePath] = time.AfterFunc(2*time.Second, func() {
		i.updateFileIndex(filePath)
		i.debounceMu.Lock()
		delete(i.debounceTimers, filePath)
		i.debounceMu.Unlock()
	})
}

func (i *Indexer) updateFileIndex(filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("[%s] Indexer: invalid path %s: %v", i.project.Name, filePath, err)
		return
	}
	relPath := filePath
	if rel, ok := utils.RelativePathWithinRoot(i.project.Config.RootPath, absPath); ok {
		relPath = rel
	}
	
	info, err := os.Stat(absPath)
	if err != nil {
		// File vanished or inaccessible - stop indexing it
		if os.IsNotExist(err) {
			log.Printf("[%s] Indexer: file %s vanished during debounce, skipping", i.project.Name, relPath)
		} else {
			log.Printf("[%s] Indexer: error statting file %s: %v", i.project.Name, relPath, err)
		}
		return
	}

	// Just in case it's a directory
	if info.IsDir() {
		return
	}

	preview := &models.FilePreview{
		AbsolutePath: absPath,
		RelativePath: relPath,
		LastModified: info.ModTime().Unix(),
	}
	
	task := i.submitFileToIndices(preview, 0)
	if task != nil {
		if task.isNewFile {
			atomic.AddInt32(&i.progress.TotalFiles, 1)
		}
		go i.processTask(task, nil)
	}
}

func (i *Indexer) cleanupRemovedFiles(currentFiles []*models.FilePreview) {
	tracked, _ := i.vectorStore.ListPhysicalFilePaths()
	current := make(map[string]bool)
	for _, f := range currentFiles { current[f.RelativePath] = true }
	for _, p := range tracked {
		if !current[p] {
			_, _ = i.vectorStore.RemoveFileAndArtifacts(p)
		}
	}
}

func (i *Indexer) emitFileUpdate(filePath string) {
	if i.eventEmitter != nil {
		i.eventEmitter("project:fileIndexed", map[string]interface{}{
			"projectId": i.project.ID,
			"filePath":  filePath,
		})
	}
}

func (i *Indexer) handleDeletion(absPath string) {
	relPath := absPath
	if rel, ok := utils.RelativePathWithinRoot(i.project.Config.RootPath, absPath); ok { relPath = rel }
	
	// Serializzare le eliminazioni per evitare 'database is locked'
	i.dbWriteMu.Lock()
	defer i.dbWriteMu.Unlock()

	// Atomically remove from DB and get the count of physical files actually removed.
	removedFiles, err := i.vectorStore.RemoveFileAndArtifacts(relPath)
	if err != nil {
		log.Printf("[%s] Indexer: failed to remove file %s: %v", i.project.Name, relPath, err)
	}

	// Only attempt directory removal if no single file was removed or if we want to be exhaustive.
	// Actually, a path could be both a file name in one project and a directory prefix in another,
	// but here we just check if it's likely a directory or if RemoveFile found nothing.
	var removedFromDir int64
	if removedFiles == 0 {
		removedFromDir, err = i.vectorStore.RemoveDirectoryAndArtifacts(relPath)
		if err != nil {
			log.Printf("[%s] Indexer: failed to remove directory %s: %v", i.project.Name, relPath, err)
		}
	}

	totalRemoved := removedFiles + removedFromDir
	if totalRemoved > 0 {
		atomic.AddInt32(&i.progress.TotalFiles, -int32(totalRemoved))
		atomic.AddInt32(&i.progress.ProcessedFiles, -int32(totalRemoved))
	}

	i.emitFileUpdate(relPath)
}

func resolveIncludePaths(root string, includes []string) []string {
	if len(includes) == 0 { return []string{root} }
	var res []string
	for _, inc := range includes {
		res = append(res, filepath.Join(root, inc))
	}
	return res
}
