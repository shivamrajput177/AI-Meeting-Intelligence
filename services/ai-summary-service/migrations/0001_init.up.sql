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

CREATE TABLE IF NOT EXISTS ai.summaries (
  id UUID PRIMARY KEY,
  meeting_id UUID NOT NULL UNIQUE,
  org_id UUID NOT NULL,
  summary_text TEXT NOT NULL,
  key_decisions JSONB NOT NULL DEFAULT '[]',
  risks JSONB NOT NULL DEFAULT '[]',
  blockers JSONB NOT NULL DEFAULT '[]',
  model_used TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- RLS policies are kept here for when each service's runtime connection
-- stops being the Postgres superuser/table owner (see
-- docs/architecture/database-schema.md's "Row-Level Security pattern"
-- section for why that's currently a no-op) — every query this service
-- runs filters by org_id explicitly in the SQL itself regardless, which
-- is the real enforcement today.
ALTER TABLE ai.summaries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai.summaries
  USING (org_id = current_setting('app.current_org', true)::uuid);

CREATE TABLE IF NOT EXISTS ai.chunks (
  id UUID PRIMARY KEY,
  meeting_id UUID NOT NULL,
  org_id UUID NOT NULL,
  chunk_index INT NOT NULL,
  text TEXT NOT NULL,
  token_count INT NOT NULL,
  start_ms INT,
  end_ms INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (meeting_id, chunk_index)
);
ALTER TABLE ai.chunks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai.chunks
  USING (org_id = current_setting('app.current_org', true)::uuid);
