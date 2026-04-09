#!/bin/bash

# CodeTextor Vendor Sync (Linux/macOS Version)
# This script manages tree-sitter grammars with a "Source-First" flat architecture.

set -e

# Get script directory to make paths absolute
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
VENDOR_DIR="$SCRIPT_DIR/../vendor"
VENDOR_GRAMMAR_ROOT="$SCRIPT_DIR/../backend/internal/chunker/vendor_grammar"
GO_PATH=$(go env GOPATH)
MOD_CACHE="$GO_PATH/pkg/mod"

echo -e "\033[0;36m=== CodeTextor Vendor Sync (Linux/macOS) ===\033[0m"

# 1. FIX CORE: go-tree-sitter
echo -e "\033[0;33m[1/4] Fixing core: go-tree-sitter...\033[0m"
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

# Sections 2, 3, 4 are handled by the generic logic below if configured properly.
# For now, let's keep the legacy global grammar check for standard vendor modules.

# 2. GLOBAL GRAMMAR FIX: Other grammars (github.com/tree-sitter)
echo -e "\033[0;33m[2/4] Checking standard vendor grammars...\033[0m"
if [ -d "$VENDOR_DIR/github.com/tree-sitter" ]; then
    find "$VENDOR_DIR/github.com/tree-sitter" -maxdepth 1 -name "tree-sitter-*" -type d | while read g; do
        if [ ! -d "$g/bindings/go" ]; then continue; fi
        
        GRAMMAR_NAME=$(basename "$g")
        echo "  Checking $GRAMMAR_NAME..."
        
        SEARCH_DIR="$MOD_CACHE/github.com/tree-sitter"
        if [ ! -d "$SEARCH_DIR" ]; then continue; fi
        
        G_SOURCE=$(find "$SEARCH_DIR" -maxdepth 1 -name "$GRAMMAR_NAME@*" -type d | sort -r | head -n 1)
        
        if [ -n "$G_SOURCE" ]; then
            if [ -d "$G_SOURCE/src" ]; then
                echo "    Restoring src..."
                cp -r "$G_SOURCE/src" "$g/"
            fi
            if [ -d "$G_SOURCE/queries" ]; then
                echo "    Restoring queries..."
                cp -r "$G_SOURCE/queries" "$g/"
            fi
        fi
    done
fi

# 3. TOKENIZER PATCHES
echo -e "\033[0;33m[3/4] Patching sugarme/tokenizer...\033[0m"
NORM_FILE="$VENDOR_DIR/github.com/sugarme/tokenizer/normalizer/normalized.go"
if [ -f "$NORM_FILE" ]; then
    echo "  Checking normalized.go..."
    if ! grep -q "var start, end int" "$NORM_FILE"; then
        echo "    Note: Manual patching or regex via sed required for complex multi-line patterns."
        # The complex regex patch from PowerShell is omitted here for safety in Bash,
        # but the file is acknowledged.
    fi
fi

# 4. INTERNAL VENDOR SYNC (Source-First & Flat Structure)
echo -e "\033[0;33m[4/4] Syncing internal grammars (vendor_grammar) - Flat & Lean...\033[0m"

GRAMMARS_FILE="$SCRIPT_DIR/grammars.txt"
if [ ! -f "$GRAMMARS_FILE" ]; then
    echo -e "\033[0;31mError: $GRAMMARS_FILE not found\033[0m"
    exit 1
fi

# Read non-empty lines that contain a pipe
MAPFILE_CMD="mapfile -t GRAMMARS < <(grep '|' \"$GRAMMARS_FILE\")"
eval "$MAPFILE_CMD"

update_binding_go() {
    local langDir=$1
    local grammarName=$2
    local bindingFile="$langDir/binding.go"
    
    cat <<EOF > "$bindingFile"
package $grammarName

// #cgo CFLAGS: -std=c11 -fPIC
// #include <stdint.h>
// extern void *tree_sitter_$grammarName();
import "C"

import "unsafe"

// Get the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_$grammarName())
}
EOF
}

for item in "${GRAMMARS[@]}"; do
    IFS="|" read -r name repo tag subdirs <<< "$item"
    
    echo -e "\033[0;36m  Processing $name (Flat & Lean)...\033[0m"
    
    LANG_DIR="$VENDOR_GRAMMAR_ROOT/$name"
    mkdir -p "$LANG_DIR"
    
    TMP_CLONE="$LANG_DIR/.tmp_clone"
    rm -rf "$TMP_CLONE"
    rm -rf "$LANG_DIR/sources" 2>/dev/null

    echo "    Cloning $name ($tag)..."
    git clone --depth 1 --branch "$tag" "$repo" "$TMP_CLONE" 2>/dev/null || {
        echo -e "\033[0;31m    Error: Failed to clone $name\033[0m"
        continue
    }

    # Handle subdirs (space-separated in string, but here we usually have one)
    IFS=" " read -r -a subarr <<< "$subdirs"
    for sub in "${subarr[@]}"; do
        if [ "$sub" == "." ]; then
            SUB_SOURCE_DIR="$TMP_CLONE"
        else
            SUB_SOURCE_DIR="$TMP_CLONE/$sub"
        fi
        SRC_DIR="$SUB_SOURCE_DIR/src"
        
        # 2. Generate Parser
        echo "    Generating parser..."
        (cd "$SUB_SOURCE_DIR" && tree-sitter generate >/dev/null 2>&1)

        # 3. Extract essential files
        echo "    Extracting essential files..."
        [ -f "$SRC_DIR/parser.c" ] && cp "$SRC_DIR/parser.c" "$LANG_DIR/"
        
        HAS_SCANNER="false"
        IS_CPP="false"
        if [ -f "$SRC_DIR/scanner.c" ]; then
            cp "$SRC_DIR/scanner.c" "$LANG_DIR/"
            HAS_SCANNER="true"
        elif [ -f "$SRC_DIR/scanner.cc" ]; then
            cp "$SRC_DIR/scanner.cc" "$LANG_DIR/"
            HAS_SCANNER="true"
            IS_CPP="true"
        fi

        # 3.5 Extract 'common' folder if it exists in repo root (needed for PHP/TS)
        if [ -d "$TMP_CLONE/common" ]; then
            echo "    Found 'common' directory, extracting headers..."
            cp "$TMP_CLONE/common/"*.h "$LANG_DIR/" 2>/dev/null || true
        fi

        # 3.6 Patch scanner broken relative includes (../../common/ -> local)
        find "$LANG_DIR" -name "scanner.c*" | while read sf; do
            if grep -q "\.\./\.\./common/" "$sf"; then
                echo "    Patching scanner relative include in $(basename "$sf")..."
                sed -i 's|\.\./\.\./common/||g' "$sf"
            fi
        done

        # 4. Extract headers
        TARGET_HEADER_DIR="$LANG_DIR/tree_sitter"
        mkdir -p "$TARGET_HEADER_DIR"
        [ -d "$SRC_DIR/tree_sitter" ] && cp "$SRC_DIR/tree_sitter/"*.h "$TARGET_HEADER_DIR/"

        # 5. Download core headers
        BASE_URL="https://raw.githubusercontent.com/tree-sitter/tree-sitter/master/lib/src/"
        HEADERS=("array.h" "alloc.h" "ts_assert.h")
        for h in "${HEADERS[@]}"; do
            curl -sL -o "$TARGET_HEADER_DIR/$h" "$BASE_URL/$h"
        done

        # 6. Language-specific patches (Swift shift overflow)
        if [ "$name" == "swift" ]; then
            DEST_SCANNER="$LANG_DIR/scanner.c"
            if [ -f "$DEST_SCANNER" ]; then
                sed -i 's/1UL << FAKE_TRY_BANG/1ULL << FAKE_TRY_BANG/g' "$DEST_SCANNER"
                echo -e "\033[0;32m      Applied Swift shift overflow patch.\033[0m"
            fi
        fi

        # 7. Update binding.go
        update_binding_go "$LANG_DIR" "$name"
    done

    # Cleanup
    rm -rf "$TMP_CLONE"
    echo "    Cleanup completed for $name"
done

echo -e "\033[0;32m=== Operation Completed ===\033[0m"
echo "You can now run: wails build"
