#!/usr/bin/env bash
# Checks the toolchain on the agent and installs dependencies.
source "$(dirname "$0")/lib.sh"

log "Toolchain"
for t in git go node pnpm wails3 makensis gh lipo hdiutil zip shasum perl; do
	command -v "$t" >/dev/null || { echo "missing tool: $t (see .jenkins/README.md)" >&2; exit 1; }
done
command -v docker >/dev/null || echo "warning: docker not found; the Windows installer needs it while Homebrew makensis is broken for Unicode scripts"
go version && node --version && pnpm --version && wails3 version && makensis -VERSION && gh --version | head -1
want="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
[[ "$(wails3 version 2>&1)" == *"${want#v}"* ]] || echo "warning: wails3 CLI is not $want; run: go install github.com/wailsapp/wails/v3/cmd/wails3@$want"

log "Dependencies"
go mod download
(cd frontend && pnpm install --frozen-lockfile)
