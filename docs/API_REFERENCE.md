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

| Tool        | Purpose                                                           |
| ----------- | ----------------------------------------------------------------- |
| `search`    | Semantic chunk retrieval for a project (top-k similarity)        |
| `outline`   | Hierarchical outline for a file (Tree-sitter symbols)            |
| `nodeSource`| Canonical snippet for a chunk/outline node id with metadata      |

#### `search`
- **Input**: `{ query: string, k?: number (1-50, default 8) }`
- **Response**: `{ results: Chunk[], totalResults: number, queryTimeMs: number }`
  - `Chunk` includes file path, line ranges, language, symbol metadata; `embedding` is an empty array (never null).

#### `outline`
- **Input**: `{ path: string, depth?: number }` where `path` is relative to the project root.
- **Response**: `{ outline: OutlineNode[] }` (may be empty if the file has no symbols).

#### `nodeSource`
- **Input**: `{ id: string, collapseBody?: boolean }` where `id` is a chunk or outline node id returned by `search`/`outline`.
- **Response**: `{ chunkId, filePath, source, startLine, endLine, language?, symbolName?, symbolKind? }`
  - If `collapseBody` is true, long snippets are truncated with a placeholder.

### Status & Tool Events
- `mcp:status`: emitted periodically with `{ isRunning, uptime, activeConnections, totalRequests, averageResponseTime, lastError? }`.
- `mcp:tools`: emitted when tool enablement changes.

---

## Notes for contributors

- Public APIs live here; internal Wails bindings stay documented in `DEV_GUIDE.md`.
- Update this file whenever MCP tool parameters or transport details change.
