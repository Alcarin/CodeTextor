package chunker

import (
	"strings"

	"github.com/go-enry/go-enry/v2"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// SubLanguageManager handles statistical detection (Markov via enry) and precise extraction
// (SetIncludedRanges) of embedded code across all supported languages.
type SubLanguageManager struct {
	// Maps Enry language names (e.g., "HTML", "SQL", "JavaScript") to our parsers.
	parsers map[string]LanguageParser
}

// NewSubLanguageManager creates a manager and establishes the Enry -> LanguageParser mapping.
func NewSubLanguageManager(extParsers map[string]LanguageParser) *SubLanguageManager {
	langParsers := make(map[string]LanguageParser)

	for ext, parser := range extParsers {
		switch strings.ToLower(ext) {
		case ".js", ".jsx":
			langParsers["JavaScript"] = parser
		case ".ts", ".tsx":
			langParsers["TypeScript"] = parser
		case ".html", ".htm":
			langParsers["HTML"] = parser
		case ".css":
			langParsers["CSS"] = parser
		case ".json":
			langParsers["JSON"] = parser
		case ".sql":
			langParsers["SQL"] = parser
		case ".go":
			langParsers["Go"] = parser
		case ".py":
			langParsers["Python"] = parser
		case ".vue":
			langParsers["Vue"] = parser
		case ".php":
			langParsers["PHP"] = parser
		case ".md":
			langParsers["Markdown"] = parser
		}
	}

	return &SubLanguageManager{
		parsers: langParsers,
	}
}

// ProcessEmbeddedCode uses enry to statistically identify the language of the content.
// If supported, it configures the appropriate tree-sitter parser using SetIncludedRanges,
// extracting symbols whose physical offsets natively match the parent file.
// Returns nil if the language is unknown or unsupported.
func (m *SubLanguageManager) ProcessEmbeddedCode(
	fullSource []byte,
	content []byte,
	startByte, endByte uint32,
	startPoint, endPoint sitter.Point,
	parentName string,
) []Symbol {
	// 1. Minimum length Check (at least 15 bytes to have statistical significance)
	if len(content) < 15 {
		return nil
	}

	// 2. Statistical detection using Enry
	lang := enry.GetLanguage("", content)
	
	// 2.5 Fallback heuristic detection for common embedded snippets
	if lang == "" {
		strContent := strings.TrimSpace(string(content))
		lowerContent := strings.ToLower(strContent)
		
		if strings.Contains(lowerContent, "<html") || strings.Contains(lowerContent, "<div") || strings.Contains(lowerContent, "<script") || strings.Contains(lowerContent, "<?xml") {
			lang = "HTML"
		} else if strings.HasPrefix(strContent, "<") && strings.HasSuffix(strContent, ">") {
			lang = "HTML"
		} else if strings.Contains(lowerContent, "select ") && strings.Contains(lowerContent, " from ") {
			lang = "SQL"
		} else if strings.Contains(lowerContent, "insert into ") || (strings.Contains(lowerContent, "update ") && strings.Contains(lowerContent, " set ")) {
			lang = "SQL"
		} else if strings.Contains(lowerContent, "delete from ") {
			lang = "SQL"
		}
	}

	if lang == "" {
		return nil
	}

	// 3. Delegate to the known language extractor if we have a parser for it
	return m.ExtractKnownLanguage(lang, fullSource, startByte, endByte, startPoint, endPoint, parentName)
}

// ExtractKnownLanguage skips statistical detection and forcefully applies a specific language parser.
// Useful when the parent context guarantees the language (e.g., HTML <script> tag -> "JavaScript").
func (m *SubLanguageManager) ExtractKnownLanguage(
	langName string,
	fullSource []byte,
	startByte, endByte uint32,
	startPoint, endPoint sitter.Point,
	parentName string,
) []Symbol {
	parser, exists := m.parsers[langName]
	if !exists {
		return nil
	}

	tsParser := sitter.NewParser()
	defer tsParser.Close()

	if err := tsParser.SetLanguage(parser.GetLanguage()); err != nil {
		return nil
	}

	// Inform tree-sitter to only look at the exact bytes of the embedded block,
	// while resolving offsets against the full original file.
	rng := []sitter.Range{
		{
			StartByte:  uint(startByte),
			EndByte:    uint(endByte),
			StartPoint: sitter.Point{Row: uint(startPoint.Row), Column: uint(startPoint.Column)},
			EndPoint:   sitter.Point{Row: uint(endPoint.Row), Column: uint(endPoint.Column)},
		},
	}
	if err := tsParser.SetIncludedRanges(rng); err != nil {
		return nil
	}

	// Parse using the FULL source. The parser will jump directly to the ranges.
	tree := tsParser.Parse(fullSource, nil)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	// Extract Symbols natively (they will have correct absolute offsets)
	symbols, err := parser.ExtractSymbols(tree, fullSource)
	if err != nil || len(symbols) == 0 {
		return nil
	}

	// Attach parent relationship to all extracted symbols and their children
	for i := range symbols {
		if symbols[i].Parent == "" {
			symbols[i].Parent = parentName
		} else if parentName != "" {
			// Prepend parent name for nested symbols to maintain full hierarchy
			symbols[i].Parent = parentName + "." + symbols[i].Parent
		}
	}

	return symbols
}
