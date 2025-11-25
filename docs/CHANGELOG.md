# Changelog

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

All notable user-facing changes to CodeTextor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
  - Tree-sitter parsers for Go, Python, TypeScript, JavaScript, Vue, HTML, CSS, Markdown
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
  - Support for 9+ languages: Go, Python, TypeScript/JavaScript, HTML, CSS, Vue, Markdown, SQL, JSON
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
- Project cards now display the slug instead of the raw UUID
- Default excluded folders now mirror `.gitignore` (user overrides still respected)
- File preview table shows only the filename and wraps the relative path below it
- Navigation moved from sidebar buttons to tab-style interface
- Project selector now displayed as prominent H1 in header with "View All Projects" option
- ProjectsView refactored into smaller, focused components (reduced from 1029 to 407 lines)
- **Outline depth parameter removed**: Full tree always returned, user controls expand/collapse in UI
- **Markdown parser**: Now builds hierarchical heading structure instead of flat list
- **HTML parser**: Extracts all tags (not just semantic) with attribute information
- **Vue parser**: Preserves section hierarchy (template/script/style) with correct line numbers
- **Chunk type extended**: Added 15+ new fields for semantic metadata (language, symbol_name, symbol_kind, parent, signature, visibility, package_name, doc_string, token_count, is_collapsed, source_code)
- **Indexer initialization**: Now accepts eventEmitter parameter and creates SemanticChunker instance
- **StatsView enhanced**: Added chunks statistics section (total chunks, avg chunk size, distribution by symbol kind)
- **Database schema**: Normalized from path-based to integer file ID references (Migration 000006)
- **Footer statistics**: Changed from mock data to real backend API calls showing cumulative stats across all projects
- **StatsView refactored**: Removed Database Location and Indexing Status banners, now shows only essential statistics
- **API migration**: Replaced mockBackend with real backend calls throughout the application

### Fixed
- SQLite compatibility issue in slug migration (removed unsupported ALTER COLUMN DROP DEFAULT syntax)
- Robustness of the database migration for adding the `slug` column, preventing potential database corruption on startup
- Timestamp conversion from Unix seconds to JavaScript milliseconds (fixed incorrect project creation dates)
- Date formatting now uses system locale format with `toLocaleString()`
- IndexingView toggle now correctly reflects database state on mount and project switch
- "Go to Indexing" button now selects project before navigation
- Project switching in IndexingView now refreshes and displays correct indexing state
- **Unit tests for indexing operations**: Fixed mock implementations and test logic to properly validate all indexing API methods
- **Markdown links**: Now include correct line numbers (was missing position tracking)
- **Markdown heading ranges**: EndLine now extends to next same/higher-level heading for proper containment
- **Outline builder**: Fixed parent matching for duplicate symbol names (e.g., multiple `div` tags)
  - Now matches by both name AND line range containment instead of just name
  - Prevents incorrect parent assignment in files with many elements of same type
- **Vue template hierarchy**: Fixed flat structure issue where all HTML tags appeared as direct children of file instead of nested tree
- **Test database isolation**: Fixed `project_service_file_test.go` to use temporary HOME directory, preventing "Test Project" entries from polluting real database

---
