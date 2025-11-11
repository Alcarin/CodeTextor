/*
  File: project.go
  Purpose: Data models for CodeTextor projects with configuration settings.
  Author: CodeTextor project
  Notes: This file defines the core project structure with indexing configuration.
         This is a public package (not internal) so Wails can generate TypeScript bindings.
*/

package models

import (
	"strings"
	"time"
)

// Project represents a CodeTextor project with its configuration and metadata.
// Each project maintains its own isolated index database and settings.
type Project struct {
	// ID is the unique, immutable, URL-safe identifier for the project.
	// This is the slug - derived from the project name at creation time.
	// Example: "my-awesome-project" from name "My Awesome Project!"
	// It's used as the primary key in the database and for the database filename.
	ID string `json:"id"`

	// Name is the human-readable project name
	Name string `json:"name"`

	// Description provides additional context about the project
	Description string `json:"description"`

	// CreatedAt is the timestamp when the project was created
	CreatedAt int64 `json:"createdAt"`

	// UpdatedAt is the timestamp of the last modification
	UpdatedAt int64 `json:"updatedAt"`

	// Config contains all indexing and processing settings
	Config ProjectConfig `json:"config"`

	// IsIndexing indicates whether continuous indexing is enabled for this project
	// This state is persisted in the database
	IsIndexing bool `json:"isIndexing"`

	// Stats contains current project statistics (not persisted in config DB)
	Stats *ProjectStats `json:"stats,omitempty"`
}

// ProjectConfig contains all configuration settings for project indexing.
// This defines what files to index and how to process them.
type ProjectConfig struct {
	// IncludePaths is a list of directories to include in indexing.
	// Can be from different file system locations (no single root path).
	IncludePaths []string `json:"includePaths"`

	// ExcludePatterns defines glob patterns for files/directories to exclude.
	// Examples: "node_modules", ".git", "*.min.js"
	ExcludePatterns []string `json:"excludePatterns"`

	// FileExtensions filters indexing to specific file types.
	// If empty, all supported file types are indexed.
	// Examples: [".go", ".ts", ".js", ".py"]
	FileExtensions []string `json:"fileExtensions"`

	// RootPath is the absolute path that serves as the base for the project.
	// IncludePaths are resolved relative to this directory.
	RootPath string `json:"rootPath"`

	// AutoExcludeHidden determines whether to automatically exclude hidden files/directories.
	AutoExcludeHidden bool `json:"autoExcludeHidden"`

	// ContinuousIndexing enables file system watching for automatic re-indexing.
	ContinuousIndexing bool `json:"continuousIndexing"`

	// ChunkSizeMin is the minimum token count for a chunk (merge smaller ones).
	// Default: 100 tokens
	ChunkSizeMin int `json:"chunkSizeMin"`

	// ChunkSizeMax is the maximum token count for a chunk (split larger ones).
	// Default: 800 tokens
	ChunkSizeMax int `json:"chunkSizeMax"`

	// EmbeddingModel specifies which embedding model to use.
	// Default: "default" (uses the system's default model)
	EmbeddingModel string `json:"embeddingModel"`

	// MaxResponseBytes is the maximum byte size for MCP API responses.
	// Default: 100000 (100KB)
	MaxResponseBytes int `json:"maxResponseBytes"`
}

// FilePreview represents a file with its metadata for display in the frontend.
type FilePreview struct {
	AbsolutePath string `json:"absolutePath"`
	RelativePath string `json:"relativePath"`
	Extension    string `json:"extension"`
	Size         string `json:"size"` // Human-readable size (e.g., "10 KB")
	Hidden       bool   `json:"hidden"`
}

// IndexingStatus defines the possible states of the indexing process.
// By using a custom type, Wails will generate a TypeScript union type.
type IndexingStatus string

const (
	// IndexingStatusIdle indicates the indexer is not running.
	IndexingStatusIdle IndexingStatus = "idle"
	// IndexingStatusIndexing indicates the indexer is actively processing files.
	IndexingStatusIndexing IndexingStatus = "indexing"
	// IndexingStatusCompleted indicates the indexer has finished a run.
	IndexingStatusCompleted IndexingStatus = "completed"
	// IndexingStatusError indicates the indexer stopped due to an error.
	IndexingStatusError IndexingStatus = "error"
)

// IndexingProgress represents the current state of an indexing operation.
type IndexingProgress struct {
	TotalFiles     int            `json:"totalFiles"`
	ProcessedFiles int            `json:"processedFiles"`
	CurrentFile    string         `json:"currentFile"`
	Status         IndexingStatus `json:"status"` // e.g., "idle", "indexing", "completed", "error"
	Error          string         `json:"error,omitempty"`
}

// ProjectStats contains current statistics about a project's index.
// These are computed from the index database, not stored in the config.
type ProjectStats struct {
	// TotalFiles is the number of indexed files
	TotalFiles int `json:"totalFiles"`

	// TotalChunks is the number of semantic chunks
	TotalChunks int `json:"totalChunks"`

	// TotalSymbols is the number of extracted symbols
	TotalSymbols int `json:"totalSymbols"`

	// DatabaseSize is the size of the index database in bytes
	DatabaseSize int64 `json:"databaseSize"`

	// LastIndexedAt is the timestamp of the last indexing operation
	LastIndexedAt *time.Time `json:"lastIndexedAt,omitempty"`

	// IsIndexing indicates whether the project is currently being indexed
	IsIndexing bool `json:"isIndexing"`

	// IndexingProgress is the current indexing progress (0.0 to 1.0)
	IndexingProgress float64 `json:"indexingProgress"`
}

// OutlineNode represents the hierarchical structure of a file that was parsed by Tree-sitter.
type OutlineNode struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	FilePath  string         `json:"filePath"`
	StartLine uint32         `json:"startLine"`
	EndLine   uint32         `json:"endLine"`
	Children  []*OutlineNode `json:"children,omitempty"`
}

// Chunk represents a piece of text from a file, along with its embedding.
type Chunk struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	FilePath  string    `json:"filePath"`
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding"`
	LineStart int       `json:"lineStart"`
	LineEnd   int       `json:"lineEnd"`
	CharStart int       `json:"charStart"`
	CharEnd   int       `json:"charEnd"`
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
}

// File represents a file that has been indexed.
type File struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	Path         string `json:"path"`
	Hash         string `json:"hash"`
	LastModified int64  `json:"lastModified"`
	ChunkCount   int    `json:"chunkCount"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// Symbol represents a code symbol extracted from a file.
type Symbol struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	FilePath  string `json:"filePath"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // e.g., "function", "class", "variable"
	Line      int    `json:"line"`
	Character int    `json:"character"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// NewProject creates a new Project instance with default configuration.
// Parameters:
//   - id: unique project identifier
//   - name: human-readable project name
//   - description: optional project description
//
// Returns a Project with sensible defaults for all configuration options.
func NewProject(id, name, description string) *Project {
	now := time.Now().Unix()
	return &Project{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Config: ProjectConfig{
			IncludePaths:       []string{"."},
			ExcludePatterns:    []string{"node_modules", ".git", ".cache", "dist", "build"},
			FileExtensions:     []string{},
			RootPath:           "",
			AutoExcludeHidden:  true,
			ContinuousIndexing: false,
			ChunkSizeMin:       100,
			ChunkSizeMax:       800,
			EmbeddingModel:     "default",
			MaxResponseBytes:   100000,
		},
		Stats: nil, // Stats are computed on demand
	}
}

// Validate checks if the project configuration is valid.
// Returns an error if any required field is missing or invalid.
func (p *Project) Validate() error {
	if p.ID == "" {
		return &ValidationError{Field: "id", Message: "project ID (slug) cannot be empty"}
	}
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "project name cannot be empty"}
	}
	if p.Config.ChunkSizeMin < 10 {
		return &ValidationError{Field: "chunkSizeMin", Message: "minimum chunk size must be at least 10 tokens"}
	}
	if p.Config.ChunkSizeMax < p.Config.ChunkSizeMin {
		return &ValidationError{Field: "chunkSizeMax", Message: "maximum chunk size must be greater than minimum"}
	}
	if p.Config.MaxResponseBytes < 1000 {
		return &ValidationError{Field: "maxResponseBytes", Message: "max response bytes must be at least 1000"}
	}
	if strings.TrimSpace(p.Config.RootPath) == "" {
		return &ValidationError{Field: "rootPath", Message: "project root path cannot be empty"}
	}
	return nil
}

// ValidationError represents a project validation error.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
