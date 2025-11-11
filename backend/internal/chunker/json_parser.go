/*
  File: json_parser.go
  Purpose: Tree-sitter parser implementation for JSON configuration files.
  Author: CodeTextor project
  Notes: Extracts key/value pairs from JSON objects and exposes them as symbols.
*/

package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

// JSONParser implements the LanguageParser interface for JSON files.
type JSONParser struct{}

// GetLanguage returns the tree-sitter Language for JSON.
func (j *JSONParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_json.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (j *JSONParser) GetFileExtensions() []string {
	return []string{".json"}
}

// ExtractSymbols walks the JSON AST and extracts each key/value pair as a symbol.
func (j *JSONParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	root := tree.RootNode()
	symbols = j.walkNode(root, source, symbols, "")
	return symbols, nil
}

// walkNode recursively visits AST nodes and records JSON pairs.
func (j *JSONParser) walkNode(node *sitter.Node, source []byte, symbols []Symbol, parent string) []Symbol {
	if node == nil {
		return symbols
	}

	switch node.Kind() {
	case "pair":
		name := "unknown"
		if keyNode := node.ChildByFieldName("key"); keyNode != nil {
			name = trimJSONKey(keyNode.Utf8Text(source))
		}

		value := ""
		if valueNode := node.ChildByFieldName("value"); valueNode != nil {
			value = strings.TrimSpace(valueNode.Utf8Text(source))
		}

		symbols = append(symbols, Symbol{
			Name:       name,
			Kind:       SymbolVariable,
			StartLine:  uint32(node.StartPosition().Row) + 1,
			EndLine:    uint32(node.EndPosition().Row) + 1,
			StartByte:  uint32(node.StartByte()),
			EndByte:    uint32(node.EndByte()),
			Source:     node.Utf8Text(source),
			Signature:  value,
			Visibility: "public",
			Parent:     parent,
		})

		if valueNode := node.ChildByFieldName("value"); valueNode != nil {
			symbols = j.walkNode(valueNode, source, symbols, name)
		}
		return symbols
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = j.walkNode(child, source, symbols, parent)
	}

	return symbols
}

// ExtractImports returns an empty list because JSON files do not have imports.
func (j *JSONParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	return []string{}, nil
}

// trimJSONKey removes quotes from JSON object keys.
func trimJSONKey(raw string) string {
	raw = strings.TrimSpace(raw)
	return trimQuotes(raw)
}
