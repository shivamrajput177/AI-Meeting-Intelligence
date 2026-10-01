# Runbook: Postgres Disaster Recovery Restore

**Status: written, not yet executed.** This sandbox has no running
Kubernetes cluster to drill against (no Docker daemon, no reachable
container registries — see `README.md`'s Phase 6 status note for the
full explanation, same constraint every earlier phase's "not
live-verified" sections name). This runbook is meant to be run for real
against a Kind cluster with `deploy/infra/backup`'s CronJob (see its own
Chart.yaml) actually having produced at least one dump — the steps below
are concrete and copy-pasteable, not a design sketch, but nobody has
executed them end to end and the RTO/RPO rows below are **targets**, not
**measurements**. Rerun this drill for real and replace those rows with
what actually happened the first time a disaster (or a scheduled
practice drill) requires it.

## Scope

Restoring `meeting-intel-postgresql`'s `meetingintel` database from the
hourly `pg_dump` the `postgres-backup` CronJob
(`deploy/infra/backup/templates/cronjob.yaml`) uploads to the MinIO
tenant, after the primary Postgres volume is lost or corrupted.

**RPO**: up to 1 hour (the CronJob's own schedule — see
`deploy/infra/backup/Chart.yaml`'s own note on why this isn't the
architecture doc's ~5-minute continuous-WAL-archiving target: that needs
pgBackRest built into the Postgres image itself, not done in this pass).

**RTO target**: under 1 hour for a database of this project's own small
development scale (the "< 1 hr" row in
`docs/architecture/observability-security.md` §5) — **not yet measured**;
see "Recording the real numbers" below for what to fill in after an
actual run.

## Prerequisites

- `kubectl` access to the cluster, context pointed at it.
- `mc` (MinIO client) available locally, or run the equivalent commands
  from a temporary pod in-cluster (step 2 shows both).
- At least one successful `postgres-backup` CronJob run already landed
  in the `pg-backups` bucket (`kubectl get cronjob -n meeting-intel-data
  postgres-backup`, then check its most recent Job succeeded).

## Steps

### 1. Confirm the blast radius

```bash
kubectl get pods -n meeting-intel-data -l app.kubernetes.io/name=postgresql
kubectl get pvc -n meeting-intel-data
```

If the primary's PVC is gone or its data is corrupted but the PVC object
itself still exists and just needs new data, skip to step 3 after
emptying it; if the PVC is gone entirely, recreate it by letting ArgoCD
re-sync the `postgresql` Application (ArgoCD self-heal will recreate the
StatefulSet + a fresh, empty PVC — see `deploy/argocd/infra-applications.yaml`).

### 2. Find and fetch the most recent dump

```bash
# From a machine with network access to the MinIO tenant (or port-forward
# it first: kubectl port-forward -n meeting-intel-data svc/meeting-intel-minio-tenant-hl 9000:9000):
mc alias set recoverminio http://meeting-intel-minio-tenant-hl.meeting-intel-data.svc.cluster.local:9000 \
  minioadmin minioadmin123   # dev-safe default — see deploy/infra/backup/values.yaml's own note
mc ls recoverminio/pg-backups | sort | tail -1   # the newest dump's filename
mc cp recoverminio/pg-backups/<newest-filename> ./meetingintel-restore.dump
```

### 3. Wait for a healthy, empty Postgres primary

```bash
kubectl wait --for=condition=Ready pod -n meeting-intel-data -l app.kubernetes.io/name=postgresql --timeout=5m
```

### 4. Restore

```bash
kubectl cp ./meetingintel-restore.dump meeting-intel-data/<postgresql-pod-name>:/tmp/restore.dump

kubectl exec -n meeting-intel-data <postgresql-pod-name> -- \
  pg_restore -U postgres -d meetingintel --clean --if-exists -j 4 /tmp/restore.dump
```

`--clean --if-exists` drops conflicting objects before recreating them —
safe here because step 3 already confirmed the target database has no
data worth preserving.

### 5. Verify

```bash
kubectl exec -n meeting-intel-data <postgresql-pod-name> -- \
  psql -U postgres -d meetingintel -c "select count(*) from \"user\".organizations;"
```

Confirm row counts look sane for the point in time the dump was taken
(compare against whatever monitoring/dashboards were showing before the
incident, e.g. the Grafana AI-pipeline dashboard's own "Meetings processed
/ day" panel — see `deploy/infra/observability/dashboards/ai-pipeline.json`).

Also confirm RLS policies survived the restore intact — `pg_restore`
preserves them as part of the schema, but this is exactly the kind of
assumption a real drill should verify rather than trust
(`docs/architecture/database-schema.md`'s own RLS section is the reason
to be careful here: RLS has already surprised this project once).

```bash
kubectl exec -n meeting-intel-data <postgresql-pod-name> -- \
  psql -U postgres -d meetingintel -c "\d+ \"user\".organizations" | grep -i "row security"
```

### 6. Point services back at it

If step 1 required ArgoCD to recreate the StatefulSet, every application
service's own `secrets.database_url` already points at the same Service
DNS name (`meeting-intel-postgresql.meeting-intel-data.svc.cluster.local`)
— no application-side reconfiguration needed, only a rolling restart if
any pod cached a dead connection:

```bash
kubectl rollout restart deployment -n meeting-intel --all
```

## Recording the real numbers

After an actual drill (or a real incident), replace this section with:

- **Date run**:
- **Time from "primary confirmed lost" to "verified restored"** (the
  real RTO):
- **Age of the dump restored from, at time of restore** (the real RPO
  this specific drill experienced):
- **What, if anything, in this runbook was wrong or missing**:
