# CodeTextor Architecture

## Design Philosophy

CodeTextor is designed around core principles that guide all architectural decisions:

1. **Local-First**: Zero cloud dependencies, complete data sovereignty
2. **Multi-Project Isolation**: Complete separation between codebases with no cross-contamination
3. **Embedded Intelligence**: Self-contained RAG-like system without external services
4. **Standard Protocols**: MCP (Model Context Protocol) for universal IDE/AI integration
5. **Developer Transparency**: All data inspectable, no black boxes

---

## High-Level Architecture

### Three-Layer Design

```
┌─────────────────────────────────────────┐
│         Frontend Layer (Vue)             │  User interface for project
│    Project management, search, stats     │  management and visualization
└─────────────────┬───────────────────────┘
                  │ Wails Bindings
┌─────────────────┴───────────────────────┐
│         Backend Layer (Go)               │  Code analysis, embedding,
│   Parsing, chunking, indexing, MCP       │  storage, and retrieval
└─────────────────┬───────────────────────┘
                  │
┌─────────────────┴───────────────────────┐
│       Storage Layer (SQLite)             │  Configuration and per-project
│   Config DB + Per-Project Vector DBs     │  vector indexes
└─────────────────────────────────────────┘
```

**Why this architecture?**

- **Frontend/Backend Separation**: UI logic separate from analysis logic enables future headless mode
- **Go Backend**: Performance for parsing large codebases, native cross-platform support
- **SQLite Storage**: Embedded database eliminates setup complexity, enables offline-first operation
- **Wails Integration**: Single binary distribution, native performance with modern web UI

---

## Multi-Project Architecture

### Design Decision: Complete Isolation

**Problem:** How to support multiple codebases without data mixing?

**Solution:** One database per project, explicit project scoping in all APIs.

**Why not a single database with project_id filtering?**

- **Simpler Queries**: No need to filter every query by project_id
- **Easier Backup**: Copy single `.db` file to backup/restore a project
- **Independent Lifecycle**: Delete, archive, or migrate projects without affecting others
- **Performance**: Smaller indexes per project, faster queries
- **Security**: Impossible to accidentally leak data between projects

### Storage Strategy

```
Configuration Storage:
  ~/.local/share/codetextor/config/projects.db
  └── Tables: app_config
      └── Contains: general app metadata such as the currently selected project

Project Storage (per-project):
  ~/.local/share/codetextor/indexes/
  ├── project-codetextor.db  ← Isolated vector DB for project "codetextor" (slug-based naming)
  │   ├── Tables: chunks, files, symbols, project_meta
  │   └── Contains: embeddings, parsed code, AST symbols, and the project-specific configuration
  ├── project-my-app.db      ← Isolated vector DB for project "my-app"
  └── ...
```

**Implementation Details:**
- Each per-project database is created automatically on project creation
- Migrations for per-project DBs are embedded in `backend/internal/store/vector_migrations/`
- Global config DB only stores app-level metadata (selected project, future global settings)
- **IMPORTANT:** No `project_id` columns in per-project tables - isolation via separate database files
- Vector stores use WAL mode for concurrent access, single connection pool for ACID guarantees
- `.gitignore` files under each project root are parsed into glob patterns and used as the default exclude list unless the user overrides it.

**Benefits:**
- Projects are portable (copy `.db` + config entry)
- No risk of cross-contamination
- Each project can have different indexing parameters
- Simpler queries (no filtering by `project_id` needed)
- Simpler to reason about data boundaries

---

## Core Subsystems

### 1. Project Management

**Purpose:** Abstract multi-project support as a tenant system.

**Key Concepts:**
- Projects are configuration containers, not tied to single directory
- Each project defines its own include/exclude paths (can span multiple directories)
- Selection state and indexing state managed in database (not localStorage) for consistency
- Auto-selection fallback when current project deleted

**Why database-based state?**
- Single source of truth accessible from frontend and backend
- Survives project deletion (auto-selects next available)
- Transactional consistency (only one selected at a time)
- Persistent indexing state survives app restarts
- No desync between UI and backend state

### 2. Semantic Chunking Engine

**Purpose:** Transform raw code into semantically meaningful retrieval units.

**Design Principles:**
- **Tree-sitter Parsing**: Language-agnostic AST extraction
- **Semantic Boundaries**: Chunks align with code structure (functions, classes, modules)
- **Context Enrichment**: Attach file/package info, merge comments, collapse long bodies
- **Adaptive Sizing**: Split large chunks, merge small ones for optimal embedding

**Why not simple line-based chunking?**
- Semantic units preserve logical context
- Better embedding quality (complete thoughts vs arbitrary splits)
- Enables accurate code navigation (jump to function definition)

### 3. Vector Indexing

**Purpose:** Enable semantic code search without external services.

**Design Decisions:**
- **SQLite-vec Extension**: Embedded vector search, no separate database
- **Per-Project Indexes**: Complete isolation between codebases
- **Incremental Updates**: Only re-index changed files (hash + mtime tracking)

**Why SQLite-vec vs dedicated vector DB?**
- Embedded: No separate server to manage
- Portable: Single `.db` file per project
- Proven: SQLite reliability + vector search capabilities
- Offline: Works without network access

### 4. MCP Server

**Purpose:** Expose code intelligence to external tools via standard protocol.

**Architecture Goals:**
- **Protocol-First**: MCP standard ensures compatibility with any MCP client
- **Project-Scoped**: Every API call requires explicit `projectId` parameter
- **Resource-Bounded**: Configurable limits prevent resource exhaustion
- **Path-Validated**: Enforce include path boundaries for security

**Why MCP vs custom API?**
- Standard protocol means broad IDE/tool support
- No vendor lock-in
- Community-driven protocol evolution

---

## Data Flow Examples

### Indexing Flow

```
User configures project paths
    ↓
Backend watches file system
    ↓
File change detected
    ↓
Tree-sitter parses file → AST
    ↓
Chunker extracts semantic units
    ↓
Embedding model generates vectors
    ↓
Store chunks + vectors in project's SQLite-vec DB
    ↓
UI updates index stats
```

**Key Decisions:**
- Incremental: Only changed files re-indexed
- Async: Background goroutine per project (concurrent indexing)
- Isolated: Each project has dedicated file watcher

### Retrieval Flow

```
MCP client sends query + projectId
    ↓
Validate projectId exists
    ↓
Embed query → vector
    ↓
Search project's vector DB (top-k similarity)
    ↓
Apply path boundary filters
    ↓
Return chunks with metadata
```

**Key Decisions:**
- Explicit projectId prevents accidental cross-project queries
- Path validation ensures results stay within configured scope
- Byte caps prevent unbounded responses

---

## Frontend Component Architecture

**Purpose:** Provide modular, maintainable UI components following Vue 3 best practices.

**Component Structure:**

```
/frontend/src/
  /components/          ← Reusable UI components
    ProjectCard.vue     ← Project card for grid view
    ProjectTable.vue    ← Project table for list view
    ProjectFormModal.vue    ← Create/edit project form
    DeleteConfirmModal.vue  ← Deletion confirmation
    ProjectSelector.vue     ← Project dropdown in header
  /views/               ← Page-level components
    ProjectsView.vue    ← Project management (orchestrator)
    IndexingView.vue    ← File indexing interface
    SearchView.vue      ← Semantic search interface
    OutlineView.vue     ← Code structure browser
    StatsView.vue       ← Project statistics
    MCPView.vue         ← MCP server management
  /composables/         ← Shared logic
    useCurrentProject.ts   ← Current project state
    useNavigation.ts       ← View routing
```

**Key Design Patterns:**

1. **Component Composition**: Large views are decomposed into smaller, focused components
   - Example: ProjectsView.vue delegates to ProjectCard, ProjectTable, ProjectFormModal
   - Each component has a single responsibility (≤300 lines per file)

2. **Props Down, Events Up**: Standard Vue pattern for parent-child communication
   - Props: Pass data and configuration down
   - Events: Emit user actions up for parent to handle

3. **Shared State via Composables**:
   - `useCurrentProject()`: Manages selected project across views
   - `useNavigation()`: Handles tab/view switching
   - Avoids global state pollution

4. **Responsive Design**:
   - Tab navigation for desktop (≥1024px)
   - Hamburger menu for mobile (<1024px)
   - Grid/table view toggle for project lists

**Component Guidelines:**
- Each component has JSDoc header explaining purpose
- All functions documented with input/output types
- CSS scoped to component to prevent leaks
- TypeScript for type safety

---

## Technology Choices

### Why Wails?

**Requirements:**
- Native performance (parsing large codebases)
- Cross-platform (Linux, macOS, Windows)
- Modern UI capabilities
- Single binary distribution

**Alternatives Considered:**
- Electron: Too heavy, not truly native
- Tauri: Rust complexity, less mature ecosystem
- Web server + browser: Extra complexity, no offline-first guarantee

**Decision:** Wails provides Go backend performance with web UI flexibility, single binary output.

### Why SQLite?

**Requirements:**
- Embedded (no separate database server)
- Reliable (codebases are critical data)
- Cross-platform
- Extensible (vector search capability)

**Alternatives Considered:**
- PostgreSQL + pgvector: Requires separate server, overkill for local-first
- Standalone vector DB (Chroma, Qdrant): Separate service to manage
- File-based JSON: No query capabilities, poor performance

**Decision:** SQLite is battle-tested, embedded, and SQLite-vec extension provides vector search.

### Why golang-migrate?

**Requirements:**
- Schema evolution as app develops
- Embedded migrations (no external files at runtime)
- Version tracking
- Rollback capability

**Alternatives Considered:**
- Custom migration system: Reinventing the wheel, error-prone
- No migrations: Manual schema updates, data loss risk

**Decision:** Industry-standard tool, embedded support, automatic dirty state detection.

---

## Security Model

### Path Boundaries

**Threat:** Malicious or accidental access to files outside project scope.

**Mitigation:**
- Each project defines include path allowlist
- All file operations validate paths against allowlist
- Directory traversal prevention (`..` not allowed)
- Symbolic links resolved before validation

### Project Isolation

**Threat:** Data leakage between projects.

**Mitigation:**
- Separate SQLite-vec database per project
- MCP API requires explicit projectId (no default project)
- Queries fail if projectId invalid/missing
- No shared state between projects

### Resource Protection

**Threat:** Resource exhaustion from large queries.

**Mitigation:**
- Configurable byte caps per project
- Top-k result limits
- Per-project request throttling
- Concurrent indexing limits

---

## Performance Considerations

### Scalability Targets

- **Large Codebases**: 100k+ files per project
- **Fast Queries**: <100ms for semantic search
- **Incremental Indexing**: <1s for small file changes
- **Concurrent Projects**: Multiple projects indexing simultaneously

### Optimization Strategies

**Parsing:**
- Tree-sitter native parsers (C bindings)
- Parallel file processing per project
- Incremental updates (hash-based change detection)

**Indexing:**
- Batch vector insertions
- Per-project goroutines (no global lock)
- Configurable chunk size (balance granularity vs volume)

**Retrieval:**
- SQLite-vec optimized indexes
- Path filters applied before similarity search
- Result pagination for large matches

---

## Future Architecture Evolution

### Potential Extensions

**Not committed, but architecturally compatible:**

1. **Language Server Protocol (LSP)**: Expose code intelligence to IDEs directly
2. **Distributed Indexing**: Split large projects across multiple machines
3. **Cloud Sync**: Optional encrypted backup to user's cloud storage
4. **Plugin System**: User-defined chunking strategies or embedding models

**Why not now?**
- Focus on core functionality first
- Avoid premature abstraction
- Validate use cases before extending

---

## Design Patterns

### Composition Over Inheritance

- Go's interface-based design
- Small, focused interfaces (e.g., per-project metadata reader, `ChunkExtractor`)
- Easy to mock for testing, swap implementations
  - Project configuration lives inside each project's SQLite-vec database (`project_meta` table), so the same metadata travels with the vector data.

### Explicit Over Implicit

- No global state (pass dependencies explicitly)
- Require projectId in all MCP calls (no "current project")
- Validate all inputs at boundaries

### Simple Over Complex

- Prefer straightforward solutions
- Avoid clever abstractions unless necessary
- Code should be readable by both humans and LLMs

---

## Conclusion

CodeTextor's architecture prioritizes:

1. **Simplicity**: Embedded components, minimal dependencies
2. **Isolation**: Multi-project support without cross-contamination
3. **Performance**: Native code, optimized data structures
4. **Standards**: MCP protocol for broad compatibility
5. **Transparency**: All data inspectable, understandable

These principles guide all implementation decisions and should be preserved as the project evolves.

---

**Last Updated:** 2025-11-07
**Version:** 0.1.0-dev
