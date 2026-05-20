#!/usr/bin/env bash
# Aggregates `go test -json` output across N runs and reports tests
# whose pass count < N (i.e., flaky). Input: a directory of `run-*.json`
# files. Output: human-readable report to stdout.
#
# T5.5 — robust-test-suite-plan.

set -euo pipefail

if [ "${1:-}" = "" ]; then
  echo "usage: $0 <dir-with-run-N.json>"
  exit 1
fi

dir="$1"
runs=$(ls "$dir"/run-*.json | wc -l)
if [ "$runs" -eq 0 ]; then
  echo "no run-*.json files in $dir"
  exit 1
fi

echo "Flakiness report — $runs runs in $dir"
echo "============================================"
echo ""

# Per (Package, Test) tuple count PASS events.
jq -r 'select(.Action == "pass" and .Test != null) | "\(.Package)::\(.Test)"' \
  "$dir"/run-*.json |
  sort | uniq -c |
  awk -v runs="$runs" '$1 < runs {
    printf "FLAKY  %d/%d  %s\n", $1, runs, substr($0, index($0, $2))
  }' | sort -n

# Failures appear when Action == "fail" — collect distinct.
echo ""
echo "Failures observed (at least one run):"
jq -r 'select(.Action == "fail" and .Test != null) | "  \(.Package)::\(.Test)"' \
  "$dir"/run-*.json |
  sort -u

# Total flaky count for the dashboard threshold (5% = page).
flaky_count=$(jq -r 'select(.Action == "pass" and .Test != null) | "\(.Package)::\(.Test)"' \
  "$dir"/run-*.json | sort | uniq -c | awk -v runs="$runs" '$1 < runs' | wc -l)
total=$(jq -r 'select(.Action == "pass" and .Test != null) | "\(.Package)::\(.Test)"' \
  "$dir"/run-*.json | sort -u | wc -l)

echo ""
echo "Summary: $flaky_count flaky / $total total"
if [ "$total" -gt 0 ]; then
  pct=$(( 100 * flaky_count / total ))
  echo "Flakiness rate: ${pct}%"
fi
