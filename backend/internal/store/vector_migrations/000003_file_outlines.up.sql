-- Migration for storing per-file outline trees

CREATE TABLE IF NOT EXISTS file_outlines (
    file_path TEXT PRIMARY KEY,
    outline_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_file_outlines_path ON file_outlines(file_path);
