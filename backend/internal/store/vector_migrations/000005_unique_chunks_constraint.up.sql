-- Migration: Add UNIQUE constraint to prevent duplicate chunks
-- This ensures that chunks cannot be duplicated during concurrent indexing operations

-- First, remove any existing duplicates by keeping only the most recent version of each chunk
-- (identified by file_path + line_start + line_end combination)
DELETE FROM chunks
WHERE id NOT IN (
    SELECT MIN(id)
    FROM chunks
    GROUP BY file_path, line_start, line_end
);

-- Create a unique index to prevent future duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_unique_location
ON chunks(file_path, line_start, line_end);
