/*
  File: html_parser.go
  Purpose: Tree-sitter parser implementation for HTML.
  Author: CodeTextor project
  Notes: Extracts elements, attributes, and structure from HTML code.
*/

package chunker

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
)

// HTMLParser implements the LanguageParser interface for HTML source code.
type HTMLParser struct{}

// GetLanguage returns the tree-sitter Language for HTML.
func (h *HTMLParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_html.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (h *HTMLParser) GetFileExtensions() []string {
	return []string{".html", ".htm"}
}

// ExtractSymbols extracts all symbols from HTML code.
// For HTML, we extract:
//   - Elements with IDs (as symbols)
//   - Script and style blocks
//   - Major structural elements (head, body, main sections)
func (h *HTMLParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = h.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
func (h *HTMLParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "element":
		// Extract elements with IDs or important semantic tags
		symbol := h.extractElement(node, source, parentName)
		if symbol != nil {
			symbols = append(symbols, *symbol)
			// Recursively process child elements with this symbol as parent
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				symbols = h.walkNode(child, source, symbol.Name, symbols)
			}
			// Don't process children again after returning
			return symbols
		}
		// If symbol is nil, continue to process children with current parent
	case "script_element":
		symbols = append(symbols, h.extractScriptElement(node, source))
		return symbols // Don't process script_element children
	case "style_element":
		symbols = append(symbols, h.extractStyleElement(node, source))
		return symbols // Don't process style_element children
	}

	// Recursively process child nodes (only reached if no symbol was extracted above)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = h.walkNode(child, source, parentName, symbols)
	}

	return symbols
}

// extractElement extracts an HTML element with its attributes.
// Extracts all HTML elements and stores important attributes in Signature field.
func (h *HTMLParser) extractElement(node *sitter.Node, source []byte, parentName string) *Symbol {
	// Get the tag name - tree-sitter HTML doesn't use field names, so we need to find the start_tag child
	var startTag *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "start_tag" {
			startTag = child
			break
		}
	}

	if startTag == nil {
		return nil
	}

	// Find tag_name within start_tag
	var tagNameNode *sitter.Node
	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "tag_name" {
			tagNameNode = child
			break
		}
	}

	if tagNameNode == nil {
		return nil
	}

	tagName := tagNameNode.Utf8Text(source)

	// Extract ID attribute for naming
	elementID := h.extractAttributeValue(startTag, "id", source)

	// Build element name
	name := tagName
	if elementID != "" {
		name = tagName + "#" + elementID
	}

	// Build signature with important attributes
	signature := h.buildAttributeSignature(startTag, source)

	return &Symbol{
		Name:       name,
		Kind:       SymbolElement,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Parent:     parentName,
		Visibility: "public",
	}
}

// extractScriptElement extracts a script block from HTML.
func (h *HTMLParser) extractScriptElement(node *sitter.Node, source []byte) Symbol {
	return Symbol{
		Name:       "script",
		Kind:       SymbolScript,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: "public",
	}
}

// extractStyleElement extracts a style block from HTML.
func (h *HTMLParser) extractStyleElement(node *sitter.Node, source []byte) Symbol {
	return Symbol{
		Name:       "style",
		Kind:       SymbolStyle,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: "public",
	}
}

// buildAttributeSignature builds a string representation of all attributes.
// Returns a string like "id='foo' class='bar baz'" or empty string if no attributes.
func (h *HTMLParser) buildAttributeSignature(startTag *sitter.Node, source []byte) string {
	var attrs []string

	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "attribute" {
			// Find attribute_name child
			var attrNameNode *sitter.Node
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "attribute_name" {
					attrNameNode = child.Child(j)
					break
				}
			}

			if attrNameNode != nil {
				attrName := attrNameNode.Utf8Text(source)

				// Find the value
				var attrValue string
				for j := uint(0); j < child.ChildCount(); j++ {
					attrChild := child.Child(j)
					if attrChild.Kind() == "quoted_attribute_value" {
						// Look for attribute_value inside quoted_attribute_value
						for k := uint(0); k < attrChild.ChildCount(); k++ {
							valueNode := attrChild.Child(k)
							if valueNode.Kind() == "attribute_value" {
								attrValue = valueNode.Utf8Text(source)
								break
							}
						}
					} else if attrChild.Kind() == "attribute_value" {
						attrValue = attrChild.Utf8Text(source)
					}
				}

				// Add to signature string
				if attrValue != "" {
					attrs = append(attrs, attrName+"='"+attrValue+"'")
				} else {
					attrs = append(attrs, attrName)
				}
			}
		}
	}

	if len(attrs) == 0 {
		return ""
	}

	result := ""
	for i, attr := range attrs {
		if i > 0 {
			result += " "
		}
		result += attr
	}
	return result
}

// extractAttributeValue extracts the value of a specific attribute from a start_tag node.
func (h *HTMLParser) extractAttributeValue(startTag *sitter.Node, attrName string, source []byte) string {
	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "attribute" {
			// Find attribute_name child
			var attrNameNode *sitter.Node
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "attribute_name" {
					attrNameNode = child.Child(j)
					break
				}
			}

			if attrNameNode != nil && attrNameNode.Utf8Text(source) == attrName {
				// Find quoted_attribute_value or attribute_value child
				for j := uint(0); j < child.ChildCount(); j++ {
					attrChild := child.Child(j)
					if attrChild.Kind() == "quoted_attribute_value" {
						// Look for attribute_value inside quoted_attribute_value
						for k := uint(0); k < attrChild.ChildCount(); k++ {
							valueNode := attrChild.Child(k)
							if valueNode.Kind() == "attribute_value" {
								return valueNode.Utf8Text(source)
							}
						}
					} else if attrChild.Kind() == "attribute_value" {
						return attrChild.Utf8Text(source)
					}
				}
			}
		}
	}
	return ""
}

// ExtractImports extracts imports from HTML (link and script src).
func (h *HTMLParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	imports = h.walkImports(rootNode, source, imports)

	return imports, nil
}

// walkImports recursively finds all link and script src attributes.
func (h *HTMLParser) walkImports(node *sitter.Node, source []byte, imports []string) []string {
	nodeType := node.Kind()

	// Handle regular elements
	if nodeType == "element" {
		// Find start_tag child
		var startTag *sitter.Node
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "start_tag" {
				startTag = child
				break
			}
		}

		if startTag != nil {
			// Find tag_name within start_tag
			var tagNameNode *sitter.Node
			for i := uint(0); i < startTag.ChildCount(); i++ {
				child := startTag.Child(i)
				if child.Kind() == "tag_name" {
					tagNameNode = child
					break
				}
			}

			if tagNameNode != nil {
				tag := tagNameNode.Utf8Text(source)
				if tag == "link" {
					href := h.extractAttributeValue(startTag, "href", source)
					if href != "" {
						imports = append(imports, href)
					}
				} else if tag == "script" {
					src := h.extractAttributeValue(startTag, "src", source)
					if src != "" {
						imports = append(imports, src)
					}
				}
			}
		}
	}

	// Handle script_element nodes
	if nodeType == "script_element" {
		// Find start_tag child
		var startTag *sitter.Node
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "start_tag" {
				startTag = child
				break
			}
		}

		if startTag != nil {
			src := h.extractAttributeValue(startTag, "src", source)
			if src != "" {
				imports = append(imports, src)
			}
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = h.walkImports(child, source, imports)
	}

	return imports
}
