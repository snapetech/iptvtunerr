#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
repo="${2:-${GITHUB_REPOSITORY:-}}"

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: $0 vX.Y.Z [owner/repository]" >&2
  exit 2
fi
if [[ -z "$repo" ]]; then
  echo "A repository name is required (owner/repository)." >&2
  exit 2
fi
if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required to dispatch release channels." >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to match dispatched release-channel runs." >&2
  exit 2
fi

failed=0
declare -a queued_workflows=() queued_titles=() queued_times=() queued_ids=()
while IFS=' ' read -r workflow input_name run_title; do
  [[ -n "$workflow" ]] || continue

  skip_channels=",${SKIP_RELEASE_CHANNELS//[[:space:]]/},"
  if [[ "$skip_channels" == *",$workflow,"* ]]; then
    echo "Skipping deferred release channel $workflow for $tag"
    continue
  fi

  if [[ ! -f ".github/workflows/$workflow" ]]; then
    echo "Configured release-channel workflow is missing: $workflow" >&2
    failed=1
    continue
  fi

  started_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  echo "Dispatching $workflow for $tag"
  dispatch_args=(--repo "$repo" --ref main -f "${input_name}=$tag")
  if [[ "$workflow" == "docker.yml" ]]; then
    dispatch_args+=(-f "publish_dockerhub=false")
  fi
  if gh workflow run "$workflow" "${dispatch_args[@]}"; then
    queued_workflows+=("$workflow")
    queued_titles+=("${run_title} ${tag}")
    queued_times+=("$started_at")
    queued_ids+=("")
    echo "Queued $workflow for $tag"
  else
    echo "ERROR: could not dispatch $workflow for $tag" >&2
    failed=1
  fi
done <<CHANNELS
docker.yml tag Publish Docker
release-aur.yml tag Publish AUR
release-ppa.yml tag Publish PPA
release-copr.yml tag Publish COPR
publish-chocolatey.yml version Publish Chocolatey
publish-winget.yml version Publish Winget
publish-snap.yml tag Publish Snap
CHANNELS

for index in "${!queued_workflows[@]}"; do
  workflow="${queued_workflows[$index]}"
  expected_title="${queued_titles[$index]}"
  started_at="${queued_times[$index]}"
  echo "Waiting for the $workflow run '$expected_title' to appear"

  for attempt in $(seq 1 60); do
    runs="$(gh run list --repo "$repo" --workflow "$workflow" --limit 100 \
      --json databaseId,displayTitle,createdAt,event)" || {
      echo "ERROR: could not list runs for $workflow" >&2
      failed=1
      break
    }
    run_id="$(jq -r --arg title "$expected_title" --arg started "$started_at" \
      '[.[] | select(.event == "workflow_dispatch" and .displayTitle == $title and .createdAt >= $started)][0].databaseId // empty' \
      <<< "$runs")"
    if [[ -n "$run_id" ]]; then
      queued_ids[$index]="$run_id"
      echo "$workflow run $run_id started for $tag"
      break
    fi
    if (( attempt == 60 )); then
      echo "ERROR: no workflow run appeared for $workflow ($expected_title)" >&2
      failed=1
      break
    fi
    sleep 5
  done
done

max_wait_seconds="${RELEASE_CHANNEL_TIMEOUT_SECONDS:-10800}"
waited_seconds=0
while (( waited_seconds < max_wait_seconds )); do
  all_finished=1
  for index in "${!queued_workflows[@]}"; do
    workflow="${queued_workflows[$index]}"
    run_id="${queued_ids[$index]}"
    [[ -n "$run_id" ]] || continue

    state="$(gh api "repos/${repo}/actions/runs/${run_id}" --jq '[.status, (.conclusion // "")] | @tsv')" || {
      echo "ERROR: could not read status for $workflow run $run_id" >&2
      failed=1
      all_finished=0
      continue
    }
    IFS=$'\t' read -r status conclusion <<< "$state"
    if [[ "$status" != "completed" ]]; then
      all_finished=0
      continue
    fi
    if [[ "$conclusion" != "success" ]]; then
      echo "ERROR: $workflow run $run_id completed with conclusion '$conclusion'" >&2
      failed=1
    else
      echo "$workflow run $run_id completed successfully"
    fi
    queued_ids[$index]=""
  done

  if (( all_finished )); then
    exit "$failed"
  fi
  sleep 15
  waited_seconds=$((waited_seconds + 15))
done

echo "ERROR: release-channel publishers did not all finish within ${max_wait_seconds} seconds" >&2
exit 1
