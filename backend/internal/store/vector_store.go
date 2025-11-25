package store

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"database/sql"
	"embed"
	"encoding/binary"
	"fmt"
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

// NewVectorStore creates a new VectorStore instance for a given project.
// It initializes the SQLite database and runs migrations if necessary.
func NewVectorStore(projectID, projectSlug string) (*VectorStore, error) {
	dataDir, err := utils.GetAppDataDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get data directory: %w", err)
	}

	projectIndexDir := filepath.Join(dataDir, "indexes")
	if err := os.MkdirAll(projectIndexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project index directory: %w", err)
	}

	dbPath := filepath.Join(projectIndexDir, fmt.Sprintf("project-%s.db", projectSlug))

	// Open with WAL mode for better concurrent access and busy timeout
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open vector database at %s: %w", dbPath, err)
	}

	// Set connection pool parameters to prevent excessive concurrent connections
	db.SetMaxOpenConns(1) // SQLite works best with a single writer
	db.SetMaxIdleConns(1)

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

func (s *VectorStore) cacheFileID(path string, id int64) {
	s.fileIDMu.Lock()
	defer s.fileIDMu.Unlock()
	s.fileIDs[path] = id
}

func (s *VectorStore) getCachedFileID(path string) (int64, bool) {
	s.fileIDMu.RLock()
	defer s.fileIDMu.RUnlock()
	id, ok := s.fileIDs[path]
	return id, ok
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
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
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
	normalized, err := normalizeOutlinePath(path)
	if err != nil {
		return 0, "", err
	}

	if cached, ok := s.getCachedFileID(normalized); ok {
		return cached, normalized, nil
	}

	row := s.db.QueryRow(`SELECT pk FROM files WHERE path = ?`, normalized)
	var fileID int64
	if err := row.Scan(&fileID); err != nil {
		if err == sql.ErrNoRows {
			if !create {
				return 0, "", fmt.Errorf("file not found: %s", normalized)
			}
			if fileID, err = s.createPlaceholderFile(normalized); err != nil {
				return 0, "", err
			}
		} else {
			return 0, "", fmt.Errorf("failed to resolve file id for %s: %w", normalized, err)
		}
	}

	s.cacheFileID(normalized, fileID)
	return fileID, normalized, nil
}

func (s *VectorStore) createPlaceholderFile(path string) (int64, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`
		INSERT INTO files (id, path, hash, last_modified, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), path, "unknown", 0, 0, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to create placeholder for %s: %w", path, err)
	}

	fileID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to determine placeholder id for %s: %w", path, err)
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
	chunk.ID = uuid.New().String()
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
		INSERT INTO files (id, path, hash, last_modified, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			id = excluded.id,
			hash = excluded.hash,
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
		SELECT id, path, hash, last_modified, chunk_count, created_at, updated_at
		FROM files
		WHERE path = ?
	`, normalizedPath)

	file := &models.File{}
	err = row.Scan(
		&file.ID,
		&file.Path,
		&file.Hash,
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
	symbol.ID = uuid.New().String()
	symbol.CreatedAt = time.Now().Unix()
	symbol.UpdatedAt = time.Now().Unix()

	fileID, normalizedPath, err := s.resolveFileID(symbol.FilePath, true)
	if err != nil {
		return err
	}
	symbol.FilePath = normalizedPath

	stmt, err := s.db.Prepare(`
		INSERT OR REPLACE INTO symbols (id, file_id, name, kind, line, character, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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
		symbol.CreatedAt,
		symbol.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert symbol: %w", err)
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

// ListAllFilePaths returns all file paths tracked in the files table.
func (s *VectorStore) ListAllFilePaths() ([]string, error) {
	rows, err := s.db.Query(`SELECT path FROM files`)
	if err != nil {
		return nil, fmt.Errorf("failed to list tracked files: %w", err)
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
func (s *VectorStore) RemoveFileAndArtifacts(filePath string) error {
	normalized, err := normalizeOutlinePath(filePath)
	if err != nil {
		return err
	}

	fileID, _, err := s.resolveFileID(normalized, false)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			return nil
		}
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin removal for %s: %w", normalized, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM chunk_symbols WHERE chunk_id IN (SELECT id FROM chunks WHERE file_id = ?)`, fileID); err != nil {
		return fmt.Errorf("failed to delete chunk-symbol links for %s: %w", normalized, err)
	}
	if _, err := tx.Exec(`DELETE FROM chunks WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete chunks for %s: %w", normalized, err)
	}
	if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete symbols for %s: %w", normalized, err)
	}
	if _, err := tx.Exec(`DELETE FROM outline_nodes WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete outline nodes for %s: %w", normalized, err)
	}
	if _, err := tx.Exec(`DELETE FROM outline_metadata WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete outline metadata for %s: %w", normalized, err)
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE pk = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete file record for %s: %w", normalized, err)
	}

	s.fileIDMu.Lock()
	delete(s.fileIDs, normalized)
	s.fileIDMu.Unlock()

	return tx.Commit()
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

// DeleteFileChunks removes all chunks associated with a file.
func (s *VectorStore) DeleteFileChunks(filePath string) error {
	fileID, normalizedPath, err := s.resolveFileID(filePath, true)
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(`DELETE FROM chunks WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("failed to delete chunks for file %s: %w", normalizedPath, err)
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
	tables := []string{
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

	s.fileIDMu.Lock()
	s.fileIDs = make(map[string]int64)
	s.fileIDMu.Unlock()

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

// SearchSimilarChunks performs a brute-force cosine similarity search over all chunks.
// This is a fallback implementation until a vector index (e.g., sqlite-vec) is integrated.
func (s *VectorStore) SearchSimilarChunks(queryEmbedding []float32, k int) ([]*models.Chunk, error) {
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
	`)
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
			ModelID:   strings.TrimSpace(modelID.String),
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

	return stats, nil
}
