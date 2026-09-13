# ebiten inbound adapter

Interactive Ebitengine client: screen routing, programmatic UI chrome,
and the colony map viewport. Presentation-only. It never mutates
authoritative simulation stores.

## Purpose

Host the player-facing session: boot, menus, new-game stubs, pause, and
the existing colony renderer. Screens consume read models (`Snapshot()`)
and emit session intents (`StartGame`, `Quit`, `ReturnToMenu`). Domain
code does not import this package.

## Owned concepts

- `App` — `ebiten.Game` shell, screen stack, overlay (pause)
- widgets: panel, label, button (hover/disabled)
- `Renderer` — colony camera, speed tiers, selection, sprite draw
- vendor sprite atlas load
- presentation animation / tile lerp clocks

## Allowed dependencies

- `internal/application/game` via the consumer-owned `Simulation`
  interface (`Step`, `Snapshot`, `Width`, `Height`, `Tick`, `Date`,
  `Activity`)
- `internal/domain` types that appear on that read model (`clock`,
  `entity`, `schedule`)
- Ebitengine, image/png, OS path lookup for the vendor pack

## Forbidden dependencies

- concrete `*game.Game` pointers except at `cmd/game` (composition root)
- mutating domain stores
- wall-clock or global RNG in anything that could leak into sim
- proprietary RimWorld art, text, or layouts

## Contracts

- Simulation is created only by `SimFactory` when the player presses
  Start. Menus must run with `renderer == nil`.
- Return to main menu drops the renderer so the menu stays headless.
- Pause overlay skips `Renderer.advance` and map input.
- Escape closes the top overlay, then pops the screen stack, then opens
  pause from the colony.
- Disabled buttons do not navigate; they expose a reason string.
- `Layout` follows the window size; camera/zoom stay presentation-only.

## Invariants

- Screen IDs and widget bounds never enter domain or save state.
- Sprite animation uses a presentation frame counter, not sim ticks, to
  pick frames. Tile lerp may use sim ticks vs `moveTick`.
- UI hit-testing consumes the pointer before the map.

## Hot paths

Colony `Draw` (terrain, pawns, daylight). Keep off-screen culling.
Menu `Draw` is cheap vector fills plus debug text.

## Determinism

This adapter is not in the authoritative tick. `Step(n)` still runs on
the simulation the factory created; speed tiers only change how often
`App` calls `advance`.

## Tests

`nav_test.go` and `app_test.go` cover routing, disabled clicks, Start
creating a sim, and pause blocking `Step`. They must not open a window.
Existing renderer unit tests stay headless.

## Benchmarks

None yet. Add them when colony draw or UI layout shows up in profiles.
