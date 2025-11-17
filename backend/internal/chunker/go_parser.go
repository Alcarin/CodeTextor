/*
  File: go_parser.go
  Purpose: Tree-sitter parser implementation for the Go programming language.
  Author: CodeTextor project
  Notes: Extracts functions, methods, types, structs, interfaces, and imports from Go code.
*/

package chunker

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GoParser implements the LanguageParser interface for Go source code.
type GoParser struct{}

// GetLanguage returns the tree-sitter Language for Go.
func (g *GoParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_go.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (g *GoParser) GetFileExtensions() []string {
	return []string{".go"}
}

// ExtractSymbols extracts all symbols (functions, methods, types, etc.) from Go code.
// It walks the AST and identifies:
//   - function_declaration (top-level functions)
//   - method_declaration (methods on types)
//   - type_declaration (type aliases, structs, interfaces)
//   - const_declaration, var_declaration (constants and variables)
func (g *GoParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	rootNode := tree.RootNode()

	// Walk the AST and extract symbols
	symbols = g.walkNode(rootNode, source, "", symbols)

	return symbols, nil
}

// walkNode recursively walks the AST and extracts symbols.
// Parameters:
//   - node: Current AST node being processed
//   - source: Original source code
//   - parentName: Name of the parent symbol (for nested symbols)
//   - symbols: Accumulated list of symbols
func (g *GoParser) walkNode(node *sitter.Node, source []byte, parentName string, symbols []Symbol) []Symbol {
	nodeType := node.Kind()

	switch nodeType {
	case "function_declaration":
		fnSymbol := g.extractFunction(node, source, parentName)
		symbols = append(symbols, fnSymbol)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = g.walkNode(child, source, fnSymbol.Name, symbols)
		}
		return symbols
	case "method_declaration":
		methodSymbol := g.extractMethod(node, source)
		symbols = append(symbols, methodSymbol)
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			symbols = g.walkNode(child, source, methodSymbol.Name, symbols)
		}
		return symbols
	case "type_declaration":
		symbols = append(symbols, g.extractTypeDeclaration(node, source)...)
	case "const_declaration", "var_declaration":
		symbols = append(symbols, g.extractVariableDeclaration(node, source, nodeType, parentName)...)
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		symbols = g.walkNode(child, source, parentName, symbols)
	}

	return symbols
}

// extractFunction extracts a function declaration.
// Example: func Add(a, b int) int { return a + b }
func (g *GoParser) extractFunction(node *sitter.Node, source []byte, parentName string) Symbol {
	name := g.findChildByType(node, "identifier")
	nameStr := "anonymous"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	signature := g.extractSignature(node, source)
	docString := g.extractLeadingComment(node, source)

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
		Visibility: g.determineVisibility(nameStr),
		DocString:  docString,
	}
}

// extractMethod extracts a method declaration.
// Example: func (r *Receiver) Method(arg string) error { ... }
func (g *GoParser) extractMethod(node *sitter.Node, source []byte) Symbol {
	name := g.findChildByType(node, "field_identifier")
	nameStr := "anonymous"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	// Extract receiver type (the struct/type this method is attached to)
	receiver := g.findChildByType(node, "parameter_list")
	receiverType := ""
	if receiver != nil {
		// The receiver is the first parameter
		receiverType = g.extractReceiverType(receiver, source)
	}

	signature := g.extractSignature(node, source)
	docString := g.extractLeadingComment(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       SymbolMethod,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  signature,
		Parent:     receiverType,
		Visibility: g.determineVisibility(nameStr),
		DocString:  docString,
	}
}

// extractTypeDeclaration extracts type declarations (structs, interfaces, type aliases).
// Example: type MyStruct struct { Field int }
func (g *GoParser) extractTypeDeclaration(node *sitter.Node, source []byte) []Symbol {
	var symbols []Symbol

	// A type_declaration can contain multiple type specs
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "type_spec" {
			symbol := g.extractTypeSpec(child, source)
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// extractTypeSpec extracts a single type specification.
func (g *GoParser) extractTypeSpec(node *sitter.Node, source []byte) Symbol {
	name := g.findChildByType(node, "type_identifier")
	nameStr := "unknown"
	if name != nil {
		nameStr = name.Utf8Text(source)
	}

	// Determine the kind of type (struct, interface, type alias)
	kind := SymbolTypeAlias
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		switch typeNode.Kind() {
		case "struct_type":
			kind = SymbolStruct
		case "interface_type":
			kind = SymbolInterface
		}
	}

	docString := g.extractLeadingComment(node, source)

	return Symbol{
		Name:       nameStr,
		Kind:       kind,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Visibility: g.determineVisibility(nameStr),
		DocString:  docString,
	}
}

// extractVariableDeclaration extracts constant or variable declarations.
// Example: const MaxSize = 100 or var count int
func (g *GoParser) extractVariableDeclaration(node *sitter.Node, source []byte, nodeType string, parentName string) []Symbol {
	var symbols []Symbol

	kind := SymbolVariable
	if nodeType == "const_declaration" {
		kind = SymbolConstant
	}

	// Extract all specs (can have multiple in one declaration)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "const_spec" || child.Kind() == "var_spec" {
			names := g.findAllChildrenByType(child, "identifier")
			for _, name := range names {
				nameStr := name.Utf8Text(source)
				symbols = append(symbols, Symbol{
					Name:       nameStr,
					Kind:       kind,
					StartLine:  uint32(child.StartPosition().Row) + 1,
					EndLine:    uint32(child.EndPosition().Row) + 1,
					StartByte:  uint32(child.StartByte()),
					EndByte:    uint32(child.EndByte()),
					Source:     child.Utf8Text(source),
					Parent:     parentName,
					Visibility: g.determineVisibility(nameStr),
				})
			}
		}
	}

	return symbols
}

// ExtractImports extracts all import statements from Go code.
// Example: import "fmt" or import ( "os"; "io" )
func (g *GoParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	rootNode := tree.RootNode()

	imports = g.walkImports(rootNode, source, imports)

	return imports, nil
}

// walkImports recursively finds all import declarations.
func (g *GoParser) walkImports(node *sitter.Node, source []byte, imports []string) []string {
	if node.Kind() == "import_declaration" {
		// Extract import specs
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "import_spec" {
				// Find the import path (string literal)
				pathNode := g.findChildByType(child, "interpreted_string_literal")
				if pathNode != nil {
					importPath := strings.Trim(pathNode.Utf8Text(source), `"`)
					imports = append(imports, importPath)
				}
			}
		}
	}

	// Recursively process child nodes
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		imports = g.walkImports(child, source, imports)
	}

	return imports
}

// Helper functions

// findChildByType finds the first child node of a specific type.
func (g *GoParser) findChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeType {
			return child
		}
	}
	return nil
}

// findAllChildrenByType finds all child nodes of a specific type.
func (g *GoParser) findAllChildrenByType(node *sitter.Node, nodeType string) []*sitter.Node {
	var nodes []*sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == nodeType {
			nodes = append(nodes, child)
		}
	}
	return nodes
}

// extractSignature extracts the function/method signature (parameters and return type).
func (g *GoParser) extractSignature(node *sitter.Node, source []byte) string {
	// Find parameter list
	params := g.findChildByType(node, "parameter_list")
	result := g.findChildByType(node, "type_identifier")

	var sig strings.Builder
	if params != nil {
		sig.WriteString(params.Utf8Text(source))
	}
	if result != nil {
		sig.WriteString(" ")
		sig.WriteString(result.Utf8Text(source))
	}

	return sig.String()
}

// extractReceiverType extracts the receiver type from a method's parameter list.
// Example: (r *Receiver) -> "Receiver"
func (g *GoParser) extractReceiverType(paramList *sitter.Node, source []byte) string {
	// The receiver is typically the first parameter
	if paramList.ChildCount() > 0 {
		param := paramList.Child(1) // Skip opening paren
		if param.Kind() == "parameter_declaration" {
			typeNode := param.ChildByFieldName("type")
			if typeNode != nil {
				typeStr := typeNode.Utf8Text(source)
				// Remove pointer asterisk if present
				return strings.TrimPrefix(typeStr, "*")
			}
		}
	}
	return ""
}

// extractLeadingComment finds and extracts the comment immediately preceding a node.
// This is typically the documentation comment for a symbol.
func (g *GoParser) extractLeadingComment(node *sitter.Node, source []byte) string {
	// Tree-sitter doesn't include comments in the main AST by default
	// We need to look for comments in the source code just before this node
	startByte := node.StartByte()

	// Look backwards from the node's start position to find comments
	// This is a simplified implementation; a full implementation would
	// parse all comments and associate them with symbols
	lines := strings.Split(string(source[:startByte]), "\n")
	var docLines []string

	// Collect consecutive comment lines immediately before the symbol
	for i := len(lines) - 2; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "//") {
			docLines = append([]string{strings.TrimPrefix(line, "//")}, docLines...)
		} else if line == "" {
			continue // Skip empty lines
		} else {
			break // Stop at first non-comment, non-empty line
		}
	}

	return strings.TrimSpace(strings.Join(docLines, "\n"))
}

// determineVisibility determines if a symbol is exported (public) or unexported (private).
// In Go, a symbol is exported if its name starts with an uppercase letter.
func (g *GoParser) determineVisibility(name string) string {
	if len(name) == 0 {
		return "private"
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return "public"
	}
	return "private"
}
