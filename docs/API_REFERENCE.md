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

| Tool                | Purpose                                                                 |
| ------------------- | ----------------------------------------------------------------------- |
| `getProjectDetails` | Overview of project scope, configuration, and statistics                |
| `listFiles`         | Explore file tree with optional path/extension filtering                |
| `search`            | Semantic natural language search across indexed code chunks             |
| `semanticSearchFiles`| High-level exploration: suggests the most relevant files for a concept |
| `outline`           | Hierarchical symbol tree (classes, functions) for a file                |
| `nodeSource`        | Precise source snippets for identified symbols/chunks                    |
| `getRecentChanges`  | Show recently modified files (VCS) and recently indexed files (DB)        |

#### `getProjectDetails`

- **Input**: `{}` (empty object)
- **Response**: `{ id, name, description, rootPath, includePaths, excludePatterns, fileExtensions, stats }`

#### `listFiles`

- **Input**: `{ path?: string, extension?: string, recursive?: boolean }`
- **Response**: `{ files: { path, size }[] }`
  - `path` is relative to the project root.

#### `search`

- **Input**: `{ query: string, k?: number (1-50, default 8) }`
- **Response**: `{ results: { path, chunks: mcpChunk[] }[], total: number }`
  - Results are grouped by file path to reduce token usage.
  - `mcpChunk` includes `id`, `content`, `similarity`, `start` (line), `end` (line), `symbol`, `kind`.

#### `semanticSearchFiles`

- **Input**: `{ query: string, k?: number (1-20, default 5) }`
- **Response**: `{ results: { path, score, summary }[] }`
  - Returns the most relevant files for a conceptual query (e.g., "Where is authentication?").
  - `score` indicates relevance (highest similarity of any chunk in the file).
  - `summary` provides context (e.g., matching chunks count and top snippet preview).

#### `outline`

- **Input**: `{ path: string, depth?: number }` where `path` is relative to the project root.
- **Response**: `{ outline: mcpOutlineNode[] }`
  - `mcpOutlineNode` includes `id`, `name`, `kind`, `start`, `end`, and nested `children`.

#### `nodeSource`

- **Input**: `{ id: string, collapseBody?: boolean }` where `id` is an identifier from `search`/`outline`.
- **Response**: `{ path, source, start, end, language?, symbol? }`
  - Focuses on the snippet content and precise boundaries.
  - If `collapseBody` is true, long snippets are truncated with a placeholder.

#### `getRecentChanges`

- **Input**: `{ limit?: number (default 10) }`
- **Response**: `{ indexed: { p, t }[], workingCopy: { p, s }[], vcs?: string }`
  - `indexed`: Files recently updated in the CodeTextor index (`p`: path, `t`: unix timestamp).
  - `workingCopy`: Real-time modifications from Git or SVN (`p`: path, `s`: VCS status code).
  - `vcs`: The active Version Control System detected (e.g., "git", "svn").

### Status & Tool Events

- `mcp:status`: emitted periodically with `{ isRunning, uptime, activeConnections, totalRequests, averageResponseTime, lastError? }`.
- `mcp:tools`: emitted when tool enablement changes.

---

## Notes for contributors

- Public APIs live here; internal Wails bindings stay documented in `DEV_GUIDE.md`.
- Update this file whenever MCP tool parameters or transport details change.
