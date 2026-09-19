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

## Status: Phase 1 (MVP) implemented, Phase 2.1–2.3 implemented

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
/meetings/{id}/transcript` is live behind the gateway. **Not live-verified
in this sandbox**: the egress proxy here blocks all container-registry
traffic, so neither the Kafka broker nor a real whisper.cpp server has
actually been run — the business logic is verified by unit test (fakes
for storage/ASR/Kafka) and the REST+Postgres path is verified against a
seeded row; the docker-compose config itself is unverified past `docker
compose config` syntax validation. Phase 2 onward otherwise remains
ahead: summarization, action-item extraction, RBAC's remaining Kafka
audit-trail hookup.

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
MinIO, all five backend services, and the web app. Each service applies
its own schema's migrations automatically on startup — nothing to run by
hand. Then:

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
  everything else (`GET /users/me`, `POST /meetings`, …).
- **MinIO console**: http://localhost:9001 (`minioadmin` / `minioadmin`).

Password reset returns its token directly in the API response in this dev
setup (`auth_dev_expose_reset_token: true` in
`deployments/configs/docker/auth-service.json`) rather than emailing it —
there's no Notification Service to send real email until Phase 4.

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
