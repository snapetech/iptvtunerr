#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dispatcher="$root/scripts/dispatch-release-channels.sh"

expected_channels=$(cat <<'CHANNELS'
docker.yml tag Publish Docker
release-aur.yml tag Publish AUR
release-ppa.yml tag Publish PPA
release-copr.yml tag Publish COPR
publish-chocolatey.yml version Publish Chocolatey
publish-winget.yml version Publish Winget
publish-snap.yml tag Publish Snap
CHANNELS
)
actual_channels=$(awk '
  /^done <<CHANNELS$/ { in_channels = 1; next }
  /^CHANNELS$/ && in_channels { exit }
  in_channels { print }
' "$dispatcher")

if [[ "$actual_channels" != "$expected_channels" ]]; then
  echo "Release channel dispatch map differs from the required channel set." >&2
  echo "Expected:" >&2
  printf '%s\n' "$expected_channels" >&2
  echo "Found:" >&2
  printf '%s\n' "$actual_channels" >&2
  exit 1
fi

while read -r workflow input_name run_title; do
  workflow_path="$root/.github/workflows/$workflow"
  if [[ ! -f "$workflow_path" ]]; then
    echo "Missing release publisher workflow: $workflow" >&2
    exit 1
  fi
  if ! rg -q "^[[:space:]]*${input_name}:" "$workflow_path"; then
    echo "$workflow does not declare the dispatched '${input_name}' input." >&2
    exit 1
  fi
  expected_run_name="run-name: ${run_title} \${{ inputs.${input_name} }}"
  if ! rg -Fq "$expected_run_name" "$workflow_path"; then
    echo "$workflow does not provide the unique run title used by the release dispatcher." >&2
    exit 1
  fi
  if ! rg -q 'ensure-release-tag-on-main|Require release tag on main' "$workflow_path"; then
    echo "$workflow does not verify that its release tag belongs to main." >&2
    exit 1
  fi
  if ! rg -q 'RELEASE_TAG_MAIN_MODE=latest' "$workflow_path"; then
    echo "$workflow does not reject a release tag older than the latest stable tag on main." >&2
    exit 1
  fi
  if ! awk '
    /^on:/ { in_on = 1; next }
    in_on && /^[^[:space:]]/ { exit }
    in_on && /^  workflow_dispatch:/ { has_dispatch = 1 }
    END { exit !has_dispatch }
  ' "$workflow_path"; then
    echo "$workflow does not expose a workflow_dispatch trigger." >&2
    exit 1
  fi
  if awk '
    /^on:/ { in_on = 1; next }
    in_on && /^[^[:space:]]/ { exit }
    in_on && /^  (push|release):/ { has_automatic_trigger = 1 }
    END { exit !has_automatic_trigger }
  ' "$workflow_path"; then
    echo "$workflow has a tag-push or release trigger that would duplicate dispatcher publishing." >&2
    exit 1
  fi
done <<< "$expected_channels"

release_workflow="$root/.github/workflows/release.yml"
docker_workflow="$root/.github/workflows/docker.yml"
if ! rg -Fq 'SKIP_RELEASE_CHANNELS: publish-snap.yml' "$release_workflow" || \
   ! rg -q 'SKIP_RELEASE_CHANNELS' "$dispatcher" || \
   ! rg -q 'Skipping deferred release channel' "$dispatcher"; then
  echo "Snap must remain explicitly deferred from tag-triggered publisher dispatch." >&2
  exit 1
fi
if ! rg -q 'dispatch-release-channels\.sh' "$release_workflow"; then
  echo "The GitHub Release workflow does not invoke the channel dispatcher." >&2
  exit 1
fi
if ! rg -Fq -- '--ref main' "$dispatcher" || \
   ! rg -q 'gh run list' "$dispatcher" || \
   ! rg -q 'repos/\$\{repo\}/actions/runs/\$\{run_id\}' "$dispatcher"; then
  echo "Release dispatch must use main workflow definitions and wait for every publisher run." >&2
  exit 1
fi
if ! rg -Fq 'type=raw,value=latest' "$docker_workflow" || \
   ! rg -Fq 'type=raw,value=${{ env.RELEASE_TAG }}' "$docker_workflow"; then
  echo "Docker must publish both the latest alias and the exact release tag." >&2
  exit 1
fi
if ! rg -Fq 'ghcr.io/${{ github.repository }}' "$docker_workflow" || \
   ! rg -Fq "inputs.publish_dockerhub && 'snapetech/iptvtunerr'" "$docker_workflow" || \
   ! rg -Fq "inputs.publish_dockerhub && 'keefshape/iptvtunerr'" "$docker_workflow" || \
   ! rg -Fq 'default: false' "$docker_workflow" || \
   ! rg -Fq 'publish_dockerhub=false' "$dispatcher"; then
  echo "GHCR must always receive latest and the release tag; Docker Hub must remain opt-in and disabled by release dispatch." >&2
  exit 1
fi
if ! rg -Fq 'build-args: VERSION=${{ env.RELEASE_TAG }}' "$docker_workflow"; then
  echo "The Docker binary version must come from the dispatched release tag." >&2
  exit 1
fi
if ! rg -q 'git show origin/main:Dockerfile > Dockerfile' "$docker_workflow" || \
   ! rg -Fq 'network=host' "$docker_workflow" || \
   ! rg -Fxq 'FROM debian:bookworm-slim' "$root/Dockerfile" || \
   ! rg -Fq 'apt-get -o Acquire::Retries=3 update' "$root/Dockerfile" || \
   ! rg -q 'Debian package install failed.*retrying after backoff' "$root/Dockerfile"; then
  echo "Docker must use the current Debian recipe, runner DNS access, and retry transient package mirror failures." >&2
  exit 1
fi
ppa_workflow="$root/.github/workflows/release-ppa.yml"
if ! rg -Fq 'consecutive_missing_sources=0' "$ppa_workflow" || \
   ! rg -Fq 'consecutive_missing_sources=0' <(sed -n '/source_link=/,/if \[\[ "\$source_status"/p' "$ppa_workflow") || \
   ! rg -Fq "ws.op=getBuilds" "$ppa_workflow" || \
   ! rg -Fq "ws.op=getPublishedBinaries" "$ppa_workflow" || \
   ! rg -Fq 'after 70 checks' "$ppa_workflow"; then
  echo "PPA polling must tolerate intermittent source records and confirm the exact built amd64 binary." >&2
  exit 1
fi
snap_manifest="$root/snap/snapcraft.yaml"
if ! rg -Fxq 'base: core24' "$snap_manifest" || \
   ! rg -Fxq 'platforms:' "$snap_manifest" || \
   ! rg -Fxq '    build-for: [amd64]' "$snap_manifest"; then
  echo "The Core24 Snap manifest must declare its amd64 platform using the platforms field." >&2
  exit 1
fi
copr_workflow="$root/.github/workflows/release-copr.yml"
if ! rg -q 'libkrb5-dev' "$copr_workflow" || \
   ! rg -q 'krb5-devel' "$copr_workflow" || \
   ! rg -q 'krb5-dev' "$copr_workflow"; then
  echo "COPR tooling must install Kerberos development files for requests-gssapi on supported runner distributions." >&2
  exit 1
fi
if ! rg -q 'build-release-assets\.sh' "$release_workflow" || \
   ! rg -q 'build-linux-package-assets\.sh' "$release_workflow"; then
  echo "The GitHub Release workflow must build versioned binaries and Linux packages." >&2
  exit 1
fi

tmp_repo="$(mktemp -d)"
trap 'rm -rf "$tmp_repo"' EXIT
git -C "$tmp_repo" init --quiet --initial-branch=main
git -C "$tmp_repo" config user.name "Release parity check"
git -C "$tmp_repo" config user.email "release-parity@example.invalid"
git -C "$tmp_repo" config commit.gpgsign false
git -C "$tmp_repo" config core.hooksPath /dev/null
printf 'v0.1.85\n' > "$tmp_repo/version.txt"
git -C "$tmp_repo" add version.txt
git -C "$tmp_repo" commit --quiet -m "add v0.1.85"
git -C "$tmp_repo" tag v0.1.85
printf 'v0.1.86\n' > "$tmp_repo/version.txt"
git -C "$tmp_repo" add version.txt
git -C "$tmp_repo" commit --quiet -m "add v0.1.86"
git -C "$tmp_repo" tag v0.1.86
RELEASE_REPO_DIR="$tmp_repo" MAIN_REF=main RELEASE_TAG_MAIN_MODE=latest \
  bash "$root/scripts/ensure-release-tag-on-main.sh" v0.1.86 >/dev/null
if RELEASE_REPO_DIR="$tmp_repo" MAIN_REF=main RELEASE_TAG_MAIN_MODE=latest \
  bash "$root/scripts/ensure-release-tag-on-main.sh" v0.1.85 >/dev/null 2>&1; then
  echo "The latest-release guard accepted stale tag v0.1.85." >&2
  exit 1
fi

echo "Release channel wiring is complete: enabled publishers dispatch the latest stable tag once, GHCR receives latest plus that version tag, and paused channels stay disabled."
