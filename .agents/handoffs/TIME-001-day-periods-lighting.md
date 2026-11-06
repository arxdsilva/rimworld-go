# Task Handoff

Task ID: TIME-001
Status: complete

## Summary

The calendar now names morning, afternoon, evening, and night from the hour. Outdoor light is a deterministic function of tick. The colony viewport applies a presentation-only sky grade, and the HUD shows `period HH:MM`. New games start at 08:00 so the first view is morning, not midnight.

## Files Changed

- `internal/domain/sim/clock/clock.go`
- `internal/domain/sim/clock/period.go`
- `internal/domain/sim/clock/clock_test.go`
- `internal/domain/sim/clock/doc.go`
- `internal/domain/world/lighting/outdoor.go`
- `internal/domain/world/lighting/outdoor_test.go`
- `internal/domain/world/lighting/doc.go`
- `internal/application/game/game.go`
- `internal/application/game/game_test.go`
- `internal/adapters/inbound/ebiten/daylight.go`
- `internal/adapters/inbound/ebiten/daylight_test.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`

## Public Contracts Changed

- `clock.Date` gained `Minute`. `Date.String()` is now `Year Y, Quadrum Q, Day D, period HH:MM`.
- Added `clock.Period`, `PeriodAt`, `HourOfDay`, `NewAt`, `StartingMorning`, `StartHour`.
- Added `lighting.Outdoor(tick) lighting.Level`.
- `game.New` starts the clock at 08:00, not tick 0.

## Invariants / Decisions

- Period and light are derived from tick; they are not stored state.
- Domain stays headless: no colors in `clock` or `lighting`.
- Sky overlay is adapter-only and drawn under the HUD.
- Calendar length is unchanged (`TicksPerHour = 1000`). At normal presentation pace a day is long; use speed 3 to watch a cycle.

## Verification

Tests: `go test ./internal/domain/sim/clock/ ./internal/domain/world/lighting/ ./internal/application/game/ ./internal/adapters/inbound/ebiten/`

In-game window was not visually QA'd in this session. Run `go run ./cmd/game` and speed 3 to see dusk/night.

## Determinism Impact

None. `Outdoor` and `PeriodAt` are pure. Starting at 08:00 changes the initial tick for all new games; existing tick-from-zero clock tests still use `clock.New()`.

## Save / Mod Impact

None yet. Future saves must persist tick (already the source of date/light), not period or overlay color.

## Art / Visual Impact

World view tints warm in morning, clears at noon, orange at dusk, blue at night.

## Screen / UI Impact

Main-colony debug HUD now includes period and HH:MM. No dedicated date-weather chrome widget.

## Known Limitations

- No indoor/roofed darkness or lamps.
- Light does not yet affect plants, work, mood, or accuracy.
- Overlay is a full-screen grade, not per-tile lighting.

## Follow-ups

- Roof/indoor and artificial lights in `world/lighting`.
- Dedicated date-weather HUD chrome (UI checklist 2.3).
- Optionally shorten `TicksPerHour` if a day should pass faster at normal speed.
