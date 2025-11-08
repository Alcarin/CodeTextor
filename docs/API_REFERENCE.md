# CodeTextor API Reference

This document only tracks **external** APIs exposed by CodeTextor. Internal
Wails bindings between the Go backend and the Vue frontend are implementation
details and are covered in `DEV_GUIDE.md` / `ARCHITECTURE.md` instead.

---

## MCP (Model Context Protocol)

The MCP server is the public interface that IDEs and AI assistants will use to
query projects. It is currently under development.

> **Status:** no external endpoints are available yet. When the MCP server is
> ready, this section will list every tool (parameters, limits, response
> structure) together with integration examples.

---

## Notes for contributors

- If you are working on the desktop app, refer to `DEV_GUIDE.md` for guidance
  on Wails bindings, backend services, and frontend usage.
- Only add entries to this file when an API is meant to be consumed outside the
  CodeTextor application (e.g., MCP tools, REST endpoints, CLI interfaces).

Until the MCP server ships, there are no public APIs to document.
