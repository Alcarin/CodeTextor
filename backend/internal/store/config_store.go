package store

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init config schema: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS embedding_models (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			backend TEXT NOT NULL DEFAULT 'onnx',
			description TEXT,
			dimension INTEGER NOT NULL,
			disk_size_bytes INTEGER,
			ram_requirement_bytes INTEGER,
			cpu_latency_ms INTEGER,
			multilingual INTEGER NOT NULL DEFAULT 0,
			code_quality TEXT,
			notes TEXT,
			source_type TEXT NOT NULL,
			source_uri TEXT,
			local_path TEXT,
			license TEXT,
			download_status TEXT NOT NULL DEFAULT 'unknown',
			requires_conversion INTEGER NOT NULL DEFAULT 0,
			preferred_filename TEXT,
			code_focus TEXT,
			estimated_tokens_per_second INTEGER,
			supports_quantization INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init embedding model schema: %w", err)
	}

	if err := ensureEmbeddingModelColumns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate embedding model schema: %w", err)
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

// ListEmbeddingModels returns all catalog entries ordered by display name.
func (s *ConfigStore) ListEmbeddingModels() ([]*models.EmbeddingModelInfo, error) {
	rows, err := s.db.Query(`
		SELECT id, display_name, backend, description, dimension, disk_size_bytes,
			ram_requirement_bytes, cpu_latency_ms, multilingual, code_quality,
			notes, source_type, source_uri, local_path, license, download_status,
			requires_conversion, preferred_filename, code_focus,
			estimated_tokens_per_second, supports_quantization,
			tokenizer_uri, tokenizer_local_path, max_sequence_length,
			created_at, updated_at
		FROM embedding_models
		ORDER BY display_name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list embedding models: %w", err)
	}
	defer rows.Close()

	var modelsList []*models.EmbeddingModelInfo
	for rows.Next() {
		var multilingualInt, requiresConvInt, supportsQuantInt int
		meta := &models.EmbeddingModelInfo{}
		if err := rows.Scan(
			&meta.ID,
			&meta.DisplayName,
			&meta.Backend,
			&meta.Description,
			&meta.Dimension,
			&meta.DiskSizeBytes,
			&meta.RAMRequirementBytes,
			&meta.CPULatencyMs,
			&multilingualInt,
			&meta.CodeQuality,
			&meta.Notes,
			&meta.SourceType,
			&meta.SourceURI,
			&meta.LocalPath,
			&meta.License,
			&meta.DownloadStatus,
			&requiresConvInt,
			&meta.PreferredFilename,
			&meta.CodeFocus,
			&meta.EstimatedTokensPerS,
			&supportsQuantInt,
			&meta.TokenizerURI,
			&meta.TokenizerLocalPath,
			&meta.MaxSequenceLength,
			&meta.CreatedAt,
			&meta.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding model: %w", err)
		}
		meta.IsMultilingual = multilingualInt == 1
		meta.RequiresConversion = requiresConvInt == 1
		meta.SupportsQuantization = supportsQuantInt == 1
		modelsList = append(modelsList, meta)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate embedding models: %w", err)
	}

	return modelsList, nil
}

// GetEmbeddingModel fetches a single model by id.
func (s *ConfigStore) GetEmbeddingModel(id string) (*models.EmbeddingModelInfo, error) {
	row := s.db.QueryRow(`
		SELECT id, display_name, backend, description, dimension, disk_size_bytes,
			ram_requirement_bytes, cpu_latency_ms, multilingual, code_quality,
			notes, source_type, source_uri, local_path, license, download_status,
			requires_conversion, preferred_filename, code_focus,
			estimated_tokens_per_second, supports_quantization,
			tokenizer_uri, tokenizer_local_path, max_sequence_length,
			created_at, updated_at
		FROM embedding_models WHERE id = ?
	`, id)

	var multilingualInt, requiresConvInt, supportsQuantInt int
	meta := &models.EmbeddingModelInfo{}
	if err := row.Scan(
		&meta.ID,
		&meta.DisplayName,
		&meta.Backend,
		&meta.Description,
		&meta.Dimension,
		&meta.DiskSizeBytes,
		&meta.RAMRequirementBytes,
		&meta.CPULatencyMs,
		&multilingualInt,
		&meta.CodeQuality,
		&meta.Notes,
		&meta.SourceType,
		&meta.SourceURI,
		&meta.LocalPath,
		&meta.License,
		&meta.DownloadStatus,
		&requiresConvInt,
		&meta.PreferredFilename,
		&meta.CodeFocus,
		&meta.EstimatedTokensPerS,
		&supportsQuantInt,
		&meta.TokenizerURI,
		&meta.TokenizerLocalPath,
		&meta.MaxSequenceLength,
		&meta.CreatedAt,
		&meta.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("embedding model not found: %s", id)
		}
		return nil, fmt.Errorf("failed to load embedding model %s: %w", id, err)
	}
	meta.IsMultilingual = multilingualInt == 1
	meta.RequiresConversion = requiresConvInt == 1
	meta.SupportsQuantization = supportsQuantInt == 1
	return meta, nil
}

// UpsertEmbeddingModel inserts or updates a catalog entry.
func (s *ConfigStore) UpsertEmbeddingModel(meta *models.EmbeddingModelInfo) error {
	if meta == nil {
		return fmt.Errorf("embedding model cannot be nil")
	}
	if meta.ID == "" {
		return fmt.Errorf("embedding model id cannot be empty")
	}
	if meta.DisplayName == "" {
		meta.DisplayName = meta.ID
	}
	if meta.Backend == "" {
		meta.Backend = "onnx"
	}
	now := time.Now().Unix()
	if meta.CreatedAt == 0 {
		meta.CreatedAt = now
	}
	meta.UpdatedAt = now

	_, err := s.db.Exec(`
		INSERT INTO embedding_models (
			id, display_name, backend, description, dimension, disk_size_bytes,
			ram_requirement_bytes, cpu_latency_ms, multilingual, code_quality,
			notes, source_type, source_uri, local_path, license, download_status,
			requires_conversion, preferred_filename, code_focus,
			estimated_tokens_per_second, supports_quantization,
			tokenizer_uri, tokenizer_local_path, max_sequence_length,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			display_name = excluded.display_name,
			backend = excluded.backend,
			description = excluded.description,
			dimension = excluded.dimension,
			disk_size_bytes = excluded.disk_size_bytes,
			ram_requirement_bytes = excluded.ram_requirement_bytes,
			cpu_latency_ms = excluded.cpu_latency_ms,
			multilingual = excluded.multilingual,
			code_quality = excluded.code_quality,
			notes = excluded.notes,
			source_type = excluded.source_type,
			source_uri = excluded.source_uri,
			local_path = excluded.local_path,
			license = excluded.license,
			download_status = excluded.download_status,
			requires_conversion = excluded.requires_conversion,
			preferred_filename = excluded.preferred_filename,
			code_focus = excluded.code_focus,
			estimated_tokens_per_second = excluded.estimated_tokens_per_second,
			supports_quantization = excluded.supports_quantization,
			tokenizer_uri = excluded.tokenizer_uri,
			tokenizer_local_path = excluded.tokenizer_local_path,
			max_sequence_length = excluded.max_sequence_length,
			updated_at = excluded.updated_at
	`, meta.ID,
		meta.DisplayName,
		meta.Backend,
		meta.Description,
		meta.Dimension,
		meta.DiskSizeBytes,
		meta.RAMRequirementBytes,
		meta.CPULatencyMs,
		boolToInt(meta.IsMultilingual),
		meta.CodeQuality,
		meta.Notes,
		meta.SourceType,
		meta.SourceURI,
		meta.LocalPath,
		meta.License,
		meta.DownloadStatus,
		boolToInt(meta.RequiresConversion),
		meta.PreferredFilename,
		meta.CodeFocus,
		meta.EstimatedTokensPerS,
		boolToInt(meta.SupportsQuantization),
		meta.TokenizerURI,
		meta.TokenizerLocalPath,
		meta.MaxSequenceLength,
		meta.CreatedAt,
		meta.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert embedding model %s: %w", meta.ID, err)
	}
	return nil
}

func ensureEmbeddingModelColumns(db *sql.DB) error {
	addColumn := func(column, decl string) error {
		stmt := fmt.Sprintf("ALTER TABLE embedding_models ADD COLUMN %s %s", column, decl)
		if _, err := db.Exec(stmt); err != nil {
			lower := strings.ToLower(err.Error())
			if !strings.Contains(lower, "duplicate column name") {
				return err
			}
		}
		return nil
	}

	if err := addColumn("tokenizer_uri", "TEXT"); err != nil {
		return err
	}
	if err := addColumn("tokenizer_local_path", "TEXT"); err != nil {
		return err
	}
	if err := addColumn("max_sequence_length", "INTEGER"); err != nil {
		return err
	}
	if err := addColumn("backend", "TEXT DEFAULT 'onnx'"); err != nil {
		return err
	}
	return nil
}
