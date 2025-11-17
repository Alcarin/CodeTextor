/*
  File: semantic_chunker.go
  Purpose: Public API for semantic code chunking.
  Author: CodeTextor project
  Notes: Provides high-level interface for parsing, enriching, and chunking source code files.
*/

package chunker

import (
	"fmt"
	"strings"
)

// SemanticChunker provides a complete pipeline for transforming source code
// into enriched, semantically meaningful chunks ready for embedding.
//
// It combines:
//   - Tree-sitter parsing (language-specific)
//   - Symbol extraction (functions, classes, etc.)
//   - Context enrichment (metadata, comments, imports)
//   - Adaptive sizing (merge small, split large)
type SemanticChunker struct {
	parser   *Parser
	enricher *ChunkEnricher
	config   ChunkConfig
}

// NewSemanticChunker creates a new semantic chunker with the given configuration.
//
// Parameters:
//   - config: Chunking configuration (sizes, thresholds, options)
//
// Returns a SemanticChunker ready to process source files.
func NewSemanticChunker(config ChunkConfig) *SemanticChunker {
	return &SemanticChunker{
		parser:   NewParser(config),
		enricher: NewChunkEnricher(config),
		config:   config,
	}
}

// ChunkFile processes a source code file and returns semantically enriched chunks.
//
// This is the main entry point for semantic chunking. It performs the complete pipeline:
//  1. Parse the file using tree-sitter
//  2. Extract symbols (functions, classes, etc.)
//  3. Enrich each symbol with context metadata
//  4. Merge small adjacent chunks
//  5. Split chunks that exceed maximum size
//
// Parameters:
//   - filePath: Path to the source file
//   - source: The file contents as bytes
//
// Returns:
//   - []CodeChunk: Enriched chunks ready for embedding
//   - error: Any error encountered during processing
//
// Example usage:
//
//	chunker := NewSemanticChunker(DefaultChunkConfig())
//	source, _ := os.ReadFile("example.go")
//	chunks, err := chunker.ChunkFile("example.go", source)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, chunk := range chunks {
//	    fmt.Printf("Chunk: %s (%d tokens)\n", chunk.SymbolName, chunk.TokenCount)
//	}
func (sc *SemanticChunker) ChunkFile(filePath string, source []byte) ([]CodeChunk, error) {
	// Step 1: Parse the file
	result, err := sc.parser.ParseFile(filePath, source)
	if err != nil {
		return nil, err
	}

	// Step 2: Enrich symbols into chunks
	chunks := sc.enricher.EnrichParseResult(result)

	// Step 3: Merge small chunks
	if sc.config.MergeSmallChunks {
		chunks = sc.enricher.MergeSmallChunks(chunks)
	}

	// Step 4: Split large chunks
	chunks = sc.enricher.SplitLargeChunks(chunks)

	// Step 5: Fill gaps - create chunks for uncovered parts of the file
	chunks = sc.fillFileGaps(chunks, source, filePath, result.Language)

	// Step 6: Merge small gap-fillers with adjacent chunks
	if sc.config.MergeSmallChunks {
		chunks = sc.enricher.MergeSmallChunks(chunks)
	}

	// Step 7: Split large chunks (including merged gap-fillers)
	chunks = sc.enricher.SplitLargeChunks(chunks)

	return chunks, nil
}

// ChunkFileWithResult processes a file and also returns the parse result.
// Useful when you need both the chunks and the raw parsing information.
//
// Parameters:
//   - filePath: Path to the source file
//   - source: The file contents as bytes
//
// Returns:
//   - []CodeChunk: Enriched chunks
//   - *ParseResult: The raw parse result with symbols, imports, errors
//   - error: Any error encountered
func (sc *SemanticChunker) ChunkFileWithResult(filePath string, source []byte) ([]CodeChunk, *ParseResult, error) {
	// Parse the file
	result, err := sc.parser.ParseFile(filePath, source)
	if err != nil {
		return nil, nil, err
	}

	// Enrich and process
	chunks := sc.enricher.EnrichParseResult(result)
	if sc.config.MergeSmallChunks {
		chunks = sc.enricher.MergeSmallChunks(chunks)
	}
	chunks = sc.enricher.SplitLargeChunks(chunks)

	return chunks, result, nil
}

// IsSupported checks if a file path is supported by the chunker.
// Returns true if the file extension has a registered parser.
//
// Parameters:
//   - filePath: Path to check
//
// Returns true if the file can be parsed and chunked.
func (sc *SemanticChunker) IsSupported(filePath string) bool {
	return sc.parser.IsSupported(filePath)
}

// GetSupportedExtensions returns all file extensions supported by this chunker.
//
// Returns a slice of extensions (e.g., [".go", ".py", ".ts", ".js", ...])
func (sc *SemanticChunker) GetSupportedExtensions() []string {
	return sc.parser.GetSupportedExtensions()
}

// GetConfig returns the current chunking configuration.
func (sc *SemanticChunker) GetConfig() ChunkConfig {
	return sc.config
}

// UpdateConfig updates the chunking configuration.
// Note: This will recreate internal components with the new config.
//
// Parameters:
//   - config: New configuration to apply
func (sc *SemanticChunker) UpdateConfig(config ChunkConfig) {
	sc.config = config
	sc.parser = NewParser(config)
	sc.enricher = NewChunkEnricher(config)
}

// fillFileGaps finds uncovered regions of the file and creates chunks for them.
// This ensures every line of the file is included in at least one chunk.
//
// Parameters:
//   - chunks: Existing chunks (possibly with gaps)
//   - source: The file source code
//   - filePath: Path to the file
//   - language: Detected language
//
// Returns chunks with gaps filled.
func (sc *SemanticChunker) fillFileGaps(chunks []CodeChunk, source []byte, filePath, language string) []CodeChunk {
	if len(chunks) == 0 {
		// No chunks at all, create one chunk for the entire file
		return sc.createFallbackChunks(source, filePath, language)
	}

	lines := splitLines(source)

	// Sort chunks by start line
	sortedChunks := make([]CodeChunk, len(chunks))
	copy(sortedChunks, chunks)

	// Simple bubble sort by StartLine (small number of chunks, simple is fine)
	for i := 0; i < len(sortedChunks)-1; i++ {
		for j := 0; j < len(sortedChunks)-i-1; j++ {
			if sortedChunks[j].StartLine > sortedChunks[j+1].StartLine {
				sortedChunks[j], sortedChunks[j+1] = sortedChunks[j+1], sortedChunks[j]
			}
		}
	}

	totalLines := len(lines)
	result := make([]CodeChunk, 0, len(sortedChunks)+5)

	// Check for gap at the beginning
	if sortedChunks[0].StartLine > 1 {
		start := uint32(1)
		end := sortedChunks[0].StartLine - 1
		if !sc.prependCommentGap(lines, &sortedChunks[0], start, end) {
			result = sc.appendGapOrSplit(result, sc.createGapOrSplit(lines, filePath, language, &result, start, end))
		}
	}

	// Add first chunk and check for gaps between chunks
	result = append(result, sortedChunks[0])

	for i := 1; i < len(sortedChunks); i++ {
		prevEnd := sortedChunks[i-1].EndLine
		currentStart := sortedChunks[i].StartLine

		// If there's a gap between chunks, fill it
		if currentStart > prevEnd+1 {
			start := prevEnd + 1
			end := currentStart - 1
			if !sc.prependCommentGap(lines, &sortedChunks[i], start, end) &&
				!sc.appendCommentGap(lines, &result[len(result)-1], start, end) {
				result = sc.appendGapOrSplit(result, sc.createGapOrSplit(lines, filePath, language, &result, start, end))
			}
		}

		result = append(result, sortedChunks[i])
	}

	// Check for gap at the end
	lastChunk := result[len(result)-1]
	if lastChunk.EndLine < uint32(totalLines) {
		start := lastChunk.EndLine + 1
		end := uint32(totalLines)
		if !sc.appendCommentGap(lines, &result[len(result)-1], start, end) {
			result = sc.appendGapOrSplit(result, sc.createGapOrSplit(lines, filePath, language, &result, start, end))
		}
	}

	return result
}

// createGapChunk creates a chunk for a gap in coverage.
func (sc *SemanticChunker) createGapChunk(lines []string, filePath, language string, startLine, endLine uint32) CodeChunk {
	if startLine < 1 || startLine > endLine || int(endLine) > len(lines) {
		return CodeChunk{}
	}

	// Extract the lines for this gap
	gapLines := lines[startLine-1 : endLine]
	if !hasMeaningfulContent(gapLines) {
		return CodeChunk{}
	}
	sourceCode := joinLines(gapLines)

	// Try to find a meaningful name from the content
	symbolName := extractMeaningfulName(gapLines, startLine, endLine)

	chunk := CodeChunk{
		FilePath:    filePath,
		Language:    language,
		SymbolName:  symbolName,
		SymbolKind:  "text",
		StartLine:   startLine,
		EndLine:     endLine,
		StartByte:   0,
		EndByte:     0,
		SourceCode:  sourceCode,
		IsCollapsed: false,
	}
	if sc.enricher != nil {
		sc.enricher.refreshChunkContent(&chunk)
	} else {
		chunk.Content = sourceCode
		chunk.TokenCount = estimateTokenCount(sourceCode)
	}
	return chunk
}

func (sc *SemanticChunker) createGapOrSplit(lines []string, filePath, language string, chunks *[]CodeChunk, startLine, endLine uint32) CodeChunk {
	gapChunk := sc.createGapChunk(lines, filePath, language, startLine, endLine)
	if gapChunk.TokenCount == 0 {
		return gapChunk
	}
	if len(*chunks) == 0 {
		return gapChunk
	}

	prev := &(*chunks)[len(*chunks)-1]
	preferred := sc.enricher.preferredChunkSize()
	if preferred <= 0 {
		preferred = sc.config.MaxChunkSize
	}
	if prev.TokenCount > preferred && prev.EndLine >= startLine-1 {
		split := sc.enricher.splitChunk(*prev, preferred)
		if len(split) > 1 {
			(*chunks)[len(*chunks)-1] = split[0]
			for i := 1; i < len(split); i++ {
				*chunks = append(*chunks, split[i])
			}
			return gapChunk
		}
	}

	if prev.TokenCount < preferred {
		prev.SourceCode = mergeGapAfterChunk(prev.SourceCode, gapChunk.SourceCode)
		prev.EndLine = gapChunk.EndLine
		prev.EndByte = gapChunk.EndByte
		sc.enricher.refreshChunkContent(prev)
		return CodeChunk{}
	}

	return gapChunk
}

func (sc *SemanticChunker) appendGapOrSplit(existing []CodeChunk, gap CodeChunk) []CodeChunk {
	if gap.TokenCount == 0 {
		return existing
	}
	return append(existing, gap)
}

func (sc *SemanticChunker) prependCommentGap(lines []string, chunk *CodeChunk, startLine, endLine uint32) bool {
	if chunk == nil || startLine > endLine {
		return false
	}
	gapLines := extractLineRange(lines, startLine, endLine)
	if len(gapLines) == 0 || !isCommentOnlyBlock(gapLines) {
		return false
	}

	gapText := joinLines(gapLines)
	chunk.SourceCode = mergeGapBeforeChunk(gapText, chunk.SourceCode)
	chunk.StartLine = startLine
	chunk.StartByte = calculateByteOffsetFromLines(lines, startLine)
	if sc.enricher != nil {
		sc.enricher.refreshChunkContent(chunk)
	}
	return true
}

func (sc *SemanticChunker) appendCommentGap(lines []string, chunk *CodeChunk, startLine, endLine uint32) bool {
	if chunk == nil || startLine > endLine {
		return false
	}
	gapLines := extractLineRange(lines, startLine, endLine)
	if len(gapLines) == 0 || !isCommentOnlyBlock(gapLines) {
		return false
	}

	gapText := joinLines(gapLines)
	chunk.SourceCode = mergeGapAfterChunk(chunk.SourceCode, gapText)
	chunk.EndLine = endLine
	chunk.EndByte = 0
	if sc.enricher != nil {
		sc.enricher.refreshChunkContent(chunk)
	}
	return true
}

// createFallbackChunks creates simple chunks when no symbols were found.
func (sc *SemanticChunker) createFallbackChunks(source []byte, filePath, language string) []CodeChunk {
	lines := splitLines(source)
	totalLines := uint32(len(lines))
	sourceCode := string(source)
	chunk := CodeChunk{
		FilePath:    filePath,
		Language:    language,
		SymbolName:  "file-content",
		SymbolKind:  "file",
		StartLine:   1,
		EndLine:     totalLines,
		StartByte:   0,
		EndByte:     uint32(len(source)),
		SourceCode:  sourceCode,
		IsCollapsed: false,
	}
	if sc.enricher != nil {
		sc.enricher.refreshChunkContent(&chunk)
	} else {
		chunk.Content = sourceCode
		chunk.TokenCount = estimateTokenCount(sourceCode)
	}

	return []CodeChunk{chunk}
}

func mergeGapBeforeChunk(gapText, original string) string {
	if strings.TrimSpace(gapText) == "" {
		return original
	}
	if strings.TrimSpace(original) == "" {
		return gapText
	}
	return strings.TrimRight(gapText, "\n") + "\n\n" + strings.TrimLeft(original, "\n")
}

func mergeGapAfterChunk(original, gapText string) string {
	if strings.TrimSpace(gapText) == "" {
		return original
	}
	if strings.TrimSpace(original) == "" {
		return gapText
	}
	return strings.TrimRight(original, "\n") + "\n\n" + strings.TrimLeft(gapText, "\n")
}

func extractLineRange(lines []string, startLine, endLine uint32) []string {
	if startLine < 1 || startLine > endLine {
		return nil
	}
	max := uint32(len(lines))
	if startLine-1 >= max {
		return nil
	}
	if endLine > max {
		endLine = max
	}
	return lines[startLine-1 : endLine]
}

func calculateByteOffsetFromLines(lines []string, line uint32) uint32 {
	if line <= 1 {
		return 0
	}
	max := uint32(len(lines))
	limit := line - 1
	if limit > max {
		limit = max
	}

	var offset uint32
	for i := uint32(0); i < limit && i < max; i++ {
		offset += uint32(len(lines[i]))
		if i < max-1 {
			offset++
		}
	}

	return offset
}

// Helper functions

func splitLines(source []byte) []string {
	return strings.Split(string(source), "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func hasMeaningfulContent(lines []string) bool {
	return !isCommentOnlyBlock(lines)
}

// extractMeaningfulName tries to extract a meaningful name from the content.
// Looks for headings, function names, or other identifiable markers.
func extractMeaningfulName(lines []string, startLine, endLine uint32) string {
	// Look for markdown headings, code patterns, etc.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for markdown heading
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.TrimLeft(trimmed, "# ")
			if heading != "" {
				return heading
			}
		}

		// Check for code block marker
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimPrefix(trimmed, "```")
			if lang != "" {
				return fmt.Sprintf("code:%s", lang)
			}
			return "code"
		}

		// Check for HTML heading
		if strings.HasPrefix(trimmed, "<h") && len(trimmed) > 3 {
			return "html-heading"
		}
	}

	// No meaningful name found, use line range
	if startLine == endLine {
		return fmt.Sprintf("L%d", startLine)
	}
	return fmt.Sprintf("L%d-%d", startLine, endLine)
}

func isCommentOnlyBlock(lines []string) bool {
	inBlock := false
	inHTML := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if inBlock {
			if strings.Contains(trimmed, "*/") {
				after := strings.TrimSpace(trimmed[strings.Index(trimmed, "*/")+2:])
				if after != "" {
					return false
				}
				inBlock = false
			}
			continue
		}

		if inHTML {
			if strings.Contains(trimmed, "-->") {
				after := strings.TrimSpace(trimmed[strings.Index(trimmed, "-->")+3:])
				if after != "" {
					return false
				}
				inHTML = false
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "//"),
			strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "/*"):
			if strings.Contains(trimmed, "*/") {
				after := strings.TrimSpace(trimmed[strings.Index(trimmed, "*/")+2:])
				if after != "" {
					return false
				}
			} else {
				inBlock = true
			}
			continue
		case strings.HasPrefix(trimmed, "<!--"):
			if strings.Contains(trimmed, "-->") {
				after := strings.TrimSpace(trimmed[strings.Index(trimmed, "-->")+3:])
				if after != "" {
					return false
				}
			} else {
				inHTML = true
			}
			continue
		case strings.HasPrefix(trimmed, "-->"):
			after := strings.TrimSpace(trimmed[3:])
			if after != "" {
				return false
			}
			inHTML = false
			continue
		case strings.HasPrefix(trimmed, "*"):
			// Treat leading * inside block comments as comment content
			continue
		}

		return false
	}

	return true
}
