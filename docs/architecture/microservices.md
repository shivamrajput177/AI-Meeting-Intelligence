# Microservices — Low Level Design

Each service: responsibilities, APIs, owned DB schema, Kafka topics
produced/consumed, and scaling strategy. Every table below lives in the
shared Postgres cluster under the service's own schema (see
[`database-schema.md`](database-schema.md) for full DDL).

## Internal Communication: REST/JSON, no gRPC

**There is one transport for request/response calls, inside the cluster and
outside it: REST over HTTP with JSON bodies.** There's no separate internal
RPC protocol — when a service needs to call another synchronously (e.g.
Action Item Service resolving an owner's display name via User Service), it
calls that service's ClusterIP DNS name directly
(`http://user-service.meeting-intel.svc.cluster.local/...`) using the same
shared `internal/platform/httpclient` wrapper (timeouts, retries, circuit
breaker, trace-header propagation) that every outbound call uses — the same
kind of client hitting Ollama or Jira. The API Gateway forwards the public
subset of these same routes, adding auth/rate-limiting/logging on top; no
protocol translation happens anywhere, since it's REST end-to-end. This
keeps one HTTP stack to learn, instrument, and debug instead of two, at the
cost of losing gRPC's compile-time contract checking and streaming — a
trade-off worth being able to state plainly (and its inverse — why a team
running dozens of services under real load often does want gRPC internally
for exactly the type-safety and multiplexing this project trades away) is
itself a fine interview answer.

Every service still documents its full route list below — those *are* both
its public contract (for whichever routes the gateway forwards) and its
internal one (for whichever routes another service calls directly).

---

## 1. API Gateway

**Responsibilities**: single public entry point; TLS termination (via ingress);
JWT verification (local signature/expiry check + a direct Redis lookup for
revoked-token jtis — no call to Auth Service needed for this) and
`org_id`/`role` extraction; reverse-proxying requests to the right internal
service over REST; rate limiting (Redis token bucket per user+org);
request/response logging with trace propagation; API versioning
(`/api/v1/...`).

**APIs**: reverse-proxies all REST endpoints in [`api-spec.md`](api-spec.md);
owns none of its own business data.

**Database**: none (stateless). Uses Redis for rate-limit counters and
short-lived idempotency keys.

**Kafka**: none directly (pure request/response tier).

**Scaling**: stateless Deployment, HPA on CPU + RPS (custom metric via
Prometheus Adapter), 3+ replicas, PDB `minAvailable: 2`.

---

## 2. Auth Service

**Responsibilities**: signup, login, logout, JWT access/refresh issuance and
rotation, password reset flows, refresh-token revocation, brute-force
throttling.

**REST (via gateway)**: `POST /auth/signup`, `POST /auth/login`,
`POST /auth/refresh`, `POST /auth/logout`,
`POST /auth/password/reset-request`, `POST /auth/password/reset-confirm`.
No separate "validate token" call exists — the gateway and any service
that needs to check a JWT do it locally (signature + expiry) plus a direct
Redis lookup for the revoked-jti set Auth Service maintains; nothing calls
Auth Service synchronously to ask "is this token good."

**Schema** (`auth.*`):
```
auth.credentials(user_id UUID PK, org_id UUID, password_hash TEXT,
                  algo TEXT DEFAULT 'argon2id', failed_attempts INT DEFAULT 0,
                  locked_until TIMESTAMPTZ, updated_at TIMESTAMPTZ)
auth.refresh_tokens(id UUID PK, user_id UUID, org_id UUID, token_hash TEXT,
                     issued_at TIMESTAMPTZ, expires_at TIMESTAMPTZ,
                     revoked_at TIMESTAMPTZ, replaced_by UUID, user_agent TEXT, ip INET)
auth.password_reset_tokens(id UUID PK, user_id UUID, token_hash TEXT,
                            expires_at TIMESTAMPTZ, used_at TIMESTAMPTZ)
```
Revoked/blacklisted access-token jtis cached in **Redis** (`revoked:{jti}` TTL
= remaining token life) so validation stays O(1) without a DB hit on every
request.

**Kafka produced**: `user.registered.v1`, `user.login-failed.v1` (for
security analytics/alerting).

**Scaling**: stateless, 2+ replicas; argon2id hashing is CPU-heavy — bound
concurrency with a worker pool per pod, HPA on CPU.

---

## 3. User Service

**Responsibilities**: user profile CRUD, org membership, role assignment
(RBAC), invite flow.

**REST**: `GET/PATCH /users/me`, `GET /orgs/{orgId}/users`,
`GET /orgs/{orgId}/users/{userId}` (used internally too — e.g. Action Item
Service resolving an owner's display name calls this directly),
`POST /orgs/{orgId}/invites`, `POST /invites/{token}/accept`,
`PATCH /orgs/{orgId}/users/{userId}/role`,
`DELETE /orgs/{orgId}/users/{userId}` (deactivate). User row creation
itself isn't a called endpoint — it happens inside this service's
`user.registered.v1` Kafka consumer, not via a synchronous request.

**Schema** (`user.*`):
```
user.users(id UUID PK, org_id UUID, email CITEXT UNIQUE, name TEXT,
           role TEXT CHECK (role IN ('owner','admin','manager','member','viewer')),
           status TEXT CHECK (status IN ('invited','active','deactivated')),
           avatar_url TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ)
user.invites(id UUID PK, org_id UUID, email CITEXT, role TEXT,
             token_hash TEXT, invited_by UUID, expires_at TIMESTAMPTZ,
             accepted_at TIMESTAMPTZ)
```

**Kafka produced**: `user.created.v1`, `user.role-changed.v1`,
`user.deactivated.v1`.
**Kafka consumed**: `user.registered.v1` (from Auth, to materialize the user row).

**Scaling**: stateless, 2 replicas, HPA on CPU; read-heavy → Redis cache for
`GetUser`/`ListOrgUsers` with short TTL + write-through invalidation.

---

## 4. Organization Service

**Responsibilities**: tenant lifecycle (create/suspend/delete org), plan &
quota enforcement (max users, max meeting-minutes/month, retention days),
org-level settings (integrations config: Slack webhook, Jira project, SMTP).

**REST**: `POST /orgs`, `GET /orgs/{orgId}`, `PATCH /orgs/{orgId}/settings`
(covers org settings, integration config, and — admin/ops-only — suspending
an org), `GET /orgs/{orgId}/usage` (quota usage). `GET /orgs/{orgId}` is
also called directly by other services when they need org-level config
(e.g. Notification Service reading `integration_configs`).

**Schema** (`org.*`):
```
org.organizations(id UUID PK, name TEXT, slug TEXT UNIQUE, plan TEXT
                   DEFAULT 'free', status TEXT DEFAULT 'active',
                   created_at TIMESTAMPTZ)
org.quotas(org_id UUID PK, max_users INT, max_minutes_per_month INT,
           retention_days INT, used_minutes_this_month INT DEFAULT 0)
org.integration_configs(org_id UUID PK, slack_webhook_url TEXT,
                         jira_base_url TEXT, jira_project_key TEXT,
                         jira_api_token_secret_ref TEXT, smtp_config_secret_ref TEXT)
```
Secrets (`*_secret_ref`) are pointers into K8s Secrets, never plaintext in
Postgres.

**Kafka produced**: `org.created.v1`, `org.quota-exceeded.v1`,
`org.suspended.v1`.

**Scaling**: low-traffic, 2 replicas is sufficient; no autoscaling needed at
project scale.

---

## 5. Meeting Service

**Responsibilities**: recording upload orchestration (issues MinIO presigned
PUT URL, or streams multipart upload through gateway), meeting metadata,
lifecycle/status machine (`uploaded → transcribing → transcribed →
summarizing → summarized → completed → failed`), participant tracking, kicks
off the processing pipeline by publishing to Kafka.

**REST**: `POST /meetings` (returns presigned upload URL),
`POST /meetings/{id}/complete-upload`, `GET /meetings/{id}` (also called
directly by other services that need meeting metadata/participants — e.g.
Action Item Service matching owners), `GET /meetings`,
`GET /meetings/{id}/status`, `DELETE /meetings/{id}`,
`POST /meetings/{id}/participants`.

**Schema** (`meeting.*`):
```
meeting.meetings(id UUID PK, org_id UUID, title TEXT, created_by UUID,
                  status TEXT, source_type TEXT CHECK (source_type IN
                  ('upload','zoom_import','teams_import')),
                  recording_object_key TEXT, duration_seconds INT,
                  started_at TIMESTAMPTZ, created_at TIMESTAMPTZ,
                  updated_at TIMESTAMPTZ)
meeting.participants(meeting_id UUID, user_id UUID NULL, email TEXT,
                      display_name TEXT, PRIMARY KEY (meeting_id, email))
meeting.status_history(id BIGSERIAL PK, meeting_id UUID, status TEXT,
                        changed_at TIMESTAMPTZ, note TEXT)
```

**Kafka produced**: `meeting.uploaded.v1` (key: `meeting_id`),
`meeting.status-changed.v1`, `meeting.deleted.v1`.
**Kafka consumed**: `transcription.completed.v1`, `summary.completed.v1`,
`action-item.extraction-completed.v1` (to advance the status machine).

**Scaling**: stateless, 2-3 replicas, HPA on CPU; upload traffic is
bursty — presigned-URL pattern means large file bytes bypass the service
entirely (client uploads straight to MinIO), so this service itself stays
light.

---

## 6. Transcription Service

**Responsibilities**: consumes `meeting.uploaded.v1`, pulls the recording
from MinIO, runs **whisper.cpp** (via its HTTP server: extract audio with
ffmpeg → POST to whisper.cpp `/inference`), produces a diarized transcript
(speaker segments if diarization model available, else single-speaker),
stores raw + segmented transcript, publishes completion event.

**REST**: `GET /meetings/{id}/transcript` (full transcript + segments —
also how AI Summary Service reads the transcript it consumes),
`POST /meetings/{id}/transcript/retry` (admin/internal use).

**Schema** (`transcription.*`):
```
transcription.transcripts(id UUID PK, meeting_id UUID, org_id UUID,
                           language TEXT, engine TEXT DEFAULT 'whisper.cpp',
                           model_name TEXT, status TEXT, raw_text TEXT,
                           word_count INT, created_at TIMESTAMPTZ)
transcription.segments(id BIGSERIAL PK, transcript_id UUID, speaker_label TEXT,
                        start_ms INT, end_ms INT, text TEXT, confidence REAL)
```

**Kafka produced**: `transcription.completed.v1` (key: `meeting_id`),
`transcription.failed.v1` → DLQ `transcription.completed.v1.dlq`.
**Kafka consumed**: `meeting.uploaded.v1`.

**Scaling**: this is a CPU-bound AI worker — scale via **KEDA** on
`transcription-service` consumer-group lag rather than CPU alone (a queue of
5 pending meetings should provision more pods before CPU even climbs, since
each job is long-running). Set `maxReplicas` conservatively on a laptop (2-3);
document GPU-node scaling for prod.

---

## 7. AI Summary Service

**Responsibilities**: consumes `transcription.completed.v1` (the event
itself just carries `meeting_id`/`transcript_id` — this service then calls
`GET /meetings/{id}/transcript` on Transcription Service over REST to fetch
the actual text); runs the **chunking pipeline** (splits transcript into
~500-token overlapping windows, speaker-aware); runs the **summarization
pipeline** via Ollama (structured
prompt → executive summary, key decisions, risks, blockers); publishes
`chunk.created.v1` per chunk batch (for Search Service to embed) and
`summary.completed.v1`.

**REST**: `GET /meetings/{id}/summary`, `POST /meetings/{id}/summary/regenerate`.

**Schema** (`ai.*`):
```
ai.summaries(id UUID PK, meeting_id UUID, org_id UUID, summary_text TEXT,
             key_decisions JSONB, risks JSONB, blockers JSONB,
             model_used TEXT, prompt_version TEXT, created_at TIMESTAMPTZ)
ai.chunks(id UUID PK, meeting_id UUID, org_id UUID, chunk_index INT,
          text TEXT, token_count INT, start_ms INT, end_ms INT,
          created_at TIMESTAMPTZ)
```
(Embeddings for these chunks are stored by **Search Service**, not here —
keeps the LLM-output-writer and the vector-index-writer decoupled.)

**Kafka produced**: `chunk.created.v1` (key: `meeting_id`),
`summary.completed.v1`, `summary.failed.v1`.
**Kafka consumed**: `transcription.completed.v1`.

**Scaling**: KEDA on consumer lag, same rationale as Transcription Service.
Circuit breaker + timeout around Ollama calls; retry with backoff, DLQ after
N attempts.

---

## 8. Action Item Service

**Responsibilities**: consumes `transcription.completed.v1` (or
`summary.completed.v1` — configurable, summary-based extraction is cheaper),
prompts the LLM with a structured-output schema to extract action items,
decisions, risks, and blockers with best-guess owners (matched against
`meeting.participants`), assigns due dates when mentioned, tracks completion
status, and is the source of truth the Notification Service polls for
reminders and the one that creates Jira tickets from.

**REST**: `GET /meetings/{id}/action-items`, `GET /action-items?owner=&status=`,
`GET /action-items/{id}`, `PATCH /action-items/{id}` (covers status update
and owner reassignment), `POST /action-items/{id}/jira-ticket` (publishes
`action-item.jira-requested.v1`, picked up by Notification Service — see
**Ticketing** in `api-spec.md`).

**Schema** (`actionitem.*`):
```
actionitem.action_items(id UUID PK, meeting_id UUID, org_id UUID,
                         description TEXT, type TEXT CHECK (type IN
                         ('action','decision','risk','blocker')),
                         owner_user_id UUID NULL, owner_raw_name TEXT,
                         due_date DATE NULL, status TEXT DEFAULT 'open'
                         CHECK (status IN ('open','in_progress','done','cancelled')),
                         priority TEXT DEFAULT 'medium', jira_issue_key TEXT NULL,
                         extracted_from_chunk_id UUID, confidence REAL,
                         created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ)
actionitem.reminders(id UUID PK, action_item_id UUID, remind_at TIMESTAMPTZ,
                      sent_at TIMESTAMPTZ NULL, channel TEXT)
```

**Kafka produced**: `action-item.extracted.v1` (batch per meeting, key:
`meeting_id`), `action-item.status-changed.v1`, `action-item.jira-requested.v1`.
**Kafka consumed**: `summary.completed.v1`.

**Scaling**: KEDA on consumer lag; extraction is a single LLM call per
meeting (cheap relative to transcription), 2-3 replicas typically enough.

---

## 9. Search Service (Knowledge Search / RAG)

**Responsibilities**: consumes `chunk.created.v1`, calls Ollama's embedding
model, stores vectors in `pgvector`; exposes semantic search, "similar
meetings", and RAG Q&A (retrieve top-k chunks across the org's meeting
history → build grounded prompt → call Ollama LLM → return answer +
source citations with meeting/timestamp links).

**REST**: `GET /search?q=`, `GET /meetings/{id}/similar`, `POST /qa/ask`,
`POST /search/reindex` (admin-only, full org backfill).

**Schema** (`search.*`):
```
search.chunk_embeddings(chunk_id UUID PK, meeting_id UUID, org_id UUID,
                         embedding VECTOR(768), model_name TEXT,
                         created_at TIMESTAMPTZ)
-- ivfflat or HNSW index, filtered by org_id (partial indexes per large tenant
-- if needed):
CREATE INDEX ON search.chunk_embeddings USING hnsw (embedding vector_cosine_ops);

search.qa_history(id UUID PK, org_id UUID, user_id UUID, question TEXT,
                   answer TEXT, cited_chunk_ids UUID[], created_at TIMESTAMPTZ)
```

**Kafka produced**: `embedding.completed.v1`.
**Kafka consumed**: `chunk.created.v1`.

**RAG flow**: embed query → `SELECT ... ORDER BY embedding <=> $1 LIMIT k
WHERE org_id = $tenant` (RLS-enforced too) → assemble context with
`[meeting_title, timestamp]` citations inline → Ollama chat completion with a
grounding system prompt → parse answer + map cited chunk_ids back to
meeting/timestamp for the response's `citations[]`.

**Scaling**: KEDA on `chunk.created.v1` lag for the embedding path; the
REST endpoints (`/search`, `/qa/ask`) are synchronous read paths — scale
via standard HPA on those service pods (a separate Deployment from the
Kafka-consumer half if load profiles diverge — read path is
latency-sensitive, embed path is throughput-oriented).

---

## 10. Notification Service

**Responsibilities**: Slack notifications, email notifications (SMTP —
Mailhog locally, real SMTP in prod), **ticket creation** from action items
via a pluggable `TicketProvider` (default: a self-built **mock Jira**
board, used by the public demo org so a ticket appears with no external
account needed; optional: a real Atlassian Jira Cloud provider for
interview demos — see `deployment-demo-strategy.md` §3 for the adapter
design), and the **reminder scheduler** (poll-based, see design decision in
`PROJECT_PLAN.md` §5).

This service has no synchronous public surface beyond the routes below — it
acts entirely on Kafka events and its own scheduler tick, dispatching to
Slack/Email/Jira/the mock board itself.

**REST**: `POST /orgs/{orgId}/integrations/test` (admin/debug), plus the
mock-board routes in `api-spec.md` §Ticketing (`GET /demo/board`,
`GET /orgs/{orgId}/mock-jira/board`, `PATCH .../mock-jira/issues/{issueKey}`).

**Schema** (`notification.*`):
```
notification.outbox(id UUID PK, org_id UUID, channel TEXT CHECK (channel IN
                     ('slack','email','jira')), payload JSONB,
                     status TEXT DEFAULT 'pending', attempts INT DEFAULT 0,
                     last_error TEXT, created_at TIMESTAMPTZ, sent_at TIMESTAMPTZ)
notification.jira_links(action_item_id UUID PK, org_id UUID, provider TEXT
                         DEFAULT 'mock_jira', jira_issue_key TEXT, jira_url TEXT,
                         created_at TIMESTAMPTZ)
notification.mock_jira_issues(id UUID PK, org_id UUID, action_item_id UUID,
                               issue_key TEXT, project_key TEXT DEFAULT 'DEMO',
                               title TEXT, status TEXT DEFAULT 'To Do',
                               created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ)
```
Uses the **transactional outbox pattern**: a DB write + outbox row in one
transaction, a poller/CDC-style dispatcher publishes to Kafka or calls the
external API — avoids dual-write inconsistency between Postgres and
Slack/Jira calls. Full schema for the mock board in
`deployment-demo-strategy.md` §3.

**Kafka produced**: `notification.sent.v1`, `notification.failed.v1`,
`action-item.reminder-due.v1` (from the scheduler poller).
**Kafka consumed**: `action-item.extracted.v1` (→ Slack "new action items"
digest), `action-item.jira-requested.v1`, `summary.completed.v1` (→ "meeting
summary ready" notification), `action-item.reminder-due.v1` (self-consumed →
dispatch).

**Reminder scheduler**: a leader-elected (Postgres advisory lock
`pg_try_advisory_lock`) goroutine ticking every 60s: `SELECT ... FROM
actionitem.reminders WHERE remind_at <= now() AND sent_at IS NULL FOR UPDATE
SKIP LOCKED` → publish `action-item.reminder-due.v1` → mark sent.

**Scaling**: stateless dispatch consumers scale on Kafka lag; exactly one
replica runs the scheduler tick loop at a time (advisory lock), but any
replica can win the lock — no dedicated singleton pod needed.

---

## 11. Analytics Service

**Responsibilities**: builds **event-sourced read models** by consuming
domain events from every other service — meeting trends, team productivity
metrics, action-item completion rate, most-discussed topics (derived from
`ai.chunks`/summary keyword extraction) — and serves pre-aggregated
dashboards without hitting operational tables (CQRS read-side).

**REST**: `GET /analytics/meetings/trends`, `GET /analytics/productivity`,
`GET /analytics/action-items/completion-rate`, `GET /analytics/topics`.

**Schema** (`analytics.*`, materialized/rollup tables, rebuilt from events —
disposable/replayable):
```
analytics.meeting_daily_rollup(org_id UUID, day DATE, meeting_count INT,
                                total_minutes INT, PRIMARY KEY (org_id, day))
analytics.action_item_rollup(org_id UUID, owner_user_id UUID, day DATE,
                              opened INT, closed INT,
                              PRIMARY KEY (org_id, owner_user_id, day))
analytics.topic_frequency(org_id UUID, topic TEXT, week DATE, mentions INT,
                           PRIMARY KEY (org_id, topic, week))
```

**Kafka consumed**: `meeting.status-changed.v1`, `action-item.status-changed.v1`,
`summary.completed.v1`, `action-item.extracted.v1` — all topics, own
consumer group `analytics-service`, offsets independent of every other
consumer so replay/backfill never affects operational services.

**Scaling**: pure consumer, KEDA on lag; since rollups are idempotent
upserts keyed by `(org_id, day/week, ...)`, at-least-once delivery is safe
without a dedup table.

---

## Cross-Service Scaling Summary

| Service | Workload shape | Scaling mechanism |
|---|---|---|
| API Gateway | sync, latency-sensitive | HPA (CPU + RPS) |
| Auth / User / Org | sync, low volume | HPA (CPU), 2 replicas floor |
| Meeting Service | sync, bursty uploads | HPA (CPU), presigned-URL offload |
| Transcription | async, CPU/GPU-bound, long jobs | **KEDA** on Kafka lag |
| AI Summary | async, LLM-bound | **KEDA** on Kafka lag |
| Action Item | async, LLM-bound (light) | **KEDA** on Kafka lag |
| Search | mixed (sync query + async embed) | split into 2 Deployments, HPA + KEDA respectively |
| Notification | async dispatch + poll loop | HPA (CPU) + Kafka lag |
| Analytics | pure async consumer | **KEDA** on Kafka lag |
