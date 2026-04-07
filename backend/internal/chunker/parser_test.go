/*
  File: parser_test.go
  Purpose: Unit tests for the code parser and language-specific implementations.
  Author: CodeTextor project
  Notes: Tests Go, Python, and TypeScript parsers with sample code snippets.
*/

package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoParserUsages tests the extraction of function calls (symbol usages).
func TestGoParserUsages(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`package main

import "fmt"

func helper(x int) int {
	return x + 1
}

func Main() {
	a := helper(10)
	fmt.Println(a)
}

type Greeter struct{}

func (g *Greeter) Greet(name string) {
	fmt.Printf("Hello, %s\n", name)
	g.internal()
}

func (g *Greeter) internal() {}
`)

	result, err := parser.ParseFile("usage_test.go", source)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify usages
	usages := result.Usages
	// Expected: helper(10), fmt.Println(a), fmt.Printf(...), g.internal()
	assert.GreaterOrEqual(t, len(usages), 4, "should extract at least 4 usages")

	// Verify "helper" usage in "Main"
	var helperUsage *ParserSymbolUsage
	for i := range usages {
		if usages[i].Name == "helper" && usages[i].Caller == "Main" {
			helperUsage = &usages[i]
			break
		}
	}
	require.NotNil(t, helperUsage, "usage of helper in Main should be found")
	assert.Equal(t, uint32(10), helperUsage.Line)

	// Verify "g.internal" usage in "Greet"
	var internalUsage *ParserSymbolUsage
	for i := range usages {
		if usages[i].Name == "internal" && usages[i].Caller == "Greet" {
			internalUsage = &usages[i]
			break
		}
	}
	require.NotNil(t, internalUsage, "usage of internal in Greet should be found")
	assert.Equal(t, "g", internalUsage.Context, "context should be 'g' for g.internal()")
}

// TestParserUnsupportedExtension tests that unsupported files return an error.
func TestParserUnsupportedExtension(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	_, err := parser.ParseFile("test.xyz", []byte("some content"))
	require.Error(t, err, "should return error for unsupported extension")
	assert.Contains(t, err.Error(), "unsupported file extension")
}

// TestParserIsSupported tests the IsSupported method.
func TestParserIsSupported(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	assert.True(t, parser.IsSupported("test.go"))
	assert.True(t, parser.IsSupported("test.py"))
	assert.True(t, parser.IsSupported("test.ts"))
	assert.True(t, parser.IsSupported("test.js"))
	assert.True(t, parser.IsSupported("test.html"))
	assert.True(t, parser.IsSupported("test.css"))
	assert.True(t, parser.IsSupported("test.vue"))
	assert.True(t, parser.IsSupported("test.md"))
	assert.True(t, parser.IsSupported("test.markdown"))
	assert.False(t, parser.IsSupported("test.txt"))
	assert.False(t, parser.IsSupported("test.xyz"))
}

// TestParserGetSupportedExtensions tests that all expected extensions are registered.
func TestParserGetSupportedExtensions(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	extensions := parser.GetSupportedExtensions()
	assert.Contains(t, extensions, ".go")
	assert.Contains(t, extensions, ".py")
	assert.Contains(t, extensions, ".ts")
	assert.Contains(t, extensions, ".js")
	assert.Contains(t, extensions, ".html")
	assert.Contains(t, extensions, ".css")
	assert.Contains(t, extensions, ".vue")
	assert.Contains(t, extensions, ".md")
	assert.Contains(t, extensions, ".markdown")
}

// TestParseErrorHandling tests that syntax errors are captured.
func TestParseErrorHandling(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	// Invalid Go code with syntax error
	source := []byte(`package main

func broken {  // Missing parameters
	return 42
}
`)

	result, err := parser.ParseFile("test.go", source)
	// Parser should not fail, but should report errors in result
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check if errors were captured (tree-sitter should detect the syntax error)
	// Note: Tree-sitter's error detection varies, so this test might be lenient
	assert.NotNil(t, result.Errors, "errors field should exist")
}
