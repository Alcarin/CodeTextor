package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateSymbolID creates a deterministic, fixed-length identifier for a code symbol.
// It uses a SHA-256 hash of the semantic descriptor (path, range, name, counter).
// This ensures that the same symbol in the same file consistently produces the same ID
// across different tools (outline, indexing, etc.) without relying on UUIDs.
func GenerateSymbolID(filePath string, startLine, endLine uint32, name string, counter int) string {
	// Create a unique semantic descriptor string.
	// Format: path:start:end:name:counter
	descriptor := fmt.Sprintf("%s:%d:%d:%s:%d", filePath, startLine, endLine, name, counter)

	// Hash the descriptor using SHA-256 to get a fixed-length ID.
	hasher := sha256.New()
	hasher.Write([]byte(descriptor))
	return hex.EncodeToString(hasher.Sum(nil))
}
