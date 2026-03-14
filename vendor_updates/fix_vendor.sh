#!/bin/bash

# CodeTextor Vendor Fixer (Linux/macOS Version)
# This script fixes tree-sitter vendor directories by copying header/source files
# from the Go module cache OR downloading them (for SQL grammar) if they are missing.

set -e

# Get script directory to make paths absolute
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
VENDOR_DIR="$SCRIPT_DIR/../vendor"
GO_PATH=$(go env GOPATH)
MOD_CACHE="$GO_PATH/pkg/mod"

echo -e "\033[0;36m=== CodeTextor Vendor Fixer (Linux/macOS) ===\033[0m"

# 1. FIX CORE: go-tree-sitter
echo -e "\033[0;33m[1/4] Fixing go-tree-sitter...\033[0m"
GT_SOURCE=$(find "$MOD_CACHE" -maxdepth 4 -name "go-tree-sitter@*" -type d -print -quit)

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
echo -e "\033[0;33m[2/4] Fixing tree-sitter-sql (downloading generated files)...\033[0m"
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
echo -e "\033[0;33m[3/4] Checking other grammars...\033[0m"
# Find all tree-sitter grammars in vendor/github.com/tree-sitter
find "$VENDOR_DIR/github.com/tree-sitter" -maxdepth 1 -name "tree-sitter-*" -type d | while read g; do
    if [ ! -d "$g/bindings/go" ]; then continue; fi
    
    GRAMMAR_NAME=$(basename "$g")
    echo "  Checking $GRAMMAR_NAME..."
    
    # Search specifically in github.com/tree-sitter in cache
    SEARCH_DIR="$MOD_CACHE/github.com/tree-sitter"
    if [ ! -d "$SEARCH_DIR" ]; then continue; fi
    
    G_SOURCE=$(find "$SEARCH_DIR" -maxdepth 1 -name "$GRAMMAR_NAME@*" -type d | sort -r | head -n 1)
    
    if [ -n "$G_SOURCE" ]; then
        echo "    Found source at: $G_SOURCE"
        
        # Restore common folder if it exists
        if [ -d "$G_SOURCE/common" ]; then
            echo "    Restoring common folder..."
            cp -r "$G_SOURCE/common" "$g/"
        fi

        if [ "$GRAMMAR_NAME" == "tree-sitter-typescript" ]; then
            echo "    Handling typescript subfolders..."
            for sub in typescript tsx; do
                if [ -d "$G_SOURCE/$sub/src" ]; then
                    echo "      Restoring $sub/src..."
                    mkdir -p "$g/$sub"
                    cp -r "$G_SOURCE/$sub/src" "$g/$sub/"
                fi
            done
        elif [ "$GRAMMAR_NAME" == "tree-sitter-markdown" ]; then
            echo "    Handling markdown subfolders..."
            for sub in tree-sitter-markdown tree-sitter-markdown-inline; do
                if [ -d "$G_SOURCE/$sub/src" ]; then
                    echo "      Restoring $sub/src..."
                    mkdir -p "$g/$sub"
                    cp -r "$G_SOURCE/$sub/src" "$g/$sub/"
                fi
            done
        else
            if [ -d "$G_SOURCE/src" ]; then
                echo "    Restoring src..."
                cp -r "$G_SOURCE/src" "$g/"
            fi
            if [ -d "$G_SOURCE/queries" ]; then
                echo "    Restoring queries..."
                cp -r "$G_SOURCE/queries" "$g/"
            fi
        fi
    else
        echo -e "\033[0;31m    Warning: Could not find source for $GRAMMAR_NAME in $SEARCH_DIR\033[0m"
    fi
done

# 4. Handle tree-sitter-grammars/tree-sitter-markdown specifically
MG_SOURCE=$(find "$MOD_CACHE/github.com/tree-sitter-grammars" -maxdepth 1 -name "tree-sitter-markdown@*" -type d 2>/dev/null | sort -r | head -n 1)
if [ -n "$MG_SOURCE" ]; then
    MG_VENDOR="$VENDOR_DIR/github.com/tree-sitter-grammars/tree-sitter-markdown"
    if [ -d "$MG_VENDOR" ]; then
        echo -e "\033[0;33m[4/4] Fixing tree-sitter-grammars/tree-sitter-markdown...\033[0m"
        
        # Restore common folder if it exists
        if [ -d "$MG_SOURCE/common" ]; then
            echo "  Restoring common folder..."
            cp -r "$MG_SOURCE/common" "$MG_VENDOR/"
        fi

        for sub in tree-sitter-markdown tree-sitter-markdown-inline; do
            if [ -d "$MG_SOURCE/$sub/src" ]; then
                echo "  Restoring $sub/src..."
                mkdir -p "$MG_VENDOR/$sub"
                cp -r "$MG_SOURCE/$sub/src" "$MG_VENDOR/$sub/"
            fi
        done
    fi
fi

echo -e "\033[0;32m=== Operation Completed ===\033[0m"
echo "You can now run: wails build"
echo "Note: Since the vendor folder is tracked by Git, these fixes are now permanent in your repository."
