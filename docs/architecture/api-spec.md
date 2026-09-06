# External REST API Specification

Base URL: `/api/v1`. Auth: `Authorization: Bearer <JWT>` unless noted public.
All responses `application/json`; errors follow
`{"error": {"code": "string", "message": "string", "requestId": "uuid"}}`.
The REST API is **entirely hand-written** — no protobuf, no codegen. The
gateway's HTTP handlers are ordinary Go code that reverse-proxy to each
internal service's own REST API (also plain HTTP/JSON — see
`microservices.md` §"Internal Communication"; there's no gRPC anywhere in
this design). `openapi.yaml` is maintained by hand alongside this doc as
documentation/tooling (Swagger UI, client codegen for the frontend), not as
the source of truth for the API's behavior; the Go handler code is the
source of truth.

## Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/signup` | public | Create org + owner user |
| POST | `/auth/login` | public | Returns access + refresh token |
| POST | `/auth/refresh` | public (refresh token in body) | Rotate access token |
| POST | `/auth/logout` | required | Revoke refresh token |
| POST | `/auth/password/reset-request` | public | Send reset email |
| POST | `/auth/password/reset-confirm` | public | Set new password |

## Organizations

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/orgs/{orgId}` | any member | Org details |
| PATCH | `/orgs/{orgId}/settings` | owner, admin | Update settings/integrations |
| GET | `/orgs/{orgId}/usage` | owner, admin | Quota usage |

## Users

**Phase 1 ships only the first two rows.** A Phase 1 org has exactly one
role model — the signup user is `owner`, everyone else who signs up is
`member` — so there's nothing yet to invite into or assign a role within.
The rest of this table (invites, role changes, deactivation) is real RBAC
and lands in **Phase 2**, once there's more than one org member to manage.
See `ROADMAP.md` Phase 1 and Phase 2 for the reasoning.

| Method | Path | Roles | Description | Phase |
|---|---|---|---|---|
| GET | `/users/me` | any | Current profile | 1 |
| PATCH | `/users/me` | any | Update own profile | 1 |
| GET | `/orgs/{orgId}/users` | any member | List org users | 2 |
| POST | `/orgs/{orgId}/invites` | owner, admin | Invite user | 2 |
| POST | `/invites/{token}/accept` | public | Accept invite | 2 |
| PATCH | `/orgs/{orgId}/users/{userId}/role` | owner, admin | Change RBAC role | 2 |
| DELETE | `/orgs/{orgId}/users/{userId}` | owner, admin | Deactivate user | 2 |

## Meetings

| Method | Path | Roles | Description |
|---|---|---|---|
| POST | `/meetings` | member+ | Create meeting, get presigned upload URL |
| POST | `/meetings/{id}/complete-upload` | member+ | Confirm upload, triggers pipeline |
| GET | `/meetings/{id}` | member+ | Meeting metadata |
| GET | `/meetings` | member+ | List (paginated, filters: status, dateRange, participant) |
| GET | `/meetings/{id}/status` | member+ | Lightweight status poll |
| DELETE | `/meetings/{id}` | owner/creator, admin | Delete + cascade |
| POST | `/meetings/{id}/participants` | member+ | Add participants |

## Transcripts & Summaries

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/meetings/{id}/transcript` | member+ | Full transcript + segments |
| GET | `/meetings/{id}/summary` | member+ | Summary, decisions, risks, blockers |
| POST | `/meetings/{id}/summary/regenerate` | member+ | Re-run summarization |

## Action Items

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/meetings/{id}/action-items` | member+ | Items for one meeting |
| GET | `/action-items` | member+ | Cross-meeting, filters: `owner`, `status`, `type`, `dueBefore` |
| PATCH | `/action-items/{id}` | member+ (owner or admin) | Update status/owner/due date |
| POST | `/action-items/{id}/jira-ticket` | member+ | Create a ticket — see **Ticketing** section below |

## Search / Knowledge (RAG)

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/search?q=` | member+ | Semantic search across org's meetings |
| GET | `/meetings/{id}/similar` | member+ | Similar past meetings |
| POST | `/qa/ask` | member+ | RAG question answering, returns citations |
| GET | `/qa/history` | member+ | Past Q&A for the current user |

## Analytics

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/analytics/meetings/trends` | manager+ | Meeting volume/duration over time |
| GET | `/analytics/productivity` | manager+ | Per-team/per-user action item throughput |
| GET | `/analytics/action-items/completion-rate` | manager+ | Completion rate, aging open items |
| GET | `/analytics/topics` | manager+ | Most-discussed topics over time |

## Integrations (admin/debug)

| Method | Path | Roles | Description |
|---|---|---|---|
| POST | `/orgs/{orgId}/integrations/test` | owner, admin | Fire a test Slack/email/Jira call |

## Ticketing (mock Jira default, real Jira optional — see `deployment-demo-strategy.md` §3)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/action-items/{id}/jira-ticket` | member+ | Create a ticket via the org's configured `TicketProvider` (`mock_jira` by default, `atlassian_jira` if configured) |
| GET | `/demo/board` | public | Read-only mock Jira board for the public demo org — no login required |
| GET | `/orgs/{orgId}/mock-jira/board` | member+ | Same board, for any org using the mock provider |
| PATCH | `/orgs/{orgId}/mock-jira/issues/{issueKey}` | member+ | Move a card between columns; fires the same status-changed event a real Jira transition would |

## Pagination, filtering, errors

- List endpoints: `?page=1&pageSize=20`, response includes
  `{"data": [...], "page": 1, "pageSize": 20, "total": N}`.
- Standard HTTP status codes; `422` for validation errors with a field-level
  `details[]` array.
- Rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`,
  `X-RateLimit-Reset`.
- Every response carries `X-Request-Id` (= trace id) for correlating with
  Tempo/Loki.

## RBAC Role Matrix (Phase 2+ target — Phase 1 is just `owner`/`member`)

| Action | owner | admin | manager | member | viewer |
|---|:-:|:-:|:-:|:-:|:-:|
| Manage org settings/integrations | ✅ | ✅ | ❌ | ❌ | ❌ |
| Invite/remove users, change roles | ✅ | ✅ | ❌ | ❌ | ❌ |
| Upload/delete meetings | ✅ | ✅ | ✅ | ✅ | ❌ |
| View meetings/transcripts/summaries | ✅ | ✅ | ✅ | ✅ | ✅ |
| Update action item status (own) | ✅ | ✅ | ✅ | ✅ | ❌ |
| Reassign any action item | ✅ | ✅ | ✅ | ❌ | ❌ |
| View analytics | ✅ | ✅ | ✅ | ❌ | ❌ |
| Ask RAG questions | ✅ | ✅ | ✅ | ✅ | ✅ |
