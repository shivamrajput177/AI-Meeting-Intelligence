-- actionitem.reminders — deferred out of 0001_init.up.sql (see that
-- file's own closing comment); Phase 4.4's Notification Service scheduler
-- is the first thing that reads it, per
-- docs/architecture/database-schema.md's schema for this table exactly.
--
-- No org_id column, by that same documented schema — same shape as
-- meeting.participants (see meeting-service's own ListParticipants doc
-- comment): every query here joins actionitem.action_items for org
-- scoping instead. RLS below is written as a join-based policy for when
-- this becomes real enforcement (see 0001's own RLS doc comment on why
-- it's a no-op today); the real, current enforcement is the explicit
-- JOIN every query already needs anyway.
CREATE TABLE IF NOT EXISTS actionitem.reminders (
  id UUID PRIMARY KEY,
  action_item_id UUID NOT NULL REFERENCES actionitem.action_items(id) ON DELETE CASCADE,
  remind_at TIMESTAMPTZ NOT NULL,
  sent_at TIMESTAMPTZ,
  channel TEXT NOT NULL DEFAULT 'slack'
);
ALTER TABLE actionitem.reminders ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON actionitem.reminders
  USING (EXISTS (
    SELECT 1 FROM actionitem.action_items a
    WHERE a.id = reminders.action_item_id
      AND a.org_id = current_setting('app.current_org', true)::uuid
  ));
CREATE INDEX IF NOT EXISTS reminders_due_idx
  ON actionitem.reminders (remind_at) WHERE sent_at IS NULL;
