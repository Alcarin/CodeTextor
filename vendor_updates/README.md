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
.\fix_vendor.ps1
```

### Linux / macOS (Bash)
```bash
cd vendor_updates
chmod +x fix_vendor.sh
./fix_vendor.sh
```

## What the scripts do

1.  **Fix `go-tree-sitter`**: Restores the `include` and `src` directories from the local Go module cache into the vendor folder.
2.  **Fix SQL Grammar**: Automatically downloads the missing `parser.c`, `scanner.c`, and headers for the SQL grammar from the official sources.
3.  **Repair other grammars**: Scans all `tree-sitter-*` folders in `vendor/` and ensures they have the necessary `src` and `queries` folders by copying them from your local Go cache. It also handles special subfolder structures for PHP and TypeScript.
4.  **Patch Tokenizer**: Automatically applies safety checks to `vendor/github.com/sugarme/tokenizer/normalizer/normalized.go` to prevent runtime panics.

## Permanent Fix

Since the `vendor/` folder is **not ignored** by Git in this project, once you run these scripts and commit the changes, the fix is permanent for everyone. You only need to run them again if you update your dependencies or run `go mod vendor` again.
