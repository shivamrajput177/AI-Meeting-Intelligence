# External REST API Specification

Base URL: `/api/v1`. Auth: `Authorization: Bearer <JWT>` unless noted public.
All responses `application/json`; errors follow
`{"error": {"code": "string", "message": "string", "requestId": "uuid"}}`.
Full machine-readable spec to be maintained as `openapi.yaml` alongside this
doc once implementation starts (generated from gRPC via `protoc-gen-openapiv2`
or hand-maintained — decide in Phase 1).

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

| Method | Path | Roles | Description |
|---|---|---|---|
| GET | `/users/me` | any | Current profile |
| PATCH | `/users/me` | any | Update own profile |
| GET | `/orgs/{orgId}/users` | any member | List org users |
| POST | `/orgs/{orgId}/invites` | owner, admin | Invite user |
| POST | `/invites/{token}/accept` | public | Accept invite |
| PATCH | `/orgs/{orgId}/users/{userId}/role` | owner, admin | Change RBAC role |
| DELETE | `/orgs/{orgId}/users/{userId}` | owner, admin | Deactivate user |

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
| POST | `/action-items/{id}/jira-ticket` | member+ | Create linked Jira ticket |

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

## RBAC Role Matrix (summary)

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
