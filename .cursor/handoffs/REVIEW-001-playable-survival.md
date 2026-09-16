# Task Handoff

Task ID: REVIEW-001
Status: complete

## Summary

Quality pass on PLAY-001 through SURV-001. Headless tests compile and pass. Duplicate wave handoffs collapsed to plan names. Starvation now no longer skipped during Sleep Do; dead pawns release bed claims. Checklists stay `[-]` where vanilla acceptance is still thin.

## Files Changed

- `internal/application/game/{tick.go,play_test.go}`
- `internal/adapters/inbound/ebiten/hud_test.go`
- `.cursor/handoffs/PLAY-001-path-jobs-names.md`
- `.cursor/handoffs/PLAY-002-eat-sleep-needs.md` (kept; deleted `PLAY-002-needs-eat-sleep.md`)
- `.cursor/handoffs/PLAY-003-stockpile-haul-architect.md` (kept; deleted `PLAY-003-first-playable.md`)
- `.cursor/handoffs/SURV-001-harvest-starvation.md` (kept; deleted `SURV-001-harvest-starve.md`)
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` (Harvest note)

## Public Contracts Changed

None.

## Invariants / Decisions

- Rest at 0 downs (`no bed`) then forced ground sleep; hunger at 0 downs even while sleeping.
- Canonical handoff names match the survival plan waves, one file each.

## Verification

`go test -count=1 -timeout 60s ./internal/application/game/ ./internal/domain/...`
`go test -count=1 -timeout 60s ./internal/adapters/inbound/ebiten/` (unsandboxed GLFW)

## Determinism Impact

Starvation now applies on the same tick-modulo path during sleep; still no wall-clock.

## Save / Mod Impact

None.

## Art / Visual Impact

None.

## Screen / UI Impact

None new.

## Known Limitations

Wave checklist items remain `[-]` with missing notes (no grow-zones, rooms, 1–4 work, temperature, mood).

## Follow-ups

None in this roadmap.
