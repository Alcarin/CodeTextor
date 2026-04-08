package chunker

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestMarkdownInMarkdownDeadlock(t *testing.T) {
	// Questo test verifica che un file Markdown contenente un blocco di codice Markdown
	// non causi un deadlock. Prima della modifica, aware.Lock() avrebbe bloccato
	// l'esecuzione all'interno di BatchExtractAll.
	
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`# Heading

Outer markdown content.

` + "```markdown" + `
## Sub heading
Recursive markdown content.
` + "```" + `
`)

	// Se c'è un deadlock, questo metodo non tornerà mai.
	result, err := parser.ParseFile("deadlock_test.md", source)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Verifica che i simboli del markdown interno siano stati estratti
	foundSubHeading := false
	for _, sym := range result.Symbols {
		if sym.Name == "Sub heading" && sym.Kind == SymbolMarkdownHeading {
			foundSubHeading = true
			break
		}
	}
	
	require.True(t, foundSubHeading, "Dovrebbe trovare l'heading nel markdown annidato")
}
