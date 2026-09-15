# Folder Structure — Go Workspace + Frontend

Single repo, but the backend is a **Go workspace** (`go.work` at the repo
root): `shared/` is its own Go module, and every `services/<name>/` is a
separate Go module with its own `go.mod`, a `main.go` sitting right next
to it (no `cmd/` subdirectory — each service is small enough that one
entrypoint at the module root reads clearer than a level of nesting for
it). `entity/`, `usecase/`, `repository/` (and, where a service has one,
`client/`/`storage/`) stay their own importable packages, flat at the
module root, parallel to `migrations/` — no `internal/` directory
anywhere in a service, and (see "no more domain/" below) no `domain/`
either. `handler.go` and
`routes.go` sit at that same module root too, as plain files next to
`main.go`, not their own packages — see "handler.go and routes.go are
`package main`" below for why. `go.work` lists all the modules
under `use` so
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
across its own module boundary.

Each service's own `entity/usecase/repository` packages don't sit under
an `internal/` directory either — they're flat at the module root,
parallel to `migrations/` and `main.go`. That does give up something
real: `internal/` is Go's one compiler-enforced privacy mechanism, and
without it nothing stops another module in this workspace from
importing, say, `services/auth-service/usecase` directly. In practice
nothing does — each service's own `main.go` is the only consumer of its
packages, `shared/` is what every service is actually meant to share,
and the module boundary (a separate `go.mod` per service) already keeps
one service's code out of another's build unless something explicitly
imports it. The privacy `internal/` would add on top of that is a safety
net this repo has chosen to go without, in exchange for one less
directory level in every service.

## No more `domain/`

Earlier revisions had a `domain/` package per service holding three
things: the service's entities (`entity.go`), its repository interface
(`repository.go`), and its own named error values (`errors.go`, e.g.
`ErrOrgNotFound = apperr.NotFound(...)`). All three moved out, each for
a different, concrete reason — not to shrink the tree for its own sake:

- **Entities moved into `entity/`.** `domain/entity.go`'s structs
  (`User`, `Meeting`, `Organization`, and so on — what `repository`
  reads out of Postgres and `usecase` operates on) merged into the same
  `entity/entity.go` that already held the service's REST-boundary wire
  structs. One package now describes every plain data shape in the
  service, wire and internal alike — see "what's in `entity/` now"
  below for how the file stays organized despite holding both.

- **Repository interfaces moved into `repository/`.** A repository
  interface (`Repository`, or for auth-service `CredentialsRepository`/
  `RefreshTokenRepository`/`PasswordResetRepository`) is only ever
  implemented by exactly one thing, `repository/postgres`, one directory
  below it. Defining it in a `domain` package elsewhere in the tree put
  it further from that implementation than it needed to be; defining it
  in `repository/interface.go` — the parent of `repository/postgres`
  — puts it exactly where its one implementation lives, and `usecase`
  still only ever depends on the interface type, never on
  `*postgres.OrgRepository` directly, which is what actually makes it
  unit-testable with an in-memory fake. The same move applies to every
  other interface-with-one-implementation in a service: auth-service's
  `OrgClient`/`UserClient` (implemented by `client/http`, so they live in
  `client/interface.go`) and meeting-service's `ObjectStorage` (implemented
  by `storage/minio`, so it lives in `storage/interface.go`) — `interface.go`
  is the file name every one of these gets, regardless of which
  top-level package (`repository`, `client`, `storage`, ...) it sits in,
  so "where's the interface for this?" always has the same one-word
  answer.

- **Named errors got inlined.** `domain/errors.go` held package-level
  `var`s like `ErrOrgNotFound = apperr.NotFound("organization not
  found")`, each used at exactly one or two call sites. The indirection
  didn't buy comparison-by-identity (nothing in this codebase does
  `errors.Is(err, ErrOrgNotFound)` across a service boundary — apperr's
  envelope, not the Go error value, is what a caller actually sees) or
  reuse across many call sites, so the file is gone and each call site
  now constructs its `apperr.NotFound(...)`/`apperr.Unauthorized(...)`/
  etc. directly. One error that turned out unused entirely
  (user-service's `ErrInvalidLogin`) simply disappeared rather than
  needing a new home.

The upshot: nothing named `domain` exists in any service anymore, and
every interface lives in the same package as (one directory above) its
one real implementation.

## handler.go and routes.go are `package main`

Every service's `handler.go` and `routes.go` sit directly beside
`main.go` — not in their own `handler/`/`routes/` subdirectories the way
they briefly did, and not in even their own separate packages. All
three files share `package main`. This is a further step past the
`entity`/`usecase`/`repository` flattening above, and a
different trade-off from it: those stayed separate *packages* (just
not nested under `internal/`), so `usecase.SignupUseCase` is still a
qualified, explicit reference wherever it's used. `Handler` and
`RegisterRoutes` are not qualified anywhere anymore — `main.go` calls
`NewHandler(...)` and `RegisterRoutes(srv.Mux, handler)` directly, no
package prefix, because there's no longer a package boundary between
them to cross. `handler.go` and `routes.go` stay two files rather than
one precisely so "what each route does" and "which path+method maps to
which method" are still easy to find independently — the file boundary
carries that distinction now, not a package boundary. This works only
because `main` is exactly one package per service already (Go requires
every `.go` file in a directory to agree on their package, and a
service's directory was always going to be `package main` for its
entrypoint) — it's the same directory-is-a-package rule that made
`shared/` need to move out from under `internal/` in the first place,
just applied in the opposite direction here: instead of moving a package
out to preserve import access, these two packages moved *in* to sit
alongside `main` and gave up being separately importable at all.

```
.
├── go.work                            # lists shared/ + every services/<name>/ module
├── Makefile                           # build/vet/test/lint loop over every module; up/down (docker compose)
├── README.md
│
├── services/
│   ├── api-gateway/
│   │   ├── go.mod
│   │   ├── main.go
│   │   ├── router.go                  # package main — Register(mux, urls, ...): routing+middleware+proxy composition; no usecase/repository, no separate handler.go (routing is this service's whole job)
│   │   └── proxy/                     # net/http/httputil.ReverseProxy wrapper — the one subpackage the gateway still has, since it's a real reusable concern, not wiring
│   │
│   ├── auth-service/
│   │   ├── go.mod
│   │   ├── main.go
│   │   ├── handler.go                 # package main — what each route does (decode entity.*Request, call usecase, encode entity.*Response)
│   │   ├── routes.go                  # package main — RegisterRoutes(mux, h) mapping path+method to a Handler method; see "handler.go and routes.go are package main" above
│   │   ├── migrations/{0001_init.up.sql, embed.go}
│   │   ├── entity/                    # every plain data shape in the service — its own model AND its REST-boundary wire structs; see "no more domain/" above
│   │   ├── usecase/                   # signup, login, refresh, logout, password reset, token issuer
│   │   ├── repository/
│   │   │   ├── interface.go           #   CredentialsRepository/RefreshTokenRepository/PasswordResetRepository interfaces
│   │   │   └── postgres/              #   their one implementation, one directory below the interfaces
│   │   └── client/
│   │       ├── interface.go           #   OrgClient/UserClient interfaces
│   │       └── http/                  #   their one implementation — outbound REST calls to org/user services, built on shared/httpclient
│   │
│   ├── user-service/       (go.mod, main.go, handler.go, routes.go, migrations/, entity/, usecase/, repository/{interface.go, postgres/})
│   ├── organization-service/ (same shape)
│   ├── meeting-service/     (same shape, + storage/{interface.go, minio/} for the ObjectStorage port — see storage/minio's own doc comment for the internal/public MinIO endpoint split)
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

## Per-service package convention (Clean Architecture)

Every `services/<name>/` follows the same shape — **entity ← usecase →
repository (interface next to implementation) ← handler**, dependencies
pointing inward, so switching between services during code review needs
zero re-orientation (the API Gateway is the one exception — it has no
usecase/repository of its own, only `router.go` + `proxy/`, since its
whole job is routing and reverse-proxying, not business logic):

```
services/<name>/
├── main.go                 # package main — wiring only: load config, construct repositories/usecases/Handler, call RegisterRoutes; each step is its own named function (loadConfig, initPostgres, ...) so main() itself reads as a short list of steps, not one long block
├── handler.go               #   package main too — what each route does: decodes an entity.*Request, calls usecase, encodes an entity.*Response
├── routes.go                #   package main too — RegisterRoutes(mux, h) maps each path+method to one Handler method; see "handler.go and routes.go are package main" above
│
├── entity/                 # every plain data shape in the service — no external deps, nothing here imports anything else in this tree
│   └── entity.go           #   the service's own model (e.g. Meeting — what repository reads and usecase operates on) AND its REST-boundary wire structs (e.g. CreateMeetingRequest, MeetingResponse) AND usecase's own Input/Output structs — all json-tagged where they cross the wire, none of it with behavior
│
├── usecase/                # business logic — implements the use cases, depends on repository's interface (never its postgres subpackage) and entity
│   ├── create_meeting.go
│   ├── list_action_items.go
│   └── ...                #   one file (or a few grouped) per use case; unit-tested against an in-memory fake satisfying repository's interface
│
└── repository/             # the interface usecase depends on, right next to its one implementation
    ├── interface.go        #   interface(s): Repository, or (auth-service) CredentialsRepository/RefreshTokenRepository/PasswordResetRepository — defined by what usecase needs, referencing entity types
    └── postgres/           #   pgx-backed implementation, owns exactly this service's schema, sets tenant context
```

A service that talks to another service directly instead of (or as well
as) Postgres gets the same shape under a differently-named top-level
package instead of forcing it into `repository/`: auth-service's
`client/interface.go` (interfaces) + `client/http/` (implementation) for
`OrgClient`/`UserClient`, meeting-service's `storage/interface.go` +
`storage/minio/` for `ObjectStorage`. `interface.go` is the file name in
every case, whatever the enclosing package is called — the principle is
the same regardless: the interface lives with its one real
implementation, and `usecase` depends on the interface type, never the
concrete one.

`entity`, `usecase`, and `repository` (and `client`/`storage`, where a
service has them) stay separate packages — `usecase` depending on
`repository`'s interface, never the concrete `*postgres.OrgRepository`,
is what makes it unit-testable with an in-memory fake and no real
Postgres. `handler.go` and `routes.go` are the exception: both `package
main`, both living beside `main.go` rather than in their own packages —
see "handler.go and routes.go are package main" above for why that
boundary specifically was worth giving up. They still stay two *files*,
not one: "what a route does" (handler.go) and "which path+method maps
to which method" (routes.go) are genuinely different questions, and
keeping them apart means changing one never risks touching the other by
accident, even without a package boundary enforcing it.

**Dependency rule**: `handler.go` depends on `usecase` and `entity`;
`usecase` depends on `repository`'s interface (never its `postgres`
subpackage directly) and `entity`; `repository/postgres` depends on
`entity` (it implements `repository`'s interface, working in terms of
`entity`'s types) — same for `client`/`http` and `storage`/`minio`.
Nothing in `entity` imports anything else in the tree — it's pure data.
This is the standard Go Clean/Hexagonal Architecture layout (the same
shape as `bxcodec/go-clean-arch`: entity → usecase → repository →
delivery) — dependency inversion at the `repository` interface boundary
is what makes `usecase` unit-testable with an in-memory fake repository
and no real Postgres/Kafka in the test, which is worth being able to
explain in a Staff-level interview.
