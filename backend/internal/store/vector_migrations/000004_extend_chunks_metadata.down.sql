-- Rollback: Remove semantic metadata columns from chunks table
-- Note: SQLite doesn't support DROP COLUMN in older versions
-- We'll need to recreate the table without the new columns

-- Drop indexes first
DROP INDEX IF EXISTS idx_chunks_symbol_name;
DROP INDEX IF EXISTS idx_chunks_symbol_kind;
DROP INDEX IF EXISTS idx_chunks_language;

-- Create temporary table with original schema
CREATE TABLE chunks_backup (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER NOT NULL,
    char_start INTEGER NOT NULL,
    char_end INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Copy data back
INSERT INTO chunks_backup
SELECT id, file_path, content, embedding, line_start, line_end, char_start, char_end, created_at, updated_at
FROM chunks;

-- Drop new table and rename backup
DROP TABLE chunks;
ALTER TABLE chunks_backup RENAME TO chunks;

-- Recreate original indexes
CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
