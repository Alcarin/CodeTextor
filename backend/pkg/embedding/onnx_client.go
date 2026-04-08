package embedding

import (
	"CodeTextor/backend/pkg/models"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	onnx "github.com/yalue/onnxruntime_go"
)

var (
	onnxRuntimeInitOnce     sync.Once
	onnxRuntimeInitErr      error
	onnxSharedLibraryPath   string
	activeSharedLibraryPath string
	activeExecutionProvider string = "CPU" // Default fallback
)

// ONNXEmbeddingClient uses ONNX Runtime + HuggingFace tokenizer.json files to compute embeddings.
type ONNXEmbeddingClient struct {
	session          *onnx.DynamicAdvancedSession
	tokenizer        *tokenizer.Tokenizer
	padID            int
	padTypeID        int
	padToken         string
	padDirection     tokenizer.PaddingDirection
	maxSeqLen        int
	inputNames       []string
	outputNames      []string
	expectTokenTypes bool
	dimension        int
	batchSize        int // Current batch size, can be rebalanced by ProjectService
	modelID          string
	mu               sync.Mutex
	lastUsed         time.Time
	chunkChan        chan chunkJob
	stopChan         chan struct{}
	wg               sync.WaitGroup
	pendingCount     atomic.Int32
	closed           atomic.Bool

	// Global Throughput Stats
	totalTokenized atomic.Int64
	totalEmbedded  atomic.Int64
	lastEfficiency atomic.Int32 // Efficiency percentage (0-100) of the last GPU batch
	startTime      time.Time

	// Centralized Tokenization Pipeline
	tokenizerChan chan tokenizerJob
	gpuWaitTime   atomic.Int64 // Cumulative nanoseconds GPU spent waiting for work
}

type tokenizerJob struct {
	text       string
	resultChan chan *TokenizedChunk
}

// computeOptimalBatchSize determines the batch size based on AVAILABLE GPU VRAM.
func computeOptimalBatchSize() int {
	provider := GetActiveExecutionProvider()
	if provider == "CPU" {
		return 4
	}

	// Important: we query the AVAILABLE VRAM at startup to avoid overlapping with other apps.
	vramMB := detectAvailableGPUVRAM()
	if vramMB <= 0 {
		// Could not detect, use safe fallback
		log.Printf("GPU VRAM: Detection failed or not implemented, using default batch size 32")
		return 32
	}

	vramGB := float64(vramMB) / 1024.0
	// Formula: target = vramGB * 8.0
	// Then round to the nearest power of 2: 2^round(log2(vramGB * 8.0))

	target := vramGB * 8.0
	exponent := math.Round(math.Log2(target))
	batchSize := int(math.Pow(2, exponent))

	// Safety clamp: keep between 8 (min performance) and 128 (stability limit)
	if batchSize < 8 {
		batchSize = 8
	}
	if batchSize > 128 {
		batchSize = 128
	}

	log.Printf("GPU VRAM Available: %.1f GB → Batch scaling factor: 8.0 → Batch size: %d", vramGB, batchSize)
	return batchSize
}

// detectAvailableGPUVRAM returns the available dedicated GPU VRAM in megabytes.
func detectAvailableGPUVRAM() int {
	return DetectAvailableGPUVRAM()
}

// GetBatchSize returns the current batch size for this client instance.
func (c *ONNXEmbeddingClient) GetBatchSize() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.batchSize
}

// SetBatchSize updates the batch size for this client instance.
func (c *ONNXEmbeddingClient) SetBatchSize(batchSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if batchSize < 1 {
		batchSize = 1
	}
	c.batchSize = batchSize
}

// ModelID returns the unique identifier for the model used by this client.
func (c *ONNXEmbeddingClient) ModelID() string {
	return c.modelID
}
// NewONNXEmbeddingClient constructs an embedding client backed by an ONNX model.
func NewONNXEmbeddingClient(meta *models.EmbeddingModelInfo) (*ONNXEmbeddingClient, error) {
	if meta == nil {
		return nil, fmt.Errorf("embedding metadata is required")
	}
	if strings.TrimSpace(meta.LocalPath) == "" {
		return nil, fmt.Errorf("model %s is missing a local ONNX path", meta.ID)
	}
	if strings.TrimSpace(meta.TokenizerLocalPath) == "" {
		return nil, fmt.Errorf("model %s is missing a tokenizer.json path", meta.ID)
	}
	if err := ensureONNXRuntimeInitialized(); err != nil {
		log.Printf("ONNX: Failed to initialize runtime: %v", err)
		return nil, err
	}
	log.Printf("ONNX: Runtime initialized. Provider: %s", GetActiveExecutionProvider())

	// Compute optimal batch size based on ACTUAL available VRAM at the moment of client creation.
	// This helps adapt to changing GPU load (e.g. after model unloads/leaking memory).
	batchSize := computeOptimalBatchSize()

	log.Printf("ONNX: Loading tokenizer from %s", meta.LocalPath)
	tk, err := pretrained.FromFile(meta.TokenizerLocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer for %s: %w", meta.ID, err)
	}

	maxSeq := meta.MaxSequenceLength
	if maxSeq <= 0 {
		maxSeq = 512
	}
	padParams := tk.GetPadding()
	padID := 0
	padType := 0
	padToken := "[PAD]"
	padDirection := tokenizer.Right
	if padParams != nil {
		padID = padParams.PadId
		padType = padParams.PadTypeId
		if padParams.PadToken != "" {
			padToken = padParams.PadToken
		}
		padDirection = padParams.Direction
	}

	// Forziamo l'ispezione dei metadati sulla CPU per evitare spike di VRAM all'avvio.
	// Una SessionOptions appena creata senza provider aggiunti usa di default la CPU.
	infoOptions, _ := onnx.NewSessionOptions()
	defer infoOptions.Destroy()

	log.Printf("ONNX: Inspecting model %s", meta.LocalPath)
	inputInfo, outputInfo, err := onnx.GetInputOutputInfoWithOptions(meta.LocalPath, infoOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect ONNX file %s: %w", meta.LocalPath, err)
	}
	if len(inputInfo) == 0 || len(outputInfo) == 0 {
		return nil, fmt.Errorf("ONNX model %s is missing inputs or outputs", meta.LocalPath)
	}

	inputNames := make([]string, len(inputInfo))
	for i, info := range inputInfo {
		inputNames[i] = info.Name
	}
	outputNames := make([]string, len(outputInfo))
	for i, info := range outputInfo {
		outputNames[i] = info.Name
	}

	log.Printf("ONNX: Creating session for %s...", meta.ID)
	session, err := newONNXSessionWithBestProvider(meta.LocalPath, inputNames, outputNames)
	if err != nil {
		log.Printf("ONNX: Failed to create session: %v", err)
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}
	log.Printf("ONNX: Session created. Batch Size: %d", batchSize)

	client := &ONNXEmbeddingClient{
		session:          session,
		tokenizer:        tk,
		padID:            padID,
		padTypeID:        padType,
		padToken:         padToken,
		padDirection:     padDirection,
		maxSeqLen:        maxSeq,
		inputNames:       inputNames,
		outputNames:      outputNames,
		expectTokenTypes: hasTokenTypeInput(inputNames),
		dimension:        meta.Dimension,
		batchSize:        batchSize,
		modelID:          meta.ID,
		chunkChan:        make(chan chunkJob, 2048), // Large buffer for atomic chunks (the "basket")
		stopChan:         make(chan struct{}),
		lastUsed:         time.Now(),
		startTime:        time.Now(),
		tokenizerChan:    make(chan tokenizerJob, 2048),
	}
	client.wg.Add(1)
	go client.inferenceWorker()
	
	// Start Tokenizer Worker Pool (Micro-Batching enabled)
	concurrency := runtime.NumCPU()
	for i := 0; i < concurrency; i++ {
		client.wg.Add(1)
		go client.tokenizerWorker()
	}
	


	return client, nil
}

func (c *ONNXEmbeddingClient) tokenizerWorker() {
	defer c.wg.Done()
	
	const maxBatchSize = 32
	const flushInterval = 10 * time.Millisecond
	
	batch := make([]tokenizerJob, 0, maxBatchSize)
	timer := time.NewTimer(flushInterval)
	if !timer.Stop() {
		<-timer.C
	}
	
	processBuffer := func() {
		if len(batch) == 0 {
			return
		}
		
		// 1. Prepare batch for library
		sanitizedInputs := make([]tokenizer.EncodeInput, len(batch))
		rawTexts := make([]string, len(batch))
		for i, job := range batch {
			s := sanitizeTokenizerInput(job.text)
			rawTexts[i] = s
			sanitizedInputs[i] = tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(s))
		}
		
		// 2. Call EncodeBatch with recover
		encodings, err := func() (encs []tokenizer.Encoding, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("tokenizer batch crashed")
				}
			}()
			return c.tokenizer.EncodeBatch(sanitizedInputs, true)
		}()
		
		// 3. Process results
		if err != nil {
			for _, job := range batch {
				job.resultChan <- nil
			}
		} else {
			for i := range encodings {
				encoding := &encodings[i]
				c.normalizeEncoding(encoding)
				
				if encoding.Len() > c.maxSeqLen {
					truncated, _ := encoding.Truncate(c.maxSeqLen, 0)
					if truncated != nil {
						encoding = truncated
					}
				}
				if encoding.Len() < c.maxSeqLen {
					encoding = encoding.Pad(c.maxSeqLen, c.padID, c.padTypeID, c.padToken, c.padDirection)
				}
				
				rawIDs := encoding.GetIds()
				rawMasks := encoding.GetAttentionMask()
				rawTypes := encoding.GetTypeIds()

				ids := make([]int64, c.maxSeqLen)
				masks := make([]int64, c.maxSeqLen)
				types := make([]int64, c.maxSeqLen)

				for j := 0; j < c.maxSeqLen; j++ {
					if j < len(rawIDs) {
						ids[j] = int64(rawIDs[j])
					} else {
						ids[j] = int64(c.padID)
					}
					if j < len(rawMasks) {
						masks[j] = int64(rawMasks[j])
					}
					if c.expectTokenTypes && j < len(rawTypes) {
						types[j] = int64(rawTypes[j])
					}
				}
				
				c.totalTokenized.Add(1)
				batch[i].resultChan <- &TokenizedChunk{IDs: ids, Masks: masks, Types: types, Text: rawTexts[i]}
			}
		}
		
		// Clear batch and restart timer
		batch = batch[:0]
		timer.Stop()
		select {
		case <-timer.C:
		default:
		}
		timer.Reset(flushInterval)
	}
	
	for {
		select {
		case job, ok := <-c.tokenizerChan:
			if !ok {
				processBuffer()
				return
			}
			
			if len(batch) == 0 {
				timer.Reset(flushInterval)
			}
			batch = append(batch, job)
			if len(batch) >= maxBatchSize {
				processBuffer()
			}
			
		case <-timer.C:
			processBuffer()
			
		case <-c.stopChan:
			processBuffer()
			return
		}
	}
}



// GenerateEmbeddings converts input strings into normalized embedding vectors.
// Uses the batch worker for optimal GPU utilization.
// It tokenizes texts in parallel before sending them to the background worker.
func (c *ONNXEmbeddingClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if c.closed.Load() {
		return nil, fmt.Errorf("embedding client is closed or closing")
	}
	c.updateActivity()

	results := make([][]float32, len(texts))
	errs := make([]error, len(texts))
	
	// Stream chunks to the GPU worker as they are tokenized
	var wg sync.WaitGroup
	for i, t := range texts {
		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()
			
			if c.closed.Load() {
				errs[idx] = fmt.Errorf("client closed during operation")
				return
			}

			tkResChan := make(chan *TokenizedChunk, 1)
			
			// Safe send to tokenizerChan
			select {
			case c.tokenizerChan <- tokenizerJob{text: text, resultChan: tkResChan}:
			case <-c.stopChan:
				errs[idx] = fmt.Errorf("client stopping")
				return
			}
			
			// Wait for tokenization with safety select
			var tkChunk *TokenizedChunk
			select {
			case tkChunk = <-tkResChan:
				if tkChunk == nil {
					errs[idx] = fmt.Errorf("tokenization failed: result was nil")
					return
				}
			case <-time.After(30 * time.Second):
				errs[idx] = fmt.Errorf("tokenization timeout")
				return
			case <-c.stopChan:
				errs[idx] = fmt.Errorf("client closed during tokenization")
				return
			}
			
			if c.closed.Load() {
				errs[idx] = fmt.Errorf("client closed during operation")
				return
			}

			embResChan := make(chan embeddingResult, 1)
			
			// Safe send to chunkChan
			select {
			case c.chunkChan <- chunkJob{chunk: tkChunk, resultChan: embResChan}:
			case <-c.stopChan:
				errs[idx] = fmt.Errorf("client stopping")
				return
			}
			
			// Wait for embedding result with safety select
			select {
			case res := <-embResChan:
				if res.err != nil {
					errs[idx] = res.err
				} else {
					results[idx] = res.embedding
				}
			case <-time.After(60 * time.Second):
				errs[idx] = fmt.Errorf("embedding timeout (GPU might be hung)")
				return
			case <-c.stopChan:
				errs[idx] = fmt.Errorf("client closed during embedding")
				return
			}
		}(i, t)
	}
	
	wg.Wait()
	for _, e := range errs {
		if e != nil { return nil, e }
	}
	
	return results, nil
}

func (c *ONNXEmbeddingClient) updateActivity() {
	c.lastUsed = time.Now()
}

// LastActivity returns the time of the last embedding operation.
func (c *ONNXEmbeddingClient) LastActivity() time.Time {
	return c.lastUsed
}

// PendingWork returns the number of strings currently waiting to be embedded.
func (c *ONNXEmbeddingClient) PendingWork() int {
	return int(c.pendingCount.Load())
}

type chunkJob struct {
	chunk      *TokenizedChunk
	resultChan chan embeddingResult
}

type embeddingResult struct {
	embedding []float32
	err       error
}

func (c *ONNXEmbeddingClient) IsClosed() bool {
	return c.closed.Load()
}

// inferenceWorker acts as the GREEDY BATCHER: it pulls chunks directly from the basket (chunkChan).
func (c *ONNXEmbeddingClient) inferenceWorker() {
	defer c.wg.Done()

	for {
		// 1. Wait for at least one chunk in the basket
		waitStart := time.Now()
		job, ok := <-c.chunkChan
		c.gpuWaitTime.Add(int64(time.Since(waitStart)))
		if !ok {
			return
		}

		// 2. Greedy pick: take up to maxBatchSize immediately
		jobs := []chunkJob{job}
		
		// Drain the channel up to c.batchSize
		for len(jobs) < c.batchSize {
			select {
			case nextJob, ok2 := <-c.chunkChan:
				if !ok2 {
					goto process
				}
				jobs = append(jobs, nextJob)
			default:
				// If not full, wait a tiny bit (50ms) to see if more chunks arrive
				// This avoids wasting a GPU cycle if we could reach full efficiency.
				timer := time.NewTimer(50 * time.Millisecond)
				select {
				case nextJob, ok3 := <-c.chunkChan:
					timer.Stop()
					if !ok3 {
						goto process
					}
					jobs = append(jobs, nextJob)
				case <-timer.C:
					goto process
				}
			}
		}

	process:
		chunks := make([]*TokenizedChunk, len(jobs))
		for i, j := range jobs {
			chunks[i] = j.chunk
		}

		embeddings, err := c.embedTokenizedBatch(chunks)

		// Map results back to individual channels
		for i, j := range jobs {
			if err != nil {
				j.resultChan <- embeddingResult{err: err}
			} else if i < len(embeddings) {
				j.resultChan <- embeddingResult{embedding: embeddings[i]}
			} else {
				j.resultChan <- embeddingResult{err: fmt.Errorf("missing result for chunk %d", i)}
			}
		}
	}
}

// Close releases ONNX runtime resources.
func (c *ONNXEmbeddingClient) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	close(c.stopChan)
	close(c.tokenizerChan)
	close(c.chunkChan)
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		if err := c.session.Destroy(); err != nil {
			return err
		}
		c.session = nil
	}
	return nil
}

// embedBatch is now a convenience wrapper around TokenizeOne + embedTokenizedBatch.
// Note: for high volume indexing, use EmbedTokenizedBatch directly.
func (c *ONNXEmbeddingClient) embedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	
	chunks := make([]*TokenizedChunk, len(texts))
	for i, t := range texts {
		chunk, err := c.TokenizeOne(t)
		if err != nil {
			return nil, err
		}
		chunks[i] = chunk
	}
	
	return c.embedTokenizedBatch(chunks)
}

// embedTokenizedBatch processes pre-tokenized chunks with fixed batch shape.
func (c *ONNXEmbeddingClient) embedTokenizedBatch(chunks []*TokenizedChunk) ([][]float32, error) {
	actualCount := len(chunks)
	if actualCount == 0 {
		return [][]float32{}, nil
	}

	// Split into sub-batches if needed (internal loop to avoid Go channel overhead)
	// We want to ensure that each call to runInference respects the fixed batch size (c.batchSize)
	var allResults [][]float32
	for i := 0; i < actualCount; i += c.batchSize {
		end := i + c.batchSize
		if end > actualCount {
			end = actualCount
		}
		
		sub, err := c.runInference(chunks[i:end])
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, sub...)
	}
	return allResults, nil
}

// TokenizeOne tokenizes a single text into a TokenizedChunk via the global pipeline.
func (c *ONNXEmbeddingClient) TokenizeOne(text string) (*TokenizedChunk, error) {
	resChan := make(chan *TokenizedChunk, 1)
	c.tokenizerChan <- tokenizerJob{text: text, resultChan: resChan}
	res := <-resChan
	if res == nil {
		return nil, fmt.Errorf("tokenization failed")
	}
	return res, nil
}

// EmbedTokenizedBatch is now a wrapper that streams pre-tokenized chunks.
func (c *ONNXEmbeddingClient) EmbedTokenizedBatch(chunks []*TokenizedChunk) ([][]float32, error) {
	if len(chunks) == 0 {
		return [][]float32{}, nil
	}
	
	c.updateActivity()
	c.pendingCount.Add(int32(len(chunks)))

	results := make([][]float32, len(chunks))
	errs := make([]error, len(chunks))
	
	var wg sync.WaitGroup
	for i, ch := range chunks {
		wg.Add(1)
		go func(idx int, currentChunk *TokenizedChunk) {
			defer wg.Done()
			
			embResChan := make(chan embeddingResult, 1)
			c.chunkChan <- chunkJob{chunk: currentChunk, resultChan: embResChan}
			
			res := <-embResChan
			if res.err != nil {
				errs[idx] = res.err
			} else {
				results[idx] = res.embedding
			}
		}(i, ch)
	}
	
	wg.Wait()
	c.pendingCount.Add(int32(-len(chunks)))
	
	for _, e := range errs {
		if e != nil { return nil, e }
	}
	return results, nil
}

// runInference (renamed from old EmbedTokenizedBatch) performs the actual session.Run.
func (c *ONNXEmbeddingClient) runInference(chunks []*TokenizedChunk) ([][]float32, error) {
	actualCount := len(chunks)
	if actualCount == 0 {
		return [][]float32{}, nil
	}

	efficiency := (actualCount * 100) / c.batchSize
	c.lastEfficiency.Store(int32(efficiency))
	c.totalEmbedded.Add(int64(actualCount))

	gpuStart := time.Now()

	// Assemble pre-tokenized chunks into batch arrays with FIXED padding
	// Dual-Mode Batching: 
	// Se abbiamo pochi chunk (es. ricerca), usiamo un batch piccolo fisso (4) per risparmiare VRAM.
	// Altrimenti usiamo il batch size ottimale calcolato.
	batchSize := c.batchSize
	if actualCount <= 4 && batchSize > 4 {
		batchSize = 4
	}
	totalLen := int64(batchSize) * int64(c.maxSeqLen)
	allIds := make([]int64, totalLen)
	allMasks := make([]int64, totalLen)
	allTypes := make([]int64, totalLen)

	// Pre-fill with padID
	for j := range allIds {
		allIds[j] = int64(c.padID)
	}

	for i, chunk := range chunks {
		if i >= batchSize { break }
		base := i * c.maxSeqLen
		copy(allIds[base:base+c.maxSeqLen], chunk.IDs)
		copy(allMasks[base:base+c.maxSeqLen], chunk.Masks)
		copy(allTypes[base:base+c.maxSeqLen], chunk.Types)
	}

	// Build Tensors with FIXED batch shape for VRAM stability
	shape := onnx.NewShape(int64(batchSize), int64(c.maxSeqLen))
	idTensor, err := onnx.NewTensor(shape, allIds)
	if err != nil {
		return nil, err
	}
	defer idTensor.Destroy()

	maskTensor, err := onnx.NewTensor(shape, allMasks)
	if err != nil {
		return nil, err
	}
	defer maskTensor.Destroy()

	var typeTensor *onnx.Tensor[int64]
	if c.expectTokenTypes {
		typeTensor, err = onnx.NewTensor(shape, allTypes)
		if err != nil {
			return nil, err
		}
		defer typeTensor.Destroy()
	}

	// Prepare inputs/outputs
	inputValues := make([]onnx.Value, 0, len(c.inputNames))
	for _, name := range c.inputNames {
		switch strings.ToLower(name) {
		case "input_ids":
			inputValues = append(inputValues, idTensor)
		case "attention_mask":
			inputValues = append(inputValues, maskTensor)
		case "token_type_ids":
			if typeTensor != nil {
				inputValues = append(inputValues, typeTensor)
			}
		}
	}

	outputValues := make([]onnx.Value, len(c.outputNames))
	err = c.session.Run(inputValues, outputValues)
	defer func() {
		for _, v := range outputValues {
			if v != nil {
				v.Destroy()
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	if actualCount > 1 || time.Since(gpuStart) > 200*time.Millisecond {
		log.Printf("GPU Inference: %dms | real:%d/%d (%d%% eff) | Queue:{Tok:%d, Chunk:%d}", 
			time.Since(gpuStart).Milliseconds(), actualCount, c.batchSize, efficiency, len(c.tokenizerChan), len(c.chunkChan))
	}

	// Post-process batch
	tensor, ok := outputValues[0].(*onnx.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type")
	}

	rawData := tensor.GetData()
	outShape := tensor.GetShape()

	results := make([][]float32, actualCount)

	if len(outShape) == 2 {
		hidden := int(outShape[1])
		for i := 0; i < actualCount; i++ {
			vec := make([]float32, hidden)
			copy(vec, rawData[i*hidden:(i+1)*hidden])
			normalizeVector(vec)
			results[i] = vec
		}
	} else if len(outShape) == 3 {
		seqLen := int(outShape[1])
		hidden := int(outShape[2])
		for i := 0; i < actualCount; i++ {
			rowMask := make([]int, seqLen)
			base := i * c.maxSeqLen
			for j := 0; j < seqLen && j < c.maxSeqLen; j++ {
				rowMask[j] = int(allMasks[base+j])
			}

			rowOutput := rawData[i*seqLen*hidden : (i+1)*seqLen*hidden]
			vec, err := postProcessSingleEmbedding(rowOutput, seqLen, hidden, rowMask)
			if err != nil {
				return nil, err
			}
			normalizeVector(vec)
			results[i] = vec
		}
	} else {
		return nil, fmt.Errorf("unsupported output shape: %v", outShape)
	}

	return results[:actualCount], nil
}

func postProcessSingleEmbedding(data []float32, seqLen, hidden int, attMask []int) ([]float32, error) {
	result := make([]float32, hidden)
	var count float32
	for i := 0; i < seqLen && i < len(attMask); i++ {
		if attMask[i] == 0 {
			continue
		}
		start := i * hidden
		for j := 0; j < hidden; j++ {
			result[j] += data[start+j]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	scale := 1 / count
	for i := range result {
		result[i] *= scale
	}
	return result, nil
}

func newONNXSessionWithBestProvider(modelPath string, inputNames, outputNames []string) (*onnx.DynamicAdvancedSession, error) {
	options, err := onnx.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	// Use Basic optimization for GPU (node fusion) to minimize the buffer count
	// Impostiamo il livello di ottimizzazione massimo per massimizzare la fusione dei nodi e ridurre i buffer intermedi.
	options.SetGraphOptimizationLevel(onnx.GraphOptimizationLevelEnableAll)

	// Enable Memory Pattern to pre-allocate and reuse buffers efficiently
	options.SetMemPattern(true)

	// Use Sequential execution to minimize concurrent activation buffers
	options.SetExecutionMode(onnx.ExecutionModeSequential)

	// Enable DirectML to use device allocator for initializers to reduce host->device copies
	options.AddSessionConfigEntry("session.use_device_allocator_for_initializers", "1")

	// Detect if we are using a GPU provider
	isGPU := GetActiveExecutionProvider() != "CPU"

	// Limit internal thread pool
	numThreads := runtime.NumCPU() / 2
	if numThreads < 1 {
		numThreads = 1
	}
	if isGPU {
		// For GPU, minimal threads reduce the DirectML workspace/arenas
		numThreads = 1
	}
	options.SetIntraOpNumThreads(numThreads)
	// Set Inter-Op threads to 1 for GPU stability and less memory overhead
	options.SetInterOpNumThreads(1)

	// Try to append specialized execution providers.
	// We use the globally detected execution provider.
	if GetActiveExecutionProvider() == "CoreML" {
		options.AppendExecutionProviderCoreMLV2(nil)
	} else if GetActiveExecutionProvider() == "DirectML" {
		options.AppendExecutionProviderDirectML(0)
	} else if GetActiveExecutionProvider() == "CUDA" {
		cudaOpts, err := onnx.NewCUDAProviderOptions()
		if err == nil {
			defer cudaOpts.Destroy()
			options.AppendExecutionProviderCUDA(cudaOpts)
		}
	}

	log.Printf("GPU Cache: VRAM total usage before session: %d MB", GetProcessVRAMUsage())

	session, err := onnx.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	log.Printf("GPU Cache: ONNX session created (VRAM total: %d MB) for %s", GetProcessVRAMUsage(), modelPath)
	return session, nil
}

func detectBestExecutionProvider() {
	options, err := onnx.NewSessionOptions()
	if err != nil {
		return
	}
	defer options.Destroy()

	if runtime.GOOS == "darwin" {
		if err := options.AppendExecutionProviderCoreMLV2(nil); err == nil {
			log.Printf("DEBUG: CoreML execution provider detected")
			activeExecutionProvider = "CoreML"
			return
		}
	}

	if runtime.GOOS == "windows" {
		if err := options.AppendExecutionProviderDirectML(0); err == nil {
			log.Printf("DEBUG: DirectML execution provider detected")
			activeExecutionProvider = "DirectML"
			return
		}
	}

	cudaOpts, err := onnx.NewCUDAProviderOptions()
	if err == nil {
		defer cudaOpts.Destroy()
		if err := options.AppendExecutionProviderCUDA(cudaOpts); err == nil {
			log.Printf("DEBUG: CUDA execution provider detected")
			activeExecutionProvider = "CUDA"
			return
		}
	}

	activeExecutionProvider = "CPU"
	log.Printf("DEBUG: CPU execution provider detected")
}

func ensureONNXRuntimeInitialized() error {
	onnxRuntimeInitOnce.Do(func() {
		trimmed := strings.TrimSpace(onnxSharedLibraryPath)
		if trimmed != "" {
			log.Printf("DEBUG: Setting ONNX shared library path to: %s", trimmed)
			onnx.SetSharedLibraryPath(trimmed)
		} else {
			log.Printf("DEBUG: No custom ONNX shared library path set. Using default search strategy.")
		}

		onnxRuntimeInitErr = onnx.InitializeEnvironment()
		if onnxRuntimeInitErr != nil {
			log.Printf("ERROR: ONNX Runtime initialization failed: %v", onnxRuntimeInitErr)
		} else {
			activeSharedLibraryPath = onnxSharedLibraryPath
			log.Printf("DEBUG: ONNX Runtime environment initialized successfully.")
			detectBestExecutionProvider()
		}
	})
	return onnxRuntimeInitErr
}

// ConfigureSharedLibraryPath sets the desired ONNX Runtime shared library path to be used on initialization.
func ConfigureSharedLibraryPath(path string) {
	onnxSharedLibraryPath = strings.TrimSpace(path)
}

// ActiveSharedLibraryPath returns the ONNX Runtime shared library path currently in use.
func ActiveSharedLibraryPath() string {
	return strings.TrimSpace(activeSharedLibraryPath)
}

// GetActiveExecutionProvider returns the name of the execution provider currently in use.
func GetActiveExecutionProvider() string {
	return activeExecutionProvider
}

// IsONNXRuntimeInstalled checks if the shared library exists without loading it.
func IsONNXRuntimeInstalled() bool {
	path := strings.TrimSpace(onnxSharedLibraryPath)
	if path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
		return false
	}

	available, err := CheckONNXRuntimeAvailability()
	return err == nil && available
}

// CheckONNXRuntimeAvailability reports whether the ONNX runtime shared library
// can be loaded in the current process. The detection result is memoized, so
// subsequent calls return immediately without touching the runtime again.
func CheckONNXRuntimeAvailability() (bool, error) {
	if err := ensureONNXRuntimeInitialized(); err != nil {
		return false, err
	}
	return true, nil
}

func hasTokenTypeInput(inputNames []string) bool {
	for _, name := range inputNames {
		if strings.EqualFold(name, "token_type_ids") {
			return true
		}
	}
	return false
}

func toInt64(values []int, maxLen int) []int64 {
	out := make([]int64, maxLen)
	for i := 0; i < maxLen && i < len(values); i++ {
		out[i] = int64(values[i])
	}
	return out
}

func clampSlice(values []int, maxLen int) []int {
	if len(values) >= maxLen {
		return values[:maxLen]
	}
	out := make([]int, maxLen)
	copy(out, values)
	return out
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func sanitizeTokenizerInput(s string) string {
	// sugarme/tokenizer has a bug in offset mapping when strings have trailing newlines
	// or CRLF sequences that get normalized to LF (or vice versa) in a way that breaks
	// the internal range calculation.
	if strings.Contains(s, "\r") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	
	s = strings.TrimSpace(s)
	
	// Adding a trailing space is a common workaround for tokenizers that crash 
	// when the last token ends exactly at the string boundary with certain normalizers.
	if s != "" && !strings.HasSuffix(s, " ") {
		return s + " "
	}
	return s
}

func normalizeVector(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	norm := float32(1 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= norm
	}
}

func (c *ONNXEmbeddingClient) normalizeEncoding(enc *tokenizer.Encoding) {
	if enc == nil {
		return
	}
	targetLen := len(enc.Ids)
	if targetLen == 0 {
		return
	}

	enc.TypeIds = padOrTrimInt(enc.TypeIds, targetLen, c.padTypeID)
	enc.Tokens = padOrTrimString(enc.Tokens, targetLen, c.padToken)
	enc.SpecialTokenMask = padOrTrimInt(enc.SpecialTokenMask, targetLen, 0)
	enc.AttentionMask = padOrTrimInt(enc.AttentionMask, targetLen, 1)
	enc.Offsets = padOrTrimOffsets(enc.Offsets, targetLen)
	enc.Words = padOrTrimInt(enc.Words, targetLen, -1)
}

func padOrTrimInt(values []int, targetLen int, fill int) []int {
	if len(values) == targetLen {
		return values
	}
	out := make([]int, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = fill
	}
	return out
}

func padOrTrimString(values []string, targetLen int, fill string) []string {
	if len(values) == targetLen {
		return values
	}
	out := make([]string, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = fill
	}
	return out
}

func padOrTrimOffsets(values [][]int, targetLen int) [][]int {
	if len(values) == targetLen {
		return values
	}
	out := make([][]int, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = []int{0, 0}
	}
	return out
}
