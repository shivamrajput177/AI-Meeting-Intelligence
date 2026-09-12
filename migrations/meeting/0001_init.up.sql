-- Serializes CREATE EXTENSION across every service's independent
-- startup migration: each service's migration runs in its own
-- transaction (see internal/platform/dbx.RunMigrations), and
-- 'CREATE EXTENSION IF NOT EXISTS' is NOT atomic against a concurrent
-- 'IF NOT EXISTS' check from another service also starting up at the
-- same time - both can pass the existence check before either commits,
-- and the second INSERT into pg_extension's catalog then hits a real
-- unique-constraint violation. A transaction-scoped advisory lock
-- (auto-released at commit/rollback) forces these to run one at a
-- time; the shared literal key just needs to be the same across every
-- service's migration that touches shared catalog objects like
-- extensions.
SELECT pg_advisory_xact_lock(727001);

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS meeting.meetings (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  title TEXT NOT NULL,
  created_by UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'uploaded' CHECK (status IN
    ('uploaded','transcribing','transcribed','summarizing','summarized',
     'completed','failed')),
  source_type TEXT NOT NULL DEFAULT 'upload' CHECK (source_type IN
    ('upload','zoom_import','teams_import')),
  recording_object_key TEXT NOT NULL,
  duration_seconds INT,
  started_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE meeting.meetings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON meeting.meetings
  USING (org_id = current_setting('app.current_org', true)::uuid);
CREATE INDEX IF NOT EXISTS meetings_org_created_idx ON meeting.meetings (org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS meeting.participants (
  meeting_id UUID NOT NULL REFERENCES meeting.meetings(id) ON DELETE CASCADE,
  user_id UUID,
  email TEXT NOT NULL,
  display_name TEXT,
  PRIMARY KEY (meeting_id, email)
);

CREATE TABLE IF NOT EXISTS meeting.status_history (
  id BIGSERIAL PRIMARY KEY,
  meeting_id UUID NOT NULL REFERENCES meeting.meetings(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  note TEXT
);
