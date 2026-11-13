# Interview Prep: Questions to Expect About This Project

A study guide, not a script — every question below is paired with a one-line
pointer to where the *real* answer lives in this repo (a file, a doc
section, a design decision), not a canned paragraph to memorize. Walk
through the pointer, understand why that choice was made, and you can
answer follow-ups a memorized answer can't survive. Organized to match how
`docs/ROADMAP.md`'s phases actually built the system, plus a cross-cutting
section at the end for "tell me about a trade-off" style questions.

See also: `docs/ROADMAP.md`'s own "Cross-Phase Interview Prep Map" table,
`docs/DESIGN_DECISIONS.md` (trade-offs already written up), and
`docs/SECURITY_CHECKLIST.md` (gaps already audited) — several questions
below point straight at those instead of restating them.

## 1. Elevator-pitch / scoping questions

- What does this system actually do, end to end, for one uploaded meeting?
  → `README.md`'s Quickstart + `docs/architecture/kafka-topics.md`'s
  event-flow diagram (upload → transcript → summary → action items →
  search index, all Kafka-driven).
- Why microservices instead of a monolith for a project this size?
  → `docs/architecture/microservices.md`'s intro — the real reason given is
  demonstrating service boundaries and independent scaling, named
  explicitly as the point of the exercise, not a trend-chase.
- Why no gRPC — isn't that unusual for microservices?
  → `docs/architecture/microservices.md` §"Internal Communication" and
  `docs/DESIGN_DECISIONS.md`'s first entry — plain REST/JSON chosen for
  operational simplicity given a simple request/response call graph.
- Walk me through the repo layout and why it's organized that way.
  → `docs/architecture/folder-structure.md` — Clean Architecture layering
  (domain/usecase/repository/delivery) per service, `go.work` tying 12
  modules together.

## 2. Authentication, authorization, multi-tenancy

- How does auth work — sessions or tokens? Access vs. refresh?
  → `services/auth-service/usecase` — JWT access+refresh, argon2id hashing;
  `docs/architecture/observability-security.md` §2.
- Why HS256 instead of RS256, if the architecture doc specifies RS256?
  → `docs/DESIGN_DECISIONS.md`'s HS256 entry and `docs/SECURITY_CHECKLIST.md`
  — a real, disclosed doc/code mismatch, not hidden. Be ready to say what
  RS256 would cost/buy here (no service needs the signing key at all).
  rotation/revocation strategy?
  → how is a refresh token revoked — `shared/redisx` / auth-service's
  revocation check (Redis-backed).
- How is RBAC enforced — once, or more than once?
  → Two-layer: gateway `RequireRole` pre-check + each service's own
  re-check (e.g. `user-service/routes.go`'s `requireOwnerOrAdmin`). Ask
  yourself: why re-check per-service instead of trusting the gateway?
  (Defense in depth — a service shouldn't assume every caller went through
  the one gateway you know about today.)
- How is a tenant's data actually isolated — RLS?
  → `docs/DESIGN_DECISIONS.md`'s RLS entry: **RLS policies exist in the
  schema but are currently inert** (every service connects as table owner,
  which RLS doesn't restrict) — the real enforcement is explicit `org_id`
  filtering in every repository query. This is a documented *finding*, not
  an assumption — know why RLS staying in the schema anyway is still worth
  defending (free defense-in-depth once per-service DB roles exist).
- Why did full RBAC (roles beyond owner/member) wait until Phase 2 instead
  of shipping in Phase 1?
  → `docs/ROADMAP.md` Phase 1's own note: ship the simplest correct
  permission model, add roles once there's a second role to actually
  enforce — deliberate YAGNI, not an oversight.

## 3. Event-driven architecture & Kafka

- Why Kafka instead of a simpler queue (SQS/RabbitMQ) for this?
  → `docs/architecture/kafka-topics.md`'s intro — consumer-group fan-out,
  replay, and it's the thing worth demonstrating KEDA scaling against in
  Phase 5.
- What's your delivery guarantee — at-least-once? How do consumers stay
  idempotent under redelivery?
  → look at any `consumer.go` (e.g. meeting-service's) — explain what
  happens if the same Kafka message is processed twice.
- How does a trace follow a request across the sync→async boundary?
  → `shared/kafkax`'s `HeaderTraceparent` doc comment — traceparent minted
  at upload-confirm time, propagated as a Kafka header, children stamped by
  every downstream consumer. One `trace_id` greps a whole pipeline run.
- What happens to a message that keeps failing? Where's your DLQ?
  → retry+DLQ handling named per-consumer in Phase 2's task list; trace the
  actual implementation in whichever service's consumer you're asked about.
- Walk me through what "KEDA scaling on Kafka lag" actually demonstrates,
  live.
  → `deploy/helm/meeting-intel/charts/*/values.yaml`'s `scaledObject`
  blocks + `docs/ROADMAP.md` Phase 5's own load-test deliverable ("don't
  just configure autoscaling, prove it fires").

## 4. AI/LLM integration

- Which models, and why local instead of a hosted API?
  → already answered in this same conversation — see the "which model"
  exchange above, or `docs/PROJECT_PLAN.md`'s tech stack row. Know the
  three models by name: whisper.cpp (`ggml-base.en`) for ASR, Qwen 2.5
  (`7b` normally, `3b` in the trimmed demo) for summarization/extraction/
  RAG generation, `nomic-embed-text` for embeddings.
- How do you get *structured* output (action items as JSON) from an LLM
  reliably?
  → `action-item-service`'s extraction prompt + parsing — be ready to talk
  about what happens when the model returns malformed JSON (this is a real
  "evaluation" learning outcome named in Phase 2).
- Why pgvector instead of a dedicated vector DB (Pinecone/Weaviate)?
  → `docs/ROADMAP.md` Phase 3's interview-topics line: "pgvector vs. a
  dedicated vector DB" — one fewer moving part, data lives next to its
  relational metadata, HNSW index covers this scale.
- Walk me through the RAG pipeline end to end, including how a citation
  maps back to a specific meeting/timestamp.
  → `search-service`'s `AskQuestion` usecase — retrieve-then-generate, the
  citation-mapping logic specifically (this is the "hallucination
  mitigation" piece interviewers probe).
- What's the real embedding dimension and why does it matter?
  → `services/search-service/migrations/0001_init.up.sql`:
  `VECTOR(768) NOT NULL` — matches `nomic-embed-text`'s native output;
  changing embedding models later means a real re-index, not a config flip.
- How did you tune chunking, and how do you know RAG answers didn't
  regress as prompts changed?
  → Phase 3's own task: a small eval script against known Q&A pairs — be
  ready to say what it actually checks.

## 5. Outbox pattern, integrations, idempotency

- What's the transactional outbox pattern and why use it here instead of
  calling Slack/Jira directly from the Kafka consumer?
  → `docs/DESIGN_DECISIONS.md`'s outbox entry — the trade-off is a few
  seconds of latency + one more table, for a guarantee (every accepted
  message eventually produces an attempt, survives a pod crash mid-send).
- How do you avoid double-posting to Slack/Jira on retry?
  → idempotency keys on the external call — named directly in
  `docs/ROADMAP.md` Phase 4's interview-topics list.
- Why build `MockJiraProvider` before the real Atlassian integration?
  → `docs/architecture/deployment-demo-strategy.md` §3 — a stranger
  clicking a resume link has no Jira account; the adapter pattern
  (`TicketProvider` interface) makes swapping providers a config flip
  (`ticket_provider`), a real strategy pattern not a workaround.
- How does the reminder scheduler avoid two replicas double-sending the
  same reminder?
  → Postgres advisory-lock leader election, named in Phase 4's learning
  outcomes — contrast with etcd/ZooKeeper-based leader election and be
  ready to say why advisory locks were enough here (no separate
  coordination service needed).

## 6. Kubernetes, Helm, GitOps (Phase 5)

- Why Helm charts per service instead of one giant chart?
  → `deploy/helm/meeting-intel/`'s structure — umbrella + 11 subcharts +
  a shared `meeting-intel-common` library chart for repeated template
  helpers; independent versioning/values per service.
- What's the role of each operator here — Strimzi, the MinIO operator,
  KEDA — and why operators at all?
  → `docs/ROADMAP.md` Phase 5's own interview-topics line calls this out
  directly: "operator pattern ... is itself a talking point" — CRD-driven
  lifecycle management instead of hand-rolled StatefulSets for the tricky
  parts.
- Why GitOps (ArgoCD pull-based) instead of a push-based pipeline
  (GitHub Actions `kubectl apply`)?
  → `deploy/argocd/`'s sync-wave design — git as the single source of
  truth, a compromised CI credential can't push straight to the cluster.
- What are ArgoCD sync waves for, concretely, in this repo?
  → `deploy/argocd/infra-applications.yaml`'s wave ordering (-1 for infra
  like observability, 0 for things like the backup CronJob, 2 for the app
  ApplicationSet) — infra must exist before the apps that depend on it.
- How do migrations run in a Kubernetes deployment, since there's no
  `docker compose up` to run them on startup the same way?
  → migration-as-PreSync-hook Job per service schema, named in Phase 5's
  task list.

## 7. Observability, security hardening, DR, chaos (Phase 6)

- Walk me through how a single slow request is diagnosed — metrics, logs,
  traces, in what order?
  → `docs/runbooks/incident-response.md` §2's own "metrics → logs → traces"
  correlation flow, including the stated gap (Postgres queries and Kafka
  hops aren't traced — only HTTP legs are).
- What's your alerting philosophy — what are the 5 alerts and why those?
  → `deploy/infra/observability/templates/alerts.yaml` — know each one's
  real signal (consumer lag, HTTP error rate, AI pipeline latency,
  notification DLQ depth, PVC near-full) and why a vague "CPU high" alert
  isn't one of them.
- Is this system secure for a multi-tenant SaaS? What's actually been
  checked vs. deferred?
  → `docs/SECURITY_CHECKLIST.md` — go in knowing the real gaps verbatim:
  HS256 vs. documented RS256, narrower rate limiting than designed, no
  request/upload validation, Linkerd mTLS as an explicit stretch goal.
  Never claim this is "fully secured" — the checklist's own honesty is the
  actual interview-worthy artifact here.
- Walk me through your DR story — what's the RTO/RPO?
  → `docs/runbooks/dr-restore.md` — be upfront that this was **authored,
  not executed** in this project's own dev sandbox (no live cluster
  available there); RTO/RPO are stated as targets, not measured numbers.
  This honesty is itself the right answer to "have you actually tested
  this" — never claim a number that was never measured.
- What would chaos-testing a stateful component (Postgres, Kafka, MinIO)
  actually verify here?
  → `scripts/chaos-test.sh` + `docs/runbooks/chaos-test-findings.md` — a
  real runnable tool with a blank findings template, same "written, not
  yet executed" honesty as the DR runbook.
- Does the full stack actually fit a 16GB laptop?
  → `docs/COST_RESOURCE_AUDIT.md` — the real, computed answer is **no, not
  comfortably** (13.12Gi requests alone is ~82% of 16GB before any OS
  overhead), with Ollama/whisper.cpp named as the dominant cost. Know the
  concrete levers to shrink it (drop the AI pipeline: ~7Gi; disable the
  observability stack: ~1.5Gi).

## 8. Public demo deployment (Phase 7)

- Why two different deployment topologies (Kind/Kafka/ArgoCD vs. a single
  VM) instead of running the production stack in both places?
  → `docs/architecture/deployment-demo-strategy.md` §2's comparison table —
  the reference architecture proves you can build the real thing; the
  public demo is a deliberately cost-trimmed version, and conflating them
  is the mistake this design explicitly avoids.
- How do you protect a publicly-writable upload endpoint from abuse?
  → `services/api-gateway/router.go`'s `demo_mode` switch (an extra
  ~1/hour-per-IP token-bucket limit) + `ConfirmUploadUseCase`'s size cap as
  a practical stand-in for a true clip-length limit (the service never
  decodes audio, so it can't measure true duration — know why that
  limitation exists and what the honest fix would be: probing duration at
  transcription time).
- How does HTTPS work on the demo without a paid cert or a static IP?
  → Cloudflare Tunnel — `deployments/cloudflared/README.md`; the real
  security property worth naming: **zero inbound ports ever open** on the
  VM, since the tunnel only makes outbound connections.
- What's the actual cost of the public demo, and what did you learn
  provisioning it?
  → be ready to talk about the real Oracle Cloud Free Tier friction this
  project hit: Always-Free Ampere A1 capacity contention in a popular
  region, the single-region limit on a trial tenancy, and the fallback
  decision tree (retry vs. a smaller Ampere slice vs. a paid trial-funded
  shape vs. running locally with a tunnel) — this is a real "production
  deployment isn't just `terraform apply`" story, worth having even though
  it's not in the written docs.

## 9. Cross-cutting: trade-offs, failure modes, "what would you change"

These are the ones most likely to be asked as open-ended follow-ups rather
than from a specific phase — `docs/DESIGN_DECISIONS.md` is written exactly
for this category, know every entry in it:

- "Walk me through how this fails and recovers" (the roadmap's own framing
  for Phase 6's whole point) — pick one real failure (a pod crash
  mid-outbox-send, a Kafka consumer redelivery, a Postgres primary loss)
  and narrate it precisely using the real mechanism, not a hand-wave.
- "What's the one thing you'd redesign if you had another month?" — have a
  real, specific answer (candidates already named in this repo: real
  per-service Postgres roles to make RLS actually enforce something,
  tracing Postgres/Kafka hops not just HTTP legs, KEDA scale-to-zero tuned
  against a real idle cluster rather than guessed at).
- "Where did you cut a corner on purpose, and how do you know it's safe?"
  → the split-Helm-chart entry in `docs/DESIGN_DECISIONS.md` is the
  cleanest worked example: both scaling Deployments run the same binary
  underneath, stated plainly in that chart's own `Chart.yaml`, not hidden.
- "What's the actual scope line you drew, and why there?" →
  `docs/DESIGN_DECISIONS.md`'s closing section: Linkerd mTLS, `cosign`
  image signing, Argo Rollouts canaries all named as stretch goals in the
  architecture doc itself and deliberately left there — know why adding a
  half-wired version of any of them would be worse than clearly naming it
  undone.
