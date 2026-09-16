# ebiten inbound adapter

Interactive Ebitengine client: screen routing, programmatic UI chrome,
and the colony map viewport. Presentation-only. It never mutates
authoritative simulation stores.

## Purpose

Host the player-facing session: boot, menus, new-game stubs, pause, and
the colony viewport with HUD chrome. Screens consume read models
(`Snapshot()`, `World()`) and emit session intents plus typed gameplay
commands via `Enqueue`. Domain code does not import this package.

## Owned concepts

- `App` — `ebiten.Game` shell, screen stack, overlay (pause), HUD
- widgets: panel, label, button (hover/disabled)
- UI chrome tokens (`assets/palettes/ui.yaml`, mirrored in Go)
- colony HUD: Food/Wood/time strip, colonist bar, Architect, Work, inspector
- main-menu colonist splash (original PNG, left of the action rail)
- `Renderer` — colony camera, speed tiers, selection, world draw, `Enqueue`
- vendor sprite atlas load
- presentation animation / tile lerp clocks

## Allowed dependencies

- `internal/application/game` via the consumer-owned `Simulation`
  interface (`Step`, `Snapshot`, `World`, `Enqueue`, `Width`, `Height`,
  `Tick`, `Date`, `Activity`)
- `internal/domain` types that appear on that read model (`clock`,
  `entity`, `schedule`, `command`)
- Ebitengine, image/png, OS path lookup for the vendor pack and menu splash

## Forbidden dependencies

- concrete `*game.Game` pointers except at `cmd/game` (composition root)
  and the compile-time `var _ Simulation = (*game.Game)(nil)` assertion
- mutating domain stores
- wall-clock or global RNG in anything that could leak into sim
- proprietary RimWorld art, text, or layouts

## Contracts

- Simulation is created only by `SimFactory` when the player presses
  Start. Menus must run with `renderer == nil`.
- `cmd/game` constructs `game.NewColony` in the factory (Ada/Bo/Cora kit).
- Return to main menu drops the renderer so the menu stays headless.
- Pause overlay skips `Renderer.advance` and map input.
- HUD, inspector, Architect, and Work consume pointer hits before map selection.
- Escape on colony: Architect, then Work, then inspector selection, then pause.
- Disabled buttons do not navigate; they expose a reason string.
- `Layout` follows the window size; camera/zoom stay presentation-only.
- Architect tools emit `DesignateWall/Bed/Stockpile/Harvest/Cancel`.
- Work Grow/Bld/Haul cells emit `SetWorkEnabled`. Right-click emits `MovePawn`.

## Invariants

- Screen IDs and widget bounds never enter domain or save state.
- Sprite animation uses a presentation frame counter, not sim ticks, to
  pick frames. Tile lerp may use sim ticks vs `moveTick`.
- UI hit-testing consumes the pointer before the map.
- Menu chrome colors must match `assets/palettes/ui.yaml`.
- Main-menu splash is presentation-only; a missing PNG still shows the
  backdrop and right-rail buttons.

## Hot paths

Colony `Draw` (terrain, world overlays, pawns, daylight). Keep off-screen culling.
Menu `Draw` is cheap vector fills plus debug text.

## Determinism

This adapter is not in the authoritative tick. `Step(n)` still runs on
the simulation the factory created; speed tiers only change how often
`App` calls `advance`.

## Tests

`nav_test.go` and `app_test.go` cover routing, disabled clicks, Start
creating a sim, and pause blocking `Step`. `hud_test.go` covers colonist
bar select, Work overlay, Architect palette, Escape order, Work cell
toggles, inspector names, and HUD hit-testing. `splash_test.go`
covers the right-rail menu and PNG decode. `widgets_test.go`
checks UI palette hexes against `assets/palettes/ui.yaml`. They must not
open a window. Existing renderer unit tests stay headless.

## Benchmarks

None yet. Add them when colony draw or UI layout shows up in profiles.
