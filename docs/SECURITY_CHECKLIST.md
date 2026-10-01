# Security Checklist

Every item from `docs/architecture/observability-security.md` §2, checked
against what's actually in this codebase today (not what the design doc
merely describes) — Phase 6's own deliverable calls for exactly this: "a
security checklist with every item checked or explicitly deferred with
rationale." Where the code and the doc disagree, that's called out
explicitly rather than the doc being copied as fact.

## AuthN

- [x] Password hashing: **argon2id** (`shared/passwordutil`) — done.
- [x] Refresh tokens: 7-day TTL, single-use, rotation links
  `replaced_by` so a stolen-and-reused old token is detectable
  (`services/auth-service/usecase/token_issuer.go`) — done.
- [~] Access tokens: 15-minute TTL as documented, but signed
  **HS256** (`shared/jwtutil.go`), not the **RS256** the architecture doc
  claims. HS256 means every service holding `jwt_signing_key` can both
  mint and verify tokens; RS256 would let services hold only a public key
  for verification. This is a real doc/code divergence, found while
  writing this checklist, not a judgment call already made and
  documented elsewhere — **deferred**: fixing it is either an honest doc
  correction (describe what's actually built) or an actual RS256
  migration (regenerate every service's trust config), and this pass
  does neither.

## AuthZ / RBAC

- [x] Role embedded in JWT claims, enforced at two layers: the API
  Gateway's `RequireRole` middleware (coarse, route-level) and each
  service's own per-endpoint re-check (e.g. `requireOwnerOrAdmin`
  wrappers across organization-service/notification-service/user-service)
  — defense in depth, built incrementally from Phase 2 through every
  later phase that added a role-gated route. Done.

## Multi-Tenant Isolation (§3)

- [x] Every repository query filters by `org_id` explicitly — done,
  and the actual enforcement mechanism (see next line).
- [ ] Postgres Row-Level Security: policies exist in the schema but are
  **inert** — every service connects as the table owner/superuser, which
  RLS never applies to (`docs/architecture/database-schema.md`'s own RLS
  section documents the incident this was found from, back in Phase 2.3).
  **Deferred, long-standing**: giving each service's own DB connection a
  non-superuser role is the fix, not tied to any one phase, named
  consistently since it was found.
- [x] MinIO: object keys namespaced `org_id/meeting_id/...`, access only
  via short-lived presigned URLs — done.
- [x] Kafka: every payload carries `org_id`; consumers apply tenant
  context before any DB write — done.

## Transport Security

- [ ] **TLS at ingress** (cert-manager + mkcert, per the doc): not built.
  Phase 5 exposes api-gateway via a bare NodePort for Kind-local use, no
  ingress controller or TLS termination layer exists anywhere in this
  repo. Plaintext HTTP end to end, same as docker-compose always was.
- [x] Intra-cluster NetworkPolicies: default-deny + explicit allow,
  tightened in this same Phase 6 pass from Phase 5's original
  namespace-wide-any-port rule to per-namespace port scoping (see every
  service chart's own `templates/networkpolicy.yaml`).
- [ ] **mTLS via Linkerd**: explicit stretch item in the architecture doc
  itself ("documented as a Phase 6 hardening step and interview topic").
  Not installed. Named as a stretch, not silently dropped.

## Secrets Management

- [x] Never in git or a committed `values.yaml` in plaintext — every
  service chart's dev-safe secrets are placeholder values, same
  philosophy as `deployments/configs/*.template.json`; real values are
  meant to come from a separate, gitignored values file or SOPS+age
  (`deploy/argocd/SECRETS.md`).
- [~] Delivery mechanism differs from the doc's own wording ("K8s Secret +
  `envFrom`"): this repo mounts each service's Secret as a **file**
  (`-secrets` flag, `shared/config.LoadMerged`), not as individual env
  vars via `envFrom` — a deliberate Phase 6 design choice matching how
  these services already read one JSON config file, not an oversight,
  but worth naming since it doesn't match the doc's literal mechanism.
- [ ] SOPS+age flow itself: **documented, not wired**. No
  `secrets.enc.yaml` exists, no `ksops` plugin is installed into any
  ArgoCD repo-server — `deploy/argocd/SECRETS.md` is a copy-and-run setup
  guide, not a built pipeline. Named plainly in that file's own opening
  section.
- [x] Rotation: manual, as the doc specifies (no Vault dynamic secrets —
  explicitly out of scope).

## Input Validation & Abuse Prevention

- [ ] **`go-playground/validator` struct-tag validation at the gateway
  boundary**: not implemented anywhere in this codebase. Every handler's
  own hand-written checks (e.g. `apperr.BadRequest("title is required")`)
  are what exists instead — ad hoc, not a systematic validation layer.
- [ ] **File-upload type/size validation before issuing a presigned URL**:
  not implemented — `CreateUploadIntentUseCase`
  (`services/meeting-service/usecase/create_upload_intent.go`) validates
  only that a title was supplied, then presigns unconditionally. A
  real deployment should reject by content-type/size before ever handing
  out a write-capable URL.
- [~] **Rate limiting**: built, but narrower than documented. What
  exists: a per-IP token bucket on `/auth/signup`, `/auth/login`, and
  password reset (`shared/middleware/ratelimit.go`, wired in Phase 1).
  What the doc also calls for and isn't built: a general **per-(org_id,
  user_id)** token bucket across authenticated routes broadly — nothing
  in this repo rate-limits an already-logged-in user's ordinary API
  traffic.

## Dependency & Image Security

- [x] `govulncheck` + `trivy` in CI — both **blocking** as of this Phase
  6 pass (`ci.yaml`'s own trivy step was report-only through Phase 5;
  `security.yaml`'s weekly gosec/trivy/govulncheck run was already
  blocking since Phase 5).
- [x] Distroless/minimal base images — every service's `deployments/Dockerfile`
  final stage is `gcr.io/distroless/static-debian12:nonroot` — done since
  Phase 5.
- [ ] **`cosign` image signing**: explicit stretch item in the
  architecture doc ("as a stretch goal"). Not implemented.

## Audit Trail

- [~] The *events* the doc names as a natural audit log
  (`action-item.status-changed.v1` and others) are genuinely produced —
  but `user.role-changed.v1`/`user.login-failed.v1` specifically, also
  named in that doc row, were never actually wired as real Kafka topics
  in any phase (see `deploy/infra/strimzi-kafka/values.yaml`'s own
  comment on the topics documented in `kafka-topics.md` but never given a
  producer). **No dedicated audit consumer exists** (Analytics Service
  does rollups, not a persisted security-event log with longer
  retention) — this whole row is a named, not-yet-started follow-up, not
  partially faked.

## Summary

Solidly done: password hashing, refresh token rotation, two-layer RBAC,
application-level tenant isolation, NetworkPolicies (tightened this
phase), blocking dependency/image scanning, distroless images.

Real, named gaps — not silently glossed over: the HS256-vs-RS256 doc
mismatch, inert RLS (long-standing), no TLS anywhere, Linkerd mTLS and
cosign signing (both explicit stretch), SOPS+age undeployed, no
systematic request validation, no upload validation, rate limiting
narrower than documented, and no dedicated audit log. None of these are
required for this repo's actual goal (a demonstrable, interview-ready
system) to be met — see the main `README.md`'s own Phase 6 section for
how that goal is scoped — but a real production deployment of this design
would need to close every one of them first.
