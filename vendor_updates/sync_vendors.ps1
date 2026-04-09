
# CodeTextor Vendor Fixer (Windows Version)
# This script fixes tree-sitter vendor directories on Windows by copying header/source files
# from the Go module cache OR downloading them (for SQL grammar) if they are missing.

$ErrorActionPreference = "Stop"
$vendorDir = Join-Path $PSScriptRoot "..\vendor"
$goPath = go env GOPATH
$modCache = Join-Path $goPath "pkg\mod"

Write-Host "=== CodeTextor Vendor Fixer (Windows) ===" -ForegroundColor Cyan

# 1. FIX CORE: go-tree-sitter
Write-Host "[1/4] Fixing core: go-tree-sitter..." -ForegroundColor Yellow
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

# [2/6] (Obsolete) tree-sitter-sql is now handled in the Source-First section.


# 2. GLOBAL GRAMMAR FIX: Other grammars
Write-Host "[2/4] Checking standard vendor grammars..." -ForegroundColor Yellow
$grammars = Get-ChildItem -Path "$vendorDir\github.com\tree-sitter" -Filter "tree-sitter-*" | Where-Object { $_.PSIsContainer }
foreach ($g in $grammars) {
    if (!(Test-Path (Join-Path $g.FullName "bindings\go"))) { continue }
    
    $moduleName = $g.Name
    Write-Host "  Checking $moduleName..."
    
    # Direct search in mod cache for this specific module
    $searchDir = Join-Path $modCache "github.com\tree-sitter"
    $moduleSource = Get-ChildItem -Path $searchDir -Filter "$moduleName@*" -Directory | Sort-Object Name -Descending | Select-Object -First 1
    
    if ($moduleSource) {
        Write-Host "    Found source at: $($moduleSource.FullName)"
        
        # Restore common folder if it exists (needed by some grammars like typescript)
        $commonPath = Join-Path $moduleSource.FullName "common"
        if (Test-Path $commonPath) {
            Write-Host "    Restoring common folder..."
            $destCommon = Join-Path $g.FullName "common"
            if (!(Test-Path $destCommon)) { New-Item -ItemType Directory -Force -Path $destCommon | Out-Null }
            Copy-Item -Path $commonPath -Destination $g.FullName -Recurse -Force
        }

        # (TypeScript, Markdown, and PHP are now handled in the Source-First section below)
        $srcPath = Join-Path $moduleSource.FullName "src"
        $queriesPath = Join-Path $moduleSource.FullName "queries"
        
        if (Test-Path $srcPath) {
            Write-Host "    Restoring src..."
            Copy-Item -Path $srcPath -Destination $g.FullName -Recurse -Force
        }
        if (Test-Path $queriesPath) {
            Write-Host "    Restoring queries..."
            Copy-Item -Path $queriesPath -Destination $g.FullName -Recurse -Force
        }
    } else {
        Write-Host "    Warning: Could not find source for $moduleName in $searchDir" -ForegroundColor Red
    }
}

# [4/6] (Obsolete) tree-sitter-markdown is now handled in the Source-First section.


# 3. TOKENIZER PATCHES
Write-Host "[3/4] Patching sugarme/tokenizer..." -ForegroundColor Yellow
$normFile = Join-Path $vendorDir "github.com\sugarme\tokenizer\normalizer\normalized.go"
if (Test-Path $normFile) {
    Write-Host "  Patching normalized.go..."
    $content = [System.IO.File]::ReadAllText($normFile)
    
    # Define tabs for indentation
    $t3 = "`t`t`t"
    $t4 = "`t`t`t`t"
    
    # Patch 1 & 2 use a similar pattern. We'll use regex to match even if spacing differs slightly.
    # Note: Regex uses \r?\n for line endings
    $pattern = "(?m)^(\s*)start := n\.alignments\[idx\]\[1\]\r?\n(\s*)end := n\.alignments\[idx\+totalBytesToRemove\]\[1\]"
    
    $replacement = "`$1var start, end int`r`n" +
                   "`$1if idx >= len(n.alignments) {`r`n" +
                   "`$1`tstart = n.alignments[len(n.alignments)-1][1]`r`n" +
                   "`$1} else {`r`n" +
                   "`$1`tstart = n.alignments[idx][1]`r`n" +
                   "`$1}`r`n`r`n" +
                   "`$1if idx+totalBytesToRemove >= len(n.alignments) {`r`n" +
                   "`$1`tend = n.alignments[len(n.alignments)-1][1]`r`n" +
                   "`$1} else {`r`n" +
                   "`$1`tend = n.alignments[idx+totalBytesToRemove][1]`r`n" +
                   "`$1}"

    if ($content -match "start := n\.alignments\[idx\]\[1\]") {
        $content = [System.Text.RegularExpressions.Regex]::Replace($content, $pattern, $replacement)
        [System.IO.File]::WriteAllText($normFile, $content)
        Write-Host "    Successfully applied protections via regex."
    } else {
        Write-Host "    Protections already present or file structure changed." -ForegroundColor DarkGray
    }
}

# 4. INTERNAL VENDOR SYNC (Source-First & Flat Structure)
Write-Host "[4/4] Syncing internal grammars (vendor_grammar) - Flat & Lean..." -ForegroundColor Yellow

$internalGrammars = @(
    @{ name = "kotlin";          repo = "https://github.com/fwcd/tree-sitter-kotlin.git";            tag = "main"; subdirs = @(".") }
    @{ name = "dart";            repo = "https://github.com/nielsenko/tree-sitter-dart.git";          tag = "main";   subdirs = @(".") }
    @{ name = "swift";           repo = "https://github.com/alex-pinkus/tree-sitter-swift.git";       tag = "main";   subdirs = @(".") }
    @{ name = "sql";             repo = "https://github.com/DerekStride/tree-sitter-sql.git";         tag = "main";   subdirs = @(".") }
    @{ name = "typescript";      repo = "https://github.com/tree-sitter/tree-sitter-typescript.git";  tag = "v0.23.2"; subdirs = @("typescript") }
    @{ name = "tsx";             repo = "https://github.com/tree-sitter/tree-sitter-typescript.git";  tag = "v0.23.2"; subdirs = @("tsx") }
    @{ name = "markdown";        repo = "https://github.com/tree-sitter-grammars/tree-sitter-markdown.git"; tag = "v0.5.1"; subdirs = @("tree-sitter-markdown") }
    @{ name = "markdown_inline"; repo = "https://github.com/tree-sitter-grammars/tree-sitter-markdown.git"; tag = "v0.5.1"; subdirs = @("tree-sitter-markdown-inline") }
    @{ name = "php";             repo = "https://github.com/tree-sitter/tree-sitter-php.git";         tag = "master"; subdirs = @("php") }
    @{ name = "php_only";        repo = "https://github.com/tree-sitter/tree-sitter-php.git";         tag = "master"; subdirs = @("php_only") }
)

$vendorGrammarRoot = Join-Path $PSScriptRoot "..\backend\internal\chunker\vendor_grammar"

function Update-BindingGo {
    param($langDir, $grammarName)
    
    $bindingFile = Join-Path $langDir "binding.go"
    $bindingContent = @"
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
"@
    $Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($bindingFile, $bindingContent, $Utf8NoBom)
}

foreach ($grammar in $internalGrammars) {
    $grammarName = $grammar.name
    $repoUrl = $grammar.repo
    $tag = $grammar.tag
    $subdirs = $grammar.subdirs
    
    Write-Host "  Processing $grammarName (Flat & Lean)..." -ForegroundColor Cyan
    
    $oldEAP_Loop = $ErrorActionPreference
    $ErrorActionPreference = "Continue"

    $langDir = Join-Path $vendorGrammarRoot $grammarName
    if (!(Test-Path $langDir)) { New-Item -ItemType Directory -Force -Path $langDir | Out-Null }
    
    # 1. Clone into temporary location
    $tmpClone = Join-Path $langDir ".tmp_clone"
    if (Test-Path $tmpClone) { Remove-Item -Recurse -Force $tmpClone }
    
    # Clean up old sources folder if it exists
    $oldSources = Join-Path $langDir "sources"
    if (Test-Path $oldSources) { Remove-Item -Recurse -Force $oldSources }

    Write-Host "    Cloning $grammarName ($tag)..." -ForegroundColor Gray
    
    $oldEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    & git clone --depth 1 --branch $tag $repoUrl $tmpClone 2>$null
    $gitStatus = $LASTEXITCODE
    $ErrorActionPreference = $oldEAP
    
    if ($gitStatus -ne 0) {
        Write-Host "    Error: Failed to clone $grammarName" -ForegroundColor Red
        continue
    }

    foreach ($sub in $subdirs) {
        $subSourcesDir = if ($sub -eq ".") { $tmpClone } else { Join-Path $tmpClone $sub }
        $srcDir = Join-Path $subSourcesDir "src"
        
        # 2. Generate Parser
        Write-Host "    Generating parser..." -ForegroundColor Gray
        Push-Location $subSourcesDir
        & tree-sitter generate 2>&1 | Out-Null
        Pop-Location

        # 3. Extract essential files
        Write-Host "    Extracting essential files..." -ForegroundColor Gray
        if (Test-Path "$srcDir\parser.c") { Copy-Item "$srcDir\parser.c" $langDir -Force }
        
        $hasScanner = $false
        $isCPP = $false
        if (Test-Path "$srcDir\scanner.c") {
            Copy-Item "$srcDir\scanner.c" $langDir -Force
            $hasScanner = $true
        }
        elseif (Test-Path "$srcDir\scanner.cc") {
            Copy-Item "$srcDir\scanner.cc" $langDir -Force
            $hasScanner = $true
            $isCPP = $true
        }

        # 4. Extract 'common' folder headers if they exist in repo root (needed for PHP/TS)
        $commonDir = Join-Path $tmpClone "common"
        if (Test-Path $commonDir) {
            Write-Host "    Found 'common' directory, extracting headers..." -ForegroundColor Gray
            Get-ChildItem -Path $commonDir -Filter "*.h" | Copy-Item -Destination $langDir -Force
        }

        # 5. Patch scanner broken relative includes (../../common/ -> local)
        $scannerFiles = Get-ChildItem -Path $langDir -Filter "scanner.c*"
        foreach ($sf in $scannerFiles) {
            $content = Get-Content $sf.FullName -Raw
            if ($content -match '\.\./\.\./common/') {
                Write-Host "    Patching scanner relative include in $($sf.Name)..." -ForegroundColor Gray
                $content = $content -replace '\.\./\.\./common/', ''
                Set-Content $sf.FullName $content -NoNewline
            }
        }

        # 6. Extract internal tree-sitter headers
        $targetHeaderDir = Join-Path $langDir "tree_sitter"
        if (!(Test-Path $targetHeaderDir)) { New-Item -ItemType Directory -Path $targetHeaderDir -Force | Out-Null }
        
        $sourceHeaderDir = Join-Path $srcDir "tree_sitter"
        if (Test-Path $sourceHeaderDir) {
            Copy-Item (Join-Path $sourceHeaderDir "*.h") -Destination $targetHeaderDir -Force
        }

        # 5. Download core headers (always for consistency)
        $baseUrl = "https://raw.githubusercontent.com/tree-sitter/tree-sitter/master/lib/src/"
        $headers = @("array.h", "alloc.h", "ts_assert.h")
        foreach ($h in $headers) {
            Invoke-WebRequest -Uri ($baseUrl + $h) -OutFile (Join-Path $targetHeaderDir $h) -TimeoutSec 10 -ErrorAction SilentlyContinue
        }

        # 6. Apply language-specific patches
        if ($grammarName -eq "swift") {
            $destScanner = Join-Path $langDir "scanner.c"
            if (Test-Path $destScanner) {
                $sContent = [System.IO.File]::ReadAllText($destScanner)
                $sContent = $sContent -replace "1UL << FAKE_TRY_BANG", "1ULL << FAKE_TRY_BANG"
                [System.IO.File]::WriteAllText($destScanner, $sContent)
                Write-Host "      Applied Swift shift overflow patch." -ForegroundColor Green
            }
        }

        # 7. Update binding.go
        Update-BindingGo $langDir $grammarName
    }

    # cleanup
    Remove-Item -Recurse -Force $tmpClone
    $ErrorActionPreference = $oldEAP_Loop
    Write-Host "    Cleanup completed for $grammarName" -ForegroundColor DarkGray
}

Write-Host "=== Operation Completed ===" -ForegroundColor Green
Write-Host "You can now run: wails build"
Write-Host "Note: Since the vendor folder is tracked by Git, these fixes are now permanent in your repository."
