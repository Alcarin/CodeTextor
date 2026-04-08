ALTER TABLE symbols ADD COLUMN language TEXT;
CREATE INDEX idx_symbols_language ON symbols(language);
