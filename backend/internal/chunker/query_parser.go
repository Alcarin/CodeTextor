package chunker

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// QueryParser implements the LanguageParser interface using Tree-sitter queries.
type QueryParser struct {
	config         *LanguageConfig
	language       *sitter.Language
	subLangManager *SubLanguageManager

	// Compiled queries
	symbolsQuery  *sitter.Query
	importsQuery  *sitter.Query
	metadataQuery *sitter.Query
	usagesQuery   *sitter.Query
	extraQueries  map[string]*sitter.Query

	// Regex for TODOs
	todoRegex *regexp.Regexp
}

// NewQueryParser creates a new QueryParser for the given configuration.
func NewQueryParser(config *LanguageConfig) (*QueryParser, error) {
	lang, err := GetGrammar(config.Language.Grammar)
	if err != nil {
		return nil, fmt.Errorf("failed to get grammar: %w", err)
	}

	qp := &QueryParser{
		config:       config,
		language:     lang,
		extraQueries: make(map[string]*sitter.Query),
	}

	// Compile queries
	if config.Queries.Symbols != "" {
		qp.symbolsQuery, err = sitter.NewQuery(lang, config.Queries.Symbols)
		if !isNil(err) {
			return nil, fmt.Errorf("failed to compile symbols query: %w", err)
		}
	}

	if config.Queries.Imports != "" {
		qp.importsQuery, err = sitter.NewQuery(lang, config.Queries.Imports)
		if !isNil(err) {
			return nil, fmt.Errorf("failed to compile imports query: %w", err)
		}
	}

	if config.Queries.Metadata != "" {
		qp.metadataQuery, err = sitter.NewQuery(lang, config.Queries.Metadata)
		if !isNil(err) {
			return nil, fmt.Errorf("failed to compile metadata query: %w", err)
		}
	}

	if config.Queries.Usages != "" {
		qp.usagesQuery, err = sitter.NewQuery(lang, config.Queries.Usages)
		if !isNil(err) {
			return nil, fmt.Errorf("failed to compile usages query: %w", err)
		}
	}

	for name, qSource := range config.Queries.Extra {
		q, err := sitter.NewQuery(lang, qSource)
		if !isNil(err) {
			return nil, fmt.Errorf("failed to compile extra query %s: %w", name, err)
		}
		qp.extraQueries[name] = q
	}

	// Compile TODO regex
	if config.Rules.Todo.Pattern != "" {
		qp.todoRegex, err = regexp.Compile(config.Rules.Todo.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile TODO regex: %w", err)
		}
	}

	return qp, nil
}

// SetSubLanguageManager implements the SubLanguageAware interface.
func (qp *QueryParser) SetSubLanguageManager(manager *SubLanguageManager) {
	qp.subLangManager = manager
}

// GetLanguage returns the tree-sitter Language for this parser.
func (qp *QueryParser) GetLanguage() *sitter.Language {
	return qp.language
}

// GetFileExtensions returns the file extensions handled by this parser.
func (qp *QueryParser) GetFileExtensions() []string {
	return qp.config.Language.Extensions
}

// ExtractSymbols extracts symbols using the configured queries.
func (qp *QueryParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	if qp.symbolsQuery == nil {
		return []Symbol{}, nil
	}

	var symbols []Symbol
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	// Re-implementing using Matches() to group captures properly
	matches := cursor.Matches(qp.symbolsQuery, tree.RootNode(), source)
	captureNames := qp.symbolsQuery.CaptureNames()
	for {
		match := matches.Next()
		if match == nil {
			break
		}

		var sym Symbol
		var kind SymbolKind

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			node := capture.Node

			switch {
			case strings.HasPrefix(captureName, "symbol."):
				kind = SymbolKind(strings.TrimPrefix(captureName, "symbol."))
				sym.StartLine = uint32(node.StartPosition().Row) + 1
				sym.EndLine = uint32(node.EndPosition().Row) + 1
				sym.StartByte = uint32(node.StartByte())
				sym.EndByte = uint32(node.EndByte())
				sym.Source = string(source[node.StartByte():node.EndByte()])
				// Use the main node for docstring
				sym.DocString = qp.extractDocString(&node, source)
			case captureName == "name":
				sym.Name = string(source[node.StartByte():node.EndByte()])
			case captureName == "parent":
				sym.Parent = string(source[node.StartByte():node.EndByte()])
			case captureName == "signature":
				sym.Signature = string(source[node.StartByte():node.EndByte()])
			}
		}

		if kind != "" {
			sym.Kind = kind
			if sym.Name == "" {
				sym.Name = "anonymous"
			}
			sym.Visibility = qp.determineVisibility(sym.Name)
			symbols = append(symbols, sym)
		}
	}

	// 4. Update with sub-language symbols and TODOs
	symbols = append(symbols, qp.extractSubLanguageSymbols(tree.RootNode(), source, "")...)
	symbols = append(symbols, qp.extractTodos(tree.RootNode(), source)...)

	// 5. Sort symbols by start byte to ensure document order (matches legacy parser)
	// For tie-breaking (same StartByte), the longer node (parent) should come first.
	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].StartByte != symbols[j].StartByte {
			return symbols[i].StartByte < symbols[j].StartByte
		}
		return symbols[i].EndByte > symbols[j].EndByte
	})

	return symbols, nil
}

// ExtractUsages extracts symbol usages using queries.
func (qp *QueryParser) ExtractUsages(tree *sitter.Tree, source []byte, symbols []Symbol) []ParserSymbolUsage {
	if qp.usagesQuery == nil {
		return []ParserSymbolUsage{}
	}

	var usages []ParserSymbolUsage
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(qp.usagesQuery, tree.RootNode(), source)
	captureNames := qp.usagesQuery.CaptureNames()

	for {
		match := matches.Next()
		if match == nil {
			break
		}

		var usage ParserSymbolUsage
		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			node := capture.Node

			if captureName == "call" {
				start := node.StartPosition()
				usage.Line = uint32(start.Row) + 1
				usage.Column = uint32(start.Column) + 1
			} else if captureName == "call.name" {
				usage.Name = string(source[node.StartByte():node.EndByte()])
			} else if captureName == "call.receiver" {
				usage.Context = string(source[node.StartByte():node.EndByte()])
			}
		}

		if usage.Name != "" {
			// Find caller
			for _, s := range symbols {
				if usage.Line >= s.StartLine && usage.Line <= s.EndLine {
					if s.Kind == SymbolKind("function") || s.Kind == SymbolKind("method") {
						usage.Caller = s.Name
						break
					}
				}
			}
			usages = append(usages, usage)
		}
	}

	return usages
}

// ExtractImports extracts imports using queries.
func (qp *QueryParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	if qp.importsQuery == nil {
		return []string{}, nil
	}

	var imports []string
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(qp.importsQuery, tree.RootNode(), source)
	captureNames := qp.importsQuery.CaptureNames()

	for {
		match := matches.Next()
		if match == nil {
			break
		}

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			if captureName == "import" {
				val := strings.Trim(string(source[capture.Node.StartByte():capture.Node.EndByte()]), `"'`)
				imports = append(imports, val)
			}
		}
	}

	return imports, nil
}

// ExtractMetadata extracts metadata using queries.
func (qp *QueryParser) ExtractMetadata(tree *sitter.Tree, source []byte) map[string]string {
	metadata := make(map[string]string)
	if qp.metadataQuery == nil {
		return metadata
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(qp.metadataQuery, tree.RootNode(), source)
	captureNames := qp.metadataQuery.CaptureNames()

	for {
		match := matches.Next()
		if match == nil {
			break
		}

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			if strings.HasPrefix(captureName, "meta.") {
				key := strings.TrimPrefix(captureName, "meta.")
				metadata[key] = string(source[capture.Node.StartByte():capture.Node.EndByte()])
			}
		}
	}

	return metadata
}

// Internal helpers

func (qp *QueryParser) determineVisibility(name string) string {
	if name == "" {
		return "private"
	}

	switch qp.config.Rules.Visibility.Type {
	case "first_letter_case":
		r := []rune(name)[0]
		if unicode.IsUpper(r) {
			return "public"
		}
		return "private"
	case "prefix_underscore":
		if strings.HasPrefix(name, "__") {
			return "private"
		}
		if strings.HasPrefix(name, "_") {
			return "protected"
		}
		return "public"
	default:
		return "public"
	}
}

func (qp *QueryParser) extractDocString(node *sitter.Node, source []byte) string {
	// 1. Check if we have an extra query for docstrings (e.g. Python)
	if q, ok := qp.extraQueries["docstring"]; ok {
		cursor := sitter.NewQueryCursor()
		defer cursor.Close()
		matches := cursor.Matches(q, node, source)
		captureNames := q.CaptureNames()
		if match := matches.Next(); match != nil {
			for _, capture := range match.Captures {
				if captureNames[capture.Index] == "docstring" {
					return strings.TrimSpace(string(source[capture.Node.StartByte():capture.Node.EndByte()]))
				}
			}
		}
	}

	// 2. Default logic: leading comments
	var docLines []string
	curr := node.PrevSibling()
	for curr != nil {
		isComment := false
		kind := curr.Kind()
		// This is a bit loose, we should check if it's a comment node type or starts with prefix
		if strings.Contains(strings.ToLower(kind), "comment") {
			isComment = true
		}

		if isComment {
			text := string(source[curr.StartByte():curr.EndByte()])
			cleaned := text
			for _, prefix := range qp.config.Rules.CommentPrefixes {
				cleaned = strings.TrimPrefix(strings.TrimSpace(cleaned), prefix)
			}
			docLines = append([]string{strings.TrimSpace(cleaned)}, docLines...)
		} else if strings.TrimSpace(string(source[curr.StartByte():curr.EndByte()])) == "" {
			// Skip empty spaces/newlines
		} else {
			break
		}
		curr = curr.PrevSibling()
	}

	return strings.Join(docLines, "\n")
}

// isNil checks if an interface is nil, including typed nil pointers.
func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return true
	}
	return false
}

func (qp *QueryParser) extractTodos(node *sitter.Node, source []byte) []Symbol {
	var todos []Symbol
	if qp.todoRegex == nil {
		return todos
	}

	// Use a simple query to find all comment nodes anywhere in the tree
	// This is much more reliable than manual tree walking for "extra" nodes like comments
	q, err := sitter.NewQuery(qp.language, "(comment) @comment")
	if !isNil(err) {
		return todos
	}
	defer q.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(q, node, source)
	for {
		m := matches.Next()
		if m == nil {
			break
		}

		for _, cap := range m.Captures {
			n := cap.Node
			text := string(source[n.StartByte():n.EndByte()])
			if qp.todoRegex.MatchString(text) {
				todos = append(todos, Symbol{
					Name:      strings.TrimSpace(cleanComment(text)),
					Kind:      SymbolTodo,
					StartLine: uint32(n.StartPosition().Row) + 1,
					EndLine:   uint32(n.EndPosition().Row) + 1,
					StartByte: uint32(n.StartByte()),
					EndByte:   uint32(n.EndByte()),
					Source:    text,
				})
			}
		}
	}

	return todos
}

func (qp *QueryParser) extractSubLanguageSymbols(node *sitter.Node, source []byte, parentName string) []Symbol {
	var symbols []Symbol
	if qp.subLangManager == nil {
		return symbols
	}

	var walk func(*sitter.Node, string)
	walk = func(n *sitter.Node, currentParent string) {
		// Heuristic: string literals or specific embedded markers
		kind := n.Kind()
		if strings.Contains(kind, "string") || strings.Contains(kind, "literal") {
			start := uint32(n.StartByte())
			end := uint32(n.EndByte())
			if end-start > 15 {
				content := source[start:end]
				subSymbols := qp.subLangManager.ProcessEmbeddedCode(
					source, content, start, end, n.StartPosition(), n.EndPosition(), currentParent,
				)
				symbols = append(symbols, subSymbols...)
			}
		}

		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), currentParent)
		}
	}

	walk(node, parentName)
	return symbols
}
