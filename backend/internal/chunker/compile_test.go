package chunker

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"embed"
)

//go:embed parsers/default/*.toml
var testEmbedFS embed.FS

func TestCompileAllDefaultQueries(t *testing.T) {
	configs, err := LoadDefaultConfigs(testEmbedFS)
	assert.NoError(t, err)

	for name, config := range configs {
		t.Run(name, func(t *testing.T) {
			qp, err := NewQueryParser(config)
			if err != nil {
				t.Fatalf("Failed to create QueryParser for %s: %v", name, err)
			}
			assert.NotNil(t, qp)
		})
	}
}
