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

### Changed
- Project cards now display the slug instead of the raw UUID
- Default excluded folders now mirror `.gitignore` (user overrides still respected)
- File preview table shows only the filename and wraps the relative path below it

### Fixed
- SQLite compatibility issue in slug migration (removed unsupported ALTER COLUMN DROP DEFAULT syntax)
- Robustness of the database migration for adding the `slug` column, preventing potential database corruption on startup
- Timestamp conversion from Unix seconds to JavaScript milliseconds (fixed incorrect project creation dates)
- Date formatting now uses system locale format with `toLocaleString()`
- IndexingView toggle now correctly reflects database state on mount and project switch
- "Go to Indexing" button now selects project before navigation
- Project switching in IndexingView now refreshes and displays correct indexing state
- **Unit tests for indexing operations**: Fixed mock implementations and test logic to properly validate all indexing API methods

---
