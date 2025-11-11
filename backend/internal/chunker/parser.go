/*
  File: parser.go
  Purpose: Main parser interface and factory for creating language-specific parsers.
  Author: CodeTextor project
  Notes: This file coordinates parsing across different languages using tree-sitter.
*/

package chunker

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Parser is the main entry point for parsing source code files.
// It automatically detects the language and uses the appropriate parser.
type Parser struct {
	parsers map[string]LanguageParser // Map of file extension to parser
	config  ChunkConfig               // Chunking configuration
}

// NewParser creates a new Parser instance with all supported language parsers.
// It initializes parsers for Go, Python, TypeScript, JavaScript, and other supported languages.
func NewParser(config ChunkConfig) *Parser {
	p := &Parser{
		parsers: make(map[string]LanguageParser),
		config:  config,
	}

	// Register all language parsers
	// Each parser is responsible for one or more file extensions
	p.registerParser(&GoParser{})
	p.registerParser(&PythonParser{})

	// Register TypeScript parser for .ts and .tsx
	tsParser := &TypeScriptParser{isTypeScript: true}
	p.parsers[".ts"] = tsParser
	p.parsers[".tsx"] = tsParser

	// Register JavaScript parser for .js and .jsx
	jsParser := &TypeScriptParser{isTypeScript: false}
	p.parsers[".js"] = jsParser
	p.parsers[".jsx"] = jsParser

	// Register HTML, CSS, Vue, and Markdown parsers
	p.registerParser(&HTMLParser{})
	p.registerParser(&CSSParser{})
	p.registerParser(&VueParser{})
	p.registerParser(&MarkdownParser{})
	p.registerParser(&SQLParser{})
	p.registerParser(&JSONParser{})

	// TODO: Add more parsers as they are implemented
	// p.registerParser(&RustParser{})
	// p.registerParser(&JavaParser{})

	return p
}

// registerParser adds a language parser to the registry.
// It maps each file extension supported by the parser to the parser instance.
func (p *Parser) registerParser(parser LanguageParser) {
	for _, ext := range parser.GetFileExtensions() {
		p.parsers[ext] = parser
	}
}

// ParseFile parses a source code file and extracts all symbols, imports, and metadata.
// Parameters:
//   - filePath: Path to the source code file
//   - source: The file contents as a byte slice
//
// Returns a ParseResult containing all extracted information, or an error if parsing fails.
func (p *Parser) ParseFile(filePath string, source []byte) (*ParseResult, error) {
	// Detect file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Find appropriate parser
	parser, ok := p.parsers[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Create tree-sitter parser
	tsParser := sitter.NewParser()
	defer tsParser.Close()

	err := tsParser.SetLanguage(parser.GetLanguage())
	if err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	// Parse source code into AST
	tree := tsParser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse file: tree is nil")
	}
	defer tree.Close()

	// Check for syntax errors in the tree
	rootNode := tree.RootNode()
	parseErrors := p.extractParseErrors(rootNode, source)

	// Extract symbols using language-specific parser
	symbols, err := parser.ExtractSymbols(tree, source)
	if err != nil {
		return nil, fmt.Errorf("failed to extract symbols: %w", err)
	}

	// Extract imports
	imports, err := parser.ExtractImports(tree, source)
	if err != nil {
		// Don't fail on import extraction errors, just log them
		imports = []string{}
	}

	// Build result
	result := &ParseResult{
		FilePath: filePath,
		Language: p.detectLanguage(ext),
		Symbols:  symbols,
		Imports:  imports,
		Errors:   parseErrors,
		Metadata: make(map[string]string),
	}

	return result, nil
}

// extractParseErrors walks the AST and collects any ERROR nodes.
// These represent syntax errors in the source code.
func (p *Parser) extractParseErrors(node *sitter.Node, source []byte) []ParseError {
	var errors []ParseError

	// Use tree-sitter's query to find ERROR nodes
	if node.Kind() == "ERROR" {
		startPos := node.StartPosition()
		errors = append(errors, ParseError{
			Line:    uint32(startPos.Row) + 1, // Convert to 1-indexed
			Column:  uint32(startPos.Column) + 1,
			Message: fmt.Sprintf("Syntax error: %s", node.Utf8Text(source)),
		})
	}

	// Recursively check child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		errors = append(errors, p.extractParseErrors(child, source)...)
	}

	return errors
}

// detectLanguage maps file extension to language name.
func (p *Parser) detectLanguage(ext string) string {
	languageMap := map[string]string{
		".go":       "go",
		".py":       "python",
		".ts":       "typescript",
		".tsx":      "typescript",
		".js":       "javascript",
		".jsx":      "javascript",
		".html":     "html",
		".htm":      "html",
		".css":      "css",
		".scss":     "scss",
		".sass":     "sass",
		".vue":      "vue",
		".md":       "markdown",
		".markdown": "markdown",
		".json":     "json",
		".sql":      "sql",
		".rs":       "rust",
		".java":     "java",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	return "unknown"
}

// GetSupportedExtensions returns a list of all file extensions supported by registered parsers.
func (p *Parser) GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(p.parsers))
	for ext := range p.parsers {
		extensions = append(extensions, ext)
	}
	return extensions
}

// IsSupported checks if a file extension is supported by any registered parser.
func (p *Parser) IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	_, ok := p.parsers[ext]
	return ok
}
