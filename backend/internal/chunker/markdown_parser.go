/*
  File: markdown_parser.go
  Purpose: Tree-sitter parser implementation for Markdown.
  Author: CodeTextor project
  Notes: Extracts headings, code blocks, links, and structure from Markdown documents.
*/

package chunker

import (
	"regexp"

	tree_sitter_markdown "github.com/tree-sitter-grammars/tree-sitter-markdown/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// MarkdownParser implements the LanguageParser interface for Markdown documents.
type MarkdownParser struct{}

// GetLanguage returns the tree-sitter Language for Markdown.
func (m *MarkdownParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_markdown.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (m *MarkdownParser) GetFileExtensions() []string {
	return []string{".md", ".markdown"}
}

// ExtractSymbols extracts all symbols from Markdown documents.
// For Markdown, we extract:
//   - Headings (h1-h6) with hierarchical parent-child relationships
//   - Code blocks (with language info)
//   - Links (assigned to their containing heading)
func (m *MarkdownParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols with hierarchical structure
	symbols = m.walkNodeWithHierarchy(rootNode, source)

	// Fix heading EndLine to include all content until next heading of same/higher level
	m.fixHeadingRanges(symbols, source)

	// Extract links and assign them to the correct heading based on line number
	linkSymbols := m.extractLinksFromText(source)
	for i := range linkSymbols {
		// Find the heading that contains this link
		linkSymbols[i].Parent = m.findParentHeadingForLine(symbols, linkSymbols[i].StartLine)
	}
	symbols = append(symbols, linkSymbols...)

	return symbols, nil
}

// headingStackEntry represents a heading in the hierarchy stack.
type headingStackEntry struct {
	name  string
	level int
}

// walkNodeWithHierarchy walks the AST and builds a hierarchical structure of headings.
func (m *MarkdownParser) walkNodeWithHierarchy(rootNode *sitter.Node, source []byte) []Symbol {
	var symbols []Symbol
	var headingStack []headingStackEntry

	// Helper function to get current parent name based on heading level
	getParentForLevel := func(level int) string {
		// Find the nearest heading with a lower level (higher heading number = lower level)
		for i := len(headingStack) - 1; i >= 0; i-- {
			if headingStack[i].level < level {
				return headingStack[i].name
			}
		}
		return ""
	}

	// Helper function to update heading stack when we encounter a new heading
	updateHeadingStack := func(name string, level int) {
		// Remove all headings with equal or higher level (lower heading numbers)
		newStack := []headingStackEntry{}
		for _, entry := range headingStack {
			if entry.level < level {
				newStack = append(newStack, entry)
			}
		}
		// Add the new heading
		newStack = append(newStack, headingStackEntry{name: name, level: level})
		headingStack = newStack
	}

	// Recursively walk nodes
	var walk func(*sitter.Node, string)
	walk = func(node *sitter.Node, currentParent string) {
		nodeType := node.Kind()

		switch nodeType {
		case "atx_heading", "setext_heading":
			// Extract heading
			symbol := m.extractHeading(node, source, "")
			if symbol != nil {
				// Determine heading level (1-6)
				levelStr := symbol.Signature // e.g., "h1", "h2", etc.
				level := 1
				if len(levelStr) >= 2 && levelStr[0] == 'h' {
					level = int(levelStr[1] - '0')
				}

				// Set parent based on heading hierarchy
				symbol.Parent = getParentForLevel(level)

				// Update heading stack
				updateHeadingStack(symbol.Name, level)

				symbols = append(symbols, *symbol)
			}

		case "fenced_code_block", "indented_code_block":
			// Extract code blocks - they belong to the current heading
			symbol := m.extractCodeBlock(node, source)
			if symbol != nil {
				// Code blocks belong to the most recent heading
				if len(headingStack) > 0 {
					symbol.Parent = headingStack[len(headingStack)-1].name
				}
				symbols = append(symbols, *symbol)
			}
		}

		// Recursively process children
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			walk(child, currentParent)
		}
	}

	walk(rootNode, "")
	return symbols
}

// walkNode recursively walks the AST and extracts symbols.
func (m *MarkdownParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "atx_heading", "setext_heading":
		// Extract headings (# Title or underlined titles)
		symbol := m.extractHeading(node, source, parentName)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	case "fenced_code_block", "indented_code_block":
		// Extract code blocks
		symbol := m.extractCodeBlock(node, source)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = m.walkNode(child, source, parentName, symbols)
	}

	return symbols
}

// extractHeading extracts a Markdown heading.
// Example: # Title or ## Subtitle
func (m *MarkdownParser) extractHeading(node *sitter.Node, source []byte, parentName string) *Symbol {
	// Get heading text
	text := node.Utf8Text(source)

	// Determine heading level
	level := m.getHeadingLevel(node, source)
	kind := SymbolMarkdownHeading

	// Clean up the heading text (remove # markers and whitespace)
	cleanText := m.cleanHeadingText(text)
	if cleanText == "" {
		return nil
	}

	return &Symbol{
		Name:       cleanText,
		Kind:       kind,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     text,
		Signature:  level, // Store heading level in signature field
		Visibility: "public",
	}
}

// extractCodeBlock extracts a code block from Markdown.
// Example: ```go ... ``` or indented code
func (m *MarkdownParser) extractCodeBlock(node *sitter.Node, source []byte) *Symbol {
	text := node.Utf8Text(source)

	// Try to extract language info for fenced code blocks
	language := m.extractCodeLanguage(node, source)

	name := "code"
	if language != "" {
		name = "code:" + language
	}

	return &Symbol{
		Name:       name,
		Kind:       SymbolMarkdownCode,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     text,
		Signature:  language,
		Visibility: "public",
	}
}

// extractLinksFromText extracts links from Markdown using regex.
// Example: [text](url)
func (m *MarkdownParser) extractLinksFromText(source []byte) []Symbol {
	var symbols []Symbol

	// Regex to match Markdown links: [text](url)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllSubmatchIndex(source, -1)

	for _, match := range matches {
		if len(match) >= 6 {
			// match[0] = start of full match, match[1] = end of full match
			// match[2] = start of text capture, match[3] = end of text capture
			// match[4] = start of url capture, match[5] = end of url capture

			fullMatch := source[match[0]:match[1]]
			url := string(source[match[4]:match[5]])

			// Calculate line numbers by counting newlines before the match
			lineNum := m.getLineNumber(source, match[0])

			symbols = append(symbols, Symbol{
				Name:       url,
				Kind:       SymbolMarkdownLink,
				StartLine:  lineNum,
				EndLine:    lineNum,
				StartByte:  uint32(match[0]),
				EndByte:    uint32(match[1]),
				Source:     string(fullMatch),
				Visibility: "public",
			})
		}
	}

	return symbols
}

// getLineNumber calculates the line number (1-indexed) for a given byte position.
func (m *MarkdownParser) getLineNumber(source []byte, bytePos int) uint32 {
	line := uint32(1)
	for i := 0; i < bytePos && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

// getHeadingLevel determines the level of a heading (1-6).
func (m *MarkdownParser) getHeadingLevel(node *sitter.Node, source []byte) string {
	// For atx_heading, count the # symbols
	if node.Kind() == "atx_heading" {
		text := node.Utf8Text(source)
		count := 0
		for _, ch := range text {
			if ch == '#' {
				count++
			} else {
				break
			}
		}
		if count >= 1 && count <= 6 {
			return "h" + string('0'+rune(count))
		}
	}
	// For setext_heading, it's either h1 or h2
	if node.Kind() == "setext_heading" {
		// Check if underline uses === (h1) or --- (h2)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "setext_h1_underline" {
				return "h1"
			}
			if child.Kind() == "setext_h2_underline" {
				return "h2"
			}
		}
	}
	return "h1" // default
}

// cleanHeadingText removes # markers and trims whitespace from heading text.
func (m *MarkdownParser) cleanHeadingText(text string) string {
	// Remove leading # symbols and whitespace
	cleaned := text
	for len(cleaned) > 0 && (cleaned[0] == '#' || cleaned[0] == ' ') {
		cleaned = cleaned[1:]
	}
	// Remove trailing # symbols and whitespace
	for len(cleaned) > 0 && (cleaned[len(cleaned)-1] == '#' || cleaned[len(cleaned)-1] == ' ' || cleaned[len(cleaned)-1] == '\n') {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned
}

// extractCodeLanguage extracts the language identifier from a fenced code block.
func (m *MarkdownParser) extractCodeLanguage(node *sitter.Node, source []byte) string {
	// Look for info_string child in fenced_code_block
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "info_string" || child.Kind() == "language" {
			return child.Utf8Text(source)
		}
	}
	return ""
}

// ExtractImports extracts imports from Markdown.
// For Markdown, we can extract links to other documents as "imports".
func (m *MarkdownParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string

	// Extract all links using regex
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllSubmatch(source, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			url := string(match[2])
			// Only include local file references (not http/https URLs)
			if url != "" && !m.isExternalURL(url) {
				imports = append(imports, url)
			}
		}
	}

	return imports, nil
}

// isExternalURL checks if a URL is external (http/https).
func (m *MarkdownParser) isExternalURL(url string) bool {
	return len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://")
}

// findParentHeadingForLine finds the heading that contains the given line number.
// Returns the name of the most recent heading before or at the line, or empty string if none.
func (m *MarkdownParser) findParentHeadingForLine(symbols []Symbol, lineNum uint32) string {
	var lastHeading string
	for _, sym := range symbols {
		// Only consider headings
		if sym.Kind != SymbolMarkdownHeading {
			continue
		}
		// If this heading is before or at the line number, it could be the parent
		if sym.StartLine <= lineNum {
			lastHeading = sym.Name
		} else {
			// Headings are in order, so once we pass the line number, we can stop
			break
		}
	}
	return lastHeading
}

// fixHeadingRanges adjusts the EndLine of headings to include all content until the next heading.
// This allows the outline builder to correctly determine containment relationships.
func (m *MarkdownParser) fixHeadingRanges(symbols []Symbol, source []byte) {
	// Count total lines in document
	totalLines := uint32(1)
	for _, b := range source {
		if b == '\n' {
			totalLines++
		}
	}

	lines := splitLines(source)

	// Process headings in reverse order
	for i := len(symbols) - 1; i >= 0; i-- {
		if symbols[i].Kind != SymbolMarkdownHeading {
			continue
		}

		// Get heading level
		levelStr := symbols[i].Signature // e.g., "h1", "h2", etc.
		currentLevel := 1
		if len(levelStr) >= 2 && levelStr[0] == 'h' {
			currentLevel = int(levelStr[1] - '0')
		}

		// Find the next heading of equal or higher level (lower number)
		nextHeadingLine := totalLines
		for j := i + 1; j < len(symbols); j++ {
			if symbols[j].Kind != SymbolMarkdownHeading {
				continue
			}

			nextLevelStr := symbols[j].Signature
			nextLevel := 1
			if len(nextLevelStr) >= 2 && nextLevelStr[0] == 'h' {
				nextLevel = int(nextLevelStr[1] - '0')
			}

			// If same or higher level (smaller number), this ends the current heading's range
			if nextLevel <= currentLevel {
				nextHeadingLine = symbols[j].StartLine - 1
				break
			}
		}

		// Set EndLine to just before the next heading (or end of document)
		symbols[i].EndLine = nextHeadingLine

		// Expand the heading's source to include the entire section content.
		startIdx := int(symbols[i].StartLine) - 1
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := int(symbols[i].EndLine)
		if endIdx > len(lines) {
			endIdx = len(lines)
		}
		if endIdx <= startIdx {
			endIdx = startIdx + 1
			if endIdx > len(lines) {
				endIdx = len(lines)
			}
		}
		if startIdx < len(lines) && endIdx > startIdx {
			section := joinLines(lines[startIdx:endIdx])
			if section != "" {
				symbols[i].Source = section
			}
		}
	}
}
