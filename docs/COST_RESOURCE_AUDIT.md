# Cost / Resource Audit

Phase 6 task: "confirm the full stack budget from `kubernetes-cicd.md` §1
actually fits your laptop." **This is a computed audit, not a measured
one** — summing every chart's own declared `resources.requests/limits`
(Phase 5's 11 service charts + the stateful infra + Phase 6's
observability stack) × each component's minimum replica count, against a
16GB laptop budget. It was not run against a live cluster (no cluster
exists in this project's own development sandbox — see `README.md`'s
Phase 5/6 status notes for why), so there's no real memory-pressure
eviction or OOM-kill data here, only arithmetic against the numbers this
repo itself already commits to in its charts. The script that produced
the table below is throwaway and wasn't committed; the numbers are
reproducible by reading the same `values.yaml` files it read.

## Minimum-scale baseline (every HPA/ScaledObject at its own `min`)

| Component | Pods | Req CPU | Req Mem | Lim CPU | Lim Mem |
|---|---:|---:|---:|---:|---:|
| 11 application services (combined) | 18 | 1.80c | 2.25Gi | 9.00c | 4.50Gi |
| Postgres (primary + 1 read replica) | 2 | 0.50c | 1.00Gi | 2.00c | 2.00Gi |
| Redis | 1 | 0.10c | 128Mi | 0.25c | 256Mi |
| MinIO tenant (1 pool) | 1 | 0.10c | 256Mi | 0.50c | 512Mi |
| Strimzi Kafka (1 combined node) | 1 | 0.50c | 1.00Gi | 1.00c | 2.00Gi |
| Ollama | 1 | 2.00c | 6.00Gi | 4.00c | 8.00Gi |
| whisper.cpp | 1 | 1.00c | 1.00Gi | 2.00c | 2.00Gi |
| Observability stack (Prometheus + Grafana + Alertmanager + Loki + Promtail + Tempo + OTel Collector + 2 exporters) | 9 | 0.85c | 1.50Gi | 3.90c | 3.00Gi |
| **Total** | **34** | **6.85c** | **13.12Gi** | **22.55c** | **22.25Gi** |

(11-service row and observability row are sums of each chart's own
per-pod numbers × that component's minimum replica count — see each
chart's own `values.yaml`/`Chart.yaml` for the line items; not repeated
pod-by-pod here to keep this table short.)

## Finding: it does **not** comfortably fit 16GB

**13.12Gi of memory *requests* alone — before counting Kind's own
control-plane, `kube-system` pods (CoreDNS, kube-proxy, kindnet), or any
OS/desktop overhead — already consumes ~82% of a 16GB machine.** Add
realistic system overhead (1-2GB is typical for Kind + host OS) and the
honest answer is this stack's *requests* are at or past the edge of what
fits, not comfortably under it. The *limits* total (22.25Gi) would
clearly exceed 16GB if every component were simultaneously pushed to its
ceiling — in practice not all of them will be at once, but it means there
is no slack if more than a couple of components spike together.

**Ollama (6Gi request / 8Gi limit) is the single dominant line item** —
exactly what `docs/architecture/observability-security.md` §4 already
names ("Ollama/whisper.cpp: the actual bottleneck on a laptop"), now with
an actual number behind that claim rather than just the assertion.

## What this means in practice

Running the **complete** stack (all 11 app services + all stateful infra
+ the full Phase 6 observability stack) simultaneously on a 16GB machine
is workable but tight, with little headroom for anything else running on
that machine at the same time (a browser, an IDE, etc.) — consistent with
`deployments/docker-compose.yaml`'s own long-standing advice to omit
`whisper`/`ollama`/`transcription-service`/`ai-summary-service` from a
`docker compose up` invocation when you don't need the AI pipeline for
what you're testing. The same trade-off applies here:

- **Full stack, AI pipeline included**: expect to be close to the edge
  on a 16GB machine; 32GB is comfortable.
- **Core app + infra, observability stack disabled**
  (`--set kube-prometheus-stack.enabled=false,loki-stack.enabled=false,tempo.enabled=false,opentelemetry-collector.enabled=false`
  on the `deploy/infra/observability` chart, or simply not syncing that
  ArgoCD Application): drops ~1.5Gi/3Gi requests/limits — a real but
  modest saving, since the observability stack isn't the dominant cost.
- **Core app + infra, no Ollama/whisper.cpp** (skip transcription/
  summarization, same as the compose-era advice): saves ~7Gi/10Gi
  requests/limits — by far the biggest lever, since the AI model servers
  are the dominant cost either way.

## KEDA scale-to-zero note

`deploy/helm/meeting-intel/charts/*/values.yaml`'s `scaledObject` blocks
default `minReplicaCount: 1` (transcription/ai-summary) or similar small
numbers for the other KEDA-scaled services — not `0` — so this audit's
baseline already assumes at least one replica of each is always running,
not scaled to zero. Setting `minReplicaCount: 0` on the lighter consumers
(action-item-service, notification-service, analytics-service — all
already `minReplicaCount: 1` today) would let KEDA fully scale them down
when their Kafka consumer group has no lag, trimming the baseline further
on an idle cluster. Not changed in this pass — the architecture doc's own
Phase 6 task list names "tune KEDA scale-to-zero idle behavior" as its
own line item, and tuning it meaningfully wants to be informed by
watching a real idle cluster's actual memory pressure, not guessed at
from this audit alone.
