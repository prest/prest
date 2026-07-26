-- pgvector extension + deterministic 3-dim embeddings for KNN/threshold tests.
-- Requires the pgvector binaries in the server image (installed via the
-- postgres service entrypoint in integration/postgres/docker-compose.yml).
--
-- L2 distances from the query vector [1,0,0]:
--   alpha [1,0,0]     -> 0
--   delta [0.9,0.1,0] -> ~0.1414
--   beta  [0,1,0]     -> ~1.4142
--   gamma [0,0,1]     -> ~1.4142
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE vector_items(
    id serial PRIMARY KEY,
    name text,
    embedding vector(3)
);

INSERT INTO vector_items (name, embedding) VALUES
    ('alpha', '[1,0,0]'),
    ('beta',  '[0,1,0]'),
    ('gamma', '[0,0,1]'),
    ('delta', '[0.9,0.1,0]');
