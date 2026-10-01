# Cloudflare Tunnel setup (Phase 7)

This is a one-time, manual setup done from an operator's own machine (or the
demo VM itself) against a real Cloudflare account — **not executed in this
project's own development sandbox**, which has no Cloudflare account, no DNS
zone, and (per this repo's other sandbox-constraint notes — see
`README.md`'s Phase 5/6 status and `docs/COST_RESOURCE_AUDIT.md`'s own
header) no outbound access to `api.cloudflare.com` to test against even if
credentials existed. Everything below is the real, standard `cloudflared`
workflow (Cloudflare's own documented steps, not invented), written out
precisely so running it is copy-paste, not guesswork — but it has not itself
been run end-to-end here.

See `docs/architecture/deployment-demo-strategy.md` §2 for why Cloudflare
Tunnel over cert-manager+mkcert: free HTTPS and a subdomain with no
port-forwarding and no domain purchase, which a Kind-cluster ingress
controller can't offer a single bare VM.

## Prerequisites

- A Cloudflare account (free tier is enough) with a domain already added as
  a Cloudflare DNS zone. If you don't own a domain, Cloudflare's own
  Free plan still requires *a* zone — registering a cheap domain (or using
  one you already have) is the one real cost this otherwise-free deployment
  path has.
- `cloudflared` installed somewhere you can run an interactive browser login
  from (your laptop is fine — the tunnel *token* this produces, not
  `cloudflared` itself, is what the VM needs).

## One-time tunnel setup

```bash
# 1. Authenticate — opens a browser, asks you to pick the Cloudflare zone
#    this tunnel is allowed to route traffic for.
cloudflared tunnel login

# 2. Create the named tunnel. This mints a tunnel ID + credentials file
#    under ~/.cloudflared/ — you don't need the credentials file itself for
#    the token-based run below, only the tunnel's existence.
cloudflared tunnel create meeting-intel-demo

# 3. Route a subdomain to it — pick whatever subdomain you want on the
#    resume link, e.g. demo.yourdomain.com.
cloudflared tunnel route dns meeting-intel-demo demo.yourdomain.com

# 4. Mint a connector token — this is the single secret the demo VM needs.
#    It authorizes a cloudflared process to connect as this tunnel without
#    needing the credentials.json file from step 2.
cloudflared tunnel token meeting-intel-demo
```

Put that token's output into `deployments/.env.demo`'s `CLOUDFLARE_TUNNEL_TOKEN`
(copy `deployments/.env.demo.template` first — see its own header).

## Ingress routing

Unlike a self-managed `config.yml` + `credentials.json` pair, the
token-based `cloudflared tunnel run` form (what
`deployments/docker-compose.demo.yaml`'s `cloudflared` service actually
runs) takes its ingress rules from the tunnel's **Public Hostname**
configuration in the Cloudflare Zero Trust dashboard instead of a local
file — simpler for a single-service-plus-API split than hand-maintaining
YAML on the VM. Configure two public hostnames against the
`meeting-intel-demo` tunnel:

| Public hostname | Path | Service (origin) |
|---|---|---|
| `demo.yourdomain.com` | `/api/v1/*` | `http://api-gateway:8080` |
| `demo.yourdomain.com` | `*` (catch-all, lower priority than the row above) | `http://web:80` |

Both origins are reached by their **docker-compose service name** — the
`cloudflared` container joins the same compose network as `web` and
`api-gateway` (see `docker-compose.demo.yaml`), so this is exactly the same
internal DNS every other service in this stack already uses to reach each
other, nothing cloud-specific about it.

## Running it

Once the tunnel exists and `deployments/.env.demo` has a real token:

```bash
docker compose -f deployments/docker-compose.yaml \
                -f deployments/docker-compose.demo.yaml \
                --env-file deployments/.env.demo up -d --build
```

`docker compose logs -f cloudflared` should show `Registered tunnel
connection` once it's actually reached Cloudflare's edge — that line is the
real signal this step worked, not just that the container started.

## Rotating or revoking

`cloudflared tunnel token meeting-intel-demo` can be re-run to mint a fresh
token at any time (the old one keeps working until explicitly revoked);
`cloudflared tunnel delete meeting-intel-demo` tears the whole tunnel down.
Neither needs touching this repo — both are account-side Cloudflare
operations, consistent with this file's secret never being committed (see
`.gitignore`'s `deployments/.env.demo` entry).
