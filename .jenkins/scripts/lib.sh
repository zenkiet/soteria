#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

export PATH="$HOME/go/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
if command -v fnm >/dev/null 2>&1; then
	eval "$(fnm env --shell bash)"
	fnm use --install-if-missing --silent-if-unchanged >/dev/null # reads .node-version
fi

CACHE="${CI_CACHE:-$HOME/.cache/soteria-ci}"
export GOMODCACHE="$CACHE/gomod" GOCACHE="$CACHE/gobuild" npm_config_store_dir="$CACHE/pnpm"
export MACOSX_DEPLOYMENT_TARGET=12.0 CGO_CFLAGS=-mmacosx-version-min=12.0 CGO_LDFLAGS=-mmacosx-version-min=12.0 # same floor as the Wails build tasks

APP=Soteria
DIST="$ROOT/dist"
TAG="${TAG_NAME:-}" # Jenkins multibranch sets TAG_NAME for tag builds
if [[ -n "$TAG" ]]; then
	[[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || { echo "tag must look like v1.2.3 or v1.2.3-beta.1, got '$TAG'" >&2; exit 1; }
	VERSION="${TAG#v}"
else
	VERSION="0.0.0-$(git rev-parse --short HEAD)"
fi
BASE_VERSION="${VERSION%%-*}" # plain X.Y.Z for Windows resources, NSIS and CFBundleVersion

# Wails regenerates the icons from appicon.png with fewer sizes during builds; keep the hand-made set.
ICON_KEEP="$(mktemp -d)"
keep_icons() { cp build/darwin/icons.icns build/windows/icon.ico "$ICON_KEEP/"; }
restore_icons() { cp "$ICON_KEEP/icons.icns" build/darwin/icons.icns && cp "$ICON_KEEP/icon.ico" build/windows/icon.ico; }

log() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
