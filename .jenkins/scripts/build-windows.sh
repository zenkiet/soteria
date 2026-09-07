#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"

log "Windows x64"
rm -rf bin frontend/.svelte-kit
keep_icons
wails3 task windows:build ARCH=amd64 CGO_ENABLED=0
restore_icons

log "Installer"
wails3 generate webview2bootstrapper -dir "$ROOT/build/windows/nsis" >/dev/null

printf 'Unicode true\nOutFile "%s/probe.exe"\nSection\nSectionEnd\n' "$ICON_KEEP" >"$ICON_KEEP/probe.nsi"

if command -v makensis >/dev/null && makensis -V0 "$ICON_KEEP/probe.nsi" >/dev/null 2>&1; then
	(cd build/windows/nsis && makensis -V2 -DARG_WAILS_AMD64_BINARY="$ROOT/bin/$APP.exe" project.nsi)
else
	echo "native makensis cannot build Unicode installers here, using Debian's makensis in Docker"
	docker image inspect soteria-nsis >/dev/null 2>&1 ||
		printf 'FROM debian:bookworm-slim\nRUN apt-get update -qq && apt-get install -y -qq nsis && rm -rf /var/lib/apt/lists/*\n' | docker build -q -t soteria-nsis -
	docker run --rm -v "$ROOT:/work" -w /work/build/windows/nsis soteria-nsis makensis -V2 -DARG_WAILS_AMD64_BINARY="/work/bin/$APP.exe" project.nsi
fi

mkdir -p "$DIST"
mv "bin/$APP-amd64-installer.exe" "$DIST/$APP-$VERSION-Windows-x64-Setup.exe"
(cd bin && zip -q -9 "$DIST/$APP-$VERSION-Windows-x64.zip" "$APP.exe")
ls -la "$DIST"
