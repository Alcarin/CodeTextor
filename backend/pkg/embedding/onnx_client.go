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
	jobChan          chan embeddingJob
	stopChan         chan struct{}
	wg               sync.WaitGroup
	pendingCount     atomic.Int32
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

// TokenizedChunk holds tokenized data for a single text, ready for batch assembly.
// Created by TokenizeOne() in CPU workers, consumed by EmbedTokenizedBatch() in GPU worker.
type TokenizedChunk struct {
	IDs   []int64
	Masks []int64
	Types []int64
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
		log.Printf("DEBUG: ensureONNXRuntimeInitialized failed: %v", err)
		return nil, err
	}
	log.Printf("DEBUG: ONNX Runtime initialized successfully")

	// Compute optimal batch size based on ACTUAL available VRAM at the moment of client creation.
	// This helps adapt to changing GPU load (e.g. after model unloads/leaking memory).
	batchSize := computeOptimalBatchSize()

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

	session, err := newONNXSessionWithBestProvider(meta.LocalPath, inputNames, outputNames)
	if err != nil {
		log.Printf("DEBUG: newONNXSessionWithBestProvider failed: %v", err)
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

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
		jobChan:          make(chan embeddingJob, 64),
		stopChan:         make(chan struct{}),
		lastUsed:         time.Now(),
	}
	client.wg.Add(1)
	go client.batchWorker()
	return client, nil
}

// GenerateEmbeddings converts input strings into normalized embedding vectors.
// Uses the batch worker for optimal GPU utilization.
func (c *ONNXEmbeddingClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	c.updateActivity()
	// Incrementiamo il carico pendente
	c.pendingCount.Add(int32(len(texts)))

	resultChan := make(chan jobResult, 1)
	c.jobChan <- embeddingJob{texts: texts, resultChan: resultChan}
	result := <-resultChan
	return result.embeddings, result.err
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

// batchWorker collects embedding requests and processes them in large batches.
func (c *ONNXEmbeddingClient) batchWorker() {
	defer c.wg.Done()

	var pendingTexts []string
	var pendingJobs []embeddingJob

	var maxBatchSize = c.batchSize // Optimal batch for this instance

	flush := func() {
		if len(pendingTexts) == 0 {
			return
		}

		// Perform batch embedding
		log.Printf("GPU Batch (ONNX): Processing %d texts (from %d jobs)", len(pendingTexts), len(pendingJobs))
		embeddings, err := c.embedBatch(pendingTexts)

		// Distribute results back to original callers
		offset := 0
		for _, job := range pendingJobs {
			count := len(job.texts)
			var jobEmbeds [][]float32
			var jobErr error

			if err != nil {
				jobErr = err
			} else {
				if offset+count <= len(embeddings) {
					jobEmbeds = make([][]float32, count)
					copy(jobEmbeds, embeddings[offset:offset+count])
				} else {
					jobErr = fmt.Errorf("onnx embedding count mismatch")
				}
			}

			job.resultChan <- jobResult{embeddings: jobEmbeds, err: jobErr}
			offset += count
			// Decrementiamo il carico pendente quando il job è completato
			c.pendingCount.Add(int32(-count))
		}

		pendingTexts = nil
		pendingJobs = nil
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case job, ok := <-c.jobChan:
			if !ok {
				return
			}
			pendingTexts = append(pendingTexts, job.texts...)
			pendingJobs = append(pendingJobs, job)

			if len(pendingTexts) >= maxBatchSize {
				flush()
				ticker.Reset(50 * time.Millisecond)
			}
		case <-ticker.C:
			flush()
		case <-c.stopChan:
			flush()
			return
		}
	}
}

// Close releases ONNX runtime resources.
func (c *ONNXEmbeddingClient) Close() error {
	close(c.stopChan)
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

// embedBatch processes a batch of texts through ONNX with fixed batch shape.
func (c *ONNXEmbeddingClient) embedBatch(texts []string) ([][]float32, error) {
	actualCount := len(texts)
	if actualCount == 0 {
		return [][]float32{}, nil
	}

	// Split into sub-batches if needed
	if actualCount > c.batchSize {
		var allResults [][]float32
		for i := 0; i < actualCount; i += c.batchSize {
			end := i + c.batchSize
			if end > actualCount {
				end = actualCount
			}
			sub, err := c.embedBatch(texts[i:end])
			if err != nil {
				return nil, err
			}
			allResults = append(allResults, sub...)
		}
		return allResults, nil
	}

	batchSize := actualCount
	if actualCount < c.batchSize {
		batchSize = c.batchSize
	}

	c.updateActivity()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil, fmt.Errorf("ONNX session is closed")
	}

	log.Printf("GPU Shape (ONNX): [%d, %d] (%d real texts, %d padding)",
		batchSize, c.maxSeqLen, actualCount, batchSize-actualCount)

	gpuStart := time.Now()

	totalLen := int64(batchSize) * int64(c.maxSeqLen)
	allIds := make([]int64, totalLen)
	allMasks := make([]int64, totalLen)
	allTypes := make([]int64, totalLen)

	for i, text := range texts {
		encoding, err := c.tokenizer.EncodeSingle(text, true)
		if err != nil {
			log.Printf("Tokenizer error for text %d: %v", i, err)
			continue
		}
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

		ids := encoding.GetIds()
		masks := encoding.GetAttentionMask()
		types := encoding.GetTypeIds()

		base := i * c.maxSeqLen
		for j := 0; j < c.maxSeqLen; j++ {
			if j < len(ids) {
				allIds[base+j] = int64(ids[j])
			} else {
				allIds[base+j] = int64(c.padID)
			}

			if j < len(masks) {
				allMasks[base+j] = int64(masks[j])
			}

			if c.expectTokenTypes && j < len(types) {
				allTypes[base+j] = int64(types[j])
			}
		}
	}

	// Build Tensors
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

	log.Printf("GPU Inference: %dms (VRAM Sys: %d MB)", time.Since(gpuStart).Milliseconds(), GetProcessVRAMUsage())

	// Post-process batch
	tensor, ok := outputValues[0].(*onnx.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type")
	}

	rawData := tensor.GetData()
	outShape := tensor.GetShape()

	results := make([][]float32, batchSize)

	if len(outShape) == 2 {
		hidden := int(outShape[1])
		for i := 0; i < batchSize; i++ {
			vec := make([]float32, hidden)
			copy(vec, rawData[i*hidden:(i+1)*hidden])
			normalizeVector(vec)
			results[i] = vec
		}
	} else if len(outShape) == 3 {
		seqLen := int(outShape[1])
		hidden := int(outShape[2])
		for i := 0; i < batchSize; i++ {
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

	// Return only actual results, discarding padding entries
	return results[:actualCount], nil
}

// TokenizeOne tokenizes a single text into a TokenizedChunk.
// This is thread-safe: the tokenizer is read-only during encoding and each call
// creates independent encoding objects. Safe for concurrent use from CPU workers.
func (c *ONNXEmbeddingClient) TokenizeOne(text string) (*TokenizedChunk, error) {
	encoding, err := c.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, err
	}
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

	return &TokenizedChunk{IDs: ids, Masks: masks, Types: types}, nil
}

// EmbedTokenizedBatch takes pre-tokenized chunks (from TokenizeOne), assembles them
// into a GPU batch, and runs inference. The GPU worker calls ONLY this — zero CPU
// tokenization overhead between batches.
// chunks must have len <= fixedBatchSize. Padding to fixedBatchSize is automatic.
func (c *ONNXEmbeddingClient) EmbedTokenizedBatch(chunks []*TokenizedChunk) ([][]float32, error) {
	actualCount := len(chunks)
	if actualCount == 0 {
		return [][]float32{}, nil
	}

	c.updateActivity()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil, fmt.Errorf("ONNX session is closed")
	}

	log.Printf("GPU Shape (ONNX): [%d, %d] (%d texts)",
		actualCount, c.maxSeqLen, actualCount)

	gpuStart := time.Now()

	// Assemble pre-tokenized chunks into batch arrays
	totalLen := int64(actualCount) * int64(c.maxSeqLen)
	allIds := make([]int64, totalLen)
	allMasks := make([]int64, totalLen)
	allTypes := make([]int64, totalLen)

	for i, chunk := range chunks {
		base := i * c.maxSeqLen
		copy(allIds[base:base+c.maxSeqLen], chunk.IDs)
		copy(allMasks[base:base+c.maxSeqLen], chunk.Masks)
		copy(allTypes[base:base+c.maxSeqLen], chunk.Types)
	}

	// Build Tensors
	shape := onnx.NewShape(int64(actualCount), int64(c.maxSeqLen))
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

	log.Printf("GPU Inference: %dms (VRAM Sys: %d MB)", time.Since(gpuStart).Milliseconds(), GetProcessVRAMUsage())

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
