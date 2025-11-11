package outline

import (
	"testing"

	"CodeTextor/backend/internal/chunker"
	"CodeTextor/backend/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nodeExpectation struct {
	nodeName   string
	parentName *string
	childName  string
}

func TestBuildOutlineNodesMaintainsHierarchy(t *testing.T) {
	parser := chunker.NewParser(chunker.DefaultChunkConfig())

	tests := []struct {
		name     string
		filePath string
		source   string
		checks   []nodeExpectation
	}{
		{
			name:     "GoLang",
			filePath: "calculator.go",
			source: `package main

type Calculator struct {}

func (c *Calculator) Multiply(a, b int) int {
    return a * b
}

func Add(a, b int) int {
    return a + b
}
`,
			checks: []nodeExpectation{
				{nodeName: "Calculator", parentName: strPtr("")},
				{nodeName: "Multiply"},
				{nodeName: "Add", parentName: strPtr("")},
			},
		},
		{
			name:     "Python",
			filePath: "calculator.py",
			source: `class Calculator:
    def multiply(self, a, b):
        return a * b

def add(a, b):
    return a + b
`,
			checks: []nodeExpectation{
				{nodeName: "Calculator", parentName: strPtr("")},
				{nodeName: "multiply", parentName: strPtr("Calculator")},
				{nodeName: "add", parentName: strPtr("")},
			},
		},
		{
			name:     "TypeScript",
			filePath: "calculator.ts",
			source: `export class Calculator {
    multiply(a: number, b: number): number {
        return a * b
    }
}

export function add(a: number, b: number): number {
    return a + b
}
`,
			checks: []nodeExpectation{
				{nodeName: "Calculator", parentName: strPtr("")},
				{nodeName: "multiply", parentName: strPtr("Calculator")},
				{nodeName: "add", parentName: strPtr("")},
			},
		},
		{
			name:     "Markdown",
			filePath: "outline.md",
			source: `# Title
## Section
### Subsection
## Another Section
`,
			checks: []nodeExpectation{
				{nodeName: "Title", parentName: strPtr("")},
				{nodeName: "Section", parentName: strPtr("Title"), childName: "Subsection"},
				{nodeName: "Subsection"},
				{nodeName: "Another Section", parentName: strPtr("Title")},
			},
		},
		{
			name:     "HTML",
			filePath: "page.html",
			source: `<div>
  <span>hello</span>
</div>
`,
			checks: []nodeExpectation{
				{nodeName: "div", parentName: strPtr("")},
				{nodeName: "span", parentName: strPtr("div")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseFile(tt.filePath, []byte(tt.source))
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Empty(t, result.Errors, "valid snippet should not produce parse errors")

			nodes := BuildOutlineNodes(tt.filePath, result.Symbols)
			require.NotNil(t, nodes)
			require.NotEmpty(t, nodes, "outline should contain nodes for %s", tt.name)

			for _, check := range tt.checks {
				node := findOutlineNode(nodes, check.nodeName)
				require.NotNil(t, node, "expected node %s to exist", check.nodeName)

				if check.parentName != nil {
					parent := findOutlineParent(nodes, node)
					parentName := ""
					if parent != nil {
						parentName = parent.Name
					}
					assert.Equal(t, *check.parentName, parentName, "node %s should be child of %s", check.nodeName, *check.parentName)
				}

				if check.childName != "" {
					child := findOutlineNode(node.Children, check.childName)
					assert.NotNil(t, child, "node %s should have child %s", check.nodeName, check.childName)
				}
			}
		})
	}
}

func TestBuildOutlineNodesHandlesMissingParentsAndDuplicates(t *testing.T) {
	symbols := []chunker.Symbol{
		{Name: "Orphan", Parent: "Missing", StartLine: 1, EndLine: 1},
		{Name: "Container", Parent: "", StartLine: 2, EndLine: 20},
		{Name: "div", Parent: "Container", StartLine: 3, EndLine: 5},
		{Name: "div", Parent: "Container", StartLine: 7, EndLine: 12},
		{Name: "div", Parent: "div", StartLine: 8, EndLine: 9},
	}

	nodes := BuildOutlineNodes("test.txt", symbols)
	require.NotNil(t, nodes)

	orphan := findOutlineNode(nodes, "Orphan")
	require.NotNil(t, orphan)

	container := findOutlineNode(nodes, "Container")
	require.NotNil(t, container)
	assert.Len(t, container.Children, 2, "container should have two div children")

	secondDiv := findOutlineNodeByLine(container.Children, "div", 7)
	require.NotNil(t, secondDiv)
	require.Len(t, secondDiv.Children, 1, "second div should have nested child")
	assert.Equal(t, uint32(8), secondDiv.Children[0].StartLine, "nested div should attach to the correct parent")
}

func strPtr(value string) *string {
	return &value
}

func findOutlineNode(nodes []*models.OutlineNode, name string) *models.OutlineNode {
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
		if child := findOutlineNode(node.Children, name); child != nil {
			return child
		}
	}
	return nil
}

func findOutlineNodeByLine(nodes []*models.OutlineNode, name string, startLine uint32) *models.OutlineNode {
	for _, node := range nodes {
		if node.Name == name && node.StartLine == startLine {
			return node
		}
		if child := findOutlineNodeByLine(node.Children, name, startLine); child != nil {
			return child
		}
	}
	return nil
}

func findOutlineParent(nodes []*models.OutlineNode, target *models.OutlineNode) *models.OutlineNode {
	for _, node := range nodes {
		for _, child := range node.Children {
			if child == target {
				return node
			}
			if parent := findOutlineParent(child.Children, target); parent != nil {
				return parent
			}
		}
	}
	return nil
}
