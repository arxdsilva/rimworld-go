# internal/compose/game

## Purpose
The client's composition root, moved out of `cmd/game` (2026-11, GAR; SOLID_GO.md §8a, HEXAGONAL_ARCHITECTURE.md §28). `Run` builds every concrete adapter the Ebitengine App needs and hands them over as its ports; `cmd/game/main.go` only parses the command line and calls `Run`.

## Owns
- `Run(opts, version, Display)` (`run.go`): log file, profiles, the fps probe, the capture, seed and biome, then the `AppConfig` and the window.
- `Report(opts, version, out)` (`selfcheck.go`): `-version` and `-selfcheck` answer before any log, config dir or window; `-selfcheck` lists every asset lookup (translations, art, splash, sounds, window icon) and the settings folder and returns 1 when one is missing (REL-003). `ConsoleOut` (`console_{windows,other}.go`) gives a `-H=windowsgui` build a stdout: a redirected one is kept, else the parent terminal's console.
- `Display` (`display.go`, 6 methods): the window and monitor, filled by `ebiten/display.Screen`; the helpers take the narrow `scaler` and `sizer`.
- Content (`content.go`, PLG-*): the plugin catalog built once at start from `modloader.DefaultRoots()` and `Settings.Plugins.Enabled` (`runContent`): its books go to new colonies, set pieces, loaded saves and the wizard; `contentid.Of` stamps saves; `view()` feeds the Plugins screens. Captures load no plugin, except the `plugins` captures (shipped `mods/` plus an in-memory fixture and in-memory settings).
- Hooks: settings and the dev graphics overrides (`settings.go`), saves, colonist card presets and custom scenarios (`files.go`), sound and hardware (`sound.go`), usage data (`usage.go`), the update check (`update.go`), the wizard's colony builder and the dev scenes (`colony.go`).

## Depends on
Every adapter the client wires, `internal/application/game` and `devscene`, `internal/platform/{logfile,profiling,config,sysinfo,openfolder}`.

## Must not depend on
Any third-party module: Ebitengine calls go through `Display` (`internal/adapters/inbound/ebiten/display`). Nothing but `cmd/` imports this package (`TestOnlyCmdImportsCompose`).

## Contracts
- Captures read and write none of the player's files (settings, card presets, scenarios) and get no hardware reader (`TestCapturesUseTheDefaultSettings`).
- A dev graphics flag applies for the run only and is never saved (`TestDevGraphicsOverridesAreNeverSaved`).
- No usage data or update check for `dev` builds and developer runs (`TestUsageEndpoint`, `TestUpdateCheckIsOffForDevBuildsAndDevRuns`); the install ID is made once (`TestInstallIDIsMadeOnceAndKept`).
- A nil concrete adapter (`*Engine`, `*Tracker`, `*Checker`) never goes into a port: the port stays nil.

## Notes
`Run` ends the process with `log.Fatal` on a bad `-biome`, `-graphics` or `-render-scale`, a window error or a failed capture, as `cmd/game` did.
