
# CodeTextor Vendor Fixer (Windows Version)
# This script fixes tree-sitter vendor directories on Windows by copying header/source files
# from the Go module cache OR downloading them (for SQL grammar) if they are missing.

$ErrorActionPreference = "Stop"
$vendorDir = "..\vendor"
$goPath = go env GOPATH
$modCache = Join-Path $goPath "pkg\mod"

Write-Host "=== CodeTextor Vendor Fixer (Windows) ===" -ForegroundColor Cyan

# 1. FIX CORE: go-tree-sitter
Write-Host "[1/3] Fixing go-tree-sitter..." -ForegroundColor Yellow
$gtSource = Get-ChildItem -Path $modCache -Include "go-tree-sitter@*" -Recurse | Select-Object -First 1
if ($gtSource) {
    $gtVendor = Join-Path $vendorDir "github.com\tree-sitter\go-tree-sitter"
    if (Test-Path $gtVendor) {
        $incSrc = Join-Path $gtSource.FullName "include"
        $srcSrc = Join-Path $gtSource.FullName "src"
        if (Test-Path $incSrc) { 
            Write-Host "  Copying include..."
            Copy-Item -Path $incSrc -Destination $gtVendor -Recurse -Force 
        }
        if (Test-Path $srcSrc) { 
            Write-Host "  Copying src..."
            Copy-Item -Path $srcSrc -Destination $gtVendor -Recurse -Force 
        }
    }
}

# 2. SPECIFIC FIX: tree-sitter-sql (DerekStride)
# This grammar doesn't include generated files (parser.c) in the master branch.
# We download them from the official gh-pages branch.
Write-Host "[2/3] Fixing tree-sitter-sql (downloading generated files)..." -ForegroundColor Yellow
$sqlVendorSrc = Join-Path $vendorDir "github.com\DerekStride\tree-sitter-sql\src"
if (!(Test-Path $sqlVendorSrc)) {
    New-Item -ItemType Directory -Force -Path $sqlVendorSrc | Out-Null
}

$baseUrl = "https://raw.githubusercontent.com/DerekStride/tree-sitter-sql/gh-pages/src"
$filesToDownload = @(
    "parser.c",
    "scanner.c",
    "tree_sitter/parser.h",
    "tree_sitter/alloc.h",
    "tree_sitter/array.h"
)

foreach ($file in $filesToDownload) {
    $destPath = Join-Path $sqlVendorSrc $file
    $destDir = Split-Path $destPath -Parent
    if (!(Test-Path $destDir)) { New-Item -ItemType Directory -Force -Path $destDir | Out-Null }
    
    Write-Host "  Downloading $file..."
    $url = "$baseUrl/$file"
    Invoke-WebRequest -Uri $url -OutFile $destPath -ErrorAction SilentlyContinue
}

# 3. GLOBAL GRAMMAR FIX: Other grammars
# Ensures all grammars have src/queries folders if they exist in the cache.
Write-Host "[3/3] Checking other grammars..." -ForegroundColor Yellow
$grammars = Get-ChildItem -Path $vendorDir -Recurse -Filter "tree-sitter-*" | Where-Object { $_.PSIsContainer }
foreach ($g in $grammars) {
    if (!(Test-Path (Join-Path $g.FullName "bindings\go"))) { continue }
    
    $relPath = $g.FullName.Substring($g.FullName.IndexOf("vendor\") + 7)
    if ($relPath -match "DerekStride\\tree-sitter-sql") { continue } # Skip SQL (already done)
    
    Write-Host "  Checking $($g.Name)..."
    
    $parentPath = Split-Path $relPath -Parent
    $encodedParentPath = $parentPath.ToLower().Replace("derekstride", "!derek!stride")
    $searchDir = Join-Path $modCache $encodedParentPath
    
    if (Test-Path $searchDir) {
        $leafName = Split-Path $relPath -Leaf
        $gSource = Get-ChildItem -Path $searchDir -Filter "$leafName@*" | Select-Object -First 1
        if ($gSource) {
            # Restore src and queries if missing
            if (!(Test-Path (Join-Path $g.FullName "src"))) {
                Write-Host "    Restoring src..."
                Copy-Item -Path (Join-Path $gSource.FullName "src") -Destination $g.FullName -Recurse -Force -ErrorAction SilentlyContinue
            }
            if ((Test-Path (Join-Path $gSource.FullName "queries")) -and !(Test-Path (Join-Path $g.FullName "queries"))) {
                Write-Host "    Restoring queries..."
                Copy-Item -Path (Join-Path $gSource.FullName "queries") -Destination $g.FullName -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

Write-Host "=== Operation Completed ===" -ForegroundColor Green
Write-Host "You can now run: wails build"
Write-Host "Note: Since the vendor folder is tracked by Git, these fixes are now permanent in your repository."
