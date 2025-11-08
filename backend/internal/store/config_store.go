package store

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"CodeTextor/backend/pkg/utils"
	_ "modernc.org/sqlite"
)

// ConfigStore manages application-wide configuration persisted in projects.db.
type ConfigStore struct {
	db *sql.DB
}

// NewConfigStore opens the global configuration database and ensures schema.
func NewConfigStore() (*ConfigStore, error) {
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	dbPath := filepath.Join(configDir, "projects.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open config database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(`
    CREATE TABLE IF NOT EXISTS app_config (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL
    )
  `); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init config schema: %w", err)
	}

	return &ConfigStore{db: db}, nil
}

// Close closes the configuration database connection.
func (s *ConfigStore) Close() error {
	return s.db.Close()
}

// SetValue inserts or updates the value for a given key.
func (s *ConfigStore) SetValue(key, value string) error {
	_, err := s.db.Exec(`
    INSERT INTO app_config (key, value) VALUES (?, ?)
    ON CONFLICT(key) DO UPDATE SET value = excluded.value
  `, key, value)
	if err != nil {
		return fmt.Errorf("failed to upsert config key %s: %w", key, err)
	}
	return nil
}

// GetValue retrieves the value for a key.
func (s *ConfigStore) GetValue(key string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT value FROM app_config WHERE key = ?`, key)
	var value string
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read config key %s: %w", key, err)
	}
	return value, true, nil
}

// DeleteValue removes a key from the store.
func (s *ConfigStore) DeleteValue(key string) error {
	_, err := s.db.Exec(`DELETE FROM app_config WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("failed to delete config key %s: %w", key, err)
	}
	return nil
}
