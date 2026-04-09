package dart

// #cgo CFLAGS: -std=c11 -fPIC
// #include "sources/src/parser.c"
// #if __has_include("sources/src/scanner.c")
// #include "sources/src/scanner.c"
// #endif
import "C"

import "unsafe"

// Get the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_dart())
}
