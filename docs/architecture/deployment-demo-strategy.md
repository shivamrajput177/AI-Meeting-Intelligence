# Deployment, Client, and Demo Strategy (Resume-Ready)

This doc separates two things that should never be conflated: the
**reference architecture** (K8s/Kind, Kafka, ArgoCD, KEDA — what proves you
can build a production-grade distributed system) and the **public demo**
(what a recruiter clicking a resume link actually experiences). Building
both to the same spec is neither necessary nor affordable on free infra —
the split below is deliberate.

## 1. Client / Capture Strategy

**Decision: the product is the web app. No extension, desktop app, or
mobile app is required to use it.**

| Interface | Status | Rationale |
|---|---|---|
| **Web app (upload + view)** | **Ships in Phase 1, is the resume link** | Zero-install, works from any device, and is the only surface that actually exercises the backend architecture being demonstrated |
| Chrome extension (live tab-capture via `chrome.tabCapture` during a browser-based Meet/Zoom call) | Optional post-MVP add-on | Real engineering value (streaming capture, a second client surface) but must never gate core usage — "install an unverified extension" is a worse ask of a recruiter than "click a link" |
| Desktop app (Electron, system-audio capture) | Not planned | Same value as the extension, more distribution friction (code signing, OS permissions); skip unless there's spare time after everything else |
| Mobile app | Not planned | Wrong form factor for recording ingestion; the responsive web app already covers "view my summary/action items on my phone" for free |

If the extension gets built later, its pitch is "live capture for power
users," not "how you use the product" — uploading a recording stays the
primary, always-supported path.

## 2. Public Demo Deployment

**Target host: Oracle Cloud Free Tier, Ampere A1 (4 OCPU / 24GB RAM) — free
forever, not a trial.** This is the only free-tier offer with enough RAM to
run Postgres+pgvector, Redis, Ollama, and whisper.cpp side by side without
falling over.

**What changes vs. the local reference stack**:

| Aspect | Local reference architecture | Public demo |
|---|---|---|
| Orchestration | Kind + Helm + ArgoCD | plain `docker compose` on one VM (no K8s control-plane overhead to pay for) |
| Kafka | Strimzi KRaft cluster | still Kafka (KRaft, single broker) — keep the event-driven pipeline real, it's cheap at this scale |
| Object storage | MinIO | MinIO (small volume) or Cloudflare R2 free tier (10GB) |
| LLM | Llama3/Qwen/Mistral 7-8B Q4 | smaller quantized model (`qwen2.5:3b` / `phi3:mini`) — fast enough on shared CPU, still a real local model, not an API |
| TLS/ingress | cert-manager + mkcert | **Cloudflare Tunnel** (free) — HTTPS + a subdomain with no port-forwarding, no domain purchase required |
| Autoscaling (KEDA/HPA) | demonstrated live | not applicable (single VM) — demonstrated instead via the local cluster + a recorded walkthrough |

**Making a stranger's first click fast and safe**:
- **Seed a demo org** with 3–4 pre-processed sample meetings (transcript,
  summary, action items, a working RAG chat already indexed). This is what
  loads instantly — nobody's first impression should be a spinner waiting
  on Whisper.
- **Allow a real live upload too**, hard-capped: short clips only (≤2 min),
  Redis-throttled per IP (~1/hour), full pipeline status visible via the
  same Kafka-driven status machine as production. Real, just rate-limited.
- A visible note near the upload box: *"Full production architecture
  (Kubernetes, Kafka, ArgoCD, KEDA autoscaling) runs on a local Kind
  cluster — see the repo docs and walkthrough video for that; this demo is
  a cost-trimmed single-VM deployment."* This line does real work: it tells
  a technical reviewer you know the difference between a demo box and a
  production topology.

**Fallback hosts** if Oracle's ID-verification signup is a blocker:
Render or Fly.io free web-service tiers for the app + a managed free-tier
Postgres — accept the cold-start delay after idling and say so in the UI.

**What goes on the resume**: the demo URL. The repo README links the demo,
`docs/ROADMAP.md`, and (once recorded) a 2–3 minute video of `argocd app
sync` and a KEDA scale-out draining a Kafka backlog on the local cluster —
the one thing the public deployment structurally can't show.

## 3. Ticket Integration: Mock Jira by Default, Real Jira as an Adapter

**Problem**: a stranger clicking a resume link has no Jira account, and
even a free Jira Cloud site requires login to view a board — there's no
true anonymous public share on most plans. Building only against real Jira
means the flagship "meeting → action item → ticket" loop is invisible to
the exact audience the demo is for.

**Decision**: implement ticket creation behind one interface, ship a
self-built **mock Jira** as the default provider (used by the public demo
org), keep a real **Atlassian Jira Cloud** provider as the documented,
optional, interview-demoable path.

```go
// internal/notificationsvc/service/ticket_provider.go
type TicketProvider interface {
    CreateTicket(ctx context.Context, item ActionItem) (TicketRef, error)
    TransitionTicket(ctx context.Context, ref TicketRef, newStatus string) error
}

type TicketRef struct {
    Provider string // "mock_jira" | "atlassian_jira"
    Key      string // e.g. "DEMO-142"
    URL      string
}
```

- `MockJiraProvider` — writes to `notification.mock_jira_issues` (see
  schema below) and is read by a small built-in Kanban view
  (`GET /demo/board`, no auth) styled to look and behave like a real Jira
  board: issue key, summary, status column (`To Do / In Progress / Done`),
  drag-or-button transitions. This is what the public demo org uses, and
  it's genuinely bidirectional for free — an action item status change and
  a board-card drag are the same event on the same app, no webhook needed.
- `AtlassianJiraProvider` — the "real" implementation against Jira Cloud's
  REST API (`POST /rest/api/3/issue`, transition calls, API-token auth),
  exactly as originally designed in `microservices.md`. Which provider an
  org uses is one config field (`org.integration_configs.ticket_provider`);
  your own personal Jira Cloud site (free tier) is the org you flip to
  `atlassian_jira` for an interview screen-share. Bidirectional sync there
  additionally wants a Jira webhook → `/webhooks/jira`, trivial to add once
  the demo VM has a public HTTPS endpoint anyway.

This is a legitimate strategy/adapter pattern, not a workaround dressed up
as one — it's the same shape you'd use in a real product to support
multiple ticket trackers (Jira today, Linear/Asana later), and it's worth
saying so explicitly in an interview.

### Schema additions (`notification.*`)

```sql
ALTER TABLE notification.jira_links
  ADD COLUMN provider TEXT NOT NULL DEFAULT 'mock_jira'
    CHECK (provider IN ('mock_jira', 'atlassian_jira'));

CREATE TABLE notification.mock_jira_issues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL,
  action_item_id UUID NOT NULL,
  issue_key TEXT NOT NULL,          -- e.g. "DEMO-142"
  project_key TEXT NOT NULL DEFAULT 'DEMO',
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'To Do'
    CHECK (status IN ('To Do', 'In Progress', 'Done')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ON notification.mock_jira_issues (org_id, issue_key);
```

### API additions

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/demo/board` | public | Read-only mock Jira board for the demo org (no login needed) |
| GET | `/orgs/{orgId}/mock-jira/board` | member+ | Same board, for a real org using the mock provider |
| PATCH | `/orgs/{orgId}/mock-jira/issues/{issueKey}` | member+ | Manually move a card between columns (also fires the same status-changed event a real Jira transition would) |

## 4. What This Changes in the Rest of the Plan

- `microservices.md` (Notification Service) — ticket creation is now
  described as "via the org's configured `TicketProvider`," default
  `mock_jira`, optional `atlassian_jira`, rather than assuming Jira Cloud
  unconditionally.
- `ROADMAP.md` Phase 4 — build `MockJiraProvider` first (it's what the demo
  ships with and needs no external account); `AtlassianJiraProvider` is a
  same-phase stretch item, exercised against your own free Jira Cloud site
  for interview purposes, not required for the public demo to function.
- A new **Phase 7 — Public Demo Deployment** closes the roadmap (see
  `ROADMAP.md`), covering the Oracle Cloud VM setup, Cloudflare Tunnel,
  demo-org seeding, and rate limiting described above.
