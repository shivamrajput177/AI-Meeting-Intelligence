# Folder Structure — Go Workspace + Frontend

Single repo, but the backend is a **Go workspace** (`go.work` at the repo
root): `shared/` is its own Go module, and every `services/<name>/` is a
separate Go module with its own `go.mod`, one `cmd/` entrypoint, and its
own `internal/` package tree. `go.work` lists all of them under `use` so
`go build`/`go test` resolve across module boundaries locally, with no
real `github.com/...` release needed for `shared` — see "Why a workspace,
not one module" below for why this only works because `shared/` isn't
nested under any `internal/` directory. Rationale for one repo despite
the module split: it keeps the REST API contracts, migrations, Docker/K8s
manifests, and the UI that consumes the API co-located and atomically
reviewable in one PR, while each service's own module boundary keeps it
independently buildable and (if ever needed) extractable into its own
repo with minimal churn — just delete it from `go.work`'s `use` list and
point its `shared` `replace` directive at a real tagged release instead
of `../../shared`. There's no `.proto`/codegen step anywhere — every API,
external and internal, is plain hand-written REST/JSON (see
`microservices.md` §"Internal Communication").

## Why a workspace, not one module

This wasn't the first design here. Go's own rule is: a package under
`internal/` is only importable by code whose import path shares the
prefix up to that `internal/` directory's parent. Concretely, if the
shared library had stayed at `internal/platform/` (as it did in an
earlier revision of this project), no other module could ever import it —
not even with `go.work` — because `internal/` visibility is enforced by
import path, not by which modules a workspace happens to bundle together.
That's what forced the earlier fallback to a single shared module for
the whole backend.

The actual fix is simpler than either extreme: keep `shared/` out of any
`internal/` directory entirely. A package's own name has no special
meaning to the Go toolchain — only a literal path segment called
`internal` does — so `shared/logger`, `shared/config`, etc. are ordinary
importable packages, and any module `go.work` lists can import them
across its own module boundary. Each service still keeps its
service-specific code under its own `internal/` (domain, usecase,
repository, handler) so *that* stays properly private to the service —
nothing outside `services/auth-service/` can import
`services/auth-service/internal/usecase`, which is exactly the
encapsulation `internal/` is for. Only the genuinely cross-service
library moved out from under it.

```
.
├── go.work                            # lists shared/ + every services/<name>/ module
├── Makefile                           # build/vet/test/lint loop over every module; up/down (docker compose)
├── README.md
│
├── services/
│   ├── api-gateway/
│   │   ├── go.mod
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── handler/               # router.go (Register) — no domain/usecase/repository, just routing+middleware+proxy composition
│   │       └── proxy/                 # net/http/httputil.ReverseProxy wrapper
│   │
│   ├── auth-service/
│   │   ├── go.mod
│   │   ├── cmd/main.go
│   │   ├── migrations/{0001_init.up.sql, embed.go}
│   │   └── internal/
│   │       ├── domain/                # entities + repository interfaces + errors — no external deps
│   │       ├── usecase/                # signup, login, refresh, logout, password reset, token issuer
│   │       ├── repository/postgres/    # pgx-backed CredentialsRepository, RefreshTokenRepository, PasswordResetRepository
│   │       ├── handler/                # this service's own REST API (handler.go + routes.go)
│   │       └── client/                 # OrgClient/UserClient — outbound REST calls to org/user services, built on shared/httpclient
│   │
│   ├── user-service/       (go.mod, cmd/, migrations/, internal/{domain,usecase,repository/postgres,handler})
│   ├── organization-service/ (same shape)
│   ├── meeting-service/     (same shape, + internal/storage/minio — see its own doc comment for the internal/public endpoint split)
│   │
│   ├── transcription-service/  # Phase 2+ — same shape once built, + a whisper.cpp client
│   ├── ai-summary-service/     #  } Phase 2+, + an Ollama client, chunking
│   ├── action-item-service/    #  } Phase 2+, + an Ollama client
│   ├── search-service/         #  } Phase 3+, + Ollama client, embeddings, RAG
│   ├── notification-service/   #  } Phase 4+, + slack/email/jira clients, scheduler
│   └── analytics-service/      #  } Phase 3+, + rollup jobs
│
├── shared/                            # its own Go module — NOT under internal/, so every services/<name>/ module can import it (see above)
│   ├── go.mod
│   ├── logger/                        # hand-rolled key=value logger, sync.Once singleton (no log/slog — see PROJECT_PLAN.md §5)
│   ├── config/                        # generic JSON config-file loader (config.Load[T]) — no env vars, see deployments/configs/
│   ├── middleware/                    # every func(http.Handler) http.Handler in the system: the generic per-service chain (RequestID, RecoverPanic, AccessLog, ContextFromHeaders, RequireInternalToken) plus the two the gateway alone installs (Auth, RateLimit)
│   ├── metrics/                       # Recorder interface + NoOp — placeholder seam for Phase 6's Prometheus wiring
│   ├── httpserver/                    # net/http + ServeMux bootstrap (no web framework): wires shared/middleware's chain together, plus H/JSON/NoContent/DecodeJSON response helpers
│   ├── apperr/                        # typed app errors -> HTTP status + the {"error":{...}} envelope
│   ├── jwtutil/                       # access-token sign/verify, opaque refresh-token generation/hashing
│   ├── passwordutil/                  # argon2id hash/verify
│   ├── reqctx/                        # context accessors for org/user/role/request-id + the header names they travel under
│   ├── dbx/                           # pgx pool, embedded-SQL migration runner, RLS tenant-context helper (+ the bypass-RLS escape hatch)
│   ├── redisx/                        # client wrapper, revocation cache, token-bucket rate limiter (atomic Lua script)
│   └── httpclient/                    # shared client for calling another service's REST API: timeout, one retry, header propagation, error-envelope mapping
│
├── deployments/
│   ├── Dockerfile                     # shared multi-stage build for every backend service, parameterized by --build-arg SERVICE=<services/ dir name>; build context is the repo root (needs go.work + shared/ visible), not this directory
│   ├── docker-compose.yaml            # local dev: postgres (pgvector image), redis, minio, all 5 backend services, web
│   └── configs/
│       ├── <service>.template.json    # checked in — the shape + dev-safe defaults; copy to <service>.json (gitignored) for `go run` on the host
│       └── docker/<service>.json      # checked in — compose-network hostnames (postgres, redis, minio, other services by name), mounted into containers by docker-compose.yaml
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

**A workspace has no single `./...`.** `go build ./...`/`go vet
./...`/`go test ./...`/`golangci-lint run ./...` from the repo root only
work rooted at one module's own directory — there's no single command
that spans `shared/` and every `services/<name>/` module at once. `make
build`/`vet`/`test`/`lint` loop over them for you; working inside one
module (`cd services/auth-service`), the plain commands work exactly as
in a normal single-module repo.

**Frontend stack**: React + Vite + TypeScript, plain CSS or Tailwind, React
Query for data fetching, no server-rendering needed — it's a thin client
over the REST API in `api-spec.md`. Built to static files and served either
by a tiny nginx container or directly by the API Gateway (one less moving
part for the public demo VM in `deployment-demo-strategy.md`). This is
what a recruiter opens; the Go backend is what they read the code for.

## Per-service internal package convention (Clean Architecture)

Every `services/<name>/internal/` follows the same shape — **domain →
usecase → repository/handler**, dependencies pointing inward, so
switching between services during code review needs zero re-orientation
(the API Gateway is the one exception — it has no domain/usecase of its
own, only `handler/` + `proxy/`, since its whole job is routing and
reverse-proxying, not business logic):

```
services/<name>/internal/
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
└── handler/                 # transport adapters — translate the outside world into usecase calls
    ├── handler.go            #   this service's own REST API (implements the routes documented for it in api-spec.md/microservices.md) — every service has one, since internal callers use the same REST transport as external clients
    ├── routes.go             #   RegisterRoutes(mux, h, ...) — mounts this service's routes on its own *http.ServeMux
    └── kafka/                #   Phase 2+: consumer + producer adapters (publish/subscribe wrappers around usecase calls)
```

**Dependency rule**: `handler` depends on `usecase`; `usecase` depends on
`domain` (interfaces only, never a concrete `repository` package);
`repository` also depends on `domain` (it implements domain's interfaces).
Nothing in `domain` imports anything else in the tree. This is the
standard Go Clean/Hexagonal Architecture layout (the same shape as
`bxcodec/go-clean-arch`: entity → usecase → repository → delivery) —
dependency inversion at the `domain` boundary is what makes `usecase`
unit-testable with an in-memory fake repository and no real Postgres/Kafka
in the test, which is worth being able to explain in a Staff-level
interview.
