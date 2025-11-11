/*
  File: css_parser.go
  Purpose: Tree-sitter parser implementation for CSS.
  Author: CodeTextor project
  Notes: Extracts selectors, rules, and at-rules from CSS code.
*/

package chunker

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_css "github.com/tree-sitter/tree-sitter-css/bindings/go"
)

// CSSParser implements the LanguageParser interface for CSS source code.
type CSSParser struct{}

// GetLanguage returns the tree-sitter Language for CSS.
func (c *CSSParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_css.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (c *CSSParser) GetFileExtensions() []string {
	return []string{".css", ".scss", ".sass"}
}

// ExtractSymbols extracts all symbols from CSS code.
// For CSS, we extract:
//   - Class selectors (.classname)
//   - ID selectors (#idname)
//   - At-rules (@media, @keyframes, etc.)
func (c *CSSParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = c.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
func (c *CSSParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "rule_set":
		// Extract CSS rule sets (selector { properties })
		symbol := c.extractRuleSet(node, source)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	case "media_statement":
		// Extract @media rules
		symbols = append(symbols, c.extractMediaRule(node, source))
	case "keyframes_statement":
		// Extract @keyframes
		symbols = append(symbols, c.extractKeyframesRule(node, source))
	case "import_statement":
		// Already handled in ExtractImports, skip here
		break
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = c.walkNode(child, source, parentName, symbols)
	}

	return symbols
}

// extractRuleSet extracts a CSS rule set.
// Example: .classname { color: red; }
func (c *CSSParser) extractRuleSet(node *sitter.Node, source []byte) *Symbol {
	// Find selectors child node
	var selectors *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "selectors" {
			selectors = child
			break
		}
	}

	if selectors == nil {
		return nil
	}

	// Get the first selector as the symbol name
	selectorText := selectors.Utf8Text(source)

	return &Symbol{
		Name:       selectorText,
		Kind:       SymbolCSSRule,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: "public",
	}
}

// extractMediaRule extracts an @media rule.
// Example: @media (max-width: 768px) { ... }
func (c *CSSParser) extractMediaRule(node *sitter.Node, source []byte) Symbol {
	query := node.ChildByFieldName("query")
	queryText := "@media"
	if query != nil {
		queryText = "@media " + query.Utf8Text(source)
	}

	return Symbol{
		Name:       queryText,
		Kind:       SymbolCSSMedia,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: "public",
	}
}

// extractKeyframesRule extracts an @keyframes rule.
// Example: @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
func (c *CSSParser) extractKeyframesRule(node *sitter.Node, source []byte) Symbol {
	// Find keyframes_name child node
	var nameNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "keyframes_name" {
			nameNode = child
			break
		}
	}

	nameStr := "@keyframes"
	if nameNode != nil {
		nameStr = "@keyframes " + nameNode.Utf8Text(source)
	}

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolCSSKeyframes,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: "public",
	}
}

// ExtractImports extracts @import statements from CSS.
func (c *CSSParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	imports = c.walkImports(rootNode, source, imports)

	return imports, nil
}

// walkImports recursively finds all @import statements.
func (c *CSSParser) walkImports(node *sitter.Node, source []byte, imports []string) []string {
	if node.Kind() == "import_statement" {
		// Extract the import path (usually a string or url())
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "string_value" || child.Kind() == "call_expression" {
				importPath := child.Utf8Text(source)
				// Clean up quotes and url()
				importPath = cleanImportPath(importPath)
				if importPath != "" {
					imports = append(imports, importPath)
				}
			}
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = c.walkImports(child, source, imports)
	}

	return imports
}

// cleanImportPath removes quotes, url(), and whitespace from import paths.
func cleanImportPath(path string) string {
	// Remove url()
	if len(path) > 4 && path[:4] == "url(" {
		path = path[4 : len(path)-1]
	}
	// Remove quotes
	path = trimQuotes(path)
	return path
}

// trimQuotes removes surrounding quotes from a string.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
