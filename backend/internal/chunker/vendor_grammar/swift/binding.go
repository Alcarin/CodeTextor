package swift

/*
  File: binding.go
  Purpose: CGO binding for the Swift Tree-sitter grammar.
*/

// #cgo CFLAGS: -std=c11 -fPIC
// #include "sources/src/parser.c"
// #undef TOKEN_COUNT
// #include "sources/src/scanner.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language for Swift.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_swift())
}
