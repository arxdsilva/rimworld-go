# ARCH-002: in-package preparation for the ebiten decomposition

## Why
Before any file leaves `internal/adapters/inbound/ebiten`, every feature must reach the shell through a seam that survives the move, so that moving a feature is mechanical (`.cursor/SOLID_GO.md` §10). Two waves, no file moves: the lead built the seam (2.0), five parallel agents repointed the feature files to it (2.1).

## What exists now
- **`ports/`** — the role interfaces (`Hinter`, `Sizer`, `Commands`, `World`, `View`, `PawnSel`, `Overlays`, `Speed`, `Settings`, `Panels`, `Dispatcher`, each ≤ 8 methods) and the closed `Intent` set (`OpenArchitect`, `CloseArchitect`, `PresetBuildTool`, `ShowWindow`, `CloseWindows`, `SelectPawn`, `TogglePawnInSet`, `ClearSelection`, `CenterOnPawn`).
- **`shell.go`** — `App.shell`: one small value per role (`shellHint`, `shellSize`, … each ≤ 8 methods, nil-safe on a menu screen), the screen table (`screenTable()`, replaces the `screenUIs` global and nine `init()`s) and the click routes (`clickRoutes()`, an ordered list with `applyClick`'s former switch semantics; `options.back` and the non-window `tab.*` ids fall through as before). Feature code calls `a.shell.Enqueue(c)`, `a.shell.World()`, `a.shell.Hint(s)`, `a.shell.Do(ports.CloseArchitect{})` …; after a move the same calls read `t.sh.X` on the package's own `Shell`.
- **`ids/`** — the click ids and prefixes more than one feature names.
- **`render/`** — `render.View` (this frame's context: size, tile, zoom, world, pawns, tick, present, alpha, flags, selection) filled at the top of `Renderer.Draw` from one read of the renderer's state; `r.view.W`/`Pawns` replace the `drawW`/`drawPawns` fields; `render.Trace` (`Layer`, `AnimalMark`) replaces the `layerTrace` field and the `animalMarkTrace` package var.
- `AppConfig.Background` / `AppConfig.WindowHooks` replace the `appBackground` / `appWindow` package vars the tests swapped (tests set `a.cfg.X` on their App). `hud()` memoises its layout on its inputs (`hudStates.lay`): same inputs, same layout, one computation per frame.
- Wave 2.1 repointed ≈ 300 sites across 68 files: `Enqueue` ≈ 120, `World` ≈ 80, `Pawns` ≈ 30, `Hint` ≈ 28, plus Tick/Date/TilePx/TileAt/WorldToScreen/Select/Selected/Overlays/Speed and ≈ 60 id literals → `ids.*`. Left in place on purpose, for the move waves: `a.renderer.game.<reader>` (129 uses; the sim reader groups are defined at move time, SOLID_GO.md §6), `a.renderer.centerOn(wx, wy, sw, sh)` (33; world-point form), `art`, `cam`, draw helpers (`drawSprite`, `drawRangeRing`, `drawPlacementGhost`), the overlay "forced" flags, trade's auto-pause block, settings' pushes into the renderer, `syncPawns`.

## Deviations from the plan
- The layout is not threaded as a parameter in this phase: `hud()` memoises instead, which is behaviour-identical today; each feature package receives the layout the root computes right before calling it when it moves (ARCH-005).
- The ten mixed App/Renderer files were not split in advance: the first mover splits its half out at move time (`scratchpad` tool `splitrecv` does it in one command).
- The facade is not methods on `App` (that would have grown it by 45): it is per-role values composed in `App.shell`, which is the ISP shape anyway.
- The root's `files`/`lines` ceilings were raised once for the seam itself (213 files, 64,178 → 64,602 lines, noted in `internal/arch/solid_budget.txt`); `App` is flat at 950 methods / 111 fields, `Renderer` 398 / 72. Nine `init` lines and four `global` lines are paid.

## Findings
- `gopls rename` (v0.17.1) works on this toolchain: ARCH-003's identifier exports are type-aware renames, not sed.
- Benchmark baseline `.cursor/handoffs/ARCH-002-bench-before.txt` ran while the `-race` suites were also running, so its timings are noisy; re-record it on a quiet machine before the first renderer-layer move (ARCH-004 step 0).
- `TestUIDrawsThroughCanvas` keys world-position conversions on the selector name; it now also recognises the shell's `WorldToScreen*`.

## Next
ARCH-003 (shared leaves): `gfx` first (lead), then `camera`, `art`, `inputframe`+`hudlayout`+`simtest`, `globe`, `uikit` in parallel; `render` gains the camera and the art catalog.
