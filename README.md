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

## Status: Phase 1 (MVP) implemented

Auth, User, Organization, and Meeting services, the API Gateway, and a
React web app are built and running — signup, login, JWT refresh/rotation,
logout, password reset, tenant isolation, org/user profile management, and
meeting upload via presigned MinIO URLs all work end-to-end. See
`docs/ROADMAP.md`'s Phase 1 section for exact scope, and Phase 2 onward
for what's next (Kafka, transcription, summarization, RBAC).

One implementation note that updates `docs/architecture/folder-structure.md`:
the backend is a **single Go module** (`go.mod` at the repo root), not
separate `go.mod` files per service under a `go.work` workspace as
originally planned — Go's `internal/` visibility rule only allows a
package under `internal/` to be imported by code sharing its module root,
so per-service modules couldn't actually share `internal/platform` the way
the design called for. One module is also just the standard pattern for a
Go monorepo of this shape.

### Quickstart

```bash
docker compose up --build
```

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
`configs/docker/auth-service.json`) rather than emailing it — there's no
Notification Service to send real email until Phase 4.

### Configuration

Every service reads its settings from a JSON file instead of environment
variables — see `configs/<service>.template.json` for the full shape and
dev-safe defaults. `docker compose up` uses the compose-network copies
already checked in under `configs/docker/` (real hostnames like `postgres`
and `minio`); running a binary directly on the host uses
`configs/<service>.json`, which is gitignored, so run `make configs` once
to seed it from the template, then edit in whatever you need to change.
Point `database_url` at any Postgres 16+ instance and `redis_addr` at any
Redis — migrations run automatically on startup either way. Pass a
different file with `-config`, e.g. `go run ./cmd/auth-service -config
/path/to/config.json`.

### Local development without Docker

Each service is a normal Go binary (`go run ./cmd/auth-service`, etc.)
reading `configs/<service>.json` by default — see Configuration above.

```bash
make configs      # seed configs/*.json from the checked-in templates
go build ./...    # compiles every service
go vet ./...
go test ./...     # unit tests — jwtutil, passwordutil, and a usecase test
                   # against an in-memory fake repository (no DB needed)
golangci-lint run ./...
```

`web/` is a standalone Vite app: `cd web && npm install && npm run dev`.
