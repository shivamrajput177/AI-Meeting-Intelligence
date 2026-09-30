# AI-Meeting-Intelligence

AI-powered Meeting Intelligence Platform built with Golang, Kafka, PostgreSQL, pgvector, Kubernetes, and local LLMs. Transcribes meetings, generates summaries, extracts action items, enables semantic search, and transforms conversations into a searchable organizational knowledge base.

## Design & Planning Docs

The full system design lives under [`docs/`](docs/PROJECT_PLAN.md):

- [`docs/PROJECT_PLAN.md`](docs/PROJECT_PLAN.md) — executive summary, HLD diagram, tech stack, key design decisions
- **[`docs/ROADMAP.md`](docs/ROADMAP.md) — the phase-wise development plan.**
  All feature work happens here, broken into 7 phases (MVP → AI processing →
  search/RAG → integrations → Kubernetes/CI-CD → production readiness →
  public demo), and each phase is further split into small sub-phases —
  one service or feature at a time (e.g. Phase 2 = 2.1 RBAC, 2.2 Kafka
  infra, 2.3 Transcription Service, 2.4 AI Summary Service, …). Start at
  the **Phase & Sub-Phase Index** table at the top of that file.
- [`docs/architecture/`](docs/architecture/) — per-service LLD, database schema, Kafka topic design & event flows, REST API spec, Kubernetes/Helm/CI-CD/ArgoCD, observability/security/multi-tenancy/DR/cost, deployment & demo strategy

All APIs — external and internal service-to-service alike — are plain
hand-written REST/JSON; there's no gRPC or protobuf codegen anywhere in
this design (see `docs/architecture/microservices.md` §"Internal
Communication" for why).

## Status: Phase 1 (MVP) implemented, Phase 2 (2.1–2.7) fully implemented, Phase 3 (3.1–3.5) fully implemented, Phase 4.1–4.2 implemented

Auth, User, Organization, and Meeting services, the API Gateway, and a
React web app are built and running — signup, login, JWT refresh/rotation,
logout, password reset, tenant isolation, org/user profile management, and
meeting upload via presigned MinIO URLs all work end-to-end. See
`docs/ROADMAP.md`'s Phase 1 section for exact scope.

Phase 2.1's full RBAC is also in: `admin`/`manager`/`viewer` roles beyond
Phase 1's `owner`/`member`, org invites (`POST /orgs/{orgId}/invites` →
`POST /invites/{token}/accept`), role changes, and user deactivation — all
enforced at two layers (API Gateway pre-check + each service's own
re-check, per `docs/architecture/observability-security.md` §2).

Phase 2.2's Kafka infra is in too: a single-node KRaft broker in
`deployments/docker-compose.yaml` plus a one-shot topic-creation service
(`deployments/kafka-init/`) covering every topic Phase 2's services
produce — see `docs/architecture/kafka-topics.md`'s "Local dev infra"
section.

Phase 2.3's Transcription Service is built and wired end-to-end: Meeting
Service publishes `meeting.uploaded.v1` on confirmed upload (best-effort —
see `usecase.ConfirmUploadUseCase`'s doc comment for the known
no-outbox-yet trade-off), Transcription Service consumes it, calls a real
whisper.cpp server over HTTP, persists the transcript + segments, and
publishes `transcription.completed.v1`/`transcription.failed.v1`; `GET
/meetings/{id}/transcript` is live behind the gateway.

Phase 2.4's AI Summary Service is in too: it consumes
`transcription.completed.v1`, fetches the transcript from Transcription
Service's new internal endpoint (`GET
/internal/meetings/{id}/transcript`, guarded the same
shared-secret-token way every other `/internal/*` route in this repo is),
splits it into speaker-aware ~500-word chunks with overlap, summarizes it
via a real Ollama server (structured JSON prompt → executive summary, key
decisions, risks, blockers), persists both, and publishes
`chunk.created.v1`/`summary.completed.v1`/`summary.failed.v1`. `GET
/meetings/{id}/summary` and `POST /meetings/{id}/summary/regenerate` are
live behind the gateway — regenerate just re-runs the same pipeline
synchronously.

Phase 2.5's Action Item Service is in as well: it consumes
`summary.completed.v1`, fetches the summary from AI Summary Service's new
internal endpoint (`GET /internal/meetings/{id}/summary`) and the
meeting's participant list from Meeting Service's new internal endpoint
(`GET /internal/meetings/{id}/participants`) — both guarded the same
shared-secret-token way every other `/internal/*` route in this repo is —
runs a structured-extraction prompt against Ollama for actionable items
(with a best-guess owner matched against those participants and a due
date when one's mentioned), turns the summary's own already-extracted key
decisions/risks/blockers into rows alongside them (no second LLM call
needed for those), persists the batch, and publishes
`action-item.extracted.v1`/`action-item.extraction-failed.v1`. `GET
/meetings/{id}/action-items`, `GET /action-items`, `GET
/action-items/{id}`, and `PATCH /action-items/{id}` (status/owner/due-date
updates, restricted to the item's own owner or an org owner/admin) are
live behind the gateway.

Phase 2.6's status-machine wiring is in too: Meeting Service now consumes
`transcription.completed.v1`/`transcription.failed.v1`,
`summary.completed.v1`/`summary.failed.v1`, and
`action-item.extracted.v1`/`action-item.extraction-failed.v1` — one
generic consumer loop (`consumer.go`'s `statusConsumers` table) maps each
straight to a status transition
(`transcribed`/`summarized`/`completed`, or `failed` from any stage) —
and publishes `meeting.status-changed.v1` every time a meeting's status
actually changes, whether that came from one of these events or the
Phase 1 manual `PATCH /meetings/{id}/status` debug endpoint (still
available, still owner-only). Honest gap: `transcribing`/`summarizing`
specifically are never set by anything, since no service publishes a
"just started" event for either stage — only completion/failure — so a
meeting currently jumps straight from `uploaded` to `transcribed` with
nothing in between.

Phase 2.7's basic observability closes out Phase 2: structured
(`key=value`) logging via `shared/logger` was already universal from
Phase 1, so the actual gap here was distributed tracing across the Kafka
boundary specifically — the thing the roadmap calls out as "much harder
to retrofit" than to build now. `shared/kafkax` hand-generates and parses
a W3C `traceparent` string (`00-{trace-id}-{span-id}-01`) rather than
pulling in the full OTel SDK: there's no Collector/Tempo backend to send
real spans to yet (that's Phase 6, once one exists), but the wire format
is the same either way, so adopting real OTel later needs no
producer/consumer changes, just a real span recorded behind the same
header. `meeting-service`'s `ConfirmUploadUseCase` mints a fresh trace
the moment a meeting is confirmed uploaded; every Kafka message published
anywhere downstream of it (by Transcription, AI Summary, Action Item, or
Meeting Service's own status-changed events) carries a `ChildTraceparent`
of that same trace-id as a message header, and every consumer logs it
as `trace_id=...` — so `grep trace_id=<id>` across all four services'
logs shows one meeting's entire pipeline run, in order, with zero
backend required to view it. Internal HTTP calls (e.g. Action Item
Service fetching a summary from AI Summary Service) aren't part of this
propagation chain yet — that's full request/async instrumentation
end-to-end, which is Phase 6's job once OTel's actual SDK is in the
picture.

Phase 3.1 through 3.4 (Search & RAG) are in as well, as a new Search
Service: it consumes `chunk.created.v1`, fetches each chunk's text from
AI Summary Service's new internal endpoint (`GET
/internal/meetings/{id}/chunks`), embeds it via a real Ollama server
running `nomic-embed-text` (768 dimensions), and stores the vectors in
`pgvector` (already available from Phase 1 — see the `postgres` service's
own comment in `docker-compose.yaml`), publishing
`embedding.completed.v1`/`embedding.failed.v1`. `GET /search?q=` and
`POST /qa/ask` both embed the query the same way and rank
`search.chunk_embeddings` by cosine distance (`<=>`); `GET
/meetings/{id}/similar` averages a meeting's own chunk embeddings into a
centroid (in Go, not a SQL `avg(vector)` — see
`SimilarMeetingsUseCase`'s doc comment for why) and ranks every other
meeting in the org by its closest-matching chunk to that centroid.
`POST /qa/ask` assembles a grounded prompt with `[meeting_title,
timestamp]` citations from the retrieved chunks (meeting titles resolved
via Meeting Service's own new internal endpoints, `GET
/internal/meetings/{id}` and `GET /internal/meetings`), calls Ollama for
the answer, and persists the exchange to `GET /qa/history`. `POST
/search/reindex` (owner/admin-only) re-embeds every meeting in the org on
demand — best-effort per meeting, so one meeting with nothing to embed
yet doesn't abort the whole backfill.

Two honest simplifications, stated plainly rather than glossed over:
every retrieved chunk is treated as "cited" in an answer (no
function-calling or citation-marker parsing to narrow that down to only
the chunks the model actually drew on), and `POST /search/reindex`
enumerates at most one page of meetings (100, Meeting Service's own
`ListMeetingsUseCase` page-size cap) — an org with more than that needs
more than one reindex call today.

Phase 3.5's Analytics Service closes out Phase 3: a pure Kafka consumer
(its own `analytics-service` consumer group, offsets independent of every
other service's) that rolls four already-flowing topics —
`meeting.status-changed.v1`, `action-item.extracted.v1`,
`action-item.status-changed.v1`, and `summary.completed.v1` — into three
disposable/replayable rollup tables (`analytics.meeting_daily_rollup`,
`analytics.action_item_rollup`, `analytics.topic_frequency`), the
CQRS-style read model `docs/architecture/microservices.md` §11 describes.
Building this also meant finishing a gap Phase 2.5 left open:
`action-item.status-changed.v1` had been created by `kafka-init` and
documented as Action Item Service's since Phase 2.5, but nothing actually
published to it until now — `UpdateActionItemUseCase` publishes it on any
PATCH that changes an item's status, keyed by `action_item_id` per its
documented partitioning. `GET /analytics/meetings/trends`, `GET
/analytics/productivity`, `GET /analytics/action-items/completion-rate`,
and `GET /analytics/topics` are live behind the gateway, gated
`manager+`(`owner`/`admin`/`manager`) at both the gateway and the
service's own re-check.

Three honest simplifications, stated plainly rather than glossed over:
`analytics.action_item_rollup`'s schema keys "opened"/"closed" counts by
`(org_id, owner_user_id, day)` with no "unassigned" bucket, so an action
item extracted with no matched owner is never rolled up into anyone's
productivity numbers; `analytics.topic_frequency`'s "topic" extraction is
a keyword-proxy, not real NLP keyword/entity extraction — each of a
summary's key decisions/risks/blockers becomes its own (lowercased,
trimmed) topic string, so "the team needs to finalize the vendor contract
by Friday" is one topic, not "vendor contract"; and every rollup write is
an idempotent upsert with no separate dedup table, so at-least-once Kafka
delivery redelivering the same event double-counts that rollup — an
accepted trade-off for a read model that's explicitly disposable and
replayable, not a source of truth.

Phase 4.1's Notification Service starts Phase 4 (Integrations &
Automation): it consumes `summary.completed.v1` and
`action-item.extracted.v1`, and turns each into a durable
`notification.outbox` row (the **transactional outbox pattern** —
docs/architecture/microservices.md §10) before ever touching a real
external system. A separate poller goroutine (any replica can run it —
`FOR UPDATE SKIP LOCKED` makes that safe with no leader election, unlike
Phase 4.4's reminder scheduler) claims pending rows and dispatches them
for real: a Slack Incoming Webhook POST for "meeting summarized"/"action
items digest" messages, and a real SMTP send (to a local
[Mailhog](https://github.com/mailhog/MailHog) instance, viewable at
http://localhost:8025) for a "your summary is ready" email to the
meeting's creator — resolved via two new internal endpoints, Meeting
Service's existing `GET /internal/meetings/{id}` (title + `createdBy`) and
a new `GET /internal/users/{id}` on User Service (email). A failed
dispatch attempt retries with backoff (approximated from the row's
`created_at` and `attempts` — see `repository/postgres`'s own doc comment
on why a real implementation would want an extra `updated_at` column) up
to 5 attempts, then gives up and publishes `notification.failed.v1` — both
`notification.sent.v1` and `notification.failed.v1` were already
documented in `kafka-topics.md` since earlier phases but never actually
created or published until now, the same kind of gap Phase 2.6 found and
fixed for `meeting.status-changed.v1`.

Phase 4.2 adds mock Jira ticketing behind a `TicketProvider` interface
(`ticketprovider.Provider`), per
`docs/architecture/deployment-demo-strategy.md` §3's adapter design: a
real `AtlassianJiraProvider` (Phase 4.3, stretch) will implement the same
interface against Jira Cloud's REST API without `DispatchUseCase`'s
dispatch logic changing at all — only which provider `main.go`
constructs. `POST /action-items/{id}/jira-ticket` (new on Action Item
Service) publishes `action-item.jira-requested.v1`, which Notification
Service turns into a `channel=jira` outbox row the same poller now
dispatches for real: `MockJiraProvider` creates a
`notification.mock_jira_issues` row with a sequential per-org issue key
(`DEMO-1`, `DEMO-2`, ...), records the provider-agnostic
`notification.jira_links` mapping, and writes the resulting issue key
back onto the action item via a new `PATCH /internal/action-items/{id}`.
The mock board is genuinely bidirectional, not just a read-only mirror:
`GET /demo/board` (public, no login — a real Jira Cloud board can't offer
that to a stranger clicking a resume link) and `GET
/orgs/{orgId}/mock-jira/board` (member+) show it, and `PATCH
/orgs/{orgId}/mock-jira/issues/{issueKey}` (member+, moving a card
between columns) writes the mapped status (`To Do`/`In
Progress`/`Done` → `open`/`in_progress`/`done`) straight back onto the
linked action item — "the same status-changed event a real Jira
transition would" fire, with no webhook needed today because there's no
real Jira in the loop yet.

Three honest simplifications here too: `CreateMockIssue`'s per-org
sequential numbering is a `SELECT MAX(...) + 1` subquery, not a real
sequence or advisory lock, so two concurrent ticket creations for the
same org could race (the table's unique index turns that into a retried
error, not a silent duplicate — an acceptable trade-off at demo scale,
not a real ticketing system's guarantee); the write-back onto the action
item (issue key after creation, status after a board transition) is
best-effort — logged on failure, never rolled back, so a down Action
Item Service leaves the mock board and the action item's own state
briefly out of sync rather than losing the ticket/transition itself; and
the Slack webhook URL/SMTP settings/ticket provider are still a single
dev-config-wide value, not per-org — Phase 4.5's Organization Service
integration config is what makes any of this per-tenant.

**Not live-verified in this sandbox, for 2.3 through 4.2**: the egress
proxy here blocks all container-registry traffic, so the Kafka broker, a
real whisper.cpp server, and a real Ollama server have never actually
been run — the business logic (including the chunking, owner-matching,
status-mapping, trace propagation, and centroid/citation logic) is
verified by unit test with fakes, and each service's REST+Postgres read
path is verified against a seeded row; the docker-compose config itself
is unverified past `docker compose config` syntax validation. `pgvector`
specifically has an extra unverified edge: which minor version the
`pgvector/pgvector:pg16` image bundles has never been confirmed against a
running instance, which is exactly why `SimilarMeetingsUseCase` computes
its centroid in Go rather than relying on a SQL vector aggregate that
only exists in pgvector >= 0.5. See
`docs/architecture/database-schema.md`'s "Row-Level Security pattern"
section for a related, now-fixed finding: every repository query in this
project filters by `org_id` explicitly rather than relying on Postgres
RLS, which turned out to be silently inert (every service connects as
the table owner/superuser, which RLS never applies to). **Phase 2 and
Phase 3 (3.1–3.5) are now fully implemented, and Phase 4.1–4.2
(Notification Service core + mock Jira ticketing) are in** — Phase 4.3
(real Jira, stretch) or 4.4 (reminder scheduler) is next; giving each
service's DB connection its own non-superuser role so RLS becomes real
defense-in-depth again remains an open follow-up, not tied to any one
phase.

The backend is a **Go workspace** (`go.work` at the repo root): `shared/`
is its own Go module with no `internal/` in its path, so every
`services/<name>/` module — each with its own `go.mod` — can import it
across module boundaries; `go.work` then lets `go build`/`go test` see
all of them locally without a real `github.com/...` release for `shared`.
See `docs/architecture/folder-structure.md` for the full layout and why
an earlier single-module design (with `shared/` still under `internal/`)
couldn't do this.

### Quickstart

```bash
make up
```

(equivalent to `docker compose -f deployments/docker-compose.yaml up --build`)

This starts Postgres (with `pgvector` pre-installed for Phase 3), Redis,
MinIO, Kafka, Ollama, whisper.cpp, Mailhog, all eleven backend services,
and the web app — including two one-shot init steps that make this a
genuine
single-command bring-up: `whisper-model-init` downloads whisper.cpp's
`ggml-base.en.bin` (~140MB) into `deployments/whisper-models/` before
`whisper` starts, and `ollama-model-init` pulls every model this stack's
services call against Ollama — `qwen2.5:7b` (AI Summary Service, Action
Item Service, and Search Service's RAG answering) and `nomic-embed-text`
(Search Service's embedding pipeline) — before those services start. Both
init steps are idempotent — safe to leave in place on every `make up`,
and a no-op once a model's already there. Expect the *first* `make up` to
take a while (a few GB total across both Ollama models, proportional to
your connection); every run after that is fast, since Docker volumes keep
every model around. Each service also applies its own schema's migrations
automatically on startup — nothing to run by hand.

**Apple Silicon note**: `ghcr.io/ggml-org/whisper.cpp` publishes no
`linux/arm64` build, so `whisper` is pinned to `platform: linux/amd64`
and runs under Rosetta emulation — CPU transcription will be noticeably
slower than on native x86_64, but `make up` itself will complete.

**If you don't need the AI pipeline** (transcript/summary/action-item/
search/RAG/analytics endpoints, or the notifications they trigger) for
what you're testing right now, it's still fine to skip `whisper`,
`whisper-model-init`, `ollama`, `ollama-model-init`,
`transcription-service`, `ai-summary-service`, `action-item-service`,
`search-service`, `analytics-service`, `notification-service`, and
`mailhog` entirely and start faster by naming only the services you want
(nothing produces `summary.completed.v1`/`action-item.extracted.v1`
without the AI pipeline running, so Notification Service would just sit
idle waiting for events that never arrive):
```bash
docker compose -f deployments/docker-compose.yaml up --build \
  postgres redis minio kafka kafka-init \
  organization-service user-service auth-service meeting-service \
  api-gateway web
```
**Caveat found while wiring Phase 4.2's own dependencies**: this doesn't
actually skip anything today. `api-gateway` itself `depends_on` every
backend service it can route to (since Phase 2.5 added Action Item
Service to that list, and Analytics/Search/Notification followed) —
Compose always starts a named service's full transitive `depends_on`
graph, so naming `api-gateway` here pulls in the entire AI pipeline
regardless of what else is listed (`docker compose config` confirms this:
`api-gateway`'s resolved `depends_on` already includes
`action-item-service`, `ai-summary-service`, `analytics-service`,
`search-service`, `transcription-service`, and `notification-service`).
A real fix (Compose profiles, tagging the AI-pipeline services so they
opt out of a base `up`) is a Phase 5 (Kubernetes & CI/CD)-adjacent
infra task, not done here — for a genuinely faster first bring-up today,
drop `api-gateway`/`web` from the list above and curl each core service
directly on its own port (see Configuration below for the port map)
instead of through the gateway.
Then:

- **Web app**: http://localhost:5173 — sign up, log in, upload a
  recording, watch it show up in your meeting list.
- **API directly**: base URL `http://localhost:8000/api/v1`, per
  `docs/architecture/api-spec.md`. Example:
  ```bash
  curl -X POST http://localhost:8000/api/v1/auth/signup \
    -H "Content-Type: application/json" \
    -d '{"orgName":"Acme Inc","email":"alice@acme.com","name":"Alice","password":"correcthorsebatterystaple"}'
  ```
  Use the returned `accessToken` as `Authorization: Bearer <token>` on
  everything else (`GET /users/me`, `POST /meetings`, …) — see "API
  Reference — curl / Postman" below for every endpoint.
- **MinIO console**: http://localhost:9001 (`minioadmin` / `minioadmin`).
- **Mailhog inbox**: http://localhost:8025 — every email Notification
  Service sends (Phase 4.1's "summary ready" notification) lands here
  instead of a real inbox.

Password reset returns its token directly in the API response in this dev
setup (`auth_dev_expose_reset_token: true` in
`deployments/configs/docker/auth-service.json`) rather than emailing it —
there's no Notification Service to send real email until Phase 4.

### API Reference — curl / Postman

Every request below goes through the API Gateway at
`http://localhost:8000/api/v1` (per `docs/architecture/api-spec.md`), the
same way a real client would — nothing here talks to a service directly
except the "Internal service-to-service APIs" section at the very end,
which is for debugging the plumbing, not for a client to call.

**To use in Postman**: create an environment with the variables below,
then paste each `curl` command into Postman's *Import → Raw text* (or
just type the request by hand) — the `{{variable}}` placeholders resolve
against your environment automatically. To run a command with plain
`curl` instead, replace every `{{...}}` with a real value first.

| Variable | Starting value | Set from |
|---|---|---|
| `base_url` | `http://localhost:8000/api/v1` | fixed |
| `access_token` | *(empty)* | `accessToken` in a signup/login/refresh/accept-invite response |
| `refresh_token` | *(empty)* | `refreshToken` in the same responses |
| `org_id` | *(empty)* | decode the JWT in `access_token` (it's the `org_id` claim), or the `orgId` in `GET /users/me`'s response |
| `user_id` | *(empty)* | `id` in `GET /users/me`'s response |
| `role` | *(empty)* | `role` in `GET /users/me`'s response — `POST /auth/refresh` needs this |
| `meeting_id` | *(empty)* | `meetingId` in `POST /meetings`'s response |
| `action_item_id` | *(empty)* | `id` of any entry in `GET /meetings/{{meeting_id}}/action-items`'s `data` array |
| `issue_key` | *(empty)* | `issueKey` of any entry in a mock Jira board's `data` array (e.g. `GET /orgs/{{org_id}}/mock-jira/board`) |
| `invite_token` | *(empty)* | logged by user-service as `dev_invite_token` (see `POST /orgs/{orgId}/invites` below) |
| `reset_token` | *(empty)* | logged by auth-service as `dev_reset_token`, or `devToken` in the response if `auth_dev_expose_reset_token: true` |

Every response follows `{"error": {"code", "message", "requestId"}}` on
failure — see `docs/architecture/api-spec.md` for the full contract.

#### Auth (public — no token needed)

**Signup** — creates a new org and its owner user, returns tokens:
```bash
curl -X POST {{base_url}}/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "orgName": "Acme Inc",
    "email": "alice@acme.com",
    "name": "Alice",
    "password": "correcthorsebatterystaple"
  }'
```

**Login**:
```bash
curl -X POST {{base_url}}/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@acme.com",
    "password": "correcthorsebatterystaple"
  }'
```

**Refresh** — rotates the refresh token (single-use; the old one is
revoked the moment a new one is issued, so refreshing twice with the same
token fails on purpose):
```bash
curl -X POST {{base_url}}/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "orgId": "{{org_id}}",
    "refreshToken": "{{refresh_token}}",
    "role": "{{role}}"
  }'
```

**Logout** — revokes a refresh token (needs a bearer token, unlike the
rest of this section):
```bash
curl -X POST {{base_url}}/auth/logout \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{
    "orgId": "{{org_id}}",
    "refreshToken": "{{refresh_token}}"
  }'
```

**Request password reset** — always returns `200` whether or not the
email exists (no account enumeration); the token is logged server-side
and only echoed in the response when
`auth_dev_expose_reset_token: true`:
```bash
curl -X POST {{base_url}}/auth/password/reset-request \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@acme.com"}'
```

**Confirm password reset**:
```bash
curl -X POST {{base_url}}/auth/password/reset-confirm \
  -H "Content-Type: application/json" \
  -d '{
    "token": "{{reset_token}}",
    "newPassword": "aDifferentSecurePassword1"
  }'
```

**Accept an org invite** — the invite-flow counterpart to signup; joins
the org and role an owner/admin already chose, returns tokens like
signup/login do:
```bash
curl -X POST {{base_url}}/invites/{{invite_token}}/accept \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Bob",
    "password": "correcthorsebatterystaple"
  }'
```

#### Users (bearer token required)

**Get my profile**:
```bash
curl {{base_url}}/users/me \
  -H "Authorization: Bearer {{access_token}}"
```

**Update my profile**:
```bash
curl -X PATCH {{base_url}}/users/me \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice Smith", "avatarUrl": "https://example.com/avatar.png"}'
```

**List my org's users** (any member; supports `?page=&pageSize=`):
```bash
curl "{{base_url}}/orgs/{{org_id}}/users?page=1&pageSize=20" \
  -H "Authorization: Bearer {{access_token}}"
```

**Get one user**:
```bash
curl {{base_url}}/orgs/{{org_id}}/users/{{user_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

**Invite a user** — owner/admin only; role must be `admin`, `manager`,
`member`, or `viewer` (never `owner`). The raw invite token is logged as
`dev_invite_token` and only appears in the response body when
`user_dev_expose_invite_token: true`:
```bash
curl -X POST {{base_url}}/orgs/{{org_id}}/invites \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"email": "bob@acme.com", "role": "member"}'
```

**Change a user's role** — owner/admin only; can't target the org owner
or your own account:
```bash
curl -X PATCH {{base_url}}/orgs/{{org_id}}/users/{{user_id}}/role \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"role": "manager"}'
```

**Deactivate a user** — owner/admin only; same self/owner restrictions
as above:
```bash
curl -X DELETE {{base_url}}/orgs/{{org_id}}/users/{{user_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

#### Organizations (bearer token required)

**Get my org**:
```bash
curl {{base_url}}/orgs/{{org_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

#### Meetings (bearer token required)

**Create a meeting / get a presigned upload URL**:
```bash
curl -X POST {{base_url}}/meetings \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"title": "Sprint Planning"}'
```
Then `PUT` the actual recording bytes straight to the `uploadUrl` the
response returns — this goes directly to MinIO, not through the gateway:
```bash
curl -X PUT "<uploadUrl from the response above>" \
  -H "Content-Type: audio/mpeg" \
  --data-binary @recording.mp3
```

**Confirm the upload** — verifies the object actually landed in MinIO,
and (Phase 2.3+) publishes `meeting.uploaded.v1` to kick off
transcription:
```bash
curl -X POST {{base_url}}/meetings/{{meeting_id}}/complete-upload \
  -H "Authorization: Bearer {{access_token}}"
```

**Get one meeting**:
```bash
curl {{base_url}}/meetings/{{meeting_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

**Lightweight status poll**:
```bash
curl {{base_url}}/meetings/{{meeting_id}}/status \
  -H "Authorization: Bearer {{access_token}}"
```

**List meetings** (supports `?page=&pageSize=`):
```bash
curl "{{base_url}}/meetings?page=1&pageSize=20" \
  -H "Authorization: Bearer {{access_token}}"
```

**Manually override status** — a Phase 1 debug endpoint (org owner
only), for simulating the pipeline before Kafka consumers existed; valid
values are `uploaded`, `transcribing`, `transcribed`, `summarizing`,
`summarized`, `completed`, `failed`:
```bash
curl -X PATCH {{base_url}}/meetings/{{meeting_id}}/status \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"status": "completed"}'
```

**Delete a meeting**:
```bash
curl -X DELETE {{base_url}}/meetings/{{meeting_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

#### Transcript & Summary (bearer token required, Phase 2.3/2.4)

These only return data once a real Kafka + whisper.cpp + Ollama pipeline
has actually run (not the case in this sandbox — see the Status section
above); against a real `docker compose up` with models pulled, they work
once transcription/summarization for that meeting has completed.

**Get the transcript**:
```bash
curl {{base_url}}/meetings/{{meeting_id}}/transcript \
  -H "Authorization: Bearer {{access_token}}"
```

**Get the summary**:
```bash
curl {{base_url}}/meetings/{{meeting_id}}/summary \
  -H "Authorization: Bearer {{access_token}}"
```

**Regenerate the summary** — re-runs the fetch/chunk/summarize pipeline
synchronously on demand (needs Transcription Service to already have a
transcript for this meeting, and a reachable Ollama server):
```bash
curl -X POST {{base_url}}/meetings/{{meeting_id}}/summary/regenerate \
  -H "Authorization: Bearer {{access_token}}"
```

#### Action Items (bearer token required, Phase 2.5)

Like Transcript & Summary above, these only populate once
`summary.completed.v1` has actually been consumed by Action Item Service
for that meeting — not the case in this sandbox (see the Status section
above); against a real `docker compose up`, they work once
summarization for that meeting has completed.

**List action items for one meeting**:
```bash
curl {{base_url}}/meetings/{{meeting_id}}/action-items \
  -H "Authorization: Bearer {{access_token}}"
```

**List action items across meetings** — filters are all optional
(`owner`, `status`, `type`, `dueBefore`, `page`, `pageSize`):
```bash
curl "{{base_url}}/action-items?status=open&type=action" \
  -H "Authorization: Bearer {{access_token}}"
```

**Get one action item**:
```bash
curl {{base_url}}/action-items/{{action_item_id}} \
  -H "Authorization: Bearer {{access_token}}"
```

**Update an action item** — status, owner, and/or due date; only the
item's own assigned owner or an org owner/admin may do this (every field
is optional, send just the one(s) you're changing):
```bash
curl -X PATCH {{base_url}}/action-items/{{action_item_id}} \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}'
```

**Create a Jira ticket** (Phase 4.2) — 202 Accepted; the ticket itself is
created asynchronously by Notification Service's outbox dispatch (mock
Jira by default — see the Status section above), which writes the
resulting `jiraIssueKey` back onto this same action item once done:
```bash
curl -X POST {{base_url}}/action-items/{{action_item_id}}/jira-ticket \
  -H "Authorization: Bearer {{access_token}}"
```

#### Ticketing — mock Jira board (Phase 4.2)

The board is genuinely bidirectional (see the Status section above): a
`PATCH` here writes the mapped status back onto the linked action item,
the same way a real Jira transition eventually will.

**Public demo board** (no auth — only populated once `demo_org_id` is
configured, see Configuration below):
```bash
curl {{base_url}}/demo/board
```

**Your org's mock Jira board**:
```bash
curl {{base_url}}/orgs/{{org_id}}/mock-jira/board \
  -H "Authorization: Bearer {{access_token}}"
```

**Move a card between columns** — `status` is one of `"To Do"`,
`"In Progress"`, `"Done"`:
```bash
curl -X PATCH {{base_url}}/orgs/{{org_id}}/mock-jira/issues/{{issue_key}} \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"status": "In Progress"}'
```

#### Search & RAG (bearer token required, Phase 3.1–3.4)

Like Transcript/Summary/Action Items above, these only return real
results once `chunk.created.v1` has actually been consumed by Search
Service for at least one meeting — not the case in this sandbox (see the
Status section above); against a real `docker compose up`, they work
once at least one meeting has been summarized (summarization is what
produces the chunks Search Service embeds).

**Semantic search across the org's meetings**:
```bash
curl "{{base_url}}/search?q=what%20did%20we%20decide%20about%20the%20launch" \
  -H "Authorization: Bearer {{access_token}}"
```

**Find meetings similar to one you're looking at**:
```bash
curl {{base_url}}/meetings/{{meeting_id}}/similar \
  -H "Authorization: Bearer {{access_token}}"
```

**Ask a question over meeting history (RAG)** — retrieves the most
relevant chunks, asks Ollama to answer grounded in them, and returns
citations back to the source meeting/timestamp:
```bash
curl -X POST {{base_url}}/qa/ask \
  -H "Authorization: Bearer {{access_token}}" \
  -H "Content-Type: application/json" \
  -d '{"question": "What did we decide about the launch date?"}'
```

**Your past questions and answers**:
```bash
curl {{base_url}}/qa/history \
  -H "Authorization: Bearer {{access_token}}"
```

**Re-embed every meeting in the org** — owner/admin-only, best-effort per
meeting (see the Status section's note on its one-page-of-100 limit):
```bash
curl -X POST {{base_url}}/search/reindex \
  -H "Authorization: Bearer {{access_token}}"
```

#### Analytics (bearer token required, `manager`/`admin`/`owner` only, Phase 3.5)

Every response here is a pre-aggregated read off a rollup table (CQRS
read-side, see the Status section above) — a `member`/`viewer` token gets
`403`, and results only reflect meetings/action items/summaries whose
Kafka events Analytics Service has actually consumed (not the case in
this sandbox — see the Status section's note on live verification).

**Meeting volume/duration over the last 30 days**:
```bash
curl {{base_url}}/analytics/meetings/trends \
  -H "Authorization: Bearer {{access_token}}"
```

**Per-user action item throughput** (opened/closed totals, all-time):
```bash
curl {{base_url}}/analytics/productivity \
  -H "Authorization: Bearer {{access_token}}"
```

**Org-wide action item completion rate**:
```bash
curl {{base_url}}/analytics/action-items/completion-rate \
  -H "Authorization: Bearer {{access_token}}"
```

**Most-discussed topics over the last 12 weeks** (top 20 by mentions —
see the Status section's note on this being a keyword-proxy, not real NLP
extraction):
```bash
curl {{base_url}}/analytics/topics \
  -H "Authorization: Bearer {{access_token}}"
```

#### Internal service-to-service APIs (debugging only — not through the gateway)

These are called by other services, never by a client, and aren't
reachable via the gateway — hit each service's own port directly, with
`X-Internal-Token` set to `internal_service_token` from its config
(`dev-internal-token` in every `deployments/configs/*.template.json`).
Listed here for completeness/debugging, not as part of the product API.

```bash
# User Service — used by Auth Service during signup
curl -X POST http://localhost:8081/internal/users \
  -H "X-Internal-Token: dev-internal-token" \
  -H "Content-Type: application/json" \
  -d '{"orgId": "{{org_id}}", "email": "alice@acme.com", "name": "Alice", "role": "owner"}'

# User Service — used by Auth Service's login flow to resolve an email to an org
curl "http://localhost:8081/internal/users/lookup?email=alice@acme.com" \
  -H "X-Internal-Token: dev-internal-token"

# User Service — used by Auth Service's AcceptInvite flow
curl -X POST http://localhost:8081/internal/invites/accept \
  -H "X-Internal-Token: dev-internal-token" \
  -H "Content-Type: application/json" \
  -d '{"token": "{{invite_token}}", "name": "Bob"}'

# Organization Service — used by Auth Service during signup
curl -X POST http://localhost:8082/internal/orgs \
  -H "X-Internal-Token: dev-internal-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Inc"}'

# Transcription Service — used by AI Summary Service to fetch a transcript
curl "http://localhost:8084/internal/meetings/{{meeting_id}}/transcript?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# AI Summary Service — used by Action Item Service to fetch a summary
curl "http://localhost:8085/internal/meetings/{{meeting_id}}/summary?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# Meeting Service — used by Action Item Service for best-guess owner matching
curl "http://localhost:8083/internal/meetings/{{meeting_id}}/participants?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# AI Summary Service — used by Search Service's embedding pipeline to fetch chunk text
curl "http://localhost:8085/internal/meetings/{{meeting_id}}/chunks?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# Meeting Service — used by Search Service to resolve a meeting title for a citation
curl "http://localhost:8083/internal/meetings/{{meeting_id}}?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# Meeting Service — used by Search Service's POST /search/reindex to enumerate an org's meetings
curl "http://localhost:8083/internal/meetings?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# Action Item Service — used by Analytics Service's opened-rollup to see each item's owner
curl "http://localhost:8086/internal/meetings/{{meeting_id}}/action-items?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# User Service — used by Notification Service to resolve a meeting creator's email
curl "http://localhost:8081/internal/users/{{user_id}}?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token"

# Action Item Service — used by Notification Service to write back a Jira issue key or mock-board-driven status change
curl -X PATCH "http://localhost:8086/internal/action-items/{{action_item_id}}?orgId={{org_id}}" \
  -H "X-Internal-Token: dev-internal-token" \
  -H "Content-Type: application/json" \
  -d '{"jiraIssueKey": "DEMO-1"}'
```

### Configuration

Every service reads its settings from a JSON file instead of environment
variables — see `deployments/configs/<service>.template.json` for the
full shape and dev-safe defaults. `make up` uses the compose-network
copies already checked in under `deployments/configs/docker/` (real
hostnames like `postgres` and `minio`); running a binary directly on the
host uses `deployments/configs/<service>.json`, which is gitignored, so
run `make configs` once to seed it from the template, then edit in
whatever you need to change. Point `database_url` at any Postgres 16+
instance and `redis_addr` at any Redis — migrations run automatically on
startup either way. Pass a different file with `-config`, e.g. `go run
./services/auth-service -config /path/to/config.json`.

### Local development without Docker

Each service is a normal Go binary (`go run ./services/auth-service`,
etc., run from the repo root so `go.work` is picked up) reading
`deployments/configs/<service>.json` by default — see Configuration
above.

```bash
make configs   # seed deployments/configs/*.json from the checked-in templates
make build     # compiles every module (shared + every service)
make vet
make test      # unit tests — jwtutil, passwordutil, and a usecase test
               # against an in-memory fake repository (no DB needed)
make lint
```

`go build`/`vet`/`test`/`golangci-lint` don't support a single `./...`
across a whole workspace from the repo root — the Makefile targets above
loop over `shared/` and each `services/<name>/` module for you. Working
inside one module (e.g. `cd services/auth-service`), the plain commands
work as usual.

`web/` is a standalone Vite app: `cd web && npm install && npm run dev`.
