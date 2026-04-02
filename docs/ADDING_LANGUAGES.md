# 🌍 Adding Support for New Languages

CodeTextor uses a dynamic, query-based engine for code parsing. Adding support for a new language typically requires zero changes to the core logic—only configuration and grammar registration.

---

## 🚀 Quick Start: 3-Step Process

### 1. Register the Tree-sitter Grammar (Static/CGO)

Before you can use a TOML file, the underlying Tree-sitter grammar must be compiled into the Go binary. This requires a C compiler and a full rebuild.

**File:** `backend/internal/chunker/grammar_registry.go`

1. Add the import (ensure the vendor folder is updated if needed).
2. Add a line to the `grammarRegistry` map:

```go
var grammarRegistry = map[string]func() *sitter.Language{
    // ... existing grammars
    "tree-sitter-rust": func() *sitter.Language { return sitter.NewLanguage(tree_sitter_rust.Language()) },
}
```

1. **Rebuild the Application**: Run `wails build` or `wails dev` to include the new grammar in the binary. This step is **mandatory** when adding a language that wasn't previously supported.

### 2. Create the TOML Configuration

Create a new TOML file in `backend/internal/chunker/parsers/default/<lang>.toml`. This file is the "brain" of your parser.

#### **Structure Overview**

```toml
[language]
name = "rust"            # Must match the name in detectLanguage()
grammar = "tree-sitter-rust"
extensions = [".rs"]

[queries]
symbols = "..."          # Core structure extraction
imports = "..."          # Dependency tracking
usages = "..."           # Cross-reference tracking
metadata = "..."         # Namespace/Package info

[queries.extra]
signature = "..."        # (Optional) Detailed params/return types
docstring = "..."        # (Optional) For languages with internal docs like Python

[rules]
comment_prefixes = ["//", "/*"] # Used to clean docstrings

[rules.visibility]
type = "prefix"          # Supported: "prefix", "first_letter_case", "keyword"
# For type="prefix":
private_prefix = "__"    
protected_prefix = "_"
# For type="keyword": (used by TS, Java, C#)
# The engine looks for @visibility.public, @visibility.private, etc. in symbols query

[rules.todo]
pattern = '(?i)(TODO|FIXME|HACK|NOTE)' # Regex for extraction
node_types = ["comment", "line_comment", "block_comment"]
```

---

## 💎 Mastering Queries: Capture Reference

### **1. Symbol Metadata (`symbols` query)**

Capture the *entire* node you want as a chunk using `@symbol.<kind>`, and capture the *identifier* part using `@name`.

| Capture Name | Target Field | Notes |
| :--- | :--- | :--- |
| `@symbol.function` | `Kind = "function"` | Top-level functions |
| `@symbol.method` | `Kind = "method"` | Functions inside classes/structs |
| `@symbol.class` | `Kind = "class"` | |
| `@symbol.struct` | `Kind = "struct"` | |
| `@symbol.interface` | `Kind = "interface"` | |
| `@symbol.variable` | `Kind = "variable"` | |
| `@symbol.constant` | `Kind = "constant"` | |
| **`@name`** | **`Name`** | **Mandatory**. The identifier node. |
| `@visibility.public` | `Visibility = "public"` | Used when `rules.visibility.type = "keyword"` |
| `@signature` | `Signature` | Parameters and return types. |

#### **Handling Methods vs Functions**

Tree-sitter queries are executed in order. To distinguish methods, use a nested query:

```query
;; 1. Catch all functions
(function_definition
  name: (identifier) @name) @symbol.function

;; 2. Catch functions inside classes as methods (overrides previous match)
(class_definition
  body: (block
    (function_definition
      name: (identifier) @name))) @symbol.method
```

> [!TIP]
> **Precision matters**: Do not capture the name of a function as `@symbol.function`. Capture the `function_declaration` node as `@symbol.function` and its internal `identifier` as `@name`.

### **Usages (`usages` query)**

Used to build the call graph and find references.

| Capture Name | Target Field | Purpose |
| :--- | :--- | :--- |
| `@call.name` | `Usage.Name` | The name of the function/method being called. |
| `@call.receiver` | `Usage.Context` | The object/package calling it (e.g., `fmt` in `fmt.Println`). |

### **Metadata (`metadata` query)**

| Capture Name | Target Field | Purpose |
| :--- | :--- | :--- |
| `@meta.package` | `metadata["package"]` | Logic grouping (Go `package`, Java `package`). |
| `@meta.module` | `metadata["module"]` | File grouping (Python `module`). |

---

## 🛡️ Best Practices & Pitfalls

### **1. Use Tree-sitter Playground**

Before writing TOML, use the [Tree-sitter Playground](https://tree-sitter.github.io/tree-sitter/playground) to inspect the AST of your target language. This is the only way to know the exact node names (e.g., `function_item` vs `function_declaration`).

### **2. Avoid "Over-capturing"**

If you have multiple types of functions (e.g., arrow functions and standard functions), write two separate blocks in the `symbols` query rather than one complex regex.

### **3. The `[language]` Header**

**CRITICAL**: Every TOML file *must* start with the `[language]` header. If you put `name = "..."` at the top level without the header, the parser will fail to initialize and complain that the grammar is missing.

### **4. Signature Extraction**

If your language makes it hard to distinguish the signature from the name in a single query, use the `[queries.extra]` block:

```toml
[queries.extra]
signature = """
(function_item
  name: (identifier) @_name
  parameters: (parameter_list) @signature)
"""
```

*Note: captures starting with `_` (like `@_name`) are used for matching but are discarded by the engine.*

---

## 🛠️ Testing Your Changes

1. **Wails Dev**: If you are running `wails dev`, the backend will rebuild and pick up the new grammar registration and TOML file.
2. **Dynamic Iteration**: Once the grammar is registered, you can modify the TOML queries and simply restart the app to see changes without full recompilation.
3. **Unit Tests**: Add a small test case in `backend/internal/chunker/parser_test.go` using a sample snippet of the new language. Ensure you call `NewParser(DefaultChunkConfig())` to trigger the TOML engine.

## 💡 Pro Tips

- **Nested Languages**: If your language contains embedded code (like HTML in JavaScript), the `SubLanguageManager` is automatically available.
- **Manual Overrides**: Users can override your default TOML by placing a file with the same name in `<AppData>/parsers/`.
