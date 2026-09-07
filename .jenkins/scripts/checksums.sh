#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "Checksums"
cd "$DIST" && rm -f SHA256SUMS.txt && shasum -a 256 -- * | tee SHA256SUMS.txt
