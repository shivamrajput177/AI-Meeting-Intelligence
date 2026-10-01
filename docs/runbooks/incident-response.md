# Runbook: Incident Response (General Triage)

Distinct from `dr-restore.md` (Postgres-specific, total-loss recovery):
this is the "something's wrong, where do I look" flow for day-to-day
on-call, using the Phase 6 observability stack. Like every other Phase 6
runbook, this describes real, wired tooling (Grafana dashboards,
Prometheus alerts, Loki logs, Tempo traces, all built in this same
phase) — it has not been exercised against a real incident, because no
live cluster exists in this project's own development sandbox (see
`README.md`'s Phase 5/6 status notes).

## 1. Start from the alert, not a guess

`deploy/infra/observability/templates/alerts.yaml`'s 5 rules are the
entry point for most incidents:

| Alert | What it means | Where to look next |
|---|---|---|
| `KafkaConsumerLagHigh` | A consumer group is falling behind | Grafana's Kafka Overview dashboard, §2 below |
| `HTTPErrorRateHigh` | A route is failing >5% of requests | Grafana's per-service RED dashboard, filtered to that route |
| `OllamaWhisperLatencyHigh` | The AI pipeline is slow | Check Ollama/whisper.cpp pod resource usage — likely CPU contention (`docs/COST_RESOURCE_AUDIT.md`'s own finding that these are the dominant cost) |
| `NotificationDLQDepthNonZero` | Outbox rows permanently failing on some channel | Check that channel's downstream (Slack webhook URL valid? SMTP reachable? Jira provider registered — see `notification-service`'s own `ticketProviders` registry) |
| `PVCNearFull` | A volume is running out of space | Postgres/MinIO/Ollama/whisper.cpp — see `kubernetes-cicd.md` §1's sizing; may need a `kubectl edit pvc` resize or data cleanup |

No alert fired but something still looks wrong? Start at the per-service
RED dashboard (`deploy/infra/observability/dashboards/red-per-service.json`)
and filter to the suspect route.

## 2. Correlate: metrics → logs → traces

1. **Grafana dashboard** shows *which* route/service and *when*.
2. **Grafana Explore → Loki**, filtered to that service and time window.
   Every log line carries `request_id` (`shared/middleware`'s `AccessLog`)
   — find the failing request's own line, then query by that
   `request_id` to pull every log line across every service that touched
   the same request.
3. **Click the derived-field trace link** on a log line containing
   `trace_id=...` (wired in `deploy/infra/observability/templates/loki-datasource.yaml`)
   to jump straight to that request's Tempo trace — *if* one exists. See
   the caveat below: only the HTTP legs of a request are traced today.

**Known gap, stated plainly**: `shared/tracing`'s own package doc names
this directly — Postgres queries and Kafka hops aren't instrumented
(`otelpgx`/manual Kafka span propagation, not built in this pass). A
trace shows every HTTP call a request chain made across services; it
does *not* show which specific DB query was slow, or which Kafka message
a later consumer step came from. For the latter, fall back to each
service's own logfmt `trace_id` field on the async side (Kafka consumers
already propagate the originating request's `traceparent` as a header —
see `shared/kafkax`) even without a visual trace span for it.

## 3. Common scenarios

**A deploy just went out and errors spiked.** Check
`deploy/argocd/applicationset.yaml`'s synced revision for that service —
`argocd app rollback <service>` to the prior revision is faster than
diagnosing forward under live error pressure; diagnose from the rolled-
back-but-still-running error logs afterward.

**One pod is unhealthy, others are fine.** `kubectl logs` that pod
directly — Loki aggregates by service, not by making single-pod triage
unnecessary. Check its own `/readyz` (still just a liveness-adjacent
200 today, not real dependency checks — see `README.md`'s own honest
note on that gap) and whether it's been repeatedly restarted
(`kubectl get pod -o wide`, check `RESTARTS`).

**Consumer lag climbing with no obvious cause.** Check whether KEDA
actually scaled out (`kubectl get scaledobject`, `kubectl get hpa` —
wait, ScaledObjects don't show in `get hpa` directly, check
`kubectl describe scaledobject <name>` for its current replica count
decision) — if it scaled but lag still climbs, the bottleneck is
downstream (Ollama/whisper.cpp saturated, matching the
`OllamaWhisperLatencyHigh` alert) rather than too few consumer replicas.

**Can't tell if it's this app or the cluster itself.** Check
`kube-prometheus-stack`'s own built-in cluster dashboards (node
CPU/memory, pod eviction events) before assuming an application bug —
`docs/COST_RESOURCE_AUDIT.md`'s own finding is that this stack runs
close to a 16GB laptop's limits, so resource pressure is a real,
likely-sometimes-true explanation, not a last resort.

## 4. Escalation / when to reach for the DR runbook instead

If the incident is "Postgres data is gone or corrupted," stop here and
go to `docs/runbooks/dr-restore.md` — that's a different, more
destructive recovery path with its own prerequisites and isn't something
to improvise from this triage flow.
