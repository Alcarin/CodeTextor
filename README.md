# 🧠 CodeTextor

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

**Local codebase context provider for LLMs, IDEs, and AI agents.**  
CodeTextor analyzes your source code using [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) and builds a lightweight **vector index** (via [SQLite-vec](https://github.com/asg017/sqlite-vec)) for fast semantic retrieval and navigation — completely offline.

---

## ✨ Overview

CodeTextor is a **local-first semantic indexer** for your projects.
It extracts structural code chunks (functions, classes, comments, modules), generates embeddings, and serves them through a simple **MCP (Model Context Protocol)** API.

Specifiche CodeTextor:

- **Local-first**:
- **Isolated**: Each project has its own database.
- **Transparent**: All data is inspectable.
- **Standards-based**: Uses the Model Context Protocol.
- IDE plugins or AI assistants to query the local codebase semantically.
- Fast "where is this defined?" or "show me related functions" queries.
- Offline RAG-style context retrieval for LLMs without cloud APIs.

---

## 🔍 Key Features

- 🚀 **Tree-sitter-based parsing** for accurate AST-aware chunking
- 🧩 **Adaptive chunking strategy**
  - Collapses large functions/classes (`{ ... }`)
  - Merges small ones with comments and metadata
- 💾 **Embedded vector store** (SQLite-vec, no external DB)
- 🗂️ **Multi-project management** with complete isolation
  - Each project has its own database
  - Switch between projects seamlessly
  - No data cross-contamination
- 📊 **Real-time statistics**
  - Per-project metrics (files, chunks, symbols)
  - Cumulative statistics across all projects
  - Live indexing progress tracking
- 🌲 **Code navigation**
  - Hierarchical outline view (functions, classes, symbols)
  - Semantic chunks browser with metadata
  - File tree with per-file loading
- 🧠 **MCP Server mode** for use with IDEs and LLM agents
  - Streamable HTTP server with `search`, `outline`, `nodeSource` tools
  - Per-project routing via `/mcp/<projectId>`
- 🖥️ **Frontend UI** (built with Wails + Vue) for local indexing, browsing, and search
- 🧠 **Per-project embedding selection** with dual FastEmbed/ONNX backends (both require ONNX Runtime), automatic runtime detection, downloadable catalog entries, and a "custom model" modal
- 🔒 100% **local & private**, no data leaves your machine

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
- [Wails ≥ 3](https://wails.io/)  
- A compiler toolchain for your OS (gcc / clang)
- [ONNX Runtime 1.24.3](https://github.com/microsoft/onnxruntime/releases/tag/v1.24.3) (configure the shared library path in **Settings → Projects → ONNX runtime path**)
  - **Windows**: Supports DirectML (GPU DirectX 12) e CUDA 12.x.
  - **macOS**: Supports CoreML (Apple Silicon via `CoreML_V2`).
  - **Linux**: Supports CUDA 12.x.

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

If you see the message **"Hardware Acceleration: CPU (Falling back to CPU)"**, it means the loaded ONNX Runtime library does not support hardware acceleration or required components are missing.

#### 1. Recommended: DirectML (Universal for Windows GPU)

DirectML is the most compatible way to enable GPU acceleration on Windows (works with NVIDIA, AMD, and Intel).

- **Download**: Do **NOT** use the standard GitHub ZIP. Instead, download the NuGet package [Microsoft.ML.OnnxRuntime.DirectML v1.24.3](https://www.nuget.org/packages/Microsoft.ML.OnnxRuntime.DirectML/1.24.3).
- **Extract**: Rename the downloaded `.nupkg` to `.zip`, extract it, and find `onnxruntime.dll` in `runtimes/win-x64/native/`.
- **Dependency**: Also download [Microsoft.AI.DirectML](https://www.nuget.org/packages/Microsoft.AI.DirectML/) and extract `DirectML.dll` from `bin/x64-win/`.
- **Configure**: Place both `onnxruntime.dll` and `DirectML.dll` in the same folder. In CodeTextor (**Settings → Projects → ONNX runtime path**), select the absolute path to your `onnxruntime.dll`.

#### 2. Advanced: NVIDIA CUDA

For high-performance on NVIDIA hardware:

- **Download**: Use the `onnxruntime-win-x64-gpu-1.24.3.zip` from [GitHub Releases](https://github.com/microsoft/onnxruntime/releases/tag/v1.24.3).
- **Requirements**: Requires [CUDA Toolkit 12.x](https://developer.nvidia.com/cuda-downloads) and [cuDNN 9.x](https://developer.nvidia.com/cudnn) to be installed on your system.
- **Environment**: Ensure the `bin` directories for both CUDA and cuDNN are in your system `PATH`.

#### 3. Apple Silicon (macOS)

CoreML is used automatically to leverage the Apple Neural Engine. Download the standard `.dylib` from [GitHub Releases](https://github.com/microsoft/onnxruntime/releases/tag/v1.24.3).

#### 4. Setup Steps

1. Configure the **ONNX runtime path** in Settings.
2. **Restart CodeTextor** to apply changes.
3. Download models from **Indexing → Embedding model**.

---

## 🧠 Using the MCP API

CodeTextor ships a streamable **HTTP** MCP server. Point your client to:

```bash
http://127.0.0.1:3030/mcp/<projectId>
```

`<projectId>` must be a valid project ID; calls to the root endpoint are rejected. The host/port and auto-start toggle live in the **MCP** tab inside the app.

Available tools:

| Tool | Description |
| ---- | ----------- |
| `search` | Semantic chunk retrieval (`query`, optional `k` 1-50) |
| `outline` | Hierarchical outline for a file (`path`, optional `depth`) |
| `nodeSource` | Canonical snippet for a chunk/outline node id (`id`, optional `collapseBody`) |

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
gemini mcp add --transport http codetextor-<projectName> http://127.0.0.1:3030/mcp/<projectId>
```

Example Antigravity / Gemini CLI (`mcp_config.json`):

```json
{
  "mcpServers": {
    "codetextor-<projectName>": {
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
- **Extensible:** easy to add languages or custom chunkers
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
