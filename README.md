# 🧠 CodeTextor

**Note:** This project is currently in early development. First release (v0.1.0) will be announced when core functionality is complete.

**Local codebase context provider for LLMs, IDEs, and AI agents.**  
CodeTextor analyzes your source code using [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) and builds a lightweight **vector index** (via [SQLite-vec](https://github.com/asg017/sqlite-vec)) for fast semantic retrieval and navigation — completely offline.

---

## ✨ Overview

CodeTextor is a **local-first semantic indexer** for your projects.  
It extracts structural code chunks (functions, classes, comments, modules), generates embeddings, and serves them through a simple **MCP (Model Context Protocol)** API.

This enables:
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
  - `retrieve`, `outline`, `nodeAt`, `nodeSource`, `searchSymbols`, etc.
- 🖥️ **Frontend UI** (built with Wails + Vue) for local indexing, browsing, and search
- 🧠 **Per-project embedding selection** with dual FastEmbed/ONNX backends (both require ONNX Runtime), automatic runtime detection, downloadable catalog entries, and a "custom model" modal
- 🔒 100% **local & private**, no data leaves your machine

---

## 🧱 Architecture

```

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

````

---

## ⚙️ Installation

### Prerequisites
- [Go ≥ 1.23](https://go.dev/)  
- [Node.js ≥ 20](https://nodejs.org/)  
- [Wails ≥ 3](https://wails.io/)  
- A compiler toolchain for your OS (gcc / clang)
- [ONNX Runtime 1.22.0](https://github.com/microsoft/onnxruntime/releases/tag/v1.22.0) (configure its shared library path via the **Projects → ONNX runtime path** field inside CodeTextor)
  - CPU-only builds work as soon as the shared library file is selected in the GUI
  - GPU builds **must** match the same ONNX Runtime version and require [CUDA 12.x](https://developer.nvidia.com/cuda-downloads) plus [cuDNN 9.x](https://developer.nvidia.com/cudnn)

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

### ONNX Runtime & CUDA setup

1. Download the ONNX Runtime 1.22.0 archive for your platform, then open **Projects → ONNX runtime path** inside CodeTextor and paste the absolute path to the extracted `libonnxruntime.so.1.22.0`/`onnxruntime.dll`.  
2. For GPU acceleration install the matching CUDA toolkit (12.6 or 12.7 recommended) plus cuDNN 9.x:
   - Linux: follow the commands provided on [developer.nvidia.com/cuda-downloads](https://developer.nvidia.com/cuda-downloads) for your distro (e.g., install `cuda-toolkit-12-7` and add `/usr/local/cuda-12.7/bin` to `PATH`).  
   - Install cuDNN 9.x for CUDA 12.x and copy its `lib` directory next to the CUDA toolkit libraries (or use the official `.deb/.rpm` packages).  
   - Ensure `LD_LIBRARY_PATH` (or `PATH` on Windows) includes both the ONNX Runtime folder and the CUDA/cuDNN provider libraries (`libonnxruntime_providers_cuda.so`, etc.).
3. Restart CodeTextor so the backend reinitializes ONNX Runtime with the updated libraries. If the GPU provider fails to load, the UI will display a warning and fall back to CPU embeddings.
4. Download the desired embedding model from **Indexing → Embedding model**. Both FastEmbed presets and ONNX models use the same download flow; the status chip switches to “Ready” only after the files are present locally.
5. During download the app shows an in-app progress modal; FastEmbed models automatically fall back to the official Hugging Face mirrors if the upstream CDN is unavailable. If a model lacks an official ONNX export (e.g., `nomic-ai/nomic-embed-code`), supply your own converted `model.onnx` + tokenizer via the “Add custom model” flow.

---

## 🧠 Using the MCP API

CodeTextor exposes a lightweight JSON-based MCP interface.
Example tools include:

| Tool                           | Description                        |
| ------------------------------ | ---------------------------------- |
| `search(projectId, query, k)`  | Semantic retrieval of code chunks  |
| `outline(path, depth)`         | Structural outline of a file       |
| `nodeAt(path, line)`           | Returns the AST node at a position |
| `nodeSource(id, collapseBody)` | Returns code snippet of a symbol   |
| `searchSymbols(query, kinds)`  | Lexical symbol search              |

Integrate it with your LLM or IDE plugin to provide local context awareness.

---

## 📚 Documentation

Developer and contributor documentation lives under [`/docs`](./docs):

* [`DEV_GUIDE.md`](./docs/DEV_GUIDE.md) — detailed architecture, coding standards, and LLM collaboration rules
* `API_REFERENCE.md` — MCP and internal API reference (coming soon)
* `ARCHITECTURE.md` — system overview diagrams and data flows

---

## 🧩 Design Principles

* **Local-first:** runs entirely on your machine
* **Modular:** each subsystem in its own package
* **Transparent:** all data and embeddings are inspectable
* **Extensible:** easy to add languages or custom chunkers
* **Readable:** written for humans *and* LLMs — every function documented

---

## 🧑‍💻 Contributing

Pull requests and ideas are welcome!
Please read the [Developer Guide](./docs/DEV_GUIDE.md) before contributing.

* Write all code and comments in **English**.
* Use **modular design** and split large files into logical parts.
* Document every function (including arrow or anonymous ones).
* Keep code clean, readable, and deterministic.

---

## 📜 License

CodeTextor is released under the **MIT License**.
See [LICENSE](./LICENSE) for details.

---

## 💬 Acknowledgments

Built with ❤️ using:

* [Tree-sitter](https://tree-sitter.github.io/tree-sitter/)
* [SQLite-vec](https://github.com/asg017/sqlite-vec)
* [Wails](https://wails.io/)
* [MCP Protocol](https://modelcontextprotocol.io/)

---

> *“Code should be easy to read — even for machines that read it to help us.”*
> — *CodeTextor Manifesto*

---
