ALTER TABLE chunks ADD COLUMN embedding_model_id TEXT;

-- Populate existing records with the most recent embedding model from project metadata when possible.
UPDATE chunks
SET embedding_model_id = (
    SELECT COALESCE(json_extract(config_json, '$.embeddingModel'), '')
    FROM project_meta
    LIMIT 1
)
WHERE embedding_model_id IS NULL;

-- Ensure every chunk has some identifier, even if unknown.
UPDATE chunks
SET embedding_model_id = 'unknown'
WHERE embedding_model_id IS NULL OR TRIM(embedding_model_id) = '';

CREATE INDEX IF NOT EXISTS idx_chunks_embedding_model ON chunks(embedding_model_id);
