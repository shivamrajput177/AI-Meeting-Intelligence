# Folder Structure — Go Monorepo + Frontend

Single repo: a **single Go module** for the backend (one `go.mod` at the
repo root, one `cmd/` entrypoint per microservice, shared code under
`internal/platform` and `pkg/`), plus a `web/` directory for the frontend.
Rationale for one repo: it keeps the REST API contracts, migrations, Helm
charts, and the UI that consumes the API co-located and atomically
reviewable in one PR, while `internal/<service>` boundaries keep the
backend services independently deployable and (if ever needed)
extractable into separate repos with minimal churn. There's no
`.proto`/codegen step anywhere — every API, external and internal, is
plain hand-written REST/JSON (see `microservices.md` §"Internal
Communication").

**Correction from the original design**: this was originally planned as a
`go.work` workspace with one `go.mod` per service, so each `cmd/<service>`
could in principle be extracted to its own repo without a rewrite. That
doesn't actually work with Go's own rules: a package under `internal/` is
only importable by code whose import path shares the prefix up to that
`internal/` directory's parent — a separate module (a different root
import path entirely) cannot import `internal/platform` no matter what
`go.work` says, since `go.work` only lets modules that are *already*
part of the same build resolve against each other's non-internal
packages. One shared module is also simply the standard, idiomatic layout
for a Go monorepo (the same shape Kubernetes, Docker, and most large Go
codebases use) — not a compromise, the actual right call once you check
what the language permits. Extracting a service into its own module and
repo later means giving `internal/platform` (or the slice of it that
service needs) its own module boundary at that point — a real but
localized change, not a sign the current layout was wrong for now.

```
.
├── go.mod
├── go.sum
├── Makefile                          # make build/vet/test/lint, make up/down (docker compose)
├── Dockerfile                        # shared multi-stage build for every backend service, parameterized by --build-arg SERVICE=<cmd dir>
├── docker-compose.yaml               # local dev: postgres (pgvector image), redis, minio, all 5 backend services, web
├── cmd/
│   ├── api-gateway/main.go
│   ├── auth-service/main.go
│   ├── user-service/main.go
│   ├── organization-service/main.go
│   ├── meeting-service/main.go        # Phase 1 — one cmd per service exists as it's built
│   ├── transcription-service/         #  } Phase 2+
│   ├── ai-summary-service/            #  }
│   ├── action-item-service/           #  }
│   ├── search-service/                #  }
│   ├── notification-service/          #  }
│   └── analytics-service/             #  }
│                                      # (no per-service Dockerfile — the root Dockerfile is
│                                      #  parameterized by --build-arg SERVICE=<dir name>)
│
├── internal/
│   ├── platform/                     # shared infra, imported by every service
│   │   ├── config/                   # env var loading with defaults
│   │   ├── logger/                   # slog JSON setup
│   │   ├── apperr/                   # typed app errors -> HTTP status + the {"error":{...}} envelope
│   │   ├── jwtutil/                  # access-token sign/verify, opaque refresh-token generation/hashing
│   │   ├── passwordutil/             # argon2id hash/verify
│   │   ├── reqctx/                   # context accessors for org/user/role/request-id + the header names they travel under
│   │   ├── dbx/                      # pgx pool, embedded-SQL migration runner, RLS tenant-context helper (+ the bypass-RLS escape hatch)
│   │   ├── redisx/                   # client wrapper, revocation cache, fixed-window rate limiter
│   │   ├── httpserver/               # Fiber bootstrap + shared middleware (request id, recovery, access log, header->context), internal-token guard
│   │   └── httpclient/               # shared client for calling another service's REST API: timeout, one retry, header propagation, error-envelope mapping
│   │
│   ├── authsvc/            (domain/usecase/repository/delivery/client — client/ holds the OrgClient/UserClient adapters)
│   ├── usersvc/            (domain/usecase/repository/delivery)
│   ├── orgsvc/              (domain/usecase/repository/delivery)
│   ├── meetingsvc/          (domain/usecase/repository/delivery, + storage/minio)
│   ├── transcriptionsvc/    (Phase 2 — domain/usecase/repository/delivery, + whisper client)
│   ├── aisummarysvc/        (Phase 2 — domain/usecase/repository/delivery, + ollama client, chunking)
│   ├── actionitemsvc/       (Phase 2 — domain/usecase/repository/delivery, + ollama client)
│   ├── searchsvc/           (Phase 3 — domain/usecase/repository/delivery, + ollama client, embeddings, rag)
│   ├── notificationsvc/     (Phase 4 — domain/usecase/repository/delivery, + slack/email/jira clients, scheduler)
│   ├── analyticssvc/        (Phase 3 — domain/usecase/repository/delivery, + rollup)
│   └── routes/              (delivery/http only — the API Gateway's route handlers, reverse-proxying REST to each service via middleware/ and proxy/ subpackages; no domain/usecase of its own)
│
├── migrations/
│   ├── auth/{0001_init.up.sql, embed.go}      # embed.go: `//go:embed *.sql` — see its doc comment for why
│   ├── user/{0001_init.up.sql, embed.go}
│   ├── org/{0001_init.up.sql, embed.go}
│   ├── meeting/{0001_init.up.sql, embed.go}
│   ├── transcription/...                       # Phase 2+
│   ├── ai/...
│   ├── actionitem/...
│   ├── search/...
│   ├── notification/...
│   └── analytics/...
│
├── web/                               # frontend — the thing a recruiter/visitor actually opens
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   ├── nginx.conf                     # SPA fallback (try_files ... /index.html) for react-router routes
│   ├── Dockerfile                     # multi-stage: `npm run build` (VITE_API_BASE_URL baked in as a build arg) -> static files served by nginx
│   ├── src/
│   │   ├── main.tsx
│   │   ├── vite-env.d.ts
│   │   ├── app/                       # App.tsx (routes) + Layout.tsx (nav/logout shell)
│   │   ├── pages/                     # LoginPage, SignupPage, MeetingsPage (list+upload), MeetingDetailPage — more land as each phase adds a feature
│   │   ├── components/                # ProtectedRoute, StatusBadge
│   │   ├── api/                       # typed REST client (client.ts + auth.ts, users.ts, meetings.ts)
│   │   ├── hooks/                     # useAuth.tsx — token storage, JWT payload decode for org/role, signup/login/logout
│   │   └── styles/
│   └── public/
│
├── deploy/
│   ├── kind/kind-config.yaml
│   ├── helm/                         # backend charts, see kubernetes-cicd.md §2, plus a `web` chart serving the built static frontend
│   └── argocd/                       # ApplicationSet + AppProject manifests
│
├── .github/workflows/
│   ├── ci.yaml                       # backend
│   ├── web-ci.yaml                   # frontend: lint/typecheck/build
│   └── security.yaml
│
├── docs/
│   ├── PROJECT_PLAN.md
│   ├── ROADMAP.md
│   └── architecture/...
│
├── scripts/
│   ├── seed-dev-data.sh
│   ├── run-migrations.sh
│   └── pull-ollama-models.sh
│
└── test/
    ├── integration/                  # spins docker-compose, hits real REST endpoints
    └── e2e/                          # Playwright, drives web/ against a running stack end-to-end
```

**Frontend stack**: React + Vite + TypeScript, plain CSS or Tailwind, React
Query for data fetching, no server-rendering needed — it's a thin client
over the REST API in `api-spec.md`. Built to static files and served either
by a tiny nginx container or directly by the API Gateway (one less moving
part for the public demo VM in `deployment-demo-strategy.md`). This is
what a recruiter opens; the Go backend is what they read the code for.

## Per-service internal package convention (Clean Architecture)

Every `internal/<service>/` follows the same shape — **domain → usecase →
repository/delivery**, dependencies pointing inward, so switching between
services during code review needs zero re-orientation:

```
internal/<service>/
├── domain/                # entities + interfaces (ports) — no external deps, nothing here imports anything else in this tree
│   ├── entity.go          #   e.g. ActionItem, Meeting — plain structs
│   ├── repository.go      #   interfaces: ActionItemRepository, defined by what usecase needs
│   └── errors.go
│
├── usecase/                # business logic — implements the use cases, depends only on domain interfaces
│   ├── create_meeting.go
│   ├── list_action_items.go
│   └── ...                #   one file (or a few grouped) per use case; unit-tested against mocked domain interfaces
│
├── repository/             # concrete adapters implementing domain's repository interfaces
│   ├── postgres/           #   pgx-backed implementation, owns exactly this service's schema, sets tenant context
│   └── redis/              #   cache-backed implementation, where used
│
└── delivery/                # transport adapters — translate the outside world into usecase calls
    ├── http/                #   this service's own REST API (its handlers implement the routes documented for it in api-spec.md/microservices.md) — every service has one, since internal callers use the same REST transport as external clients
    └── kafka/                #   consumer + producer adapters (publish/subscribe wrappers around usecase calls)
```

**Dependency rule**: `delivery` depends on `usecase`; `usecase` depends on
`domain` (interfaces only, never a concrete `repository` package);
`repository` also depends on `domain` (it implements domain's interfaces).
Nothing in `domain` imports anything else in the tree. This is the
standard Go Clean/Hexagonal Architecture layout (the same shape as
`bxcodec/go-clean-arch`: entity → usecase → repository → delivery) —
dependency inversion at the `domain` boundary is what makes `usecase`
unit-testable with an in-memory fake repository and no real Postgres/Kafka
in the test, which is worth being able to explain in a Staff-level
interview.
