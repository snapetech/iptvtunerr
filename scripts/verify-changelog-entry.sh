#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CHANGELOG="${CHANGELOG:-docs/CHANGELOG.md}"

err() {
  echo "[verify-changelog-entry] ERROR: $*" >&2
  exit 1
}

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/verify-changelog-entry.sh release <vX.Y.Z>

Rules:
  - Release tags require a populated docs/CHANGELOG.md section for that tag.
  - The release workflow separately checks that the section matches the
    validated release-note fragments included by the tag.
EOF
  exit 2
}

extract_release_section() {
  local tag="$1"
  awk -v tag="$tag" '
    $0 ~ "^## \\[" tag "\\]" { in_section = 1; next }
    in_section && /^## / { exit }
    in_section { print }
  ' "$CHANGELOG"
}

verify_release() {
  local tag="$1"
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || err "release tag must look like vX.Y.Z; got '$tag'"

  local section
  section="$(extract_release_section "$tag")"
  [[ -n "$section" ]] || err "$CHANGELOG has no section for $tag"
  grep -Eq '^### ' <<<"$section" || err "$CHANGELOG section for $tag must include at least one subsection heading"
  grep -Eq '^- .+' <<<"$section" || err "$CHANGELOG section for $tag must include at least one bullet"
  grep -Eiq '\*\(none\)\*|TBD|TODO|placeholder' <<<"$section" && err "$CHANGELOG section for $tag still looks like a placeholder"

  echo "[verify-changelog-entry] $CHANGELOG has a populated section for $tag"
}

[[ $# -eq 2 && "$1" == release ]] || usage
verify_release "$2"
