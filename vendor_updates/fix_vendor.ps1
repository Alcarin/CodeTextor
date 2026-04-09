
# CodeTextor Vendor Fixer (Windows Version)
# This script fixes tree-sitter vendor directories on Windows by copying header/source files
# from the Go module cache OR downloading them (for SQL grammar) if they are missing.

$ErrorActionPreference = "Stop"
$vendorDir = Join-Path $PSScriptRoot "..\vendor"
$goPath = go env GOPATH
$modCache = Join-Path $goPath "pkg\mod"

Write-Host "=== CodeTextor Vendor Fixer (Windows) ===" -ForegroundColor Cyan

# 1. FIX CORE: go-tree-sitter
Write-Host "[1/6] Fixing go-tree-sitter..." -ForegroundColor Yellow
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
Write-Host "[2/6] Fixing tree-sitter-sql (downloading generated files)..." -ForegroundColor Yellow
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
Write-Host "[3/6] Checking other grammars..." -ForegroundColor Yellow
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

        if ($moduleName -eq "tree-sitter-typescript") {
            Write-Host "    Handling typescript subfolders..."
            foreach ($sub in @("typescript", "tsx")) {
                $srcPath = Join-Path $moduleSource.FullName "$sub\src"
                $destPath = Join-Path $g.FullName "$sub" # Should be vendor/.../tree-sitter-typescript/typescript/
                if (Test-Path $srcPath) {
                    Write-Host "      Restoring $sub\src..."
                    if (!(Test-Path $destPath)) { New-Item -ItemType Directory -Force -Path $destPath | Out-Null }
                    Copy-Item -Path $srcPath -Destination $destPath -Recurse -Force
                }
            }
        } elseif ($moduleName -eq "tree-sitter-markdown") {
            Write-Host "    Handling markdown subfolders..."
            foreach ($sub in @("tree-sitter-markdown", "tree-sitter-markdown-inline")) {
                $srcPath = Join-Path $moduleSource.FullName "$sub\src"
                $destPath = Join-Path $g.FullName "$sub"
                if (Test-Path $srcPath) {
                    Write-Host "      Restoring $sub\src..."
                    if (!(Test-Path $destPath)) { New-Item -ItemType Directory -Force -Path $destPath | Out-Null }
                    Copy-Item -Path $srcPath -Destination $destPath -Recurse -Force
                }
            }
        } elseif ($moduleName -eq "tree-sitter-php") {
            Write-Host "    Handling php subfolders..."
            foreach ($sub in @("php", "php_only")) {
                $srcPath = Join-Path $moduleSource.FullName "$sub\src"
                $destPath = Join-Path $g.FullName "$sub"
                if (Test-Path $srcPath) {
                    Write-Host "      Restoring $sub\src..."
                    if (!(Test-Path $destPath)) { New-Item -ItemType Directory -Force -Path $destPath | Out-Null }
                    Copy-Item -Path $srcPath -Destination $destPath -Recurse -Force
                }
            }
        } else {
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
        }
    } else {
        Write-Host "    Warning: Could not find source for $moduleName in $searchDir" -ForegroundColor Red
    }
}

# 4. Handle tree-sitter-grammars/tree-sitter-markdown specifically
$mgSource = Get-ChildItem -Path "$modCache\github.com\tree-sitter-grammars" -Filter "tree-sitter-markdown@*" -Directory | Sort-Object Name -Descending | Select-Object -First 1
if ($mgSource) {
    $mgVendor = Join-Path $vendorDir "github.com\tree-sitter-grammars\tree-sitter-markdown"
    if (Test-Path $mgVendor) {
        Write-Host "[4/6] Fixing tree-sitter-grammars/tree-sitter-markdown..." -ForegroundColor Yellow
        
        # Restore common folder if it exists
        $commonPath = Join-Path $mgSource.FullName "common"
        if (Test-Path $commonPath) {
            Write-Host "  Restoring common folder..."
            if (!(Test-Path (Join-Path $mgVendor "common"))) { New-Item -ItemType Directory -Force -Path (Join-Path $mgVendor "common") | Out-Null }
            Copy-Item -Path $commonPath -Destination $mgVendor -Recurse -Force
        }

        foreach ($sub in @("tree-sitter-markdown", "tree-sitter-markdown-inline")) {
            $srcPath = Join-Path $mgSource.FullName "$sub\src"
            $destPath = Join-Path $mgVendor "$sub" # Should stay as vendor/.../tree-sitter-markdown/tree-sitter-markdown/
            if (Test-Path $srcPath) {
                Write-Host "  Restoring $sub\src..."
                if (!(Test-Path $destPath)) { New-Item -ItemType Directory -Force -Path $destPath | Out-Null }
                Copy-Item -Path $srcPath -Destination $destPath -Recurse -Force
            }
        }
    }
}

# 5. TOKENIZER PATCHES
Write-Host "[5/6] Patching sugarme/tokenizer..." -ForegroundColor Yellow
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

# 6. INTERNAL VENDOR SYNC
Write-Host "[6/6] Syncing internal grammars (vendor_grammar)..." -ForegroundColor Yellow
$internalGrammars = @{
    "kotlin" = "https://raw.githubusercontent.com/fwcd/tree-sitter-kotlin/master"
    "dart"   = "https://raw.githubusercontent.com/nielsenko/tree-sitter-dart/main"
    "swift"  = "https://raw.githubusercontent.com/alex-pinkus/tree-sitter-swift/main"
}

foreach ($grammarName in $internalGrammars.Keys) {
    $baseUrl = $internalGrammars[$grammarName]
    Write-Host "  Processing $grammarName (Source-First approach)..." -ForegroundColor Cyan
    
    $vendorGrammarRoot = Join-Path $PSScriptRoot "..\backend\internal\chunker\vendor_grammar"
    $langDir = Join-Path $vendorGrammarRoot $grammarName
    $sourcesDir = Join-Path $langDir "sources"
    $sourcesSrcDir = Join-Path $sourcesDir "src"

    if (!(Test-Path $langDir)) { New-Item -ItemType Directory -Force -Path $langDir | Out-Null }
    if (!(Test-Path $sourcesDir)) { New-Item -ItemType Directory -Force -Path $sourcesDir | Out-Null }
    if (!(Test-Path $sourcesSrcDir)) { New-Item -ItemType Directory -Force -Path $sourcesSrcDir | Out-Null }

    # 1. Download Binding and Source files
    $coreFiles = @{
        "bindings/go/binding.go" = "binding.go"
        "grammar.js"             = "sources/grammar.js"
        "package.json"           = "sources/package.json"
        "tree-sitter.json"       = "sources/tree-sitter.json"
        "src/scanner.c"          = "sources/src/scanner.c"
    }

    foreach ($fPath in $coreFiles.Keys) {
        $destRel = $coreFiles[$fPath]
        $destPath = Join-Path $langDir ($destRel.Replace("/", "\"))
        $destDir = Split-Path $destPath -Parent
        if (!(Test-Path $destDir)) { New-Item -ItemType Directory -Force -Path $destDir | Out-Null }

        Write-Host "    Downloading $fPath -> $(Split-Path $destPath -Leaf)..."
        $url = "$baseUrl/$fPath"
        try {
            Invoke-WebRequest -Uri $url -OutFile $destPath -ErrorAction Stop
        } catch {
            if ($fPath -match "scanner.c") {
                 Write-Host "    (Note: scanner.c not found, might be a pure grammar)" -ForegroundColor Gray
            } else {
                 Write-Host "    Warning: Could not download $fPath" -ForegroundColor Yellow
            }
            if (Test-Path $destPath) { Remove-Item $destPath }
        }
    }

    # 2. Patch binding.go
    $bindingFile = Join-Path $langDir "binding.go"
    if (Test-Path $bindingFile) {
        $content = [System.IO.File]::ReadAllText($bindingFile)
        $content = $content -replace "package tree_sitter_$grammarName", "package $grammarName"
        # Force all internal grammars to use the 'sources/src/' path in binding.go
        $content = $content -replace '"src/', '"sources/src/'
        $content = $content -replace '"../../src/', '"sources/src/'
        [System.IO.File]::WriteAllText($bindingFile, $content)
        Write-Host "    Updated binding.go: package '$grammarName' and CGO paths fixed." -ForegroundColor Green
    }

    # 3. Generate Parser
    $parserFile = Join-Path $sourcesSrcDir "parser.c"
    if (!(Test-Path $parserFile)) {
        Write-Host "    Generating parser for $grammarName..." -ForegroundColor Yellow
        $globalTS = Get-Command "tree-sitter" -ErrorAction SilentlyContinue
        
        if ($globalTS) {
            Write-Host "      Using GLOBAL tree-sitter-cli..." -ForegroundColor Cyan
            Push-Location $sourcesDir
            try {
                tree-sitter generate
                Write-Host "      Generation successful!" -ForegroundColor Green
            } catch {
                Write-Host "      Generation failed: $($_.Exception.Message)" -ForegroundColor Red
            }
            Pop-Location
        } elseif (Get-Command "npm" -ErrorAction SilentlyContinue) {
            Write-Host "      Using LOCAL npm fallback..." -ForegroundColor Cyan
            Push-Location $sourcesDir
            try {
                npm install tree-sitter-cli@latest --no-save
                .\node_modules\.bin\tree-sitter.cmd generate
                Write-Host "      Generation successful (local)!" -ForegroundColor Green
                
                # Cleanup local install
                Pop-Location
                Remove-Item -Path (Join-Path $sourcesDir "node_modules") -Recurse -Force -ErrorAction SilentlyContinue
                Remove-Item -Path (Join-Path $sourcesDir "package-lock.json") -Force -ErrorAction SilentlyContinue
            } catch {
                if ($null -ne $sourcesDir) { try { Pop-Location } catch {} }
                Write-Host "      Local generation failed: $($_.Exception.Message)" -ForegroundColor Red
            }
        } else {
            Write-Host "      Warning: tree-sitter or npm not found. Parser.c cannot be generated automatically." -ForegroundColor Magenta
        }
    }

    # 4. Apply Language-specific patches
    if ($grammarName -eq "swift") {
        Write-Host "    Applying Swift-specific CGO patches..." -ForegroundColor Cyan
        
        # Patch 1: binding.go (#undef TOKEN_COUNT)
        if (Test-Path $bindingFile) {
            $bContent = [System.IO.File]::ReadAllText($bindingFile)
            if ($bContent -notmatch "#undef TOKEN_COUNT") {
                $bContent = $bContent -replace '(#include "sources/src/parser.c")', "`$1`r`n// #undef TOKEN_COUNT"
                [System.IO.File]::WriteAllText($bindingFile, $bContent)
                Write-Host "      Patched binding.go (#undef TOKEN_COUNT)" -ForegroundColor Green
            }
        }

        # Patch 2: scanner.c (Shift Overflow: 1UL -> 1ULL)
        $scannerFile = Join-Path $sourcesSrcDir "scanner.c"
        if (Test-Path $scannerFile) {
            $sContent = [System.IO.File]::ReadAllText($scannerFile)
            if ($sContent -match "1UL << FAKE_TRY_BANG") {
                $sContent = $sContent -replace "1UL << FAKE_TRY_BANG", "1ULL << FAKE_TRY_BANG"
                [System.IO.File]::WriteAllText($scannerFile, $sContent)
                Write-Host "      Patched scanner.c (1UL -> 1ULL shift overflow fix)" -ForegroundColor Green
            }
        }
    }
}


Write-Host "=== Operation Completed ===" -ForegroundColor Green
Write-Host "You can now run: wails build"
Write-Host "Note: Since the vendor folder is tracked by Git, these fixes are now permanent in your repository."
