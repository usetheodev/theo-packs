#!/usr/bin/env bash
# Opens a pull request against usetheodev/theo-packs via the GitHub REST API.
#
# Why this exists: sandboxes that author branches frequently have neither
# `gh` authenticated nor an interactive shell to run `gh auth login`. The
# branch is pushed; only the PR creation needs a credential.
#
# All four PR fields default to the multi-language-providers PR but can be
# overridden via env vars to reuse this script for any subsequent PR.
#
# Usage examples
# --------------
# Multi-language providers PR (defaults):
#   GH_TOKEN=<pat> ./scripts/open-pr.sh
#
# Dockerfile correctness + efficiency PR:
#   GH_TOKEN=<pat> \
#     HEAD_BRANCH=feat/dockerfile-correctness-and-efficiency \
#     BASE_BRANCH=develop \
#     TITLE='fix(dockerfile): correctness + efficiency (11 audit findings)' \
#     BODY_FILE=docs/plans/PR_DESCRIPTION_DOCKERFILE_FIX.md \
#     ./scripts/open-pr.sh
#
# Or with gh (covers this case too):
#   gh pr create --base develop --head <branch> --title '<title>' --body-file <file>
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
