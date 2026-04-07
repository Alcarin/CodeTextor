package embedding

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	fastembed "github.com/anush008/fastembed-go"
)

type FastEmbedClient struct {
	model        *fastembed.FlagEmbedding
	batchSize    int
	jobChan      chan embeddingJob
	stopChan     chan struct{}
	wg           sync.WaitGroup
	modelMu      sync.Mutex
	lastUsed     time.Time
	lastUsedMu   sync.Mutex
	batchSizeMu  sync.Mutex
	modelID      string
	pendingCount atomic.Int32
	closed       atomic.Bool
}

const (
	fastEmbedDefaultBatchSize = 64
	fastEmbedMaxBatchSize     = 256
)

// NewFastEmbedClient initializes a fastembed runtime for the provided model metadata.
func NewFastEmbedClient(meta *models.EmbeddingModelInfo) (EmbeddingClient, error) {
	modelID, err := mapFastEmbedModel(meta)
	if err != nil {
		return nil, err
	}

	cacheRoot, err := utils.GetModelsDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(cacheRoot, "fastembed")
	targetDir := filepath.Join(cacheDir, string(modelID))
	if info, err := os.Stat(targetDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("fastembed model %s not found locally. Download it from the Indexing view and try again", meta.DisplayName)
	}

	showProgress := false
	options := fastembed.InitOptions{
		Model:                modelID,
		CacheDir:             cacheDir,
		MaxLength:            fastEmbedMaxSequence(meta),
		ShowDownloadProgress: &showProgress,
	}

	flagModel, err := fastembed.NewFlagEmbedding(&options)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize FastEmbed model %s: %w", modelID, err)
	}

	c := &FastEmbedClient{
		model:     flagModel,
		batchSize: fastEmbedDefaultBatchSize,
		modelID:   meta.ID,
		jobChan:   make(chan embeddingJob, 100),
		stopChan:  make(chan struct{}),
		lastUsed:  time.Now(),
	}

	c.wg.Add(1)
	go c.batchWorker()

	return c, nil
}

// batchWorker collects embedding requests and processes them in large batches.
func (c *FastEmbedClient) batchWorker() {
	defer c.wg.Done()

	var pendingTexts []string
	var pendingJobs []embeddingJob

	flush := func() {
		if len(pendingTexts) == 0 {
			return
		}

		// Perform batch embedding with exclusive model access
		c.modelMu.Lock()
		log.Printf("GPU Batch (FastEmbed): Processing %d texts (from %d jobs)", len(pendingTexts), len(pendingJobs))
		embeddings, err := c.model.PassageEmbed(pendingTexts, c.batchSize)
		c.modelMu.Unlock()

		// Distribute results back to original callers
		offset := 0
		for _, job := range pendingJobs {
			count := len(job.texts)
			var jobEmbeds [][]float32
			var jobErr error

			if err != nil {
				jobErr = err
			} else {
				// Safety check: ensure we don't go out of bounds
				if offset+count <= len(embeddings) {
					jobEmbeds = make([][]float32, count)
					copy(jobEmbeds, embeddings[offset:offset+count])
				} else {
					jobErr = fmt.Errorf("embedding count mismatch: expected %d, got %d", count, len(embeddings)-offset)
				}
			}

			job.resultChan <- jobResult{embeddings: jobEmbeds, err: jobErr}
			offset += count
			c.pendingCount.Add(int32(-count))
		}

		// Clear buffers
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

			// If we reached a large enough batch, flush immediately
			if len(pendingTexts) >= fastEmbedMaxBatchSize {
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

func (c *FastEmbedClient) updateActivity() {
	c.lastUsedMu.Lock()
	c.lastUsed = time.Now()
	c.lastUsedMu.Unlock()
}

func (c *FastEmbedClient) LastActivity() time.Time {
	c.lastUsedMu.Lock()
	defer c.lastUsedMu.Unlock()
	return c.lastUsed
}

// PendingWork returns the number of strings currently waiting to be embedded.
func (c *FastEmbedClient) PendingWork() int {
	return int(c.pendingCount.Load())
}

// GetBatchSize returns the batch size configured for this client.
func (c *FastEmbedClient) GetBatchSize() int {
	c.batchSizeMu.Lock()
	defer c.batchSizeMu.Unlock()
	return c.batchSize
}

// SetBatchSize updates the batch size for this client instance.
func (c *FastEmbedClient) SetBatchSize(batchSize int) {
	c.batchSizeMu.Lock()
	defer c.batchSizeMu.Unlock()
	if batchSize < 1 {
		batchSize = 1
	}
	c.batchSize = batchSize
}

// ModelID returns the unique identifier for the model used by this client.
func (c *FastEmbedClient) ModelID() string {
	return c.modelID
}
// IsClosed returns true if the client has been shut down.
func (c *FastEmbedClient) IsClosed() bool {
	return c.closed.Load()
}

// GenerateEmbeddings embeds the provided texts using the fastembed runtime.
func (c *FastEmbedClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if c.closed.Load() {
		return nil, fmt.Errorf("fastembed client is closed")
	}

	c.updateActivity()
	
	// Incrementiamo il carico pendente
	c.pendingCount.Add(int32(len(texts)))

	// Bypass: if it's a single text and the queue is empty, process immediately
	if len(texts) == 1 && len(c.jobChan) == 0 {
		log.Printf("GPU Bypass (FastEmbed): Single text detected, processing immediately")
		c.modelMu.Lock()
		defer c.modelMu.Unlock()
		res, err := c.model.PassageEmbed(texts, 1)
		c.pendingCount.Add(-1)
		return res, err
	}

	resultChan := make(chan jobResult, 1)
	
	select {
	case c.jobChan <- embeddingJob{
		texts:      texts,
		resultChan: resultChan,
	}:
	case <-c.stopChan:
		return nil, fmt.Errorf("fastembed client is stopping")
	}

	result := <-resultChan
	return result.embeddings, result.err
}

// EmbedTokenizedBatch extracts text from already tokenized chunks and processes them via the standard embedding queue.
// FastEmbed doesn't support direct token injection, so we fall back to raw text.
func (c *FastEmbedClient) EmbedTokenizedBatch(chunks []*TokenizedChunk) ([][]float32, error) {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}
	return c.GenerateEmbeddings(texts)
}

// Close releases any resources held by the fastembed runtime.
func (c *FastEmbedClient) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(c.stopChan)
	c.wg.Wait()

	if c.model != nil {
		return c.model.Destroy()
	}
	return nil
}

func fastEmbedMaxSequence(meta *models.EmbeddingModelInfo) int {
	if meta != nil && meta.MaxSequenceLength > 0 {
		return meta.MaxSequenceLength
	}
	return 512
}
