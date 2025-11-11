/*
  File: types.go
  Purpose: Common types and structures for code parsing and chunking.
  Author: CodeTextor project
  Notes: This file defines the core data structures used across all language parsers.
*/

package chunker

import sitter "github.com/tree-sitter/go-tree-sitter"

// SymbolKind represents the type of code symbol extracted from the AST.
type SymbolKind string

const (
	// Programming language symbols
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolClass     SymbolKind = "class"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolImport    SymbolKind = "import"
	SymbolComment   SymbolKind = "comment"
	SymbolModule    SymbolKind = "module"
	SymbolNamespace SymbolKind = "namespace"
	SymbolEnum      SymbolKind = "enum"
	SymbolTypeAlias SymbolKind = "type_alias"

	// HTML/XML symbols
	SymbolElement SymbolKind = "element"
	SymbolScript  SymbolKind = "script"
	SymbolStyle   SymbolKind = "style"

	// CSS symbols
	SymbolCSSRule      SymbolKind = "rule"
	SymbolCSSMedia     SymbolKind = "media"
	SymbolCSSKeyframes SymbolKind = "keyframes"

	// Markdown symbols
	SymbolMarkdownHeading SymbolKind = "heading"
	SymbolMarkdownCode    SymbolKind = "code_block"
	SymbolMarkdownLink    SymbolKind = "link"

	// SQL symbols
	SymbolSQLStatement SymbolKind = "sql_statement"
)

// Symbol represents a single code symbol extracted from the AST.
// It contains the symbol's name, kind, location, and source code.
type Symbol struct {
	Name       string     `json:"name"`                 // Symbol name (e.g., function name, class name)
	Kind       SymbolKind `json:"kind"`                 // Symbol type (function, class, etc.)
	StartLine  uint32     `json:"start_line"`           // Starting line number (1-indexed)
	EndLine    uint32     `json:"end_line"`             // Ending line number (1-indexed)
	StartByte  uint32     `json:"start_byte"`           // Starting byte offset
	EndByte    uint32     `json:"end_byte"`             // Ending byte offset
	Source     string     `json:"source"`               // Full source code of the symbol
	Signature  string     `json:"signature,omitempty"`  // Function/method signature (if applicable)
	Parent     string     `json:"parent,omitempty"`     // Parent symbol name (e.g., class name for methods)
	Visibility string     `json:"visibility,omitempty"` // public, private, protected, etc.
	DocString  string     `json:"doc_string,omitempty"` // Associated documentation/comment
}

// ParseResult represents the output of parsing a single file.
// It contains all extracted symbols and any errors encountered.
type ParseResult struct {
	FilePath string            `json:"file_path"` // Path to the parsed file
	Language string            `json:"language"`  // Detected language (go, python, typescript, etc.)
	Symbols  []Symbol          `json:"symbols"`   // All extracted symbols
	Imports  []string          `json:"imports"`   // List of imported modules/packages
	Errors   []ParseError      `json:"errors"`    // Any parsing errors encountered
	Metadata map[string]string `json:"metadata"`  // Additional metadata (encoding, package name, etc.)
}

// ParseError represents an error encountered during parsing.
type ParseError struct {
	Line    uint32 `json:"line"`    // Line number where error occurred
	Column  uint32 `json:"column"`  // Column number where error occurred
	Message string `json:"message"` // Error description
}

// LanguageParser is the interface that all language-specific parsers must implement.
// Each language parser (Go, Python, TypeScript, etc.) provides its own implementation.
type LanguageParser interface {
	// GetLanguage returns the tree-sitter Language for this parser.
	GetLanguage() *sitter.Language

	// ExtractSymbols extracts all symbols from the given AST tree.
	// Parameters:
	//   - tree: The parsed tree-sitter AST
	//   - source: The original source code as byte slice
	// Returns a slice of Symbol structs representing all extracted code symbols.
	ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error)

	// ExtractImports extracts all import statements from the AST.
	// Parameters:
	//   - tree: The parsed tree-sitter AST
	//   - source: The original source code as byte slice
	// Returns a slice of import strings (module/package names).
	ExtractImports(tree *sitter.Tree, source []byte) ([]string, error)

	// GetFileExtensions returns the file extensions this parser handles.
	// For example: [".go"] for Go, [".py"] for Python, [".ts", ".tsx"] for TypeScript.
	GetFileExtensions() []string
}

// ChunkConfig defines configuration for chunking behavior.
type ChunkConfig struct {
	MaxChunkSize      int  // Maximum size in tokens for a single chunk (default: 800)
	MinChunkSize      int  // Minimum size in tokens for a single chunk (default: 100)
	CollapseThreshold int  // Threshold for collapsing long function bodies (default: 500)
	MergeSmallChunks  bool // Whether to merge small adjacent chunks (default: true)
	IncludeComments   bool // Whether to attach leading comments to symbols (default: true)
}

// DefaultChunkConfig returns the default chunking configuration.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		MaxChunkSize:      800,
		MinChunkSize:      100,
		CollapseThreshold: 500,
		MergeSmallChunks:  true,
		IncludeComments:   true,
	}
}
