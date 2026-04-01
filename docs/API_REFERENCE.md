# CodeTextor API Reference

This document only tracks **external** APIs exposed by CodeTextor. Internal
Wails bindings between the Go backend and the Vue frontend are implementation
details and are covered in `DEV_GUIDE.md` / `ARCHITECTURE.md` instead.

---

## MCP (Model Context Protocol) Server

CodeTextor ships a streamable **HTTP** MCP server powered by the official
`modelcontextprotocol/go-sdk`. It serves code context from the local per-project
index; requests are read-only.

### Transport & URLs

- Protocol: `http`
- Default bind: `127.0.0.1:3030` (configurable in the MCP tab)
- Base path: `http://<host>:<port>/mcp/<projectId>`
  - Requests without `<projectId>` return an error (`projectId is required`)
- Max connections: configurable; defaults to 32
- No authentication (local-only)

### Tools

| Tool                | Purpose                                                                   |
| ------------------- | ------------------------------------------------------------------------- |
| `getProjectDetails` | Overview of project scope, configuration, and statistics                  |
| `listFiles`         | Explore file tree with optional path/extension filtering                  |
| `search`            | Semantic natural language search across indexed code chunks               |
| `semanticSearchFiles` | High-level exploration: suggests the most relevant files for a concept  |
| `outline`           | Hierarchical symbol tree (classes, functions) for a file                  |
| `nodeSource`        | Precise source snippets for identified symbols/chunks                     |
| `getRecentChanges`  | Show recently modified files (VCS) and recently indexed files (DB)        |
| `grepSearch`        | Literal or regex search across files (precise and OS-independent)         |
| `findReferences`    | Find all locations (file, line) where a symbol is referenced or used      |
| `getCallGraph`      | Get the hierarchical call relationships for a specific function           |
| `findTodos`         | Discover TODO, FIXME, HACK, XXX, and NOTE comments across the project     |
| `getPackageGraph`   | Get a high-level overview of package dependencies and coupling            |
| `findImplementations` | Discover all classes or interfaces that implement a specific interface |

#### `getProjectDetails`

- **Input**: `{}` (empty object)
- **Response**: `{ id, name, description, rootPath, fileExtensions, summary, stats }`
  - `summary` provides a high-level overview (main languages, entry points, packages).
  - `stats` provides concise numeric metrics.

#### `listFiles`

- **Input**: `{ path?: string, extension?: string, recursive?: boolean }`
- **Response**: `{ files: { path, size }[] }`
  - `path` is relative to the project root.
  - Validates that `path` exists; returns an error with the project root if it does not.

#### `search`

- **Input**: `{ query: string, k?: number (1-50, default 8) }`
- **Response**: `{ results: { path, chunks: mcpChunk[] }[], total: number }`
  - Results are grouped by file path to reduce token usage.
  - `mcpChunk` includes `id`, `content`, `similarity`, `start` (line), `end` (line), `symbol`, `kind`.

#### `semanticSearchFiles`

- **Input**: `{ query: string, k?: number (1-20, default 5) }`
- **Response**: `{ results: { path, score, nodes: { id, score }[] }[] }`
  - Returns the most relevant files for a conceptual query (e.g., "Where is authentication?").
  - `score` indicates relevance (highest similarity of any chunk in the file).
  - `nodes` contains the top 5 relevant node IDs and their individual scores for direct fetching.

#### `outline`

- **Input**: `{ path: string, depth?: number }` where `path` is relative to the project root.
- **Response**: `{ outline: mcpOutlineNode[] }`
  - `mcpOutlineNode` includes `id`, `name`, `kind`, `start`, `end`, and nested `children`.

#### `nodeSource`

- **Input**: `{ id: string[], collapseBody?: boolean }` where `id` is an array of identifiers.
- **Response**: `{ results: { id, path, source, start, end, language?, symbol? }[] }`
  - Fetches source snippets for one or more identifiers.
  - If `collapseBody` is true, long snippets are truncated with a placeholder.
  - Each result object includes the original `id` to clarify multiple requests.

#### `getRecentChanges`

- **Input**: `{ limit?: number (default 10) }`
- **Response**: `{ indexed: { p, t }[], workingCopy: { p, s }[], vcs?: string }`
  - `indexed`: Files recently updated in the CodeTextor index (`p`: relative path, `t`: unix timestamp).
  - `workingCopy`: Real-time modifications from Git or SVN (`p`: relative path, `s`: status code like "M", "A").
  - `vcs`: The active Version Control System detected (e.g., "git", "svn").

#### `grepSearch`

- **Input**: `{ query: string, isRegex?: boolean, path?: string (relative), limit?: number (default 100) }`
- **Response**: `{ results: { path, matches: { line, content }[] }[], total: number, timeMs: number }`
  - High-precision search for exact terms or regex patterns.
  - Validates that `path` exists; returns an error with the project root if it does not.
  - Results are grouped by file to minimize path redundancy.
  - `content` is automatically truncated to 500 characters to reduce token consumption.
  - Maximum of 50 matches per file.

#### `findReferences`

- **Input**: `{ nodeID?: string, symbolName?: string, path?: string }`
- **Response**: `{ files: { path, references: { line, content }[] }[] }`
  - Highly efficient exact reference tracking without fuzzy similarity.
  - Grouped by file. Includes exact line number and code snippet where the symbol is referenced.
  - Provide `path` alongside `symbolName` if multiple symbols share the same name across files.

#### `getCallGraph`

- **Input**: `{ nodeID?: string, symbolName?: string, path?: string, direction?: "incoming" | "outgoing" | "both", depth?: number }`
- **Response**: `{ direction: string, root: { symbol: string, location: string, content?: string, calls: CallDetails[] } }`
  - Maps architectural call flows hierarchically. Nested `calls` array makes traversal native for LLM contexts.
  - `location` points to the *definition* of the called/calling function (format `path:line`).
  - `content` explicitly extracts the source code context snippet where the call took place.

#### `findTodos`

- **Input**: `{}` (empty object)
- **Response**: `{ todos: { type, content, path, line }[] }`
  - Scans for standard comment tags: `TODO`, `FIXME`, `HACK`, `XXX`, `NOTE`.
  - Returns the exact location and the full comment text.

#### `getPackageGraph`

- **Input**: `{ depth?: number (default 0, unlimited) }`
- **Response**: `{ [sourcePackage: string]: { [targetPackage: string]: weight: number } }`
  - Compact adjacency map representing package-level coupling.
  - `weight` is the number of symbol references between the two packages.
  - `depth` aggregates folders to a specific level (e.g. `depth: 1` groups all `backend/pkg/*` under `backend`).
  - External dependencies are prefixed with `@external/` followed by the library name (e.g., `@external/go/fmt`).
  - Respects system-level exclusions (gitignore, etc.) by using the already indexed data.

#### `findImplementations`

- **Input**: `{ nodeID?: string, symbolName?: string, path?: string }`
- **Response**: `{ implementations: { symbolName, location, content }[] }`
  - Specialized tool for OOP languages (Java, PHP, TS).
  - Finds all symbols that explicitly `implement` or `extend` the target interface/class.
  - **Go Support**: Returns a warning suggesting `findReferences` instead, as Go interfaces are implicit.
  - `location` is the `path:line` of the implementor.
  - `content` is a source snippet of the implementor's definition.

### Status & Tool Events

- `mcp:status`: emitted periodically with `{ isRunning, uptime, activeConnections, totalRequests, averageResponseTime, lastError? }`.
- `mcp:tools`: emitted when tool enablement changes.

---

## Notes for contributors

- Public APIs live here; internal Wails bindings stay documented in `DEV_GUIDE.md`.
- Update this file whenever MCP tool parameters or transport details change.
