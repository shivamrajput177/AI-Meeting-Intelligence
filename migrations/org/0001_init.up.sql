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

CREATE TABLE IF NOT EXISTS org.organizations (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  plan TEXT NOT NULL DEFAULT 'free',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org.quotas (
  org_id UUID PRIMARY KEY REFERENCES org.organizations(id),
  max_users INT NOT NULL DEFAULT 10,
  max_minutes_per_month INT NOT NULL DEFAULT 1000,
  retention_days INT NOT NULL DEFAULT 365,
  used_minutes_this_month INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS org.integration_configs (
  org_id UUID PRIMARY KEY REFERENCES org.organizations(id),
  slack_webhook_url TEXT,
  jira_base_url TEXT,
  jira_project_key TEXT,
  jira_api_token_secret_ref TEXT,
  smtp_config_secret_ref TEXT
);
