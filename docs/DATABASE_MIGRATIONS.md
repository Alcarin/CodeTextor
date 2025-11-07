# Database Migrations

## Overview

CodeTextor uses [golang-migrate/migrate](https://github.com/golang-migrate/migrate) for database schema management. Migrations run automatically at startup and are embedded in the binary.

**Why migrations?**
- Automatic schema upgrades when users update the app
- No data loss during updates
- Versioned, auditable changes

## Quick Start

### Adding a Migration

1. **Create SQL files** in `backend/internal/store/migrations/`:
   ```bash
   000003_add_column.up.sql    # Apply change
   000003_add_column.down.sql  # Rollback (optional)
   ```

2. **Write SQL**:
   ```sql
   ALTER TABLE projects ADD COLUMN last_indexed_at INTEGER DEFAULT 0;
   CREATE INDEX IF NOT EXISTS idx_projects_last_indexed ON projects(last_indexed_at);
   ```

3. **Test**: `go test ./backend/internal/store/...`

### Critical Rules

- **NEVER modify released migrations** - always add new ones
- Use sequential numbers (000001, 000002, ...)
- Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Add `DEFAULT` values for new columns
- Test on empty and existing databases

## How It Works

Migrations are **embedded** in the binary via `//go:embed` and run at startup:

```
1. App starts
2. ProjectStore initializes
3. runMigrations() executes pending migrations
4. schema_migrations table tracks applied versions
```

**Version tracking table:**
```sql
CREATE TABLE schema_migrations (
    version bigint PRIMARY KEY,
    dirty boolean NOT NULL
);
```

## Migration History

### v1 - Initial Schema (2025-11-06)
- Created `projects` table with basic fields
- Indexes on `name` and `created_at`

### v2 - Project Selection (2025-11-07)
- Added `is_selected INTEGER DEFAULT 0` column
- Index on `is_selected`
- **Purpose:** Database-based project selection (replaced localStorage)

## Troubleshooting

### Error: "Dirty database version X"

**Cause:** A migration failed partway through.

**Fix:**
```bash
# Check status
sqlite3 ~/.local/share/codetextor/config/projects.db
> SELECT * FROM schema_migrations;

# If dirty=1, manually fix the issue, then:
> UPDATE schema_migrations SET dirty = 0;
```

### Debugging

```bash
# View current version
sqlite3 ~/.local/share/codetextor/config/projects.db
> SELECT * FROM schema_migrations;

# View schema
> .schema projects

# List embedded migrations
ls -la backend/internal/store/migrations/
```

## SQLite Limitations

SQLite doesn't support `ALTER TABLE DROP COLUMN` (pre-3.35). Workaround:
1. Create new table without the column
2. Copy data: `INSERT INTO new_table SELECT cols FROM old_table`
3. Drop old table and rename new one

**Note:** Downgrade migrations are limited. Focus on correct forward migrations.

## References

- golang-migrate: https://github.com/golang-migrate/migrate
- SQLite ALTER TABLE: https://www.sqlite.org/lang_altertable.html
- Implementation: `backend/internal/store/migrations.go`
