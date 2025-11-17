/*
  File: enrichment.go
  Purpose: Chunk enrichment and semantic chunking based on parsed symbols.
  Author: CodeTextor project
  Notes: Transforms raw Symbol data into enriched, context-aware code chunks for embedding.
*/

package chunker

import (
	"fmt"
	"strings"
)

// CodeChunk represents an enriched, semantically meaningful unit of code.
// It combines the raw symbol information with additional context metadata.
type ChunkSymbol struct {
	Name string     `json:"name"`
	Kind SymbolKind `json:"kind"`
}

type CodeChunk struct {
	// Core content
	Content    string `json:"content"`     // Enriched content with metadata headers
	SourceCode string `json:"source_code"` // Raw source code without enrichment

	// Location information
	FilePath  string `json:"file_path"`  // Path to the source file
	StartLine uint32 `json:"start_line"` // Starting line number (1-indexed)
	EndLine   uint32 `json:"end_line"`   // Ending line number (1-indexed)
	StartByte uint32 `json:"start_byte"` // Starting byte offset
	EndByte   uint32 `json:"end_byte"`   // Ending byte offset

	// Semantic metadata
	Language   string     `json:"language"`             // Programming language
	SymbolName string     `json:"symbol_name"`          // Name of the symbol (function, class, etc.)
	SymbolKind SymbolKind `json:"symbol_kind"`          // Type of symbol
	Parent     string     `json:"parent,omitempty"`     // Parent symbol (e.g., class for a method)
	Signature  string     `json:"signature,omitempty"`  // Function signature or type definition
	Visibility string     `json:"visibility,omitempty"` // public, private, protected, etc.
	Symbols    []ChunkSymbol

	// Context enrichment
	PackageName string   `json:"package_name,omitempty"` // Package/module name
	Imports     []string `json:"imports,omitempty"`      // Relevant imports for this chunk
	DocString   string   `json:"doc_string,omitempty"`   // Documentation/comments

	// Chunk metadata
	TokenCount  int  `json:"token_count"`  // Estimated token count
	IsCollapsed bool `json:"is_collapsed"` // Whether the body was collapsed
}

// ChunkEnricher handles the enrichment and transformation of parsed symbols into code chunks.
type ChunkEnricher struct {
	config ChunkConfig
}

// NewChunkEnricher creates a new chunk enricher with the given configuration.
func NewChunkEnricher(config ChunkConfig) *ChunkEnricher {
	return &ChunkEnricher{
		config: config,
	}
}

// refreshChunkContent regenerates the enriched content and token count for a chunk.
func (e *ChunkEnricher) refreshChunkContent(chunk *CodeChunk) {
	e.updateSymbolSummary(chunk)
	chunk.Content = e.buildEnrichedContentFromChunk(chunk)
	chunk.TokenCount = estimateTokenCount(chunk.Content)
}

func (e *ChunkEnricher) updateSymbolSummary(chunk *CodeChunk) {
	if len(chunk.Symbols) == 0 {
		return
	}

	if len(chunk.Symbols) == 1 {
		chunk.SymbolName = chunk.Symbols[0].Name
		chunk.SymbolKind = chunk.Symbols[0].Kind
		return
	}

	names := make([]string, 0, len(chunk.Symbols))
	for _, sym := range chunk.Symbols {
		if sym.Name != "" {
			names = append(names, fmt.Sprintf("%s (%s)", sym.Name, sym.Kind))
		} else {
			names = append(names, string(sym.Kind))
		}
	}

	chunk.SymbolName = strings.Join(names, ", ")
	chunk.SymbolKind = SymbolKind("group")
}

func (e *ChunkEnricher) buildEnrichedContentFromChunk(chunk *CodeChunk) string {
	var builder strings.Builder

	// File context
	builder.WriteString(fmt.Sprintf("# File: %s (%s)\n", chunk.FilePath, chunk.Language))

	// Symbol metadata (if available)
	if len(chunk.Symbols) > 0 {
		builder.WriteString("# Symbols: ")
		symbolEntries := make([]string, 0, len(chunk.Symbols))
		for _, sym := range chunk.Symbols {
			if sym.Name != "" {
				symbolEntries = append(symbolEntries, fmt.Sprintf("%s (%s)", sym.Name, sym.Kind))
			} else {
				symbolEntries = append(symbolEntries, string(sym.Kind))
			}
		}
		builder.WriteString(strings.Join(symbolEntries, ", "))
		builder.WriteString("\n")
	} else if chunk.SymbolName != "" {
		builder.WriteString(fmt.Sprintf("# Symbol: %s\n", chunk.SymbolName))
	}

	builder.WriteString("\n")

	// Docstrings/comments
	if e.config.IncludeComments && chunk.DocString != "" {
		docLines := strings.Split(chunk.DocString, "\n")
		for _, line := range docLines {
			builder.WriteString("// ")
			builder.WriteString(strings.TrimSpace(line))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// Source content
	builder.WriteString(chunk.SourceCode)
	return builder.String()
}

// EnrichParseResult converts a ParseResult into enriched CodeChunks.
// It performs the following transformations:
//   - Extracts semantic metadata from symbols
//   - Adds file and language context
//   - Merges leading comments into chunks
//   - Adds package/import information
//   - Estimates token counts
//
// Parameters:
//   - result: The parsed file result containing symbols and metadata
//
// Returns a slice of enriched CodeChunks ready for embedding.
func (e *ChunkEnricher) EnrichParseResult(result *ParseResult) []CodeChunk {
	var chunks []CodeChunk

	// Extract package name from metadata if available
	packageName := result.Metadata["package"]

	skipSymbol := computeSkippableSymbols(result.Symbols)

	// Process each symbol and convert to enriched chunk
	for idx, symbol := range result.Symbols {
		// Skip link-only symbols (handled within their parent sections)
		if symbol.Kind == SymbolMarkdownLink {
			continue
		}

		if skipSymbol[idx] {
			continue
		}

		// Skip local variables/constants (only keep top-level ones)
		if (symbol.Kind == SymbolVariable || symbol.Kind == SymbolConstant) && symbol.Parent != "" {
			continue
		}

		chunk := e.symbolToChunk(symbol, result)
		chunk.PackageName = packageName
		chunk.Imports = result.Imports
		chunks = append(chunks, chunk)
	}

	return chunks
}

// symbolToChunk converts a single Symbol into an enriched CodeChunk.
// Parameters:
//   - symbol: The symbol to convert
//   - result: The parent ParseResult for additional context
//
// Returns an enriched CodeChunk with all metadata populated.
func (e *ChunkEnricher) symbolToChunk(symbol Symbol, result *ParseResult) CodeChunk {
	chunk := CodeChunk{
		SourceCode:  symbol.Source,
		FilePath:    result.FilePath,
		StartLine:   symbol.StartLine,
		EndLine:     symbol.EndLine,
		StartByte:   symbol.StartByte,
		EndByte:     symbol.EndByte,
		Language:    result.Language,
		SymbolName:  symbol.Name,
		SymbolKind:  symbol.Kind,
		Parent:      symbol.Parent,
		Signature:   symbol.Signature,
		Visibility:  symbol.Visibility,
		DocString:   symbol.DocString,
		IsCollapsed: false,
		Symbols: []ChunkSymbol{
			{Name: symbol.Name, Kind: symbol.Kind},
		},
	}
	e.refreshChunkContent(&chunk)
	return chunk
}

// buildEnrichedContent constructs the chunk content with contextual metadata.
// The enriched content includes:
//   - File path header
//   - Language identifier
//   - Symbol type and name
//   - Parent context (if applicable)
//   - Documentation/comments
//   - The actual source code
//
// This enrichment helps the embedding model understand the context and purpose
// of the code chunk, improving retrieval quality.
//
// Parameters:
//   - symbol: The symbol to enrich
//   - result: The parse result for additional context
//
// Returns the enriched content as a string.
func (e *ChunkEnricher) buildEnrichedContent(symbol Symbol, result *ParseResult) string {
	chunk := CodeChunk{
		FilePath:   result.FilePath,
		Language:   result.Language,
		SymbolName: symbol.Name,
		SymbolKind: symbol.Kind,
		Parent:     symbol.Parent,
		Signature:  symbol.Signature,
		Visibility: symbol.Visibility,
		DocString:  symbol.DocString,
		SourceCode: symbol.Source,
	}
	return e.buildEnrichedContentFromChunk(&chunk)
}

// MergeSmallChunks combines adjacent small chunks to meet the minimum chunk size.
// This is useful when parsing produces many tiny symbols that would be inefficient
// to embed separately.
//
// Parameters:
//   - chunks: The input chunks to potentially merge
//
// Returns a slice of chunks where small adjacent chunks have been combined.
func (e *ChunkEnricher) MergeSmallChunks(chunks []CodeChunk) []CodeChunk {
	if !e.config.MergeSmallChunks || len(chunks) == 0 {
		return chunks
	}

	var merged []CodeChunk
	var current *CodeChunk
	currentWasMerged := false
	targetSize := e.preferredChunkSize()

	for i := 0; i < len(chunks); i++ {
		chunk := chunks[i]

		// If this is the first chunk or we can't merge, start a new group
		if current == nil {
			current = &chunk
			currentWasMerged = false
			continue
		}

		// Check if we should merge this chunk with the current one
		// Merge if:
		// 1. Both chunks are below MinChunkSize, OR
		// 2. Current chunk is very small (<50 tokens), hasn't already been merged, and the merge won't exceed MaxChunkSize, OR
		// 3. Incoming chunk is very small (<50 tokens) and merging won't exceed MaxChunkSize
		verySmallThreshold := 50
		wouldExceedMax := (current.TokenCount + chunk.TokenCount) > e.config.MaxChunkSize
		currentTiny := current.TokenCount < verySmallThreshold
		incomingTiny := chunk.TokenCount < verySmallThreshold
		bothBelowMin := current.TokenCount < e.config.MinChunkSize && chunk.TokenCount < e.config.MinChunkSize

		if current.TokenCount >= targetSize {
			merged = append(merged, *current)
			current = &chunk
			currentWasMerged = false
			continue
		}

		shouldMerge := current.FilePath == chunk.FilePath &&
			current.Language == chunk.Language &&
			chunkSemanticGroup(*current) == chunkSemanticGroup(chunk) &&
			!wouldExceedMax &&
			(bothBelowMin ||
				(!currentWasMerged && currentTiny) ||
				incomingTiny)

		if shouldMerge {
			// Merge the chunks
			currentWasMerged = true
			*current = e.mergeTwoChunks(*current, chunk)
			continue
		}

		// If the incoming chunk is tiny but couldn't merge with the previous chunk (e.g. the merge
		// would exceed MaxChunkSize), try to merge it with the next chunk to avoid isolated tiny chunks.
		if chunk.TokenCount < verySmallThreshold && i+1 < len(chunks) {
			nextChunk := chunks[i+1]
			forwardWouldExceedMax := (chunk.TokenCount + nextChunk.TokenCount) > e.config.MaxChunkSize
			forwardShouldMerge := chunk.FilePath == nextChunk.FilePath &&
				chunk.Language == nextChunk.Language &&
				chunkSemanticGroup(chunk) == chunkSemanticGroup(nextChunk) &&
				!forwardWouldExceedMax

			if forwardShouldMerge {
				merged = append(merged, *current)
				mergedForward := e.mergeTwoChunks(chunk, nextChunk)
				current = &mergedForward
				currentWasMerged = true
				i++ // Skip the chunk we just consumed via forward merge
				continue
			}
		}

		if current.TokenCount >= targetSize {
			merged = append(merged, *current)
			current = nil
			continue
		}

		// Save the current chunk and start a new one
		merged = append(merged, *current)
		current = &chunk
		currentWasMerged = false
	}
	// Don't forget the last chunk
	if current != nil {
		merged = append(merged, *current)
	}

	return merged
}

// mergeTwoChunks combines two chunks into a single chunk.
// Used internally by MergeSmallChunks.
//
// Parameters:
//   - first: The first chunk
//   - second: The second chunk
//
// Returns a new chunk that combines both inputs.
func (e *ChunkEnricher) mergeTwoChunks(first, second CodeChunk) CodeChunk {
	merged := first
	merged.Symbols = append(append([]ChunkSymbol{}, merged.Symbols...), second.Symbols...)

	// Update range to span both chunks - ensure we always have min/max lines
	if second.StartLine < merged.StartLine {
		merged.StartLine = second.StartLine
	}
	if second.EndLine > merged.EndLine {
		merged.EndLine = second.EndLine
	}

	// Update byte range similarly
	if second.StartByte < merged.StartByte || merged.StartByte == 0 {
		merged.StartByte = second.StartByte
	}
	if second.EndByte > merged.EndByte {
		merged.EndByte = second.EndByte
	}

	// Combine source code with spacing, optionally stripping comments from text chunks when comments disabled
	firstSource := merged.SourceCode
	secondSource := second.SourceCode
	if !e.config.IncludeComments && merged.SymbolKind == "text" {
		firstSource = stripCommentLines(firstSource)
	}
	if !e.config.IncludeComments && second.SymbolKind == "text" {
		secondSource = stripCommentLines(secondSource)
	}

	switch {
	case strings.TrimSpace(firstSource) == "":
		merged.SourceCode = secondSource
	case strings.TrimSpace(secondSource) == "":
		merged.SourceCode = firstSource
	default:
		merged.SourceCode = strings.TrimRight(firstSource, "\n") + "\n\n" + strings.TrimLeft(secondSource, "\n")
	}

	// Combine docstrings if both present
	if first.DocString != "" && second.DocString != "" {
		merged.DocString = first.DocString + "\n" + second.DocString
	} else if second.DocString != "" {
		merged.DocString = second.DocString
	}

	e.refreshChunkContent(&merged)
	return merged
}

func stripCommentLines(source string) string {
	lines := strings.Split(source, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func chunkSemanticGroup(chunk CodeChunk) string {
	if hasTemplateSymbol(chunk.Symbols) || isTemplateKind(chunk.SymbolKind) {
		return "template"
	}
	if hasStyleSymbol(chunk.Symbols) || isStyleKind(chunk.SymbolKind) {
		return "style"
	}
	return "code"
}

func hasTemplateSymbol(symbols []ChunkSymbol) bool {
	for _, sym := range symbols {
		if isTemplateKind(sym.Kind) {
			return true
		}
	}
	return false
}

func hasStyleSymbol(symbols []ChunkSymbol) bool {
	for _, sym := range symbols {
		if isStyleKind(sym.Kind) {
			return true
		}
	}
	return false
}

func isTemplateKind(kind SymbolKind) bool {
	return kind == SymbolElement
}

func isStyleKind(kind SymbolKind) bool {
	switch kind {
	case SymbolStyle, SymbolCSSRule, SymbolCSSMedia, SymbolCSSKeyframes:
		return true
	default:
		return false
	}
}

func (e *ChunkEnricher) preferredChunkSize() int {
	size := e.config.MaxChunkSize
	if size <= 0 {
		size = 800
	}
	preferred := size / 2
	if preferred <= 0 {
		preferred = size
	}
	if e.config.MinChunkSize > 0 && preferred < e.config.MinChunkSize {
		preferred = e.config.MinChunkSize
	}
	if preferred < 100 {
		preferred = 100
	}
	return preferred
}

func computeSkippableSymbols(symbols []Symbol) []bool {
	hasChildren := make([]bool, len(symbols))

	for i := range symbols {
		switch symbols[i].Kind {
		case SymbolElement, SymbolScript, SymbolStyle, SymbolCSSRule, SymbolCSSMedia, SymbolCSSKeyframes:
			for j := range symbols {
				if i == j {
					continue
				}
				if symbolContains(symbols[i], symbols[j]) {
					hasChildren[i] = true
					break
				}
			}
		case SymbolMarkdownHeading:
			for j := range symbols {
				if i == j {
					continue
				}
				if symbols[j].Parent == symbols[i].Name &&
					symbols[j].StartLine >= symbols[i].StartLine {
					hasChildren[i] = true
					break
				}
			}
		}
	}

	return hasChildren
}

func symbolContains(parent, child Symbol) bool {
	if parent.StartByte != 0 || parent.EndByte != 0 {
		if parent.StartByte > child.StartByte || parent.EndByte < child.EndByte {
			return false
		}
		if parent.StartByte == child.StartByte && parent.EndByte == child.EndByte {
			return false
		}
		return true
	}

	if parent.StartLine > child.StartLine || parent.EndLine < child.EndLine {
		return false
	}
	if parent.StartLine == child.StartLine && parent.EndLine == child.EndLine {
		return false
	}
	return true
}

// SplitLargeChunks splits chunks that exceed the maximum token limit.
// This ensures that no chunk is too large for the embedding model.
// Note: We compare against Content (enriched) token count, since that's what
// gets embedded, but we split based on SourceCode to maintain proper boundaries.
//
// Parameters:
//   - chunks: The input chunks to potentially split
//
// Returns a slice of chunks where large chunks have been split.
func (e *ChunkEnricher) SplitLargeChunks(chunks []CodeChunk) []CodeChunk {
	var result []CodeChunk
	target := e.preferredChunkSize()

	for _, chunk := range chunks {
		// Check if the enriched content exceeds the limit
		if chunk.TokenCount > target {
			// Split this chunk based on source code
			splitChunks := e.splitChunk(chunk, target)
			result = append(result, splitChunks...)
		} else {
			// Keep as-is
			result = append(result, chunk)
		}
	}

	return result
}

// splitChunk splits a single large chunk into multiple smaller chunks.
// It tries to split at natural boundaries (newlines) when possible.
// Strategy: We estimate the overhead of enrichment headers (~200 chars = 50 tokens)
// and split the source code to ensure the final enriched content stays under the limit.
//
// Parameters:
//   - chunk: The chunk to split
//
// Returns a slice of smaller chunks.
func (e *ChunkEnricher) splitChunk(chunk CodeChunk, targetTokens int) []CodeChunk {
	lines := strings.Split(chunk.SourceCode, "\n")
	var result []CodeChunk
	var currentLines []string
	// Reserve space for enrichment headers (~50 tokens)
	enrichmentOverhead := 50
	if targetTokens <= 0 {
		targetTokens = 400
	}
	maxSourceTokens := targetTokens - enrichmentOverhead
	if maxSourceTokens < 10 {
		maxSourceTokens = 10 // Minimum viable chunk
	}

	var currentTokens int
	startLine := chunk.StartLine

	for i, line := range lines {
		lineTokens := estimateTokenCount(line)

		// If adding this line would exceed the max (accounting for overhead), save current chunk
		if currentTokens+lineTokens > maxSourceTokens && len(currentLines) > 0 {
			newChunk := e.createSplitChunk(chunk, currentLines, startLine, startLine+uint32(len(currentLines))-1)
			result = e.appendBalancedChunk(result, newChunk, targetTokens)
			currentLines = []string{}
			currentTokens = 0
			startLine = chunk.StartLine + uint32(i)
		}

		currentLines = append(currentLines, line)
		currentTokens += lineTokens
	}

	// Add remaining lines
	if len(currentLines) > 0 {
		endLine := startLine + uint32(len(currentLines)) - 1
		newChunk := e.createSplitChunk(chunk, currentLines, startLine, endLine)
		result = e.appendBalancedChunk(result, newChunk, targetTokens)
	}

	return result
}

// createSplitChunk creates a new chunk from a subset of lines.
// Helper function for splitChunk.
//
// Parameters:
//   - original: The original chunk being split
//   - lines: The lines for this split chunk
//   - startLine: Starting line number
//   - endLine: Ending line number
//
// Returns a new CodeChunk representing the split portion.
func (e *ChunkEnricher) createSplitChunk(original CodeChunk, lines []string, startLine, endLine uint32) CodeChunk {
	sourceCode := strings.Join(lines, "\n")

	split := original
	split.SourceCode = sourceCode
	split.StartLine = startLine
	split.EndLine = endLine
	split.SymbolName = fmt.Sprintf("%s[%d-%d]", original.SymbolName, startLine, endLine)

	// Reset byte positions as we cannot accurately calculate them after splitting
	// These will be 0, which indicates they are not available for split chunks
	split.StartByte = 0
	split.EndByte = 0

	e.refreshChunkContent(&split)
	return split
}

func (e *ChunkEnricher) appendBalancedChunk(result []CodeChunk, chunk CodeChunk, target int) []CodeChunk {
	maxSize := e.config.MaxChunkSize
	if maxSize <= 0 {
		maxSize = target * 2
	}
	if chunk.TokenCount > maxSize && target > 20 {
		nextTarget := target / 2
		if nextTarget < 20 {
			nextTarget = 20
		}
		subChunks := e.splitChunk(chunk, nextTarget)
		return append(result, subChunks...)
	}
	return append(result, chunk)
}

// estimateTokenCount provides a rough estimate of token count for a string.
// Uses a simple heuristic: 1 token ≈ 4 characters (common approximation for English text).
// Code tends to be slightly more token-dense, but this is a reasonable estimate.
//
// Parameters:
//   - text: The text to estimate tokens for
//
// Returns the estimated token count.
func estimateTokenCount(text string) int {
	// Simple heuristic: 1 token ≈ 4 characters
	// This is a rough approximation but sufficient for chunking decisions
	return len(text) / 4
}
