-- Extend chunks table with semantic chunking metadata
-- This migration adds additional fields to store semantic chunk information

-- Add semantic metadata columns to chunks table
ALTER TABLE chunks ADD COLUMN language TEXT;
ALTER TABLE chunks ADD COLUMN symbol_name TEXT;
ALTER TABLE chunks ADD COLUMN symbol_kind TEXT;
ALTER TABLE chunks ADD COLUMN parent TEXT;
ALTER TABLE chunks ADD COLUMN signature TEXT;
ALTER TABLE chunks ADD COLUMN visibility TEXT;
ALTER TABLE chunks ADD COLUMN package_name TEXT;
ALTER TABLE chunks ADD COLUMN doc_string TEXT;
ALTER TABLE chunks ADD COLUMN token_count INTEGER;
ALTER TABLE chunks ADD COLUMN is_collapsed BOOLEAN DEFAULT 0;
ALTER TABLE chunks ADD COLUMN source_code TEXT;

-- Add index for symbol-based queries
CREATE INDEX IF NOT EXISTS idx_chunks_symbol_name ON chunks(symbol_name);
CREATE INDEX IF NOT EXISTS idx_chunks_symbol_kind ON chunks(symbol_kind);
CREATE INDEX IF NOT EXISTS idx_chunks_language ON chunks(language);
