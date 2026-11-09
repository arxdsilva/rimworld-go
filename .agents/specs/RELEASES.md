# Releases

Pushing a `v*` tag (`git tag v0.1.0 && git push origin v0.1.0`) runs `.github/workflows/release.yml`: test, then one build job per OS, then the release. A manual run (`workflow_dispatch`) builds and smoke-tests everything under the version `v0.0.0-dev.<run>` and publishes nothing. A tag with a `-` (e.g. `v0.2.0-rc1`) becomes a prerelease.

## What each OS ships

Installers are the main downloads; the portable archives (`scripts/package_release.sh`) stay as the second choice.

| OS | Runner | Installer | Portable archive |
| --- | --- | --- | --- |
| macOS | `macos-latest`, cgo arm64 + amd64 joined with `lipo`, `MACOSX_DEPLOYMENT_TARGET=13.0` (Go 1.27's floor) | `rimworld-go-<tag>-macos.dmg` (`scripts/package_macos.sh`): `rimworld-go.app` (`Contents/MacOS/rimworld-go`, `Contents/Resources/{assets,rimworld-go.icns}`, `Info.plist` from `packaging/macos/Info.plist.in`) plus an `/Applications` link, ULFO | `-macos-universal.tar.gz` |
| Windows | `windows-latest`, `CGO_ENABLED=0`, `-H=windowsgui`; icon and version resource from `go-winres` (pinned, `cmd/game/rsrc_windows_amd64.syso`, gitignored, removed after the build) | `rimworld-go-<tag>-windows-setup.exe` (`scripts/package_windows.ps1`, Inno Setup `packaging/windows/rimworld-go.iss`): per-user by default (`%LOCALAPPDATA%\Programs\rimworld-go`, no admin), Start menu entry, optional desktop icon, uninstaller; the AppId GUID never changes | `-windows-amd64.zip` |
| Linux | `ubuntu-22.04` (its glibc is the floor players need), cgo against X11, GL and ALSA | `rimworld-go-<tag>-x86_64.AppImage` (appimagetool and the type2 runtime pinned by URL and sha256), `-linux-amd64.deb` and `-linux-x86_64.rpm` (nfpm pinned, `packaging/linux/nfpm.yaml`: `/opt/rimworld-go`, `/usr/bin/rimworld-go` link, desktop file, hicolor icons) | `-linux-amd64.tar.gz` |

Every installer is smoke-tested on its own OS before upload: the DMG is mounted and the app copied out, the setup installs silently for the user and uninstalls, the `.deb` installs with apt and is removed, the AppImage runs with `--appimage-extract-and-run`, and the `.rpm`'s file list is checked. Each runs `rimworld-go -version` (must print the tag) and `rimworld-go -selfcheck` (every asset root must resolve inside the installed copy; `packaging/smoke.sh`) from `/` or `C:\`, so the working directory can't lend it the repository's files. Locally, `make dist-macos` builds and smoke-tests the DMG; `make dist-linux-pkgs` only checks the `.deb`/`.rpm`/AppDir layout (its cross-built binary has no cgo and does not run as the game).

## Runtime files

`packaging/assets.txt` is the one list of what every package copies. `internal/arch` `TestPackagedAssetsCoverRuntime` fails when code looks up an asset through `internal/platform/assetroot` with a path the list doesn't ship, or with a path that isn't a constant. `assetroot` searches the working directory, then a macOS bundle's `Contents/Resources`, then the executable's directory (symlinks resolved). The icons come from `packaging/icons` (`make app-icon`); CI fails without them, and `ALLOW_NO_ICON=1` skips them only for local tests.

## Signing

Unsigned unless the secrets exist; each step switches on by itself. macOS: `APPLE_CERT_P12_BASE64`, `APPLE_CERT_PASSWORD`, `APPLE_TEAM_ID`, `APPLE_ID`, `APPLE_APP_PASSWORD` (Developer ID with hardened runtime, then `notarytool` and `stapler`); without them the app is ad-hoc signed. Windows: `WINDOWS_CERT_PFX_BASE64`, `WINDOWS_CERT_PASSWORD` (`signtool` on the game and the setup). The release notes tell players how to open unsigned builds.

## Publishing

GoReleaser (`.goreleaser.yaml`, `builds: skip`) uploads the installers first and the archives second (`release.extra_files`), `checksums.txt` over all of them, a per-OS download table as the header, a changelog grouped by commit type from the previous tag, and the commit bodies (`scripts/release_details.sh`), so write commit messages for the people reading the release. The notes start at the last tag that has a published release (`scripts/previous_release.sh`), so a tag whose workflow failed loses nothing. The menu shows the version from `-ldflags "-X rimworld-go/internal/platform/buildinfo.Version=<tag>"`; `rimworld-go -version` prints it. Both workflows test through `scripts/ci_test.sh`, which publishes what failed and the slowest test binaries as annotations, readable without a login (job logs are not): `https://api.github.com/repos/arxdsilva/rimworld-go/check-runs/<job id>/annotations`.
