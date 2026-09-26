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

CREATE TABLE IF NOT EXISTS actionitem.action_items (
  id UUID PRIMARY KEY,
  meeting_id UUID NOT NULL,
  org_id UUID NOT NULL,
  description TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('action','decision','risk','blocker')),
  owner_user_id UUID,
  owner_raw_name TEXT,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','done','cancelled')),
  priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low','medium','high')),
  jira_issue_key TEXT,
  extracted_from_chunk_id UUID,
  confidence REAL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- RLS policies are kept here for when each service's runtime connection
-- stops being the Postgres superuser/table owner (see
-- docs/architecture/database-schema.md's "Row-Level Security pattern"
-- section for why that's currently a no-op) — every query this service
-- runs filters by org_id explicitly in the SQL itself regardless, which
-- is the real enforcement today.
ALTER TABLE actionitem.action_items ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON actionitem.action_items
  USING (org_id = current_setting('app.current_org', true)::uuid);
CREATE INDEX IF NOT EXISTS action_items_org_owner_status_idx
  ON actionitem.action_items (org_id, owner_user_id, status);
CREATE INDEX IF NOT EXISTS action_items_meeting_idx
  ON actionitem.action_items (meeting_id);

-- actionitem.reminders (docs/architecture/database-schema.md's other
-- table for this schema) isn't created yet — nothing in Phase 2.5 writes
-- or reads it; it's Phase 4.4's Notification Service scheduler that
-- needs it, added then instead of speculatively now.
