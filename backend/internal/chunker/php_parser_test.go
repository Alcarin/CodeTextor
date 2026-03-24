package chunker

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhpParser_ExtractSymbols(t *testing.T) {
	parser := &PhpParser{}
	source, err := os.ReadFile("test_example.php")
	require.NoError(t, err)

	tree := sitterNewParser(parser.GetLanguage(), source)
	defer tree.Close()

	symbols, err := parser.ExtractSymbols(tree, source)
	assert.NoError(t, err)

	// Verify symbols
	foundNames := make(map[string]bool)
	for _, s := range symbols {
		foundNames[s.Name] = true
	}

	assert.True(t, foundNames["App\\Services"])   // Namespace
	assert.True(t, foundNames["AuthService"])    // Class
	assert.True(t, foundNames["LoggerInterface"]) // Interface
	assert.True(t, foundNames["HelperTrait"])     // Trait
	assert.True(t, foundNames["global_helper"])   // Function
	assert.True(t, foundNames["login"])           // Method
	assert.True(t, foundNames["log"])             // Method
}

func TestPhpParser_ExtractImports(t *testing.T) {
	parser := &PhpParser{}
	source, err := os.ReadFile("test_example.php")
	require.NoError(t, err)

	tree := sitterNewParser(parser.GetLanguage(), source)
	defer tree.Close()

	imports, err := parser.ExtractImports(tree, source)
	assert.NoError(t, err)

	assert.Contains(t, imports, "App\\Models\\User")
	assert.Contains(t, imports, "App\\Interfaces\\LoggerInterface")
}

// Helper function to create a tree-sitter tree for testing
func sitterNewParser(lang *sitter.Language, source []byte) *sitter.Tree {
	p := sitter.NewParser()
	p.SetLanguage(lang)
	return p.Parse(source, nil)
}
