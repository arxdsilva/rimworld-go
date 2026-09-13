# Task Handoff

Task ID: UI-001
Status: complete

## Summary

The interactive client now boots into menus. `App` is the `ebiten.Game`; the colony `Renderer` is created only when the player presses Start. New Game walks stub scenario → storyteller → world gen → landing → pawns screens. Escape closes pause, then pops the stack. Disabled actions stay visible and explain why.

## Files Changed

- `cmd/game/main.go`
- `internal/adapters/inbound/ebiten/app.go`
- `internal/adapters/inbound/ebiten/nav.go`
- `internal/adapters/inbound/ebiten/widgets.go`
- `internal/adapters/inbound/ebiten/screen_boot.go`
- `internal/adapters/inbound/ebiten/screen_menu.go`
- `internal/adapters/inbound/ebiten/screen_newgame.go`
- `internal/adapters/inbound/ebiten/screen_pause.go`
- `internal/adapters/inbound/ebiten/screen_options.go`
- `internal/adapters/inbound/ebiten/app_test.go`
- `internal/adapters/inbound/ebiten/nav_test.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/AGENTS.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

## Public Contracts Changed

- `cmd/game` launches `NewApp` (1280×720, resizable) with a `SimFactory`. It no longer constructs a `Game` at process start.
- Exported: `App`, `AppConfig`, `SimFactory`, `NewApp`, `DefaultWindowWidth`, `DefaultWindowHeight`.
- `Renderer` is no longer the `ebiten.Game` (`Layout` removed; `App` owns the window).

## Invariants / Decisions

- Menus run with `renderer == nil`. Start is the only path that calls `SimFactory`. Return to main menu drops the renderer.
- Pause overlay skips `Renderer.advance` and map input. Full-screen Options from pause pops back to colony+pause.
- Session intents (`StartGame` / `Quit` / `ReturnToMenu`) live in the adapter. There is still no application gameplay command catalog.
- Chrome is programmatic dark panels + debug text (ART §17), not sprite chrome.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/` and `go test ./...` (Ebitengine tests need a non-sandboxed run).
Benchmarks: none.

In-game window was not visually QA'd in this session. Run `go run ./cmd/game`, skip boot, New Game → Start, Escape for pause.

## Determinism Impact

None. Headless `Step(n)` is unchanged; the client only calls it after Start, and not while paused or in menus.

## Save / Mod Impact

None. Load is an empty stub. Continue is disabled (`no saves yet`).

## Art / Visual Impact

No new sprites. Menu chrome is vector fills. Vendor pack still loads during boot and is attached to the colony renderer on Start.

## Screen / UI Impact

First-session graph from spec §57: Boot, Main Menu, five new-game stubs, Colony, Pause, Options, Load, Credits. Later-wave screens (Architect, Work, inspectors, world map, plugins) are not routes.

## Known Limitations

- Wizard selections do not change the spawned map or pawn count; Start still uses `cmd/game` flags.
- No virtual lists, text fields, rich tooltips, or shortcut manager.
- Options tabs are labels only.
- No plugin/fatal boot recovery, scenario editor, or save/load persistence.
- §59 DoD is not met; wave items are `[-]`.

## Follow-ups

- Wire Start to real scenario/worldgen/pawn defs once those systems exist.
- Expand Wave UI-0 (virtual lists, focus ring, command catalog).
- Colony HUD chrome beyond debug text (Wave UI-1 inspector, colonist bar).
