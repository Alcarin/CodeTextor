package chunker

import (
	"fmt"
	"io/ioutil"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHybridParsing(t *testing.T) {
	filePath := "testdata/test_mixed.php"
	source, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	config := DefaultChunkConfig()
	parser := NewParser(config)

	// In NewParser, the parsers map is initialized and SubLanguageManager is injected.
	res, err := parser.ParseFile(filePath, source)
	assert.NoError(t, err)

	fmt.Printf("\n--- Extracted Symbols for %s ---\n", filePath)
	for i, sym := range res.Symbols {
		fmt.Printf("[%d] Kind: %s | Name: %s | Parent: %s | Lines: %d-%d\n", 
			i, sym.Kind, sym.Name, sym.Parent, sym.StartLine, sym.EndLine)
	}
	fmt.Println("---------------------------------------")

	// Verify PHP function was found
	foundPHP := false
	for _, sym := range res.Symbols {
		if sym.Name == "test_function" && sym.Kind == SymbolFunction {
			foundPHP = true
			break
		}
	}
	assert.True(t, foundPHP, "Should have found PHP test_function")

	// We can also check if HTML/JS was detected and extracted
	// Given the hybrid parser, the text blocks containing HTML should have been passed to HTML parser
	// Let's assert that there is at least one symbol that is not PHP
	foundNonPHP := false
	for _, sym := range res.Symbols {
		if sym.Kind == SymbolElement || sym.Kind == SymbolScript {
			foundNonPHP = true
			break
		}
	}
	assert.True(t, foundNonPHP, "Should have found HTML elements or script block from hybrid parsing")
}
