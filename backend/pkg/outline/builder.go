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
	"CodeTextor/backend/pkg/utils"
)

// BuildOutlineNodes constructs a tree of OutlineNode values from the ordered list of symbols.
// Returns the root nodes and a flat list of all nodes for easy lookup.
func BuildOutlineNodes(filePath string, symbols []chunker.Symbol) ([]*models.OutlineNode, []*models.OutlineNode) {
	// Map from symbol name to all nodes with that name
	symbolMap := make(map[string][]*models.OutlineNode)
	// Flat list of all nodes
	var allNodes []*models.OutlineNode
	// Track occurrences of the same span/name to keep IDs unique.
	idCounters := make(map[string]int)

	// Always create a virtual root node for the file
	rootNode := &models.OutlineNode{
		ID:        utils.GenerateSymbolID(filePath, 0, 0, "root", 1),
		Name:      "root",
		Kind:      "file",
		FilePath:  filePath,
		StartLine: 0,
		EndLine:   0,
	}
	allNodes = append(allNodes, rootNode)

	for _, symbol := range symbols {
		idKey := fmt.Sprintf("%s:%d:%d:%s", filePath, symbol.StartLine, symbol.EndLine, symbol.Name)
		idCounters[idKey]++

		node := &models.OutlineNode{
			ID:        utils.GenerateSymbolID(filePath, symbol.StartLine, symbol.EndLine, symbol.Name, idCounters[idKey]),
			Name:      symbol.Name,
			Kind:      string(symbol.Kind),
			FilePath:  filePath,
			StartLine: symbol.StartLine,
			EndLine:   symbol.EndLine,
		}

		parentName := strings.TrimSpace(symbol.Parent)
		var parent *models.OutlineNode

		if parentName != "" {
			// Find the correct parent by looking for a node with matching name
			// that contains this symbol's line range
			if candidates, found := symbolMap[parentName]; found {
				// Find the innermost (most recent) parent that contains this node
				for i := len(candidates) - 1; i >= 0; i-- {
					candidate := candidates[i]
					if candidate.StartLine <= node.StartLine && candidate.EndLine >= node.EndLine {
						parent = candidate
						break
					}
				}
			}
		}

		// If no specific parent found, attach to the virtual root
		if parent == nil {
			parent = rootNode
		}

		parent.Children = append(parent.Children, node)

		// Add this node to the symbol map
		symbolMap[symbol.Name] = append(symbolMap[symbol.Name], node)
		allNodes = append(allNodes, node)
	}

	return []*models.OutlineNode{rootNode}, allNodes
}
