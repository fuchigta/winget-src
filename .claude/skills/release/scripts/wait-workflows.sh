#!/usr/bin/env bash
# Wait for the Release workflow to complete.
# Exits 0 on success, 1 on timeout (still in progress), 2 on failure.
# Usage: bash wait-workflows.sh [max_iterations] [sleep_interval_seconds]
#
# Designed to fit within Claude Code's 10-minute Bash timeout:
#   default 8 iterations x 60s = max 8 minutes per call.
# If it exits 1 (timeout), re-run the script to continue waiting.

REPO="fuchigta/winget-src"
MAX=${1:-8}
INTERVAL=${2:-60}

for i in $(seq 1 "$MAX"); do
  REL=$(gh run list --repo "$REPO" --workflow=release.yml --limit=1 --json status,conclusion,databaseId --jq '.[0]')

  REL_STATUS=$(echo "$REL" | jq -r '.status')
  REL_RESULT=$(echo "$REL" | jq -r '.conclusion // "—"')
  REL_ID=$(echo "$REL" | jq -r '.databaseId')

  echo "[$i/$MAX] Release=$REL_STATUS($REL_RESULT) run-id=$REL_ID"

  if [ "$REL_STATUS" = "completed" ]; then
    if [ "$REL_RESULT" = "success" ]; then
      echo "done: workflow succeeded"
      exit 0
    else
      echo "done: failure detected (Release=$REL_RESULT)"
      exit 2
    fi
  fi

  [ "$i" -lt "$MAX" ] && sleep "$INTERVAL"
done

echo "timeout: workflow still in progress — re-run this script to continue"
exit 1
