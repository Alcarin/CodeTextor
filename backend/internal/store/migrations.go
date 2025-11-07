/*
  File: migrations.go
  Purpose: Database schema migration system using golang-migrate.
  Author: CodeTextor project
  Notes: Uses golang-migrate/migrate for robust schema management.
*/

package store

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations executes all pending database migrations using golang-migrate
func (s *ProjectStore) runMigrations() error {
	// Create source driver from embedded filesystem
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver with NoTxWrap to prevent connection closing
	dbDriver, err := sqlite.WithInstance(s.db, &sqlite.Config{
		NoTxWrap: true, // Don't wrap in transaction, we manage that in migrations
	})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"sqlite",
		dbDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Don't close the migrate instance - it would close our DB connection
	// The source driver will be closed when the function exits

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
