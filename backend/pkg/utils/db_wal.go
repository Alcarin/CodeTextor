/*
  File: db_wal.go
  Purpose: Utility to enable WAL mode on existing SQLite databases
  Author: CodeTextor project
  Notes: WAL (Write-Ahead Logging) mode improves concurrent access performance
*/

package utils

import (
	"database/sql"
	"fmt"
)

// EnableWALMode enables Write-Ahead Logging on a SQLite database.
// This improves concurrent read/write performance and reduces SQLITE_BUSY errors.
// Should be called once when opening a database connection.
func EnableWALMode(db *sql.DB) error {
	// Set journal mode to WAL
	_, err := db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout to 5 seconds
	_, err = db.Exec("PRAGMA busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	return nil
}
