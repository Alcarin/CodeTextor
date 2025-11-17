-- Rollback: Remove UNIQUE constraint on chunks

DROP INDEX IF EXISTS idx_chunks_unique_location;
