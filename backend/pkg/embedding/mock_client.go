package embedding

import (
	"math/rand"
)

// MockEmbeddingClient is a fake embedding client for testing.
// It generates random vectors of a specified dimension.
type MockEmbeddingClient struct {
	Dimension int
}

// NewMockEmbeddingClient creates a new mock client.
func NewMockEmbeddingClient(dimension int) *MockEmbeddingClient {
	return &MockEmbeddingClient{Dimension: dimension}
}

// GenerateEmbeddings generates random float32 vectors.
func (c *MockEmbeddingClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, c.Dimension)
		for j := range embedding {
			embedding[j] = rand.Float32()
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}
