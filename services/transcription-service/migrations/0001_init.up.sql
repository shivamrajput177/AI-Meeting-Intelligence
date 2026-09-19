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

CREATE TABLE IF NOT EXISTS transcription.transcripts (
  id UUID PRIMARY KEY,
  meeting_id UUID NOT NULL UNIQUE,
  org_id UUID NOT NULL,
  language TEXT,
  engine TEXT NOT NULL DEFAULT 'whisper.cpp',
  model_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'completed', 'failed')),
  raw_text TEXT NOT NULL DEFAULT '',
  word_count INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE transcription.transcripts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON transcription.transcripts
  USING (org_id = current_setting('app.current_org', true)::uuid);

CREATE TABLE IF NOT EXISTS transcription.segments (
  id BIGSERIAL PRIMARY KEY,
  transcript_id UUID NOT NULL REFERENCES transcription.transcripts(id) ON DELETE CASCADE,
  speaker_label TEXT,
  start_ms INT NOT NULL,
  end_ms INT NOT NULL,
  text TEXT NOT NULL,
  confidence REAL
);
ALTER TABLE transcription.segments ENABLE ROW LEVEL SECURITY;
-- No org_id column here (see docs/architecture/microservices.md §6's
-- schema) — tenant isolation goes through a join back to transcripts,
-- which does carry org_id. A correlated EXISTS subquery per row is more
-- expensive than a plain column comparison, but this table is read a
-- handful of rows at a time per transcript, never scanned wholesale
-- across tenants, so the cost is negligible at this project's scale.
CREATE POLICY tenant_isolation ON transcription.segments
  USING (
    EXISTS (
      SELECT 1 FROM transcription.transcripts t
      WHERE t.id = segments.transcript_id
        AND t.org_id = current_setting('app.current_org', true)::uuid
    )
  );
