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
}


// embeddingJob represents a unit of work sent to a background batching worker.
type embeddingJob struct {
	texts      []string
	resultChan chan jobResult
}

// jobResult contains the output of an embedding job.
type jobResult struct {
	embeddings [][]float32
	err        error
}
