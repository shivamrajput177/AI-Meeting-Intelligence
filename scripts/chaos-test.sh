#!/usr/bin/env bash
# Chaos test for Phase 6 — docs/architecture/observability-security.md §5's
# "Chaos testing" row: kill a random pod per stateful component, verify
# self-heal + no data loss, document findings.
#
# NOT YET RUN — see docs/runbooks/chaos-test-findings.md, left blank with
# a template for recording what actually happens the first real run
# against a live Kind cluster (this sandbox has none — same constraint
# named throughout README.md's Phase 5/6 status notes). This script is a
# real, runnable tool, not a design sketch: every command below is one
# you could paste into a terminal with kubectl pointed at a real cluster
# today and get a real result.
#
# Usage: ./scripts/chaos-test.sh <target>
#   target: postgres | kafka | minio | node
set -euo pipefail

TARGET="${1:-}"
if [[ -z "$TARGET" ]]; then
  echo "usage: $0 <postgres|kafka|minio|node>" >&2
  exit 1
fi

kill_random_pod() {
  local namespace="$1" label="$2"
  local pod
  pod=$(kubectl get pods -n "$namespace" -l "$label" -o jsonpath='{.items[0].metadata.name}')
  if [[ -z "$pod" ]]; then
    echo "no pod found matching $label in $namespace" >&2
    exit 1
  fi
  echo "killing pod $pod in $namespace..."
  kubectl delete pod -n "$namespace" "$pod"
  echo "watching for a replacement to become Ready (90s timeout)..."
  kubectl wait --for=condition=Ready pod -n "$namespace" -l "$label" --timeout=90s
  echo "self-heal confirmed: a new pod matching $label is Ready."
}

case "$TARGET" in
  postgres)
    kill_random_pod meeting-intel-data app.kubernetes.io/name=postgresql
    echo "Next: verify no data loss — compare a row count or recent record"
    echo "against what Grafana's Postgres Overview dashboard showed just before"
    echo "the kill (deploy/infra/observability/dashboards/postgres-overview.json)."
    ;;
  kafka)
    kill_random_pod meeting-intel-data strimzi.io/cluster=meeting-intel-kafka
    echo "Next: verify no message loss for in-flight consumption — check"
    echo "kafka_consumergroup_lag in Grafana's Kafka Overview dashboard"
    echo "returns to its pre-kill baseline rather than climbing forever."
    ;;
  minio)
    kill_random_pod meeting-intel-data v1.min.io/tenant=meeting-intel
    echo "Next: verify an in-progress or just-completed meeting upload is"
    echo "still readable (GET the recording's presigned URL again)."
    ;;
  node)
    echo "Draining a Kind worker (simulated node loss) — this one needs a"
    echo "node name, not a pod label:"
    kubectl get nodes
    read -rp "node to drain: " node
    kubectl drain "$node" --ignore-daemonsets --delete-emptydir-data --force --timeout=120s
    echo "Watch pods reschedule onto remaining nodes:"
    kubectl get pods -A -o wide --watch &
    WATCH_PID=$!
    read -rp "press enter once rescheduling looks complete to stop watching and uncordon..."
    kill "$WATCH_PID" 2>/dev/null || true
    kubectl uncordon "$node"
    ;;
  *)
    echo "unknown target: $TARGET (expected postgres|kafka|minio|node)" >&2
    exit 1
    ;;
esac

echo
echo "Record what actually happened in docs/runbooks/chaos-test-findings.md —"
echo "this script confirms self-heal happened, it doesn't write the findings for you."
