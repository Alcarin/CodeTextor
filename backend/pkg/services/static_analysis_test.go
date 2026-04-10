package services

import (
	"context"
	"path/filepath"
	"testing"

	"CodeTextor/backend/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectService_StaticAnalysis(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx := context.Background()

	// Setup temp project
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("APPDATA", tempDir)
	t.Setenv("LOCALAPPDATA", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("CODETEXTOR_APP_DATA", tempDir)
	t.Setenv("CODETEXTOR_INDEXES_DIR", filepath.Join(tempDir, "indexes"))

	service, err := NewProjectService(ctx)
	require.NoError(err)
	defer service.Close()

	project, err := service.CreateProject(CreateProjectRequest{
		Name:     "Static Analysis Test",
		Slug:     "test-static",
		RootPath: tempDir,
	})
	require.NoError(err)

	vs, err := service.GetVectorStore(project.ID)
	require.NoError(err)

	// 1. Setup mock data in DB
	// Persist outline (this also creates/resolves the file)
	callerNode := &models.OutlineNode{
		ID:        "node-caller",
		Name:      "main",
		Kind:      "function",
		StartLine: 1,
		EndLine:   5,
	}
	targetNode := &models.OutlineNode{
		ID:        "node-target",
		Name:      "Calculate",
		Kind:      "function",
		StartLine: 10,
		EndLine:   15,
	}

	err = vs.UpsertFileOutline("test.go", []*models.OutlineNode{callerNode, targetNode})
	assert.NoError(err)

	// Insert a usage
	usage := &models.SymbolUsage{
		CallerNodeID: "node-caller",
		TargetNodeID: "node-target",
		Line:         3,
		Column:       5,
	}
	
	err = vs.InsertSymbolUsage(usage)
	assert.NoError(err)

	// 2. Test FindReferences
	t.Run("FindReferences_ByID", func(t *testing.T) {
		refs, err := service.FindReferences(project.ID, "node-target", "", "")
		assert.NoError(err)
		
		assert.Len(refs.Targets, 1)
		target := refs.Targets[0]
		
		// Header + 1 data row
		assert.Len(target.Results, 2)
		assert.Equal("File", target.Results[0][0])
		
		row := target.Results[1]
		assert.Equal("test.go", row[0])   // File
		assert.Equal(3, row[1])           // Line
		assert.Equal("main", row[2])       // Caller
		assert.Equal("function", row[3])  // Kind
	})

	t.Run("FindReferences_ByName", func(t *testing.T) {
		refs, err := service.FindReferences(project.ID, "", "Calculate", "")
		assert.NoError(err)
		assert.Len(refs.Targets, 1)
		assert.Len(refs.Targets[0].Results, 2)
	})

	// 3. Test GetCallGraph
	t.Run("GetCallGraph_Outgoing", func(t *testing.T) {
		resp, err := service.GetCallGraph(project.ID, "node-caller", "", "", "outgoing", 1)
		require.NoError(err)
		require.NotNil(resp)
		require.Equal("main", resp.Root.Symbol)
		require.Len(resp.Root.Calls, 1)
		require.Equal("Calculate", resp.Root.Calls[0].Symbol)
		require.Equal("test.go:10", resp.Root.Calls[0].Location)
	})

	t.Run("GetCallGraph_Incoming", func(t *testing.T) {
		resp, err := service.GetCallGraph(project.ID, "node-target", "", "", "incoming", 1)
		require.NoError(err)
		require.NotNil(resp)
		require.Equal("Calculate", resp.Root.Symbol)
		require.Len(resp.Root.Calls, 1)
		require.Equal("main", resp.Root.Calls[0].Symbol)
		require.Equal("test.go:1", resp.Root.Calls[0].Location)
	})

}
