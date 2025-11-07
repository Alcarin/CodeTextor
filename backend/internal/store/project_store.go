/*
  File: project_store.go
  Purpose: SQLite-based persistent storage for CodeTextor project configurations.
  Author: CodeTextor project
  Notes: This file manages the projects.db database containing all project metadata.
*/

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"

	_ "modernc.org/sqlite"
)

// ProjectStore manages persistent storage of project configurations.
// It uses a single SQLite database to store all projects' metadata and settings.
type ProjectStore struct {
	db *sql.DB
}

// NewProjectStore creates a new ProjectStore instance and initializes the database.
// The database file is located at <ConfigDir>/projects.db
// Returns an error if the database cannot be opened or initialized.
func NewProjectStore() (*ProjectStore, error) {
	// Get the database path
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	dbPath := configDir + "/projects.db"

	return NewProjectStoreWithPath(dbPath)
}

// NewProjectStoreWithPath creates a new ProjectStore with a custom database path.
// This is useful for testing with temporary databases.
// Parameters:
//   - dbPath: full path to the SQLite database file
//
// Returns an error if the database cannot be opened or initialized.
func NewProjectStoreWithPath(dbPath string) (*ProjectStore, error) {
	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &ProjectStore{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema runs all database migrations to ensure schema is up to date.
// Uses the migration system defined in migrations.go
func (s *ProjectStore) initSchema() error {
	return s.runMigrations()
}

// Create inserts a new project into the database.
// Returns an error if the project ID already exists or validation fails.
func (s *ProjectStore) Create(project *models.Project) error {
	// Validate project
	if err := project.Validate(); err != nil {
		return err
	}

	// Serialize config to JSON
	configJSON, err := json.Marshal(project.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert into database (is_selected defaults to 0)
	query := `
		INSERT INTO projects (id, name, description, created_at, updated_at, config_json, is_selected)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`

	_, err = s.db.Exec(
		query,
		project.ID,
		project.Name,
		project.Description,
		project.CreatedAt.Unix(),
		project.UpdatedAt.Unix(),
		string(configJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert project: %w", err)
	}

	return nil
}

// Get retrieves a project by its ID.
// Returns nil if the project doesn't exist.
func (s *ProjectStore) Get(id string) (*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, config_json
		FROM projects
		WHERE id = ?
	`

	var project models.Project
	var createdAtUnix, updatedAtUnix int64
	var configJSON string

	err := s.db.QueryRow(query, id).Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&createdAtUnix,
		&updatedAtUnix,
		&configJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query project: %w", err)
	}

	// Parse timestamps
	project.CreatedAt = time.Unix(createdAtUnix, 0)
	project.UpdatedAt = time.Unix(updatedAtUnix, 0)

	// Deserialize config
	if err := json.Unmarshal([]byte(configJSON), &project.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &project, nil
}

// List returns all projects in the database.
// Projects are ordered by creation time (newest first).
// Returns an empty slice (not nil) if no projects exist.
func (s *ProjectStore) List() ([]*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, config_json
		FROM projects
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	// Initialize with empty slice instead of nil to ensure JSON serialization returns [] not null
	projects := make([]*models.Project, 0)

	for rows.Next() {
		var project models.Project
		var createdAtUnix, updatedAtUnix int64
		var configJSON string

		err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&createdAtUnix,
			&updatedAtUnix,
			&configJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}

		// Parse timestamps
		project.CreatedAt = time.Unix(createdAtUnix, 0)
		project.UpdatedAt = time.Unix(updatedAtUnix, 0)

		// Deserialize config
		if err := json.Unmarshal([]byte(configJSON), &project.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		projects = append(projects, &project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}

// Update modifies an existing project in the database.
// The updated_at timestamp is automatically set to the current time.
// Returns an error if the project doesn't exist.
func (s *ProjectStore) Update(project *models.Project) error {
	// Validate project
	if err := project.Validate(); err != nil {
		return err
	}

	// Update timestamp
	project.UpdatedAt = time.Now()

	// Serialize config to JSON
	configJSON, err := json.Marshal(project.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Update in database
	query := `
		UPDATE projects
		SET name = ?, description = ?, updated_at = ?, config_json = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(
		query,
		project.Name,
		project.Description,
		project.UpdatedAt.Unix(),
		string(configJSON),
		project.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	// Check if project was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("project not found: %s", project.ID)
	}

	return nil
}

// Delete removes a project from the database.
// Returns an error if the project doesn't exist.
// Note: This does NOT delete the project's index database file.
func (s *ProjectStore) Delete(id string) error {
	query := `DELETE FROM projects WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	// Check if project was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("project not found: %s", id)
	}

	return nil
}

// Exists checks if a project with the given ID exists in the database.
func (s *ProjectStore) Exists(id string) (bool, error) {
	query := `SELECT 1 FROM projects WHERE id = ? LIMIT 1`

	var exists int
	err := s.db.QueryRow(query, id).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return true, nil
}

// SetSelected marks a project as selected and unmarks all others.
// Only one project can be selected at a time.
func (s *ProjectStore) SetSelected(id string) error {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// First, unmark all projects
	_, err = tx.Exec(`UPDATE projects SET is_selected = 0`)
	if err != nil {
		return fmt.Errorf("failed to unmark projects: %w", err)
	}

	// Then, mark the selected project
	result, err := tx.Exec(`UPDATE projects SET is_selected = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to mark project as selected: %w", err)
	}

	// Check if project was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("project not found: %s", id)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetSelected returns the currently selected project.
// Returns nil if no project is selected.
// If multiple projects are marked as selected (shouldn't happen), returns the oldest one.
func (s *ProjectStore) GetSelected() (*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, config_json
		FROM projects
		WHERE is_selected = 1
		ORDER BY created_at ASC
		LIMIT 1
	`

	var project models.Project
	var createdAtUnix, updatedAtUnix int64
	var configJSON string

	err := s.db.QueryRow(query).Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&createdAtUnix,
		&updatedAtUnix,
		&configJSON,
	)

	if err == sql.ErrNoRows {
		// No project selected, try to auto-select the oldest one
		return s.autoSelectOldest()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query selected project: %w", err)
	}

	// Parse timestamps
	project.CreatedAt = time.Unix(createdAtUnix, 0)
	project.UpdatedAt = time.Unix(updatedAtUnix, 0)

	// Deserialize config
	if err := json.Unmarshal([]byte(configJSON), &project.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &project, nil
}

// autoSelectOldest automatically selects the oldest project if none is selected.
// Returns nil if no projects exist.
func (s *ProjectStore) autoSelectOldest() (*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at, config_json
		FROM projects
		ORDER BY created_at ASC
		LIMIT 1
	`

	var project models.Project
	var createdAtUnix, updatedAtUnix int64
	var configJSON string

	err := s.db.QueryRow(query).Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&createdAtUnix,
		&updatedAtUnix,
		&configJSON,
	)

	if err == sql.ErrNoRows {
		// No projects exist
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query oldest project: %w", err)
	}

	// Parse timestamps
	project.CreatedAt = time.Unix(createdAtUnix, 0)
	project.UpdatedAt = time.Unix(updatedAtUnix, 0)

	// Deserialize config
	if err := json.Unmarshal([]byte(configJSON), &project.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Mark it as selected
	if err := s.SetSelected(project.ID); err != nil {
		return nil, fmt.Errorf("failed to auto-select project: %w", err)
	}

	return &project, nil
}

// ClearSelection unmarks all projects as selected.
func (s *ProjectStore) ClearSelection() error {
	_, err := s.db.Exec(`UPDATE projects SET is_selected = 0`)
	if err != nil {
		return fmt.Errorf("failed to clear selection: %w", err)
	}
	return nil
}

// Close closes the database connection.
// Should be called when the store is no longer needed.
func (s *ProjectStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
