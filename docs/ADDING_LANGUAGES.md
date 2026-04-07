# 🌍 Adding Support for New Languages

CodeTextor uses a hybrid parsing engine: a high-performance **CGO-based Tree-sitter core** for tokenization and a **declarative TOML-based configuration** for semantic extraction. 

Adding a new language is a two-step process:
1.  **Grammar Registration**: Compiling the Tree-sitter binding in Go.
2.  **Parser Configuration**: Defining queries and rules in a TOML file.

---

## 🛠️ Step 1: Register the Grammar (Go/CGO)

Tree-sitter grammars are written in C and must be compiled into the Go binary.

1.  **Add the Go binding**: If not already present in `go.mod`, add the corresponding Tree-sitter binding for your language (e.g., `github.com/tree-sitter/tree-sitter-rust`).
2.  **Update the Registry**: Modify `backend/internal/chunker/grammar_registry.go`:
    - Add the import for the new grammar.
    - Register it in the `grammarRegistry` map:
      ```go
      "tree-sitter-rust": func() *sitter.Language { return sitter.NewLanguage(tree_sitter_rust.Language()) },
      ```
3.  **Rebuild**: Run `wails dev` or `wails build`. This step is **mandatory** because C dependencies cannot be loaded dynamically at runtime.

---

## 📄 Step 2: Configure the Parser (TOML)

Once the grammar is registered, you can define how CodeTextor should "understand" the language using a TOML file. 

Default configurations are located in `backend/internal/chunker/parsers/default/<lang>.toml`.

### 🔹 Basic Structure

```toml
[language]
name = "rust"
grammar = "tree-sitter-rust"
extensions = [".rs"]

[queries]
symbols = "(function_item name: (identifier) @name) @symbol.function"
imports = """
(use_declaration
  argument: (scoped_identifier) @import)
"""

[rules]
comment_prefixes = ["//", "/*"]

[rules.visibility]
type = "prefix_underscore" # Options: first_letter_case, prefix_underscore, keyword, public

[rules.todo]
pattern = '(?i)(TODO|FIXME|HACK):?\s*(.*)'
node_types = ["line_comment", "block_comment"]
```

---

## 💎 Mastering TOML Parameters

### 1. Queries (`[queries]`)

CodeTextor uses **Tree-sitter Queries** with specific capture tags:

| Capture Tag | Target Field | Notes |
| :--- | :--- | :--- |
| `@symbol.<kind>` | `Kind` | Defines the chunk type (e.g., `@symbol.function`, `@symbol.class`). |
| `@name` | `Name` | **Mandatory**. The identifier of the symbol. |
| `@parent` | `Parent` | Hints the hierarchical container (used if range matching fails). |
| `@signature` | `Signature` | Extra context (parameters, return types). |
| `@import` | `Imports` | Used in the `imports` query to track dependencies. |
| `@meta.<key>` | `Metadata` | Custom metadata (e.g., `@meta.package` for Go packages). |
| `@call.name` | `Usage` | Target name for cross-references. |

#### **Extra Queries (`[queries.extra]`)**
Used to extract supplemental data that might be easier to target with a separate pass (e.g., `docstring`, `signature`).

#### **Regex Patterns (`[[queries.symbol_patterns]]`)**
A list of fallback rules for languages where Tree-sitter queries are overly complex or insufficient.

### 2. Value Formatting (`[rules.formatting]`)
Allows you to transform captured values before they are stored.
```toml
[rules.formatting]
id = { prefix = "#", lowercase = true } # Applied to @name.id or @signature.id
```

### 3. Sub-Languages (`[sub_languages]`)
Defines tags or node types that contain embedded code in other languages.
```toml
[sub_languages]
script_element = "javascript"
style_element = "css"
interpreted_string_literal_content = "detect" # Automatic detection (e.g., SQL in Go)
```

---

## 💡 Examples

### Go Configuration (Simple)
```toml
[language]
name = "go"
grammar = "tree-sitter-go"

[queries]
symbols = """
(function_declaration name: (identifier) @name) @symbol.function
(type_declaration (type_spec name: (type_identifier) @name type: (struct_type)) @symbol.struct)
"""

[rules.visibility]
type = "first_letter_case" # Uppercase = Public
```

### Vue Configuration (Advanced Sub-Languages)
```toml
[sub_languages]
script_element = "javascript"
style_element = "css"

[queries]
symbols = """
(element (start_tag (tag_name) @name (#eq? @name "template"))) @symbol.element
"""
```

---

## 🚀 Iteration & Debugging

1.  **User Customization**: Users can override default parsers by placing a TOML file in the **Config Directory** under `parsers/`.
    - **Windows**: `%LOCALAPPDATA%\codetextor\parsers\`
    - **Linux**: `~/.local/share/codetextor/parsers/`
    - **macOS**: `~/Library/Application Support/codetextor/parsers/`
2.  **Live Debugging**: Use the `ti` tool to test your TOML files against source code without restarting the app:
    ```bash
    .tmp/ti.exe rust sample.rs path/to/rust.toml
    ```

---

## 🧩 Advanced Implementation (Go Only)

If a language requires logic that cannot be expressed via TOML (e.g., complex symbol resolution that depends on external state), you can implement the `LanguageParser` interface in `backend/internal/chunker/types.go` and register it manually in `parser.go`.
