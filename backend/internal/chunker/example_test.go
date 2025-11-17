/*
  File: example_test.go
  Purpose: Usage examples for the semantic chunking system.
  Author: CodeTextor project
  Notes: These are example tests that demonstrate how to use the semantic chunker.
*/

package chunker_test

import (
	"CodeTextor/backend/internal/chunker"
	"fmt"
	"os"
)

// ExampleSemanticChunker_basic demonstrates basic usage of the semantic chunker.
func ExampleSemanticChunker_basic() {
	// Create a semantic chunker with default configuration
	semanticChunker := chunker.NewSemanticChunker(chunker.DefaultChunkConfig())

	// Example Go code
	source := []byte(`package main

import "fmt"

// Greet prints a greeting message.
func Greet(name string) {
    fmt.Printf("Hello, %s!\n", name)
}

func main() {
    Greet("World")
}
`)

	// Chunk the file
	chunks, err := semanticChunker.ChunkFile("example.go", source)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display information about each chunk
	for i, chunk := range chunks {
		fmt.Printf("Chunk %d: %s (%s)\n", i+1, chunk.SymbolName, chunk.SymbolKind)
		fmt.Printf("  Lines: %d-%d\n", chunk.StartLine, chunk.EndLine)
		fmt.Printf("  Tokens: %d\n", chunk.TokenCount)
		fmt.Printf("  Language: %s\n", chunk.Language)
	}

	// Output will vary based on parsing and merging
	// Output: Chunk 1: Greet (function)
}

// ExampleSemanticChunker_withCustomConfig demonstrates using custom configuration.
func ExampleSemanticChunker_withCustomConfig() {
	// Create custom configuration
	config := chunker.ChunkConfig{
		MaxChunkSize:      1000, // Larger chunks
		MinChunkSize:      50,   // Smaller minimum
		CollapseThreshold: 500,
		MergeSmallChunks:  true,
		IncludeComments:   true,
	}

	semanticChunker := chunker.NewSemanticChunker(config)

	source := []byte(`def calculate(x):
    """Calculate something."""
    return x * 2

def process(data):
    """Process data."""
    return [calculate(x) for x in data]
`)

	chunks, err := semanticChunker.ChunkFile("example.py", source)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Created %d chunks\n", len(chunks))

	// Output will show merged or split chunks based on config
	// Output: Created 2 chunks
}

// ExampleSemanticChunker_checkSupport demonstrates checking file support.
func ExampleSemanticChunker_checkSupport() {
	semanticChunker := chunker.NewSemanticChunker(chunker.DefaultChunkConfig())

	files := []string{
		"example.go",
		"script.py",
		"component.tsx",
		"README.txt",
		"data.json",
	}

	for _, file := range files {
		if semanticChunker.IsSupported(file) {
			fmt.Printf("%s: supported\n", file)
		} else {
			fmt.Printf("%s: not supported\n", file)
		}
	}

	// Output:
	// example.go: supported
	// script.py: supported
	// component.tsx: supported
	// README.txt: not supported
	// data.json: supported
}

// ExampleSemanticChunker_enrichedContent demonstrates accessing enriched content.
func ExampleSemanticChunker_enrichedContent() {
	semanticChunker := chunker.NewSemanticChunker(chunker.DefaultChunkConfig())

	source := []byte(`package math

// Add adds two integers.
func Add(a, b int) int {
    return a + b
}
`)

	chunks, err := semanticChunker.ChunkFile("math.go", source)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(chunks) > 0 {
		chunk := chunks[0]

		// The Content field contains enriched metadata
		fmt.Println("Enriched content includes:")
		fmt.Println("- File path header")
		fmt.Println("- Language identifier")
		fmt.Println("- Symbol metadata")
		fmt.Println("- Documentation comments")
		fmt.Println("- Source code")

		// The SourceCode field contains raw code
		fmt.Printf("\nRaw source (%d chars):\n", len(chunk.SourceCode))

		// For embedding, use Content
		fmt.Printf("Enriched content (%d chars) is ready for embedding\n", len(chunk.Content))
	}

	// Output:
	// Enriched content includes:
	// - File path header
	// - Language identifier
	// - Symbol metadata
	// - Documentation comments
	// - Source code
	//
	// Raw source (84 chars):
	// Enriched content (159 chars) is ready for embedding
}

// ExampleSemanticChunker_realFile demonstrates processing a real file.
func ExampleSemanticChunker_realFile() {
	semanticChunker := chunker.NewSemanticChunker(chunker.DefaultChunkConfig())

	// Read a real file (replace with actual path for testing)
	source, err := os.ReadFile("types.go")
	if err != nil {
		// File might not exist in test environment
		fmt.Println("File processing example")
		return
	}

	chunks, err := semanticChunker.ChunkFile("types.go", source)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Processed file: types.go\n")
	fmt.Printf("Total chunks: %d\n", len(chunks))

	// Count chunks by type
	functionCount := 0
	structCount := 0
	for _, chunk := range chunks {
		switch chunk.SymbolKind {
		case chunker.SymbolFunction:
			functionCount++
		case chunker.SymbolStruct:
			structCount++
		}
	}

	fmt.Printf("Functions: %d\n", functionCount)
	fmt.Printf("Structs: %d\n", structCount)

	// Output will vary based on actual file
	// Output: Processed file: types.go
}

// ExampleChunkEnricher_pipeline demonstrates the enrichment pipeline.
func ExampleChunkEnricher_pipeline() {
	// This example shows the internal pipeline (usually called via SemanticChunker)
	enricher := chunker.NewChunkEnricher(chunker.DefaultChunkConfig())

	// Assume we have a ParseResult (normally from Parser)
	result := &chunker.ParseResult{
		FilePath: "example.go",
		Language: "go",
		Symbols: []chunker.Symbol{
			{
				Name:      "SmallFunc1",
				Kind:      chunker.SymbolFunction,
				Source:    "func SmallFunc1() {}",
				StartLine: 1,
				EndLine:   1,
			},
			{
				Name:      "SmallFunc2",
				Kind:      chunker.SymbolFunction,
				Source:    "func SmallFunc2() {}",
				StartLine: 3,
				EndLine:   3,
			},
		},
	}

	// Step 1: Enrich
	chunks := enricher.EnrichParseResult(result)
	fmt.Printf("After enrichment: %d chunks\n", len(chunks))

	// Step 2: Merge small chunks
	merged := enricher.MergeSmallChunks(chunks)
	fmt.Printf("After merging: %d chunks\n", len(merged))

	// Step 3: Split large chunks (if any)
	final := enricher.SplitLargeChunks(merged)
	fmt.Printf("After splitting: %d chunks\n", len(final))

	// Output:
	// After enrichment: 2 chunks
	// After merging: 1 chunks
	// After splitting: 1 chunks
}
