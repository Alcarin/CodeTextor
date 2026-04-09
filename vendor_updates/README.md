# Tree-sitter Vendor Fixes

This directory contains scripts to fix common issues when using `tree-sitter` grammars with Go's `vendor` folder on Windows, Linux, and macOS.

## The Problem

Tree-sitter grammars often rely on CGO and generated C files. When you run `go mod vendor`, some crucial files might be missing because:
1.  **Windows Incompatibility**: Some Go wrappers for Tree-sitter don't correctly include header files in the vendor folder when generated on Windows.
2.  **Missing Generated Files**: Certain grammars (like `tree-sitter-sql`) do not include the generated `parser.c` in their master branch, making the vendor folder incomplete and unbuildable.

## How to use

If you have issues with `wails build` or `go build -mod=vendor` related to missing `.c` or `.h` files in the `vendor/` directory, run the appropriate script for your OS from this directory:

### Windows (PowerShell)
```powershell
cd vendor_updates
.\sync_vendors.ps1
```

### Linux / macOS (Bash)
```bash
cd vendor_updates
chmod +x sync_vendors.sh
./sync_vendors.sh
```

## What the scripts do

1.  **Fix Core**: Restores the `include` and `src` directories for `go-tree-sitter` from the local Go module cache.
2.  **Standard Vendor Fixes**: Scans standard `tree-sitter-*` folders in `vendor/` and ensures they have the necessary `src` files to allow CGO compilation on any OS.
3.  **Patch Tokenizer**: Automatically applies safety checks to the `tokenizer` package to prevent runtime panics.
4.  **Source-First Management (Internal)**: This is the most important part for CodeTextor. It manages the 10 internal grammars (PHP, Swift, Kotlin, etc.) in `vendor_grammar/` by:
    - Cloning the specific grammar repositories.
    - Generating the parser using the **Tree-sitter CLI**.
    - Extracting only the essential files (`parser.c`, `scanner.c`, headers) into a **Flat & Lean** structure.
    - Automatically generating the Go bindings.
    - Cleaning up all redundant source files and temporary clones.

## Permanent Fix

Since the `vendor/` folder is **not ignored** by Git in this project, once you run these scripts and commit the changes, the fix is permanent for everyone. You only need to run them again if you update your dependencies or run `go mod vendor` again.
