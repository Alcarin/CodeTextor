-- Add size_bytes column to files table
ALTER TABLE files ADD COLUMN size_bytes INTEGER DEFAULT 0;
