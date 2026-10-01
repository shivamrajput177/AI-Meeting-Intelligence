# Runbook: Public Demo VM Provisioning (Phase 7)

**Not executed in this project's own development sandbox** — this sandbox
has no cloud provider credentials, no Oracle Cloud account, and (confirmed
the same way every other cloud/registry endpoint in this project was
checked — see `docs/COST_RESOURCE_AUDIT.md`'s header and
`deployments/docker-compose.yaml`'s own whisper.cpp note on blocked
registry egress) no outbound access to Oracle's own API endpoints to test
against. Every step below is Oracle Cloud's and Docker's own documented
procedure, written out precisely enough to run as a real provisioning
session — but it has not itself been run end-to-end here, and there is no
measured "it actually came up" result to report, the same honest gap
`docs/runbooks/dr-restore.md` and `docs/runbooks/chaos-test-findings.md`
already disclose for their own execution-dependent tasks.

See `docs/architecture/deployment-demo-strategy.md` §2 for why Oracle Cloud
Free Tier specifically: it's the only free-forever tier with enough RAM
(24GB) to run Postgres+pgvector, Redis, Kafka, MinIO, Ollama, and
whisper.cpp side by side without falling over — more headroom than Phase
6's `docs/COST_RESOURCE_AUDIT.md` found the full Kind-cluster stack needs,
once the K8s control-plane/observability overhead that audit measured is
gone and the LLM is downsized (see `docker-compose.demo.yaml`'s own header).

## 1. Provision the VM

1. Sign up for Oracle Cloud Free Tier (requires identity verification and a
   card for fraud-prevention purposes only — the Always Free Ampere A1
   shape is never billed within its included 4 OCPU / 24GB allowance).
   **If this signup friction is a blocker**, `deployment-demo-strategy.md`
   §2 names Render/Fly.io free web-service tiers + a managed free Postgres
   as the fallback — accept their cold-start-after-idling trade-off and say
   so in the demo UI if you take that path instead.
2. Create a Compute instance:
   - Shape: **VM.Standard.A1.Flex**, 4 OCPUs, 24GB memory (the full Always
     Free Ampere allowance — a smaller slice works too if you don't need
     every service running simultaneously, per
     `docs/COST_RESOURCE_AUDIT.md`'s own "what this means in practice"
     section).
   - Image: Ubuntu 22.04 (or any image with a maintained Docker package).
   - Boot volume: the default (50GB) is comfortable for 5 Docker images'
     worth of layers plus Ollama's model store.
3. Open the VM's security list / network security group for **outbound**
   traffic only — `cloudflared` initiates an outbound connection to
   Cloudflare's edge, so unlike a traditional reverse proxy, **no inbound
   port needs opening at all**, not even 443. This is a real security
   property worth stating in an interview: the VM has no listening public
   port for an attacker to scan for.
4. Note the VM's public IP for SSH access only (`ssh ubuntu@<ip>`) — nothing
   else needs it reachable.

## 2. Install Docker + Compose

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
# log out/in (or `newgrp docker`) for the group change to take effect
docker compose version   # confirms the Compose plugin (not the old
                          # standalone docker-compose binary) is present
```

## 3. Clone and configure

```bash
git clone https://github.com/shivamrajput177/AI-Meeting-Intelligence.git
cd AI-Meeting-Intelligence/deployments
cp .env.demo.template .env.demo
# edit .env.demo: CLOUDFLARE_TUNNEL_TOKEN (from
# deployments/cloudflared/README.md's one-time setup) and DEMO_API_BASE_URL
# (your real subdomain)
```

**Rotate the dev-safe placeholder credentials now** — see
`deployments/configs/demo/README.md`'s own explicit list (JWT signing key,
MinIO secret key, internal service token, Postgres password) before this
VM's ports are reachable from the public internet via the tunnel. This is a
manual edit of the checked-out files on the VM, not something committed
back to git.

## 4. Bring the stack up

```bash
docker compose -f docker-compose.yaml -f docker-compose.demo.yaml \
               --env-file .env.demo up -d --build
```

First boot pulls every image and both Ollama models (`qwen2.5:3b`,
`nomic-embed-text`, per `docker-compose.demo.yaml`'s override of
`ollama-model-init`) — expect this to take a while on first run, same as
local dev's own first `docker compose up` (see the base
`docker-compose.yaml`'s own notes on `whisper-model-init`/`ollama-model-init`).

Verify:
```bash
docker compose -f docker-compose.yaml -f docker-compose.demo.yaml ps
docker compose -f docker-compose.yaml -f docker-compose.demo.yaml logs -f cloudflared
# look for "Registered tunnel connection" — see cloudflared/README.md
```

Then from any machine (not the VM): `curl https://demo.yourdomain.com/api/v1/...`
against a public route, and load the web UI in a browser.

## 5. Seed the demo org

Run `scripts/seed-demo-org.sh` against the now-live public URL — see that
script's own header for prerequisites (it needs real short sample audio
files, which this repo does not ship — see the script's own honest note on
why).

## 6. Ongoing operation

- `docker compose ... logs -f <service>` for troubleshooting — there's no
  Loki/Grafana here (Phase 6's observability stack is Kind-cluster-only,
  per `deployment-demo-strategy.md` §2's comparison table: "not applicable
  — demonstrated instead via the local cluster"). Plain `docker logs` is
  the real, honest story for this deployment target.
- `docker compose ... pull && docker compose ... up -d --build` to deploy a
  new commit — there's no CI/CD pipeline wired to this VM (Phase 5's
  GitHub Actions + ArgoCD pipeline targets the Kind cluster, not this box)
  — redeploying here is a manual SSH-in-and-pull operation, which is an
  honest, named trade-off of the single-VM demo path, not an oversight.
- `docker system df` / `docker system prune` periodically — the 50GB boot
  volume has no automatic image-layer garbage collection configured.

## 7. Known gaps, stated plainly

- No automated backup of this VM's Postgres data — Phase 6's
  `deploy/infra/backup` CronJob targets the Kind/Kubernetes stack; this
  single-VM deployment has no equivalent. A lost demo VM means re-running
  step 5, not a real data-loss incident (it's seed data, not a production
  tenant's own meetings) — a deliberate scope line, not a missed one.
- No TLS termination choice to make here — Cloudflare Tunnel's own edge
  terminates HTTPS, matching `docs/SECURITY_CHECKLIST.md`'s own note that
  the Kind cluster has no TLS at all; the public demo is, un-intuitively,
  the more secure of this project's two deployment targets on that one
  axis specifically, purely because Cloudflare supplies it for free.
