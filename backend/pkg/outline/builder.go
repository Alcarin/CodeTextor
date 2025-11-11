/*
  File: builder.go
  Purpose: Create hierarchical outline trees from parser symbols.
  Author: CodeTextor project
  Notes: This package keeps the outline assembly logic separate from the indexing flow.
*/

package outline

import (
	"fmt"
	"strings"

	"CodeTextor/backend/internal/chunker"
	"CodeTextor/backend/pkg/models"
)

// BuildOutlineNodes constructs a tree of OutlineNode values from the ordered list of symbols.
func BuildOutlineNodes(filePath string, symbols []chunker.Symbol) []*models.OutlineNode {
	if len(symbols) == 0 {
		return nil
	}

	var roots []*models.OutlineNode
	// Map from symbol name to all nodes with that name
	symbolMap := make(map[string][]*models.OutlineNode)

	for _, symbol := range symbols {
		node := &models.OutlineNode{
			ID:        outlineNodeID(filePath, symbol),
			Name:      symbol.Name,
			Kind:      string(symbol.Kind),
			FilePath:  filePath,
			StartLine: symbol.StartLine,
			EndLine:   symbol.EndLine,
		}

		parentName := strings.TrimSpace(symbol.Parent)
		if parentName == "" {
			// No parent, add to roots
			roots = append(roots, node)
		} else {
			// Find the correct parent by looking for a node with matching name
			// that contains this symbol's line range
			if candidates, found := symbolMap[parentName]; found {
				var parent *models.OutlineNode
				// Find the innermost (most recent) parent that contains this node
				for i := len(candidates) - 1; i >= 0; i-- {
					candidate := candidates[i]
					if candidate.StartLine <= node.StartLine && candidate.EndLine >= node.EndLine {
						parent = candidate
						break
					}
				}
				if parent != nil {
					parent.Children = append(parent.Children, node)
				} else {
					// Parent not found by line range, add to roots
					roots = append(roots, node)
				}
			} else {
				// Parent name not found in map, add to roots
				roots = append(roots, node)
			}
		}

		// Add this node to the symbol map
		symbolMap[symbol.Name] = append(symbolMap[symbol.Name], node)
	}

	return roots
}

func outlineNodeID(filePath string, symbol chunker.Symbol) string {
	return fmt.Sprintf("%s:%d:%d:%s", filePath, symbol.StartLine, symbol.EndLine, symbol.Name)
}
