/*
  File: simple_test.go
  Purpose: Simple test to verify tree-sitter bindings work correctly.
  Author: CodeTextor project
*/

package chunker

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTreeSitterBasic tests that tree-sitter Go binding works.
func TestTreeSitterBasic(t *testing.T) {
	// Get Go language
	lang := sitter.NewLanguage(tree_sitter_go.Language())
	require.NotNil(t, lang, "Go language should not be nil")

	// Create parser
	parser := sitter.NewParser()
	require.NotNil(t, parser, "Parser should not be nil")
	defer parser.Close()

	// Set language
	err := parser.SetLanguage(lang)
	require.NoError(t, err, "Should set language without error")

	// Parse simple Go code
	source := []byte("package main")
	tree := parser.Parse(source, nil)
	require.NotNil(t, tree, "Tree should not be nil")
	defer tree.Close()

	// Verify tree has root node
	rootNode := tree.RootNode()
	require.NotNil(t, rootNode)
	assert.Equal(t, "source_file", rootNode.Kind())
}
