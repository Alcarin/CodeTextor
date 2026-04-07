package utils

import (
	"fmt"
	"strings"
)

// GenerateSymbolID creates a deterministic, human-readable identifier for a code symbol.
// It uses a slug format: path|Lstart-end|name[#counter].
// This ensures that the same symbol in the same file consistently produces the same ID
// across different tools (outline, indexing, etc.) while remaining token-efficient.
func GenerateSymbolID(filePath string, startLine, endLine uint32, name string, counter int) string {
	// Sanitize inputs to avoid conflicts with our separator '|'
	sanitize := func(s string) string {
		return strings.ReplaceAll(s, "|", "-")
	}

	safePath := sanitize(filePath)
	safeName := sanitize(name)
	if safeName == "" {
		safeName = "anonymous"
	}

	// Create a unique semantic descriptor string.
	// Format: path|Lstart-end|name
	id := fmt.Sprintf("%s|L%d-%d|%s", safePath, startLine, endLine, safeName)

	// Append counter only if it's relevant for uniqueness (> 1)
	if counter > 1 {
		id = fmt.Sprintf("%s#%d", id, counter)
	}

	return id
}
// NormalizeRelativePath converts a file path into a standard, cross-platform relative path.
// It ensures forward slashes are used and redundant separators are removed.
// This is used as the canonical ID for files in the database.
func NormalizeRelativePath(path string) string {
	// Convert to forward slashes for cross-platform consistency
	normalized := strings.ReplaceAll(path, "\\", "/")
	// Remove any leading "./" or "/"
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	return normalized
}
