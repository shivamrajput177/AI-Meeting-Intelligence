# Phase 7 demo configs

These five files are the only configs the public demo overrides — see
`docs/architecture/deployment-demo-strategy.md` §2 and
`deployments/docker-compose.demo.yaml`'s own header comment for how
`docker compose`'s `${VAR:-default}` interpolation wires them in. Everything
else (auth/user/organization/transcription/analytics/notification-service,
Postgres/Redis/MinIO/Kafka/Mailhog) keeps using the exact same
`../docker/*.json` configs as local dev — there was no reason to duplicate
configs that don't change for the demo.

What's actually different here vs. `../docker/*.json`:

- `api-gateway.json`: `demo_mode: true` (wires the extra ~1/hour-per-IP
  upload rate limit — see `services/api-gateway/router.go`'s `Register` doc
  comment).
- `meeting-service.json`: `max_upload_bytes: 26214400` (25MiB — the
  clip-length guard; see `usecase.ConfirmUploadUseCase`'s doc comment for
  why this is a size cap, not a true duration cap).
- `ai-summary-service.json` / `action-item-service.json`:
  `ollama_model: "qwen2.5:3b"` instead of `"qwen2.5:7b"` — the smaller
  quantized model `deployment-demo-strategy.md` §2's comparison table
  calls for.
- `search-service.json`: `ollama_chat_model: "qwen2.5:3b"` (same reasoning;
  `ollama_embed_model` stays `nomic-embed-text`, already small).

## This file is checked into git. Your real deployment should not ship it unmodified.

Every credential-shaped value in these five files (and in the `../docker/*`
ones they're otherwise identical to) is the same long-standing **dev-safe,
publicly-known placeholder** this whole repo uses for local Kind/
docker-compose work — see `README.md`'s own repeated notes on this. That's
fine for a laptop behind no public DNS. It is **not** fine once
`docker-compose.demo.yaml` + Cloudflare Tunnel put these same containers on
the public internet. Before a real demo VM goes live, rotate, at minimum:

- `api-gateway.json`'s `jwt_signing_key` — every access/refresh token this
  deployment issues is only as strong as this HMAC secret.
- `meeting-service.json`'s `minio_secret_key` and the `internal_service_token`
  shared by every service in `../docker/*.json` too.
- The Postgres password baked into every `database_url` above AND into
  `docker-compose.demo.yaml`'s Postgres service environment — these two
  must be changed together or every service fails to connect.

None of this is wired through Kubernetes Secrets/SOPS the way Phase 5's
`deploy/argocd/SECRETS.md` describes — this is a single plain `docker
compose` VM, not the Kind/ArgoCD reference cluster, so there's no secret
backend to integrate with here. The honest, proportionate fix for a single
VM: edit these files directly on the VM after cloning (not in git), the same
way you'd edit any other server config by hand on a box you SSH into
directly.
