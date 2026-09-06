# Folder Structure — Go Monorepo + Frontend

Single repo: a `go.work` workspace for the backend (one `cmd/` entrypoint per
microservice, shared code under `internal/platform` and `pkg/`), plus a
`web/` directory for the frontend. Rationale for one repo: it keeps the
`.proto`-derived contracts, migrations, Helm charts, and the UI that
consumes the REST API co-located and atomically reviewable in one PR, while
`internal/<service>` boundaries keep the backend services independently
deployable and (if ever needed) extractable into separate repos with
minimal churn.

```
.
├── go.work
├── go.work.sum
├── Makefile                          # make proto, make lint, make test, make up (docker compose), make kind-up
├── docker-compose.yaml               # local dev: postgres, redis, kafka, minio, ollama, whisper.cpp
├── cmd/
│   ├── api-gateway/{main.go,go.mod,Dockerfile}
│   ├── auth-service/{main.go,go.mod,Dockerfile}
│   ├── user-service/{main.go,go.mod,Dockerfile}
│   ├── organization-service/{main.go,go.mod,Dockerfile}
│   ├── meeting-service/{main.go,go.mod,Dockerfile}
│   ├── transcription-service/{main.go,go.mod,Dockerfile}
│   ├── ai-summary-service/{main.go,go.mod,Dockerfile}
│   ├── action-item-service/{main.go,go.mod,Dockerfile}
│   ├── search-service/{main.go,go.mod,Dockerfile}
│   ├── notification-service/{main.go,go.mod,Dockerfile}
│   └── analytics-service/{main.go,go.mod,Dockerfile}
│
├── internal/
│   ├── platform/                     # shared infra, imported by every service
│   │   ├── config/                   # env/flag loading
│   │   ├── logger/                   # slog setup, trace-id enrichment
│   │   ├── tracing/                  # OTel SDK bootstrap
│   │   ├── metrics/                  # prometheus registry helpers
│   │   ├── db/                       # pgx pool, migration runner, RLS tenant-context helper
│   │   ├── kafka/                    # producer/consumer wrappers, DLQ helper, trace header propagation
│   │   ├── grpcserver/               # server bootstrap, auth/tenant interceptors, health/reflection
│   │   ├── grpcclient/               # client dial helpers, retry/circuit-breaker interceptors
│   │   ├── httpserver/               # Fiber bootstrap, middleware chain
│   │   ├── redis/                    # client wrapper, rate limiter, cache helpers
│   │   └── errors/                   # typed app errors -> gRPC status / HTTP code mapping
│   │
│   ├── authsvc/            (domain/usecase/repository/delivery)
│   ├── usersvc/            (domain/usecase/repository/delivery)
│   ├── orgsvc/              (domain/usecase/repository/delivery)
│   ├── meetingsvc/          (domain/usecase/repository/delivery)
│   ├── transcriptionsvc/    (domain/usecase/repository/delivery, + whisper client)
│   ├── aisummarysvc/        (domain/usecase/repository/delivery, + ollama client, chunking)
│   ├── actionitemsvc/       (domain/usecase/repository/delivery, + ollama client)
│   ├── searchsvc/           (domain/usecase/repository/delivery, + ollama client, embeddings, rag)
│   ├── notificationsvc/     (domain/usecase/repository/delivery, + slack/email/jira clients, scheduler)
│   ├── analyticssvc/        (domain/usecase/repository/delivery, + rollup)
│   └── gateway/             (delivery/http only — routes + REST-to-gRPC translation, no domain/usecase of its own)
│
├── pkg/                              # importable outside this repo if ever needed
│   ├── jwtutil/
│   ├── validator/
│   └── apierrors/
│
├── proto/
│   ├── common.proto
│   ├── auth.proto
│   ├── user.proto
│   ├── organization.proto
│   ├── meeting.proto
│   ├── transcription.proto
│   ├── ai_summary.proto
│   ├── action_item.proto
│   ├── search.proto
│   ├── notification.proto
│   ├── analytics.proto
│   ├── buf.yaml / buf.gen.yaml       # buf for lint + codegen (Go stubs only — see api-spec.md for why no OpenAPI codegen)
│   └── gen/                          # generated Go stubs (checked in or gitignored + `make proto`)
│
├── migrations/
│   ├── auth/0001_init.up.sql ...
│   ├── user/...
│   ├── org/...
│   ├── meeting/...
│   ├── transcription/...
│   ├── ai/...
│   ├── actionitem/...
│   ├── search/...
│   ├── notification/...
│   └── analytics/...
│
├── web/                               # frontend — the thing a recruiter/visitor actually opens
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── Dockerfile                     # multi-stage: `npm run build` -> static files served by nginx (or by API Gateway)
│   ├── src/
│   │   ├── main.tsx
│   │   ├── app/                       # routing + top-level layout
│   │   ├── pages/                     # Login, Signup, MeetingList, MeetingDetail, Search/RAG chat, ActionItems, Analytics, DemoBoard
│   │   ├── components/                # shared UI (buttons, cards, status badges, upload widget)
│   │   ├── api/                       # typed REST client (fetch wrappers per resource: auth.ts, meetings.ts, actionItems.ts, search.ts …)
│   │   ├── hooks/                     # data-fetching hooks (react-query), auth context
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
│   ├── security.yaml
│   └── proto-lint.yaml
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
    ├── integration/                  # spins docker-compose, hits real gRPC endpoints
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
    ├── grpc/                #   gRPC server implementing the service's .proto contract
    ├── http/                #   REST handlers, only present where a service is called directly (mostly just the gateway)
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
