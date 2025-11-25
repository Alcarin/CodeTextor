package models

// SearchResponse represents the result of a semantic search against a project's index.
type SearchResponse struct {
	Chunks       []*Chunk `json:"chunks"`
	TotalResults int      `json:"totalResults"`
	QueryTimeMs  int64    `json:"queryTime"`
}

// SearchRequest represents a semantic search query.
type SearchRequest struct {
	ProjectID string `json:"projectId"`
	Query     string `json:"query"`
	K         int    `json:"k"`
}
