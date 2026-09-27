-- Serializes CREATE EXTENSION across every service's independent
-- startup migration: each service's migration runs in its own
-- transaction (see shared/dbx.RunMigrations), and
-- 'CREATE EXTENSION IF NOT EXISTS' is NOT atomic against a concurrent
-- 'IF NOT EXISTS' check from another service also starting up at the
-- same time - both can pass the existence check before either commits,
-- and the second INSERT into pg_extension's catalog then hits a real
-- unique-constraint violation. A transaction-scoped advisory lock
-- (auto-released at commit/rollback) forces these to run one at a
-- time; the shared literal key just needs to be the same across every
-- service's migration that touches shared catalog objects like
-- extensions.
SELECT pg_advisory_xact_lock(727001);

CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- Already available in the pgvector/pgvector:pg16 base image (see
-- deployments/docker-compose.yaml's postgres service comment — brought
-- in from Phase 1 specifically so this Phase 3 migration costs nothing
-- extra at the image layer).
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS search.chunk_embeddings (
  chunk_id UUID PRIMARY KEY,
  meeting_id UUID NOT NULL,
  org_id UUID NOT NULL,
  -- 768 dims matches nomic-embed-text (see
  -- deployments/configs/docker/search-service.json's ollama_embed_model)
  -- — a different embedding model would need a different column width,
  -- since pgvector's VECTOR(n) is a fixed dimension.
  embedding VECTOR(768) NOT NULL,
  model_name TEXT NOT NULL,
  -- text/start_ms/end_ms are a deliberate denormalization beyond
  -- database-schema.md's minimal schema sketch: a search hit needs to
  -- render a snippet + timestamp immediately, and re-fetching that from
  -- AI Summary Service's chunks per result on every search/ask request
  -- would turn one local index lookup into N internal HTTP round-trips.
  -- ai.chunks (AI Summary Service) stays the source of truth; this is a
  -- read-optimized copy, refreshed by ReplaceEmbeddings whenever a
  -- meeting is (re-)embedded.
  chunk_text TEXT NOT NULL,
  start_ms INT NOT NULL,
  end_ms INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- RLS policies are kept here for when each service's runtime connection
-- stops being the Postgres superuser/table owner (see
-- docs/architecture/database-schema.md's "Row-Level Security pattern"
-- section for why that's currently a no-op) — every query this service
-- runs filters by org_id explicitly in the SQL itself regardless, which
-- is the real enforcement today.
ALTER TABLE search.chunk_embeddings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON search.chunk_embeddings
  USING (org_id = current_setting('app.current_org', true)::uuid);
CREATE INDEX IF NOT EXISTS chunk_embeddings_hnsw_idx ON search.chunk_embeddings
  USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS chunk_embeddings_meeting_idx ON search.chunk_embeddings (meeting_id);

CREATE TABLE IF NOT EXISTS search.qa_history (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  user_id UUID NOT NULL,
  question TEXT NOT NULL,
  answer TEXT NOT NULL,
  -- TEXT[], not UUID[] as database-schema.md's schema sketch shows: pgx
  -- v5's default type map binds/scans Go []string against TEXT[]
  -- unambiguously with no extra type registration, whereas UUID[]
  -- support depends on registration details this sandbox has no live
  -- Postgres to verify against — chunk_id values are still UUIDs, just
  -- stored/read as their string form here.
  cited_chunk_ids TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE search.qa_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON search.qa_history
  USING (org_id = current_setting('app.current_org', true)::uuid);
CREATE INDEX IF NOT EXISTS qa_history_org_user_idx ON search.qa_history (org_id, user_id, created_at DESC);
