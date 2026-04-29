#!/usr/bin/env bash
# Opens the multi-language-providers PR against usetheodev/theo-packs.
#
# Why this exists: the sandbox where this branch was authored has neither
# `gh` authenticated nor an interactive shell to run `gh auth login`. The
# branch is pushed; only the PR creation needs a credential. Any of the
# following invocations is sufficient.
#
# Usage:
#   GH_TOKEN=<personal-access-token> ./scripts/open-multi-language-pr.sh
#
# Or with gh (which already covers this — but kept here for parity):
#   gh pr create \
#     --base main \
#     --head feat/add-six-language-providers \
#     --title 'feat: add Rust, Java, .NET, Ruby, PHP, and Deno language providers' \
#     --body-file docs/plans/PR_DESCRIPTION.md
#
# The PAT needs `repo` scope (Classic) or `Pull requests: write` +
# `Contents: read` (fine-grained, scoped to usetheodev/theo-packs).

set -euo pipefail

REPO="${REPO:-usetheodev/theo-packs}"
HEAD_BRANCH="${HEAD_BRANCH:-feat/add-six-language-providers}"
BASE_BRANCH="${BASE_BRANCH:-main}"
TITLE="${TITLE:-feat: add Rust, Java, .NET, Ruby, PHP, and Deno language providers}"
BODY_FILE="${BODY_FILE:-docs/plans/PR_DESCRIPTION.md}"

if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  echo "error: set GH_TOKEN (or GITHUB_TOKEN) to a PAT with repo / Pull requests:write scope." >&2
  exit 1
fi
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN}}"

if [[ ! -f "$BODY_FILE" ]]; then
  echo "error: body file not found: $BODY_FILE" >&2
  echo "       run this script from the repository root." >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required to safely build the JSON payload." >&2
  echo "       install with: apt-get install -y jq  (or your package manager)" >&2
  exit 1
fi

PAYLOAD=$(jq -n \
  --arg title "$TITLE" \
  --arg head "$HEAD_BRANCH" \
  --arg base "$BASE_BRANCH" \
  --rawfile body "$BODY_FILE" \
  '{title: $title, head: $head, base: $base, body: $body, draft: false, maintainer_can_modify: true}')

echo "Opening PR ${HEAD_BRANCH} -> ${BASE_BRANCH} on ${REPO}…" >&2

RESPONSE=$(curl -fsSL \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  -X POST \
  "https://api.github.com/repos/${REPO}/pulls" \
  -d "$PAYLOAD")

URL=$(echo "$RESPONSE" | jq -r '.html_url // empty')
NUMBER=$(echo "$RESPONSE" | jq -r '.number // empty')

if [[ -z "$URL" ]]; then
  echo "error: GitHub did not return an html_url. Full response:" >&2
  echo "$RESPONSE" >&2
  exit 2
fi

echo "PR #${NUMBER} opened: ${URL}"
