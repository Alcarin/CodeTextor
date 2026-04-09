package chunker

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_css "github.com/tree-sitter/tree-sitter-css/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
    tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
    tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
    tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
    tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
    // Note: markdown and sql might have different binding paths or versions
    tree_sitter_markdown "github.com/tree-sitter-grammars/tree-sitter-markdown/bindings/go"
    tree_sitter_sql "github.com/DerekStride/tree-sitter-sql/bindings/go"
	"sync"
)

var (
	// cache for initialized languages
	languageCache = make(map[string]*sitter.Language)
	cacheMu       sync.RWMutex
)

// grammarRegistry maps grammar names (from TOML) to their getter functions.
var grammarRegistry = map[string]func() *sitter.Language{
	"tree-sitter-go":         func() *sitter.Language { return sitter.NewLanguage(tree_sitter_go.Language()) },
	"tree-sitter-python":     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_python.Language()) },
	"tree-sitter-typescript": func() *sitter.Language { return sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()) },
	"tree-sitter-javascript": func() *sitter.Language { return sitter.NewLanguage(tree_sitter_javascript.Language()) },
	"tree-sitter-php":        func() *sitter.Language { return sitter.NewLanguage(tree_sitter_php.LanguagePHP()) },
	"tree-sitter-html":       func() *sitter.Language { return sitter.NewLanguage(tree_sitter_html.Language()) },
	"tree-sitter-css":        func() *sitter.Language { return sitter.NewLanguage(tree_sitter_css.Language()) },
	"tree-sitter-json":       func() *sitter.Language { return sitter.NewLanguage(tree_sitter_json.Language()) },
	"tree-sitter-markdown":   func() *sitter.Language { return sitter.NewLanguage(tree_sitter_markdown.Language()) },
	"tree-sitter-sql":        func() *sitter.Language { return sitter.NewLanguage(tree_sitter_sql.Language()) },
	"tree-sitter-tsx":        func() *sitter.Language { return sitter.NewLanguage(tree_sitter_typescript.LanguageTSX()) },
	"tree-sitter-java":       func() *sitter.Language { return sitter.NewLanguage(tree_sitter_java.Language()) },
	"tree-sitter-c":          func() *sitter.Language { return sitter.NewLanguage(tree_sitter_c.Language()) },
	"tree-sitter-cpp":        func() *sitter.Language { return sitter.NewLanguage(tree_sitter_cpp.Language()) },
	"tree-sitter-c-sharp":    func() *sitter.Language { return sitter.NewLanguage(tree_sitter_csharp.Language()) },
	"tree-sitter-rust":       func() *sitter.Language { return sitter.NewLanguage(tree_sitter_rust.Language()) },
}

// GetGrammar returns the tree-sitter Language for the given grammar name.
// It caches the result to avoid expensive re-initialization.
func GetGrammar(name string) (*sitter.Language, error) {
	// 1. Check cache with read lock
	cacheMu.RLock()
	lang, ok := languageCache[name]
	cacheMu.RUnlock()
	if ok {
		return lang, nil
	}

	// 2. Not in cache, need write lock
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Double check in case another goroutine initialized it while we were waiting
	if lang, ok = languageCache[name]; ok {
		return lang, nil
	}

	getter, ok := grammarRegistry[name]
	if !ok {
		return nil, fmt.Errorf("grammar not found in registry: %s", name)
	}

	lang = getter()
	languageCache[name] = lang
	return lang, nil
}
