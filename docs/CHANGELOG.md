# Changelog

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

All notable user-facing changes to CodeTextor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- **`findTodos` Tool Optimization (Breaking Change)**: Refactored the tool to return a structured map categorized by marker type (TODO, FIXME, HACK, etc.) instead of a flat list. This change significantly reduces token overhead by using semantic Symbol IDs and includes a new `category` input filter for surgical discovery.
- **`grepSearch` Tool Optimization (Breaking Change)**: Refactored the tool output from a nested JSON structure to a compact tabular format (`[File, Line, Content]`). This significantly reduces token consumption for AI agents while maintaining visibility of matching code lines.

- **Enhanced `nodeSource` Tool (Fuzzy Matching)**: Implemented a robust resolution system for node IDs. When an exact match fails, the server now attempts to interpret user intent by parsing "talking" IDs (`path|name`, `path|L-range`), searching for relevant symbols or line intersections, and returning multiple candidates if the query is ambiguous.
- **Enhanced `listFiles` Tool**: Rewritten for tabular output (`Lang`, `Size`, `Lines`, `Sym`) and added support for `extension` and `depth` filters.
- **Recursive Sub-Language Parsing**: Improved support for nested code (e.g., PHP -> HTML -> JS) using the `"detect"` keyword in TOML configurations and automated `go-enry` detection.
- **Symbol Language Tracking**: Added `language` column to the `symbols` table for precise multi-language symbol identification.
- **Improved `ti` Debug Tool**: Enhanced with automatic config loading and recursive sub-language extraction testing capabilities.
- **Dynamic TOML Parsing Engine**: Transitioned the core extraction logic to a fully declarative, query-based system. Language-specific features (symbols, imports, usages, metadata) are now defined in `.toml` files using Tree-sitter queries, enabling instant updates and new language support without backend recompilation.
- **Human-Readable Symbol IDs**: Replaced opaque SHA-256 hashes with semantic, human-readable IDs in the format `path|Lstart-end|name`. This improves database transparency, simplifies AI cross-referencing, and reduces token overhead.
- **Atomic File Persistence**: Implemented `InsertFileTasksInTransaction` to ensure that all file artifacts (chunks, symbols, outlines, and usages) are saved in a single, durable SQLite transaction, eliminating referential integrity errors (Foreign Key constraints).
- **Two-Stage Indexing Pipeline**: Optimized processing throughput by decoupling CPU-heavy tasks (parsing/tokenization) from GPU/DB operations (embedding/persistence) using a prioritized semaphore-controlled queue.
- **Tree-sitter Inspector (ti)**: Introduced a specialized CLI tool for real-time validation of TOML configurations and AST inspection, dramatically accelerating parser development and debugging.
- **Automated ONNX Runtime Setup**: One-click download and installation of required libraries for Windows, Linux, and macOS directly from the Settings UI.
- **Heavy Embedding Rework (GPU/CPU Optimization)**:
  - **Dynamic Batch Scaling**: Automatically adjusts embedding batch sizes based on available VRAM to maximize GPU throughput while preventing Out-of-Memory (OOM) errors.
  - **Power-of-2 Alignment**: Batch sizes are rounded to the nearest power of 2 for optimal hardware thread alignment and constant memory performance.
  - **Embedding Task Priorities**: Introduced a priority-based queue (`PriorityHigh`, `PriorityNormal`, `PriorityLow`) to ensure real-time user actions (like search) always remain responsive during background indexing.
  - **Improved Vector Store Concurrency**: Better handling of project indexing states and parallel processing.
- **Nested Code Parsing**: Automatically detects and parses embedded code (HTML/JS in PHP, SQL in Go/Python, etc.) across all 10 supported languages.
  - Integration with `go-enry` for statistical language detection.
  - Precision parsing using Tree-sitter's `SetIncludedRanges` to maintain absolute file offsets.
  - Added `SubLanguageManager` as a central coordinator for cross-parser delegation.
- **PHP Language Support**: Added full support for PHP parsing and chunking.
  - Extracts namespaces, classes, interfaces, traits, functions, methods, and `use` statements.
  - Hierarchical outline support for PHP files.
- **Permanent Tokenizer Fixes**: Integrated safety patches for `sugarme/tokenizer` directly into `fix_vendor.ps1`.
  - Prevents "index out of range" panics during normalization.
  - Fixes are automatically reapplied after `go mod vendor` updates.
- Structural Project Summary: `getProjectDetails` now provides a high-level overview including main languages, entry points, and detected packages.
- Automatic Package Extraction: Added metadata extraction to parsers (starting with Go) to identify package/module names during indexing.
- Grep Search MCP Tool: Precise, OS-independent textual search with regex support now available for AI agents.
- **Static Analysis Tools**: Added `findReferences`, `getCallGraph`, and `getPackageGraph` MCP tools.
  - `findReferences` pinpoints accurate definitions and usages across the codebase by parsing explicit call edges.
  - `getCallGraph` returns an LLM-friendly nested JSON tree representing the architectural call hierarchy.
  - `getPackageGraph` provides a macro-level dependency map between packages/folders, supporting depth-based aggregation and detailed external library visibility (`@external/package`).
  - `findImplementations` allows discovering all classes or interfaces that implement a specific target in OOP languages (PHP, TypeScript, Java).
  - Workaround implemented to handle recursive schemas with the MCP Go SDK.
- **TypeScript Parser Enhancement**: Fixed a bug where class methods lost their reference to the parent symbol in the outline, restoring hierarchical accuracy for TypeScript/JavaScript files.
- Lean MCP Output: Optimized `getProjectDetails` response by removing redundant technical metadata and configuration paths (`includePaths`, `excludePatterns`) for maximum token efficiency.
  - Automatic platform detection and NuGet (ZIP) / GitHub (TGZ) asset selection
  - Mandatory SHA256 integrity verification via an embedded runtime manifest
  - Automatic configuration update and "Restart Required" notifications
- **ONNX Runtime 1.24.4 Update**: Improved GPU support and simplified cross-platform configuration
  - **Windows**: Native **DirectML** support (GPU acceleration without CUDA/cuDNN)
  - **macOS**: Optimized **CoreML** support for Apple Silicon (M1/M2/M3)
  - **Linux/Windows**: Support for **CUDA 12.x** and **CUDA Graph**
- Shared `utils.ShouldSkipPath` utility for consistent file/directory exclusion (hidden, `.git`, `.gitignore`, custom patterns) across both indexing and live watchers
- Concurrency semaphore for indexing to limit CPU usage and maintain system responsiveness (50% target core utilization)
- Live MCP server status in the application footer via Wails events (`mcp:status`)
- Toggle to automatically include patterns from `.gitignore` in the project exclusion lists
- FastEmbed model download fallback to HuggingFace for improved reliability
- Track and display the active indexing engine (DirectML, CoreML, CUDA, or CPU) in the user interface
- Streamable HTTP MCP server powered by the official go-sdk with persisted config (host/port/protocol/autostart/max connections), lifecycle management (start/stop), and periodic status/tool events (`mcp:status`, `mcp:tools`)
- MCP tools `search`, `semanticSearchFiles`, `outline`, and `nodeSource` exposed per-project via `/mcp/<projectId>`; Wails bindings + Vue MCP view now surface live metrics, tool list, and ready-to-paste client snippets (Codex CLI, Claude Code, VS Code/Cursor/Windsurf)
- Backend `GetChunkByID` API (VectorStore + ProjectService) to fetch canonical chunk/source snippets for MCP `nodeSource`
- New `SelectFile` Wails binding for filtered file pickers alongside existing directory selector
- Embedding model catalog with per-project selection snapshot, custom-model modal, download manager storing artifacts under `<AppDataDir>/models/<modelId>`, and ONNX Runtime-based embedding generation (automatic tokenizer/ONNX downloads, shared sessions per model)
- Multi-project management (create, edit, delete, select) with per-project SQLite databases
- Human-readable project slugs and slug auto-generation/validation
- Persistent indexing state and visual feedback in the Projects view
- Continuous indexing toggle with backend synchronization
- Cross-platform configuration directory plus per-project `project_meta` storage
- Indexing infrastructure (manager, start/stop controls, progress polling, background workers)
- Indexing view improvements (folder pickers, include/exclude pills, extension filters, file preview)
- Auto-save of project configuration changes
- `.gitignore` parsing to seed default exclude folders
- Initial UI mockups (Projects, Indexing, Stats, Search)
- Wails-based Go↔TS bindings, Vue 3 frontend, Go backend, SQLite storage
- Developer documentation (DEV_GUIDE, ARCHITECTURE, API reference) and automated tests for core components
- Tab-based navigation for desktop view (Indexing, Search, Outline, Stats, MCP)
- Responsive hamburger menu for mobile/narrow viewports (<1024px)
- Grid/table view toggle for project list with persistent selection
- Modular component architecture (ProjectCard, ProjectTable, ProjectFormModal, DeleteConfirmModal)
- **Outline System**: Hierarchical code structure visualization
  - Tree-sitter parsers for Go, Python, TypeScript, JavaScript, Vue, HTML, CSS, Markdown, PHP
- Cached outline trees in SQLite (`outline_nodes` + `outline_metadata` tables)
  - File tree browser with per-file outline loading
  - Recursive symbol tree rendering with expand/collapse
  - Icons per symbol kind (functions, classes, headings, etc.)
  - Line number ranges displayed for each symbol
- **Continuous Outline Updates**: Automatic outline refresh during file modifications
  - 10-second debouncing to coalesce rapid file changes
  - Per-project file watchers with fsnotify
  - Automatic database updates after debounce period
  - Thread-safe timer management per file
- **Semantic Chunking System**: Intelligent code chunking for embedding
  - Tree-sitter-based semantic boundaries (functions, classes, methods, etc.)
  - Context enrichment with metadata headers (file, language, symbol info, comments)
  - Adaptive sizing: merge small chunks (<100 tokens), split large chunks toward ~400 tokens (hard max 800)
  - Local variables/constants are pruned; only semantically relevant top-level symbols become chunks
  - Support for 10+ languages: Go, Python, TypeScript/JavaScript, HTML, CSS, Vue, Markdown, SQL, JSON, PHP
  - Token estimation with enrichment overhead accounting (~50 tokens for metadata)
  - Configurable via `ChunkConfig` (MaxChunkSize, MinChunkSize, MergeSmallChunks, IncludeComments)
  - Public API: `SemanticChunker.ChunkFile()` with fallback to line-based chunking for unsupported formats
  - Integrated with indexer: automatic semantic chunking for supported files
  - 38 unit tests covering enrichment, semantic chunking, and integration
  - 6 documented usage examples
- **Chunks View**: Visual inspection of semantic chunks
  - File tree browser with per-file chunk loading
  - Chunk metadata display (symbol name/kind, line ranges, token counts)
  - Dual content view: enriched content (embedded) + original source code
  - Metadata panel showing language, visibility, package, signature, docstring
  - ChunkTreeNode and ChunkContentViewer components
- **Database Schema Enhancements**:
  - Migration 000004: Extended chunks table with 11 semantic metadata fields
  - Migration 000005: Unique constraint on chunks (file_id, line_start, line_end)
  - Migration 000006: Normalized schema with integer file IDs, foreign keys, chunk_symbols mapping table
  - File ID caching system (thread-safe in-memory cache) for performance
- **Indexing Performance Improvements**:
  - Hash-based change detection (SHA-256) to skip unchanged files
  - Modified time comparison for quick change detection
  - Incremental chunk updates (delete old, insert new)
  - Concurrent file processing with semaphore (10 parallel operations)
  - File ID resolution caching to reduce database queries
- **New Utility Functions**:
  - `utils.ComputeHash()`: SHA-256 file content hashing for change detection
  - `VectorStore.resolveFileID()`: Cached file ID resolution with auto-creation
  - `VectorStore.createPlaceholderFile()`: Automatic file record creation on demand
- **Statistics System**: Real-time project metrics and multi-project aggregation
  - `VectorStore.GetStats()`: Per-project statistics (files, chunks, symbols, database size, last indexed timestamp)
  - `ProjectService.GetProjectStats(projectID)`: Project-specific statistics with indexing status
  - `ProjectService.GetAllProjectsStats()`: Cumulative statistics across all projects
  - `App.GetProjectStats()` and `App.GetAllProjectsStats()`: Wails bindings for frontend
  - Frontend API wrappers: `backend.getProjectStats()` and `backend.getAllProjectsStats()`
  - Footer statistics: Real-time cumulative metrics (files, chunks, symbols) across all projects
  - Stats View: Per-project detailed statistics with refresh capability
  - Automatic statistics refresh every 5 seconds in footer
  - Statistics include indexing progress tracking when projects are being indexed

### Changed

- File outline requests now auto-generate and persist outlines on demand (Tree-sitter) instead of erroring when no cached outline exists
- Search results and chunk lookups keep `embedding` as an empty slice (never null) for MCP schema compatibility
- Project cards now display the slug instead of the raw UUID
- Default excluded folders now mirror `.gitignore` (user overrides still respected)
- File preview table shows only the filename and wraps the relative path below it
- Navigation moved from sidebar buttons to tab-style interface
- Project selector now displayed as prominent H1 in header with "View All Projects" option
- ProjectsView refactored into smaller, focused components (reduced from 1029 to 407 lines)
- **Outline depth parameter removed**: Full tree always returned, user controls expand/collapse in UI
- **Markdown parser**: Now builds hierarchical heading structure instead of flat list
- **HTML parser**: Extracts all tags (not just semantic) with attribute information
- **Vue parser**: Refactored to use `SubLanguageManager` for dynamic delegation of template/script/style sections, preserving absolute line numbers using Tree-sitter's `SetIncludedRanges` API.
- **Chunk type extended**: Added 15+ new fields for semantic metadata (language, symbol_name, symbol_kind, parent, signature, visibility, package_name, doc_string, token_count, is_collapsed, source_code)
- **Indexer initialization**: Now accepts eventEmitter parameter and creates SemanticChunker instance
- **StatsView enhanced**: Added chunks statistics section (total chunks, avg chunk size, distribution by symbol kind)
- **Database schema**: Normalized from path-based to integer file ID references (Migration 000006)
- **Footer statistics**: Changed from mock data to real backend API calls showing cumulative stats across all projects
- **StatsView refactored**: Removed Database Location and Indexing Status banners, now shows only essential statistics
- **API migration**: Replaced mockBackend with real backend calls throughout the application
- **Automatic Parser Registration**: The parsing engine now dynamically discovers and registers all language parsers from TOML configurations at startup.

### Removed

- Legacy hardcoded parsers for Go, Python, and TypeScript (`go_parser.go`, `python_parser.go`, `typescript_parser.go`).
- Hardcoded parser selection logic in favor of dynamic extension-based discovery.
- `EnableShadowParsing` and associated parity check background tasks.

### Fixed

- **Virtual Symbol Consistency**: Implemented automatic garbage collection for orphaned external references (`@external/`) to maintain database integrity.
- Resolved "nil pointer dereference" errors during project loading and startup by improving ONNX environment initialization checks
- Fixed MCP status label in the footer incorrectly displaying "Stopped" when the server was active
- Improved project selection robustness when no projects exist or during simultaneous initialization
- SQLite compatibility issue in slug migration (removed unsupported ALTER COLUMN DROP DEFAULT syntax)
- Robustness of the database migration for adding the `slug` column, preventing potential database corruption on startup
- Timestamp conversion from Unix seconds to JavaScript milliseconds (fixed incorrect project creation dates)
- Date formatting now uses system locale format with `toLocaleString()`
- IndexingView toggle now correctly reflects database state on mount and project switch
- "Go to Indexing" button now selects project before navigation
- Project switching in IndexingView now refreshes and displays correct indexing state
- **Outline builder**: Fixed parent matching for duplicate symbol names (e.g., multiple `div` tags) in XML-based and PHP files.
  - Now matches by both name AND line range containment instead of just name
  - Prevents incorrect parent assignment in files with many elements of same type
- **Vue template hierarchy**: Fixed flat structure issue where all HTML tags appeared as direct children of file instead of nested tree
- **Test database isolation**: Fixed `project_service_file_test.go` to use temporary HOME directory, preventing "Test Project" entries from polluting real database

---
