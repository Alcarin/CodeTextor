/*
  File: php_parser.go
  Purpose: Tree-sitter parser implementation for the PHP programming language.
  Author: CodeTextor project
  Notes: Extracts classes, interfaces, traits, functions, methods, and namespaces from PHP code.
*/

package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// PhpParser implements the LanguageParser interface for PHP source code.
type PhpParser struct {
	subLangManager *SubLanguageManager
}

// SetSubLanguageManager implements the SubLanguageAware interface.
func (p *PhpParser) SetSubLanguageManager(manager *SubLanguageManager) {
	p.subLangManager = manager
}

// GetLanguage returns the tree-sitter Language for PHP.
func (p *PhpParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_php.LanguagePHP())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (p *PhpParser) GetFileExtensions() []string {
	return []string{".php"}
}

// ExtractSymbols extracts all symbols (classes, functions, namespaces, etc.) from PHP code.
func (p *PhpParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = p.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
func (p *PhpParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "namespace_definition":
		symbol := p.extractNamespace(node, source)
		symbols = append(symbols, symbol)
		// Process children to find classes/functions within namespace
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = p.walkNode(child, source, symbol.Name, symbols)
		}
		return symbols

	case "class_declaration", "interface_declaration", "trait_declaration":
		kind := SymbolClass
		if nodeType == "interface_declaration" {
			kind = SymbolInterface
		}
		symbol := p.extractClassLike(node, source, kind)
		symbols = append(symbols, symbol)
		// Process children to find methods within class
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "declaration_list" {
				symbols = p.walkNode(child, source, symbol.Name, symbols)
			}
		}
		return symbols

	case "function_definition":
		symbol := p.extractFunction(node, source, parentName, SymbolFunction)
		symbols = append(symbols, symbol)
	case "method_declaration":
		symbol := p.extractFunction(node, source, parentName, SymbolMethod)
		symbols = append(symbols, symbol)

	case "text", "string", "encapsed_string", "heredoc_body":
		// Delegate embedded languages (HTML, JS, SQL, etc) inside strings or raw text blocks
		if p.subLangManager != nil {
			startByte := uint32(node.StartByte())
			endByte := uint32(node.EndByte())
			if endByte-startByte > 15 {
				content := source[startByte:endByte]
				startPoint := node.StartPosition()
				endPoint := node.EndPosition()

				subSymbols := p.subLangManager.ProcessEmbeddedCode(
					source, content, startByte, endByte, startPoint, endPoint, parentName,
				)
				if len(subSymbols) > 0 {
					symbols = append(symbols, subSymbols...)
				}
			}
		}
	}

	// Recursively process child nodes for other cases
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = p.walkNode(child, source, parentName, symbols)
	}

	return symbols
}

// extractNamespace extracts a namespace definition.
func (p *PhpParser) extractNamespace(node *sitter.Node, source []byte) Symbol {
	nameNode := node.ChildByFieldName("name")
	nameStr := "global"
	if nameNode != nil {
		nameStr = nameNode.Utf8Text(source)
	}

	return Symbol{
		Name:      nameStr,
		Kind:      SymbolNamespace,
		StartLine: uint32(node.StartPosition().Row) + 1,
		EndLine:   uint32(node.EndPosition().Row) + 1,
		StartByte: uint32(node.StartByte()),
		EndByte:   uint32(node.EndByte()),
		Source:    node.Utf8Text(source),
	}
}

// extractClassLike extracts class, interface, or trait declarations.
func (p *PhpParser) extractClassLike(node *sitter.Node, source []byte, kind SymbolKind) Symbol {
	nameNode := node.ChildByFieldName("name")
	nameStr := "Anonymous"
	if nameNode != nil {
		nameStr = nameNode.Utf8Text(source)
	}

	docString := p.extractLeadingComment(node, source)

	return Symbol{
		Name:      nameStr,
		Kind:      kind,
		StartLine: uint32(node.StartPosition().Row) + 1,
		EndLine:   uint32(node.EndPosition().Row) + 1,
		StartByte: uint32(node.StartByte()),
		EndByte:   uint32(node.EndByte()),
		Source:    node.Utf8Text(source),
		DocString: docString,
	}
}

// extractFunction extracts a function or method definition.
func (p *PhpParser) extractFunction(node *sitter.Node, source []byte, parentName string, kind SymbolKind) Symbol {
	nameNode := node.ChildByFieldName("name")
	nameStr := "anonymous"
	if nameNode != nil {
		nameStr = nameNode.Utf8Text(source)
	}

	// Extract parameters
	paramsNode := node.ChildByFieldName("parameters")
	signature := ""
	if paramsNode != nil {
		signature = paramsNode.Utf8Text(source)
	}

	docString := p.extractLeadingComment(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       kind,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Parent:     parentName,
		DocString:  docString,
		Visibility: p.extractVisibility(node),
	}
}

// ExtractImports extracts all use statements from PHP code.
func (p *PhpParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	p.walkImports(rootNode, source, &imports)

	return imports, nil
}

// walkImports recursively finds all namespace use declarations.
func (p *PhpParser) walkImports(node *sitter.Node, source []byte, imports *[]string) {
	if node.Kind() == "namespace_use_declaration" {
		// Example: use App\Models\User;
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "namespace_use_clause" {
				*imports = append(*imports, child.Utf8Text(source))
			}
		}
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		p.walkImports(child, source, imports)
	}
}

// ExtractMetadata extracts file-level metadata.
func (p *PhpParser) ExtractMetadata(tree *sitter.Tree, source []byte) map[string]string {
	return make(map[string]string)
}

// extractVisibility attempts to find visibility modifiers (public, private, protected).
func (p *PhpParser) extractVisibility(node *sitter.Node) string {
	// Look through children for visibility keywords
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		text := strings.ToLower(child.Kind())
		if text == "public" || text == "private" || text == "protected" {
			return text
		}
	}
	return "public" // Default in PHP if not specified
}

// extractLeadingComment extracts comments immediately preceding a node.
func (p *PhpParser) extractLeadingComment(node *sitter.Node, source []byte) string {
	// Simplified implementation as in go_parser.go
	startByte := node.StartByte()
	lines := strings.Split(string(source[:startByte]), "\n")
	var docLines []string

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/**") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") {
			// Very basic comment detection
			docLines = append([]string{line}, docLines...)
		} else {
			break
		}
	}

	return strings.Join(docLines, "\n")
}

// ExtractUsages is a stub for PhpParser.
func (p *PhpParser) ExtractUsages(tree *sitter.Tree, source []byte, symbols []Symbol) []ParserSymbolUsage {
	return nil
}
