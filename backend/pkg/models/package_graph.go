package models

// PackageGraphResponse rappresenta una mappa di adiacenza delle dipendenze tra package.
// Chiave 1: Package Sorgente (Path troncato alla profondità data)
// Chiave 2: Package Destinazione (Path troncato)
// Valore: Numero di riferimenti (Peso dell'arco)
type PackageGraphResponse map[string]map[string]int

// UsagePath rappresenta una singola relazione di dipendenza tra due file.
type UsagePath struct {
	SourcePath string
	TargetPath string
}
