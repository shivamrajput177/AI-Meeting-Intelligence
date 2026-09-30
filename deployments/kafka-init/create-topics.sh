#!/bin/bash
# create-topics.sh runs once per `docker compose up` (the kafka-init
# service's whole job) to create every topic each landed phase's services
# produce — see docs/architecture/kafka-topics.md for the full catalog,
# naming convention, and partition-key/retention rationale this mirrors
# exactly.
#
# Scope: only topics *produced* by a landed phase's service (Meeting,
# Transcription, AI Summary, Action Item — Phase 2; Search — Phase 3;
# Notification — Phase 4.1; see docs/ROADMAP.md) are created here. The
# catalog also documents topics owned by later phases' services
# (Analytics's own consumer group needs no topics of its own to create,
# and Auth/User/Org's own event stream, plus Notification's own
# action-item.jira-requested.v1/action-item.reminder-due.v1 consumed
# topics, land in Phase 4.2-4.5) — those get created here once the phase
# that builds their producer lands, the same way this file itself didn't
# exist until Phase 2 needed it.
#
# DLQ topics: per kafka-topics.md's own rule, only *.completed.v1,
# *.extracted.v1, and *.status-changed.v1 topics get a .dlq companion —
# not every topic (e.g. chunk.created.v1 and the *.failed.v1 topics
# don't, since a "failed" topic already *is* the failure signal).
set -euo pipefail

BROKER="kafka:9092"
DAY=$((24 * 60 * 60 * 1000))

create_topic() {
	local topic="$1" partitions="$2" retention_ms="$3"
	/opt/kafka/bin/kafka-topics.sh --bootstrap-server "$BROKER" --create --if-not-exists \
		--topic "$topic" --partitions "$partitions" --replication-factor 1 \
		--config "retention.ms=$retention_ms"
}

# topic                                  partitions  retention
create_topic meeting.uploaded.v1                  12 $((7 * DAY))
create_topic meeting.status-changed.v1            12 $((30 * DAY))
create_topic meeting.status-changed.v1.dlq        12 $((30 * DAY))
create_topic meeting.deleted.v1                   12 $((7 * DAY))

create_topic transcription.completed.v1           12 $((30 * DAY))
create_topic transcription.completed.v1.dlq       12 $((30 * DAY))
create_topic transcription.failed.v1              12 $((7 * DAY))

create_topic chunk.created.v1                     12 $((30 * DAY))

create_topic summary.completed.v1                 12 $((30 * DAY))
create_topic summary.completed.v1.dlq             12 $((30 * DAY))
create_topic summary.failed.v1                    12 $((7 * DAY))

create_topic action-item.extracted.v1             12 $((30 * DAY))
create_topic action-item.extracted.v1.dlq         12 $((30 * DAY))
create_topic action-item.extraction-failed.v1     12 $((7 * DAY))
create_topic action-item.status-changed.v1         6 $((30 * DAY))
create_topic action-item.status-changed.v1.dlq     6 $((30 * DAY))

create_topic embedding.completed.v1               12 $((7 * DAY))
create_topic embedding.completed.v1.dlq           12 $((7 * DAY))
create_topic embedding.failed.v1                  12 $((7 * DAY))

# Neither topic below gets a .dlq — a delivery audit/failure signal
# already *is* the outcome, per this file's own header comment.
create_topic notification.sent.v1                  6 $((7 * DAY))
create_topic notification.failed.v1                6 $((7 * DAY))

echo "kafka-init: topics created."
