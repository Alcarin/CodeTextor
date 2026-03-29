package models

// EmbeddingPriority defines the importance of an embedding task.
type EmbeddingPriority int

const (
	PriorityLow    EmbeddingPriority = iota // Background file edits
	PriorityNormal                          // Startup/Initial sync
	PriorityHigh                           // Manual reindex/Explicit user action
)
