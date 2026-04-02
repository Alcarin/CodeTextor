package chunker

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryCompilationGo(t *testing.T) {
	lang, err := GetGrammar("tree-sitter-go")
	require.NoError(t, err)

	source := `(function_declaration name: (identifier) @name) @symbol.function`
	query, err := sitter.NewQuery(lang, source)
	if !isNil(err) {
		t.Fatalf("Failed to compile simple query: type=%T, value=%#v, msg=%v", err, err, err)
	}
	assert.NotNil(t, query)
}

func TestQueryParserGo(t *testing.T) {
	// Initialize the query parsers (this happens automatically in NewParser(DefaultChunkConfig()))
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`package main

import "fmt"

// Add adds two integers and returns the sum.
func Add(a, b int) int {
	return a + b
}

type Calculator struct {
	Name string
}

func (c *Calculator) Multiply(a, b int) int {
	return a * b
}

const Version = "1.0.0"

func main() {
	res := Add(1, 2)
	fmt.Println(res)
}
`)

	result, err := parser.ParseFile("test.go", source)
	require.NoError(t, err)
	require.NotNil(t, result)

	// The result is now directly provided by the dynamic QueryParser registered for .go files.
	// We test via the main parser to ensure correct integration and TOML config loading.

	// Verify symbols
	symbols := result.Symbols
	
	findSymbol := func(name string, kind SymbolKind) *Symbol {
		for i := range symbols {
			if symbols[i].Name == name && symbols[i].Kind == kind {
				return &symbols[i]
			}
		}
		return nil
	}

	assert.NotNil(t, findSymbol("Add", SymbolFunction))
	assert.NotNil(t, findSymbol("Calculator", SymbolStruct))
	assert.NotNil(t, findSymbol("Multiply", SymbolMethod))
	assert.NotNil(t, findSymbol("Version", SymbolConstant))

	// Verify usages
	usages := result.Usages
	hasUsage := func(name string, caller string) bool {
		for _, u := range usages {
			if u.Name == name && u.Caller == caller {
				return true
			}
		}
		return false
	}

	assert.True(t, hasUsage("Add", "main"), "should find Add usage in main")
	assert.True(t, hasUsage("Println", "main"), "should find Println usage in main")
}
