# Changelog

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

All notable user-facing changes to CodeTextor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
  - Cached outline trees in SQLite (`file_outlines` table)
  - File tree browser with per-file outline loading
  - Recursive symbol tree rendering with expand/collapse
  - Icons per symbol kind (functions, classes, headings, etc.)
  - Line number ranges displayed for each symbol
- **Continuous Outline Updates**: Automatic outline refresh during file modifications
  - 10-second debouncing to coalesce rapid file changes
  - Per-project file watchers with fsnotify
  - Automatic database updates after debounce period
  - Thread-safe timer management per file

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

---
