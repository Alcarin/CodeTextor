-- Rollback migration for per-project vector database

DROP INDEX IF EXISTS idx_symbols_name;
DROP INDEX IF EXISTS idx_symbols_file_path;
DROP INDEX IF EXISTS idx_files_path;
DROP INDEX IF EXISTS idx_chunks_file_path;

DROP TABLE IF EXISTS symbols;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS chunks;
