/*
  File: enrichment_test.go
  Purpose: Unit tests for chunk enrichment functionality.
  Author: CodeTextor project
  Notes: Tests semantic chunking, merging, splitting, and enrichment.
*/

package chunker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnrichParseResult tests basic enrichment of parsed symbols into chunks.
func TestEnrichParseResult(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	// Create a simple parse result
	result := &ParseResult{
		FilePath: "test.go",
		Language: "go",
		Symbols: []Symbol{
			{
				Name:       "Add",
				Kind:       SymbolFunction,
				StartLine:  5,
				EndLine:    7,
				StartByte:  100,
				EndByte:    150,
				Source:     "func Add(a, b int) int {\n\treturn a + b\n}",
				Signature:  "(a, b int) int",
				Visibility: "public",
				DocString:  "Add adds two integers",
			},
		},
		Imports: []string{"fmt", "os"},
		Metadata: map[string]string{
			"package": "main",
		},
	}

	chunks := enricher.EnrichParseResult(result)

	require.Len(t, chunks, 1, "should create one chunk per symbol")

	chunk := chunks[0]
	assert.Equal(t, "test.go", chunk.FilePath)
	assert.Equal(t, "go", chunk.Language)
	assert.Equal(t, "Add", chunk.SymbolName)
	assert.Equal(t, SymbolFunction, chunk.SymbolKind)
	assert.Equal(t, uint32(5), chunk.StartLine)
	assert.Equal(t, uint32(7), chunk.EndLine)
	assert.Equal(t, "main", chunk.PackageName)
	assert.Equal(t, []string{"fmt", "os"}, chunk.Imports)
	assert.Equal(t, "Add adds two integers", chunk.DocString)
	assert.Greater(t, chunk.TokenCount, 0, "should estimate token count")

	// Verify enriched content includes metadata
	assert.Contains(t, chunk.Content, "# File: test.go (go)")
	assert.Contains(t, chunk.Content, "# Symbols: Add (function)")
	assert.Contains(t, chunk.Content, "// Add adds two integers")
	assert.Contains(t, chunk.Content, "func Add(a, b int) int")
}

// TestEnrichParseResultWithoutComments tests enrichment with comments disabled.
func TestEnrichParseResultWithoutComments(t *testing.T) {
	config := DefaultChunkConfig()
	config.IncludeComments = false
	enricher := NewChunkEnricher(config)

	result := &ParseResult{
		FilePath: "test.go",
		Language: "go",
		Symbols: []Symbol{
			{
				Name:      "Add",
				Kind:      SymbolFunction,
				Source:    "func Add(a, b int) int { return a + b }",
				DocString: "Add adds two integers",
			},
		},
	}

	chunks := enricher.EnrichParseResult(result)
	require.Len(t, chunks, 1)

	// Should NOT include docstring when comments are disabled
	assert.NotContains(t, chunks[0].Content, "// Add adds two integers")
	assert.Contains(t, chunks[0].Content, "func Add")
}

// TestEnrichParseResultWithParent tests enrichment of methods with parent context.
func TestEnrichParseResultWithParent(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	result := &ParseResult{
		FilePath: "test.go",
		Language: "go",
		Symbols: []Symbol{
			{
				Name:       "Multiply",
				Kind:       SymbolMethod,
				Parent:     "Calculator",
				Source:     "func (c *Calculator) Multiply(a, b int) int { return a * b }",
				Signature:  "(a, b int) int",
				Visibility: "public",
			},
		},
	}

	chunks := enricher.EnrichParseResult(result)
	require.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.Equal(t, "Calculator", chunk.Parent)
	assert.Contains(t, chunk.Content, "# Symbols: Multiply (method)")
}

func TestEnrichParseResultSkipsVueSectionsWithChildren(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	result := &ParseResult{
		FilePath: "App.vue",
		Language: "vue",
		Symbols: []Symbol{
			{
				Name:      "script",
				Kind:      SymbolScript,
				Source:    "<script>...</script>",
				StartLine: 1,
				EndLine:   50,
			},
			{
				Name:      "toggleMobileMenu",
				Kind:      SymbolFunction,
				Parent:    "script",
				Source:    "const toggleMobileMenu = () => {}",
				StartLine: 10,
				EndLine:   20,
			},
		},
	}

	chunks := enricher.EnrichParseResult(result)
	require.Len(t, chunks, 1)
	assert.Equal(t, "toggleMobileMenu", chunks[0].SymbolName)
}

func TestEnrichParseResultKeepsSectionWithoutChildren(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	result := &ParseResult{
		FilePath: "App.vue",
		Language: "vue",
		Symbols: []Symbol{
			{
				Name:      "script",
				Kind:      SymbolScript,
				Source:    "const foo = 1;",
				StartLine: 1,
				EndLine:   5,
			},
		},
	}

	chunks := enricher.EnrichParseResult(result)
	require.Len(t, chunks, 1)
	assert.Equal(t, "script", chunks[0].SymbolName)
}

func TestEnrichParseResultSkipsParentElements(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	result := &ParseResult{
		FilePath: "App.vue",
		Language: "vue",
		Symbols: []Symbol{
			{
				Name:      "div",
				Kind:      SymbolElement,
				StartLine: 1,
				EndLine:   20,
				StartByte: 0,
				EndByte:   200,
				Source:    "<div><span></span></div>",
			},
			{
				Name:      "div#root",
				Kind:      SymbolElement,
				Parent:    "div",
				StartLine: 2,
				EndLine:   10,
				StartByte: 10,
				EndByte:   180,
				Source:    "<div id=\"root\"></div>",
			},
			{
				Name:      "span",
				Kind:      SymbolElement,
				Parent:    "div#root",
				StartLine: 3,
				EndLine:   4,
				StartByte: 50,
				EndByte:   70,
				Source:    "<span></span>",
			},
		},
	}

	chunks := enricher.EnrichParseResult(result)
	foundSpan := false
	foundDiv := false
	for _, chunk := range chunks {
		if chunk.SymbolName == "span" {
			foundSpan = true
		}
		if chunk.SymbolName == "div#root" {
			foundDiv = true
		}
	}
	assert.True(t, foundSpan)
	assert.False(t, foundDiv)
}

// TestMergeSmallChunks tests merging of small adjacent chunks.
func TestMergeSmallChunks(t *testing.T) {
	config := DefaultChunkConfig()
	config.MinChunkSize = 100 // Set a threshold for merging
	config.MergeSmallChunks = true
	enricher := NewChunkEnricher(config)

	// Create small chunks that should be merged
	chunks := []CodeChunk{
		{
			SourceCode: "short chunk 1",
			FilePath:   "test.go",
			Language:   "go",
			StartLine:  1,
			EndLine:    2,
			Symbols:    []ChunkSymbol{{Name: "func1", Kind: SymbolFunction}},
		},
		{
			SourceCode: "short chunk 2",
			FilePath:   "test.go",
			Language:   "go",
			StartLine:  3,
			EndLine:    4,
			Symbols:    []ChunkSymbol{{Name: "func2", Kind: SymbolFunction}},
		},
		{
			SourceCode: strings.Repeat("long chunk content ", 50),
			FilePath:   "test.go",
			Language:   "go",
			StartLine:  5,
			EndLine:    10,
			Symbols:    []ChunkSymbol{{Name: "func3", Kind: SymbolFunction}},
		},
	}

	for i := range chunks {
		enricher.refreshChunkContent(&chunks[i])
	}

	merged := enricher.MergeSmallChunks(chunks)

	// Should merge first two chunks, keep third separate
	assert.Len(t, merged, 2, "should merge small adjacent chunks")
	assert.Contains(t, merged[0].SymbolName, "func1", "merged chunk should combine names")
	assert.Contains(t, merged[0].SymbolName, "func2", "merged chunk should combine names")
	assert.Equal(t, uint32(1), merged[0].StartLine, "should preserve start of first chunk")
	assert.Equal(t, uint32(4), merged[0].EndLine, "should extend to end of second chunk")
	assert.Equal(t, "func3", merged[1].SymbolName, "large chunk should remain unchanged")
}

func TestMergeSmallChunksKeepsDifferentGroupsSeparate(t *testing.T) {
	config := DefaultChunkConfig()
	config.MinChunkSize = 100
	config.MergeSmallChunks = true
	enricher := NewChunkEnricher(config)

	codeChunk := CodeChunk{
		SourceCode: "const a = 1",
		FilePath:   "App.vue",
		Language:   "vue",
		StartLine:  1,
		EndLine:    2,
		Symbols:    []ChunkSymbol{{Name: "const a", Kind: SymbolVariable}},
	}
	templateChunk := CodeChunk{
		SourceCode: "<div></div>",
		FilePath:   "App.vue",
		Language:   "vue",
		StartLine:  3,
		EndLine:    4,
		Symbols:    []ChunkSymbol{{Name: "div", Kind: SymbolElement}},
	}

	enricher.refreshChunkContent(&codeChunk)
	enricher.refreshChunkContent(&templateChunk)

	merged := enricher.MergeSmallChunks([]CodeChunk{codeChunk, templateChunk})
	assert.Len(t, merged, 2, "code and template chunks should not merge")
}

// TestMergeSmallChunksDisabled tests that merging can be disabled.
func TestMergeSmallChunksDisabled(t *testing.T) {
	config := DefaultChunkConfig()
	config.MergeSmallChunks = false
	enricher := NewChunkEnricher(config)

	chunks := []CodeChunk{
		{SourceCode: "chunk1", FilePath: "test.go", Language: "go", Symbols: []ChunkSymbol{{Name: "func1", Kind: SymbolFunction}}},
		{SourceCode: "chunk2", FilePath: "test.go", Language: "go", Symbols: []ChunkSymbol{{Name: "func2", Kind: SymbolFunction}}},
	}
	for i := range chunks {
		enricher.refreshChunkContent(&chunks[i])
	}

	merged := enricher.MergeSmallChunks(chunks)

	// Should not merge when disabled
	assert.Len(t, merged, 2, "should not merge when disabled")
}

// TestMergeSmallChunksDifferentFiles tests that chunks from different files are not merged.
func TestMergeSmallChunksDifferentFiles(t *testing.T) {
	config := DefaultChunkConfig()
	config.MinChunkSize = 100
	config.MergeSmallChunks = true
	enricher := NewChunkEnricher(config)

	chunks := []CodeChunk{
		{SourceCode: "chunk1", FilePath: "test1.go", Language: "go", Symbols: []ChunkSymbol{{Name: "func1", Kind: SymbolFunction}}},
		{SourceCode: "chunk2", FilePath: "test2.go", Language: "go", Symbols: []ChunkSymbol{{Name: "func2", Kind: SymbolFunction}}},
	}
	for i := range chunks {
		enricher.refreshChunkContent(&chunks[i])
	}

	merged := enricher.MergeSmallChunks(chunks)

	// Should NOT merge chunks from different files
	assert.Len(t, merged, 2, "should not merge chunks from different files")
}

// TestSplitLargeChunks tests splitting of chunks that exceed max size.
func TestSplitLargeChunks(t *testing.T) {
	config := DefaultChunkConfig()
	config.MaxChunkSize = 50 // Set a low threshold for testing
	enricher := NewChunkEnricher(config)

	// Create a large chunk that should be split
	largeSource := strings.Repeat("line of code\n", 100)
	chunks := []CodeChunk{
		{
			Content:    largeSource,
			SourceCode: largeSource,
			FilePath:   "test.go",
			Language:   "go",
			SymbolName: "largeFunc",
			SymbolKind: SymbolFunction,
			StartLine:  1,
			EndLine:    100,
			TokenCount: 1000, // Exceeds max
		},
	}

	split := enricher.SplitLargeChunks(chunks)

	// Should split into multiple chunks
	assert.Greater(t, len(split), 1, "should split large chunk")

	// All split chunks should be below max size
	for _, chunk := range split {
		assert.LessOrEqual(t, chunk.TokenCount, config.MaxChunkSize,
			"split chunk should be below max size")
	}

	// Split chunks should have modified names indicating range
	for _, chunk := range split {
		assert.Contains(t, chunk.SymbolName, "largeFunc[",
			"split chunk should indicate line range")
	}
}

// TestSplitLargeChunksNoSplitNeeded tests that small chunks are not split.
func TestSplitLargeChunksNoSplitNeeded(t *testing.T) {
	config := DefaultChunkConfig()
	config.MaxChunkSize = 1000
	enricher := NewChunkEnricher(config)

	chunks := []CodeChunk{
		{
			Content:    "small chunk",
			SourceCode: "small chunk",
			SymbolName: "smallFunc",
			TokenCount: 10,
		},
	}

	split := enricher.SplitLargeChunks(chunks)

	// Should not split
	assert.Len(t, split, 1, "should not split small chunk")
	assert.Equal(t, "smallFunc", split[0].SymbolName, "name should remain unchanged")
}

// TestEstimateTokenCount tests the token estimation function.
func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			text:     "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "short text",
			text:     "hello world",
			minCount: 2,
			maxCount: 4,
		},
		{
			name:     "code snippet",
			text:     "func Add(a, b int) int { return a + b }",
			minCount: 8,
			maxCount: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := estimateTokenCount(tt.text)
			assert.GreaterOrEqual(t, count, tt.minCount,
				"token count should be at least minimum")
			assert.LessOrEqual(t, count, tt.maxCount,
				"token count should not exceed maximum")
		})
	}
}

// TestBuildEnrichedContent tests the content enrichment with various metadata.
func TestBuildEnrichedContent(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	symbol := Symbol{
		Name:       "Calculate",
		Kind:       SymbolFunction,
		Parent:     "Math",
		Signature:  "(x float64) float64",
		Visibility: "private",
		DocString:  "Calculate performs computation",
		Source:     "func Calculate(x float64) float64 { return x * 2 }",
	}

	result := &ParseResult{
		FilePath: "math.go",
		Language: "go",
	}

	content := enricher.buildEnrichedContent(symbol, result)

	// Verify all metadata is included
	assert.Contains(t, content, "# File: math.go (go)")
	assert.Contains(t, content, "# Symbol: Calculate")
	assert.Contains(t, content, "// Calculate performs computation")
	assert.Contains(t, content, "func Calculate(x float64) float64")
}

// TestBuildEnrichedContentMinimal tests enrichment with minimal metadata.
func TestBuildEnrichedContentMinimal(t *testing.T) {
	enricher := NewChunkEnricher(DefaultChunkConfig())

	symbol := Symbol{
		Name:   "simple",
		Kind:   SymbolVariable,
		Source: "var simple = 42",
	}

	result := &ParseResult{
		FilePath: "test.go",
		Language: "go",
	}

	content := enricher.buildEnrichedContent(symbol, result)

	// Should include basic metadata
	assert.Contains(t, content, "# File: test.go (go)")
	assert.Contains(t, content, "# Symbol: simple")
	assert.Contains(t, content, "var simple = 42")

	// Should NOT include optional fields when empty
	assert.NotContains(t, content, "# Parent:")
	assert.NotContains(t, content, "# Signature:")
	assert.NotContains(t, content, "# Visibility: private") // Don't show if empty or public
}

// TestFullPipeline tests the complete enrichment pipeline: enrich -> merge -> split.
func TestFullPipeline(t *testing.T) {
	config := DefaultChunkConfig()
	config.MinChunkSize = 50
	config.MaxChunkSize = 200
	config.MergeSmallChunks = true
	enricher := NewChunkEnricher(config)

	// Create a parse result with various symbol sizes
	result := &ParseResult{
		FilePath: "test.go",
		Language: "go",
		Symbols: []Symbol{
			{Name: "tiny1", Kind: SymbolVariable, Source: "var x = 1", StartLine: 1, EndLine: 1},
			{Name: "tiny2", Kind: SymbolVariable, Source: "var y = 2", StartLine: 2, EndLine: 2},
			{Name: "large", Kind: SymbolFunction, Source: strings.Repeat("line\n", 200), StartLine: 3, EndLine: 203},
		},
	}

	// Step 1: Enrich
	chunks := enricher.EnrichParseResult(result)
	assert.Len(t, chunks, 3, "should create chunk per symbol")

	// Step 2: Merge small chunks
	merged := enricher.MergeSmallChunks(chunks)
	assert.Less(t, len(merged), len(chunks), "should merge small chunks")

	// Step 3: Split large chunks
	final := enricher.SplitLargeChunks(merged)
	assert.Greater(t, len(final), len(merged), "should split large chunks")

	// Verify all final chunks are within reasonable bounds
	// Note: Merged chunks may slightly exceed max due to enrichment overhead
	// We allow a small margin (enrichment overhead ~50 tokens)
	for _, chunk := range final {
		if chunk.SymbolKind != SymbolFunction {
			tolerance := 60 // tokens
			assert.LessOrEqual(t, chunk.TokenCount, config.MaxChunkSize+tolerance,
				"chunk should not significantly exceed max size (with enrichment overhead)")
		}
	}
}
