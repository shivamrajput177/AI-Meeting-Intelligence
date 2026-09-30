-- Phase 4.2's ticket-integration tables, deferred out of 0001_init.up.sql
-- (see that file's own closing comment) — nothing wrote or read them
-- until now. See docs/architecture/deployment-demo-strategy.md §3 for
-- the full TicketProvider/MockJiraProvider design this backs.

CREATE TABLE IF NOT EXISTS notification.jira_links (
  action_item_id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  -- 'atlassian_jira' is accepted from the start even though only
  -- MockJiraProvider exists in Phase 4.2 — AtlassianJiraProvider is
  -- Phase 4.3's stretch job, and this column's value set is fixed by the
  -- design doc regardless of build order (same reasoning as 0001's
  -- 'jira' outbox channel).
  provider TEXT NOT NULL DEFAULT 'mock_jira' CHECK (provider IN ('mock_jira', 'atlassian_jira')),
  jira_issue_key TEXT NOT NULL,
  jira_url TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE notification.jira_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.jira_links
  USING (org_id = current_setting('app.current_org', true)::uuid);

CREATE TABLE IF NOT EXISTS notification.mock_jira_issues (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL,
  action_item_id UUID NOT NULL,
  issue_key TEXT NOT NULL,          -- e.g. "DEMO-142"
  project_key TEXT NOT NULL DEFAULT 'DEMO',
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'To Do'
    CHECK (status IN ('To Do', 'In Progress', 'Done')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE notification.mock_jira_issues ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.mock_jira_issues
  USING (org_id = current_setting('app.current_org', true)::uuid);
CREATE UNIQUE INDEX IF NOT EXISTS mock_jira_issues_org_key_idx
  ON notification.mock_jira_issues (org_id, issue_key);
