# Design Decisions and Trade-offs I'd Defend

One page, the Phase 6 "walk me through how this fails and recovers"
interview prep the roadmap asks for. Each one is a real choice made
somewhere in this codebase, with the alternative named and why it lost.

**REST/JSON everywhere, no gRPC.** Every internal service-to-service call
(`shared/httpclient`) and every external one is plain HTTP+JSON. gRPC
would give smaller payloads and generated clients; it costs a protobuf
toolchain, codegen step, and a second protocol to debug/observe
alongside the public REST API gateway already has to speak. For 11
services whose call graphs are simple request/response, not
high-throughput streaming, the operational simplicity won. I'd revisit
this if any one internal call path became genuinely bandwidth- or
latency-bound — nothing here is.

**Transactional outbox over synchronous side-effects.** Notification
Service never calls Slack/email/Jira synchronously from inside a Kafka
consumer; it writes an `outbox` row in the same logical operation and a
separate poller drains it (`FOR UPDATE SKIP LOCKED`, safe across
replicas with zero coordination). The alternative — call Slack directly
from the consumer — is simpler code but couples message durability to
whether Slack's API happens to be up at that exact moment. The trade-off
is a few seconds of added latency and one more table, for a guarantee
(every accepted Kafka message eventually produces an attempt, survives a
pod crash mid-send) worth defending.

**KEDA over plain HPA for the Kafka-consumer services.** CPU-based HPA
would scale `transcription-service` on CPU load, which lags the real
signal (Kafka backlog) by however long it takes CPU to actually climb.
KEDA's `ScaledObject` scales directly on `kafka_consumergroup_lag` — the
thing you actually care about — and supports scale-to-zero, a real cost
story on a laptop, not just a cloud-bill one. The cost is a second
scaling primitive to understand instead of one; worth it because the
five AI-pipeline services and the five CRUD/proxy services genuinely
have different scaling shapes (queue depth vs. request rate), and
pretending they're the same with one mechanism would be the actual
oversimplification.

**Application-level `org_id` filtering as the real tenant boundary, not
Postgres RLS.** RLS policies exist in the schema (per the original
design) but don't do anything — every service connects as the table
owner, which RLS never restricts. Found via `database-schema.md`'s own
documented incident, not hidden: every repository query now filters by
`org_id` explicitly as the actual enforcement. I'd defend keeping RLS in
the schema even though it's currently inert — it's cheap insurance for
the day each service gets its own non-superuser role (the named,
not-yet-done fix), at which point RLS becomes real defense-in-depth for
free, with zero query-layer changes.

**Logfmt over structured JSON logging.** Every log line is hand-built
`key=value` text (`shared/logger`), not `log/slog` with a JSON handler.
JSON is more machine-parseable in the abstract; logfmt is what Loki's
`| logfmt` LogQL stage expects natively, costs nothing to read in a
terminal during local dev, and needed no third-party logging library.
The trade-off is giving up nested structured fields — never needed here,
since every log line's payload is flat key/value pairs already.

**Split Helm charts for scaling, same binary underneath.**
`search-service` ships two Deployments (`-api` HPA'd, `-worker`
KEDA-`ScaledObject`'d) of the *literal same image*, because the
architecture doc asks for independent scaling knobs but the Go binary
has no `-api-only`/`-worker-only` mode. I'd defend shipping this now,
with the caveat stated plainly in that chart's own `Chart.yaml` (every
pod runs both halves regardless of which Deployment it's in), over either
silently pretending the split is cleaner than it is, or not shipping
separate scaling knobs at all until a real binary split exists. Honest
partial progress beats blocking on the ideal.

**HS256 JWTs where the architecture doc specifies RS256.** Found while
writing `docs/SECURITY_CHECKLIST.md` for this same phase, not fixed in
it. A real production deployment should defend RS256 (verification-only
public keys, no service needs the signing key at all) — but I'd rather
surface a real doc/code mismatch explicitly than quietly "fix" the doc to
match the code without flagging that the original security design was
never actually built as specified.

**Where this project draws its own line.** Nothing above required adding
scope beyond what's genuinely load-bearing for this system's own
behavior. The same discipline applied in reverse too: Linkerd mTLS,
`cosign` image signing, and Argo Rollouts canaries are named stretch
goals in the architecture doc itself and stayed that way here — adding
them wouldn't have taught anything this project doesn't already
demonstrate with its existing security/GitOps story, and partially-wired
versions of either would be worse than clearly naming them undone (see
`docs/SECURITY_CHECKLIST.md`).
