# CodeTextor API Reference

## Overview

CodeTextor exposes an external API through the **MCP (Model Context Protocol)** server for IDE and AI integration. This document describes the public-facing tools and endpoints available for external clients.

**Note:** Internal Wails bindings between Go backend and TypeScript frontend are not documented here as they are implementation details.

---

## MCP Server API

The MCP (Model Context Protocol) server provides tools for code retrieval, navigation, and analysis across multiple projects.

**Status:** Planned - Not yet implemented (see [TODO.md](TODO.md) tasks 7.x-9.x)

**Transport:** HTTP and STDIO

**Multi-Project Architecture:** All MCP tools require a `projectId` parameter to specify which project's index to query. This ensures complete isolation between projects and prevents data leakage.

---

## Core Principles

### Project Scoping

**All MCP tools require a `projectId` parameter.**

```json
{
  "tool": "retrieve",
  "params": {
    "projectId": "550e8400-e29b-41d4-a716-446655440000",
    "query": "authentication logic",
    "k": 10
  }
}
```

**Validation:**
- Requests without `projectId` are rejected
- Invalid or non-existent `projectId` returns error
- Results are strictly scoped to the specified project

### Path Boundaries

All tools enforce configured include path boundaries:
- Only return results for files within project's configured paths
- Reject requests for files outside project scope
- Validate path safety (no directory traversal)

### Resource Limits

- **Result Size:** Configurable byte caps per project
- **Top-K Results:** Maximum number of returned chunks
- **Pagination:** Support for large result sets

---

## Planned MCP Tools

### retrieve

Semantic search for code chunks using vector similarity.

**Purpose:** Find relevant code based on natural language queries.

**Signature:**
```json
{
  "tool": "retrieve",
  "params": {
    "projectId": "uuid",
    "query": "string",
    "k": 10,
    "filters": {
      "fileTypes": [".ts", ".go"],
      "paths": ["src/"],
      "excludeTests": false
    }
  }
}
```

**Parameters:**
- `projectId` (string, required): Project UUID
- `query` (string, required): Natural language query
- `k` (integer, optional): Number of results to return (default: 10)
- `filters` (object, optional): Filter criteria
  - `fileTypes` (array): File extensions to include
  - `paths` (array): Path prefixes to search within
  - `excludeTests` (boolean): Skip test files

**Returns:**
```json
{
  "results": [
    {
      "chunkId": "string",
      "filePath": "string",
      "lineStart": 42,
      "lineEnd": 87,
      "content": "string",
      "score": 0.95,
      "metadata": {
        "symbolName": "authenticate",
        "symbolType": "function",
        "language": "typescript"
      }
    }
  ]
}
```

**Errors:**
- `PROJECT_NOT_FOUND`: Invalid projectId
- `INVALID_QUERY`: Empty or malformed query
- `RESOURCE_LIMIT_EXCEEDED`: Results exceed byte cap

---

### outline

Get hierarchical structure of a file.

**Purpose:** Retrieve file outline with functions, classes, and modules.

**Signature:**
```json
{
  "tool": "outline",
  "params": {
    "projectId": "uuid",
    "path": "src/main.go",
    "depth": 2,
    "collapseBody": true
  }
}
```

**Parameters:**
- `projectId` (string, required): Project UUID
- `path` (string, required): File path relative to project root
- `depth` (integer, optional): Nesting depth (default: unlimited)
- `collapseBody` (boolean, optional): Replace function bodies with `{ ... }` (default: true)

**Returns:**
```json
{
  "filePath": "src/main.go",
  "language": "go",
  "outline": [
    {
      "type": "package",
      "name": "main",
      "line": 1,
      "children": [
        {
          "type": "function",
          "name": "main",
          "line": 10,
          "signature": "func main()",
          "docComment": "main initializes the application"
        }
      ]
    }
  ]
}
```

**Errors:**
- `PROJECT_NOT_FOUND`: Invalid projectId
- `FILE_NOT_FOUND`: Path not in project
- `PATH_OUTSIDE_BOUNDARY`: Path outside configured include paths

---

### nodeAt

Return AST node at specific line/column position.

**Purpose:** Get detailed information about code at cursor position.

**Signature:**
```json
{
  "tool": "nodeAt",
  "params": {
    "projectId": "uuid",
    "path": "src/auth.ts",
    "line": 42,
    "column": 15
  }
}
```

**Parameters:**
- `projectId` (string, required): Project UUID
- `path` (string, required): File path
- `line` (integer, required): Line number (1-indexed)
- `column` (integer, optional): Column number (default: 0)

**Returns:**
```json
{
  "node": {
    "type": "function_declaration",
    "name": "authenticate",
    "startLine": 40,
    "endLine": 55,
    "content": "function authenticate(user: User) { ... }",
    "parent": "class UserController"
  }
}
```

---

### searchSymbols

Lexical (text-based) symbol search.

**Purpose:** Find symbols by name pattern (functions, classes, variables).

**Signature:**
```json
{
  "tool": "searchSymbols",
  "params": {
    "projectId": "uuid",
    "query": "UserController",
    "kinds": ["function", "class"],
    "caseSensitive": false,
    "regex": false
  }
}
```

**Parameters:**
- `projectId` (string, required): Project UUID
- `query` (string, required): Symbol name or pattern
- `kinds` (array, optional): Symbol types to match (function, class, variable, etc.)
- `caseSensitive` (boolean, optional): Case-sensitive matching (default: false)
- `regex` (boolean, optional): Treat query as regex (default: false)

**Returns:**
```json
{
  "symbols": [
    {
      "name": "UserController",
      "kind": "class",
      "filePath": "src/controllers/user.ts",
      "line": 10,
      "docComment": "Handles user authentication and authorization"
    }
  ]
}
```

---

### nodeSource

Retrieve source code for a specific chunk/node by ID.

**Purpose:** Fetch full source code of a previously retrieved chunk.

**Signature:**
```json
{
  "tool": "nodeSource",
  "params": {
    "projectId": "uuid",
    "chunkId": "abc123",
    "collapseBody": false,
    "includeContext": 5
  }
}
```

**Parameters:**
- `projectId` (string, required): Project UUID
- `chunkId` (string, required): Chunk identifier from retrieve results
- `collapseBody` (boolean, optional): Collapse function bodies (default: false)
- `includeContext` (integer, optional): Lines of context before/after (default: 0)

**Returns:**
```json
{
  "source": "function authenticate(user: User) {\n  // ...\n}",
  "filePath": "src/auth.ts",
  "lineStart": 40,
  "lineEnd": 55
}
```

---

### findDefinition *(Optional)*

Find definition of a symbol.

**Status:** Optional feature, may not be in initial release.

**Signature:**
```json
{
  "tool": "findDefinition",
  "params": {
    "projectId": "uuid",
    "symbolName": "authenticate",
    "fromPath": "src/routes.ts",
    "fromLine": 10
  }
}
```

---

### findReferences *(Optional)*

Find all references to a symbol.

**Status:** Optional feature, may not be in initial release.

**Signature:**
```json
{
  "tool": "findReferences",
  "params": {
    "projectId": "uuid",
    "symbolName": "authenticate"
  }
}
```

---

## Error Handling

### Standard Error Format

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "Project with ID '...' does not exist",
    "details": {}
  }
}
```

### Error Codes

|            Code           |              Description              |
|---------------------------|---------------------------------------|
| `PROJECT_NOT_FOUND`       | Invalid or non-existent projectId     |
| `FILE_NOT_FOUND`          | Requested file not in project index   |
| `PATH_OUTSIDE_BOUNDARY`   | Path outside configured include paths |
| `INVALID_QUERY`           | Malformed or empty query              |
| `RESOURCE_LIMIT_EXCEEDED` | Result size exceeds project limits    |
| `CHUNK_NOT_FOUND`         | Invalid chunkId                       |
| `INTERNAL_ERROR`          | Unexpected server error               |

---

## Security

### Path Validation

All file paths are validated against:
1. **Include path allowlist:** Only files within configured project paths
2. **Directory traversal prevention:** No `..` or absolute paths
3. **Symbolic link handling:** Resolved to real paths before validation

### Resource Protection

- **Byte caps:** Configurable maximum response size per project
- **Rate limiting:** Per-project request throttling
- **Concurrent limits:** Maximum parallel requests per project

### Project Isolation

- **Database isolation:** Each project uses separate SQLite-vec database
- **No cross-contamination:** Results strictly scoped to requested projectId
- **Access control:** Projects only access their configured paths

---

## Best Practices

### Client Implementation

**DO:**
```json
// ✅ Always provide projectId
{
  "tool": "retrieve",
  "params": {
    "projectId": "550e8400-...",
    "query": "authentication"
  }
}

// ✅ Handle pagination for large results
{
  "tool": "retrieve",
  "params": {
    "projectId": "...",
    "query": "...",
    "k": 100,
    "offset": 0
  }
}

// ✅ Use filters to narrow results
{
  "tool": "retrieve",
  "params": {
    "projectId": "...",
    "query": "...",
    "filters": {
      "paths": ["src/auth/"],
      "fileTypes": [".ts"]
    }
  }
}
```

**DON'T:**
```json
// ❌ Omit projectId
{
  "tool": "retrieve",
  "params": {
    "query": "authentication"
  }
}

// ❌ Request all results at once
{
  "tool": "retrieve",
  "params": {
    "projectId": "...",
    "query": "...",
    "k": 999999
  }
}

// ❌ Use absolute paths
{
  "tool": "outline",
  "params": {
    "projectId": "...",
    "path": "/home/user/project/src/main.go"
  }
}
```

### Performance Tips

- **Use filters:** Narrow search scope with path and file type filters
- **Paginate:** Request results in batches rather than all at once
- **Cache:** Client-side caching of outline and symbol search results
- **Debounce:** Wait for user to finish typing before querying

---

**Last Updated:** 2025-11-07
