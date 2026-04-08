-- Rimuovi la colonna size_bytes (richiede SQLite 3.35.0+)
ALTER TABLE files DROP COLUMN size_bytes;
