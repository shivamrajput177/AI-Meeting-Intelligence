#!/usr/bin/env bash
#
# Creates a private GitLab project from linkcheck.bundle and opens the practice
# merge request "Check links concurrently with a worker pool".
#
# Usage:
#   ./setup-gitlab-mr.sh <your-gitlab-username> [project-name]
#
# Requirements:
#   - git, and an SSH key added to GitLab (or set REMOTE to an HTTPS URL).
#   - Optional: GITLAB_TOKEN (a personal access token with the `api` scope).
#     With a token, the MR gets the full description from MR_DESCRIPTION.md.
#     Without one, the MR is created with push options and you paste the
#     description in yourself.
#
# Environment overrides: GITLAB_HOST (default gitlab.com), REMOTE, DEST.
set -euo pipefail

NAMESPACE="${1:?usage: $0 <gitlab-username> [project-name]}"
PROJECT="${2:-linkcheck}"
HOST="${GITLAB_HOST:-gitlab.com}"
REMOTE="${REMOTE:-git@${HOST}:${NAMESPACE}/${PROJECT}.git}"
HERE="$(cd "$(dirname "$0")" && pwd)"
DEST="${DEST:-$PWD/$PROJECT}"
BRANCH="concurrent-link-checks"
TITLE="Check links concurrently with a worker pool"

if [[ -e "$DEST" ]]; then
  echo "error: $DEST already exists. Remove it or set DEST." >&2
  exit 1
fi

echo "==> Cloning the practice repository into $DEST"
git clone --quiet --branch main "$HERE/linkcheck.bundle" "$DEST"
cd "$DEST"
git branch --quiet "$BRANCH" "origin/$BRANCH"
git remote remove origin
git remote add origin "$REMOTE"

# Pushing to a project that doesn't exist yet creates it as a *private*
# project (GitLab "push to create").
echo "==> Pushing main to $REMOTE (creates a private project)"
git push --quiet -u origin main

if [[ -n "${GITLAB_TOKEN:-}" ]]; then
  echo "==> Pushing $BRANCH"
  git push --quiet -u origin "$BRANCH"

  echo "==> Opening the merge request through the API"
  project_id="$(python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$NAMESPACE/$PROJECT")"
  payload="$(python3 - "$HERE/MR_DESCRIPTION.md" "$BRANCH" "$TITLE" <<'PY'
import json, sys
path, branch, title = sys.argv[1:]
print(json.dumps({
    "source_branch": branch,
    "target_branch": "main",
    "title": title,
    "description": open(path).read(),
    "remove_source_branch": True,
}))
PY
)"
  curl --silent --show-error --fail \
    --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
    --header "Content-Type: application/json" \
    --data "$payload" \
    "https://${HOST}/api/v4/projects/${project_id}/merge_requests" \
    | python3 -c 'import json, sys; print("==> MR created: " + json.load(sys.stdin)["web_url"])'
else
  echo "==> Pushing $BRANCH and opening the merge request with push options"
  git push -u origin "$BRANCH" \
    -o merge_request.create \
    -o merge_request.target=main \
    -o "merge_request.title=$TITLE" \
    -o merge_request.remove_source_branch
  echo
  echo "Next: open the MR on GitLab, choose Edit, and paste the contents of"
  echo "  $HERE/MR_DESCRIPTION.md"
  echo "into the description so it reads like a real MR."
fi

echo
echo "Done. Your local clone is at $DEST"
echo "Start with: cd $DEST && git checkout $BRANCH && make test"
