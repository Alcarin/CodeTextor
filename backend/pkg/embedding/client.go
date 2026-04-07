package embedding

import "time"

// EmbeddingClient is the interface for any embedding service client.
type EmbeddingClient interface {
	GenerateEmbeddings(texts []string) ([][]float32, error)
	GetBatchSize() int
	SetBatchSize(batchSize int)
	Close() error
	LastActivity() time.Time
	PendingWork() int
	ModelID() string
	EmbedTokenizedBatch(chunks []*TokenizedChunk) ([][]float32, error)
	IsClosed() bool
}

// TokenizedChunk represents a single chunk of text already converted to token IDs and masks.
type TokenizedChunk struct {
	IDs   []int64
	Masks []int64
	Types []int64
	Text  string // Original text for fallback or verification
}


// embeddingJob represents a unit of work sent to a background batching worker.
type embeddingJob struct {
	texts      []string
	chunks     []*TokenizedChunk
	resultChan chan jobResult
}

// jobResult contains the output of an embedding job.
type jobResult struct {
	embeddings [][]float32
	err        error
}
