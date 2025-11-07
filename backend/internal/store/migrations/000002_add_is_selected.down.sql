-- Note: SQLite doesn't support DROP COLUMN directly
-- This would require recreating the table, which we avoid for safety
-- In production, we typically don't rollback this type of change

-- Drop index (this is safe)
DROP INDEX IF EXISTS idx_projects_is_selected;

-- For a proper rollback, you would need to:
-- 1. CREATE TABLE projects_backup with old schema
-- 2. INSERT INTO projects_backup SELECT id, name, description, created_at, updated_at, config_json FROM projects
-- 3. DROP TABLE projects
-- 4. ALTER TABLE projects_backup RENAME TO projects
-- But we omit this for safety - data preservation is more important
