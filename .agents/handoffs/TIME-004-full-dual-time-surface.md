# Task Handoff

Task ID: TIME-004
Status: complete

## Summary

Finished the dual-time surface: clock-face HH:MM:SS (2500-tick hour mapped onto 60 minutes), 1-based year/quadrum/day in `Date.String()`, speeds labeled normal/fast/superfast (1x/3x/6x), and a shared 24-hour schedule grid whose current block is on the HUD and `PawnView`.

## Files Changed

- `internal/domain/sim/clock/clock.go`
- `internal/domain/sim/clock/clock_test.go`
- `internal/domain/pawn/schedule/*`
- `internal/application/game/game.go`
- `internal/application/game/game_test.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/renderer_test.go`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`
- `.cursor/HEXAGONAL_ARCHITECTURE.md`

## Public Contracts Changed

- `clock.Date` gained `Second`. `Date.String()` is 1-based for year/quadrum/day and includes seconds.
- Added `clock.SimSeconds`, `MinutesPerHour`, `SecondsPerMinute`, `ClockSecondsPerHour`.
- New `pawn/schedule` package: `Activity`, `Grid`, `DefaultColony`.
- `game.PawnView.Activity`, `Game.Activity()`, `Game` holds one shared grid.
- Renderer `Simulation` now includes `Activity()`.

## Invariants / Decisions

- Simulation seconds (`tick/60`) and clock-face minutes/seconds are different: an hour is 2500 ticks (~42 sim seconds) displayed as 60:00.
- Schedule is derived policy, not a hard script. Needs/jobs do not consume it yet.
- All pawns share `DefaultColony` until per-pawn grids exist.
- Default blocks are original (night sleep, dawn anything, day work, evening recreation).

## Verification

Tests: `go test ./internal/domain/sim/clock/ ./internal/domain/pawn/schedule/ ./internal/application/game/ ./internal/adapters/inbound/ebiten/`

## Determinism Impact

None beyond adding a constant-derived activity field to snapshots.

## Save / Mod Impact

Future saves should persist tick + per-pawn grid, not formatted date strings.

## Art / Visual Impact

None.

## Screen / UI Impact

HUD shows full clock, speed name+multiplier, and current schedule. No Schedule screen.

## Known Limitations

- No paint UI, copy, or per-pawn grids.
- Activity does not yet bias AI.

## Follow-ups

- Per-pawn grids and Schedule screen (Wave UI-2 / checklist 8.1).
- Schedule-aware intent.
