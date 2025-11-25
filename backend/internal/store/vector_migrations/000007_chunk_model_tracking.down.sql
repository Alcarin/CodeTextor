DROP INDEX IF EXISTS idx_chunks_embedding_model;

CREATE TABLE chunks_old (
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
    FOREIGN KEY(file_id) REFERENCES files(pk) ON DELETE CASCADE
);

INSERT INTO chunks_old (
    id, file_id, content, embedding,
    line_start, line_end, char_start, char_end,
    language, symbol_name, symbol_kind, parent,
    signature, visibility, package_name, doc_string,
    token_count, is_collapsed, source_code,
    created_at, updated_at
)
SELECT
    id, file_id, content, embedding,
    line_start, line_end, char_start, char_end,
    language, symbol_name, symbol_kind, parent,
    signature, visibility, package_name, doc_string,
    token_count, is_collapsed, source_code,
    created_at, updated_at
FROM chunks;

DROP TABLE chunk_symbols;
DROP TABLE chunks;
ALTER TABLE chunks_old RENAME TO chunks;

CREATE INDEX idx_chunks_file_id ON chunks(file_id);
CREATE INDEX idx_chunks_symbol_name ON chunks(symbol_name);
CREATE INDEX idx_chunks_symbol_kind ON chunks(symbol_kind);
CREATE INDEX idx_chunks_language ON chunks(language);
CREATE UNIQUE INDEX idx_chunks_unique_location
ON chunks(file_id, line_start, line_end);

CREATE TABLE chunk_symbols (
    chunk_id TEXT NOT NULL,
    symbol_id TEXT NOT NULL,
    PRIMARY KEY (chunk_id, symbol_id),
    FOREIGN KEY(chunk_id) REFERENCES chunks(id) ON DELETE CASCADE,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
);

INSERT INTO chunk_symbols (chunk_id, symbol_id)
SELECT c.id, s.id
FROM chunks c
JOIN symbols s ON c.file_id = s.file_id
WHERE s.line BETWEEN c.line_start AND c.line_end;
