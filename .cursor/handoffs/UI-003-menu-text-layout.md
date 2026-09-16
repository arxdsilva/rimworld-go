# Task Handoff

Task ID: UI-003
Status: complete

## Summary

Wizard body copy no longer draws under left-rail buttons. `bodyOrigin` places text to the right of non-footer controls. Scenario shows name/desc/summary in that column. World Gen’s disabled Generate sits in the footer.

## Files Changed

- `internal/adapters/inbound/ebiten/app.go`
- `internal/adapters/inbound/ebiten/screen_newgame.go`
- `internal/adapters/inbound/ebiten/screen_boot.go`
- `internal/adapters/inbound/ebiten/screen_menu.go`
- `internal/adapters/inbound/ebiten/screen_options.go`
- `internal/adapters/inbound/ebiten/app_test.go`

## Public Contracts Changed

None. Button IDs are unchanged.

## Invariants / Decisions

- Footer buttons (`y` at `sh-64`) do not shift body origin.
- Any other button on the screen forms a left rail; copy starts at `max(x+w)+16`.
- Boot still uses `bodyDefaultX` (no rail).

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/`
Benchmarks: none.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

None. Layout only.

## Screen / UI Impact

Scenario, Storyteller, Starting Pawns: list/actions left, copy right. World Gen, Load, Options, Credits: full-width copy above the footer.

## Known Limitations

- Still one body panel, not separate center/right panels.
- DebugPrint does not wrap long lines.

## Follow-ups

- True three-column scenario chrome when fonts exist.
- Enable Generate when worldgen exists.
