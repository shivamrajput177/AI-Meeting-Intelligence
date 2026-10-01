# Secrets in GitOps — SOPS + age

Per `docs/architecture/kubernetes-cicd.md` §4: application manifests
never contain plaintext secrets, even though every service chart's own
`values.yaml` ships **dev-safe placeholder** secrets (`dev-internal-token`,
`postgres:postgres@...`, `minioadmin`/`minioadmin123` — the same
philosophy as `deployments/configs/*.template.json`, fine for a local
Kind cluster, never for anything real). This file is how a real
deployment replaces those placeholders without ever committing the real
values to git, using [SOPS](https://github.com/getsops/sops) +
[age](https://github.com/FiloSottile/age) — both free, both run entirely
locally, no Vault/cloud KMS dependency required.

**Not set up or exercised in this repo** — there is no committed
`secrets.enc.yaml`, no age key generated, and no `ksops`/init-step
decryption actually wired into any Application above. What follows is the
intended flow per kubernetes-cicd.md §4, documented so it's a
copy-and-run setup rather than a design note nobody could act on; building
it out is real follow-up work, not claimed as done.

## One-time setup

```bash
age-keygen -o age-key.txt    # keep this OUTSIDE git — e.g. ~/.config/sops/age/keys.txt
export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
```

For GitHub Actions, store the key file's contents as a repo secret
(`AGE_KEY`) instead — never as a file committed anywhere.

## Encrypting a service's real secrets

Each service chart's `templates/secret.yaml` renders `.Values.secrets`
into a Kubernetes `Secret` (see e.g.
`deploy/helm/meeting-intel/charts/auth-service/values.yaml`'s own
`secrets:` block for the dev-safe shape to replace). To supply real
values without touching the committed `values.yaml`:

```bash
cat > auth-service.secrets.yaml <<'EOF'
secrets:
  database_url: postgres://real-user:real-password@prod-postgres:5432/meetingintel
  internal_service_token: <a real random token>
  jwt_signing_key: <a real random signing key>
EOF

sops --encrypt --age "$(cat age-key.txt | grep 'public key:' | cut -d' ' -f4)" \
  auth-service.secrets.yaml > auth-service.secrets.enc.yaml
rm auth-service.secrets.yaml   # never leave the plaintext on disk
git add auth-service.secrets.enc.yaml   # safe to commit — it's encrypted
```

## Decrypting at apply time

A `SopsSecretGenerator` (via
[`ksops`](https://github.com/viaduct-ai/kustomize-sops), a kustomize
plugin ArgoCD's repo-server can be configured to run) decrypts
`*.secrets.enc.yaml` into real Helm values at sync time, so the decrypted
plaintext never touches git and only ever exists in-cluster. Wiring this
in means:

1. Installing the `ksops` plugin into the ArgoCD repo-server image (a
   custom repo-server image or an init-container that drops the binary
   in — see ksops' own install docs).
2. Pointing each Application's `source.helm.valueFiles` (or a kustomize
   overlay, if this repo moves to kustomize for secrets specifically)
   at the decrypted output.
3. Providing `SOPS_AGE_KEY_FILE` to the repo-server pod via its own
   Kubernetes `Secret` (created once, out-of-band, the same way any
   bootstrap secret is — see `../helm/README.md`'s own note on creating
   a service's `Secret` directly for a bare Kind cluster without this
   flow set up at all).

None of this is Kind-cluster-local-demo-required — the dev-safe defaults
baked into every chart's `values.yaml` are enough to bring the whole
platform up locally with zero secrets setup. This flow only matters once
real credentials (a real Postgres, a real Slack webhook, a real JWT
signing key for anything beyond a laptop demo) enter the picture.
