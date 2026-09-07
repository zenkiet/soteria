#!/usr/bin/env bash
# Tag builds only: the tag is the single source of the version. Stamps it into the Wails config and the
# frontend, then regenerates the platform build assets (Info.plist, Windows resources, NSIS, Linux files).
source "$(dirname "$0")/lib.sh"
[[ -n "$TAG" ]] || { echo "not a tag build, keeping the repository version"; exit 0; }

log "Version $VERSION"
perl -pi -e "s/^(  version: \")[^\"]*/\${1}$VERSION/" build/config.yml
perl -pi -e "s/(\"version\": \")[^\"]*/\${1}$VERSION/" frontend/package.json
keep_icons
wails3 task common:update:build-assets
restore_icons

# Windows version resources, NSIS and CFBundleVersion accept only plain numbers.
perl -pi -e "s/(\"file_version\": \")[^\"]*/\${1}$BASE_VERSION/" build/windows/info.json
perl -pi -e "s/(define INFO_PRODUCTVERSION \")[^\"]*/\${1}$BASE_VERSION/" build/windows/nsis/wails_tools.nsh
perl -0pi -e "s/(<key>CFBundleVersion<\/key>\s*<string>)[^<]*/\${1}$BASE_VERSION/" build/darwin/Info.plist
grep -n "version" build/config.yml frontend/package.json | head -3
