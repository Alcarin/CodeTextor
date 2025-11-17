-- Create new files table with integer primary key
CREATE TABLE files_new (
    pk INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL UNIQUE,
    hash TEXT NOT NULL,
    last_modified INTEGER NOT NULL,
    chunk_count INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO files_new (id, path, hash, last_modified, chunk_count, created_at, updated_at)
SELECT id, path, hash, last_modified, chunk_count, created_at, updated_at FROM files;

-- Insert placeholder file rows for any orphaned paths
WITH existing_paths AS (
    SELECT path FROM files
),
all_paths AS (
    SELECT DISTINCT file_path AS path FROM chunks
    UNION
    SELECT DISTINCT file_path AS path FROM symbols
    UNION
    SELECT DISTINCT file_path AS path FROM file_outlines
),
missing_paths AS (
    SELECT path FROM all_paths
    EXCEPT
    SELECT path FROM existing_paths
)
INSERT INTO files_new (id, path, hash, last_modified, chunk_count, created_at, updated_at)
SELECT 'legacy-' || lower(hex(randomblob(8))), path, 'unknown', 0, 0, strftime('%s','now'), strftime('%s','now') FROM missing_paths;

-- Create normalized chunks table referencing files by pk
CREATE TABLE chunks_new (
    id TEXT PRIMARY KEY,
    file_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER NOT NULL,
    char_start INTEGER NOT NULL,
    char_end INTEGER NOT NULL,
    language TEXT,
    symbol_name TEXT,
    symbol_kind TEXT,
    parent TEXT,
    signature TEXT,
    visibility TEXT,
    package_name TEXT,
    doc_string TEXT,
    token_count INTEGER,
    is_collapsed BOOLEAN DEFAULT 0,
    source_code TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files_new(pk) ON DELETE CASCADE
);

INSERT INTO chunks_new (
    id, file_id, content, embedding, line_start, line_end, char_start, char_end,
    language, symbol_name, symbol_kind, parent, signature, visibility,
    package_name, doc_string, token_count, is_collapsed, source_code,
    created_at, updated_at
)
SELECT
    c.id,
    f.pk,
    c.content,
    c.embedding,
    c.line_start,
    c.line_end,
    c.char_start,
    c.char_end,
    c.language,
    c.symbol_name,
    c.symbol_kind,
    c.parent,
    c.signature,
    c.visibility,
    c.package_name,
    c.doc_string,
    c.token_count,
    c.is_collapsed,
    c.source_code,
    c.created_at,
    c.updated_at
FROM chunks c
JOIN files_new f ON f.path = c.file_path;

-- Create normalized symbols table referencing files by pk
CREATE TABLE symbols_new (
    id TEXT PRIMARY KEY,
    file_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    line INTEGER NOT NULL,
    character INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files_new(pk) ON DELETE CASCADE
);

INSERT INTO symbols_new (id, file_id, name, kind, line, character, created_at, updated_at)
SELECT s.id, f.pk, s.name, s.kind, s.line, s.character, s.created_at, s.updated_at
FROM symbols s
JOIN files_new f ON f.path = s.file_path;

-- Create mapping table between chunks and symbols
CREATE TABLE chunk_symbols (
    chunk_id TEXT NOT NULL,
    symbol_id TEXT NOT NULL,
    PRIMARY KEY (chunk_id, symbol_id),
    FOREIGN KEY(chunk_id) REFERENCES chunks_new(id) ON DELETE CASCADE,
    FOREIGN KEY(symbol_id) REFERENCES symbols_new(id) ON DELETE CASCADE
);

INSERT INTO chunk_symbols (chunk_id, symbol_id)
SELECT c.id, s.id
FROM chunks_new c
JOIN symbols_new s ON c.file_id = s.file_id
WHERE s.line BETWEEN c.line_start AND c.line_end;

-- Outline nodes table replaces file_outlines JSON storage
CREATE TABLE outline_nodes (
    id TEXT PRIMARY KEY,
    file_id INTEGER NOT NULL,
    parent_id TEXT,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    position INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files_new(pk) ON DELETE CASCADE,
    FOREIGN KEY(parent_id) REFERENCES outline_nodes(id) ON DELETE CASCADE
);
CREATE INDEX idx_outline_nodes_file ON outline_nodes(file_id);
CREATE INDEX idx_outline_nodes_parent ON outline_nodes(parent_id);

CREATE TABLE outline_metadata (
    file_id INTEGER PRIMARY KEY,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files_new(pk) ON DELETE CASCADE
);

DROP TABLE file_outlines;
DROP TABLE chunks;
ALTER TABLE chunks_new RENAME TO chunks;
DROP TABLE symbols;
ALTER TABLE symbols_new RENAME TO symbols;
DROP TABLE files;
ALTER TABLE files_new RENAME TO files;

CREATE INDEX idx_chunks_file_id ON chunks(file_id);
CREATE INDEX idx_chunks_symbol_name ON chunks(symbol_name);
CREATE INDEX idx_chunks_symbol_kind ON chunks(symbol_kind);
CREATE INDEX idx_chunks_language ON chunks(language);
CREATE UNIQUE INDEX idx_chunks_unique_location
ON chunks(file_id, line_start, line_end);

CREATE INDEX idx_symbols_file_id ON symbols(file_id);
CREATE INDEX idx_symbols_name ON symbols(name);
