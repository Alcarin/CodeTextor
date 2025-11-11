/*
  File: python_parser.go
  Purpose: Tree-sitter parser implementation for the Python programming language.
  Author: CodeTextor project
  Notes: Extracts functions, classes, methods, imports, and docstrings from Python code.
*/

package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// PythonParser implements the LanguageParser interface for Python source code.
type PythonParser struct{}

// GetLanguage returns the tree-sitter Language for Python.
func (p *PythonParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_python.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (p *PythonParser) GetFileExtensions() []string {
	return []string{".py"}
}

// ExtractSymbols extracts all symbols (functions, classes, methods, etc.) from Python code.
// It walks the AST and identifies:
//   - function_definition (functions and methods)
//   - class_definition (classes)
//   - decorated_definition (decorated functions/classes)
func (p *PythonParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = p.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
func (p *PythonParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "function_definition":
		symbol := p.extractFunction(node, source, parentName)
		symbols = append(symbols, symbol)
		// Recursively process nested functions (closures)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "block" {
				symbols = p.walkNode(child, source, symbol.Name, symbols)
			}
		}
	case "class_definition":
		symbol := p.extractClass(node, source)
		symbols = append(symbols, symbol)
		// Recursively process class body for methods
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "block" {
				symbols = p.walkNode(child, source, symbol.Name, symbols)
			}
		}
	case "decorated_definition":
		// Handle decorated functions/classes (e.g., @property, @staticmethod)
		symbols = p.walkNode(node, source, parentName, symbols)
	default:
		// Recursively process child nodes
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = p.walkNode(child, source, parentName, symbols)
		}
	}

	return symbols
}

// extractFunction extracts a function or method definition.
// Example: def add(a, b): return a + b
func (p *PythonParser) extractFunction(node *sitter.Node, source []byte, parentName string) Symbol {
	name := node.ChildByFieldName("name")
	nameStr := "anonymous"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	// Extract parameters
	params := node.ChildByFieldName("parameters")
	signature := ""
	if params != nil {
		signature = params.Utf8Text(source)
	}

	// Extract docstring (first string literal in function body)
	docString := p.extractDocstring(node, source)

	// Determine if this is a method (has 'self' or 'cls' as first parameter)
	kind := SymbolFunction
	if p.isMethod(params, source) {
		kind = SymbolMethod
	}

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
		Visibility: p.determineVisibility(nameStr),
		DocString:  docString,
	}
}

// extractClass extracts a class definition.
// Example: class MyClass: pass
func (p *PythonParser) extractClass(node *sitter.Node, source []byte) Symbol {
	name := node.ChildByFieldName("name")
	nameStr := "AnonymousClass"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	// Extract superclasses
	superclasses := node.ChildByFieldName("superclasses")
	signature := ""
	if superclasses != nil {
		signature = superclasses.Utf8Text(source)
	}

	// Extract docstring
	docString := p.extractDocstring(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolClass,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Visibility: p.determineVisibility(nameStr),
		DocString:  docString,
	}
}

// ExtractImports extracts all import statements from Python code.
// Handles both "import x" and "from x import y" syntax.
func (p *PythonParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	imports = p.walkImports(rootNode, source, imports)

	return imports, nil
}

// walkImports recursively finds all import statements.
func (p *PythonParser) walkImports(node *sitter.Node, source []byte, imports []string) []string {
	nodeType := node.Kind()

	if nodeType == "import_statement" {
		// import x, y, z
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "dotted_name" || child.Kind() == "identifier" {
				imports = append(imports, child.Utf8Text(source))
			}
		}
	} else if nodeType == "import_from_statement" {
		// from x import y
		moduleName := node.ChildByFieldName("module_name")
		if moduleName != nil {
			imports = append(imports, moduleName.Utf8Text(source))
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = p.walkImports(child, source, imports)
	}

	return imports
}

// Helper functions

// extractDocstring extracts the docstring from a function or class.
// The docstring is the first expression statement that contains a string literal.
func (p *PythonParser) extractDocstring(node *sitter.Node, source []byte) string {
	// Find the body/block of the function/class
	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}

	// The first child of the body should be an expression_statement with a string
	if body.ChildCount() > 0 {
		firstChild := body.Child(0)
		if firstChild.Kind() == "expression_statement" {
			// Check if it contains a string
			for i := uint(0); i < firstChild.ChildCount(); i++ {
				child := firstChild.Child(i)
				if child.Kind() == "string" {
					// Remove quotes and clean up the docstring
					docStr := child.Utf8Text(source)
					docStr = strings.Trim(docStr, `"'`)
					docStr = strings.TrimPrefix(docStr, `"""`)
					docStr = strings.TrimSuffix(docStr, `"""`)
					docStr = strings.TrimPrefix(docStr, `'''`)
					docStr = strings.TrimSuffix(docStr, `'''`)
					return strings.TrimSpace(docStr)
				}
			}
		}
	}

	return ""
}

// isMethod checks if a function is a method by examining its parameters.
// A function is a method if its first parameter is 'self' or 'cls'.
func (p *PythonParser) isMethod(params *sitter.Node, source []byte) bool {
	if params == nil || params.ChildCount() == 0 {
		return false
	}

	// Find the first parameter
	for i := uint(0); i < params.ChildCount(); i++ {
		child := params.Child(i)
		if child.Kind() == "identifier" {
			firstParam := child.Utf8Text(source)
			return firstParam == "self" || firstParam == "cls"
		}
	}

	return false
}

// determineVisibility determines if a symbol is public or private.
// In Python, symbols starting with underscore are considered private.
func (p *PythonParser) determineVisibility(name string) string {
	if len(name) == 0 {
		return "public"
	}
	if strings.HasPrefix(name, "_") {
		if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
			return "private" // Name mangling
		}
		return "protected" // Single underscore
	}
	return "public"
}
