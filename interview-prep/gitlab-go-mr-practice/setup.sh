#!/usr/bin/env bash
# Builds the practice repository with a realistic history:
#
#   main                          <- the "existing" project
#   feature/concurrent-checks     <- the merge request you will review
#
# Usage:
#   ./setup.sh <target-dir>                     # local repo only
#   ./setup.sh <target-dir> <gitlab-remote-url> # also pushes and opens the MR
#
# Example:
#   ./setup.sh ~/code/endpoint-monitor git@gitlab.com:<your-username>/endpoint-monitor.git
#
# Pushing to a project path that does not exist yet makes GitLab create it as
# a PRIVATE project ("push to create"), and the push options below open the
# merge request for you.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
target="${1:?usage: $0 <target-dir> [gitlab-remote-url]}"
remote="${2:-}"
branch="feature/concurrent-checks"

if [ -e "$target" ]; then
  echo "error: $target already exists" >&2
  exit 1
fi

mkdir -p "$target"
cd "$target"
git init -q -b main

commit() {
  local name="$1" email="$2" date="$3" msg="$4"
  git add -A
  GIT_AUTHOR_NAME="$name" GIT_AUTHOR_EMAIL="$email" GIT_AUTHOR_DATE="$date" \
  GIT_COMMITTER_NAME="$name" GIT_COMMITTER_EMAIL="$email" GIT_COMMITTER_DATE="$date" \
    git commit -q -m "$msg"
}

# --- main ---------------------------------------------------------------
cp -R "$here/base/." .
commit "Priya Maintainer" "priya.maintainer@example.com" "2026-09-01T10:00:00Z" \
  "Add endpoint monitor service

Periodically checks a list of HTTP endpoints and exposes the latest
results on /status."

# --- merge request branch ------------------------------------------------
git checkout -q -b "$branch"

cp -R "$here/mr/." .
cp "$here/mr-commit1/internal/monitor/checker_test.go" internal/monitor/checker_test.go
commit "Sam Contributor" "sam.contributor@example.com" "2026-10-01T14:12:00Z" \
  "Check endpoints concurrently

With a few dozen endpoints a full run takes longer than the check
interval because every endpoint is checked sequentially. Check them
concurrently, limited by the new -concurrency flag.

Also add:
- a -once mode for use in CI jobs
- run statistics exposed on /stats"

cp "$here/mr/internal/monitor/checker_test.go" internal/monitor/checker_test.go
commit "Sam Contributor" "sam.contributor@example.com" "2026-10-01T16:47:00Z" \
  "Fix flaky CheckAll test"

git checkout -q main

echo "Repository created in $target"
echo "  main:     $(git log --oneline -1 main)"
echo "  $branch: $(git rev-list --count main.."$branch") commits ahead of main"

if [ -z "$remote" ]; then
  echo
  echo "No remote given. To review locally:  git diff main...$branch"
  exit 0
fi

git remote add origin "$remote"
git push -q -u origin main
git push -q -u origin "$branch" \
  -o merge_request.create \
  -o merge_request.target=main \
  -o "merge_request.title=Check endpoints concurrently and add -once mode" \
  -o merge_request.remove_source_branch

echo
echo "Pushed. Open the merge request in GitLab and paste MR_DESCRIPTION.md"
echo "into its description (push options cannot carry multi-line text)."
