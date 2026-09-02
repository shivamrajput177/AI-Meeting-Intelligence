# Observability, Security, Multi-Tenancy, DR, Cost Optimization

## 1. Observability

### Metrics (Prometheus + Grafana)
- Every Go service exposes `/metrics` via `prometheus/client_golang`,
  scraped through the Prometheus Operator's `ServiceMonitor` CRD (attached
  by the shared Helm template).
- Standard **RED** metrics per service: `http_requests_total{method,route,
  status}`, `http_request_duration_seconds` (histogram), `grpc_server_
  handled_total`, `grpc_server_handling_seconds`.
- Kafka: `kafka_consumergroup_lag` via `kafka-exporter` — the primary signal
  for KEDA scaling and for the "pipeline is backed up" alert.
- Postgres: `postgres_exporter` — connections, replication lag, RLS-bypass
  audit query, slow query rate.
- Business metrics (custom): `meetings_processed_total`,
  `transcription_duration_seconds`, `llm_call_duration_seconds{model}`,
  `action_items_extracted_total`, `rag_query_duration_seconds`.
- Grafana dashboards: one per service (RED), one Kafka overview (lag per
  topic/group), one Postgres overview, one "AI pipeline" business dashboard
  (meetings/day, avg time-to-summary, action item completion rate,
  most-discussed topics).
- Alertmanager rules: consumer lag > threshold for 5m, error rate > 5% for
  5m, Ollama/whisper.cpp p99 latency breach, DLQ depth > 0, disk/PVC > 80%.

### Logging (Loki)
- Structured JSON logs via `log/slog` (Go 1.21+), one line per event, fields:
  `ts, level, service, trace_id, span_id, org_id, msg, ...`.
- Shipped via Promtail (or the OTel Collector's log pipeline) to Loki;
  Grafana "Explore" correlates a log line's `trace_id` directly to its Tempo
  trace via a derived field link.
- No PII/secrets in logs — password hashes, tokens, and raw transcript text
  are never logged; a lint rule (custom `golangci-lint` config) flags common
  offenders.

### Tracing (OpenTelemetry + Tempo)
- OTel Go SDK auto-instruments Fiber middleware, the gRPC client/server
  interceptors, the Postgres driver (`otelpgx`), the Kafka producer/consumer
  (manual span + header propagation per the flow in `kafka-topics.md`), and
  outbound HTTP to Ollama/whisper.cpp/Jira/Slack.
- All services export via OTLP/gRPC to a cluster-local **OTel Collector**,
  which fans out: traces → Tempo, metrics → Prometheus remote-write, logs →
  Loki. Centralizing through the Collector (rather than each service
  exporting directly to three backends) is itself a design talking point:
  single point to add sampling, PII scrubbing, or batching.
- Sampling: head-based 100% in dev, tail-based (error-biased) documented for
  prod via the Collector's `tailsampling` processor.

## 2. Security Architecture

- **AuthN**: JWT access tokens (RS256, 15 min TTL) + rotating refresh tokens
  (7 day TTL, single-use — issuing a new one revokes the old and links
  `replaced_by`, so replay of a stolen refresh token is detectable).
  Passwords hashed with **argon2id**.
- **AuthZ / RBAC**: role (`owner|admin|manager|member|viewer`) embedded in
  the JWT claims, enforced at two layers: (1) API Gateway middleware
  rejects obviously unauthorized routes early; (2) each service's gRPC
  interceptor re-checks role for the specific RPC (defense in depth — the
  gateway is not trusted as the sole enforcement point).
- **Multi-tenant isolation**: see §3 below.
- **Transport security**: TLS at the ingress (cert-manager + a local CA via
  `mkcert`, trusted by the dev machine); intra-cluster traffic is plaintext
  by default locally (NetworkPolicy-restricted) with **mTLS via a service
  mesh (Linkerd)** documented as a Phase 6 hardening step and interview
  topic — Linkerd chosen over Istio for lower local resource footprint.
- **Secrets management**: never in git or Helm `values.yaml` in plaintext;
  SOPS+age-encrypted manifests decrypted at apply time (see
  `kubernetes-cicd.md` §4); Kafka/Postgres/Redis credentials injected via
  K8s `Secret` + `envFrom`, rotated manually in this project's scope (Vault
  dynamic secrets called out as a future upgrade).
- **Input validation & abuse prevention**: request validation via
  `go-playground/validator` struct tags at the gateway boundary; file-upload
  type/size validation before issuing a MinIO presigned URL; Redis
  token-bucket rate limiting per `(org_id, user_id)` and a stricter
  per-IP limit on `/auth/*`.
- **Dependency & image security**: `govulncheck` + `trivy` in CI (see
  `kubernetes-cicd.md` §3); distroless/minimal base images for every
  service; images signed with `cosign` (keyless, via GitHub OIDC) as a
  stretch goal.
- **Audit trail**: `user.role-changed.v1`, `user.login-failed.v1`,
  `action-item.status-changed.v1` etc. give a natural, already-emitted audit
  log — Analytics Service (or a dedicated small audit consumer) persists
  security-relevant events with longer retention than the operational topics.

## 3. Multi-Tenancy Design

Isolation model: **shared cluster, shared Postgres instance, tenant-scoped
rows** (pool model — the standard SaaS trade-off vs. silo-per-tenant, chosen
because it's the pattern most real systems use and the one interviewers
probe hardest).

| Layer | Isolation mechanism |
|---|---|
| Postgres | `org_id` column on every tenant table + **Row-Level Security** policy (`current_setting('app.current_org')`) — enforced even if application code has a bug |
| Application | Every gRPC request carries `RequestContext.org_id` (from JWT, not client-supplied body field) — repository layer issues `SET LOCAL app.current_org` per transaction before any query |
| MinIO | Object keys namespaced `org_id/meeting_id/...`; bucket policy denies cross-prefix listing; access only via short-lived presigned URLs scoped to one object |
| pgvector search | Same RLS policy applies to `search.chunk_embeddings` — a similarity query physically cannot return another tenant's vectors |
| Kafka | Messages carry `org_id` in the payload; consumers must apply tenant context before any DB write — no shared "global" topic mixes data at rest, only in transit |
| Rate limits & quotas | Redis counters and `org.quotas` keyed by `org_id`; a noisy tenant is throttled independently |
| Kubernetes (future silo tier) | Documented upgrade path: a "dedicated" plan gets its own namespace + NetworkPolicy + resource quota, without changing application code, since isolation is already tenant-aware at every layer |

## 4. Scaling Strategy

- **Stateless request-path services** (Gateway, Auth, User, Org, Meeting,
  Search-read-path): HPA on CPU + custom RPS metric via Prometheus Adapter.
- **Kafka-consumer AI workers** (Transcription, AI Summary, Action Item,
  Search-embed-path, Analytics): **KEDA** `ScaledObject` on consumer-group
  lag — scales to the actual backlog, including scale-to-zero when idle
  (real cost savings, and a clean interview story on queue-based
  autoscaling vs. CPU-based).
- **Database**: read replica for analytics-heavy/read-heavy queries;
  connection pooling via `pgbouncer` sidecar/service to avoid exhausting
  Postgres `max_connections` as services scale out.
- **Kafka**: partition count set with future scale in mind (see
  `kafka-topics.md`) — repartitioning later is disruptive, over-provisioning
  partitions up front is cheap.
- **Ollama/whisper.cpp**: the actual bottleneck on a laptop. Documented prod
  path: dedicated GPU node pool, model server replicas behind a queue-aware
  load balancer, or batching multiple chunk-embedding requests per call.

## 5. Disaster Recovery Strategy

| Component | Backup | RPO | RTO | Restore procedure |
|---|---|---|---|---|
| Postgres | WAL archiving to MinIO (via `pgBackRest`/`wal-g`) + nightly logical `pg_dump` per schema | ~5 min (WAL) | < 1 hr | `pgBackRest restore` to a point-in-time, replay WAL, verify RLS policies intact |
| MinIO (recordings) | Bucket versioning on; optional replication to a second local MinIO instance simulating cross-region | near-zero for new writes | minutes | Re-point service to replica bucket, or restore versioned objects |
| Kafka | Topic retention is not a backup; source-of-truth is Postgres. On total Kafka loss, replay is unnecessary — operational state already landed in Postgres via consumers; only in-flight (unconsumed) events are lost, bounded by retention window | — | — | Redeploy Strimzi, recreate `KafkaTopic` CRDs (declarative, in git) |
| Full cluster | Everything is defined in git (Helm + ArgoCD) — a fresh Kind cluster + `argocd app sync` reconstructs the entire application layer; only Postgres/MinIO data needs restoring from backup | — | < 2 hr end-to-end | Documented runbook in `docs/runbooks/dr-restore.md` (to be written in Phase 6), rehearsed via a chaos drill |
| Chaos testing | `kubectl delete pod` on a random Kafka broker/Postgres pod, verify self-heal via ArgoCD + StatefulSet; simulate node loss by draining a Kind worker | — | — | Interview-relevant: demonstrates understanding of failure-mode testing, not just steady-state design |

## 6. Cost Optimization Strategy (local, $0)

- 100% open-source components; no paid API keys anywhere (no OpenAI, no
  managed Confluent/Aiven/Atlas) — the whole stack runs on Docker/Kind.
- **Quantized models** keep the AI stack laptop-sized: Llama 3 8B / Qwen2.5
  7B / Mistral 7B at **Q4_K_M** (~4-5GB each) via Ollama; whisper.cpp
  `base`/`small` GGML models (~150MB-500MB) rather than `large`. Documented
  upgrade path to `large-v3`/unquantized models on real GPU hardware.
- **KEDA scale-to-zero** for AI worker consumers when idle — on a laptop
  this directly frees RAM for whatever else is running, not just a cloud
  cost line item.
- **GHCR free tier** for image storage (no Docker Hub rate limits, no paid
  registry); **GitHub Actions** free minutes for CI (monorepo path-filtering
  keeps matrix builds small).
- Resource requests/limits tuned per the budget table in
  `kubernetes-cicd.md` §1 so the whole stack fits comfortably in 16GB.
- For a real deployment, the same design translates to cost control via:
  KEDA scale-to-zero on cloud compute (pay only for active processing),
  spot/preemptible nodes for the stateless tier, and S3 Intelligent-Tiering
  equivalent lifecycle policies on MinIO/object storage for old recordings.
