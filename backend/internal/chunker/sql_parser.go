/*
  File: sql_parser.go
  Purpose: Tree-sitter parser implementation for SQL scripts.
  Author: CodeTextor project
  Notes: Extracts statements (DDL/DML) and exposes them as structured symbols.
*/

package chunker

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_sql "github.com/DerekStride/tree-sitter-sql/bindings/go"
)

// SQLParser implements the LanguageParser interface for SQL files.
type SQLParser struct{}

// GetLanguage returns the tree-sitter Language for SQL.
func (s *SQLParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_sql.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (s *SQLParser) GetFileExtensions() []string {
	return []string{".sql"}
}

// ExtractSymbols walks the SQL AST and builds a symbol for each statement.
func (s *SQLParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol
	root := tree.RootNode()
	symbols = s.walkNode(root, source, symbols, "")
	return symbols, nil
}

// walkNode recursively visits AST nodes and records relevant statements.
func (s *SQLParser) walkNode(node *sitter.Node, source []byte, symbols []Symbol, parent string) []Symbol {
	if node == nil {
		return symbols
	}

	switch node.Kind() {
	case "transaction", "block":
		name := strings.ToUpper(node.Kind())
		sym := s.makeSymbol(node, source, name, parent)
		symbols = append(symbols, sym)

		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			symbols = s.walkNode(child, source, symbols, sym.Name)
		}
		return symbols
	case "statement":
		// Handle statement wrapper node - extract the actual statement from children
		// Special case: SELECT statements may be split into "select" and "from" nodes
		var selectNode *sitter.Node
		var otherStatements []*sitter.Node

		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child.Kind() == "select" {
				selectNode = child
			} else if s.isStatement(child.Kind()) {
				otherStatements = append(otherStatements, child)
			}
		}

		// If we found both select and from, combine them into one symbol
		if selectNode != nil {
			// Use the statement node itself to get the full text including FROM
			sym := s.makeSymbol(node, source, s.statementName(selectNode, source), parent)
			symbols = append(symbols, sym)
		} else {
			// Handle other statement types
			for _, stmt := range otherStatements {
				sym := s.makeSymbol(stmt, source, s.statementName(stmt, source), parent)
				symbols = append(symbols, sym)
			}
		}
		return symbols
	}

	if s.isStatement(node.Kind()) {
		sym := s.makeSymbol(node, source, s.statementName(node, source), parent)
		symbols = append(symbols, sym)
	}

	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		symbols = s.walkNode(child, source, symbols, parent)
	}

	return symbols
}

// statementName formats a readable name for each SQL statement.
func (s *SQLParser) statementName(node *sitter.Node, source []byte) string {
	object := s.objectReferenceName(node, source)
	switch node.Kind() {
	case "create_table":
		return s.buildName("CREATE TABLE", object)
	case "create_view":
		return s.buildName("CREATE VIEW", object)
	case "create_materialized_view":
		return s.buildName("CREATE MATERIALIZED VIEW", object)
	case "create_index":
		return s.buildName("CREATE INDEX", object)
	case "create_database":
		return s.buildName("CREATE DATABASE", object)
	case "create_schema":
		return s.buildName("CREATE SCHEMA", object)
	case "create_function":
		return s.buildName("CREATE FUNCTION", object)
	case "create_type":
		return s.buildName("CREATE TYPE", object)
	case "create_role":
		return s.buildName("CREATE ROLE", object)
	case "create_sequence":
		return s.buildName("CREATE SEQUENCE", object)
	case "create_extension":
		return s.buildName("CREATE EXTENSION", object)
	case "create_trigger":
		return s.buildName("CREATE TRIGGER", object)
	case "drop_table":
		return s.buildName("DROP TABLE", object)
	case "drop_view":
		return s.buildName("DROP VIEW", object)
	case "drop_index":
		return s.buildName("DROP INDEX", object)
	case "drop_type":
		return s.buildName("DROP TYPE", object)
	case "drop_schema":
		return s.buildName("DROP SCHEMA", object)
	case "drop_database":
		return s.buildName("DROP DATABASE", object)
	case "drop_role":
		return s.buildName("DROP ROLE", object)
	case "drop_sequence":
		return s.buildName("DROP SEQUENCE", object)
	case "drop_extension":
		return s.buildName("DROP EXTENSION", object)
	case "drop_function":
		return s.buildName("DROP FUNCTION", object)
	case "alter_table":
		return s.buildName("ALTER TABLE", object)
	case "select_statement", "select":
		return s.buildName("SELECT", object)
	case "insert_statement", "insert":
		return s.buildName("INSERT", object)
	case "update_statement", "update":
		return s.buildName("UPDATE", object)
	case "delete_statement", "delete":
		return s.buildName("DELETE", object)
	case "merge_statement", "merge":
		return s.buildName("MERGE", object)
	case "truncate_statement", "truncate":
		return s.buildName("TRUNCATE", object)
	case "copy_statement", "copy":
		return s.buildName("COPY", object)
	}

	return strings.ToUpper(node.Kind())
}

// makeSymbol builds a symbol record for the node.
func (s *SQLParser) makeSymbol(node *sitter.Node, source []byte, name, parent string) Symbol {
	text := strings.TrimSpace(node.Utf8Text(source))
	if name == "" {
		name = strings.ToUpper(node.Kind())
	}

	return Symbol{
		Name:       name,
		Kind:       SymbolSQLStatement,
		StartLine:  uint32(node.StartPosition().Row) + 1,
		EndLine:    uint32(node.EndPosition().Row) + 1,
		StartByte:  uint32(node.StartByte()),
		EndByte:    uint32(node.EndByte()),
		Source:     node.Utf8Text(source),
		Signature:  text,
		Visibility: "public",
		Parent:     parent,
	}
}

// buildName combines a keyword with an optional object reference.
func (s *SQLParser) buildName(prefix, object string) string {
	if object == "" {
		return prefix
	}
	return fmt.Sprintf("%s %s", prefix, object)
}

// objectReferenceName finds the first object_reference node and returns its text.
func (s *SQLParser) objectReferenceName(node *sitter.Node, source []byte) string {
	// For SELECT statements, look for the FROM clause in parent's siblings
	if node.Kind() == "select" && node.Parent() != nil {
		parent := node.Parent()
		for i := uint(0); i < parent.NamedChildCount(); i++ {
			child := parent.NamedChild(i)
			if child.Kind() == "from" {
				if ref := findObjectReference(child); ref != nil {
					return strings.TrimSpace(ref.Utf8Text(source))
				}
			}
		}
	}

	if ref := findObjectReference(node); ref != nil {
		return strings.TrimSpace(ref.Utf8Text(source))
	}
	return ""
}

// findObjectReference searches for the first object_reference descendant.
func findObjectReference(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Kind() == "object_reference" {
		return node
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if child := findObjectReference(node.NamedChild(i)); child != nil {
			return child
		}
	}
	return nil
}

// isStatement reports whether the node kind represents a SQL statement worth capturing.
func (s *SQLParser) isStatement(kind string) bool {
	switch kind {
	case "create_table",
		"create_view",
		"create_materialized_view",
		"create_index",
		"create_database",
		"create_schema",
		"create_function",
		"create_type",
		"create_role",
		"create_sequence",
		"create_extension",
		"create_trigger",
		"drop_table",
		"drop_view",
		"drop_index",
		"drop_type",
		"drop_schema",
		"drop_database",
		"drop_role",
		"drop_sequence",
		"drop_extension",
		"drop_function",
		"alter_table",
		"select_statement",
		"insert_statement",
		"update_statement",
		"delete_statement",
		"merge_statement",
		"truncate_statement",
		"copy_statement",
		// DerekStride grammar node names
		"select",
		"insert",
		"update",
		"delete",
		"merge",
		"truncate",
		"copy":
		return true
	}
	return false
}

// ExtractImports returns an empty list because SQL does not declare imports.
func (s *SQLParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	return []string{}, nil
}
