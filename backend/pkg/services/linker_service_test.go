package services

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestLinkerService_ResolveUsages(t *testing.T) {
	// Setup temp directory for DB
	tmpDir := t.TempDir()
	os.Setenv("CODETEXTOR_INDEXES_DIR", tmpDir)

	projectID := "test-linker"
	vs, err := store.NewVectorStore(projectID, projectID)
	if err != nil {
		t.Fatalf("Failed to create vector store: %v", err)
	}
	defer vs.Close()

	// 1. Create a file
	err = vs.InsertFile(&models.File{
		Path: "main.go",
		ID:   uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Failed to insert file: %v", err)
	}
	
	// Create BOTH nodes in the same outline to avoid deletion
	targetNode := &models.OutlineNode{
		ID:        "target-1",
		Name:      "MyFunction",
		Kind:      "function",
		StartLine: 10,
		EndLine:   12,
	}
	callerNode := &models.OutlineNode{
		ID:        "caller-1",
		Name:      "CallingFunction",
		Kind:      "function",
		StartLine: 20,
		EndLine:   25,
	}

	err = vs.UpsertFileOutline("main.go", []*models.OutlineNode{targetNode, callerNode})
	if err != nil {
		t.Fatalf("Failed to insert outline: %v", err)
	}

	// 2. Create unresolved usages
	// Internal match
	usage1 := &models.SymbolUsage{
		CallerNodeID:  "caller-1",
		RawTargetName: "MyFunction",
		Line:          22,
	}
	if err := vs.InsertSymbolUsage(usage1); err != nil {
		t.Fatalf("Failed to insert usage 1: %v", err)
	}

	// External match (Virtual)
	usage2 := &models.SymbolUsage{
		CallerNodeID:     "caller-1",
		RawTargetName:    "Println",
		RawTargetContext: "fmt",
		Line:             23,
	}
	if err := vs.InsertSymbolUsage(usage2); err != nil {
		t.Fatalf("Failed to insert usage 2: %v", err)
	}

	// 3. Run Linker
	linker := NewLinkerService()
	if err := linker.ResolveUsages(projectID, vs); err != nil {
		t.Fatalf("Linker failed: %v", err)
	}

	// 4. Verify results
	// Check usage 1 (Internal)
	resolvedUsages, err := vs.GetSymbolUsages("target-1")
	if err != nil {
		t.Fatalf("Failed to get resolved usages for target-1: %v", err)
	}
	foundInternal := false
	for _, res := range resolvedUsages {
		if res.ID == usage1.ID {
			foundInternal = true
			break
		}
	}
	if !foundInternal {
		t.Errorf("Usage 1 was not resolved to target-1 (found %d resolved usages)", len(resolvedUsages))
	}

	// Check usage 2 (Virtual)
	virtualID := utils.GenerateSymbolID("@external/fmt", 0, 0, "Println", 1)
	virtualUsages, err := vs.GetSymbolUsages(virtualID)
	if err != nil {
		t.Fatalf("Failed to get resolved usages for virtual ID: %v", err)
	}
	if len(virtualUsages) == 0 {
		t.Errorf("Usage 2 was not resolved to a virtual symbol %s", virtualID)
	}
}
