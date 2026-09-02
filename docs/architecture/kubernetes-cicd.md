# Kubernetes Deployment, Helm, CI/CD, GitOps

## 1. Kubernetes Deployment Strategy

**Local cluster**: Kind, 1 control-plane + 2 worker nodes, port-mapped
ingress (`kind-config.yaml` under `deploy/kind/`). Namespace layout:

| Namespace | Contents |
|---|---|
| `meeting-intel` | all 11 application services |
| `meeting-intel-data` | Postgres, Redis, MinIO, Kafka (Strimzi) |
| `meeting-intel-ai` | Ollama, whisper.cpp server |
| `observability` | Prometheus, Grafana, Loki, Tempo, OTel Collector |
| `argocd` | ArgoCD itself |

**Per-service Kubernetes objects** (templated identically by the Helm
subchart, see §2): `Deployment`, `Service` (ClusterIP), `ServiceAccount`,
`ConfigMap` (non-secret config), reference to a `Secret` (DB creds, JWT
signing key, integration tokens — created out-of-band, never in Helm
values), `HorizontalPodAutoscaler` **or** KEDA `ScaledObject`,
`PodDisruptionBudget`, `ServiceMonitor` (Prometheus Operator CRD),
`NetworkPolicy` (default-deny + explicit allow from gateway/Kafka).

**Stateful infra**: Postgres via `bitnami/postgresql` chart (custom image
with `pgvector` compiled in, or `pgvector/pgvector` image), single primary +
1 read replica (demonstrates HA even locally); Kafka via **Strimzi Kafka
Operator** in KRaft mode (`KafkaNodePool` for broker+controller combined
role locally); Redis via `bitnami/redis` (standalone locally, sentinel
documented for prod); MinIO via official MinIO Operator, single-tenant,
4-drive erasure-coded locally to demonstrate the pattern.

**Probes**: every service exposes `/healthz` (liveness — process up) and
`/readyz` (readiness — DB/Kafka/Redis reachable) distinctly, per the
standard Kubernetes pattern; AI worker services additionally gate readiness
on a successful Ollama/whisper.cpp ping so they don't receive work before
the model is warm.

**Resource requests/limits**: every Deployment sets both (required for HPA
and for the cluster to actually schedule predictably on a laptop's fixed
RAM). Example budget for a 16GB MacBook:

| Component | Requests | Limits |
|---|---|---|
| Go services (each) | 100m CPU / 128Mi | 500m CPU / 256Mi |
| Postgres | 250m / 512Mi | 1 / 1Gi |
| Kafka broker | 500m / 1Gi | 1 / 2Gi |
| Redis | 100m / 128Mi | 250m / 256Mi |
| MinIO | 100m / 256Mi | 500m / 512Mi |
| Ollama (7-8B Q4) | 2 / 6Gi | 4 / 8Gi |
| whisper.cpp (base/small) | 1 / 1Gi | 2 / 2Gi |

**Rollout strategy**: `RollingUpdate` (`maxUnavailable: 0, maxSurge: 1`) for
stateless services; ArgoCD sync waves ensure infra (Postgres/Kafka/Redis/
MinIO) is `Healthy` before application Deployments sync (see §4).

## 2. Helm Chart Structure

```
deploy/helm/
├── meeting-intel/                     # umbrella chart
│   ├── Chart.yaml                     # dependencies: all subcharts + bitnami/postgresql, bitnami/redis
│   ├── values.yaml                    # shared defaults
│   ├── values-dev.yaml                # Kind-local overrides (low resources, NodePort)
│   ├── values-prod.yaml               # placeholder for real-cluster overrides
│   └── charts/
│       ├── api-gateway/
│       │   ├── Chart.yaml
│       │   ├── values.yaml
│       │   └── templates/
│       │       ├── deployment.yaml
│       │       ├── service.yaml
│       │       ├── hpa.yaml
│       │       ├── configmap.yaml
│       │       ├── servicemonitor.yaml
│       │       ├── networkpolicy.yaml
│       │       └── _helpers.tpl
│       ├── auth-service/          (same template set)
│       ├── user-service/
│       ├── organization-service/
│       ├── meeting-service/
│       ├── transcription-service/     # + scaledobject.yaml (KEDA) instead of hpa.yaml
│       ├── ai-summary-service/        # + scaledobject.yaml
│       ├── action-item-service/       # + scaledobject.yaml
│       ├── search-service/            # deployment-api + deployment-worker + hpa + scaledobject
│       ├── notification-service/      # + scaledobject.yaml
│       └── analytics-service/         # + scaledobject.yaml
└── infra/
    ├── strimzi-kafka/                 # Kafka + KafkaTopic CRDs (one per topic in kafka-topics.md)
    ├── ollama/                        # Deployment + PVC for model cache
    └── whisper-cpp/                   # Deployment + PVC for model cache
```

A shared `_helpers.tpl` per chart (or a library chart `meeting-intel-common`)
standardizes labels (`app.kubernetes.io/*`), so every service gets
consistent Prometheus scrape annotations and Grafana dashboard discovery for
free.

## 3. GitHub Actions CI/CD Pipeline

`.github/workflows/ci.yaml` (matrix over services, monorepo-aware — only
builds what changed via `dorny/paths-filter`):

```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

jobs:
  detect-changes:
    runs-on: ubuntu-latest
    outputs:
      services: ${{ steps.filter.outputs.changes }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            api-gateway: 'cmd/api-gateway/**'
            auth-service: 'cmd/auth-service/**'
            # ... one entry per service, plus 'internal/platform/**' fans out to all

  lint-test-build:
    needs: detect-changes
    if: ${{ needs.detect-changes.outputs.services != '[]' }}
    strategy:
      matrix:
        service: ${{ fromJson(needs.detect-changes.outputs.services) }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go vet ./...
      - run: golangci-lint run ./cmd/${{ matrix.service }}/...
      - run: go test -race -coverprofile=coverage.out ./internal/${{ matrix.service }}/...
      - uses: aquasecurity/trivy-action@master   # image + dep vuln scan
        with: { scan-type: fs, scan-ref: '.' }
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: cmd/${{ matrix.service }}/Dockerfile
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: ghcr.io/${{ github.repository }}/${{ matrix.service }}:${{ github.sha }}
      - run: helm lint deploy/helm/meeting-intel/charts/${{ matrix.service }}

  update-gitops:
    needs: lint-test-build
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Bump image tags in Helm values
        run: |
          for svc in ${{ join(needs.detect-changes.outputs.services, ' ') }}; do
            yq -i ".image.tag = \"${{ github.sha }}\"" \
              deploy/helm/meeting-intel/charts/$svc/values.yaml
          done
      - uses: stefanzweifel/git-auto-commit-action@v5
        with: { commit_message: "chore: bump image tags [skip ci]" }
```

Images publish to **GHCR** (free for public/private repos under GitHub's
free tier limits). The commit that bumps `values.yaml` is what ArgoCD's
auto-sync detects — CI never talks to the cluster directly (GitOps
principle: git is the only deployment trigger).

Separate `security.yaml` workflow: `gosec`, `trivy image`, `govulncheck`,
scheduled weekly + on every PR touching `go.mod`.

## 4. ArgoCD GitOps Setup

**App-of-apps via `ApplicationSet`**, generated from a simple list so adding
a 12th service later is a one-line change:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: meeting-intel-services
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - name: api-gateway
          - name: auth-service
          - name: user-service
          - name: organization-service
          - name: meeting-service
          - name: transcription-service
          - name: ai-summary-service
          - name: action-item-service
          - name: search-service
          - name: notification-service
          - name: analytics-service
  template:
    metadata:
      name: '{{name}}'
      annotations:
        argocd.argoproj.io/sync-wave: "2"
    spec:
      project: meeting-intel
      source:
        repoURL: https://github.com/shivamrajput177/AI-Meeting-Intelligence
        targetRevision: main
        path: deploy/helm/meeting-intel/charts/{{name}}
        helm:
          valueFiles: [values.yaml, ../../values-dev.yaml]
      destination:
        server: https://kubernetes.default.svc
        namespace: meeting-intel
      syncPolicy:
        automated: { prune: true, selfHeal: true }
        syncOptions: [CreateNamespace=true]
```

**Sync waves**: `-1` infra CRDs (Strimzi operator, MinIO operator) → `0`
stateful infra (Postgres, Kafka, Redis, MinIO, Ollama, whisper.cpp) → `1`
DB migrations (Argo `PreSync` Job hook running `golang-migrate` per service
schema) → `2` application services. This guarantees dependencies are
`Healthy` before dependents sync.

**Secrets in GitOps**: application manifests never contain plaintext
secrets. Use **SOPS + age** (fully free/local, no Vault dependency) to
encrypt a `secrets.enc.yaml` committed to git; a `SopsSecretGenerator` (via
`ksops` or a small init step) decrypts into a real `Secret` at apply time. A
key file lives outside git in `~/.config/sops/age/keys.txt` (or GitHub
Actions secret for CI-generated ones).

**Progressive delivery (stretch)**: Argo Rollouts with a canary step for
the API Gateway (10% → 50% → 100%, automated analysis on Prometheus error
rate) — documented as a Phase 6 nice-to-have and a strong interview topic
even if only demoed on the gateway.
