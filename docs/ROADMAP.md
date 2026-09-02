# Implementation Roadmap

Six phases, each independently demoable and each mapped to a set of
distributed-systems topics worth being able to speak to in a Senior/Staff
backend interview. Build order is deliberately **walking skeleton →
vertical slice → breadth → depth**: Phase 1 gets one meeting through the
whole pipeline end-to-end on a single node before anything is scaled,
event-driven, or Kubernetes-native — a pattern worth naming explicitly in an
interview ("I always build a thin E2E slice before parallelizing scope").

---

## Phase 1 — MVP (walking skeleton)

**Goal**: one user, one org, upload a recording, get back a transcript —
running on `docker compose`, no Kubernetes yet.

**Tasks**
- Scaffold the Go monorepo (`go.work`, `cmd/`, `internal/platform`) per
  `folder-structure.md`.
- Postgres schema + migrations for `org`, `user`, `auth`, `meeting`
  (RLS from day one, not bolted on later).
- Auth Service: signup/login/JWT (access + refresh), argon2id hashing.
- Organization + User services: create org, invite flow, RBAC role storage.
- API Gateway: JWT middleware, routing to the above over gRPC.
- Meeting Service: presigned MinIO upload, meeting metadata, manual status
  update (no pipeline yet — status flips via a debug endpoint).
- `docker-compose.yaml`: Postgres, Redis, MinIO.
- Basic integration tests (`test/integration`) hitting real gRPC endpoints
  against docker-compose services.

**Deliverables**: a runnable `docker compose up`, `curl` scripts (or Postman
collection) that signup → login → create org → upload a file → fetch
metadata; migrations applied automatically on startup; README quickstart.

**Learning outcomes**: JWT auth design (access/refresh rotation), RBAC
modeling, multi-tenant schema design with Postgres RLS from the start,
presigned-URL upload pattern (why the API server shouldn't proxy large file
bytes), clean-architecture layering in Go.

**Interview topics covered**: authentication vs. authorization, token
rotation/revocation strategies, multi-tenant data modeling, RLS vs.
application-layer isolation trade-offs, REST API design, database schema
design for a SaaS product.

---

## Phase 2 — AI Processing

**Goal**: a real transcript, summary, and action items come out the other
end of an uploaded recording, driven by Kafka events, running local models.

**Tasks**
- Stand up Kafka (KRaft, single-node docker-compose) and the topics from
  `kafka-topics.md` relevant to this phase.
- Transcription Service: whisper.cpp server integration, `meeting.uploaded.v1`
  consumer, transcript + segment persistence, `transcription.completed.v1`.
- Ollama integration (`internal/platform` shared client): pull Llama3/Qwen/
  Mistral, health-check readiness gating.
- AI Summary Service: chunking pipeline, summarization prompt + parsing,
  `chunk.created.v1` / `summary.completed.v1`.
- Action Item Service: extraction prompt (structured JSON output), owner
  matching against `meeting.participants`, `action-item.extracted.v1`.
- Meeting Service: consume completion events to drive the status machine.
- Retry + DLQ handling for every new consumer.
- Structured logging + basic OTel tracing across the async boundary
  (trace_id in Kafka headers) — don't wait for Phase 6 to get this right,
  it's much harder to retrofit.

**Deliverables**: end-to-end demo — upload a real meeting recording, watch
status progress through Kafka-driven stages, fetch transcript/summary/action
items via REST.

**Learning outcomes**: event-driven pipeline design, idempotent consumer
patterns, prompt engineering for structured extraction against a local LLM,
running CPU-bound ML inference inside a service boundary, circuit breakers/
retries around slow external calls.

**Interview topics covered**: event-driven architecture, Kafka consumer
group semantics, at-least-once delivery + idempotency, saga-like multi-step
async workflows without a central orchestrator, designing for partial
failure, LLM integration patterns (prompting, structured output, evaluation).

---

## Phase 3 — Search & RAG

**Goal**: ask a question across the org's entire meeting history and get a
grounded answer with citations.

**Tasks**
- Enable `pgvector`, `search.chunk_embeddings` table + HNSW index.
- Search Service: `chunk.created.v1` consumer → Ollama embedding model →
  vector upsert.
- `SemanticSearch`, `FindSimilarMeetings`, `AskQuestion` (RAG) gRPC + REST
  endpoints, including the citation-mapping logic.
- Tune chunking (overlap, size) against retrieval quality; add a small
  eval script (a handful of known Q&A pairs) to sanity-check RAG answers
  don't regress as prompts change.
- Analytics Service groundwork: consume `summary.completed.v1` /
  `action-item.extracted.v1` into the rollup tables (trend/topic data now
  has something to aggregate).

**Deliverables**: `/search`, `/meetings/{id}/similar`, `/qa/ask` all working
against a corpus of multiple seeded meetings; a short demo showing a
question answered with a correct citation back to a specific meeting/timestamp.

**Learning outcomes**: vector search fundamentals (embeddings, ANN indexes,
cosine vs. L2), RAG architecture (retrieve-then-generate, grounding/citation,
hallucination mitigation), pgvector operational characteristics (index
build cost, recall/latency trade-offs), CQRS-style read models for analytics.

**Interview topics covered**: RAG system design, vector database trade-offs
(pgvector vs. a dedicated vector DB), embedding pipeline design, search
relevance/evaluation, CQRS and event-sourced read models.

---

## Phase 4 — Integrations & Automation

**Goal**: meeting output turns into real work — Slack pings, Jira tickets,
reminders — without anyone re-typing anything.

**Tasks**
- Notification Service: transactional outbox pattern, Slack webhook
  integration, SMTP (Mailhog locally) integration.
- `TicketProvider` interface + **`MockJiraProvider`** first (backing
  `notification.mock_jira_issues`, the `GET /demo/board` read-only UI, and
  manual column-move transitions) — this is what the public demo org runs
  and needs zero external account, so build and prove it before the real
  integration. See `deployment-demo-strategy.md` §3 for the full design.
- **`AtlassianJiraProvider`** as the same-phase stretch: real Jira REST API
  client (issue creation, org-scoped project config) against your own free
  Jira Cloud site — not required for the public demo, but keep it behind
  the same interface so it's a config flip (`ticket_provider = atlassian_jira`)
  for interview screen-shares, plus a `/webhooks/jira` handler for
  real-Jira bidirectional sync.
- Reminder scheduler: poll loop with Postgres advisory-lock leader
  election, `action-item.reminder-due.v1`.
- Org-level integration config CRUD (Organization Service) — Slack webhook
  URL, `ticket_provider` selection, Jira project/token (secret-ref, not
  plaintext) when Atlassian is configured.
- Wire `action-item.jira-requested.v1` end-to-end for both providers,
  including writing the returned ticket key back onto the action item.
- Failure handling: what happens when Slack/Jira is down — outbox retry
  with backoff, `notification.failed.v1` alerting rather than silent drop.

**Deliverables**: create an action item in a demo meeting, click "create
ticket," see it land on the mock Jira board (`GET /demo/board`, no login)
with the same event driving a real Jira Cloud issue when the org is
configured for `atlassian_jira`; a scheduled reminder actually fires into
Slack after its due time.

**Learning outcomes**: outbox pattern for reliable external-system
integration, idempotent external API calls (Jira/Slack de-dup on retry),
leader election without a dedicated coordination service, third-party API
client design (timeouts, retries, config per tenant), adapter/strategy
pattern for swappable external integrations.

**Interview topics covered**: transactional outbox / dual-write problem,
idempotency keys for external side effects, distributed leader election
patterns (advisory locks vs. etcd/ZK-based), webhook/integration
architecture, multi-tenant third-party credential management, designing
pluggable integrations behind a stable interface.

---

## Phase 5 — Kubernetes & CI/CD

**Goal**: the whole system runs on Kind via Helm, deployed through GitHub
Actions + ArgoCD, not `docker compose`.

**Tasks**
- Kind cluster config (`deploy/kind/`), Strimzi Kafka operator, MinIO
  operator, Bitnami Postgres/Redis charts (or self-authored StatefulSets).
- Author the umbrella + per-service Helm charts per
  `kubernetes-cicd.md` §2 (Deployment, Service, ConfigMap, HPA/ScaledObject,
  PDB, ServiceMonitor, NetworkPolicy).
- Install **KEDA**, wire `ScaledObject`s for the four Kafka-consumer AI
  services on consumer-group lag.
- GitHub Actions pipeline: path-filtered matrix build, lint/test/vuln-scan,
  GHCR push, automated `values.yaml` image-tag bump commit.
- ArgoCD: install, `AppProject`, `ApplicationSet` app-of-apps, sync waves,
  SOPS+age secret decryption flow.
- Migration-as-PreSync-hook Job per service schema.
- Load test (k6 or hey) to actually trigger HPA/KEDA scale-out and observe
  it in Grafana — don't just configure autoscaling, prove it fires.

**Deliverables**: `kind create cluster`, `argocd app sync meeting-intel`,
full system comes up from git alone; a documented demo of pushing a code
change → CI builds/pushes → ArgoCD auto-syncs → new pod version live, and a
second demo of Kafka backlog → KEDA scale-out → backlog drains → scale-in.

**Learning outcomes**: Kubernetes operational patterns (probes, PDBs,
resource management), Helm chart authoring at multi-service scale, GitOps
principles (git as single source of truth, pull-based deployment), queue-
based autoscaling with KEDA vs. plain HPA.

**Interview topics covered**: Kubernetes deployment strategies, Helm chart
design, GitOps vs. push-based CI/CD, canary/rolling update mechanics,
autoscaling strategies (CPU vs. custom metrics vs. queue-depth), operator
pattern (Strimzi/MinIO/KEDA all being CRD-driven operators is itself a
talking point).

---

## Phase 6 — Production Readiness

**Goal**: the system is observable, secure, resilient, and documented like
something a real team could operate on-call.

**Tasks**
- Full observability stack: OTel Collector, Prometheus + Alertmanager,
  Grafana dashboards (per-service RED, Kafka, Postgres, business metrics),
  Loki + Promtail, Tempo — wired per `observability-security.md` §1.
- Security hardening pass: NetworkPolicies (default-deny), mTLS via
  Linkerd (stretch), `trivy`/`gosec`/`govulncheck` gates in CI made
  blocking, `cosign` image signing (stretch).
- DR drill: implement Postgres WAL backup to MinIO, write and **actually
  execute** `docs/runbooks/dr-restore.md` against a deliberately destroyed
  local Postgres volume, record the real RTO achieved.
- Chaos testing: kill a random pod per stateful component during a load
  test, confirm self-heal + no data loss; document findings.
- Cost/resource pass: confirm the full stack budget from
  `kubernetes-cicd.md` §1 actually fits your laptop; tune KEDA scale-to-zero
  idle behavior.
- Progressive delivery stretch: Argo Rollouts canary on the API Gateway
  with automated Prometheus-based analysis.
- Write the interview-facing artifacts: an architecture README with the
  HLD diagram, a one-page "design decisions and trade-offs I'd defend"
  doc, and a short incident-response runbook.

**Deliverables**: Grafana dashboards screenshot-ready for a portfolio;
a written DR drill report with measured RTO/RPO; a security checklist with
every item checked or explicitly deferred with rationale; the project in a
state where it could survive a "walk me through how this fails and
recovers" interview question live.

**Learning outcomes**: full-stack observability instrumentation, security-
in-depth for a multi-tenant system, disaster recovery planning and
*verification* (not just documentation), chaos engineering basics, cost/
resource governance.

**Interview topics covered**: observability (metrics/logs/traces
correlation), distributed tracing design, security architecture for
multi-tenant SaaS, disaster recovery (RPO/RTO, backup/restore strategy),
chaos engineering, SRE practices (SLOs, alerting philosophy, runbooks).

---

## Phase 7 — Public Demo Deployment (Resume-Ready)

**Goal**: a URL that goes on the resume — always up, loads instantly, safe
against abuse, and usable by a stranger with zero setup. Full design in
`architecture/deployment-demo-strategy.md`.

**Tasks**
- Provision the Oracle Cloud Free Tier Ampere A1 VM (4 OCPU/24GB, free
  forever); fall back to Render/Fly.io free tiers if signup friction is a
  blocker.
- Trim the stack to plain `docker compose` on that one VM: Postgres+pgvector,
  Redis, Kafka (single KRaft broker), MinIO (or Cloudflare R2 free tier),
  Ollama with a small quantized model (`qwen2.5:3b`/`phi3:mini`), whisper.cpp
  `base`.
- Front it with a **Cloudflare Tunnel** for HTTPS + a subdomain, no port
  forwarding, no domain purchase required.
- Seed a demo org: 3-4 pre-processed sample meetings (transcript, summary,
  action items, indexed for RAG) so the landing experience is instant, not
  a cold-start pipeline run.
- Enable a real, rate-limited live upload path (≤2 min clips, ~1/hour per
  IP via the existing Redis rate limiter) so a visitor can also see the
  actual Kafka-driven pipeline run, not just canned data.
- Confirm the demo org defaults to `MockJiraProvider` (from Phase 4) so the
  "create ticket" flow works with no Jira account — `GET /demo/board` is
  reachable and public.
- Write the README banner distinguishing this deployment from the
  Kind/Kafka/ArgoCD reference architecture (see decision in
  `PROJECT_PLAN.md` §5), and record the 2-3 minute local-cluster walkthrough
  video (ArgoCD sync + a KEDA scale-out) referenced from it.

**Deliverables**: a public URL, live, on the resume; a repo README that
sends a technical reviewer from "click the link and try it" to "read the
docs and watch the video" for the parts a single free VM can't show.

**Learning outcomes**: production-vs-demo deployment trade-offs on a $0
budget, tunneling/ingress without a static IP or paid domain, rate limiting
and abuse-resistant design for a publicly writable pipeline, communicating
architecture scope honestly to a technical audience.

**Interview topics covered**: deployment topology trade-offs, cost-aware
infrastructure decisions, rate limiting/abuse prevention design, the
difference between a demo environment and a production environment (a
question that comes up directly when a project has a live link).

---

## Cross-Phase Interview Prep Map

| Topic cluster | Phases where it's built | Where to point in a live interview |
|---|---|---|
| Distributed systems / event-driven architecture | 2, 3, 4 | Kafka topic design, idempotent consumers, outbox pattern |
| Microservices design | 1–6 | Per-service LLD in `architecture/microservices.md`, layering convention in `folder-structure.md` |
| Kubernetes & GitOps | 5 | Helm charts, ArgoCD ApplicationSet, KEDA scaling proof |
| AI/LLM systems | 2, 3 | Local model integration, RAG design, prompt/extraction reliability |
| Multi-tenancy | 1, 6 | RLS policies, isolation layers table in `observability-security.md` §3 |
| Observability/SRE | 2 (basics), 6 (full) | Grafana dashboards, trace correlation demo |
| Security | 1, 4, 6 | JWT/RBAC design, secrets management, DR drill report |
| Integration/adapter patterns | 4 | `TicketProvider` interface, mock-vs-real Jira in `deployment-demo-strategy.md` §3 |
| Deployment strategy | 5, 7 | Reference architecture (local Kind) vs. public demo (Oracle VM) split, `deployment-demo-strategy.md` §2 |
