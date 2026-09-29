CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Every table here is a materialized/rollup read model rebuilt from Kafka
-- events (see docs/architecture/microservices.md §11) — disposable and
-- replayable, not a source of truth. RLS policies are kept here for when
-- each service's runtime connection stops being the Postgres
-- superuser/table owner (see docs/architecture/database-schema.md's
-- "Row-Level Security pattern" section for why that's currently a no-op)
-- — every query this service runs filters by org_id explicitly in the SQL
-- itself regardless, which is the real enforcement today.

CREATE TABLE IF NOT EXISTS analytics.meeting_daily_rollup (
  org_id UUID NOT NULL,
  day DATE NOT NULL,
  meeting_count INT NOT NULL DEFAULT 0,
  total_minutes INT NOT NULL DEFAULT 0,
  PRIMARY KEY (org_id, day)
);
ALTER TABLE analytics.meeting_daily_rollup ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON analytics.meeting_daily_rollup
  USING (org_id = current_setting('app.current_org', true)::uuid);

CREATE TABLE IF NOT EXISTS analytics.action_item_rollup (
  org_id UUID NOT NULL,
  -- No "unassigned" row: an action item with no matched owner has nothing
  -- to key this table's per-owner PK on — see
  -- entity.ActionItemRollup's doc comment for the full reasoning.
  owner_user_id UUID NOT NULL,
  day DATE NOT NULL,
  opened INT NOT NULL DEFAULT 0,
  closed INT NOT NULL DEFAULT 0,
  PRIMARY KEY (org_id, owner_user_id, day)
);
ALTER TABLE analytics.action_item_rollup ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON analytics.action_item_rollup
  USING (org_id = current_setting('app.current_org', true)::uuid);

CREATE TABLE IF NOT EXISTS analytics.topic_frequency (
  org_id UUID NOT NULL,
  topic TEXT NOT NULL,
  week DATE NOT NULL,
  mentions INT NOT NULL DEFAULT 0,
  PRIMARY KEY (org_id, topic, week)
);
ALTER TABLE analytics.topic_frequency ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON analytics.topic_frequency
  USING (org_id = current_setting('app.current_org', true)::uuid);
