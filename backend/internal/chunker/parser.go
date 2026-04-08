/*
  File: parser.go
  Purpose: Main parser interface and factory for creating language-specific parsers.
  Author: CodeTextor project
  Notes: This file coordinates parsing across different languages using tree-sitter.
*/

package chunker

import (
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

//go:embed parsers/default/*.toml
var defaultParserConfigs embed.FS

// Parser is the main entry point for parsing source code files.
// It automatically detects the language and uses the appropriate parser.
type Parser struct {
	parsers      map[string]LanguageParser // Map of file extension to parser
	queryParsers map[string]*QueryParser   // New query-based parsers
	config       ChunkConfig               // Chunking configuration
	errorQueries sync.Map                  // Cache for pre-compiled error queries (map[*sitter.Language]*sitter.Query)
}

// NewParser creates a new Parser instance with all supported language parsers.
// It initializes parsers for Go, Python, TypeScript, JavaScript, and other supported languages.
func NewParser(config ChunkConfig) *Parser {
	p := &Parser{
		parsers: make(map[string]LanguageParser),
		config:  config,
	}

	// All parsers have been migrated to the Query-Based TOML configuration system.
	// They are automatically loaded and registered in loadQueryParsers().

	// Initialize and register the Query-Based Parser Engine
	p.queryParsers = make(map[string]*QueryParser)
	p.loadQueryParsers()

	// TODO: Add more parsers as they are implemented
	// p.registerParser(&RustParser{})
	// p.registerParser(&JavaParser{})

	// Initialize the sub-language manager with the registered parsers
	subLangManager := NewSubLanguageManager(p.parsers)

	// Inject the manager into any parser that needs it (avoids circular dependency in Initialization)
	for _, parser := range p.parsers {
		if aware, ok := parser.(SubLanguageAware); ok {
			aware.SetSubLanguageManager(subLangManager)
		}
	}

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
	parseErrors := p.extractParseErrors(rootNode, source, parser.GetLanguage())

	var symbols []Symbol
	var usages []ParserSymbolUsage
	var imports []string
	var metadata map[string]string

	// If it's a QueryParser, use its unified Parse method which handles sub-languages correctly
	if qp, ok := parser.(*QueryParser); ok {
		res, err := qp.Parse(source)
		if err != nil {
			return nil, err
		}
		symbols = res.Symbols
		usages = res.Usages
		imports = res.Imports
		metadata = res.Metadata
	} else {
		// Fallback for any other legacy/custom parsers
		var err error
		symbols, err = parser.ExtractSymbols(tree, source)
		if err != nil {
			return nil, fmt.Errorf("failed to extract symbols: %w", err)
		}
		usages = parser.ExtractUsages(tree, source, symbols)
		imports, err = parser.ExtractImports(tree, source)
		if err != nil {
			imports = []string{}
		}
		metadata = parser.ExtractMetadata(tree, source)
	}

	// Build result
	result := &ParseResult{
		FilePath: filePath,
		Language: p.detectLanguage(ext),
		Symbols:  symbols,
		Usages:   usages,
		Imports:  imports,
		Errors:   parseErrors,
		Metadata: metadata,
	}

	return result, nil
}

// loadQueryParsers loads TOML configurations and initializes QueryParsers.
func (p *Parser) loadQueryParsers() {
	// 1. Load embedded defaults
	defaults, err := LoadDefaultConfigs(defaultParserConfigs)
	if err != nil {
		log.Printf("Warning: failed to load default parser configs: %v", err)
		return
	}

	// 2. Load user overrides from AppData/parsers/
	// (Simplification: using a local path for now, in a real app this would be OS-specific)
	userDir := "parsers" // Should be absolute path in production
	userConfigs, _ := LoadUserConfigs(userDir)

	// 3. Merge
	merged := MergeConfigs(defaults, userConfigs)

	// 4. Initialize and Auto-Register QueryParsers
	for _, config := range merged {
		qp, err := NewQueryParser(config)
		if err != nil {
			log.Printf("Warning: failed to initialize QueryParser for %s: %v", config.Language.Name, err)
			continue
		}
		p.queryParsers[config.Language.Name] = qp

		// Register as primary parser for its extensions
		p.registerParser(qp)
		log.Printf("[Parser] Registered language: %s (extensions: %v)", config.Language.Name, config.Language.Extensions)
	}
}

// extractParseErrors uses a Tree-sitter query to efficiently collect any ERROR nodes.
// These represent syntax errors in the source code.
func (p *Parser) extractParseErrors(node *sitter.Node, source []byte, lang *sitter.Language) []ParseError {
	var errors []ParseError

	// Get or compile error query for this language
	var query *sitter.Query
	if val, ok := p.errorQueries.Load(lang); ok {
		query = val.(*sitter.Query)
	} else {
		var err error
		query, err = sitter.NewQuery(lang, "(ERROR) @error")
		if !isNil(err) {
			log.Printf("Warning: failed to compile error query for language: %v", err)
			return errors
		}
		p.errorQueries.Store(lang, query)
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	// Execute query to find all ERROR nodes in one native pass
	matches := cursor.Matches(query, node, source)
	for {
		match := matches.Next()
		if match == nil {
			break
		}

		for _, capture := range match.Captures {
			startPos := capture.Node.StartPosition()
			errors = append(errors, ParseError{
				Line:    uint32(startPos.Row) + 1,
				Column:  uint32(startPos.Column) + 1,
				Message: fmt.Sprintf("Syntax error: %s", capture.Node.Utf8Text(source)),
			})
		}
	}

	return errors
}

// detectLanguage maps file extension to language name.
func (p *Parser) detectLanguage(ext string) string {
	// Dynamically look up the extension in registered parsers
	if parser, ok := p.parsers[ext]; ok {
		// Each LanguageParser knows its own name from the TOML config
		if qp, ok := parser.(*QueryParser); ok {
			return qp.config.Language.Name
		}
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

var todoRegex = regexp.MustCompile(`(?i)^(?://|/\*|#|--|;|\s|<!--|<!|[\/*#;!])*?(TODO|FIXME|HACK|XXX|NOTE):?\s*`)

// cleanComment removes comment markers (//, /*, */, #, --, <!--, -->) and TODO/FIXME prefixes from a string.
func cleanComment(text string) string {
	text = strings.TrimSpace(text)
	// Remove //, /*, */, #, --, <!--, -->, <! ... >
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimPrefix(text, "/*")
	text = strings.TrimSuffix(text, "*/")
	text = strings.TrimPrefix(text, "#")
	text = strings.TrimPrefix(text, "--")
	text = strings.TrimPrefix(text, "<!--")
	text = strings.TrimSuffix(text, "-->")

	// Remove TODO/FIXME/etc prefixes
	// User requested to keep them for better context:
	// if todoRegex.MatchString(text) {
	// 	text = todoRegex.ReplaceAllString(text, "")
	// }

	return strings.TrimSpace(text)
}
