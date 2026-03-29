-- Migration 000008: Static Analysis Support
-- Adds is_virtual flag to files and creates symbol_usages table.

-- Add is_virtual column to files table
ALTER TABLE files ADD COLUMN is_virtual BOOLEAN DEFAULT 0;

-- Create symbol_usages table for tracking references
CREATE TABLE IF NOT EXISTS symbol_usages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    caller_node_id TEXT NOT NULL,
    target_node_id TEXT,
    raw_target_name TEXT NOT NULL,
    raw_target_context TEXT,
    line INTEGER NOT NULL,
    column INTEGER NOT NULL,
    FOREIGN KEY(caller_node_id) REFERENCES outline_nodes(id) ON DELETE CASCADE,
    FOREIGN KEY(target_node_id) REFERENCES outline_nodes(id) ON DELETE SET NULL
);

-- Index for finding callers of a symbol (Call Graph)
CREATE INDEX IF NOT EXISTS idx_symbol_usages_target ON symbol_usages(target_node_id);

-- Index for finding all usages within a specific context (Outline navigation)
CREATE INDEX IF NOT EXISTS idx_symbol_usages_caller ON symbol_usages(caller_node_id);
