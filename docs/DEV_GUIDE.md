# 🧩 CodeTextor — Development Guidelines

**Version:** 1.0
**Audience:** Human developers and LLM-based code agents.
**Language:** English only for all identifiers, comments, and documentation.
**Primary goal:** Provide a consistent structure, modular design, and high readability for a local-first *codebase context provider* built on Wails (Go + Web).

---

## 1. Project Purpose

**CodeTextor** is a local application designed to analyze source code using **Tree-sitter**, extract semantic **chunks** (functions, classes, modules, comments), and build a **vector index** using **SQLite-vec** for fast semantic retrieval.
It acts as an **MCP (Model Context Protocol) server** and can be used by IDE plugins or LLM assistants to retrieve contextual information about a project, enabling code understanding, navigation, and search.

The project’s goal is to:

* Build an *embedded* RAG-like retrieval system for code, without external dependencies.
* Provide precise, hierarchical code chunks with semantic meaning.
* Expose structured APIs for retrieval, navigation, and symbol introspection.
* Remain modular, lightweight, and transparent for both developers and AI agents.

---

## 2. Architectural Overview

### 🔹 Layers

| Layer        | Language                    | Purpose                                                                                     |
| ------------ | --------------------------- | ------------------------------------------------------------------------------------------- |
| **Frontend** | TypeScript + Vue            | Provide GUI for indexing progress, search results, and visual exploration of the code tree. |
| **Backend**  | Go (Wails)                  | Handle parsing, chunking, embedding, storage, and MCP API endpoints.                        |
| **Storage**  | SQLite-vec                  | Store vector embeddings and symbol metadata.                                                |

### 🔹 Core Subsystems

1. **Chunker** — Parses code using Tree-sitter, generates semantic chunks, collapses long functions, merges small ones, and attaches comments.
2. **Indexer** — Embeds chunks, stores vectors and metadata in SQLite-vec, and supports incremental updates.
3. **Retriever (MCP)** — Serves top-k semantic matches for a given query, plus additional tools (outline, nodeSource, symbolSearch).
4. **Frontend UI** — Allows users to start indexing, inspect chunks, and query the codebase visually.

---

## 3. Folder Structure Guidelines

### Root layout

```
/frontend/         → UI source (Vue/React, TypeScript)
  /components/
  /views/
  /store/
  /styles/

 /backend/          → Go backend (Wails)
  /internal/
    /chunker/       → Tree-sitter logic, code parsing, collapsing
    /indexer/       → Embedding generation, vector storage
    /mcp/           → Model Context Protocol tools
    /store/         → SQLite-vec database models
    /search/        → Semantic and lexical query logic
  /cmd/             → CLI commands, entry points
  /pkg/             → Shared utilities and abstractions
```

### Best Practices

* Never create monolithic files.
  Each logical component (e.g., chunker, store, retriever) **must have its own file** or sub-package.
* Group related functions by purpose, not by type.
* Keep each file under ~300 lines of code when possible.
* Use consistent naming: lowercase, underscore-separated for Go files; PascalCase for structs/types; camelCase for variables/functions.

---

## 4. Coding Conventions

### General

* **All identifiers (functions, variables, files, directories) must be in English.**
* **Every function—named, anonymous, or arrow—must be preceded by a comment** describing:

  1. Its purpose and expected behavior.
  2. Input parameters and expected types.
  3. Returned values and possible side effects.
* Functions that perform complex logic should also include a brief *inline* comment per key section.
* Comments must be written in **concise, descriptive English** and kept up-to-date.

Example:

```go
// collapseFunction shortens a long function node by removing its body.
// It keeps the function signature and replaces the block with "{ ... }".
func collapseFunction(node *sitter.Node, src []byte) string { ... }
```

### File headers

Each source file should begin with:

```
/*
  File: collapse.go
  Purpose: Tree-sitter utilities for collapsing long function/class blocks.
  Author: CodeTextor project
  Notes: This file is part of the internal chunker module.
*/
```

---

## 5. Chunking & Indexing Strategy

### Core principles

1. **Tree-sitter-based parsing:** Extract syntactic nodes like `function_declaration`, `class_body`, `comment`, etc.
2. **Chunk enrichment:**

   * Prepend file, package, and symbol info.
   * Merge leading comments.
   * Collapse long blocks (`{ ... }` placeholder).
3. **Adaptive chunk size:**

   * Split large nodes (> 800 tokens).
   * Merge small ones (< 100 tokens) with neighbors or file context.
4. **Semantic embedding:** Generate vector representations for chunk content + metadata.
5. **Incremental indexing:** Only update changed files (based on hash + mtime).

---

## 6. MCP Server Responsibilities

Expose lightweight, composable tools (JSON-RPC or HTTP) usable by IDEs and AI agents:

| Tool                                            | Description                             |
| ----------------------------------------------- | --------------------------------------- |
| `retrieve(query, k, filters)`                   | Semantic top-k retrieval from vector DB |
| `outline(path, depth)`                          | Get structural outline of a file        |
| `nodeAt(path, line)`                            | Return AST node at specific position    |
| `nodeSource(id, collapseBody)`                  | Return source snippet of node           |
| `searchSymbols(query, kinds)`                   | Lexical symbol search                   |
| `findDefinition(name)` / `findReferences(name)` | Optional, reference navigation          |

All endpoints must:

* Return **bounded results** (limited by byte size).
* Support **pagination** when returning large sets.
* Never include the entire AST in a single response.

---

## 7. Frontend Guidelines

* Written in **TypeScript**, using **Vue 3** (Tailwind).
* Components must be modular: one component = one purpose.
* Avoid global state except for user settings and cache.
* Use composition API / hooks for business logic separation.
* All UI strings in English.
* Document every component with JSDoc block at the top.

---

## 8. Testing and Documentation

* **Unit tests** for each backend package (`*_test.go`) and frontend component.
* **Integration tests** for MCP endpoints.
* Maintain `/docs/` directory with:

  * `ARCHITECTURE.md`
  * `API_REFERENCE.md`
  * `DEV_GUIDE.md` (this document)
  * `CHANGELOG.md`
* Every major change must update documentation accordingly.

---

## 9. Quality & Readability Targets

| Metric                   | Target      |
| ------------------------ | ----------- |
| Function doc coverage    | 100%        |
| File header doc coverage | 100%        |
| Average file length      | ≤ 300 lines |
| Lint/format errors       | 0           |
| Tests passing            | 100%        |

---

## 10. Design Philosophy Summary

* **Local-first:** No cloud dependencies; everything runs locally.
* **Modular:** Each concern isolated in its own package or component.
* **Transparent:** All data (chunks, symbols, embeddings) are inspectable.
* **Extensible:** Easy to integrate with other tools or MCP servers.
* **Readable:** Code designed to be understood by both humans and LLMs.

---

## 11. AI Collaboration Principles

This project will be co-developed by human and LLM agents.
LLMs working on CodeTextor must:

* Follow all sections of this document.
* Write code and comments in **English only**.
* Never merge unrelated concerns in one file.
* Respect modular architecture and folder separation.
* Always generate clear, documented functions with headers and comments.
* Prioritize correctness and clarity over conciseness.
* Avoid hallucinating APIs or renaming functions arbitrarily.

---

**End of Document**
*(This file should be stored as `/docs/DEV_GUIDE.md` and used as the canonical reference for human and AI contributors.)*
