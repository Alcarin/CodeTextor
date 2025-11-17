-- Recreate legacy tables
CREATE TABLE files_old (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    hash TEXT NOT NULL,
    last_modified INTEGER NOT NULL,
    chunk_count INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO files_old (id, path, hash, last_modified, chunk_count, created_at, updated_at)
SELECT id, path, hash, last_modified, chunk_count, created_at, updated_at FROM files;

CREATE TABLE chunks_old (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
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
    updated_at INTEGER NOT NULL
);

INSERT INTO chunks_old (
    id, file_path, content, embedding, line_start, line_end, char_start, char_end,
    language, symbol_name, symbol_kind, parent, signature, visibility,
    package_name, doc_string, token_count, is_collapsed, source_code,
    created_at, updated_at
)
SELECT
    c.id,
    f.path,
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
JOIN files f ON f.pk = c.file_id;

CREATE TABLE symbols_old (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    line INTEGER NOT NULL,
    character INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO symbols_old (id, file_path, name, kind, line, character, created_at, updated_at)
SELECT s.id, f.path, s.name, s.kind, s.line, s.character, s.created_at, s.updated_at
FROM symbols s
JOIN files f ON f.pk = s.file_id;

CREATE TABLE file_outlines (
    file_path TEXT PRIMARY KEY,
    outline_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_file_outlines_path ON file_outlines(file_path);

DROP TABLE chunk_symbols;
DROP TABLE outline_nodes;
DROP TABLE outline_metadata;
DROP TABLE chunks;
ALTER TABLE chunks_old RENAME TO chunks;
DROP TABLE symbols;
ALTER TABLE symbols_old RENAME TO symbols;
DROP TABLE files;
ALTER TABLE files_old RENAME TO files;

CREATE INDEX idx_chunks_file_path ON chunks(file_path);
CREATE INDEX idx_chunks_symbol_name ON chunks(symbol_name);
CREATE INDEX idx_chunks_symbol_kind ON chunks(symbol_kind);
CREATE INDEX idx_chunks_language ON chunks(language);
CREATE UNIQUE INDEX idx_chunks_unique_location
ON chunks(file_path, line_start, line_end);

CREATE INDEX idx_symbols_file_path ON symbols(file_path);
CREATE INDEX idx_symbols_name ON symbols(name);
