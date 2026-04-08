package store

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"database/sql"
	"embed"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed vector_migrations/*.sql
var vectorMigrationsFS embed.FS

// VectorStore manages the project-specific index database (chunks, files, symbols).
type VectorStore struct {
	db        *sql.DB
	projectID string
	dbPath    string
	fileIDMu  sync.RWMutex
	fileIDs   map[string]int64
}

// ResetCache clears all in-memory file ID mappings.
func (s *VectorStore) ResetCache() {
	s.fileIDMu.Lock()
	defer s.fileIDMu.Unlock()
	s.fileIDs = make(map[string]int64)
}

func (s *VectorStore) getCachedFileID(path string) (int64, bool) {
	s.fileIDMu.RLock()
	defer s.fileIDMu.RUnlock()
	if s.fileIDs == nil {
		return 0, false
	}
	id, ok := s.fileIDs[path]
	return id, ok
}

func (s *VectorStore) cacheFileID(path string, id int64) {
	s.fileIDMu.Lock()
	defer s.fileIDMu.Unlock()
	if s.fileIDs == nil {
		s.fileIDs = make(map[string]int64)
	}
	s.fileIDs[path] = id
}

// NewVectorStore creates a new VectorStore instance for a given project.
// It initializes the SQLite database and runs migrations if necessary.
func NewVectorStore(projectID, projectSlug string) (*VectorStore, error) {
	projectIndexDir := os.Getenv("CODETEXTOR_INDEXES_DIR")
	if projectIndexDir == "" {
		dataDir, err := utils.GetAppDataDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get data directory: %w", err)
		}
		projectIndexDir = filepath.Join(dataDir, "indexes")
	}

	if err := os.MkdirAll(projectIndexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project index directory: %w", err)
	}

	dbPath := filepath.Join(projectIndexDir, fmt.Sprintf("project-%s.db", projectSlug))

	// Open with WAL mode for better concurrent access and busy timeout.
	// We also enable foreign keys to support ON DELETE CASCADE.
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open vector database at %s: %w", dbPath, err)
	}

	// Set connection pool parameters to allow concurrent readers.
	// SQLite in WAL mode allows multiple readers alongside a single writer.
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)

	// Run migrations for the vector database schema
	if err := runVectorMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run vector database migrations: %w", err)
	}

	return &VectorStore{
		db:        db,
		projectID: projectID,
		dbPath:    dbPath,
		fileIDs:   make(map[string]int64),
	}, nil
}

// runVectorMigrations runs the embedded migrations for the per-project vector database
func runVectorMigrations(db *sql.DB) error {
	sourceDriver, err := iofs.New(vectorMigrationsFS, "vector_migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source driver: %w", err)
	}

	dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// RunVectorMigrations applies the embedded vector migrations to the database at dbPath.
func RunVectorMigrations(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("failed to open vector database for migrations: %w", err)
	}
	defer db.Close()

	return runVectorMigrations(db)
}

func normalizeOutlinePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}
	cleaned := filepath.Clean(trimmed)
	return filepath.ToSlash(cleaned), nil
}

func (s *VectorStore) resolveFileID(path string, create bool) (int64, string, error) {
	return s.resolveFileIDTx(nil, path, create)
}

func (s *VectorStore) resolveFileIDTx(tx *sql.Tx, path string, create bool) (int64, string, error) {
	normalized, err := normalizeOutlinePath(path)
	if err != nil {
		return 0, "", err
	}

	if cached, ok := s.getCachedFileID(normalized); ok {
		return cached, normalized, nil
	}

	var row *sql.Row
	if tx != nil {
		row = tx.QueryRow(`SELECT pk FROM files WHERE path = ?`, normalized)
	} else {
		row = s.db.QueryRow(`SELECT pk FROM files WHERE path = ?`, normalized)
	}

	var fileID int64
	if err := row.Scan(&fileID); err != nil {
		if err == sql.ErrNoRows {
			if !create {
				return 0, "", fmt.Errorf("file not found: %s", normalized)
			}
			if tx != nil {
				if fileID, err = s.createPlaceholderFileTx(tx, normalized, false); err != nil {
					return 0, "", err
				}
			} else {
				if fileID, err = s.createPlaceholderFile(normalized, false); err != nil {
					return 0, "", err
				}
			}
		} else {
			return 0, "", fmt.Errorf("failed to resolve file id for %s: %w", normalized, err)
		}
	}

	s.cacheFileID(normalized, fileID)
	return fileID, normalized, nil
}

func (s *VectorStore) createPlaceholderFile(path string, isVirtual bool) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to start placeholder transaction: %w", err)
	}
	defer tx.Rollback()

	fileID, err := s.createPlaceholderFileTx(tx, path, isVirtual)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit placeholder transaction: %w", err)
	}

	return fileID, nil
}

func (s *VectorStore) createPlaceholderFileTx(tx *sql.Tx, path string, isVirtual bool) (int64, error) {
	now := time.Now().Unix()
	
	_, err := tx.Exec(`
		INSERT INTO files (id, path, hash, is_virtual, last_modified, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			is_virtual = CASE WHEN is_virtual = 1 THEN 1 ELSE excluded.is_virtual END,
			updated_at = ?
	`, uuid.New().String(), path, "unknown", isVirtual, 0, 0, now, now, now)
	
	if err != nil {
		return 0, fmt.Errorf("failed to upsert placeholder for %s (virtual=%v): %w", path, isVirtual, err)
	}

	var fileID int64
	err = tx.QueryRow(`SELECT pk FROM files WHERE path = ?`, path).Scan(&fileID)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve pk for placeholder %s: %w", path, err)
	}

	return fileID, nil
}

// SaveProjectMetadata persists the project metadata using this vector database.
func (s *VectorStore) SaveProjectMetadata(project *models.Project) error {
	return saveProjectMetadataWithDB(s.db, project)
}

// Close closes the database connection.
func (s *VectorStore) Close() error {
	return s.db.Close()
}

// InsertChunk inserts a new chunk into the database with semantic metadata.
// If a chunk with the same file and line range already exists, it will be replaced.
func (s *VectorStore) InsertChunk(chunk *models.Chunk) error {
	if chunk.ID == "" {
		chunk.ID = uuid.New().String()
	}
	chunk.CreatedAt = time.Now().Unix()
	chunk.UpdatedAt = time.Now().Unix()
	if strings.TrimSpace(chunk.EmbeddingModelID) == "" {
		chunk.EmbeddingModelID = "unknown"
	}

	fileID, normalizedPath, err := s.resolveFileID(chunk.FilePath, true)
	if err != nil {
		return err
	}
	chunk.FilePath = normalizedPath

	stmt, err := s.db.Prepare(`
		INSERT OR REPLACE INTO chunks (
			id, file_id, content, embedding, embedding_model_id,
			line_start, line_end, char_start, char_end,
			language, symbol_name, symbol_kind, parent,
			signature, visibility, package_name, doc_string,
			token_count, is_collapsed, source_code,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert chunk statement: %w", err)
	}
	defer stmt.Close()

	// Convert []float32 to []byte for storage
	embeddingBytes, err := float32SliceToByteSlice(chunk.Embedding)
	if err != nil {
		return fmt.Errorf("failed to convert embedding to bytes: %w", err)
	}

	_, err = stmt.Exec(
		chunk.ID,
		fileID,
		chunk.Content,
		embeddingBytes,
		chunk.EmbeddingModelID,
		chunk.LineStart,
		chunk.LineEnd,
		chunk.CharStart,
		chunk.CharEnd,
		chunk.Language,
		chunk.SymbolName,
		chunk.SymbolKind,
		chunk.Parent,
		chunk.Signature,
		chunk.Visibility,
		chunk.PackageName,
		chunk.DocString,
		chunk.TokenCount,
		chunk.IsCollapsed,
		chunk.SourceCode,
		chunk.CreatedAt,
		chunk.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}

	return nil
}

// InsertFile inserts a new file record into the database.
// If a file with the same path already exists, it will be replaced.
func (s *VectorStore) InsertFile(file *models.File) error {
	file.ID = uuid.New().String()
	file.CreatedAt = time.Now().Unix()
	file.UpdatedAt = time.Now().Unix()

	normalizedPath, err := normalizeOutlinePath(file.Path)
	if err != nil {
		return err
	}
	file.Path = normalizedPath

	stmt, err := s.db.Prepare(`
		INSERT INTO files (id, path, hash, is_virtual, last_modified, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			id = excluded.id,
			hash = excluded.hash,
			is_virtual = excluded.is_virtual,
			last_modified = excluded.last_modified,
			chunk_count = excluded.chunk_count,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert file statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		file.ID,
		file.Path,
		file.Hash,
		file.IsVirtual,
		file.LastModified,
		file.ChunkCount,
		file.CreatedAt,
		file.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	if _, _, err := s.resolveFileID(file.Path, false); err != nil {
		return err
	}

	return nil
}

// GetFile retrieves file metadata by path.
// Returns nil if the file is not found in the database.
func (s *VectorStore) GetFile(path string) (*models.File, error) {
	normalizedPath, err := normalizeOutlinePath(path)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRow(`
		SELECT id, path, hash, is_virtual, last_modified, chunk_count, created_at, updated_at
		FROM files
		WHERE path = ?
	`, normalizedPath)

	file := &models.File{}
	err = row.Scan(
		&file.ID,
		&file.Path,
		&file.Hash,
		&file.IsVirtual,
		&file.LastModified,
		&file.ChunkCount,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // File not found
		}
		return nil, fmt.Errorf("failed to get file %s: %w", normalizedPath, err)
	}

	if _, _, err := s.resolveFileID(file.Path, false); err != nil {
		return nil, err
	}

	return file, nil
}

// InsertSymbol inserts a new symbol record into the database.
func (s *VectorStore) InsertSymbol(symbol *models.Symbol) error {
	if symbol.ID == "" {
		symbol.ID = uuid.New().String()
	}
	symbol.CreatedAt = time.Now().Unix()
	symbol.UpdatedAt = time.Now().Unix()

	fileID, normalizedPath, err := s.resolveFileID(symbol.FilePath, true)
	if err != nil {
		return err
	}
	symbol.FilePath = normalizedPath

	stmt, err := s.db.Prepare(`
		INSERT OR REPLACE INTO symbols (id, file_id, name, kind, line, character, parent, language, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert symbol statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		symbol.ID,
		fileID,
		symbol.Name,
		symbol.Kind,
		symbol.Line,
		symbol.Character,
		sql.NullString{String: symbol.Parent, Valid: symbol.Parent != ""},
		symbol.Language,
		symbol.CreatedAt,
		symbol.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert symbol: %w", err)
	}

	if len(symbol.Implements) > 0 {
		implStmt, err := s.db.Prepare(`
			INSERT INTO symbol_implementations (id, interface_name, implementor_id, file_id)
			VALUES (?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare insert symbol implementation statement: %w", err)
		}
		defer implStmt.Close()

		for _, impl := range symbol.Implements {
			_, err = implStmt.Exec(uuid.New().String(), impl, symbol.ID, fileID)
			if err != nil {
				return fmt.Errorf("failed to insert symbol implementation: %w", err)
			}
		}
	}

	return nil
}

// DeleteFileSymbols removes all symbols for a given file path.
func (s *VectorStore) DeleteFileSymbols(filePath string) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, true)
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete symbols for file %s: %w", normalizedPath, err)
	}
	return nil
}

// UpsertFileOutline saves the outline tree for a file.
func (s *VectorStore) UpsertFileOutline(filePath string, outline []*models.OutlineNode) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, true)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start outline transaction for %s: %w", normalizedPath, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM outline_nodes WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to clear outline nodes for %s: %w", normalizedPath, err)
	}

	if len(outline) > 0 {
		if err := s.insertOutlineNodes(tx, fileID, outline, sql.NullString{}); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO outline_metadata (file_id, updated_at)
		VALUES (?, ?)
		ON CONFLICT(file_id) DO UPDATE SET updated_at = excluded.updated_at
	`, fileID, time.Now().Unix()); err != nil {
		return fmt.Errorf("failed to update outline metadata for %s: %w", normalizedPath, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit outline for %s: %w", normalizedPath, err)
	}
	return nil
}

// InsertOutlineNodes persists a forest of outline nodes for a file.
func (s *VectorStore) InsertOutlineNodes(fileID int64, nodes []*models.OutlineNode) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.insertOutlineNodes(tx, fileID, nodes, sql.NullString{}); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *VectorStore) insertOutlineNodes(tx *sql.Tx, fileID int64, nodes []*models.OutlineNode, parent sql.NullString) error {
	for idx, node := range nodes {
		nodeID := node.ID
		if strings.TrimSpace(nodeID) == "" {
			nodeID = uuid.New().String()
		}
		if _, err := tx.Exec(`
			INSERT INTO outline_nodes (
				id, file_id, parent_id, name, kind, start_line, end_line, position
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, nodeID, fileID, parent, node.Name, node.Kind, node.StartLine, node.EndLine, idx); err != nil {
			return fmt.Errorf("failed to insert outline node %s: %w", nodeID, err)
		}

		if len(node.Children) > 0 {
			nextParent := sql.NullString{String: nodeID, Valid: true}
			if err := s.insertOutlineNodes(tx, fileID, node.Children, nextParent); err != nil {
				return err
			}
		}
	}
	return nil
}

// InsertFileTasksInTransaction performs a high-speed atomic update for a single file and all its artifacts.
// It combines file metadata, semantic chunks, symbols, and structural outline in a single transaction.
// InsertFileTasksInTransaction persists all artifacts for a file in a single transaction.
// Returns true if a new file record was created, false if an existing one was updated.
func (s *VectorStore) InsertFileTasksInTransaction(
	file *models.File,
	chunks []*models.Chunk,
	symbols []*models.Symbol,
	outline []*models.OutlineNode,
	usages []*models.SymbolUsage,
) (bool, error) {
	if file == nil {
		return false, fmt.Errorf("file record cannot be nil")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin atomic file transaction for %s: %w", file.Path, err)
	}
	defer tx.Rollback()

	// Check if file already exists to determine if it's a new entry
	var existingPK int64
	err = tx.QueryRow(`SELECT pk FROM files WHERE path = ?`, file.Path).Scan(&existingPK)
	isNew := err == sql.ErrNoRows

	now := time.Now().Unix()
	file.CreatedAt = now
	file.UpdatedAt = now

	// 1. Upsert File and get the internal integer PK
	// We use ON CONFLICT(path) as the primary way to identify existing files.
	// Since id is also unique, we update it in the SET clause.
	_, err = tx.Exec(`
		INSERT INTO files (id, path, hash, is_virtual, last_modified, chunk_count, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET 
			id = excluded.id,
			hash = excluded.hash,
			is_virtual = excluded.is_virtual,
			last_modified = excluded.last_modified,
			chunk_count = excluded.chunk_count,
			updated_at = excluded.updated_at
	`, file.ID, file.Path, file.Hash, file.IsVirtual, file.LastModified, file.ChunkCount, file.CreatedAt, file.UpdatedAt)
	if err != nil {
		return false, fmt.Errorf("failed to upsert file %s: %w", file.Path, err)
	}

	var fileID int64
	if err := tx.QueryRow(`SELECT pk FROM files WHERE path = ?`, file.Path).Scan(&fileID); err != nil {
		return false, fmt.Errorf("failed to retrieve pk for %s: %w", file.Path, err)
	}
	s.cacheFileID(file.Path, fileID)

	// 2. Clear old file artifacts.
	// Since all tables (chunks, symbols, outline_nodes) reference files(pk) with ON DELETE CASCADE,
	// we just need to ensure the foreign keys are working. 
	// However, to avoid any issues with existing data, we explicitly delete associated records for this file.
	if _, err := tx.Exec(`DELETE FROM chunks WHERE file_id = ?`, fileID); err != nil {
		return false, fmt.Errorf("failed to clear old chunks for %s: %w", file.Path, err)
	}
	// Note: symbols, implementations, and outline_nodes are cleared by CASCADE or explicit DELETE if needed.
	// We delete symbols explicitly just to be 100% sure the transaction sees it immediately.
	if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return false, fmt.Errorf("failed to clear old symbols for %s: %w", file.Path, err)
	}
	if _, err := tx.Exec(`DELETE FROM outline_nodes WHERE file_id = ?`, fileID); err != nil {
		return false, fmt.Errorf("failed to clear old outline nodes for %s: %w", file.Path, err)
	}

	// 3. Parallel-style insert for chunks
	if len(chunks) > 0 {
		chunkStmt, err := tx.Prepare(`
			INSERT INTO chunks (
				id, file_id, content, embedding, embedding_model_id,
				line_start, line_end, char_start, char_end,
				language, symbol_name, symbol_kind, parent,
				signature, visibility, package_name, doc_string,
				token_count, is_collapsed, source_code,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return false, fmt.Errorf("failed to prepare chunk statement for %s: %w", file.Path, err)
		}
		defer chunkStmt.Close()

		for _, chunk := range chunks {
			if chunk.ID == "" {
				chunk.ID = uuid.New().String()
			}
			embeddingBytes, _ := float32SliceToByteSlice(chunk.Embedding)
			_, err = chunkStmt.Exec(
				chunk.ID, fileID, chunk.Content, embeddingBytes, chunk.EmbeddingModelID,
				chunk.LineStart, chunk.LineEnd, chunk.CharStart, chunk.CharEnd,
				chunk.Language, chunk.SymbolName, chunk.SymbolKind, chunk.Parent,
				chunk.Signature, chunk.Visibility, chunk.PackageName, chunk.DocString,
				chunk.TokenCount, chunk.IsCollapsed, chunk.SourceCode, now, now,
			)
			if err != nil {
				return false, fmt.Errorf("failed to insert chunk for %s: %w", file.Path, err)
			}
		}
	}

	// 4. Batch insert symbols
	if len(symbols) > 0 {
		symStmt, err := tx.Prepare(`
			INSERT INTO symbols (id, file_id, name, kind, line, character, parent, language, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return false, fmt.Errorf("failed to prepare symbol statement for %s: %w", file.Path, err)
		}
		defer symStmt.Close()

		for _, sym := range symbols {
			_, err = symStmt.Exec(sym.ID, fileID, sym.Name, sym.Kind, sym.Line, sym.Character,
				sql.NullString{String: sym.Parent, Valid: sym.Parent != ""}, sym.Language, now, now)
			if err != nil {
				return false, fmt.Errorf("failed to insert symbol for %s: %w", file.Path, err)
			}
		}
	}

	// 5. Recursive outline insertion
	if len(outline) > 0 {
		if err := s.insertOutlineNodes(tx, fileID, outline, sql.NullString{}); err != nil {
			return false, fmt.Errorf("failed to insert outline for %s: %w", file.Path, err)
		}
	}

	// 6. Usages
	if len(usages) > 0 {
		usageStmt, err := tx.Prepare(`
			INSERT INTO symbol_usages (
				caller_node_id, target_node_id, raw_target_name, raw_target_context, line, column
			) VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return false, fmt.Errorf("failed to prepare usage statement for %s: %w", file.Path, err)
		}
		defer usageStmt.Close()

		for _, u := range usages {
			_, err = usageStmt.Exec(u.CallerNodeID, sql.NullString{String: u.TargetNodeID, Valid: u.TargetNodeID != ""},
				u.RawTargetName, sql.NullString{String: u.RawTargetContext, Valid: u.RawTargetContext != ""}, u.Line, u.Column)
			if err != nil {
				return false, fmt.Errorf("failed to insert usage for %s: %w", file.Path, err)
			}
		}
	}

	// 7. Metadata update
	if _, err := tx.Exec(`
		INSERT INTO outline_metadata (file_id, updated_at)
		VALUES (?, ?)
		ON CONFLICT(file_id) DO UPDATE SET updated_at = excluded.updated_at
	`, fileID, now); err != nil {
		return false, fmt.Errorf("failed to update metadata for %s: %w", file.Path, err)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return isNew && !file.IsVirtual, nil
}

// GetFileOutline retrieves a stored outline tree.
func (s *VectorStore) GetFileOutline(filePath string) ([]*models.OutlineNode, error) {
	fileID, normalizedPath, err := s.resolveFileID(filePath, false)
	if err != nil {
		// If the file is unknown, report no outline instead of propagating the error
		if strings.Contains(err.Error(), "file not found") {
			return nil, nil
		}
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, parent_id, name, kind, start_line, end_line, position
		FROM outline_nodes
		WHERE file_id = ?
		ORDER BY parent_id, position
	`, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to query outline nodes for %s: %w", normalizedPath, err)
	}
	defer rows.Close()

	childMap := make(map[string][]*models.OutlineNode)
	for rows.Next() {
		var id string
		var parent sql.NullString
		var name, kind string
		var startLine, endLine int64
		var position int
		if err := rows.Scan(&id, &parent, &name, &kind, &startLine, &endLine, &position); err != nil {
			return nil, fmt.Errorf("failed to scan outline node: %w", err)
		}

		node := &models.OutlineNode{
			ID:        id,
			Name:      name,
			Kind:      kind,
			FilePath:  normalizedPath,
			StartLine: uint32(startLine),
			EndLine:   uint32(endLine),
		}

		parentKey := ""
		if parent.Valid {
			parentKey = parent.String
		}
		childMap[parentKey] = append(childMap[parentKey], node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outline rows: %w", err)
	}

	if len(childMap) == 0 {
		return nil, nil
	}

	var attachChildren func(parentKey string) []*models.OutlineNode
	attachChildren = func(parentKey string) []*models.OutlineNode {
		children := childMap[parentKey]
		for _, child := range children {
			child.Children = attachChildren(child.ID)
		}
		return children
	}

	return attachChildren(""), nil
}

// DeleteFileOutline removes stored outline entries for a file.
func (s *VectorStore) DeleteFileOutline(filePath string) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, false)
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(`DELETE FROM outline_nodes WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete outline nodes for %s: %w", normalizedPath, err)
	}
	if _, err := s.db.Exec(`DELETE FROM outline_metadata WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete outline metadata for %s: %w", normalizedPath, err)
	}
	return nil
}

// GetRecentFiles retrieves the last N files updated in the database.
func (s *VectorStore) GetRecentFiles(limit int) ([]*models.File, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT id, path, hash, last_modified, chunk_count, created_at, updated_at
		FROM files
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent files: %w", err)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		err := rows.Scan(
			&file.ID,
			&file.Path,
			&file.Hash,
			&file.LastModified,
			&file.ChunkCount,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent file: %w", err)
		}
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recent files: %w", err)
	}

	return files, nil
}

// ListPhysicalFilePaths returns all physical (non-virtual) file paths tracked in the files table.
func (s *VectorStore) ListPhysicalFilePaths() ([]string, error) {
	rows, err := s.db.Query(`SELECT path FROM files WHERE is_virtual = 0`)
	if err != nil {
		return nil, fmt.Errorf("failed to list tracked physical files: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan file path: %w", err)
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate file paths: %w", err)
	}
	return paths, nil
}

// RemoveFileAndArtifacts deletes all stored data for the given file path.
// If the file is not tracked, it succeeds silently.
// RemoveFileAndArtifacts removes a file and its associated data. Returns the number of physical files removed.
func (s *VectorStore) RemoveFileAndArtifacts(filePath string) (int64, error) {
	normalized, err := normalizeOutlinePath(filePath)
	if err != nil {
		return 0, err
	}

	fileID, _, err := s.resolveFileID(normalized, false)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			return 0, nil
		}
		return 0, err
	}

	var isVirtual bool
	if err := s.db.QueryRow(`SELECT is_virtual FROM files WHERE pk = ?`, fileID).Scan(&isVirtual); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin removal for %s: %w", normalized, err)
	}
	defer tx.Rollback()

	// Since we enabled foreign keys with ON DELETE CASCADE in the connection string,
	// simply deleting the file from the 'files' table will remove chunks, symbols, 
	// outlines, and chunk-symbol mappings automatically.
	if _, err := tx.Exec(`DELETE FROM files WHERE pk = ?`, fileID); err != nil {
		return 0, fmt.Errorf("failed to delete file record for %s: %w", normalized, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	s.fileIDMu.Lock()
	delete(s.fileIDs, normalized)
	s.fileIDMu.Unlock()

	if !isVirtual {
		return 1, nil
	}
	return 0, nil
}

// PurgeOrphanedVirtualFiles removes virtual file entries that are no longer referenced by any symbol usage.
func (s *VectorStore) PurgeOrphanedVirtualFiles() (int64, error) {
	result, err := s.db.Exec(`
		DELETE FROM files 
		WHERE is_virtual = 1 
		AND pk NOT IN (
			SELECT DISTINCT f.pk 
			FROM files f
			JOIN outline_nodes n ON n.file_id = f.pk
			JOIN symbol_usages u ON u.target_node_id = n.id
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to purge orphaned virtual files: %w", err)
	}

	return result.RowsAffected()
}

// RemoveDirectoryAndArtifacts deletes all stored data for files within the given directory path.
// dirPath should be the absolute path to the directory.
// RemoveDirectoryAndArtifacts removes all files under a directory and their associated data. Returns the number of physical files removed.
func (s *VectorStore) RemoveDirectoryAndArtifacts(dirPath string) (int64, error) {
	normalized, err := normalizeOutlinePath(dirPath)
	if err != nil {
		return 0, err
	}

	// Ensure prefix matching for directory paths (e.g., path/to/dir/file.txt)
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}

	// Check if there are any files to remove before starting a transaction.
	// This reduces lock contention in high-concurrency scenarios (e.g. wails build).
	var hasFiles bool
	err = s.db.QueryRow(`SELECT 1 FROM files WHERE path LIKE ? LIMIT 1`, normalized+"%").Scan(&hasFiles)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin directory removal for %s: %w", normalized, err)
	}
	defer tx.Rollback()

	// 1. Clear cached file IDs for all files under this directory
	s.fileIDMu.Lock()
	for path := range s.fileIDs {
		if strings.HasPrefix(path, normalized) {
			delete(s.fileIDs, path)
		}
	}
	s.fileIDMu.Unlock()

	// 2. Count non-virtual files that will be removed
	var removedCount int64
	err = tx.QueryRow(`SELECT COUNT(*) FROM files WHERE path LIKE ? AND is_virtual = 0`, normalized+"%").Scan(&removedCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count files for directory %s: %w", normalized, err)
	}

	// 3. Delete the directory entries and all Cascading children (chunks, symbols, etc.)
	// We use LIKE to matches everything starting with the directory path.
	if _, err := tx.Exec(`DELETE FROM files WHERE path LIKE ?`, normalized+"%"); err != nil {
		return 0, fmt.Errorf("failed to remove files for directory %s: %w", normalized, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return removedCount, nil
}

// GetFileOutlineTimestamp retrieves the last update timestamp for a file's outline.
// Returns 0 if the file has no outline stored.
func (s *VectorStore) GetFileOutlineTimestamp(filePath string) (int64, error) {
	fileID, _, err := s.resolveFileID(filePath, false)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			return 0, nil
		}
		return 0, err
	}

	row := s.db.QueryRow(`SELECT updated_at FROM outline_metadata WHERE file_id = ?`, fileID)
	var timestamp int64
	if err := row.Scan(&timestamp); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to fetch outline timestamp: %w", err)
	}
	return timestamp, nil
}

// GetAllOutlineTimestamps retrieves all file outline timestamps for the project.
// Returns a map of file paths to their last update timestamps.
func (s *VectorStore) GetAllOutlineTimestamps() (map[string]int64, error) {
	rows, err := s.db.Query(`
		SELECT f.path, m.updated_at
		FROM outline_metadata m
		JOIN files f ON f.pk = m.file_id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch outline timestamps: %w", err)
	}
	defer rows.Close()

	timestamps := make(map[string]int64)
	for rows.Next() {
		var path string
		var timestamp int64
		if err := rows.Scan(&path, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan outline timestamp: %w", err)
		}
		timestamps[path] = timestamp
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outline timestamps: %w", err)
	}

	return timestamps, nil
}

// GetFileChunks retrieves all chunks for a given file path.
func (s *VectorStore) GetFileChunks(filePath string) ([]*models.Chunk, error) {
	normalizedPath, err := normalizeOutlinePath(filePath)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT
			c.id, f.path, c.content, c.embedding_model_id,
			c.line_start, c.line_end, c.char_start, c.char_end,
			c.language, c.symbol_name, c.symbol_kind, c.parent, c.signature, c.visibility,
			c.package_name, c.doc_string, c.token_count, c.is_collapsed, c.source_code,
			c.created_at, c.updated_at
		FROM chunks c
		JOIN files f ON f.pk = c.file_id
		WHERE f.path = ?
		ORDER BY c.line_start ASC
	`, normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks for file %s: %w", normalizedPath, err)
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		chunk := &models.Chunk{}
		var language, symbolName, symbolKind, parent, signature, visibility sql.NullString
		var packageName, docString, sourceCode sql.NullString
		var tokenCount sql.NullInt64
		var isCollapsed sql.NullBool

		err := rows.Scan(
			&chunk.ID, &chunk.FilePath, &chunk.Content, &chunk.EmbeddingModelID,
			&chunk.LineStart, &chunk.LineEnd, &chunk.CharStart, &chunk.CharEnd,
			&language, &symbolName, &symbolKind, &parent, &signature, &visibility,
			&packageName, &docString, &tokenCount, &isCollapsed, &sourceCode,
			&chunk.CreatedAt, &chunk.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		// Assign nullable fields
		if language.Valid {
			chunk.Language = language.String
		}
		if symbolName.Valid {
			chunk.SymbolName = symbolName.String
		}
		if symbolKind.Valid {
			chunk.SymbolKind = symbolKind.String
		}
		if parent.Valid {
			chunk.Parent = parent.String
		}
		if signature.Valid {
			chunk.Signature = signature.String
		}
		if visibility.Valid {
			chunk.Visibility = visibility.String
		}
		if packageName.Valid {
			chunk.PackageName = packageName.String
		}
		if docString.Valid {
			chunk.DocString = docString.String
		}
		if tokenCount.Valid {
			chunk.TokenCount = int(tokenCount.Int64)
		}
		if isCollapsed.Valid {
			chunk.IsCollapsed = isCollapsed.Bool
		}
		if sourceCode.Valid {
			chunk.SourceCode = sourceCode.String
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunks: %w", err)
	}

	return chunks, nil
}

// GetChunkByID retrieves a single chunk from the index.
func (s *VectorStore) GetChunkByID(chunkID string) (*models.Chunk, error) {
	trimmed := strings.TrimSpace(chunkID)
	if trimmed == "" {
		return nil, fmt.Errorf("chunk id cannot be empty")
	}

	row := s.db.QueryRow(`
		SELECT
			c.id, f.path, c.content, c.embedding_model_id,
			c.line_start, c.line_end, c.char_start, c.char_end,
			c.language, c.symbol_name, c.symbol_kind, c.parent, c.signature, c.visibility,
			c.package_name, c.doc_string, c.token_count, c.is_collapsed, c.source_code,
			c.created_at, c.updated_at
		FROM chunks c
		JOIN files f ON f.pk = c.file_id
		LEFT JOIN chunk_symbols cs ON c.id = cs.chunk_id
		WHERE c.id = ? OR cs.symbol_id = ?
		LIMIT 1
	`, trimmed, trimmed)

	chunk := &models.Chunk{}
	var language, symbolName, symbolKind, parent, signature, visibility sql.NullString
	var packageName, docString, sourceCode sql.NullString
	var tokenCount sql.NullInt64
	var isCollapsed sql.NullBool

	err := row.Scan(
		&chunk.ID, &chunk.FilePath, &chunk.Content, &chunk.EmbeddingModelID,
		&chunk.LineStart, &chunk.LineEnd, &chunk.CharStart, &chunk.CharEnd,
		&language, &symbolName, &symbolKind, &parent, &signature, &visibility,
		&packageName, &docString, &tokenCount, &isCollapsed, &sourceCode,
		&chunk.CreatedAt, &chunk.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", trimmed)
		}
		return nil, fmt.Errorf("failed to load chunk %s: %w", trimmed, err)
	}

	if language.Valid {
		chunk.Language = language.String
	}
	if symbolName.Valid {
		chunk.SymbolName = symbolName.String
	}
	if symbolKind.Valid {
		chunk.SymbolKind = symbolKind.String
	}
	if parent.Valid {
		chunk.Parent = parent.String
	}
	if signature.Valid {
		chunk.Signature = signature.String
	}
	if visibility.Valid {
		chunk.Visibility = visibility.String
	}
	if packageName.Valid {
		chunk.PackageName = packageName.String
	}
	if docString.Valid {
		chunk.DocString = docString.String
	}
	if tokenCount.Valid {
		chunk.TokenCount = int(tokenCount.Int64)
	}
	if isCollapsed.Valid {
		chunk.IsCollapsed = isCollapsed.Bool
	}
	if sourceCode.Valid {
		chunk.SourceCode = sourceCode.String
	}

	chunk.Embedding = nil
	return chunk, nil
}

// FindChunksFuzzy searches for chunks in a file that match a symbol name or intersect a line range.
// It limits results to the top 10 matches to avoid context bloat.
func (s *VectorStore) FindChunksFuzzy(filePath string, startLine, endLine int, symbolName string) ([]*models.Chunk, error) {
	normalizedPath, err := normalizeOutlinePath(filePath)
	if err != nil {
		return nil, err
	}

	fileID, _, err := s.resolveFileID(normalizedPath, false)
	if err != nil {
		// If file doesn't exist, we can't find chunks for it fuzzy or not.
		if strings.Contains(err.Error(), "file not found") {
			return nil, nil
		}
		return nil, err
	}

	// Priority order:
	// 1. Exact symbol name match
	// 2. Line range intersection (if lines provided)
	// We use coalesce-like logic in ORDER BY to ensure best matches come first.
	query := `
		SELECT
			c.id, f.path, c.content, c.embedding_model_id,
			c.line_start, c.line_end, c.char_start, c.char_end,
			c.language, c.symbol_name, c.symbol_kind, c.parent, c.signature, c.visibility,
			c.package_name, c.doc_string, c.token_count, c.is_collapsed, c.source_code,
			c.created_at, c.updated_at
		FROM chunks c
		JOIN files f ON f.pk = c.file_id
		WHERE c.file_id = ? 
		  AND (
			  (? != '' AND c.symbol_name = ?) 
			  OR 
			  (? > 0 AND ? > 0 AND c.line_start <= ? AND c.line_end >= ?)
		  )
		ORDER BY 
			CASE WHEN ? != '' AND c.symbol_name = ? THEN 0 ELSE 1 END,
			c.line_start ASC
		LIMIT 10
	`

	rows, err := s.db.Query(query, 
		fileID, 
		symbolName, symbolName, 
		startLine, endLine, endLine, startLine,
		symbolName, symbolName)
	if err != nil {
		return nil, fmt.Errorf("failed to fuzzy query chunks for file %s: %w", normalizedPath, err)
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		chunk := &models.Chunk{}
		var language, sName, symbolKind, parent, signature, visibility sql.NullString
		var packageName, docString, sourceCode sql.NullString
		var tokenCount sql.NullInt64
		var isCollapsed sql.NullBool

		err := rows.Scan(
			&chunk.ID, &chunk.FilePath, &chunk.Content, &chunk.EmbeddingModelID,
			&chunk.LineStart, &chunk.LineEnd, &chunk.CharStart, &chunk.CharEnd,
			&language, &sName, &symbolKind, &parent, &signature, &visibility,
			&packageName, &docString, &tokenCount, &isCollapsed, &sourceCode,
			&chunk.CreatedAt, &chunk.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan fuzzy chunk: %w", err)
		}

		if language.Valid { chunk.Language = language.String }
		if sName.Valid { chunk.SymbolName = sName.String }
		if symbolKind.Valid { chunk.SymbolKind = symbolKind.String }
		if parent.Valid { chunk.Parent = parent.String }
		if signature.Valid { chunk.Signature = signature.String }
		if visibility.Valid { chunk.Visibility = visibility.String }
		if packageName.Valid { chunk.PackageName = packageName.String }
		if docString.Valid { chunk.DocString = docString.String }
		if tokenCount.Valid { chunk.TokenCount = int(tokenCount.Int64) }
		if isCollapsed.Valid { chunk.IsCollapsed = isCollapsed.Bool }
		if sourceCode.Valid { chunk.SourceCode = sourceCode.String }

		chunk.Embedding = nil
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fuzzy chunks: %w", err)
	}

	return chunks, nil
}

// DeleteFileChunks removes all chunks, symbols, and outline nodes associated with a file.
func (s *VectorStore) DeleteFileChunks(filePath string) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, true)
	if err != nil {
		return err
	}

	// Delete chunks
	if _, err := s.db.Exec(`DELETE FROM chunks WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete chunks for file %s: %w", normalizedPath, err)
	}

	// Delete symbols
	if _, err := s.db.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete symbols for file %s: %w", normalizedPath, err)
	}

	// Delete outline nodes (this will cascade to symbol_usages)
	if _, err := s.db.Exec(`DELETE FROM outline_nodes WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete outline nodes for file %s: %w", normalizedPath, err)
	}

	return nil
}

// RebuildChunkSymbolLinks refreshes the chunk_symbols mapping for a file.
func (s *VectorStore) RebuildChunkSymbolLinks(filePath string) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, true)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to rebuild chunk-symbol links for %s: %w", normalizedPath, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		DELETE FROM chunk_symbols
		WHERE chunk_id IN (SELECT id FROM chunks WHERE file_id = ?)
	`, fileID); err != nil {
		return fmt.Errorf("failed to clear chunk-symbol links for %s: %w", normalizedPath, err)
	}

	if _, err := tx.Exec(`
		INSERT INTO chunk_symbols (chunk_id, symbol_id)
		SELECT c.id, s.id
		FROM chunks c
		JOIN symbols s ON c.file_id = s.file_id
		WHERE c.file_id = ?
		  AND s.line BETWEEN c.line_start AND c.line_end
	`, fileID); err != nil {
		return fmt.Errorf("failed to insert chunk-symbol links for %s: %w", normalizedPath, err)
	}

	return tx.Commit()
}

// ResetProjectData removes all indexed artifacts (chunks, symbols, outlines, files).
func (s *VectorStore) ResetProjectData() error {
	// Order is important: deleting from 'files' first would cascade to everything else,
	// but we'll be' explicit to ensure all tables are wiped.
	tables := []string{
		"symbol_usages",
		"symbol_implementations",
		"chunk_symbols",
		"chunks",
		"symbols",
		"outline_nodes",
		"outline_metadata",
		"files",
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin reset transaction: %w", err)
	}
	defer tx.Rollback()

	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("failed to clear %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit reset: %w", err)
	}

	// Always clear cache after a successful reset to prevent stale FK references.
	s.ResetCache()
	log.Printf("[VectorStore] Project data reset successfully, cache cleared.")

	return nil
}

// Helper to convert []float32 to []byte (little-endian)
func float32SliceToByteSlice(floats []float32) ([]byte, error) {
	if len(floats) == 0 {
		return []byte{}, nil
	}
	out := make([]byte, 4*len(floats))
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(out[i*4:], bits)
	}
	return out, nil
}

// Helper to convert []byte to []float32 (little-endian)
func byteSliceToFloat32Slice(bytes []byte) ([]float32, error) {
	if len(bytes) == 0 {
		return []float32{}, nil
	}
	if len(bytes)%4 != 0 {
		return nil, fmt.Errorf("embedding byte slice length %d is not a multiple of 4", len(bytes))
	}
	count := len(bytes) / 4
	out := make([]float32, count)
	for i := 0; i < count; i++ {
		bits := binary.LittleEndian.Uint32(bytes[i*4:])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}

// SearchSimilarChunks performs a brute-force cosine similarity search over chunks.
// It filters by modelID to ensure valid vector comparisons.
func (s *VectorStore) SearchSimilarChunks(queryEmbedding []float32, modelID string, k int) ([]*models.Chunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is empty")
	}

	if k <= 0 {
		k = 10
	}

	rows, err := s.db.Query(`
		SELECT c.id, f.path, c.content, c.embedding, c.embedding_model_id, c.line_start, c.line_end, c.char_start, c.char_end,
		       c.language, c.symbol_name, c.symbol_kind, c.parent, c.signature, c.visibility,
		       c.package_name, c.doc_string, c.token_count, c.is_collapsed, c.source_code,
		       c.created_at, c.updated_at
		FROM chunks c
		JOIN files f ON f.pk = c.file_id
		WHERE c.embedding_model_id = ?
	`, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks for search: %w", err)
	}
	defer rows.Close()

	queryNorm := dotProduct(queryEmbedding, queryEmbedding)
	if queryNorm == 0 {
		return nil, fmt.Errorf("query embedding has zero norm")
	}
	queryNorm = math.Sqrt(queryNorm)

	top := newMinHeap(k)

	for rows.Next() {
		chunk := &models.Chunk{}
		var embeddingBytes []byte
		var language, symbolName, symbolKind, parent, signature, visibility sql.NullString
		var packageName, docString, sourceCode sql.NullString
		var tokenCount sql.NullInt64
		var isCollapsed sql.NullBool

		err := rows.Scan(
			&chunk.ID,
			&chunk.FilePath,
			&chunk.Content,
			&embeddingBytes,
			&chunk.EmbeddingModelID,
			&chunk.LineStart,
			&chunk.LineEnd,
			&chunk.CharStart,
			&chunk.CharEnd,
			&language,
			&symbolName,
			&symbolKind,
			&parent,
			&signature,
			&visibility,
			&packageName,
			&docString,
			&tokenCount,
			&isCollapsed,
			&sourceCode,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk for search: %w", err)
		}

		vec, err := byteSliceToFloat32Slice(embeddingBytes)
		if err != nil {
			return nil, err
		}
		chunk.Embedding = vec

		// Assign nullable fields
		if language.Valid {
			chunk.Language = language.String
		}
		if symbolName.Valid {
			chunk.SymbolName = symbolName.String
		}
		if symbolKind.Valid {
			chunk.SymbolKind = symbolKind.String
		}
		if parent.Valid {
			chunk.Parent = parent.String
		}
		if signature.Valid {
			chunk.Signature = signature.String
		}
		if visibility.Valid {
			chunk.Visibility = visibility.String
		}
		if packageName.Valid {
			chunk.PackageName = packageName.String
		}
		if docString.Valid {
			chunk.DocString = docString.String
		}
		if tokenCount.Valid {
			chunk.TokenCount = int(tokenCount.Int64)
		}
		if isCollapsed.Valid {
			chunk.IsCollapsed = isCollapsed.Bool
		}
		if sourceCode.Valid {
			chunk.SourceCode = sourceCode.String
		}

		if len(vec) == 0 {
			continue
		}
		score := cosineSimilarity(queryEmbedding, vec, queryNorm)
		chunk.Similarity = score
		top.Push(chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search rows: %w", err)
	}

	result := top.Sorted()
	return result, nil
}

func cosineSimilarity(a []float32, b []float32, normA float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	normB := float64(0)
	dot := float64(0)
	for i := 0; i < len(a); i++ {
		dot += float64(a[i]) * float64(b[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normB == 0 || normA == 0 {
		return 0
	}
	return dot / (normA * math.Sqrt(normB))
}

func dotProduct(a []float32, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	dot := float64(0)
	for i := 0; i < len(a); i++ {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}

// minHeap keeps top-k chunks by similarity (ascending heap).
type minHeap struct {
	cap  int
	data []*models.Chunk
}

func newMinHeap(capacity int) *minHeap {
	if capacity <= 0 {
		capacity = 10
	}
	return &minHeap{cap: capacity, data: make([]*models.Chunk, 0, capacity)}
}

func (h *minHeap) Push(c *models.Chunk) {
	if len(h.data) < h.cap {
		h.data = append(h.data, c)
		h.up(len(h.data) - 1)
		return
	}
	if len(h.data) == 0 {
		return
	}
	if c.Similarity <= h.data[0].Similarity {
		return
	}
	h.data[0] = c
	h.down(0)
}

func (h *minHeap) Sorted() []*models.Chunk {
	// Return in descending order
	res := make([]*models.Chunk, len(h.data))
	copy(res, h.data)
	sort.Slice(res, func(i, j int) bool { return res[i].Similarity > res[j].Similarity })
	return res
}

func (h *minHeap) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h.data[parent].Similarity <= h.data[i].Similarity {
			break
		}
		h.data[parent], h.data[i] = h.data[i], h.data[parent]
		i = parent
	}
}

func (h *minHeap) down(i int) {
	n := len(h.data)
	for {
		left := 2*i + 1
		right := 2*i + 2
		smallest := i
		if left < n && h.data[left].Similarity < h.data[smallest].Similarity {
			smallest = left
		}
		if right < n && h.data[right].Similarity < h.data[smallest].Similarity {
			smallest = right
		}
		if smallest == i {
			break
		}
		h.data[i], h.data[smallest] = h.data[smallest], h.data[i]
		i = smallest
	}
}

// GetStats returns statistics for the project index.
func (s *VectorStore) GetStats() (*models.ProjectStats, error) {
	stats := &models.ProjectStats{}

	// Get total files count
	err := s.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.TotalFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	// Get total chunks count
	err = s.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&stats.TotalChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to count chunks: %w", err)
	}

	// Get total symbols count
	err = s.db.QueryRow("SELECT COUNT(*) FROM symbols").Scan(&stats.TotalSymbols)
	if err != nil {
		return nil, fmt.Errorf("failed to count symbols: %w", err)
	}

	// Get database size
	fileInfo, err := os.Stat(s.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	stats.DatabaseSize = fileInfo.Size()

	// Get last indexed timestamp (from the most recently updated file)
	var lastIndexedUnix sql.NullInt64
	err = s.db.QueryRow("SELECT MAX(updated_at) FROM outline_metadata").Scan(&lastIndexedUnix)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last indexed time: %w", err)
	}
	if lastIndexedUnix.Valid && lastIndexedUnix.Int64 > 0 {
		t := time.Unix(lastIndexedUnix.Int64, 0)
		stats.LastIndexedAt = &t
		stats.LastIndexedAtUnix = t.Unix()
	}

	rows, err := s.db.Query(`
		SELECT embedding_model_id, COUNT(*) as cnt
		FROM chunks
		GROUP BY embedding_model_id
		ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate embedding models: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var modelID sql.NullString
		var count int64
		if err := rows.Scan(&modelID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan embedding model usage: %w", err)
		}
		usage := models.ProjectEmbeddingModelUsage{
			ModelID:    strings.TrimSpace(modelID.String),
			ChunkCount: int(count),
		}
		if usage.ModelID == "" {
			usage.ModelID = "unknown"
		}
		stats.EmbeddingModels = append(stats.EmbeddingModels, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate embedding model usage: %w", err)
	}

	// Add summary
	summary, err := s.GetProjectSummary()
	if err == nil {
		stats.Summary = summary
	}

	return stats, nil
}

// GetProjectSummary aggregates structural information about the project.
func (s *VectorStore) GetProjectSummary() (*models.ProjectSummary, error) {
	summary := &models.ProjectSummary{
		Languages:      []string{},
		Packages:       []string{},
		EntryPoints:    []string{},
		MainComponents: []string{},
	}

	// 1. Get top 5 languages
	rows, err := s.db.Query(`
		SELECT language, COUNT(*) as count 
		FROM chunks 
		WHERE language IS NOT NULL AND language != "" 
		GROUP BY language 
		ORDER BY count DESC 
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lang string
			var count int
			if err := rows.Scan(&lang, &count); err == nil {
				summary.Languages = append(summary.Languages, lang)
			}
		}
	}

	// 2. Get top 8 packages
	rows, err = s.db.Query(`
		SELECT package_name, COUNT(*) as count 
		FROM chunks 
		WHERE package_name IS NOT NULL AND package_name != "" 
		GROUP BY package_name 
		ORDER BY count DESC 
		LIMIT 8
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pkg string
			var count int
			if err := rows.Scan(&pkg, &count); err == nil {
				summary.Packages = append(summary.Packages, pkg)
			}
		}
	}

	// 3. Get potential entry points
	// Look for main functions or files named main/index/app
	rows, err = s.db.Query(`
		SELECT DISTINCT f.path 
		FROM symbols s 
		JOIN files f ON f.pk = s.file_id 
		WHERE s.name IN ('main', 'main.go', 'index.ts', 'app.py') 
		   OR f.path LIKE 'main.%' 
		   OR f.path LIKE 'index.%' 
		   OR f.path LIKE 'app.%'
		   OR f.path LIKE '%/main.%'
		   OR f.path LIKE '%/index.%'
		   OR f.path LIKE '%/app.%'
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err == nil {
				summary.EntryPoints = append(summary.EntryPoints, path)
			}
		}
	}

	// 4. Get main components (top-level directories)
	rows, err = s.db.Query(`
		SELECT DISTINCT 
			CASE 
				WHEN instr(path, '/') > 0 THEN substr(path, 1, instr(path, '/') - 1)
				ELSE path 
			END as root_dir,
			COUNT(*) as count
		FROM files 
		GROUP BY root_dir 
		ORDER BY count DESC 
		LIMIT 6
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var dir string
			var count int
			if err := rows.Scan(&dir, &count); err == nil {
				// Avoid adding single files as components unless they are the only things
				if !strings.Contains(dir, ".") || len(summary.MainComponents) < 2 {
					summary.MainComponents = append(summary.MainComponents, dir)
				}
			}
		}
	}

	return summary, nil
}

// InsertSymbolUsage inserts a new symbol usage record into the database.
func (s *VectorStore) InsertSymbolUsage(usage *models.SymbolUsage) error {
	stmt, err := s.db.Prepare(`
		INSERT INTO symbol_usages (
			caller_node_id, target_node_id, raw_target_name, raw_target_context, line, column
		) VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert symbol usage statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(
		usage.CallerNodeID,
		sql.NullString{String: usage.TargetNodeID, Valid: usage.TargetNodeID != ""},
		usage.RawTargetName,
		sql.NullString{String: usage.RawTargetContext, Valid: usage.RawTargetContext != ""},
		usage.Line,
		usage.Column,
	)
	if err != nil {
		return fmt.Errorf("failed to insert symbol usage: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		usage.ID = id
	}

	return nil
}

// GetSymbolUsages retrieves all usages (references) for a specific target symbol.
func (s *VectorStore) GetSymbolUsages(targetNodeID string) ([]*models.SymbolUsage, error) {
	rows, err := s.db.Query(`
		SELECT 
			u.id, f.path, u.caller_node_id, n.name, n.kind, u.target_node_id, 
			u.line, u.column
		FROM symbol_usages u
		JOIN outline_nodes n ON n.id = u.caller_node_id
		JOIN files f ON f.pk = n.file_id
		WHERE u.target_node_id = ?
		ORDER BY f.path, u.line
	`, targetNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query symbol usages for %s: %w", targetNodeID, err)
	}
	defer rows.Close()

	var usages []*models.SymbolUsage
	for rows.Next() {
		usage := &models.SymbolUsage{}
		var targetID sql.NullString
		err := rows.Scan(
			&usage.ID, &usage.FilePath, &usage.CallerNodeID, &usage.CallerName, &usage.CallerKind, &targetID,
			&usage.Line, &usage.Column,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan symbol usage: %w", err)
		}
		if targetID.Valid {
			usage.TargetNodeID = targetID.String
		}
		usages = append(usages, usage)
	}

	return usages, nil
}

// GetOutgoingCalls returns all symbol usage records initiated by the specified caller node.
func (s *VectorStore) GetOutgoingCalls(callerNodeID string) ([]*models.SymbolUsage, error) {
	rows, err := s.db.Query(`
		SELECT id, target_node_id, line
		FROM symbol_usages
		WHERE caller_node_id = ? AND target_node_id IS NOT NULL
	`, callerNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query outgoing calls for %s: %w", callerNodeID, err)
	}
	defer rows.Close()

	var usages []*models.SymbolUsage
	for rows.Next() {
		u := &models.SymbolUsage{CallerNodeID: callerNodeID}
		if err := rows.Scan(&u.ID, &u.TargetNodeID, &u.Line); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// GetOutlineNodes returns the details for a list of node IDs.
func (s *VectorStore) GetOutlineNodes(ids []string) ([]*models.OutlineNode, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT n.id, f.path, n.parent_id, n.name, n.kind, n.start_line, n.end_line, n.position
		FROM outline_nodes n
		JOIN files f ON f.pk = n.file_id
		WHERE n.id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query outline nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.OutlineNode
	for rows.Next() {
		n := &models.OutlineNode{}
		var parentID sql.NullString
		var position int
		if err := rows.Scan(&n.ID, &n.FilePath, &parentID, &n.Name, &n.Kind, &n.StartLine, &n.EndLine, &position); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// ListUnresolvedUsages returns all symbol usages that haven't been linked to a target node yet.
func (s *VectorStore) ListUnresolvedUsages() ([]*models.SymbolUsage, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_node_id, raw_target_name, raw_target_context, line, column
		FROM symbol_usages
		WHERE target_node_id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query unresolved usages: %w", err)
	}
	defer rows.Close()

	var usages []*models.SymbolUsage
	for rows.Next() {
		u := &models.SymbolUsage{}
		var context sql.NullString
		if err := rows.Scan(&u.ID, &u.CallerNodeID, &u.RawTargetName, &context, &u.Line, &u.Column); err != nil {
			return nil, err
		}
		if context.Valid {
			u.RawTargetContext = context.String
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// ListUnresolvedUsagesForFile returns all symbol usages in a specific file that haven't been linked yet.
func (s *VectorStore) ListUnresolvedUsagesForFile(filePath string) ([]*models.SymbolUsage, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.caller_node_id, u.raw_target_name, u.raw_target_context, u.line, u.column
		FROM symbol_usages u
		JOIN outline_nodes n ON n.id = u.caller_node_id
		JOIN files f ON f.pk = n.file_id
		WHERE u.target_node_id IS NULL AND f.path = ?
	`, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query unresolved usages for file %s: %w", filePath, err)
	}
	defer rows.Close()

	var usages []*models.SymbolUsage
	for rows.Next() {
		u := &models.SymbolUsage{}
		var context sql.NullString
		if err := rows.Scan(&u.ID, &u.CallerNodeID, &u.RawTargetName, &context, &u.Line, &u.Column); err != nil {
			return nil, err
		}
		if context.Valid {
			u.RawTargetContext = context.String
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// UpdateSymbolUsageTarget sets the target node ID for an existing usage record.
func (s *VectorStore) UpdateSymbolUsageTarget(usageID int64, targetNodeID string) error {
	_, err := s.db.Exec(`
		UPDATE symbol_usages 
		SET target_node_id = ? 
		WHERE id = ?
	`, targetNodeID, usageID)
	return err
}

// UpdateSymbolUsageTargets updates target node IDs for multiple usage records in a single transaction.
func (s *VectorStore) UpdateSymbolUsageTargets(updates map[int64]string) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE symbol_usages 
		SET target_node_id = ? 
		WHERE id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for usageID, targetNodeID := range updates {
		if _, err := stmt.Exec(targetNodeID, usageID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// FindSymbolNodesByName searches for all outline nodes matching a given name across the project.
func (s *VectorStore) FindSymbolNodesByName(name string) ([]*models.OutlineNode, error) {
	rows, err := s.db.Query(`
		SELECT n.id, n.file_id, n.parent_id, n.name, n.kind, n.start_line, n.end_line, n.position, f.path
		FROM outline_nodes n
		JOIN files f ON f.pk = n.file_id
		WHERE n.name = ?
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*models.OutlineNode
	for rows.Next() {
		n := &models.OutlineNode{}
		var parentID sql.NullString
		var fileID int64
		var position int
		if err := rows.Scan(&n.ID, &fileID, &parentID, &n.Name, &n.Kind, &n.StartLine, &n.EndLine, &position, &n.FilePath); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// InsertVirtualSymbol adds a new virtual symbol (for external dependencies) to the outline.
func (s *VectorStore) InsertVirtualSymbol(path string, node *models.OutlineNode) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin virtual symbol transaction: %w", err)
	}
	defer tx.Rollback()

	fileID, _, err := s.resolveFileIDTx(tx, path, true) // create if doesn't exist
	if err != nil {
		return err
	}

	// Update file to be virtual if it wasn't already
	if _, err := tx.Exec(`UPDATE files SET is_virtual = 1 WHERE pk = ?`, fileID); err != nil {
		return fmt.Errorf("failed to mark file virtual: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO outline_nodes (id, file_id, parent_id, name, kind, start_line, end_line, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			file_id = excluded.file_id,
			parent_id = NULL,
			name = excluded.name,
			kind = excluded.kind,
			start_line = excluded.start_line,
			end_line = excluded.end_line
	`, node.ID, fileID, nil, node.Name, node.Kind, node.StartLine, node.EndLine, 0)
	
	if err != nil {
		return fmt.Errorf("failed to create virtual symbol: %w", err)
	}
	
	return tx.Commit()
}

// GetDB returns the underlying database handle (for testing).
func (s *VectorStore) GetDB() *sql.DB {
	return s.db
}

// GetTodos retrieves all symbols of kind SymbolTodo from the database,
// grouping them by category (TODO, FIXME, etc.) extracted from the message.
func (s *VectorStore) GetTodos() (map[string][]string, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.name
		FROM symbols s
		WHERE s.kind = 'todo'
		ORDER BY s.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query todos: %w", err)
	}
	defer rows.Close()

	categories := make(map[string][]string)
	keywords := []string{"FIXME", "TODO", "HACK", "XXX", "NOTE"}

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}

		upperName := strings.ToUpper(name)
		category := "OTHER"
		for _, kw := range keywords {
			if strings.HasPrefix(upperName, kw) {
				category = kw
				break
			}
		}

		categories[category] = append(categories[category], id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetSymbolUsagePaths retrieves all pairs of file paths (source and target) 
// that are linked via symbol usage records.
func (s *VectorStore) GetSymbolUsagePaths() ([]models.UsagePath, error) {
	rows, err := s.db.Query(`
		SELECT f1.path as source_path, f2.path as target_path
		FROM symbol_usages u
		JOIN outline_nodes n1 ON u.caller_node_id = n1.id
		JOIN files f1 ON n1.file_id = f1.pk
		JOIN outline_nodes n2 ON u.target_node_id = n2.id
		JOIN files f2 ON n2.file_id = f2.pk
		WHERE f1.is_virtual = 0
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query symbol usage paths: %w", err)
	}
	defer rows.Close()

	var results []models.UsagePath
	for rows.Next() {
		var up models.UsagePath
		if err := rows.Scan(&up.SourcePath, &up.TargetPath); err != nil {
			return nil, fmt.Errorf("failed to scan usage path: %w", err)
		}
		results = append(results, up)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage path rows: %w", err)
	}

	return results, nil
}

// GetSymbolImplementations retrieves all symbols that implement the given interface name,
// returning models.SymbolImplementation structs.
func (s *VectorStore) GetSymbolImplementations(interfaceName string) ([]models.SymbolImplementation, error) {
	rows, err := s.db.Query(`
		SELECT 
			s.name,
			f.path,
			s.line,
			c.content
		FROM symbol_implementations si
		JOIN symbols s ON s.id = si.implementor_id
		JOIN files f ON f.pk = si.file_id
		LEFT JOIN chunks c ON c.file_id = f.pk AND c.symbol_name = s.name AND c.line_start = s.line
		WHERE si.interface_name = ?
	`, interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to query symbol implementations: %w", err)
	}
	defer rows.Close()

	var impls []models.SymbolImplementation
	for rows.Next() {
		var name, path string
		var line int
		var content sql.NullString
		
		if err := rows.Scan(&name, &path, &line, &content); err != nil {
			return nil, fmt.Errorf("failed to scan symbol implementation: %w", err)
		}

		// Ensure we don't return entirely blank content if we didn't find the chunk.
		snippet := ""
		if content.Valid {
			snippet = content.String
		}

		impls = append(impls, models.SymbolImplementation{
			SymbolName: name,
			Location:   fmt.Sprintf("%s:%d", path, line),
			Content:    snippet,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over symbol implementations: %w", err)
	}

	return impls, nil
}

// FileSemanticStats represents semantic metadata for a file extracted from the DB.
type FileSemanticStats struct {
	Lines     int
	Symbols   int
	Languages []string
}

func (s *VectorStore) GetFileSemanticStats(paths []string) (map[string]FileSemanticStats, error) {
	if len(paths) == 0 {
		return make(map[string]FileSemanticStats), nil
	}

	results := make(map[string]FileSemanticStats)
	placeholders := s.makePlaceholders(len(paths))
	args := make([]interface{}, len(paths))
	for i, p := range paths {
		args[i] = p
	}

	// 1. Get lines from chunks table
	queryChunks := `
		SELECT f.path, MAX(c.line_end)
		FROM chunks c
		JOIN files f ON f.pk = c.file_id
		WHERE f.path IN (` + placeholders + `)
		GROUP BY f.path
	`

	rowsChunks, err := s.db.Query(queryChunks, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query file semantic stats (chunks): %w", err)
	}
	defer rowsChunks.Close()

	for rowsChunks.Next() {
		var path string
		var maxLine int
		if err := rowsChunks.Scan(&path, &maxLine); err != nil {
			return nil, err
		}
		
		results[path] = FileSemanticStats{
			Lines: maxLine,
		}
	}

	// 2. Get symbol counts and languages from symbols table
	querySym := `
		SELECT f.path, COUNT(*), GROUP_CONCAT(DISTINCT s.language)
		FROM symbols s
		JOIN files f ON f.pk = s.file_id
		WHERE f.path IN (` + placeholders + `)
		GROUP BY f.path
	`

	rowsSym, err := s.db.Query(querySym, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query file semantic stats (symbols): %w", err)
	}
	defer rowsSym.Close()

	for rowsSym.Next() {
		var path string
		var count int
		var languages sql.NullString
		if err := rowsSym.Scan(&path, &count, &languages); err != nil {
			return nil, err
		}
		
		stats, ok := results[path]
		if !ok {
			stats = FileSemanticStats{}
		}
		stats.Symbols = count
		
		if languages.Valid && languages.String != "" {
			for _, l := range strings.Split(languages.String, ",") {
				lang := strings.TrimSpace(l)
				if lang != "" {
					stats.Languages = append(stats.Languages, lang)
				}
			}
			sort.Strings(stats.Languages)
		}
		
		results[path] = stats
	}

	return results, nil
}

func (s *VectorStore) makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("?")
		if i < n-1 {
			b.WriteString(",")
		}
	}
	return b.String()
}
