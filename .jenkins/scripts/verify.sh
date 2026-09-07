#!/usr/bin/env bash
# Lint, type-check and test both halves; the same commands developers run locally.
source "$(dirname "$0")/lib.sh"

log "Go"
unformatted="$(gofmt -l ./*.go ./internal)"
[[ -z "$unformatted" ]] || { printf 'gofmt: run gofmt -w on:\n%s\n' "$unformatted" >&2; exit 1; }
go vet . ./internal/...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet . ./internal/...
go test . ./internal/...

log "Frontend"
cd frontend
pnpm exec prettier --check src
pnpm exec eslint src
pnpm exec svelte-check --tsconfig ./tsconfig.json
pnpm build
