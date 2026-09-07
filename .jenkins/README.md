# Release pipeline

Jenkins builds every branch and pull request, and publishes a GitHub release for every tag that looks like `v1.2.3` or `v1.2.3-beta.1`. The `Jenkinsfile` at the repository root only sequences the steps; each step is a bash script in `scripts/` that you can also run by hand.

| Step | Script | What it does |
| --- | --- | --- |
| Setup | `setup.sh` | Checks the toolchain, installs Go modules and frontend packages |
| Verify | `verify.sh` | gofmt, `go vet` (macOS and Windows), `go test`, prettier, eslint, svelte-check, frontend build |
| Version | `version.sh` | Tag builds only. Writes the tag version into the Wails config and the frontend, regenerates the platform build assets |
| Build macOS | `build-macos.sh arm64` and `amd64` | One DMG per architecture: `Soteria-<version>-macOS-apple-silicon.dmg`, `Soteria-<version>-macOS-intel.dmg` |
| Build Windows | `build-windows.sh` | `Soteria-<version>-Windows-x64-Setup.exe` (NSIS, bundles the WebView2 bootstrapper) and `Soteria-<version>-Windows-x64.zip` (portable) |
| Checksums | `checksums.sh` | `SHA256SUMS.txt` |
| Publish | `release.sh` | Tag builds only. Creates the GitHub release with generated notes, marks `-beta`/`-rc` tags as pre-release, uploads `dist/*` |

Everything lands in `dist/`, which Jenkins also archives on the build.

## Agent: Mac mini (Apple silicon)

Label the node `mac-m4`. Install once:

```bash
xcode-select --install
brew install go nsis gh fnm pnpm
fnm install 26 && fnm default 26      # scripts pick the version from .node-version
go install github.com/wailsapp/wails/v3/cmd/wails3@latest   # keep in step with go.mod
```

Make sure the Jenkins agent process sees `~/go/bin` and `/opt/homebrew/bin`; `scripts/lib.sh` adds both to `PATH` and loads fnm. pnpm from Homebrew switches itself to the version pinned in `frontend/package.json` (`packageManager`), so no corepack is needed. Go modules, the Go build cache and the pnpm store live in `~/.cache/soteria-ci` so tag builds stay fast.

Windows is cross-compiled on the Mac with CGO disabled (the WinFsp driver is loaded at runtime). No Windows machine is needed; the build is x64 only.

The installer is compiled by `makensis`. Homebrew's makensis 3.12 bottle for Apple silicon currently aborts (`std::bad_alloc`) on every Unicode NSIS script, so `build-windows.sh` probes it and, when it fails, runs Debian's makensis in a small Docker image (`soteria-nsis`, built once from `debian:bookworm-slim`). Install Docker Desktop or `brew install colima docker && colima start` on the agent until Homebrew ships a working bottle.

## Credentials

| Jenkins credential id | Kind | Used by |
| --- | --- | --- |
| `github-token` | Secret text: a fine-grained personal access token with **Contents: read and write** on this repository | `release.sh` via `GH_TOKEN` |

## Job

Create a **Multibranch Pipeline**:

1. Branch source: GitHub, this repository, with the GitHub app or token credential for checkout.
2. Behaviours: *Discover branches*, *Discover pull requests from origin*, and **Discover tags**.
3. Install the *Basic Branch Build Strategies* plugin and add the **Tags** build strategy (otherwise Jenkins indexes tags but does not build them). Set the "ignore tags older than" window to a few days.
4. Scan repository triggers: a GitHub webhook to `<jenkins>/github-webhook/` gives instant builds; a periodic scan is the fallback.

## Releasing

```bash
git tag v1.0.0
git push origin v1.0.0
```

Jenkins picks up the tag, runs Verify, builds the three packages, and publishes them. Re-running the same tag build replaces the assets instead of failing. The repository never stores the version: `build/config.yml` and `frontend/package.json` keep `0.0.1` and are stamped inside the workspace only.

## Running a step locally

```bash
TAG_NAME=v1.0.0 .jenkins/scripts/version.sh   # optional, otherwise the version is 0.0.0-<sha>
.jenkins/scripts/build-macos.sh arm64
.jenkins/scripts/build-windows.sh
.jenkins/scripts/checksums.sh
```

`release.sh` needs `GH_TOKEN` (or a logged-in `gh`) and an existing tag.

## Not included yet

Code signing and notarization. Unsigned builds work but macOS asks the user to right-click and choose Open the first time, and Windows shows a SmartScreen warning. Both can be added as extra steps once an Apple Developer ID and an Authenticode certificate exist.
