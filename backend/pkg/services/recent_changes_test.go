package services

import (
	"CodeTextor/backend/pkg/models"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"CodeTextor/backend/pkg/utils"
	"time"
)

func TestGetRecentChanges_Git(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	// Create a temp project root
	root := t.TempDir()
	
	// Initialize git
	cmd := exec.Command("git", "init")
	utils.SetHideWindow(cmd)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Skip("git not available or failed to init:", err)
	}

	// Create a file
	filePath := filepath.Join(root, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Create the project in CodeTextor
	project, err := service.CreateProject(CreateProjectRequest{
		Name:     "Git Project",
		RootPath: root,
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Test GetRecentChanges
	res, err := service.GetRecentChanges(project.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges failed: %v", err)
	}

	if res.VCSType != "git" {
		t.Errorf("Expected VCSType git, got %s", res.VCSType)
	}

	// Check if test.txt is in working copy (status ?? for untracked)
	found := false
	for _, f := range res.WorkingCopy {
		if f.Path == "test.txt" {
			found = true
			if f.Status != "??" {
				t.Errorf("Expected status ?? for untracked file, got %s", f.Status)
			}
		}
	}
	if !found {
		t.Error("test.txt not found in WorkingCopy")
	}
}

func TestGetRecentChanges_DB(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	project := createProject(t, service, "DB Project")
	
	// Mock indexing by manually inserting a file via VectorStore
	vs, err := service.GetVectorStore(project.ID)
	if err != nil {
		t.Fatalf("failed to get vector store: %v", err)
	}
	
	err = vs.InsertFile(&models.File{
		Path: "indexed.go",
		Hash: "abc",
	})
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	res, err := service.GetRecentChanges(project.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges failed: %v", err)
	}

	if len(res.Indexed) == 0 {
		t.Error("Expected at least one indexed file")
	} else {
		idx := res.Indexed[0]
		if idx.Path != "indexed.go" {
			t.Errorf("Expected indexed.go, got %s", idx.Path)
		}
		// Verify time format
		_, err := time.Parse(time.RFC3339, idx.Time)
		if err != nil {
			t.Errorf("Expected RFC3339 time format, got %s (error: %v)", idx.Time, err)
		}
	}
}
