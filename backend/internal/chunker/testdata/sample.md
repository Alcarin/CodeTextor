# CodeTextor Architecture

An overview of the project components.

## Parser Engine

The parser engine uses Tree-sitter grammars with [TOML configuration](./docs/ADDING_LANGUAGES.md).

### Query-Based Extraction

Symbols are extracted using S-expression queries defined in `.toml` files.

### Sub-Language Support

Vue and PHP files use [sub-language delegation](./docs/SUB_LANGUAGES.md) for embedded code.

## Semantic Chunker

The chunker transforms symbols into enriched [code chunks](./docs/CHUNKS.md).

```go
func main() {
    chunker := NewSemanticChunker(config)
    result, _ := chunker.ChunkFile("main.go", source)
    fmt.Println(len(result))
}
```

```python
def analyze(path):
    with open(path) as f:
        return parse(f.read())
```

## Roadmap

- [ ] Add Rust support
- [ ] Add Java support
- [x] Migrate all parsers to TOML
TODO: write benchmarking guide

External resources: [Tree-sitter docs](https://tree-sitter.github.io/).
