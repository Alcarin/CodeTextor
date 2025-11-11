/*
  File: typescript_parser.go
  Purpose: Tree-sitter parser implementation for TypeScript and JavaScript.
  Author: CodeTextor project
  Notes: Extracts functions, classes, methods, arrow functions, and imports from TS/JS code.
*/

package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// TypeScriptParser implements the LanguageParser interface for TypeScript and JavaScript.
type TypeScriptParser struct {
	isTypeScript bool // true for .ts/.tsx, false for .js/.jsx
}

// GetLanguage returns the tree-sitter Language for TypeScript or JavaScript.
// Note: TypeScript grammar is a superset of JavaScript.
func (t *TypeScriptParser) GetLanguage() *sitter.Language {
	if t.isTypeScript {
		return sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	}
	return sitter.NewLanguage(tree_sitter_javascript.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (t *TypeScriptParser) GetFileExtensions() []string {
	return []string{".ts", ".tsx", ".js", ".jsx"}
}

// ExtractSymbols extracts all symbols from TypeScript/JavaScript code.
// Handles:
//   - function_declaration (named functions)
//   - arrow_function (arrow functions)
//   - method_definition (class methods)
//   - class_declaration (classes)
//   - lexical_declaration (const/let)
//   - variable_declaration (var)
func (t *TypeScriptParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = t.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
func (t *TypeScriptParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "function_declaration", "function":
		symbols = append(symbols, t.extractFunction(node, source, parentName))
	case "class_declaration":
		symbol := t.extractClass(node, source)
		symbols = append(symbols, symbol)
		// Process class body for methods
		body := node.ChildByFieldName("body")
		if body != nil {
			symbols = t.walkNode(body, source, symbol.Name, symbols)
		}
	case "method_definition":
		symbols = append(symbols, t.extractMethod(node, source, parentName))
	case "lexical_declaration", "variable_declaration":
		// Check if this is a function assigned to a variable (const foo = () => {})
		symbols = append(symbols, t.extractVariableDeclaration(node, source, parentName)...)
	case "export_statement":
		// Process exported symbols
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = t.walkNode(child, source, parentName, symbols)
		}
		return symbols
	}

	// Recursively process child nodes (except for nodes we've already processed)
	if nodeType != "class_declaration" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = t.walkNode(child, source, parentName, symbols)
		}
	}

	return symbols
}

// extractFunction extracts a function declaration.
// Example: function add(a, b) { return a + b; }
func (t *TypeScriptParser) extractFunction(node *sitter.Node, source []byte, parentName string) Symbol {
	name := node.ChildByFieldName("name")
	nameStr := "anonymous"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	signature := t.extractSignature(node, source)
	docString := t.extractJSDoc(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolFunction,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Parent:     parentName,
		Visibility: "public", // JavaScript doesn't have built-in visibility
		DocString:  docString,
	}
}

// extractClass extracts a class declaration.
// Example: class MyClass extends BaseClass { }
func (t *TypeScriptParser) extractClass(node *sitter.Node, source []byte) Symbol {
	name := node.ChildByFieldName("name")
	nameStr := "AnonymousClass"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	// Extract heritage (extends clause)
	heritage := node.ChildByFieldName("heritage")
	signature := ""
	if heritage != nil {
		signature = heritage.Utf8Text(source)
	}

	docString := t.extractJSDoc(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolClass,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Visibility: "public",
		DocString:  docString,
	}
}

// extractMethod extracts a class method.
// Example: myMethod(arg) { return arg * 2; }
func (t *TypeScriptParser) extractMethod(node *sitter.Node, source []byte, parentName string) Symbol {
	name := node.ChildByFieldName("name")
	nameStr := "anonymous"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	signature := t.extractSignature(node, source)
	docString := t.extractJSDoc(node, source)

	// Determine visibility from modifiers (TypeScript only)
	visibility := t.extractVisibility(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolMethod,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Parent:     parentName,
		Visibility: visibility,
		DocString:  docString,
	}
}

// extractVariableDeclaration extracts variable declarations, particularly those assigned to functions.
// Example: const add = (a, b) => a + b;
func (t *TypeScriptParser) extractVariableDeclaration(node *sitter.Node, source []byte, parentName string) []Symbol {
	var symbols []Symbol

	// Look for variable_declarator nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "variable_declarator" {
			name := child.ChildByFieldName("name")
			value := child.ChildByFieldName("value")

			if name != nil && value != nil {
				// Check if the value is a function (arrow_function or function)
				if value.Kind() == "arrow_function" || value.Kind() == "function" {
					nameStr := name.Utf8Text(source)
					signature := t.extractSignature(value, source)
					docString := t.extractJSDoc(node, source)

					symbols = append(symbols, Symbol{
						Name:       nameStr,
						Kind:       SymbolFunction,
						StartLine:  uint32(child.StartPosition().Row) + 1,
						EndLine:    uint32(child.EndPosition().Row) + 1,
						StartByte:  uint32(child.StartByte()),
						EndByte:    uint32(child.EndByte()),
						Source:     child.Utf8Text(source),
						Signature:  signature,
						Parent:     parentName,
						Visibility: "public",
						DocString:  docString,
					})
				}
			}
		}
	}

	return symbols
}

// ExtractImports extracts all import statements.
// Handles: import, import from, require()
func (t *TypeScriptParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	imports = t.walkImports(rootNode, source, imports)

	return imports, nil
}

// walkImports recursively finds all import statements.
func (t *TypeScriptParser) walkImports(node *sitter.Node, source []byte, imports []string) []string {
	nodeType := node.Kind()

	if nodeType == "import_statement" {
		// import foo from 'module' or import 'module'
		sourceNode := node.ChildByFieldName("source")
		if sourceNode != nil {
			importPath := strings.Trim(sourceNode.Utf8Text(source), `"'`)
			imports = append(imports, importPath)
		}
	} else if nodeType == "call_expression" {
		// require('module')
		function := node.ChildByFieldName("function")
		if function != nil && function.Utf8Text(source) == "require" {
			args := node.ChildByFieldName("arguments")
			if args != nil && args.ChildCount() > 1 {
				// Get first argument (the module string)
				arg := args.Child(1) // Skip opening paren
				if arg.Kind() == "string" {
					importPath := strings.Trim(arg.Utf8Text(source), `"'`)
					imports = append(imports, importPath)
				}
			}
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = t.walkImports(child, source, imports)
	}

	return imports
}

// Helper functions

// extractSignature extracts function/method signature (parameters and return type).
func (t *TypeScriptParser) extractSignature(node *sitter.Node, source []byte) string {
	params := node.ChildByFieldName("parameters")
	returnType := node.ChildByFieldName("return_type")

	var sig strings.Builder
	if params != nil {
		sig.WriteString(params.Utf8Text(source))
	}
	if returnType != nil {
		sig.WriteString(": ")
		sig.WriteString(returnType.Utf8Text(source))
	}

	return sig.String()
}

// extractJSDoc extracts JSDoc comment preceding a node.
// Example: /** This is a JSDoc comment */
func (t *TypeScriptParser) extractJSDoc(node *sitter.Node, source []byte) string {
	// Look for comment nodes before this node
	// This is simplified; a full implementation would parse JSDoc properly
	startByte := node.StartByte()
	if startByte == 0 {
		return ""
	}

	// Look backwards for /** ... */ or // comments
	lines := strings.Split(string(source[:startByte]), "\n")
	var docLines []string

	for i := len(lines) - 2; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "//") {
			docLines = append([]string{strings.TrimPrefix(line, "//")}, docLines...)
		} else if strings.Contains(line, "*/") {
			// Start of JSDoc block, collect until we find /**
			jsdoc := line
			for j := i - 1; j >= 0; j-- {
				prevLine := strings.TrimSpace(lines[j])
				jsdoc = prevLine + "\n" + jsdoc
				if strings.Contains(prevLine, "/**") {
					// Clean up JSDoc
					jsdoc = strings.ReplaceAll(jsdoc, "/**", "")
					jsdoc = strings.ReplaceAll(jsdoc, "*/", "")
					jsdoc = strings.ReplaceAll(jsdoc, "*", "")
					return strings.TrimSpace(jsdoc)
				}
			}
			break
		} else if line == "" {
			continue
		} else {
			break
		}
	}

	return strings.TrimSpace(strings.Join(docLines, "\n"))
}

// extractVisibility extracts visibility modifiers (TypeScript only).
// Example: private myMethod() { }
func (t *TypeScriptParser) extractVisibility(node *sitter.Node, source []byte) string {
	// Look for accessibility_modifier in TypeScript
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "accessibility_modifier" {
			return child.Utf8Text(source)
		}
	}
	return "public"
}
