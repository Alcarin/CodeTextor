package utils

import (
	"path/filepath"
	"strings"
)

// RelativePathWithinRoot attempts to express absPath relative to root.
// Returns the normalized relative path (with forward slashes) and true if successful.
// If the root is empty, invalid, or the path falls outside of it, the second return
// value is false and the caller should treat the original absolute path as authoritative.
func RelativePathWithinRoot(root, absPath string) (string, bool) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", false
	}

	rootClean := filepath.Clean(root)
	if !filepath.IsAbs(rootClean) {
		if absRoot, err := filepath.Abs(rootClean); err == nil {
			rootClean = absRoot
		} else {
			return "", false
		}
	}

	target := filepath.Clean(absPath)
	if !filepath.IsAbs(target) {
		return "", false
	}

	rel, err := filepath.Rel(rootClean, target)
	if err != nil {
		return "", false
	}

	if rel == "." {
		return ".", true
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
