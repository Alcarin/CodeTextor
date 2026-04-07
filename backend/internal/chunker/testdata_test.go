package chunker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// symbolSpec describes a symbol expected in a parsed file.
type symbolSpec struct {
	Name       string   `json:"name"`
	Kind       SymbolKind `json:"kind"`
	Parent     string   `json:"parent,omitempty"`     // optional: expected parent
	Visibility string   `json:"visibility,omitempty"` // optional: expected visibility
	Signature  string   `json:"signature,omitempty"`  // optional: expected signature (substring match)
	Implements []string `json:"implements,omitempty"` // optional: expected implements list
}

// todoSpec describes an expected TODO symbol.
type todoSpec struct {
	Contains string `json:"contains"` // substring that must appear in the TODO name
}

// usageSpec describes an expected function/method call usage.
type usageSpec struct {
	Name    string `json:"name"`
	Context string `json:"context,omitempty"` // receiver or package name
}

// importSpec is just a string expected in result.Imports.
type importSpec = string

// testdataCase defines a complete test case for a single language file.
type testdataCase struct {
	Name string `json:"name"`
	File string `json:"file"` // relative to testdata/

	// ---------- symbols ----------
	MinSymbols     int                `json:"minSymbols,omitempty"`     // minimum number of symbols expected
	ExpectSymbols  []symbolSpec      `json:"expectSymbols,omitempty"`  // symbols that MUST be present
	AbsentSymbols  []string           `json:"absentSymbols,omitempty"`  // symbol names that must NOT be present
	SymbolKindSets map[SymbolKind]int `json:"symbolKindSets,omitempty"` // minimum count per kind

	// ---------- imports ----------
	ExpectImports []importSpec `json:"expectImports,omitempty"`
	AbsentImports []string     `json:"absentImports,omitempty"`

	// ---------- usages ----------
	MinUsages    int         `json:"minUsages,omitempty"`
	ExpectUsages []usageSpec `json:"expectUsages,omitempty"`

	// ---------- todos ----------
	ExpectTodos []todoSpec `json:"expectTodos,omitempty"`

	// ---------- metadata ----------
	Language       string            `json:"language,omitempty"`
	ExpectMetadata map[string]string `json:"expectMetadata,omitempty"`
}

func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "failed to read testdata/%s", name)
	return data
}

func findSymbol(symbols []Symbol, name string) *Symbol {
	for i := range symbols {
		if symbols[i].Name == name {
			return &symbols[i]
		}
	}
	return nil
}

func findSymbolByKind(symbols []Symbol, kind SymbolKind) []*Symbol {
	var matched []*Symbol
	for i := range symbols {
		if symbols[i].Kind == kind {
			matched = append(matched, &symbols[i])
		}
	}
	return matched
}

// TestTestdataParsers runs comprehensive tests for each language using external fixture files.
func TestTestdataParsers(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	// Discover all .json files in testdata folder
	files, err := os.ReadDir("testdata")
	require.NoError(t, err, "failed to read testdata directory")

	var cases []testdataCase
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") || f.Name() == "sample.json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join("testdata", f.Name()))
		require.NoError(t, err, "failed to read %s", f.Name())

		var tc testdataCase
		err = json.Unmarshal(data, &tc)
		require.NoError(t, err, "failed to unmarshal %s", f.Name())

		if tc.Name == "" {
			tc.Name = f.Name()
		}

		cases = append(cases, tc)
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			source := loadTestdata(t, tc.File)

			result, err := parser.ParseFile("test"+filepath.Ext(tc.File), source)
			require.NoError(t, err, "parsing %s should not fail", tc.File)
			require.NotNil(t, result)


			// ─── Language ──────────────────────────────────
			if tc.Language != "" {
				assert.Equal(t, tc.Language, result.Language, "language mismatch")
			}

			// ─── Minimum Symbols ───────────────────────────
			if tc.MinSymbols > 0 {
				assert.GreaterOrEqual(t, len(result.Symbols), tc.MinSymbols,
					"expected at least %d symbols, got %d", tc.MinSymbols, len(result.Symbols))
			}

			// ─── Expected Symbols ──────────────────────────
			for _, spec := range tc.ExpectSymbols {
				sym := findSymbol(result.Symbols, spec.Name)
				if !assert.NotNilf(t, sym, "symbol %q should be present", spec.Name) {
					continue
				}
				assert.Equal(t, spec.Kind, sym.Kind, "symbol %q kind mismatch", spec.Name)

				if spec.Parent != "" {
					assert.Equal(t, spec.Parent, sym.Parent, "symbol %q parent mismatch", spec.Name)
				}
				if spec.Visibility != "" {
					assert.Equal(t, spec.Visibility, sym.Visibility, "symbol %q visibility mismatch", spec.Name)
				}
				if spec.Signature != "" {
					assert.Contains(t, sym.Signature, spec.Signature, "symbol %q signature mismatch", spec.Name)
				}
				if len(spec.Implements) > 0 {
					for _, iface := range spec.Implements {
						assert.Contains(t, sym.Implements, iface, "symbol %q should implement %s", spec.Name, iface)
					}
				}

				// Verify line numbers are set
				assert.Greater(t, sym.StartLine, uint32(0), "symbol %q should have StartLine > 0", spec.Name)
				assert.GreaterOrEqual(t, sym.EndLine, sym.StartLine, "symbol %q EndLine >= StartLine", spec.Name)
			}

			// ─── Absent Symbols ────────────────────────────
			for _, name := range tc.AbsentSymbols {
				sym := findSymbol(result.Symbols, name)
				assert.Nilf(t, sym, "symbol %q should NOT be present", name)
			}

			// ─── Symbol Kind Counts ────────────────────────
			for kind, minCount := range tc.SymbolKindSets {
				matched := findSymbolByKind(result.Symbols, kind)
				assert.GreaterOrEqualf(t, len(matched), minCount,
					"expected at least %d symbols of kind %s, got %d", minCount, kind, len(matched))
			}

			// ─── Imports ───────────────────────────────────
			for _, imp := range tc.ExpectImports {
				assert.Contains(t, result.Imports, imp, "import %q should be present", imp)
			}
			for _, imp := range tc.AbsentImports {
				assert.NotContains(t, result.Imports, imp, "import %q should NOT be present", imp)
			}

			// ─── Usages ────────────────────────────────────
			if tc.MinUsages > 0 {
				assert.GreaterOrEqual(t, len(result.Usages), tc.MinUsages,
					"expected at least %d usages, got %d", tc.MinUsages, len(result.Usages))
			}
			for _, uSpec := range tc.ExpectUsages {
				found := false
				for _, u := range result.Usages {
					if u.Name == uSpec.Name {
						if uSpec.Context == "" || u.Context == uSpec.Context {
							found = true
							break
						}
					}
				}
				assert.Truef(t, found, "usage %q (context=%q) should be present", uSpec.Name, uSpec.Context)
			}

			// ─── TODOs ─────────────────────────────────────
			todoSymbols := findSymbolByKind(result.Symbols, SymbolTodo)
			for _, td := range tc.ExpectTodos {
				found := false
				for _, ts := range todoSymbols {
					if assert.ObjectsAreEqual(ts.Kind, SymbolTodo) {
						if contains(ts.Name, td.Contains) {
							found = true
							break
						}
					}
				}
				assert.Truef(t, found, "should have a TODO containing %q", td.Contains)
			}

			// ─── Metadata ──────────────────────────────────
			for key, val := range tc.ExpectMetadata {
				assert.Equal(t, val, result.Metadata[key], "metadata %q mismatch", key)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
