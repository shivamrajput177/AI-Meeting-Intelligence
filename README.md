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

## Status: Phase 1 (MVP) implemented, Phase 2.1–2.4 implemented

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

**Not live-verified in this sandbox, for either 2.3 or 2.4**: the egress
proxy here blocks all container-registry traffic, so the Kafka broker, a
real whisper.cpp server, and a real Ollama server have never actually
been run — the business logic (including the chunking algorithm) is
verified by unit test with fakes, and each service's REST+Postgres read
path is verified against a seeded row; the docker-compose config itself
is unverified past `docker compose config` syntax validation. See
`docs/architecture/database-schema.md`'s "Row-Level Security pattern"
section for a related, now-fixed finding: every repository query in this
project filters by `org_id` explicitly rather than relying on Postgres
RLS, which turned out to be silently inert (every service connects as
the table owner/superuser, which RLS never applies to). Phase 2 onward
remains ahead: action-item extraction, and giving each service's DB
connection its own non-superuser role so RLS becomes real defense-in-depth
again.

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
MinIO, Kafka, Ollama, whisper.cpp, all seven backend services, and the web
app. Each service applies its own schema's migrations automatically on
startup — nothing to run by hand.

**If `whisper`'s image fails to pull** (`no matching manifest for
linux/arm64/...`) — confirmed on Apple Silicon Macs, since
`ghcr.io/ggml-org/whisper.cpp` publishes no arm64 build — `docker
compose` aborts the whole `up`, not just that one container.
`docker-compose.yaml` now pins `whisper` to `platform: linux/amd64` so it
at least pulls (via Rosetta emulation, so noticeably slower than native).
If you don't need the AI transcription/summarization pipeline for what
you're testing, it's simpler to skip it and everything downstream of it
entirely by naming only the services you want:
```bash
docker compose -f deployments/docker-compose.yaml up --build \
  postgres redis minio kafka kafka-init \
  organization-service user-service auth-service meeting-service \
  api-gateway web
```
That covers every Auth/Users/Organizations/Meetings endpoint in the API
Reference below — just not Transcript/Summary, which need
`transcription-service`/`ai-summary-service` (and, in turn, `whisper`
and `ollama`) actually running. Then:

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
make build     # compiles every module (shared + all 5 services)
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
