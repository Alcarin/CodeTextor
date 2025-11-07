-- Add is_selected column for tracking currently selected project
ALTER TABLE projects ADD COLUMN is_selected INTEGER DEFAULT 0;

-- Create index for quick lookup of selected project
CREATE INDEX IF NOT EXISTS idx_projects_is_selected ON projects(is_selected);
