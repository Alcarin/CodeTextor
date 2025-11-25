package embedding

// EmbeddingClient is the interface for any embedding service client.
type EmbeddingClient interface {
	GenerateEmbeddings(texts []string) ([][]float32, error)
	Close() error
}
