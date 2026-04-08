package chunker

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// QueryParser implements the LanguageParser interface using Tree-sitter queries.
type QueryParser struct {
	config         *LanguageConfig
	language       *sitter.Language
	subLangManager *SubLanguageManager

	// Compiled queries
	symbolsQuery        *sitter.Query
	importsQuery        *sitter.Query
	metadataQuery       *sitter.Query
	usagesQuery        *sitter.Query
	subLanguagesQuery  *sitter.Query
	extraQueries       map[string]*sitter.Query

	// Regex for TODOs and imports
	todoRegex            *regexp.Regexp
	importRegex          *regexp.Regexp
	excludeImportRegex   *regexp.Regexp
	symbolPatternRegexes []*regexp.Regexp
	includedRanges       []sitter.Range
	recursionDepth       int
	mu                   sync.Mutex
}

// NewQueryParser creates a new QueryParser for the given configuration.
func NewQueryParser(config *LanguageConfig) (*QueryParser, error) {
	lang, err := GetGrammar(config.Language.Grammar)
	if err != nil {
		log.Printf("Error: failed to get grammar for language %q (grammar: %q): %v", config.Language.Name, config.Language.Grammar, err)
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

	if config.Queries.ExcludeImportPattern != "" {
		re, err := regexp.Compile(config.Queries.ExcludeImportPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile exclude import regex: %w", err)
		}
		qp.excludeImportRegex = re
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

	// Compile Import regex
	if config.Queries.ImportPattern != "" {
		qp.importRegex, err = regexp.Compile(config.Queries.ImportPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile import regex: %w", err)
		}
	}

	// Compile Symbol Pattern regexes
	for _, pc := range config.Queries.SymbolPatterns {
		re, err := regexp.Compile(pc.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile symbol pattern regex '%s': %w", pc.Pattern, err)
		}
		qp.symbolPatternRegexes = append(qp.symbolPatternRegexes, re)
	}

	// Compile Sub-languages Discovery Query
	// Compile Sub-languages Discovery Query
	// Compile Sub-languages Discovery Query
	if len(config.SubLanguages) > 0 {
		var qStr strings.Builder
		qStr.WriteString("[")
		
		for kind := range config.SubLanguages {
			// Skip special keys or already handled ones
			if strings.ContainsAny(kind, "()[]{} @.") {
				continue
			}
			
			// Verify if the node type exists before adding it to avoid query compilation failure
			qTest, err := sitter.NewQuery(lang, fmt.Sprintf("(%s) @sub", kind))
			if isNil(err) && !isNil(qTest) {
				qStr.WriteString(fmt.Sprintf("(%s) ", kind))
				qTest.Close()
			}
		}
		qStr.WriteString("] @sub")

		q, err := sitter.NewQuery(lang, qStr.String())
		if isNil(err) && !isNil(q) {
			qp.subLanguagesQuery = q
		} else {
			log.Printf("Warning: failed to compile sub-languages query for %s: %v", config.Language.Name, err)
		}
	}

	return qp, nil
}

// Parse implements the LanguageParser interface.
func (qp *QueryParser) Parse(source []byte) (ParseResult, error) {
	// For legacy support when Lock/SetIncludedRanges/SetRecursionDepth are used externally
	qp.mu.Lock()
	ranges := qp.includedRanges
	depth := qp.recursionDepth
	qp.includedRanges = nil // Reset state
	qp.mu.Unlock()

	return qp.ParseWithContext(source, ranges, depth)
}

// ParseWithContext implements the SubLanguageAware interface.
func (qp *QueryParser) ParseWithContext(source []byte, ranges []sitter.Range, depth int) (ParseResult, error) {
	// Recursion protection
	const MaxRecursionDepth = 3
	if depth > MaxRecursionDepth {
		// Stop deep recursion, just return basic result or empty to avoid infinite loop
		return ParseResult{Language: qp.config.Language.Name, FilePath: ""}, nil
	}

	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(qp.language)
	
	if len(ranges) > 0 {
		parser.SetIncludedRanges(ranges)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return ParseResult{}, fmt.Errorf("failed to parse source code")
	}
	defer tree.Close()

	symbols, err := qp.ExtractSymbols(tree, source)
	if err != nil {
		return ParseResult{}, fmt.Errorf("failed to extract symbols: %w", err)
	}

	imports, err := qp.ExtractImports(tree, source)
	if err != nil {
		return ParseResult{}, fmt.Errorf("failed to extract imports: %w", err)
	}

	metadata := qp.ExtractMetadata(tree, source)
	usages := qp.ExtractUsages(tree, source, symbols)

	// Process Sub-languages (SQL, HTML, JS in template/strings)
	subSyms, subImports, subUsages := qp.processSubLanguages(tree, source, symbols, depth)
	symbols = append(symbols, subSyms...)
	imports = append(imports, subImports...)
	usages = append(usages, subUsages...)

	return ParseResult{
		Language: qp.config.Language.Name,
		Symbols:  symbols,
		Imports:  imports,
		Usages:   usages,
		Metadata: metadata,
		FilePath: "", // Will be set by caller
	}, nil
}

// SetSubLanguageManager implements the SubLanguageAware interface.
func (qp *QueryParser) SetSubLanguageManager(manager *SubLanguageManager) {
	qp.subLangManager = manager
}

// SetIncludedRanges implements the SubLanguageAware interface.
func (qp *QueryParser) SetIncludedRanges(ranges []sitter.Range) {
	qp.includedRanges = ranges
}

// SetRecursionDepth implements the SubLanguageAware interface.
func (qp *QueryParser) SetRecursionDepth(depth int) {
	qp.recursionDepth = depth
}

// Lock implements the SubLanguageAware interface.
func (qp *QueryParser) Lock() {
	qp.mu.Lock()
}

// Unlock implements the SubLanguageAware interface.
func (qp *QueryParser) Unlock() {
	qp.mu.Unlock()
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

	symbolMap := make(map[string]Symbol)
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
				sym.Name = strings.TrimSpace(string(source[node.StartByte():node.EndByte()]))
			case strings.HasPrefix(captureName, "name."):
				suffix := strings.TrimPrefix(captureName, "name.")
				value := strings.Trim(string(source[node.StartByte():node.EndByte()]), `"' `)
				formatted := qp.applyFormatting(suffix, value)
				if len(formatted) > 0 {
					rule := qp.config.Rules.Formatting[suffix]
					joiner := rule.Join
					if joiner == "" {
						joiner = " " // Default separator for symbol name
					}
					if sym.Name != "" {
						sym.Name += joiner + strings.Join(formatted, joiner)
					} else {
						sym.Name = strings.Join(formatted, joiner)
					}
				}
			case captureName == "parent":
				sym.Parent = strings.TrimSpace(string(source[node.StartByte():node.EndByte()]))
			case captureName == "implements":
				val := strings.TrimSpace(string(source[node.StartByte():node.EndByte()]))
				if val != "" {
					sym.Implements = append(sym.Implements, val)
				}
			case captureName == "signature":
				val := strings.TrimSpace(string(source[node.StartByte():node.EndByte()]))
				if val != "" {
					if sym.Signature != "" {
						if !strings.Contains(sym.Signature, val) {
							sym.Signature += " " + val
						}
					} else {
						sym.Signature = val
					}
				}
			case strings.HasPrefix(captureName, "signature."):
				suffix := strings.TrimPrefix(captureName, "signature.")
				value := strings.Trim(string(source[node.StartByte():node.EndByte()]), `"' `)
				formatted := qp.applyFormatting(suffix, value)
				for _, val := range formatted {
					if val != "" {
						sig := fmt.Sprintf("%s='%s'", suffix, strings.TrimPrefix(val, qp.config.Rules.Formatting[suffix].Prefix))
						if sym.Signature != "" {
							if !strings.Contains(sym.Signature, sig) {
								sym.Signature += " " + sig
							}
						} else {
							sym.Signature = sig
						}
					}
				}
			}
		}

		if kind != "" {
			sym.Kind = kind
			sym.Language = qp.config.Language.Name
			sym.Name = strings.Trim(strings.TrimSpace(sym.Name), "\"")
			if sym.Name == "" {
				sym.Name = "anonymous"
			}
			// Special case for code_block to match legacy behavior
			if (kind == SymbolMarkdownCode || kind == "code") && sym.Signature != "" && !strings.HasPrefix(sym.Name, "code:") {
				sym.Name = "code:" + sym.Signature
			}
			sym.Visibility = qp.determineVisibility(sym.Name)

			// Deduplication and naming logic
			key := fmt.Sprintf("%d-%d", sym.StartByte, sym.EndByte)
			if existing, ok := symbolMap[key]; ok {
				if shouldReplace(existing.Kind, sym.Kind) {
					symbolMap[key] = sym
				} else if existing.Kind == sym.Kind {
					if sym.Name != "anonymous" && (existing.Name == "anonymous" || !strings.Contains(existing.Name, sym.Name)) {
						if existing.Name == "anonymous" {
							existing.Name = sym.Name
						} else {
							// HTML/CSS join logic: avoid spaces for selectors like tag#id or tag.class
							if strings.HasPrefix(sym.Name, ".") || strings.HasPrefix(sym.Name, "#") {
								existing.Name += sym.Name
							} else if strings.HasPrefix(existing.Name, ".") || strings.HasPrefix(existing.Name, "#") {
								existing.Name = sym.Name + existing.Name
							} else {
								existing.Name = sym.Name + " " + existing.Name
							}
						}
					}
					if sym.Signature != "" && !strings.Contains(existing.Signature, sym.Signature) {
						if existing.Signature == "" {
							existing.Signature = sym.Signature
						} else {
							existing.Signature += " " + sym.Signature
						}
					}
					if len(sym.Implements) > 0 {
						for _, impl := range sym.Implements {
							found := false
							for _, v := range existing.Implements {
								if v == impl {
									found = true
									break
								}
							}
							if !found {
								existing.Implements = append(existing.Implements, impl)
							}
						}
					}
					symbolMap[key] = existing
				}
			} else {
				symbolMap[key] = sym
			}
		}
	}

	// Transfer to slice and handle simple parent inheritance based on range nesting
	for _, sym := range symbolMap {
		symbols = append(symbols, sym)
	}

	// Sort by range size (smallest first) to find the immediate parent easily
	sort.Slice(symbols, func(i, j int) bool {
		si := symbols[i].EndByte - symbols[i].StartByte
		sj := symbols[j].EndByte - symbols[j].StartByte
		return si < sj
	})

	for i := range symbols {
		if symbols[i].Parent != "" {
			continue
		}
		// Look for the smallest symbol that strictly contains this one
		for j := range symbols {
			if i == j {
				continue
			}
			if symbols[j].StartByte <= symbols[i].StartByte && symbols[j].EndByte >= symbols[i].EndByte {
				// Don't use the same symbol or exactly the same range unless different kind?
				// For JSON/SQL, nested objects are parents.
				if symbols[j].StartByte < symbols[i].StartByte || symbols[j].EndByte > symbols[i].EndByte {
					if symbols[j].Name != "anonymous" {
						symbols[i].Parent = symbols[j].Name
						break // Found the immediate parent
					}
				}
			}
		}
	}

	// 2. Extract symbols using regex patterns
	for i, re := range qp.symbolPatternRegexes {
		pc := qp.config.Queries.SymbolPatterns[i]
		matches := re.FindAllSubmatchIndex(source, -1)
		for _, m := range matches {
			sym := Symbol{
				Kind: SymbolKind(pc.Kind),
			}

			// Extract name/sig
			if pc.NameGroup >= 0 && pc.NameGroup*2+1 < len(m) {
				start, end := m[pc.NameGroup*2], m[pc.NameGroup*2+1]
				if start >= 0 && end >= 0 {
					sym.Name = pc.NamePrefix + string(source[start:end])
				}
			} else {
				sym.Name = pc.NamePrefix
			}
			if pc.SignatureGroup >= 0 && pc.SignatureGroup*2+1 < len(m) {
				start, end := m[pc.SignatureGroup*2], m[pc.SignatureGroup*2+1]
				if start >= 0 && end >= 0 {
					sym.Signature = pc.SignaturePrefix + string(source[start:end])
				}
			} else {
				sym.Signature = pc.SignaturePrefix
			}

			startByte, endByte := uint32(m[0]), uint32(m[1])
			sym.StartByte, sym.EndByte = startByte, endByte
			sym.Source = string(source[startByte:endByte])
			contentBefore := source[:startByte]
			sym.StartLine = uint32(strings.Count(string(contentBefore), "\n")) + 1
			sym.EndLine = sym.StartLine + uint32(strings.Count(string(source[startByte:endByte]), "\n"))

			// If match range starts near an existing symbol, override its name/kind strictly
			found := false
			for j, existing := range symbols {
				diff := int(existing.StartByte) - int(startByte)
				if diff >= -15 && diff <= 15 {
					if sym.Name != "" {
						symbols[j].Name = sym.Name
					}
					if sym.Kind != "" {
						symbols[j].Kind = sym.Kind
					}
					// Strictly override signature if regex provides one
					if sym.Signature != "" {
						symbols[j].Signature = sym.Signature
					}
					found = true
					break
				}
			}
			if !found {
				sym.Visibility = qp.determineVisibility(sym.Name)
				symbols = append(symbols, sym)
			}
		}
	}

	// 4. Update with TODOs
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

		var baseUsage ParserSymbolUsage
		var names []string

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			node := capture.Node

			if captureName == "call" {
				start := node.StartPosition()
				baseUsage.Line = uint32(start.Row) + 1
				baseUsage.Column = uint32(start.Column) + 1
			} else if captureName == "call.name" {
				names = []string{string(source[node.StartByte():node.EndByte()])}
			} else if strings.HasPrefix(captureName, "call.name.") {
				suffix := strings.TrimPrefix(captureName, "call.name.")
				value := strings.Trim(string(source[node.StartByte():node.EndByte()]), `"' `)
				names = qp.applyFormatting(suffix, value)
			} else if captureName == "call.receiver" {
				baseUsage.Context = string(source[node.StartByte():node.EndByte()])
			}
		}

		for _, name := range names {
			if name == "" {
				continue
			}
			usage := baseUsage
			usage.Name = name

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
	var imports []string

	if qp.importsQuery != nil {
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
					val := strings.Trim(string(source[capture.Node.StartByte():capture.Node.EndByte()]), `"' `)
					if val != "" {
						// Filter through excludeImportRegex if provided
						if qp.excludeImportRegex != nil {
							if qp.excludeImportRegex.MatchString(val) {
								continue
							}
						}

						// Filter through importRegex if provided
						if qp.importRegex != nil {
							if qp.importRegex.MatchString(val) {
								imports = append(imports, val)
							}
						} else {
							imports = append(imports, val)
						}
					}
				}
			}
		}
	}

	// Also use regex if pattern is provided
	if qp.importRegex != nil && qp.importsQuery == nil {
		matches := qp.importRegex.FindAllSubmatch(source, -1)
		for _, match := range matches {
			if len(match) >= 1 {
				// Use the first capture group if it exists, otherwise the full match
				val := ""
				if len(match) >= 2 {
					val = string(match[1])
				} else {
					val = string(match[0])
				}
				val = strings.Trim(val, `"' `)
				
				// Apply exclusion
				if qp.excludeImportRegex != nil && qp.excludeImportRegex.MatchString(val) {
					continue
				}

				if val != "" {
					imports = append(imports, val)
				}
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


func (qp *QueryParser) extractTodos(node *sitter.Node, source []byte) []Symbol {
	var todos []Symbol
	if qp.todoRegex == nil {
		return todos
	}

	// Dynamic TODO extraction based on configured node types
	nodeTypes := qp.config.Rules.Todo.NodeTypes
	if len(nodeTypes) == 0 {
		nodeTypes = []string{"comment"} // Fallback
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		isTargetType := false
		for _, t := range nodeTypes {
			if n.Kind() == t {
				isTargetType = true
				break
			}
		}

		if isTargetType {
			text := string(source[n.StartByte():n.EndByte()])
			if matches := qp.todoRegex.FindStringSubmatch(text); len(matches) > 0 {
				// Use the full match as content to include the keyword (TODO, FIXME, etc.)
				// cleanComment will take care of comment markers but not the user-configured keywords.
				content := text

				todos = append(todos, Symbol{
					Name:      strings.TrimSpace(cleanComment(content)),
					Kind:      SymbolTodo,
					Language:  qp.config.Language.Name,
					StartLine: uint32(n.StartPosition().Row) + 1,
					EndLine:   uint32(n.EndPosition().Row) + 1,
					StartByte: uint32(n.StartByte()),
					EndByte:   uint32(n.EndByte()),
					Source:    text,
				})
			}
		}

		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return todos
}

// processSubLanguages performs a single highly optimized pass to discover and parse
// all embedded code fragments using the pre-compiled subLanguagesQuery.
func (qp *QueryParser) processSubLanguages(tree *sitter.Tree, source []byte, symbols []Symbol, depth int) ([]Symbol, []string, []ParserSymbolUsage) {
	if qp.subLanguagesQuery == nil || qp.subLangManager == nil {
		return nil, nil, nil
	}

	
	type langBatch struct {
		ranges       []sitter.Range
		parentNames  []string
	}
	batches := make(map[string]*langBatch)

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	
	matches := cursor.Matches(qp.subLanguagesQuery, tree.RootNode(), source)
	count := 0
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		count++
		
		for _, capture := range match.Captures {
			node := capture.Node
			kind := node.Kind()
			
			// 1. Determine target language strategy from config
			targetLang := qp.config.SubLanguages[kind]
			if targetLang == "" {
				continue
			}
			
			// 2. Identify the actual content node (raw_text/text for tags, or the node itself)
			contentNode := &node
			// If this node is a container (like script_element), its actual code is usually in a 'raw_text' or 'text' child.
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				cKind := child.Kind()
				if cKind == "raw_text" || cKind == "text" {
					contentNode = child
					break
				}
			}
			
			// Identify the best parent name: host symbol > node kind
			containerPrefix := ""
			for _, sym := range symbols {
				if uint(node.StartByte()) >= uint(sym.StartByte) && uint(node.EndByte()) <= uint(sym.EndByte) {
					containerPrefix = sym.Name
				}
			}

			if containerPrefix == "" {
				containerPrefix = kind
			}
			
			// 3. Queue for batch processing (with range deduplication)
			newRange := sitter.Range{
				StartByte:  uint(contentNode.StartByte()),
				EndByte:    uint(contentNode.EndByte()),
				StartPoint: sitter.Point{Row: uint(contentNode.StartPosition().Row), Column: uint(contentNode.StartPosition().Column)},
				EndPoint:   sitter.Point{Row: uint(contentNode.EndPosition().Row), Column: uint(contentNode.EndPosition().Column)},
			}
			
			// Check if this exact range was already added to ANY batch
			duplicate := false
			for _, b := range batches {
				for _, r := range b.ranges {
					if r.StartByte == newRange.StartByte && r.EndByte == newRange.EndByte {
						duplicate = true
						break
					}
				}
				if duplicate { break }
			}
			
			if !duplicate {
				batch, ok := batches[targetLang]
				if !ok {
					batch = &langBatch{}
					batches[targetLang] = batch
				}
				batch.ranges = append(batch.ranges, newRange)
				batch.parentNames = append(batch.parentNames, containerPrefix)
			}
		}
	}

	// 4. Batch Process each language
	var allSymbols []Symbol
	var allImports []string
	var allUsages []ParserSymbolUsage

	for lang, batch := range batches {
		if len(batch.ranges) == 0 {
			continue
		}

		subSyms, subImports, subUsages := qp.subLangManager.BatchExtractAll(lang, source, batch.ranges, batch.parentNames, symbols, depth+1)
		allSymbols = append(allSymbols, subSyms...)
		allImports = append(allImports, subImports...)
		allUsages = append(allUsages, subUsages...)
	}

	if len(batches) > 0 {
		// log.Printf("[SubLang] Optimized batch processing for %d languages in %v", len(batches), time.Since(start))
	}

	return allSymbols, allImports, allUsages
}

// applyFormatting applies TOML-defined formatting rules to a value.
func (qp *QueryParser) applyFormatting(suffix, value string) []string {
	rule, ok := qp.config.Rules.Formatting[suffix]
	if !ok {
		return []string{value}
	}

	// 1. Normalization
	if rule.Lowercase {
		value = strings.ToLower(value)
	}

	// 2. Splitting (multi-value attributes like HTML classes)
	var parts []string
	if rule.Split != "" {
		parts = strings.FieldsFunc(value, func(r rune) bool {
			return strings.ContainsRune(rule.Split, r)
		})
	} else {
		parts = []string{value}
	}

	// 3. Prefixing
	for i := range parts {
		parts[i] = rule.Prefix + parts[i]
	}

	return parts
}

func shouldReplace(oldKind, newKind SymbolKind) bool {
	priority := map[SymbolKind]int{
		SymbolComponent:  15,
		SymbolClass:      10,
		SymbolInterface:  10,
		SymbolHook:       9,
		SymbolMethod:     9,
		SymbolFunction:   8,
		SymbolVariable:   5,
		SymbolConstant:   5,
		"element":        3,
		"media":         3,
		"element_generic": 2,
		"heading":        3,
	}

	return priority[newKind] > priority[oldKind]
}
