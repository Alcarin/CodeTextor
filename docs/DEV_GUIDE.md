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

## 2. Documentation Guidelines

### 📁 Documentation Structure

CodeTextor maintains documentation in `/docs/`:

| Document | Purpose | Keep Updated |
|----------|---------|--------------|
| **DEV_GUIDE.md** | Development standards and practices | Always (this is the source of truth) |
| **ARCHITECTURE.md** | System design, data flow, component interaction | When architecture changes |
| **API_REFERENCE.md** | Wails and MCP API specifications | When APIs change |
| **TODO.md** | Development roadmap and task tracking | As tasks complete/added |
| **CHANGELOG.md** | User-facing changes per version | At release time |

### 📝 When to Document

**DO create/update documentation for:**
- ✅ Architectural decisions (new subsystems, major refactors)
- ✅ Non-obvious design choices (why we chose X over Y)
- ✅ Public APIs and interfaces (MCP tools, Wails bindings)
- ✅ Development workflows (how to add migrations, run tests)
- ✅ Breaking changes or migration paths

**DON'T create documentation for:**
- ❌ Individual bug fixes (use commit messages + code comments)
- ❌ Implementation details already clear from code
- ❌ Temporary workarounds (add to TODO.md, never in code comments)
- ❌ Experimental features (add to TODO.md as pending tasks)

### 📏 Documentation Proportionality

**Size guideline:** Documentation should be proportional to complexity and impact.

- **Small change** (bug fix, minor feature): Good commit message + code comments
- **Medium change** (new component, API change): Update relevant section in existing docs
- **Large change** (new subsystem, architecture shift): New section or document

**Example:**
- ✅ Database migration system → Short section in DEV_GUIDE.md (32 lines)
- ❌ Database migration system → 3 separate documents + 220 lines in DEV_GUIDE
- ✅ Bug fix (null array) → Commit message + inline comment explaining the fix
- ❌ Bug fix (null array) → Separate BUGFIX_NULL_ARRAY.md document

### 🔄 Documentation Maintenance

**Update docs when:**
1. **Architecture changes**: Update ARCHITECTURE.md with new components or data flows
2. **API changes**: Update API_REFERENCE.md (DEV_GUIDE.md only for API guidelines)
3. **Development workflow changes**: Update DEV_GUIDE.md procedures
4. **Tasks completed or new tasks identified**: Update TODO.md
5. **Releases**: Update CHANGELOG.md with user-facing changes

**How to update:**
- Keep changes minimal and focused
- Remove outdated information rather than adding "deprecated" notes
- Consolidate related information (don't scatter across multiple files)
- Link to code when appropriate instead of duplicating

### 🎯 Documentation Quality

**Good documentation:**
- Explains **why**, not just **what** (code already shows what)
- Provides context and rationale for decisions
- Is concise and scannable (use bullet points, tables)
- Stays synchronized with code

**Poor documentation:**
- Repeats what code already says
- Contains outdated information
- Focuses on minutiae instead of concepts
- Exists in multiple contradictory places

---

## 3. Architectural Overview

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

## 4. Multi-Project Architecture

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
  projects.json           → Project metadata (name, id, description, createdAt, indexing settings)
```

**Benefits:**
* **Complete isolation**: No risk of mixing data between projects
* **Easy backup/restore**: Copy single `.db` file + project config
* **Independent lifecycle**: Delete, archive, or migrate projects independently
* **Simpler queries**: No need to filter by `project_id` in every query

### 🔹 Project Lifecycle

1. **Creation**: User provides project name, project ID, optional description
   - **Note**: Projects do NOT have a single root path. Instead, users configure flexible indexing scope.
2. **Initialization**: Create `indexes/<projectId>.db` and project config entry
3. **Indexing Configuration**:
   - User selects a root folder that anchors the project; include paths are stored relative to that root
   - User adds relative include folders (root is always included by default)
   - User defines exclude patterns (e.g., `node_modules`, `.git`, `.cache`)
   - User optionally filters by file extensions
   - Auto-exclude hidden directories option

   All of those settings are persisted inside the project's own vector database (`indexes/project-<id>.db`), so moving a project to a new machine is as simple as copying that single file while include paths stay relative to the configured root.

   The Indexing view now exposes a **"File Type Filter"** card: it lists the files that match the include/exclude configuration and lets you pin which extensions should end up in the index. Selection is saved immediately into the project configuration (`fileExtensions`) and is reloaded every time the view opens, so the filter stays persistent. In the future the panel can evolve to exclude single files in addition to extensions.
  4. **Indexing**: Tree-sitter parsing → chunking → embedding → store in project's DB
5. **Querying**: All MCP tools receive `projectId` and query the correct DB
6. **Deletion**: Remove `indexes/<projectId>.db` and config entry

### 🔹 Indexing Status & Active Project

   * **Active Project**: The project currently selected in the UI for configuration/viewing (stored in the global `projects.db` under `app_config`)
* **Indexing Status**: Indicates which projects are currently being indexed by the backend
  - **Multiple projects can be indexed concurrently** using separate goroutines/watchers per project
  - Each project has its own file system watcher and indexing queue to avoid interference
  - The UI displays an "Indexing" badge on project cards that are currently being indexed
  - Active project ≠ Indexing projects (user can configure one project while multiple others are indexing)
* **MCP Server**: Serves all indexed projects simultaneously
  - All MCP tool calls **require a `projectId` parameter** to specify which project's index to query
  - This ensures queries are executed against the correct isolated database (`indexes/<projectId>.db`)

### 🔹 Security & Boundaries

* **Path Allowlist**: Each project has a whitelist of allowed include paths configured by the user
* **Path Validation**: Tools like `nodeSource`, `fileAt` reject paths outside configured include paths
* **Size Limits**: Independent byte caps per project for MCP responses
* **Resource Limits**: Per-project throttling to prevent CPU monopolization

---

## 5. Folder Structure Guidelines

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
    /store/         → Project metadata, database migrations, and project-scoped storage
      /migrations/  → SQL migration files (embedded in binary)
    /search/        → Semantic and lexical query logic
  /cmd/             → CLI commands, entry points
  /pkg/             → Shared utilities, models, and services
    /models/        → Public data models (Project, ProjectConfig, etc.)
    /services/      → Business logic layer (ProjectService, etc.)
    /utils/         → Cross-platform utilities (paths, etc.)
```

### Best Practices

* Never create monolithic files.
  Each logical component (e.g., chunker, store, retriever) **must have its own file** or sub-package.
* Group related functions by purpose, not by type.
* Keep each file under ~300 lines of code when possible.
* Use consistent naming: lowercase, underscore-separated for Go files; PascalCase for structs/types; camelCase for variables/functions.

---

## 6. Database Migrations

CodeTextor uses **[golang-migrate/migrate](https://github.com/golang-migrate/migrate)** for database schema changes.

### Migration Types

CodeTextor has two types of migrations:

1. **Config DB migrations** (`backend/internal/store/migrations/`): For global app configuration
2. **Per-Project DB migrations** (`backend/internal/store/vector_migrations/`): For per-project vector databases

### Adding a Migration

1. Create SQL files in the appropriate directory:
   ```bash
   # Format: NNNNNN_description.{up|down}.sql
   000003_add_column.up.sql    # Apply change
   000003_add_column.down.sql  # Rollback
   ```

2. Write idempotent SQL:
   ```sql
   ALTER TABLE projects ADD COLUMN new_col INTEGER DEFAULT 0;
   CREATE INDEX IF NOT EXISTS idx_new_col ON projects(new_col);
   ```

3. Test: `go test ./backend/internal/store/...`

### Critical Rules

- **NEVER modify existing migrations** after release
- Always use sequential version numbers (000001, 000002, ...)
- Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Add `DEFAULT` values for new columns
- Test on both empty and existing databases

### Handling Data in Migrations

When a migration alters the schema in a way that adds new constraints (e.g., `UNIQUE`, `NOT NULL`), it is **critical** that the migration also handles any pre-existing data to make it conform to the new schema. A failure to do so will result in a "dirty" database state if the migration is run on a database with existing data.

**Key Takeaway:** A migration is not just about schema changes; it's about safely transitioning **both the schema and the data** to a new state. Always assume the database is not empty.

Migrations are embedded in the binary and run automatically:
- **Config DB migrations**: Run once at app startup
- **Per-project migrations**: Run when each project database is opened/created

**Recent Per-Project Migrations:**
- `000004_extend_chunks_metadata`: Added 11 semantic metadata fields to chunks table (language, symbol_name, symbol_kind, parent, signature, visibility, package_name, doc_string, token_count, is_collapsed, source_code)
- `000005_unique_chunks_constraint`: Added unique constraint on chunks (file_id, line_start, line_end) to prevent duplicate chunks
- `000006_normalize_schema`: Major schema normalization - replaced path-based references with integer file IDs, added foreign keys, created chunk_symbols mapping table, restructured outline storage

---

## 7. Coding Conventions

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

## 8. Chunking & Indexing Strategy

### Core principles

1. **Tree-sitter-based parsing:** Extract syntactic nodes like `function_declaration`, `class_body`, `comment`, etc.
2. **Chunk enrichment:**

   * Prepend file, package, and symbol info.
   * Merge leading comments.
   * Collapse long blocks (`{ ... }` placeholder).
   * Keep only semantically relevant symbols (functions, classes, top-level variables/constants) to avoid redundant chunks.
3. **Adaptive chunk size:**

   * Split large nodes targeting ~400 tokens (max 800).
   * Merge small ones (< 100 tokens) with neighbors or file context.
4. **Semantic embedding:** Generate vector representations for chunk content + metadata.
5. **Incremental indexing:** Only update changed files (based on hash + mtime).

### Per-Project Indexing

Each project maintains its own complete indexing state with concurrent indexing support:

* **Independent indexes**: Each project's chunks and vectors are stored in `indexes/<projectId>.db`
* **Per-project configuration**: Indexing parameters (chunk size, embedding model, file filters) are stored in project metadata
* **Isolated file watchers**: Each project runs its own goroutine with a dedicated file system watcher for incremental updates
* **Concurrent indexing**: Multiple projects can be indexed simultaneously without interference
  - Each project has its own indexing queue and worker pool
  - Projects do not share resources or slow each other down
  - UI shows "Indexing" badge on all projects currently being processed
* **Project-specific exclusions**: `.gitignore`, custom ignore patterns are applied per project

### Embedding Model Catalog & Selection

* **Global catalog**: The config database (`projects.db`) stores the authoritative list of available embedding models. Each entry records the model id, display name, vector dimension, on-disk size, typical RAM usage, CPU speed estimate, multilingual flag, code-quality score, download source, expected local path, and source format (ONNX, HF checkpoint, etc.).
* **Indexing view UI**: A dedicated card (before "Indexing Scope") displays the catalog. Users can pick a preset or open the "Add custom model" modal, which captures the same metadata fields and writes them back to the catalog table.
* **Per-project snapshot**: `ProjectConfig.EmbeddingModel` persists the selected model id plus the snapshot of its metadata inside `project_meta`. When a project database moves to another machine, the backend rehydrates the catalog entry from this snapshot and uses it to (re)download or convert the required files.
* **Download manager**: Each catalog entry tracks its download status (`pending`, `downloading`, `ready`, `missing`, `error`) plus the `localPath`. The Indexing view displays these badges and exposes a one-click "Download model" action that streams the artifact into `<AppDataDir>/models/<modelId>/` (Linux: `~/.local/share/codetextor`, macOS: `~/Library/Application Support/codetextor`, Windows: `%LOCALAPPDATA%\codetextor`).
* **Download progress + fallback**: The backend emits `embedding:download-progress` events while streaming each file so the frontend modal can display a determinate percentage (or a spinner if the total size is unknown). FastEmbed archives fall back to Hugging Face mirrors when the public CDN responds with 403/404, and presets that do not ship ONNX assets (for example `nomic-ai/nomic-embed-code`) stay in `pending` until the user supplies custom Source URIs.
* **Custom models**: The modal lets users record disk/RAM estimates, latency, multilingual flag, source URI (HTTP or local copy), and license notes. These values feed the download helper and are persisted in the catalog for reuse.
* **Per-project snapshot**: `ProjectConfig.EmbeddingModelInfo` captures the metadata (id, label, dimension, download status, local path, etc.) inside `project_meta`. When a project `.db` moves to another machine, CodeTextor can recreate the catalog entry and re-download the artifact using this snapshot.
* **FastEmbed backend**: Lightweight CPU-friendly models (BGE Small, GTE Small, etc.) ship preconfigured under the "FastEmbed" group. They still rely on ONNX Runtime (same requirement as the ONNX group), but cache/download artifacts automatically and expose a consistent API to the backend.
* **Runtime detection & reuse**: At startup the backend tries to initialize the ONNX Runtime shared library using the path stored in the config database (set via the Projects view). If detection succeeds, only one ONNX session per model id is kept in memory and the UI enables both "FastEmbed" and "ONNX" groups; if it fails every ONNX-dependent option is disabled and projects fall back to the mock embedding client until the runtime is installed and the app restarted.

---

## 9. MCP Server Responsibilities

Expose lightweight, composable tools (JSON-RPC or HTTP) usable by IDEs and AI agents.

### 🔹 MCP Tools with Project Scoping

**All MCP tools require a `projectId` parameter.** Requests without it are rejected with an error.

| Tool                                                    | Description                                      |
| ------------------------------------------------------- | ------------------------------------------------ |
| `retrieve(projectId, query, k, filters)`                | Semantic top-k retrieval from project's vector DB |
| `outline(projectId, path, depth)`                       | Get structural outline of a file in project       |
| `nodeAt(projectId, path, line)`                         | Return AST node at specific position in project   |
| `nodeSource(projectId, id, collapseBody)`               | Return source snippet of node from project        |
| `search(projectId, query, k)`                           | Semantic chunk search (cosine)                     |
| `searchSymbols(projectId, query, kinds)`                | Lexical symbol search within project              |
| `findDefinition(projectId, name)` / `findReferences(projectId, name)` | Optional, reference navigation within project     |

### 🔹 MCP Server Requirements

All endpoints must:

* **Require projectId**: Validate that the project exists and is accessible
* **Query correct database**: Use `indexes/<projectId>.db` for the specified project
* **Enforce path boundaries**: Only return results for files within the project's configured include paths
* **Return bounded results**: Limited by byte size (configurable per project)
* **Support pagination**: When returning large result sets
* **Never leak cross-project data**: Results must be strictly scoped to the requested project
* **Concurrent serving**: Support serving multiple projects simultaneously
  - MCP server handles requests for any indexed project via `projectId` parameter
  - Multiple projects can be actively indexing while MCP serves queries on all of them
  - Each query is isolated to its project's database

---

## 10. Frontend Guidelines

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

## 11. Testing and Documentation

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

## 12. Quality & Readability Targets

| Metric                   | Target      |
| ------------------------ | ----------- |
| Function doc coverage    | 100%        |
| File header doc coverage | 100%        |
| Average file length      | ≤ 300 lines |
| Lint/format errors       | 0           |
| Tests passing            | 100%        |

---

## 13. Design Philosophy Summary

* **Local-first:** No cloud dependencies; everything runs locally.
* **Modular:** Each concern isolated in its own package or component.
* **Transparent:** All data (chunks, symbols, embeddings) are inspectable.
* **Extensible:** Easy to integrate with other tools or MCP servers.
* **Readable:** Code designed to be understood by both humans and LLMs.

---

## 14. AI Collaboration Principles

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
