#!/usr/bin/env bash
# Seeds the Phase 7 public demo org — docs/architecture/deployment-demo-strategy.md
# §2's "3-4 pre-processed sample meetings (transcript, summary, action
# items, a working RAG chat already indexed)" so a stranger's first click
# loads instantly instead of hitting a cold-start pipeline run.
#
# This is a real, runnable tool against a real running deployment — it
# signs up through the actual public REST API (POST /auth/signup), uploads
# through the actual presigned-URL flow (POST /meetings -> PUT the bytes ->
# POST .../complete-upload), and polls the actual Kafka-driven status
# machine (GET .../status) exactly the way a real user's browser does. It
# deliberately does NOT insert rows into Postgres directly — that would
# seed data the real pipeline never touched, which defeats the point of a
# "watch a real system work" demo.
#
# NOT YET RUN — same honest gap as scripts/chaos-test.sh and
# docs/runbooks/dr-restore.md: this needs a live deployment (local
# docker-compose or the Phase 7 demo VM) to run against, and this project's
# own development sandbox has neither a Docker daemon nor a reachable VM
# (see README.md's Phase 5/6/7 status notes). There is also one real input
# this repo cannot supply: short sample meeting recordings. Dropping a
# handful of real, short (under the demo's ~25MB / ~2min clip-length cap —
# see deployments/configs/demo/README.md) audio files into
# deployments/demo-seed/samples/ before running this script is a manual,
# one-time step for whoever actually stands up the demo — this script
# processes whatever's in that directory, it doesn't fabricate recordings.
#
# Usage:
#   BASE_URL=https://demo.yourdomain.com/api/v1 ./scripts/seed-demo-org.sh
#   (defaults to http://localhost:8000/api/v1 — a local docker-compose stack)
#
# Requires: curl, jq
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8000/api/v1}"
SAMPLES_DIR="${SAMPLES_DIR:-$(dirname "$0")/../deployments/demo-seed/samples}"
ORG_NAME="${DEMO_ORG_NAME:-AI Meeting Intelligence Demo}"
OWNER_NAME="${DEMO_OWNER_NAME:-Demo Owner}"
OWNER_EMAIL="${DEMO_OWNER_EMAIL:-demo-owner@example.com}"
OWNER_PASSWORD="${DEMO_OWNER_PASSWORD:-change-me-before-real-use}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-10}"
POLL_TIMEOUT_SECONDS="${POLL_TIMEOUT_SECONDS:-600}"

for bin in curl jq; do
  command -v "$bin" >/dev/null || { echo "missing required tool: $bin" >&2; exit 1; }
done

if [[ ! -d "$SAMPLES_DIR" ]] || [[ -z "$(ls -A "$SAMPLES_DIR" 2>/dev/null)" ]]; then
  echo "no sample recordings found in $SAMPLES_DIR" >&2
  echo "this repo doesn't ship any (see this script's own header) — drop 3-4" >&2
  echo "short (<2 min) real audio files there before running this script." >&2
  exit 1
fi

# signup first; a second run against an org that already exists falls back
# to login rather than failing outright, so this script is safe to re-run
# (e.g. to seed additional samples later) without hand-editing credentials.
echo "signing up demo org '$ORG_NAME'..."
signup_body=$(jq -n --arg org "$ORG_NAME" --arg name "$OWNER_NAME" --arg email "$OWNER_EMAIL" --arg pw "$OWNER_PASSWORD" \
  '{orgName: $org, name: $name, email: $email, password: $pw}')
signup_http_code=$(curl -s -o /tmp/seed-demo-signup.json -w '%{http_code}' \
  -X POST "$BASE_URL/auth/signup" -H 'Content-Type: application/json' -d "$signup_body")

if [[ "$signup_http_code" == "201" || "$signup_http_code" == "200" ]]; then
  access_token=$(jq -r '.accessToken' /tmp/seed-demo-signup.json)
  echo "signup succeeded."
else
  echo "signup returned HTTP $signup_http_code (likely already seeded) — trying login instead..."
  login_body=$(jq -n --arg email "$OWNER_EMAIL" --arg pw "$OWNER_PASSWORD" '{email: $email, password: $pw}')
  login_http_code=$(curl -s -o /tmp/seed-demo-login.json -w '%{http_code}' \
    -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' -d "$login_body")
  if [[ "$login_http_code" != "200" ]]; then
    echo "login also failed (HTTP $login_http_code) — can't proceed. Response:" >&2
    cat /tmp/seed-demo-login.json >&2
    exit 1
  fi
  access_token=$(jq -r '.accessToken' /tmp/seed-demo-login.json)
  echo "login succeeded."
fi
rm -f /tmp/seed-demo-signup.json /tmp/seed-demo-login.json

auth_header="Authorization: Bearer $access_token"

upload_one() {
  local file="$1" title
  title="$(basename "$file")"
  title="${title%.*}"

  echo
  echo "--- seeding '$title' from $file ---"

  local intent_body intent_http_code meeting_id upload_url
  intent_body=$(jq -n --arg t "$title" '{title: $t}')
  intent_http_code=$(curl -s -o /tmp/seed-demo-intent.json -w '%{http_code}' \
    -X POST "$BASE_URL/meetings" -H "$auth_header" -H 'Content-Type: application/json' -d "$intent_body")
  if [[ "$intent_http_code" != "201" ]]; then
    echo "create-upload-intent failed (HTTP $intent_http_code):" >&2
    cat /tmp/seed-demo-intent.json >&2
    return 1
  fi
  meeting_id=$(jq -r '.meetingId' /tmp/seed-demo-intent.json)
  upload_url=$(jq -r '.uploadUrl' /tmp/seed-demo-intent.json)
  rm -f /tmp/seed-demo-intent.json
  echo "meeting $meeting_id created, uploading recording bytes directly to object storage..."

  local put_http_code
  put_http_code=$(curl -s -o /dev/null -w '%{http_code}' -X PUT --upload-file "$file" "$upload_url")
  if [[ "$put_http_code" != "200" ]]; then
    echo "presigned PUT failed (HTTP $put_http_code)" >&2
    return 1
  fi

  local confirm_http_code
  confirm_http_code=$(curl -s -o /dev/null -w '%{http_code}' \
    -X POST "$BASE_URL/meetings/$meeting_id/complete-upload" -H "$auth_header")
  if [[ "$confirm_http_code" != "200" ]]; then
    echo "complete-upload failed (HTTP $confirm_http_code)" >&2
    return 1
  fi
  echo "upload confirmed — meeting.uploaded.v1 published, pipeline running."

  local waited=0 status
  while (( waited < POLL_TIMEOUT_SECONDS )); do
    status=$(curl -s -H "$auth_header" "$BASE_URL/meetings/$meeting_id/status" | jq -r '.status')
    echo "  [$waited s] status: $status"
    case "$status" in
      completed) echo "  done."; return 0 ;;
      failed) echo "  pipeline reported failed — check each service's logs (see docs/runbooks/incident-response.md)." >&2; return 1 ;;
    esac
    sleep "$POLL_INTERVAL_SECONDS"
    waited=$((waited + POLL_INTERVAL_SECONDS))
  done
  echo "  timed out after ${POLL_TIMEOUT_SECONDS}s still at status '$status'" >&2
  return 1
}

seeded=0
failed=0
for file in "$SAMPLES_DIR"/*; do
  [[ -f "$file" ]] || continue
  if upload_one "$file"; then
    seeded=$((seeded + 1))
  else
    failed=$((failed + 1))
  fi
done

echo
echo "=== seeding complete: $seeded succeeded, $failed failed ==="
echo "Demo org owner login: $OWNER_EMAIL"
echo "Mock Jira board (no auth needed): $BASE_URL/demo/board"
[[ "$failed" -eq 0 ]]
