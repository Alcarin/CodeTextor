-- Migration 000008: Static Analysis Support (Down)

-- Drop indexes first
DROP INDEX IF EXISTS idx_symbol_usages_target;
DROP INDEX IF EXISTS idx_symbol_usages_caller;

-- Drop symbol_usages table
DROP TABLE IF EXISTS symbol_usages;

-- NOTE: Dropping columns in SQLite requires recreating the table.
-- Given this is a new feature, we skip the column removal to avoid complex table recreation in down phase.
-- ALTER TABLE files DROP COLUMN is_virtual;
