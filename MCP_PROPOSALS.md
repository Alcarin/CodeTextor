# MCP Server Evolution: Proposed Tools

This document outlines potential new tools for the CodeTextor MCP server, including an analysis of their applicability, development costs, and benefits for LLM agents.

## Comparative Analysis

| Tool | Applicability (Feasibility) | Benefits (User Value) | Development Cost | Priority |
| :--- | :--- | :--- | :--- | :--- |
| **`findReferences`** | **High**: Requires extra indexing of usages during parsing. | **Maximum**: Essential for safe refactoring. | Medium | **DONE** |
| **`semanticSearchFiles`** | **Very High**: Simple weighted aggregation of chunk similarity. | **High**: Provides a thematic overview of the repo. | Low | **DONE** |
| **`getProjectSummary`** | **High**: Simple reading of README and DB statistics. | **High**: Instantly orients the AI at session start. | Low | **DONE** |
| **`getRecentChanges`** | **High**: Simple integration with Git commands (shell). | **High**: Useful for debugging recent regressions. | Low | **DONE** |
| **`getCallGraph`** | **Medium**: Requires deeper static analysis to map calls. | **High**: Clarifies complex architectures. | High | **DONE** |
| **`grepSearch`** | **Very High**: Textual scanning of the filesystem. | **High**: Precise search for exact strings or regex (OS-independent). | Low | **DONE** |
| **`findImplementations`** | **High**: Requires type/interface resolution during parsing. | **High**: Crucial for OOP polymorphism. | High | **DONE** |
| **`getPackageGraph`** | **Medium**: File-level and folder-level dependency math. | **Very High**: Architectural understanding. | Medium | **DONE** |
| **`findTodos`** | **Very High**: Easy AST query for comment nodes. | **High**: Easy discovery of tech-debt and tasks. | Low | **DONE** |
| **`listParsingErrors`** | **High**: Uses the optimized error extraction from the index. | **Medium**: Helps AI identify corrupted or partial context. | Low | **PLANNED** |

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

#### `findImplementations` DONE

- **Description**: Given an interface or abstract class, find all classes/structs that implement it.
- **Feasibility**: High complexity. Unlike `findReferences` which is a direct symbol match, this requires resolving type signatures or navigating "implements" declarations across files.
- **Cost/Benefit**: Essential for AI agents working in OOP/Interface-heavy codebases (Go, Java, TS) where implementations are not co-located with the interface definition.

### 2. High-Level Semantic Search

#### `semanticSearchFiles` DONE

- **Description**: Instead of returning individual chunks, suggests the most relevant *files* for a concept (e.g., "Where is authentication logic handled?").
- **Feasibility**: Can be implemented by aggregating the similarity of chunks belonging to the same file. This is a purely mathematical operation that requires no new data.
- **Cost/Benefit**: Very low development cost with high benefit for speeding up the initial exploration of unknown projects.

#### `getProjectSummary` DONE in `getProjectDetails`

- **Description**: A structural map of the project (main packages, entry points, detected technologies).
- **Feasibility**: Simple aggregation of data already present in the index.
- **Cost/Benefit**: Extremely cheap to implement; helps the AI avoid "wasting time" loading irrelevant files.

#### `getPackageGraph` DONE

- **Description**: Returns a macro-level dependency graph showing how different folders/packages (including external libraries) interact.
- **Feasibility**: Implemented by aggregating `symbol_usages` from the database into a package-level adjacency map.
- **Cost/Benefit**: Gives the LLM an instant "Bounded Context" map, preventing architectural boundary violations and enabling depth-aware exploration.

#### `findTodos` DONE

- **Description**: Instantly surfaces all `TODO:`, `FIXME:`, and `HACK:` comments across the project, along with the function scope they belong to.
- **Feasibility**: Extremely high. Tree-sitter already parses comment nodes; it just requires filtering and associating them with their parent AST node.
- **Cost/Benefit**: Very low development cost for a massive UX win. Agents can be prompted to "fix open issues" and they can autonomously discover their tasks.

### 3. Integrated Development (Git)

#### `getRecentChanges` DONE

- **Description**: Show recently modified files via Git/SVN integration and recently indexed files from the database.
- **Feasibility**: Integrated VCS commands with a robust database fallback/complement.
- **Cost/Benefit**: Low cost, high context value for AI agents.

### 4. Utilities & Portability

#### `grepSearch` DONE

- **Description**: Literal or regex search across the project files.
- **Feasibility**: Can be implemented in Go by scanning files and looking for matches, ensuring OS-independence.
- **Cost/Benefit**: Highly valuable for cases where semantic search is too fuzzy (e.g., error codes, unique constants, specific patterns). Low development cost.

### 5. Project Health & Diagnostics

#### `listParsingErrors` PLANNED

- **Description**: Returns a list of all files that failed to parse correctly during indexing, including line/column of the errors.
- **Feasibility**: Very High. The optimized `extractParseErrors` already collects this data. We just need to expose it through a new tool.
- **Cost/Benefit**: High value for "closing the loop" on AI context. If an agent can't find a symbol, it can check if the file was simply unparseable due to syntax errors.
