# Chaos Test Findings

**Status: not yet run.** `scripts/chaos-test.sh` is a real, runnable tool
(see its own header comment) — this file is the template for recording
what actually happens each time it's run for real against a live
cluster, not a report of results that don't exist yet. Per
`docs/architecture/observability-security.md` §5: "kill a random pod per
stateful component during a load test, confirm self-heal + no data loss;
document findings."

Fill in one section per run. Delete the "(template — not yet run)"
marker once a target actually has a real result.

## Postgres pod kill (template — not yet run)

- **Date run**:
- **Command**: `./scripts/chaos-test.sh postgres`
- **Time to self-heal** (pod deleted → replacement Ready):
- **Data loss observed?** (compare row counts / recent records
  before vs. after, per the Grafana Postgres Overview dashboard):
- **In-flight requests during the kill**: did any write fail, and if so,
  did the calling service retry/surface it sanely?
- **Anything surprising**:

## Kafka broker pod kill (template — not yet run)

- **Date run**:
- **Command**: `./scripts/chaos-test.sh kafka`
- **Time to self-heal**:
- **Consumer lag behavior**: did `kafka_consumergroup_lag` spike and
  recover, or climb and stay high?
- **Message loss observed?** (Strimzi's own replication should prevent
  this with replicas >= 2 — note Phase 5's own single-combined-node
  dev-local Kafka setup has `replicas: 1`, so this specific drill is
  expected to show real unavailability during the kill, not the
  zero-downtime story a real multi-broker cluster would — see
  `deploy/infra/strimzi-kafka/values.yaml`'s own comment on why):
- **Anything surprising**:

## MinIO pod kill (template — not yet run)

- **Date run**:
- **Command**: `./scripts/chaos-test.sh minio`
- **Time to self-heal**:
- **Object availability during/after**: could a recording still be
  fetched via its presigned URL?
- **Anything surprising**:

## Simulated node loss (Kind worker drain) (template — not yet run)

- **Date run**:
- **Command**: `./scripts/chaos-test.sh node`
- **Which pods rescheduled, and how long did it take**:
- **Any pod that failed to reschedule (and why — PVC node affinity,
  resource pressure on remaining nodes, etc.)**:
- **Anything surprising**:

## Overall assessment (fill in after at least one full pass)

- Does the system's actual self-heal behavior match what
  `docs/architecture/observability-security.md` §5 claims?
- What, if anything, in this project's PodDisruptionBudgets, resource
  requests/limits, or readiness probes (all from Phase 5) needs
  adjusting based on what was actually observed?
