#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="${RELEASE_REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$ROOT_DIR"

TAG="${1:-${GITHUB_REF_NAME:-}}"
MAIN_REF="${MAIN_REF:-origin/main}"
MODE="${RELEASE_TAG_MAIN_MODE:-exact}"

err() {
  echo "[ensure-release-tag-on-main] ERROR: $*" >&2
  exit 1
}

[[ -n "$TAG" ]] || err "usage: $0 <tag>"
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null || err "tag '$TAG' not found"

if git remote get-url origin >/dev/null 2>&1; then
  git fetch --quiet origin main --tags
fi

git rev-parse -q --verify "$MAIN_REF" >/dev/null || err "main ref '$MAIN_REF' not found"

TAG_COMMIT="$(git rev-list -n 1 "$TAG")"
MAIN_COMMIT="$(git rev-parse "$MAIN_REF")"

case "$MODE" in
  exact)
    if [[ "$TAG_COMMIT" != "$MAIN_COMMIT" ]]; then
      err "release tag '$TAG' points to $TAG_COMMIT, but $MAIN_REF is $MAIN_COMMIT; release tags must point at current main"
    fi
    echo "[ensure-release-tag-on-main] $TAG is on current main ($MAIN_COMMIT)"
    ;;
  ancestor)
    if ! git merge-base --is-ancestor "$TAG_COMMIT" "$MAIN_REF"; then
      err "release tag '$TAG' points to $TAG_COMMIT, which is not in $MAIN_REF history"
    fi
    echo "[ensure-release-tag-on-main] $TAG is in $MAIN_REF history ($TAG_COMMIT)"
    ;;
  latest)
    if ! git merge-base --is-ancestor "$TAG_COMMIT" "$MAIN_REF"; then
      err "release tag '$TAG' points to $TAG_COMMIT, which is not in $MAIN_REF history"
    fi
    if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      err "release tag '$TAG' is not a stable vX.Y.Z release tag"
    fi
    LATEST_STABLE_TAG="$(git tag --merged "$MAIN_REF" --list 'v[0-9]*' --sort=-version:refname | awk '/^v[0-9]+\.[0-9]+\.[0-9]+$/ { print; exit }')"
    [[ -n "$LATEST_STABLE_TAG" ]] || err "no stable release tags are reachable from $MAIN_REF"
    if [[ "$TAG" != "$LATEST_STABLE_TAG" ]]; then
      err "refusing to publish stale release '$TAG'; latest stable release on $MAIN_REF is '$LATEST_STABLE_TAG'"
    fi
    echo "[ensure-release-tag-on-main] $TAG is the latest stable release on $MAIN_REF"
    ;;
  *)
    err "unsupported RELEASE_TAG_MAIN_MODE '$MODE' (expected exact, ancestor, or latest)"
    ;;
esac
