package models

// GrepLine rappresenta una singola riga corrispondente in un file.
type GrepLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// GrepFileMatch raggruppa le corrispondenze trovate all'interno di un singolo file.
type GrepFileMatch struct {
	Path    string     `json:"path"`
	Matches []GrepLine `json:"matches"`
}

// GrepSearchResponse rappresenta la risposta strutturata per una ricerca testuale.
type GrepSearchResponse struct {
	Results      []GrepFileMatch `json:"results"`
	TotalMatches int             `json:"total"`
	QueryTimeMs  int64           `json:"timeMs"`
}
