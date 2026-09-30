-- org.integration_configs already exists (0001_init.up.sql), but its
-- documented schema (docs/architecture/database-schema.md /
-- docs/architecture/microservices.md §4) never actually included a
-- ticket_provider column — only
-- docs/architecture/deployment-demo-strategy.md §3's prose mentions
-- "org.integration_configs.ticket_provider" as the one config field
-- selecting mock_jira vs. atlassian_jira, without the schema tables ever
-- being updated to match. This migration closes that real, if small,
-- documentation gap: Phase 4.5 is what actually needs the column to
-- exist.
ALTER TABLE org.integration_configs
  ADD COLUMN IF NOT EXISTS ticket_provider TEXT NOT NULL DEFAULT 'mock_jira'
    CHECK (ticket_provider IN ('mock_jira', 'atlassian_jira'));
