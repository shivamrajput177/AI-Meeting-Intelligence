# Go Monorepo Folder Structure

Single repo, single `go.work` workspace, one `cmd/` entrypoint per
microservice, shared code under `internal/platform` and `pkg/`. Rationale:
a monorepo keeps the `.proto`-derived contracts, migrations, and Helm charts
co-located and atomically reviewable in one PR, while `internal/<service>`
boundaries keep the services themselves independently deployable and
(if ever needed) extractable into separate repos with minimal churn.

```
.
├── go.work
├── go.work.sum
├── Makefile                          # make proto, make lint, make test, make up (docker compose), make kind-up
├── docker-compose.yaml               # local dev: postgres, redis, kafka, minio, ollama, whisper.cpp
├── cmd/
│   ├── api-gateway/
│   │   ├── main.go
│   │   ├── go.mod
│   │   └── Dockerfile
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
│   │   ├── config/                   # env/flag loading (viper or plain env)
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
│   ├── authsvc/
│   │   ├── handler/                  # gRPC handlers implementing auth.proto
│   │   ├── service/                  # business logic (token issuance, password policy)
│   │   ├── repository/               # auth.* schema access
│   │   └── model/
│   ├── usersvc/            (handler/service/repository/consumer/model)
│   ├── orgsvc/              (handler/service/repository/model)
│   ├── meetingsvc/          (handler/service/repository/producer/consumer/model)
│   ├── transcriptionsvc/    (handler/service/repository/consumer/producer/whisper/model)
│   ├── aisummarysvc/        (handler/service/repository/consumer/producer/ollama/chunking/model)
│   ├── actionitemsvc/       (handler/service/repository/consumer/producer/ollama/model)
│   ├── searchsvc/           (handler/service/repository/consumer/producer/ollama/embeddings/rag/model)
│   ├── notificationsvc/     (handler/service/repository/consumer/producer/slack/email/jira/scheduler/model)
│   ├── analyticssvc/        (service/repository/consumer/rollup/model)
│   └── gateway/             (router/middleware/proxy — REST-to-gRPC translation)
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
│   ├── buf.yaml / buf.gen.yaml       # buf for lint + codegen
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
├── deploy/
│   ├── kind/kind-config.yaml
│   ├── helm/                         # see kubernetes-cicd.md §2
│   └── argocd/                       # ApplicationSet + AppProject manifests
│
├── .github/workflows/
│   ├── ci.yaml
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
    └── e2e/                          # full upload -> transcript -> summary -> action item flow
```

## Per-service internal package convention

Every `internal/<service>/` follows the same four-layer shape so switching
between services during code review needs zero re-orientation:

```
internal/<service>/
├── handler/       # gRPC (and REST, for gateway) — request/response mapping only, no business logic
├── service/       # business logic, orchestrates repository + kafka + external clients
├── repository/    # SQL (pgx), owns exactly this service's schema, sets tenant context
├── consumer/      # Kafka consumer group handlers (if the service consumes)
├── producer/      # Kafka publish helpers (if the service produces)
└── model/         # domain structs, separate from both DB rows and proto-generated types
```

`handler` depends on `service`; `service` depends on `repository`/`producer`/
external clients; nothing ever imports "up" the stack — a standard hexagonal/
clean-architecture layering, explicitly called out because it's a common
Staff-level interview discussion point (dependency direction, testability via
interfaces at the `service` boundary).
