#!/usr/bin/env bash
# Tag builds only: creates the GitHub release once (notes generated from merged PRs and commits) and uploads
# every file in dist/. Re-running replaces the assets instead of failing.
source "$(dirname "$0")/lib.sh"
[[ -n "$TAG" ]] || { echo "not a tag build, nothing to publish"; exit 0; }
: "${GH_TOKEN:?GH_TOKEN is required (Jenkins credential github-token)}"

log "GitHub release $TAG"
flags=(); [[ "$VERSION" == *-* ]] && flags=(--prerelease)
if ! gh release view "$TAG" >/dev/null 2>&1; then
	gh release create "$TAG" --verify-tag --title "$APP $VERSION" --generate-notes "${flags[@]}"
fi
gh release upload "$TAG" "$DIST"/* --clobber
gh release view "$TAG" --json url -q .url
