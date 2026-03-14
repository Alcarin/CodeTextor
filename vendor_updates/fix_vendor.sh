#!/bin/bash

# CodeTextor Vendor Fixer (Linux/macOS Version)
# This script fixes tree-sitter vendor directories by copying header/source files
# from the Go module cache OR downloading them (for SQL grammar) if they are missing.

set -e

VENDOR_DIR="../vendor"
GO_PATH=$(go env GOPATH)
MOD_CACHE="$GO_PATH/pkg/mod"

echo -e "\033[0;36m=== CodeTextor Vendor Fixer (Linux/macOS) ===\033[0m"

# 1. FIX CORE: go-tree-sitter
echo -e "\033[0;33m[1/3] Fixing go-tree-sitter...\033[0m"
# Find the go-tree-sitter directory in cache
GT_SOURCE=$(find "$MOD_CACHE" -name "go-tree-sitter@*" -type d -print -quit)

if [ -n "$GT_SOURCE" ]; then
    GT_VENDOR="$VENDOR_DIR/github.com/tree-sitter/go-tree-sitter"
    if [ -d "$GT_VENDOR" ]; then
        if [ -d "$GT_SOURCE/include" ]; then
            echo "  Copying include..."
            cp -r "$GT_SOURCE/include" "$GT_VENDOR/"
        fi
        if [ -d "$GT_SOURCE/src" ]; then
            echo "  Copying src..."
            cp -r "$GT_SOURCE/src" "$GT_VENDOR/"
        fi
    fi
fi

# 2. SPECIFIC FIX: tree-sitter-sql (DerekStride)
echo -e "\033[0;33m[2/3] Fixing tree-sitter-sql (downloading generated files)...\033[0m"
SQL_VENDOR_SRC="$VENDOR_DIR/github.com/DerekStride/tree-sitter-sql/src"
mkdir -p "$SQL_VENDOR_SRC"

BASE_URL="https://raw.githubusercontent.com/DerekStride/tree-sitter-sql/gh-pages/src"
FILES_TO_DOWNLOAD=(
    "parser.c"
    "scanner.c"
    "tree_sitter/parser.h"
    "tree_sitter/alloc.h"
    "tree_sitter/array.h"
)

for file in "${FILES_TO_DOWNLOAD[@]}"; do
    DEST_PATH="$SQL_VENDOR_SRC/$file"
    mkdir -p "$(dirname "$DEST_PATH")"
    
    echo "  Downloading $file..."
    curl -sL -o "$DEST_PATH" "$BASE_URL/$file"
done

# 3. GLOBAL GRAMMAR FIX: Other grammars
echo -e "\033[0;33m[3/3] Checking other grammars...\033[0m"
# Find all tree-sitter grammars in vendor
find "$VENDOR_DIR" -maxdepth 4 -name "tree-sitter-*" -type d | while read g; do
    if [ ! -d "$g/bindings/go" ]; then continue; fi
    
    # Skip SQL (already done)
    if [[ "$g" == *"DerekStride/tree-sitter-sql"* ]]; then continue; fi
    
    GRAMMAR_NAME=$(basename "$g")
    echo "  Checking $GRAMMAR_NAME..."
    
    # We try to find the source in cache. This is a bit complex due to Go's ! encoding
    # for uppercase, but usually grammars are lowercase anyway.
    # We look for the folder by name in the mod cache.
    G_SOURCE=$(find "$MOD_CACHE" -name "$GRAMMAR_NAME@*" -type d -print -quit)
    
    if [ -n "$G_SOURCE" ]; then
        # Restore src and queries if missing
        if [ ! -d "$g/src" ] && [ -d "$G_SOURCE/src" ]; then
            echo "    Restoring src..."
            cp -r "$G_SOURCE/src" "$g/"
        fi
        if [ ! -d "$g/queries" ] && [ -d "$G_SOURCE/queries" ]; then
            echo "    Restoring queries..."
            cp -r "$G_SOURCE/queries" "$g/"
        fi
    fi
done

echo -e "\033[0;32m=== Operation Completed ===\033[0m"
echo "You can now run: wails build"
echo "Note: Since the vendor folder is tracked by Git, these fixes are now permanent in your repository."
