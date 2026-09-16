# Task Handoff

Task ID: PLAY-002
Status: complete

## Summary

Ground food (and wood) stacks, hunger/rest permille decay on `schedule.DefaultColony`, Eat job, and Sleep on the current tile until a bed exists. Critical hunger/rest interrupt Work. Start kit: Ada, Bo, Cora plus food. Inspector food/rest bars. Downed-at-0 is SURV-001.

## Files Changed

- `internal/domain/economy/item/`
- `internal/domain/pawn/needs/`
- `internal/application/game/{game.go,tick.go,play_test.go}`
- `internal/adapters/inbound/ebiten/inspector.go`
- `internal/adapters/inbound/ebiten/hud.go`

## Public Contracts Changed

`PawnView.{Hunger,Rest,Ate,SleptInBed}` (HUD percents 0–100). `WorldView` item stacks + Food/Wood totals. `Game.SpawnStack`, `SetNeeds`, `LoadStartKit`.

## Invariants / Decisions

- Needs are 0–10000 permille; HUD uses `needs.Percent`.
- Hunger decays every 12 ticks awake; rest every 4 ticks awake; sleep restores 3/tick.
- Eat reserves the food entity; no negative quantity (`item.Take` / qty clamp).
- Sleep on the pawn's current tile until PLAY-003 beds exist.
- Critical hunger interrupts Work/Move to Eat when food exists.

## Verification

`TestEatRestoresHunger`, `TestStartKitHasNamedPawnsFoodWoodPlants`
`go test ./internal/domain/economy/item/ ./internal/domain/pawn/needs/`

## Determinism Impact

Decay is tick-modulo, not wall-clock. Candidate food is insertion-order + distance tie-break.

## Save / Mod Impact

None.

## Art / Visual Impact

Vector food stacks and need bars. No bulk art.

## Screen / UI Impact

Inspector food/rest bars; Needs tab selectable. Top chrome Food count. UI/UX §9.1 remains `[-]` (no mood/recreation).

## Known Limitations

No cooking, tables, diet restrictions, or mood. Wood stacks exist for PLAY-003 haul/build.

## Follow-ups

PLAY-003 stockpile/haul/build (done in this roadmap).
