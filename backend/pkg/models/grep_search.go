package models

// GrepSearchResponse rappresenta la risposta strutturata per una ricerca testuale.
// Utilizza un formato tabulare [File, Line, Content] per ottimizzare l'output JSON.
type GrepSearchResponse struct {
	Results      [][]any `json:"results"` // Tabella di risultati: [Path, Line, Content]
	TotalMatches int     `json:"total"`
	QueryTimeMs  int64   `json:"timeMs"`
}
