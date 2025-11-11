package store

import (
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// SaveProjectMetadata persists the project metadata using this vector database.
func (s *VectorStore) SaveProjectMetadata(project *models.Project) error {
	return saveProjectMetadataWithDB(s.db, project)
}

// Close closes the database connection.
func (s *VectorStore) Close() error {
	return s.db.Close()
}

// InsertChunk inserts a new chunk into the database.
func (s *VectorStore) InsertChunk(chunk *models.Chunk) error {
	chunk.ID = uuid.New().String()
	chunk.CreatedAt = time.Now().Unix()
	chunk.UpdatedAt = time.Now().Unix()

	stmt, err := s.db.Prepare(`
		INSERT INTO chunks (id, file_path, content, embedding, line_start, line_end, char_start, char_end, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		chunk.FilePath,
		chunk.Content,
		embeddingBytes,
		chunk.LineStart,
		chunk.LineEnd,
		chunk.CharStart,
		chunk.CharEnd,
		chunk.CreatedAt,
		chunk.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}

	return nil
}

// InsertFile inserts a new file record into the database.
func (s *VectorStore) InsertFile(file *models.File) error {
	file.ID = uuid.New().String()
	file.CreatedAt = time.Now().Unix()
	file.UpdatedAt = time.Now().Unix()

	stmt, err := s.db.Prepare(`
		INSERT INTO files (id, path, hash, last_modified, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
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

	return nil
}

// InsertSymbol inserts a new symbol record into the database.
func (s *VectorStore) InsertSymbol(symbol *models.Symbol) error {
	symbol.ID = uuid.New().String()
	symbol.CreatedAt = time.Now().Unix()
	symbol.UpdatedAt = time.Now().Unix()

	stmt, err := s.db.Prepare(`
		INSERT INTO symbols (id, file_path, name, kind, line, character, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert symbol statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		symbol.ID,
		symbol.FilePath,
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

// UpsertFileOutline saves the outline tree for a file.
func (s *VectorStore) UpsertFileOutline(filePath string, outline []*models.OutlineNode) error {
	pathKey, err := normalizeOutlinePath(filePath)
	if err != nil {
		return err
	}

	data, err := json.Marshal(outline)
	if err != nil {
		return fmt.Errorf("failed to marshal outline nodes: %w", err)
	}

	stmt, err := s.db.Prepare(`
		INSERT INTO file_outlines (file_path, outline_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			outline_json = excluded.outline_json,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare outline upsert: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(pathKey, string(data), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to upsert outline for %s: %w", pathKey, err)
	}
	return nil
}

// GetFileOutline retrieves a stored outline tree.
func (s *VectorStore) GetFileOutline(filePath string) ([]*models.OutlineNode, error) {
	pathKey, err := normalizeOutlinePath(filePath)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRow(`SELECT outline_json FROM file_outlines WHERE file_path = ?`, pathKey)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch outline for %s: %w", filePath, err)
	}

	var nodes []*models.OutlineNode
	if err := json.Unmarshal([]byte(payload), &nodes); err != nil {
		return nil, fmt.Errorf("failed to decode outline for %s: %w", filePath, err)
	}

	return nodes, nil
}

// DeleteFileOutline removes a stored outline entry.
func (s *VectorStore) DeleteFileOutline(filePath string) error {
	pathKey, err := normalizeOutlinePath(filePath)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`DELETE FROM file_outlines WHERE file_path = ?`, pathKey)
	if err != nil {
		return fmt.Errorf("failed to delete outline for %s: %w", pathKey, err)
	}
	return nil
}

// GetFileOutlineTimestamp retrieves the last update timestamp for a file's outline.
// Returns 0 if the file has no outline stored.
func (s *VectorStore) GetFileOutlineTimestamp(filePath string) (int64, error) {
	pathKey, err := normalizeOutlinePath(filePath)
	if err != nil {
		return 0, err
	}

	row := s.db.QueryRow(`SELECT updated_at FROM file_outlines WHERE file_path = ?`, pathKey)
	var timestamp int64
	if err := row.Scan(&timestamp); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to fetch outline timestamp for %s: %w", filePath, err)
	}
	return timestamp, nil
}

// GetAllOutlineTimestamps retrieves all file outline timestamps for the project.
// Returns a map of file paths to their last update timestamps.
func (s *VectorStore) GetAllOutlineTimestamps() (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT file_path, updated_at FROM file_outlines`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch outline timestamps: %w", err)
	}
	defer rows.Close()

	timestamps := make(map[string]int64)
	for rows.Next() {
		var filePath string
		var timestamp int64
		if err := rows.Scan(&filePath, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan outline timestamp: %w", err)
		}
		if normalized, err := normalizeOutlinePath(filePath); err == nil && normalized != "" {
			timestamps[normalized] = timestamp
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outline timestamps: %w", err)
	}

	return timestamps, nil
}

// Helper to convert []float32 to []byte
func float32SliceToByteSlice(floats []float32) ([]byte, error) {
	// TODO: Implement proper conversion (e.g., using binary.Write)
	// For now, a placeholder that will need to be replaced with actual serialization
	log.Println("WARNING: Using placeholder for float32SliceToByteSlice. This needs proper serialization.")
	return []byte(fmt.Sprintf("%v", floats)), nil
}

// Helper to convert []byte to []float32
func byteSliceToFloat32Slice(bytes []byte) ([]float32, error) {
	// TODO: Implement proper conversion (e.g., using binary.Read)
	// For now, a placeholder that will need to be replaced with actual deserialization
	log.Println("WARNING: Using placeholder for byteSliceToFloat32Slice. This needs proper deserialization.")
	return []float32{}, nil
}
