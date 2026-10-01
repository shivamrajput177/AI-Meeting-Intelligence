# Helm charts — `deploy/helm/meeting-intel/`

Umbrella chart for the whole platform. See
`docs/architecture/kubernetes-cicd.md` §2 for the design this follows,
and the repo root `README.md`'s Phase 5 section for what's actually been
exercised versus still only authored-and-reviewed (no Kind cluster has
been brought up from inside this project's own development sandbox — see
that section for exactly why).

## One-time setup

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add minio-operator https://operator.min.io
helm repo update

cd deploy/helm/meeting-intel
helm dependency build   # resolves postgresql/redis/minioOperator/minioTenant
                         # from the repos above, and packages the local
                         # meeting-intel-common library chart + the 11
                         # service subcharts under ./charts/*.tgz
```

`helm dependency build` writes `.tgz` files into `./charts/` for the
external dependencies — those are build output, not committed (see
`.gitignore`); the 11 service subcharts and `meeting-intel-common`
themselves live as real, committed source directories under `./charts/`
(Helm's file-based local dependencies work either way — build just
packages them too for consistency).

## Install

```bash
kind create cluster --config ../../kind/kind-config.yaml --name meeting-intel

kubectl create namespace meeting-intel
kubectl create namespace meeting-intel-data
kubectl create namespace meeting-intel-ai

# Each service's own Secret (DB URL, JWT signing key, internal service
# token, integration tokens) — never templated from Helm values (see
# ../argocd/SECRETS.md for the SOPS+age flow this stands in for locally).
# For a bare Kind cluster without SOPS set up, create them directly, e.g.:
#   kubectl -n meeting-intel create secret generic auth-service-secrets \
#     --from-file=secrets.json=/path/to/auth-service.secrets.json
# matching the shape each chart's templates/secret.yaml expects — see that
# service's own values.yaml comment for its exact secret key name.

helm install meeting-intel . \
  -f values.yaml -f values-dev.yaml \
  -n meeting-intel --create-namespace
```

## Lint / template without a cluster

```bash
helm lint .
helm template meeting-intel . -f values.yaml -f values-dev.yaml | less
```

Both work with no cluster and no network access beyond the one-time
`dependency build` above — this is how this chart's own correctness was
checked in this project's development sandbox (see the repo root
`README.md`'s Phase 5 section).
