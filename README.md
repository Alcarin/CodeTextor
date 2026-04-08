# 🧠 CodeTextor

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

**Local codebase context provider for LLMs, IDEs, and AI agents.**  
CodeTextor analyzes your source code using [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) and builds a lightweight **vector index** (via [SQLite-vec](https://github.com/asg017/sqlite-vec)) for fast semantic retrieval and navigation — completely offline.

---

## ✨ Overview

CodeTextor is a **local-first semantic indexer** for your projects.
It extracts structural code chunks (functions, classes, comments, modules), generates embeddings, and serves them through a modular **MCP (Model Context Protocol)** API.

- **Local-first**: Zero cloud dependencies, complete data sovereignty.
- **Isolated**: Each project has its own dedicated SQLite-vec database.
- **Standards-based**: Uses the Model Context Protocol for universal IDE/AI integration.
- **Efficient**: Human-readable Symbol IDs and two-stage asynchronous indexing.

---

## 🔍 Key Features

- 🚀 **Dynamic Tree-sitter Parsing**: Advanced AST-aware chunking engine driven by TOML configurations. Add or customize language support without backend recompilation.
- 🧬 **Nested Code Parsing**: Deeply analyzes multi-language files (e.g., JS/CSS in Vue, SQL in Go) with a recursive engine that automatically identifies and indexes embedded code with absolute offset precision.
- 🛠️ **Technical Debt Extraction**: Surfaces `TODO`, `FIXME`, and `HACK` comments across all formats for a unified project status overview.
- ⚡ **Asynchronous Indexing Pipeline**: Decoupled CPU workers (parsing) and GPU/DB workers (embedding/persistence) with prioritized semaphore control.
- 💾 **Integrated Vector Store**: Uses `sqlite-vec` for embedded vector search without external database servers.
- 🗂️ **Multi-Project Isolation**: Complete separation between codebases; switch between projects seamlessly with no data cross-contamination.
- 📊 **Real-time Statistics**: Per-project and cumulative metrics (files, chunks, symbols) with live progress tracking.
- 🌲 **Symbol Outline & Chunks**: Explore code structure through a hierarchical symbol tree or inspect semantic chunks with full metadata.
- 🧠 **Multi-Model Support**: Support for dual FastEmbed/ONNX backends with automatic runtime detection and downloadable model catalog.
- 🔒 **Privacy-First**: 100% local and private; your code and embeddings never leave your machine.

---

## 🧱 Architecture

```text
frontend/        → Wails UI (Vue/React + TypeScript)
backend/
internal/
chunker/     → Tree-sitter parsing & chunking
indexer/     → Embeddings & SQLite-vec store
mcp/         → MCP tools (context retrieval API)
store/       → DB schema & helpers
search/      → Lexical + semantic query logic
cmd/           → CLI entry points
docs/            → Developer documentation & API references
```

---

## ⚙️ Installation

### Prerequisites

- [Go ≥ 1.23](https://go.dev/)
- [Node.js ≥ 20](https://nodejs.org/)
- [Wails ≥ 2](https://wails.io/)
- A compiler toolchain for your OS (gcc / clang)
- **ONNX Runtime 1.24.4**: The application handles the installation of required libraries for your platform automatically.
  - **Windows**: Supports DirectML (GPU DirectX 12).
  - **macOS**: Supports CoreML (Apple Silicon via `CoreML_V2`).
  - **Linux**: Standard CPU/OpenVINO or CUDA (manual config).

### Build

```bash
git clone https://github.com/<your-org>/codetextor.git
cd codetextor
wails build
````

### Run

```bash
./build/bin/codetextor
```

or in dev mode:

```bash
wails dev
```

CodeTextor will launch both the local web UI and the MCP server.

### ONNX Runtime & GPU Setup

CodeTextor simplifies the hardware acceleration setup by automating the management of ONNX Runtime libraries and **optimizing GPU resource allocation**.

#### 🚀 GPU & VRAM Optimization

CodeTextor includes a **VRAM-aware indexing engine** that automatically scales batch sizes based on available GPU memory (using power-of-2 alignment) and prioritizes real-time user actions (like semantic search) over background indexing to ensure maximum responsiveness.

#### 1. Recommended: Automatic Setup (UI)

The most efficient way to configure the environment on **Windows**, **macOS**, and **Linux** is:

1. Launch CodeTextor.
2. Go to **Settings → Projects**.
3. Click the **"Download and Install Runtime"** button.
4. **Restart CodeTextor** to apply the changes.

The application securely fetches the appropriate NuGet or GitHub assets (including `DirectML.dll` on Windows), verifies their integrity via SHA256, and installs them in the local configuration folder.

#### 2. Advanced: NVIDIA CUDA (Manual)

For high-performance scenarios on NVIDIA hardware:

- **Download**: Use the `onnxruntime-win-x64-gpu-1.24.4.zip` from [GitHub Releases](https://github.com/microsoft/onnxruntime/releases/tag/v1.24.4).
- **Requirements**: Requires [CUDA Toolkit 12.x](https://developer.nvidia.com/cuda-downloads) and [cuDNN 9.x](https://developer.nvidia.com/cudnn) installed on your system.
- **Environment**: Ensure CUDA and cuDNN `bin` directories are in your system `PATH`.
- **Configure**: Manually select the `onnxruntime.dll` path in the Settings modal.

#### 3. Setup Summary

1. Configure **ONNX runtime path** (automatic via "Download" or manual via file picker).
2. **Restart CodeTextor** to apply changes.
3. Download embedding models from **Indexing → Embedding model**.

---

## 🧠 Using the MCP API

CodeTextor ships a streamable **HTTP** MCP server. Point your client to:

```bash
http://127.0.0.1:3030/mcp/<projectId>
```

`<projectId>` must be a valid project ID; calls to the root endpoint are rejected. The host/port and auto-start toggle live in the **MCP** tab inside the app.

| Tool | Description |
| ---- | ----------- |
| `getProjectDetails` | High-level overview (main languages, packages, entry points) and project statistics |
| `listFiles` | Explore the project structure with detailed file statistics and language detection |
| `search` | Semantic natural language search (results grouped by file) |
| `semanticSearchFiles` | Suggests the most relevant files (returns node IDs and similarity) |
| `outline` | Hierarchical symbol tree (classes, functions) for a file |
| `nodeSource` | Robust source snippets for an ARRAY of IDs (supports fuzzy matching) |
| `getRecentChanges` | Recently modified (VCS) and indexed (DB) files |
| `grepSearch` | Literal or regex search (OS-independent). Returns compact tabular results [File, Line, Content]. |
| `findReferences` | Find all locations where a symbol is referenced or used |
| `getCallGraph` | Get the hierarchical call relationships for a specific function |
| `findTodos` | Instantly surface all TODO, FIXME, and HACK comments across the project |
| `getPackageGraph` | Get a high-level overview of package dependencies and coupling |
| `findImplementations` | Discover all classes or interfaces that implement a specific interface |

Example Codex CLI config (`~/.codex/config.toml`):

```toml
[mcp_servers.codetextor]
url = "http://127.0.0.1:3030/mcp/<projectId>"
transport = "http"
enabled = true

[features]
rmcp_client = true
```

Example Gemini CLI (Command line):

```bash
gemini mcp add --transport http codetextor-<projectId> http://127.0.0.1:3030/mcp/<projectId>
```

Example Antigravity / Gemini CLI (`mcp_config.json`):

```json
{
  "mcpServers": {
    "codetextor-<projectId>": {
      "serverURL": "http://127.0.0.1:3030/mcp/<projectId>"
    }
  }
}
```

---

## 📚 Documentation

Developer and contributor documentation lives under [`/docs`](./docs):

- [`DEV_GUIDE.md`](./docs/DEV_GUIDE.md) — detailed architecture, coding standards, and LLM collaboration rules
- `API_REFERENCE.md` — system overview diagrams and data flows
- `ARCHITECTURE.md` — system overview diagrams and data flows

---

## 🧩 Design Principles

- **Local-first:** runs entirely on your machine
- **Modular:** each subsystem in its own package
- **Transparent:** all data and embeddings are inspectable
- **Extensible:** Customize extraction logic instantly via TOML. Adding a completely new language requires a simple one-time grammar registration in the Go backend.
- **Readable:** written for humans *and* LLMs — every function documented

---

## 🧑‍💻 Contributing

Pull requests and ideas are welcome!
Please read the [Developer Guide](./docs/DEV_GUIDE.md) before contributing.

- Write all code and comments in **English**.
- Use **modular design** and split large files into logical parts.
- Document every function (including arrow or anonymous ones).
- Keep code clean, readable, and deterministic.

---

## 📜 License

CodeTextor is released under the **MIT License**.
See [LICENSE](./LICENSE) for details.

---

## 💬 Acknowledgments

Built with ❤️ using:

- [Tree-sitter](https://tree-sitter.github.io/tree-sitter/)
- [SQLite-vec](https://github.com/asg017/sqlite-vec)
- [Wails](https://wails.io/)
- [MCP Protocol](https://modelcontextprotocol.io/)

---

> *“Code should be easy to read — even for machines that read it to help us.”*
> — *CodeTextor Manifesto*

---
