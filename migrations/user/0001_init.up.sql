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
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS "user".users (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  email CITEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('owner','admin','manager','member','viewer')),
  status TEXT NOT NULL DEFAULT 'invited' CHECK (status IN ('invited','active','deactivated')),
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (org_id, email)
);
ALTER TABLE "user".users ENABLE ROW LEVEL SECURITY;
-- The second OR arm is a narrow, explicit "break-glass" escape hatch for
-- exactly one legitimate cross-tenant query: Auth Service's login flow
-- needs to find which org(s) an email belongs to *before* it knows the
-- org_id (see usersvc's LookupByEmail). Without this, RLS's default
-- fail-closed behavior (no app.current_org set => zero rows, not
-- all-tenants) would make login lookups always find nothing. Only that
-- one repository method ever sets app.bypass_tenant_isolation, and only
-- for its own transaction (dbx.WithBypassRLSTx) — everywhere else this
-- setting is unset, so the OR arm is false and normal tenant isolation
-- applies.
CREATE POLICY tenant_isolation ON "user".users
  USING (
    org_id = current_setting('app.current_org', true)::uuid
    OR current_setting('app.bypass_tenant_isolation', true) = 'on'
  );

CREATE TABLE IF NOT EXISTS "user".invites (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  email CITEXT NOT NULL,
  role TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  invited_by UUID NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ
);
