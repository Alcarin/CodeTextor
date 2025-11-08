package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"CodeTextor/backend/pkg/models"
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbConnectionSuffix = "?_journal_mode=WAL&_busy_timeout=5000"
	projectDBPattern   = "project-*.db"
)

// ListProjectDBPaths returns the full paths of all project databases under indexesDir.
func ListProjectDBPaths(indexesDir string) ([]string, error) {
	entries, err := os.ReadDir(indexesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read indexes directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "project-") || !strings.HasSuffix(name, ".db") {
			continue
		}
		paths = append(paths, filepath.Join(indexesDir, name))
	}

	return paths, nil
}

// LoadProjectMetadata reads the project metadata row from the given vector database.
func LoadProjectMetadata(dbPath string) (*models.Project, error) {
	db, err := openProjectDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	projectID, ok := projectIDFromFileName(filepath.Base(dbPath))
	if !ok {
		return nil, fmt.Errorf("invalid project database name: %s", filepath.Base(dbPath))
	}

	var (
		name        string
		description string
		configJSON  string
		isIndexing  int
		createdAt   int64
		updatedAt   int64
	)
	row := db.QueryRow(`
		SELECT id, name, description, config_json, is_indexing, created_at, updated_at
		FROM project_meta WHERE id = ?
	`, projectID)

	var id string
	if err := row.Scan(&id, &name, &description, &configJSON, &isIndexing, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project metadata not found for %s", projectID)
		}
		return nil, fmt.Errorf("failed to scan project metadata: %w", err)
	}

	var config models.ProjectConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config for %s: %w", id, err)
	}

	project := &models.Project{
		ID:          id,
		Name:        name,
		Description: description,
		Config:      config,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		IsIndexing:  isIndexing == 1,
	}

	return project, nil
}

// SaveProjectMetadata writes or updates the project metadata row in the database.
func SaveProjectMetadata(dbPath string, project *models.Project) error {
	db, err := openProjectDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	return saveProjectMetadataWithDB(db, project)
}

// saveProjectMetadataWithDB uses the provided *sql.DB connection to persist metadata.
func saveProjectMetadataWithDB(db *sql.DB, project *models.Project) error {
	if project == nil {
		return fmt.Errorf("project cannot be nil")
	}
	configBytes, err := json.Marshal(project.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO project_meta (id, name, description, config_json, is_indexing, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			config_json = excluded.config_json,
			is_indexing = excluded.is_indexing,
			created_at = project_meta.created_at,
			updated_at = excluded.updated_at
	`, project.ID, project.Name, project.Description, string(configBytes), boolToInt(project.IsIndexing), project.CreatedAt, project.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert project metadata: %w", err)
	}
	return nil
}

func openProjectDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath+dbConnectionSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to open project database %s: %w", dbPath, err)
	}
	return db, nil
}

func projectIDFromFileName(fileName string) (string, bool) {
	if !strings.HasPrefix(fileName, "project-") || !strings.HasSuffix(fileName, ".db") {
		return "", false
	}
	id := strings.TrimPrefix(fileName, "project-")
	id = strings.TrimSuffix(id, ".db")
	return id, id != ""
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
