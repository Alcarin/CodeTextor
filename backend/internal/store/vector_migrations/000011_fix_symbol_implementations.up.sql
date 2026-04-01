-- Recrea la tabella con i tipi corretti per file_id (INTEGER che fa riferimento a files.pk)
DROP TABLE IF EXISTS symbol_implementations;

CREATE TABLE symbol_implementations (
    id TEXT PRIMARY KEY,
    interface_name TEXT NOT NULL,
    implementor_id TEXT NOT NULL,
    file_id INTEGER NOT NULL,
    FOREIGN KEY(implementor_id) REFERENCES symbols(id) ON DELETE CASCADE,
    FOREIGN KEY(file_id) REFERENCES files(pk) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_symbol_implementations_interface ON symbol_implementations(interface_name);
