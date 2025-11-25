package embedding

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	fastembed "github.com/anush008/fastembed-go"
)

// FastEmbedClient wraps the upstream fastembed FlagEmbedding runtime.
type FastEmbedClient struct {
	model     *fastembed.FlagEmbedding
	batchSize int
	mu        sync.Mutex
}

const fastEmbedDefaultBatchSize = 64

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

	return &FastEmbedClient{
		model:     flagModel,
		batchSize: fastEmbedDefaultBatchSize,
	}, nil
}

// GenerateEmbeddings embeds the provided texts using the fastembed runtime.
func (c *FastEmbedClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Use passage embeddings because our inputs are document chunks.
	batchSize := c.batchSize
	if len(texts) < batchSize {
		batchSize = len(texts)
	}
	if batchSize <= 0 {
		batchSize = fastEmbedDefaultBatchSize
	}

	embeddings, err := c.model.PassageEmbed(texts, batchSize)
	if err != nil {
		return nil, err
	}

	result := make([][]float32, len(embeddings))
	for i, vec := range embeddings {
		result[i] = vec
	}
	return result, nil
}

// Close releases any resources held by the fastembed runtime.
func (c *FastEmbedClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
