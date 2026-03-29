# MCP Server Evolution: Proposed Tools

This document outlines potential new tools for the CodeTextor MCP server, including an analysis of their applicability, development costs, and benefits for LLM agents.

## Comparative Analysis

| Tool | Applicability (Feasibility) | Benefits (User Value) | Development Cost | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **`findReferences`** | **High**: Requires extra indexing of usages during parsing. | **Maximum**: Essential for safe refactoring. | Medium | **DONE** |
| **`semanticSearchFiles`** | **Very High**: Simple weighted aggregation of chunk similarity. | **High**: Provides a thematic overview of the repo. | Low | **DONE** |
| **`getProjectSummary`** | **High**: Simple reading of README and DB statistics. | **High**: Instantly orients the AI at session start. | Low | **DONE** |
| **`runTests`** | **Medium**: Requires mapping project test commands. | **Very High**: Allows the AI to validate its changes. | Medium | **P1** |
| **`getRecentChanges`** | **High**: Simple integration with Git commands (shell). | **High**: Useful for debugging recent regressions. | Low | **DONE** |
| **`getCallGraph`** | **Medium**: Requires deeper static analysis to map calls. | **High**: Clarifies complex architectures. | High | **DONE** |
| **`grepSearch`** | **Very High**: Textual scanning of the filesystem. | **High**: Precise search for exact strings or regex. | Low | **DONE** |
| **`summarizeFile`** | **High**: Requires a backend LLM call or heuristics. | **Med/High**: Significant token savings for users. | Medium | **P3** |

---

## Tool Details

### 1. Navigation & Static Analysis

#### `findReferences` DONE

- **Description**: Find all files and lines where a symbol (function, class, variable) is used.
- **Feasibility**: The Tree-sitter parser can be extended to detect not just definitions but also usages, saving them into a `usages` table in the project's SQLite-vec database.
- **Cost/Benefit**: Implementing reference indexing requires a database schema update, but it's the best way to give AI agents "superpowers," eliminating the risk of breaking unseen parts of the code.

#### `getCallGraph` DONE

- **Description**: Show who calls a function or what other functions are called by it.
- **Feasibility**: More complex than symbol search, as it requires resolving types and imports to ensure graph accuracy.
- **Cost/Benefit**: High value for structural understanding, but development effort is high to correctly support multiple languages.

### 2. High-Level Semantic Search

#### `semanticSearchFiles` DONE

- **Description**: Instead of returning individual chunks, suggests the most relevant *files* for a concept (e.g., "Where is authentication logic handled?").
- **Feasibility**: Can be implemented by aggregating the similarity of chunks belonging to the same file. This is a purely mathematical operation that requires no new data.
- **Cost/Benefit**: Very low development cost with high benefit for speeding up the initial exploration of unknown projects.

#### `getProjectSummary` DONE in `getProjectDetails`

- **Description**: A structural map of the project (main packages, entry points, detected technologies).
- **Feasibility**: Simple aggregation of data already present in the index.
- **Cost/Benefit**: Extremely cheap to implement; helps the AI avoid "wasting time" loading irrelevant files.

### 3. Integrated Development (Git & Test)

#### `getRecentChanges` DONE

- **Description**: Show recently modified files via Git/SVN integration and recently indexed files from the database.
- **Feasibility**: Integrated VCS commands with a robust database fallback/complement.
- **Cost/Benefit**: Low cost, high context value for AI agents.

#### `runTests`

- **Description**: Run configured tests and return the results.
- **Feasibility**: Requires project configuration to know how to launch tests (e.g., `go test`).
- **Cost/Benefit**: Immense value for an AI agent that can then operate in "Test-Driven Development" mode, instantly verifying its proposals.

### 4. Token Efficiency

#### `summarizeFile`

- **Description**: Returns a summary of a file's responsibilities.
- **Feasibility**: Can be implemented by sending the file's main comments and symbol list to a "lightweight" LLM on the backend.
- **Cost/Benefit**: Great for user token savings, but introduces an operational cost (API call) if not done via simple heuristics.

### 5. Utilities & Portability

#### `grepSearch`

- **Description**: Literal or regex search across the project files.
- **Feasibility**: Can be implemented in Go by scanning files and looking for matches, ensuring OS-independence.
- **Cost/Benefit**: Highly valuable for cases where semantic search is too fuzzy (e.g., error codes, unique constants, specific patterns). Low development cost.
