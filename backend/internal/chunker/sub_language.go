package chunker

import (
	"strings"
	"sync"

	"github.com/go-enry/go-enry/v2"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// SubLanguageManager handles statistical detection (Markov via enry) and precise extraction
// (SetIncludedRanges) of embedded code across all supported languages.
type SubLanguageManager struct {
	// Maps Enry language names (e.g., "HTML", "SQL", "JavaScript") to our parsers.
	parsers map[string]LanguageParser

	// parserPools caches sitter.Parser instances per language to avoid CGO allocation overhead.
	parserPools map[string]*sync.Pool
}

// NewSubLanguageManager creates a manager and establishes the Enry -> LanguageParser mapping.
func NewSubLanguageManager(extParsers map[string]LanguageParser) *SubLanguageManager {
	langParsers := make(map[string]LanguageParser)
	pools := make(map[string]*sync.Pool)

	for ext, parser := range extParsers {
		target := ""
		switch strings.ToLower(ext) {
		case ".js":
			target = "JavaScript"
		case ".jsx":
			target = "JSX"
		case ".ts":
			target = "TypeScript"
		case ".tsx":
			target = "TSX"
		case ".html", ".htm":
			target = "HTML"
		case ".css":
			target = "CSS"
		case ".json":
			target = "JSON"
		case ".sql":
			target = "SQL"
		case ".go":
			target = "Go"
		case ".py":
			target = "Python"
		case ".vue":
			target = "Vue"
		case ".php":
			target = "PHP"
		case ".md":
			target = "Markdown"
		}

		if target != "" {
			langParsers[target] = parser
			// Initialize a pool for this language if not already done
			if _, ok := pools[target]; !ok {
				lang := parser.GetLanguage()
				if !isNil(lang) {
					pools[target] = &sync.Pool{
						New: func() interface{} {
							p := sitter.NewParser()
							p.SetLanguage(lang)
							return p
						},
					}
				}
			}
		}
	}

	return &SubLanguageManager{
		parsers:     langParsers,
		parserPools: pools,
	}
}

// GetParser returns a registered parser for the given file extension (e.g., ".js", ".ts").
// Returns nil if no parser is registered for that extension.
func (m *SubLanguageManager) GetParser(ext string) LanguageParser {
	switch strings.ToLower(ext) {
	case ".js":
		return m.parsers["JavaScript"]
	case ".jsx":
		return m.parsers["JSX"]
	case ".ts":
		return m.parsers["TypeScript"]
	case ".tsx":
		return m.parsers["TSX"]
	case ".html", ".htm":
		return m.parsers["HTML"]
	case ".css":
		return m.parsers["CSS"]
	case ".json":
		return m.parsers["JSON"]
	case ".sql":
		return m.parsers["SQL"]
	case ".go":
		return m.parsers["Go"]
	case ".py":
		return m.parsers["Python"]
	case ".vue":
		return m.parsers["Vue"]
	case ".php":
		return m.parsers["PHP"]
	case ".md":
		return m.parsers["Markdown"]
	}
	return nil
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
	return m.ExtractKnownLanguage(lang, fullSource, startByte, endByte, startPoint, endPoint, parentName, 0)
}

// ExtractKnownLanguage skips statistical detection and forcefully applies a specific language parser.
// Useful when the parent context guarantees the language (e.g., HTML <script> tag -> "JavaScript").
func (m *SubLanguageManager) ExtractKnownLanguage(
	langName string,
	fullSource []byte,
	startByte, endByte uint32,
	startPoint, endPoint sitter.Point,
	parentName string,
	depth int,
) []Symbol {
	rng := sitter.Range{
		StartByte:  uint(startByte),
		EndByte:    uint(endByte),
		StartPoint: sitter.Point{Row: uint(startPoint.Row), Column: uint(startPoint.Column)},
		EndPoint:   sitter.Point{Row: uint(endPoint.Row), Column: uint(endPoint.Column)},
	}
	symbols, _, _ := m.BatchExtractAll(langName, fullSource, []sitter.Range{rng}, []string{parentName}, nil, depth)
	return symbols
}

// BatchExtractAll performs a single parsing pass for a specific sub-language
// and extracts symbols, imports, and usages in one go.
// Support 'detect' as langName to perform dynamic language identification per fragment.
func (m *SubLanguageManager) BatchExtractAll(
	langName string,
	fullSource []byte,
	ranges []sitter.Range,
	parentNames []string,
	mainSymbols []Symbol,
	depth int,
) ([]Symbol, []string, []ParserSymbolUsage) {
	if m == nil || len(ranges) == 0 {
		return nil, nil, nil
	}

	// Handle "detect" by grouping ranges by their identified language
	if strings.ToLower(langName) == "detect" {
		return m.batchExtractWithDetection(fullSource, ranges, parentNames, mainSymbols, depth)
	}

	targetLang := ""
	lowerLang := strings.ToLower(langName)
	for name := range m.parsers {
		if strings.ToLower(name) == lowerLang {
			targetLang = name
			break
		}
	}

	if targetLang == "" {
		return nil, nil, nil
	}

	parser := m.parsers[targetLang]

	// 0. Use stateless parsing if supported (avoiding locks and potential deadlocks)
	if aware, ok := parser.(SubLanguageAware); ok {
		res, err := aware.ParseWithContext(fullSource, ranges, depth)
		if err != nil {
			return nil, nil, nil
		}
		return m.processParseResult(res, ranges, parentNames)
	}

	// 1. Fallback for legacy parsers (using lock to ensure atomicity of state + parse)

	// 1. Perform full parse (this enables recursion if it's a QueryParser)
	res, err := parser.Parse(fullSource)
	if err != nil {
		return nil, nil, nil
	}

	// 2. Process results
	return m.processParseResult(res, ranges, parentNames)
}

func (m *SubLanguageManager) processParseResult(res ParseResult, ranges []sitter.Range, parentNames []string) ([]Symbol, []string, []ParserSymbolUsage) {
	symbols := res.Symbols
	// Attach parent relationships correctly for each fragment in the batch
	for i := range symbols {
		// Find which range this symbol belongs to
		for idx, r := range ranges {
			if uint32(symbols[i].StartByte) >= uint32(r.StartByte) && uint32(symbols[i].EndByte) <= uint32(r.EndByte) {
				if idx < len(parentNames) {
					pn := parentNames[idx]
					if symbols[i].Parent == "" {
						symbols[i].Parent = pn
					}
				}
				break
			}
		}
	}

	return symbols, res.Imports, res.Usages
}

func (m *SubLanguageManager) batchExtractWithDetection(
	fullSource []byte,
	ranges []sitter.Range,
	parentNames []string,
	mainSymbols []Symbol,
	depth int,
) ([]Symbol, []string, []ParserSymbolUsage) {
	// Group ranges by identified language
	type group struct {
		ranges      []sitter.Range
		parentNames []string
	}
	groups := make(map[string]*group)

	for i, r := range ranges {
		content := fullSource[r.StartByte:r.EndByte]
		lang := enry.GetLanguage("", content)
		
		// Fallback for detection if enry fails
		if lang == "" {
			strContent := strings.TrimSpace(string(content))
			lowerContent := strings.ToLower(strContent)
			
			// SQL detection
			isSQL := (strings.Contains(lowerContent, "select ") && strings.Contains(lowerContent, " from ")) || 
				strings.Contains(lowerContent, "insert into ") || 
				strings.Contains(lowerContent, "create table ") ||
				strings.Contains(lowerContent, "update ") ||
				strings.Contains(lowerContent, "delete from ") ||
				strings.Contains(lowerContent, "alter table ") ||
				strings.Contains(lowerContent, "drop table ")
			
			if isSQL {
				lang = "SQL"
			} else if strings.Contains(lowerContent, "<div>") || strings.Contains(lowerContent, "<h1>") || 
				strings.Contains(lowerContent, "<p>") || strings.Contains(lowerContent, "</span>") ||
				strings.Contains(lowerContent, "</div>") || strings.Contains(lowerContent, "<html>") ||
				strings.Contains(lowerContent, "<head>") || strings.Contains(lowerContent, "<body>") ||
				strings.Contains(lowerContent, "<style") || strings.Contains(lowerContent, "<script") ||
				strings.Contains(lowerContent, "<section") || strings.Contains(lowerContent, "<footer") ||
				strings.Contains(lowerContent, "<header") {
				lang = "HTML"
			} else if (strings.HasPrefix(strContent, "{") && strings.HasSuffix(strContent, "}")) || 
				(strings.HasPrefix(strContent, "[") && strings.HasSuffix(strContent, "]")) {
				// Basic heuristic for JSON
				if strings.Contains(strContent, ":") {
					lang = "JSON"
				}
			} else if strings.Contains(lowerContent, "package ") || strings.Contains(lowerContent, "func ") || 
				strings.Contains(lowerContent, "import (") || strings.Contains(lowerContent, "chan ") ||
				strings.Contains(lowerContent, "select {") {
				// Basic heuristic for Go
				lang = "Go"
			} else if strings.Contains(lowerContent, "function") && (strings.Contains(lowerContent, "=>") || strings.Contains(lowerContent, "return ")) {
				// Basic heuristic for JS
				lang = "JavaScript"
			}
		}

		if lang == "" {
			continue 
		}

		// Normalize lang
		g, ok := groups[lang]
		if !ok {
			g = &group{}
			groups[lang] = g
		}
		g.ranges = append(g.ranges, r)
		if i < len(parentNames) {
			g.parentNames = append(g.parentNames, parentNames[i])
		}
	}

	var allSymbols []Symbol
	var allImports []string
	var allUsages []ParserSymbolUsage

	for lang, g := range groups {
		syms, imps, usgs := m.BatchExtractAll(lang, fullSource, g.ranges, g.parentNames, mainSymbols, depth)
		allSymbols = append(allSymbols, syms...)
		allImports = append(allImports, imps...)
		allUsages = append(allUsages, usgs...)
	}

	return allSymbols, allImports, allUsages
}

// BatchExtractKnownLanguage extracts symbols from multiple ranges in a single parse call.
// Deprecated: use BatchExtractAll for better performance.
func (m *SubLanguageManager) BatchExtractKnownLanguage(
	langName string,
	fullSource []byte,
	ranges []sitter.Range,
	parentNames []string,
) []Symbol {
	symbols, _, _ := m.BatchExtractAll(langName, fullSource, ranges, parentNames, nil, 0)
	return symbols
}


func (m *SubLanguageManager) ExtractImports(
	langName string,
	fullSource []byte,
	startByte, endByte uint32,
	startPoint, endPoint sitter.Point,
) []string {
	rng := sitter.Range{
		StartByte:  uint(startByte),
		EndByte:    uint(endByte),
		StartPoint: sitter.Point{Row: uint(startPoint.Row), Column: uint(startPoint.Column)},
		EndPoint:   sitter.Point{Row: uint(endPoint.Row), Column: uint(endPoint.Column)},
	}
	return m.BatchExtractImports(langName, fullSource, []sitter.Range{rng})
}

// BatchExtractImports extracts imports from multiple ranges in a single parse call.
func (m *SubLanguageManager) BatchExtractImports(
	langName string,
	fullSource []byte,
	ranges []sitter.Range,
) []string {
	if len(ranges) == 0 {
		return nil
	}

	targetLang := ""
	lowerLang := strings.ToLower(langName)
	for name := range m.parsers {
		if strings.ToLower(name) == lowerLang {
			targetLang = name
			break
		}
	}

	if targetLang == "" {
		return nil
	}
	parser := m.parsers[targetLang]

	tsParser := sitter.NewParser()
	defer tsParser.Close()

	if err := tsParser.SetLanguage(parser.GetLanguage()); err != nil {
		return nil
	}

	if err := tsParser.SetIncludedRanges(ranges); err != nil {
		return nil
	}

	tree := tsParser.Parse(fullSource, nil)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	imports, _ := parser.ExtractImports(tree, fullSource)
	return imports
}

func (m *SubLanguageManager) ExtractUsages(
	langName string,
	fullSource []byte,
	startByte, endByte uint32,
	startPoint, endPoint sitter.Point,
	symbols []Symbol,
) []ParserSymbolUsage {
	rng := sitter.Range{
		StartByte:  uint(startByte),
		EndByte:    uint(endByte),
		StartPoint: sitter.Point{Row: uint(startPoint.Row), Column: uint(startPoint.Column)},
		EndPoint:   sitter.Point{Row: uint(endPoint.Row), Column: uint(endPoint.Column)},
	}
	return m.BatchExtractUsages(langName, fullSource, []sitter.Range{rng}, symbols)
}

// BatchExtractUsages extracts usages from multiple ranges in a single parse call.
func (m *SubLanguageManager) BatchExtractUsages(
	langName string,
	fullSource []byte,
	ranges []sitter.Range,
	symbols []Symbol,
) []ParserSymbolUsage {
	if len(ranges) == 0 {
		return nil
	}

	targetLang := ""
	lowerLang := strings.ToLower(langName)
	for name := range m.parsers {
		if strings.ToLower(name) == lowerLang {
			targetLang = name
			break
		}
	}

	if targetLang == "" {
		return nil
	}
	parser := m.parsers[targetLang]

	tsParser := sitter.NewParser()
	defer tsParser.Close()

	if err := tsParser.SetLanguage(parser.GetLanguage()); err != nil {
		return nil
	}

	if err := tsParser.SetIncludedRanges(ranges); err != nil {
		return nil
	}

	tree := tsParser.Parse(fullSource, nil)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	if qp, ok := parser.(*QueryParser); ok {
		// For QueryParsers, we must respect their internal recursive logic
		// This is a bit of a fallback since usages usually need mainSymbols for better context
		res, _ := qp.Parse(fullSource)
		return res.Usages
	}

	return parser.ExtractUsages(tree, fullSource, symbols)
}
