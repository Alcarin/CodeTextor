package sql

// #cgo CFLAGS: -std=c11 -fPIC
// #include <stdint.h>
// extern void *tree_sitter_sql();
import "C"

import "unsafe"

// Get the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_sql())
}