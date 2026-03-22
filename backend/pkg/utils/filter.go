/*
  File: filter.go
  Purpose: Shared logic for filtering files and directories based on patterns.
  Author: CodeTextor project
*/

package utils

import (
	"path/filepath"
	"strings"
)

// ShouldSkipPath determines if a file or directory should be skipped during indexing or watching.
// It handles absolute paths, relative paths, and .gitignore-style patterns.
func ShouldSkipPath(root, absPath string, patterns []string, autoExcludeHidden bool) bool {
	absClean := filepath.Clean(absPath)
	absSlash := filepath.ToSlash(absClean)
	name := filepath.Base(absClean)

	// 1. Check for hidden files/directories if enabled
	if autoExcludeHidden && strings.HasPrefix(name, ".") && len(name) > 1 {
		return true
	}

	if len(patterns) == 0 {
		return false
	}

	// Calculate relative path for pattern matching
	relPath := ""
	if rel, ok := RelativePathWithinRoot(root, absClean); ok {
		relPath = rel
	} else {
		relPath = filepath.ToSlash(absClean)
	}
	relPath = strings.TrimPrefix(relPath, "./")

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Normalize pattern
		patternSlash := filepath.ToSlash(filepath.Clean(pattern))
		patternSlash = strings.TrimSuffix(patternSlash, "/")
		
		// CASE 1: Pattern is an absolute path (from UI folder picker)
		if filepath.IsAbs(patternSlash) {
			if absSlash == patternSlash || strings.HasPrefix(absSlash, patternSlash+"/") {
				return true
			}
			continue
		}

		// CASE 2: Recursive patterns (e.g., **/node_modules)
		if strings.HasPrefix(patternSlash, "**/") {
			suffix := strings.TrimPrefix(patternSlash, "**/")
			if relPath == suffix || strings.HasPrefix(relPath, suffix+"/") || 
			   strings.HasSuffix(relPath, "/"+suffix) || strings.Contains(relPath, "/"+suffix+"/") {
				return true
			}
		}

		// CASE 3: Simple name matching (e.g., "vendor" matches any path component named vendor)
		if !strings.Contains(patternSlash, "/") {
			parts := strings.Split(relPath, "/")
			for _, part := range parts {
				if matched, _ := filepath.Match(patternSlash, part); matched {
					return true
				}
			}
		} else {
			// CASE 4: Relative path matching (e.g., "frontend/dist")
			if relPath == patternSlash || strings.HasPrefix(relPath, patternSlash+"/") {
				return true
			}
			// Fallback to glob match
			if matched, _ := filepath.Match(patternSlash, relPath); matched {
				return true
			}
		}
	}

	return false
}
