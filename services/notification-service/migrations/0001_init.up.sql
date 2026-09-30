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

CREATE TABLE IF NOT EXISTS notification.outbox (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  -- 'jira' is accepted from the start even though nothing enqueues or
  -- dispatches it until Phase 4.2/4.3 — docs/architecture/microservices.md
  -- §10 documents this table's full channel set up front, and adding a
  -- CHECK value later would be a real migration; accepting it now costs
  -- nothing since an unused enum value is not unused code.
  channel TEXT NOT NULL CHECK (channel IN ('slack','email','jira')),
  payload JSONB NOT NULL,
  -- No CHECK on status: unlike channel, its value set
  -- (pending/sending/sent/failed) isn't in the documented schema and may
  -- still grow (e.g. a distinct "retrying" state) without a migration.
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at TIMESTAMPTZ
);
-- RLS policies are kept here for when each service's runtime connection
-- stops being the Postgres superuser/table owner (see
-- docs/architecture/database-schema.md's "Row-Level Security pattern"
-- section for why that's currently a no-op) — every query this service
-- runs that's scoped to one org filters by org_id explicitly in the SQL
-- itself regardless; ClaimBatch is the one cross-org query (see
-- repository/postgres's own doc comment on why), which is exactly the
-- case RLS is meant to backstop once it's real.
ALTER TABLE notification.outbox ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.outbox
  USING (org_id = current_setting('app.current_org', true)::uuid);
-- Partial index: only pending rows are ever scanned by ClaimBatch, so
-- indexing the other 99% of eventually-sent rows would be pure waste.
CREATE INDEX IF NOT EXISTS outbox_pending_idx ON notification.outbox (created_at)
  WHERE status = 'pending';

-- notification.jira_links and notification.mock_jira_issues
-- (docs/architecture/microservices.md §10's other two tables for this
-- schema) aren't created yet — nothing in Phase 4.1 writes or reads
-- them; that's Phase 4.2's (mock Jira board) and 4.3's (real Jira) job,
-- added then instead of speculatively now.
