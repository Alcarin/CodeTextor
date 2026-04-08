# 🔍 Tree-sitter Inspector (ti)

`ti` is the universal debugging tool for CodeTextor that allows you to inspect the Abstract Syntax Tree (AST) and test Tree-sitter queries (including full TOML configurations) in real-time.

## Key Features

- **AST Inspection**: Hierarchical node visualization for any code snippet or file.
- **TOML Support**: Automatically load all queries (`symbols`, `imports`, `metadata`, `usages`, `extra`) defined in a CodeTextor configuration file.
- **Sub-Language Debugging**: Automatically loads all `.toml` parsers from the same directory to initialize a `SubLanguageManager`, enabling recursive extraction testing (e.g., PHP -> HTML -> JS).
- **Query Validation**: Test individual queries with capture highlighting (`@name`, `@symbol.class`, etc.).
- **Flexible Input**: Accepts both raw code strings and local file paths.

> [!TIP]
> **Best Practice**: It is highly recommended to pass local file paths for both the **source** and the **query**. This avoids shell character escaping issues (e.g., `()`, `@`, `{}`).

## Installation

From the project root, compile the tool into the temporary folder (git-ignored):

```bash
go build -o .tmp/ti.exe backend/internal/chunker/debug/ti.go
```

## Usage

### Inspect a File (AST)

To see the node structure of an existing file:

```bash
.tmp/ti.exe go backend/internal/chunker/testdata/sample.go
```

### Test a Full TOML Configuration

This is the most powerful way to validate a new parser or changes to an existing one without restarting the backend:

```bash
.tmp/ti.exe vue backend/internal/chunker/testdata/sample.vue backend/internal/chunker/parsers/default/vue.toml
```

### Test a Quick Query

To quickly check if a query extracts what you expect:

```bash
.tmp/ti.exe css ".bg-red { color: red; }" "(class_name) @name"
```

### Advanced Usage: Query Files (.scm)

If your query contains many special characters, you can save it to a file (e.g., `test.scm`) and pass it as an argument:

```bash
.tmp/ti.exe css path/to/source.css path/to/query.scm
```

## Why is it useful?

Tree-sitter grammars can behave differently depending on the version or specific code structure. This tool allows you to:

1. **Discover Nodes**: Understand if an element is a direct child or mediated by lists (e.g., `import_spec_list`).
2. **Instant Validation**: Test entire `.toml` configurations in a few milliseconds.
3. **Sub-language Testing**: Verify that embedded blocks (HTML/JS/SQL) are correctly identified and parsed using the full power of the dynamic engine.
4. **Fast Debugging**: Resolve `Impossible pattern` errors or missing captures without launching the full application.
