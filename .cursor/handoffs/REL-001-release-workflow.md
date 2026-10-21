# REL-001 — Release builds on tags

## What changed
- `.github/workflows/release.yml`: when a `v*` tag is pushed, it runs vet plus a race test on Ubuntu (xvfb). It then builds:
  - Linux amd64 (cgo plus X11/GL/ALSA headers);
  - Windows amd64 (`CGO_ENABLED=0`, `-H=windowsgui`);
  - macOS universal (cgo arm64 plus amd64 on `macos-latest`, joined with `lipo`).
  
  Each build is packaged and published with `gh release create --generate-notes`. A tag containing `-` is published as a prerelease. `workflow_dispatch` builds the artifacts without publishing.
- `scripts/package_release.sh <bin> <name> <zip|tar.gz>`: stages the binary with the runtime assets into `dist/` and archives them:
  - `assets/generated/{world,ui,audio}`, `assets/metadata`, `assets/palettes`;
  - the menu splash;
  - README and licenses.
  
  The vendor sprite pack (a fallback only) and the review PNGs are left out.
- `internal/platform/assetroot`: looks for assets from the cwd first, then from the executable's directory, so a double-clicked release finds `assets/`. It is used by `LocatePack`, `LocateArt`, `LocateMenuSplash` and `LocateAudio`.
- `cmd/game`: `var version = "dev"`, set by `-ldflags "-X main.version=<tag>"` and shown on the main menu.
- `app.loadPack`: the missing vendor-pack warning is dropped when the original art loads.
- `AGENTS.md`: added a work-on-`main` rule (single developer, no branches, worktrees or PRs unless asked) and a Releases section. The README's "Run it" section points to Releases.

## Verified
- `actionlint` and `shellcheck` are clean.
- The universal macOS binary was packaged (81 MB tar.gz), extracted and run from an unrelated cwd:
  - the menu capture shows the diorama, the version and no asset warning;
  - the gameplay capture renders the original art.
- `go vet ./...` and `go test -race ./...` pass.
- Not verified locally: the Linux and Windows CI runs. They are covered by the first tag push.

## Follow-ups
- Signing and notarisation (macOS) and Authenticode (Windows) are not done. The release notes explain the Gatekeeper and SmartScreen workarounds.
- Any new runtime asset directory must be added to `scripts/package_release.sh`.
