package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComputeHash calculates the SHA-256 hash of the given content.
// This is used to detect file changes and determine if re-indexing is needed.
func ComputeHash(content []byte) string {
	hasher := sha256.New()
	hasher.Write(content)
	return hex.EncodeToString(hasher.Sum(nil))
}
