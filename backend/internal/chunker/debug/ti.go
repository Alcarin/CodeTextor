package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"CodeTextor/backend/internal/chunker"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ti <lang> <source|path> [query|path|config.toml]")
		fmt.Println("\nArguments:")
		fmt.Println("  <lang>          Language name (e.g., go, python, css)")
		fmt.Println("  <source|path>   Source code string OR path to a local file (preferred)")
		fmt.Println("  [query|path]    Optional: Tree-sitter query string, path to a query file (.scm, .txt),")
		fmt.Println("                  OR path to a .toml configuration file")
		fmt.Println("\nNote: Passing file paths for both source and query is PREFERRED for performance,")
		fmt.Println("      reusability, and to avoid shell-specific special character issues.")
		fmt.Println("\nExamples:")
		fmt.Println("  ti go path/to/file.go                      # View file AST")
		fmt.Println("  ti go file.go \"(import_spec) @i\"           # Run specific query string")
		fmt.Println("  ti go file.go query.scm                    # Run query from file (stable)")
		fmt.Println("  ti go file.go parsers/go.toml              # Run all queries from config")
		return
	}

	langName := os.Args[1]
	grammarName := langName
	if !strings.HasPrefix(langName, "tree-sitter-") {
		grammarName = "tree-sitter-" + langName
	}
	
	lang, err := chunker.GetGrammar(grammarName)
	if err != nil {
		fmt.Printf("Grammar error: %v\n", err)
		return
	}

	sourceRaw := os.Args[2]
	var source []byte
	if _, err := os.Stat(sourceRaw); err == nil {
		source, err = os.ReadFile(sourceRaw)
		if err != nil {
			fmt.Printf("File read error: %v\n", err)
			return
		}
	} else {
		source = []byte(sourceRaw)
	}
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree := parser.Parse(source, nil)
	
	if tree == nil {
		fmt.Println("Parsing error.")
		return
	}

	if len(os.Args) > 3 {
		queryArg := os.Args[3]
		if strings.HasSuffix(queryArg, ".toml") {
			data, err := os.ReadFile(queryArg)
			if err != nil {
				fmt.Printf("TOML read error: %v\n", err)
				return
			}
			cfg, err := chunker.LoadConfigFromBytes(data)
			if err != nil {
				fmt.Printf("TOML parsing error: %v\n", err)
				return
			}

			fmt.Printf("\n--- RESULTS FROM CONFIGURATION (%s) ---\n", queryArg)
			
			// 1. Run raw queries for debugging
			fmt.Printf("\n>> RAW QUERY CAPTURES\n")
			runQuery(lang, "SYMBOLS", cfg.Queries.Symbols, tree.RootNode(), source)
			runQuery(lang, "IMPORTS", cfg.Queries.Imports, tree.RootNode(), source)
			runQuery(lang, "METADATA", cfg.Queries.Metadata, tree.RootNode(), source)
			runQuery(lang, "USAGES", cfg.Queries.Usages, tree.RootNode(), source)
			for name, qstr := range cfg.Queries.Extra {
				runQuery(lang, "EXTRA:"+strings.ToUpper(name), qstr, tree.RootNode(), source)
			}

			// 2. Test full QueryParser logic (including sub-languages)
			qp, err := chunker.NewQueryParser(cfg)
			if err != nil {
				fmt.Printf("\nQueryParser initialization error: %v\n", err)
				return
			}
			
			// Try to initialize a SubLanguageManager by loading other configs from the same directory
			parsers := make(map[string]chunker.LanguageParser)
			configDir := filepath.Dir(queryArg)
			files, _ := os.ReadDir(configDir)
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".toml") {
					path := filepath.Join(configDir, f.Name())
					d, _ := os.ReadFile(path)
					c, _ := chunker.LoadConfigFromBytes(d)
					if c != nil {
						p, _ := chunker.NewQueryParser(c)
						if p != nil {
							for _, ext := range c.Language.Extensions {
								parsers[ext] = p
							}
						}
					}
				}
			}
			
			slm := chunker.NewSubLanguageManager(parsers)
			qp.SetSubLanguageManager(slm)
			
			// DO NOT FORGET: set the sub-language manager for all loaded sub-parsers too!
			// This enables recursive extraction (e.g., PHP -> HTML -> JS)
			for _, p := range parsers {
				if aware, ok := p.(chunker.SubLanguageAware); ok {
					aware.SetSubLanguageManager(slm)
				}
			}
			
			res, err := qp.Parse(source)
			if err == nil {
				fmt.Printf("\n>> FULL EXTRACTION RESULTS (QueryParser.Parse - %d symbols)\n", len(res.Symbols))
				for _, s := range res.Symbols {
					parentStr := ""
					if s.Parent != "" {
						parentStr = " (Parent: " + s.Parent + ")"
					}
					fmt.Printf("  [%s] %s [L:%d-%d]%s\n", s.Kind, s.Name, s.StartLine, s.EndLine, parentStr)
				}
			} else {
				fmt.Printf("\nParse error: %v\n", err)
			}

			// 3. Test sub-languages discovery query
			if len(cfg.SubLanguages) > 0 {
				var qStr strings.Builder
				qStr.WriteString("[")
				for kind := range cfg.SubLanguages {
					if !strings.ContainsAny(kind, "()[]{} @.") {
						qStr.WriteString(fmt.Sprintf("(%s) ", kind))
					}
				}
				qStr.WriteString("] @sub")
				runQuery(lang, "SUB_LANGUAGE_CONTAINERS", qStr.String(), tree.RootNode(), source)
			}
		} else if _, err := os.Stat(queryArg); err == nil {
			data, err := os.ReadFile(queryArg)
			if err != nil {
				fmt.Printf("Query file read error: %v\n", err)
				return
			}
			fmt.Printf("\n--- QUERY FROM FILE (%s) ---\n", queryArg)
			runQuery(lang, "FILE", string(data), tree.RootNode(), source)
		} else {
			fmt.Printf("\n--- DIRECT QUERY RESULTS ---\n")
			runQuery(lang, "INPUT", queryArg, tree.RootNode(), source)
		}
	} else {
		fmt.Printf("\n--- AST FOR %s ---\n\n", grammarName)
		printNode(tree.RootNode(), 0)
	}
}

func runQuery(lang *sitter.Language, label, query string, root *sitter.Node, source []byte) {
	if query == "" {
		return
	}
	q, err := sitter.NewQuery(lang, query)
	if err != nil {
		fmt.Printf("\n[%s] QUERY ERROR: %v\n", label, err)
		return
	}

	fmt.Printf("\n>> %s\n", label)
	cursor := sitter.NewQueryCursor()
	matches := cursor.Matches(q, root, source)
	count := 0
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		captureNames := q.CaptureNames()
		for _, capture := range match.Captures {
			name := captureNames[capture.Index]
			fmt.Printf("  Capture @%s: %s\n", name, string(source[capture.Node.StartByte():capture.Node.EndByte()]))
			count++
		}
	}
	if count == 0 {
		fmt.Println("  (no results)")
	}
}

func printNode(n *sitter.Node, depth int) {
	fmt.Printf("%s%s [%d-%d]\n", 
		strings.Repeat("  ", depth), 
		n.Kind(), 
		n.StartByte(), 
		n.EndByte(),
	)
	for i := uint(0); i < n.ChildCount(); i++ {
		printNode(n.Child(i), depth+1)
	}
}
