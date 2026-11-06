# DEPS-001: libraries at the edge, and cleaning up what we wrote by hand

Asked 2026-11-03: `usagehttp.NewID` built a UUID by hand. The user asked why, asked for a map of everything similar, and asked which rules had to change.

## What changed

- **Rules:** `GO_BEST_PRACTICES` §86 is now standard library ready to call, then a well-known library, then our own code with the reason written down. Libraries are allowed in `cmd`, adapters and platform. `internal/arch` forbids them in domain, application and `pkg`, and fails when a `go.mod` module is missing from `THIRD_PARTY_NOTICES.md`.
- **Saves:** the spec now describes what the game does (gob + gzip, one migration per version). `TestSaveSchema` lists all 2,790 saved fields and `TestSaveV12Fixture` pins a v12 reference save. Refresh with `RIMWORLD_UPDATE_SAVE_SCHEMA=1` and `RIMWORLD_UPDATE_SAVE_V12=1`.
- **Atomic writes:** `internal/platform/atomicfile`, used by saves, views, settings, scenarios, the grown-colony cache and the `sfxgen` ledger.
- **YAML:** one reader, `pkg/flatyaml`, for the game and the tools.
- **Duplication:** `pkg/mathx` (`Abs`, `Clamp`) and `randx.SplitMix64`/`Mix64` replace 38 `abs`, 4 clamps and about 20 mixers.
- **Stubs:** 10 empty packages deleted. The other 23 doc-only packages stay, by user decision.
- **CI:** `make vuln` and `make lint` now run on every push. Go went to 1.26.10 for 12 reachable standard-library vulnerabilities.
- **Notices:** `make licenses` regenerates `third_party/go-modules/`. The release embedded 12 modules with no notice before this.

## New dependencies (§86)

All three are pure Go on the Windows no-cgo build, have no effect on determinism because they stay in adapters, and were not archived when checked on 2026-11-03.

| Module | Capability | Last repo activity | License | Pulls in | Notes |
| --- | --- | --- | --- | --- | --- |
| `github.com/google/uuid` v1.6.0 | Install IDs | 2024-11 (stable, little change) | BSD-3-Clause | nothing | IDs come from `crypto/rand`. `ValidID` still refuses `urn:uuid:` and braces. |
| `golang.org/x/mod` v0.42.0 | Comparing versions (`semver`) | 2026-10 | BSD-3-Clause | `x/tools` in the module graph, not linked | The tag only becomes text; the download URL is a constant. |
| `github.com/cli/browser` v1.3.0 | Opening the download page | 2026-10 | BSD-2-Clause | `x/sys` | Starts a local program (`ShellExecute` on Windows, `xdg-open` and others on Linux). `OpenURL` runs in a goroutine, so it never blocks the game loop, and it only ever gets the constant `pageURL`. |

## Decisions and measurements

- **No YAML library.** The rule was set before measuring: at most +20% and +100 ms on the boot index. `go.yaml.in/yaml/v3` read all 7,236 files alike but added 115 ms (79%). Numbers are in `pkg/flatyaml/AGENTS.md`.
- **The parser is in `pkg/flatyaml`, not `art` as the plan said.** `art` imports ebiten, and the tools deliberately don't.
- **Parser proof:** a temporary dump over every real file was byte-identical before and after for `art`, `mainmenu`, `atlasgen` and `normalgen`. `make atlas` changed nothing. Reading the sprite files went from 8.6 ms to 4.6 ms.
- **Own atomic writer:** `renameio` exports nothing on Windows.
- **Merged atomic-write errors:** per-step messages became one per caller, and views, settings and scenarios now get an fsync. Settings save only on discrete clicks, so this causes no hitch.
- **Duplication proof:** 59 helpers and 7 inline forms were compared with the shared function over 200k inputs. No golden was re-pinned. `cmd/benchmark` stayed within noise, between -7% and +7% with no direction.
- **Clamp:** the 3 forms differ only when lo > hi. The two that differ have constant, ordered bounds at every call; the others are the same expression.
- **`rendererMix` stays a separate copy** in `skills_test.go`, because that test pins legacy looks independently of `randx`.
- **Left in place:** int32 `abs`/`clamp` variants (no shared function), `vegetation.go`'s generic, and inline `if x < 0` forms.
- **Lint:** 109 findings fixed with no `//lint:ignore`. `journey_world_test` meant a double click but short-circuited to one; it now clicks twice and still passes. The `Kind` limit check in `ax_contract_test` was vacuous and now tests wrap-around.

## Known problems, not fixed

- A map saved straight from `newMapState`, before its first room rebuild, loads with a different state hash. Whether `EnterMap` can reach that window was not checked.
- `SaveInfo.Day` says "days since landing" but is `clock / TicksPerDay`. A new colony reads about day 53.
- `RIMWORLD_UPDATE_SAVE_FIXTURE` rewrites `ng1_v2.sav` at the current version, which should break `TestNG1V2Fixture` (read in the code, not run).
- `TestSaveSchema` can't check that `SaveVersion` moved. A pure type rename also shows as a change, and its message explains the exception.
- `Game.invalidateOpinions` was deleted as unused, though its comment called it an entry point for host systems. Restore it with a caller if that work resumes.
- `govulncheck` still reports vulnerabilities in `x/text` v0.38.0 and `x/image` v0.43.0 that the game doesn't call. Fixed in 0.41.0 and 0.45.0, left alone.
- `make licenses` doesn't remove the folder of a dropped module.
- `api/proto/.gitkeep` is an empty Protobuf placeholder, outside the 10 stubs.

## Verification

- `go build ./...`, `make fmt`, `make vet`, `make lint`, `make vuln`, `go test -race ./...`, `go test -count=3 -run 'Determin|SameSeed|Hash|Golden' ./internal/...`, and the Windows no-cgo build of `cmd/game`.
- The guards were proven to fail: an external import in each of the three core layers, and a renamed saved field.
- `make art-check` needs Pillow, and the `python3` first on this machine's PATH doesn't have it. It passed with `/usr/local/opt/python@3.9/bin` first on the PATH.
- The plan mentioned a docs-reference test in `internal/arch`. None exists, so stub citations were checked by grep.

## Not tested

The real Download click: `go run -ldflags "-X main.version=v0.1.0-localtest" ./cmd/game`. It also sends usage data with that label.

## UI

Nothing new is player-visible, so no HUD work.
