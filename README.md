# AI-Meeting-Intelligence

AI-powered Meeting Intelligence Platform built with Golang, Kafka, PostgreSQL, pgvector, Kubernetes, and local LLMs. Transcribes meetings, generates summaries, extracts action items, enables semantic search, and transforms conversations into a searchable organizational knowledge base.

## Design & Planning Docs

The full system design lives under [`docs/`](docs/PROJECT_PLAN.md):

- [`docs/PROJECT_PLAN.md`](docs/PROJECT_PLAN.md) — executive summary, HLD diagram, tech stack, key design decisions
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — 7-phase implementation roadmap (MVP → AI processing → search/RAG → integrations → Kubernetes/CI-CD → production readiness → public demo deployment)
- [`docs/architecture/`](docs/architecture/) — per-service LLD, database schema, Kafka topic design & event flows, REST API spec, Kubernetes/Helm/CI-CD/ArgoCD, observability/security/multi-tenancy/DR/cost, deployment & demo strategy
- [`proto/`](proto/) — gRPC contracts (also the schema source for Kafka payloads)

No implementation code exists yet — this repository currently holds the architecture and planning artifacts that Phase 1 will be built against.
