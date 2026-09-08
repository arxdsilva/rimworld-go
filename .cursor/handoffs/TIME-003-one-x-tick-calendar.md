# Task Handoff

Task ID: TIME-003
Status: complete

## Summary

Aligned the dual time system: 60 ticks per real second at 1x, 2500 ticks per hour, 60,000 ticks per day, 60-day year. Speeds are 1x / 3x / 6x. Prototype pawns change cell every 15 ticks so 1x walking stays readable; sprites still lerp across that window.

## Files Changed

- `internal/domain/sim/clock/clock.go`
- `internal/domain/sim/clock/clock_test.go`
- `internal/domain/sim/clock/doc.go`
- `internal/application/game/game.go`
- `internal/application/game/game_test.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/renderer_test.go`
- `cmd/game/main.go`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`

## Public Contracts Changed

- `clock.TicksPerHour` is 2500 (was 1000). Added `TicksPerSecond`, `DaysPerYear`, `RealSecondsPerHour`.
- `game.TicksPerCell` (15) gates random-walk cell changes.
- Renderer steps 1/3/6 ticks per Update frame. HUD labels are `1x` / `3x` / `6x`.
- `cmd/game` sets Ebitengine TPS to 60.

## Invariants / Decisions

- Speed only changes ticks processed per real second, never clock math or movement rules.
- Animation remains presentation-clocked.
- Interpolation duration is sim ticks (`TicksPerCell`), not frames-between-ticks.
- Calendar numbers are our content conversion (public 1x hour/day length), not copied work tables or schedule data.

## Verification

Tests: `go test ./internal/domain/sim/clock/ ./internal/application/game/ ./internal/adapters/inbound/ebiten/`

## Determinism Impact

New games start at 08:00 of the new calendar (`8*2500`). Movement RNG is consulted only on cadence ticks, so positions after N ticks differ from the old every-tick walker for the same seed.

## Save / Mod Impact

Saves that persist raw ticks will read a different hour if `TicksPerHour` changed. No save format exists yet.

## Art / Visual Impact

Walk cycles at 1x last ~0.25s per tile. Day/night still follows hour-of-day; a day is ~16.7 real minutes at 1x.

## Screen / UI Impact

HUD speed text is `1x` / `3x` / `6x`.

## Known Limitations

- `TicksPerCell` is a prototype stand-in for a move-speed stat.
- No schedule tab (Work / Anything / Recreation / Sleep).

## Follow-ups

- Per-pawn movement speed and path following.
- Schedule blocks driven by this clock.
