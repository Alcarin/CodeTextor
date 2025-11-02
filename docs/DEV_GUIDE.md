# 🧩 CodeTextor — Development Guidelines

**Version:** 1.0
**Audience:** Human developers and LLM-based code agents.
**Language:** English only for all identifiers, comments, and documentation.
**Primary goal:** Provide a consistent structure, modular design, and high readability for a local-first *codebase context provider* built on Wails (Go + Web).

---

## 1. Project Purpose

**CodeTextor** is a local **multi-project** application designed to analyze source code using **Tree-sitter**, extract semantic **chunks** (functions, classes, modules, comments), and build **isolated vector indexes** using **SQLite-vec** for fast semantic retrieval.
It acts as an **MCP (Model Context Protocol) server** and can be used by IDE plugins or LLM assistants to retrieve contextual information about multiple projects simultaneously, enabling code understanding, navigation, and search across different codebases.

The project's goal is to:

* Build an *embedded* RAG-like retrieval system for code, without external dependencies.
* Support **multiple projects** with complete isolation (one SQLite-vec database per project).
* Provide precise, hierarchical code chunks with semantic meaning.
* Expose structured APIs for retrieval, navigation, and symbol introspection with **explicit project scoping**.
* Remain modular, lightweight, and transparent for both developers and AI agents.

---

## 2. Architectural Overview

### 🔹 Layers

| Layer        | Language                    | Purpose                                                                                     |
| ------------ | --------------------------- | ------------------------------------------------------------------------------------------- |
| **Frontend** | TypeScript + Vue            | Provide GUI for project management, indexing progress, search results, and code exploration. |
| **Backend**  | Go (Wails)                  | Handle parsing, chunking, embedding, storage, and MCP API endpoints with project isolation.  |
| **Storage**  | SQLite-vec (per-project)    | One database file per project (`indexes/<projectId>.db`) for complete isolation.             |

### 🔹 Core Subsystems

1. **Chunker** — Parses code using Tree-sitter, generates semantic chunks, collapses long functions, merges small ones, and attaches comments.
2. **Indexer** — Embeds chunks, stores vectors and metadata in SQLite-vec, and supports incremental updates.
3. **Retriever (MCP)** — Serves top-k semantic matches for a given query, plus additional tools (outline, nodeSource, symbolSearch).
4. **Frontend UI** — Allows users to start indexing, inspect chunks, and query the codebase visually.

---

## 3. Multi-Project Architecture

### 🔹 Design Principles

CodeTextor is designed as a **multi-project application** with complete isolation between projects:

1. **One Database Per Project**: Each project has its own SQLite-vec database file stored at `indexes/<projectId>.db`.
2. **Explicit Project Scoping**: All MCP API calls **require** a `projectId` parameter. Requests without it are rejected.
3. **No Cross-Contamination**: Search results, embeddings, and metadata never mix between projects.
4. **Independent Configuration**: Each project can have its own indexing parameters, file filters, and embedding models.

### 🔹 Storage Strategy

```
/indexes/
  project-abc123.db       → SQLite-vec database for project "abc123"
  project-def456.db       → SQLite-vec database for project "def456"
  ...

/config/
  projects.json           → Project metadata (name, path, createdAt, settings)
```

**Benefits:**
* **Complete isolation**: No risk of mixing data between projects
* **Easy backup/restore**: Copy single `.db` file + project config
* **Independent lifecycle**: Delete, archive, or migrate projects independently
* **Simpler queries**: No need to filter by `project_id` in every query

### 🔹 Project Lifecycle

1. **Creation**: User provides project name, root path, optional description
2. **Initialization**: Create `indexes/<projectId>.db` and project config entry
3. **Indexing**: Tree-sitter parsing → chunking → embedding → store in project's DB
4. **Querying**: All MCP tools receive `projectId` and query the correct DB
5. **Deletion**: Remove `indexes/<projectId>.db` and config entry

### 🔹 Security & Boundaries

* **Root Path Enforcement**: Each project has a whitelist of allowed root paths
* **Path Validation**: Tools like `nodeSource`, `fileAt` reject paths outside project roots
* **Size Limits**: Independent byte caps per project for MCP responses
* **Resource Limits**: Per-project throttling to prevent CPU monopolization

---

## 4. Folder Structure Guidelines

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

## 5. Coding Conventions

### General

* **All identifiers (functions, variables, files, directories) must be in English.**
* **Every function—named, anonymous, or arrow—must be preceded by a comment** describing:

  1. Its purpose and expected behavior.
  2. Input parameters and expected types.
  3. Returned values and possible side effects.
* Functions that perform complex logic should also include a brief *inline* comment per key section.
  Remove or update these comments when the implementation changes or becomes self-explanatory to avoid drift.
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

Use the native doc-comment style of each language. For example, TypeScript/Vue files should prefer `/** ... */` JSDoc headers, while Go files use `/* ... */` as illustrated above.

---

## 6. Chunking & Indexing Strategy

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

### Per-Project Indexing

Each project maintains its own complete indexing state:

* **Independent indexes**: Each project's chunks and vectors are stored in `indexes/<projectId>.db`
* **Per-project configuration**: Indexing parameters (chunk size, embedding model, file filters) are stored in project metadata
* **Isolated file watchers**: Each project has its own file system watcher for incremental updates
* **Separate indexing queues**: Multiple projects can be indexed concurrently without interference
* **Project-specific exclusions**: `.gitignore`, custom ignore patterns are applied per project

---

## 7. MCP Server Responsibilities

Expose lightweight, composable tools (JSON-RPC or HTTP) usable by IDEs and AI agents.

### 🔹 MCP Tools with Project Scoping

**All MCP tools require a `projectId` parameter.** Requests without it are rejected with an error.

| Tool                                                    | Description                                      |
| ------------------------------------------------------- | ------------------------------------------------ |
| `retrieve(projectId, query, k, filters)`                | Semantic top-k retrieval from project's vector DB |
| `outline(projectId, path, depth)`                       | Get structural outline of a file in project       |
| `nodeAt(projectId, path, line)`                         | Return AST node at specific position in project   |
| `nodeSource(projectId, id, collapseBody)`               | Return source snippet of node from project        |
| `searchSymbols(projectId, query, kinds)`                | Lexical symbol search within project              |
| `findDefinition(projectId, name)` / `findReferences(projectId, name)` | Optional, reference navigation within project     |

### 🔹 MCP Server Requirements

All endpoints must:

* **Require projectId**: Validate that the project exists and is accessible
* **Query correct database**: Use `indexes/<projectId>.db` for the specified project
* **Enforce path boundaries**: Only return results for files within the project's root path
* **Return bounded results**: Limited by byte size (configurable per project)
* **Support pagination**: When returning large result sets
* **Never leak cross-project data**: Results must be strictly scoped to the requested project

---

## 8. Frontend Guidelines

* Written in **TypeScript**, using **Vue 3** (Tailwind).
* Components must be modular: one component = one purpose.
* Avoid global state except for user settings and cache.
* Use composition API / hooks for business logic separation.
* All UI strings in English.
* Document every component with JSDoc block at the top.

Example:

```ts
/** 
 * Component: ProjectList
 * Purpose: Display the list of indexed projects with quick actions.
 * Props: projects (ProjectSummary[])
 */
```

---

## 9. Testing and Documentation

* **Unit tests** for each backend package (`*_test.go`) and frontend component.
* **Integration tests** for MCP endpoints with multi-project scenarios.
* **Multi-project test coverage**:
  * Test project isolation (no data leakage between projects)
  * Test concurrent indexing of multiple projects
  * Test project creation, deletion, and switching
  * Test MCP tools with valid and invalid projectId parameters
  * Test path boundary enforcement
* Maintain `/docs/` directory with:

  * `ARCHITECTURE.md`
  * `API_REFERENCE.md`
  * `DEV_GUIDE.md` (this document)
  * `CHANGELOG.md`
* Every major change must update documentation accordingly.

---

## 10. Quality & Readability Targets

| Metric                   | Target      |
| ------------------------ | ----------- |
| Function doc coverage    | 100%        |
| File header doc coverage | 100%        |
| Average file length      | ≤ 300 lines |
| Lint/format errors       | 0           |
| Tests passing            | 100%        |

---

## 11. Design Philosophy Summary

* **Local-first:** No cloud dependencies; everything runs locally.
* **Modular:** Each concern isolated in its own package or component.
* **Transparent:** All data (chunks, symbols, embeddings) are inspectable.
* **Extensible:** Easy to integrate with other tools or MCP servers.
* **Readable:** Code designed to be understood by both humans and LLMs.

---

## 12. AI Collaboration Principles

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
