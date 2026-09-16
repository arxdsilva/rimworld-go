# Task Handoff

Task ID: PLAY-003
Status: complete

## Summary

First playable gate: paint a stockpile (food+wood), haul, wood stacks, build wall or bed (wood → frame → Construct). Walls block pathing. Beds are claimed for Sleep. Architect: Wall, Bed, Stockpile, Cancel (+ Harvest from SURV) + drag ghosts. Work overlay enables Haul/Construct/Grow. Headless fixture hauls food, completes a bed, a pawn eats, a pawn sleeps in that bed.

## Files Changed

- `internal/domain/building/{zone,construction}/`
- `internal/domain/ai/work/work.go`
- `internal/application/game/{game.go,tick.go,play_test.go}`
- `internal/adapters/inbound/ebiten/{architect,work,hud,app,renderer}.go`
- `cmd/game/main.go`, `cmd/simulate/main.go`

## Public Contracts Changed

Commands: `DesignateStockpile`, `DesignateWall`, `DesignateBed`, `DesignateCancel`, `SetWorkEnabled`.
`WorldView` lists items, frames, buildings, stockpiles. `PawnView.WorkHaul/Construct/Grow`.
`game.NewColony` loads the start kit (`cmd/game` / `cmd/simulate`). `Game.Enqueue` is the adapter alias of `Apply`.

## Invariants / Decisions

- Stockpile filter is food+wood only; haul to an empty matching cell.
- Frames stay walkable; completed walls `SetWalkable(false)`.
- Wall wood cost 2 / 40 work ticks; bed 3 / 60. Original numbers, not copied balance.
- Work types are enable flags, not 1–4 priorities. Table is not virtualized.
- Architect drag: walls/beds enqueue one command per cell; zones use a rect.
- Bed `Claimant` is the sleeping pawn; unmatched sleep is on the current tile (`no bed`).

## Verification

`TestFirstPlayableHaulBedEatSleep` (~80k ticks)
`TestCompletedWallBlocksWalkability`
`go test ./internal/adapters/inbound/ebiten/` (Architect palette, Work toggle, Escape; needs a real display if GLFW is sandboxed)

## Determinism Impact

Job discovery is insertion-order pawns, then distance with EntityID tie-break. Harvest is last in the work order so it does not starve haul.

## Save / Mod Impact

None.

## Art / Visual Impact

Vector stockpile tint, frame ghosts, wall/bed fills. Vendor pawns. No bulk art.

## Screen / UI Impact

Architect overlay + Work enable cells. HUD Food/Wood. Wave UI-2 Architect/placement/zones/Work are `[-]`. Escape: Architect → Work → inspector → pause. Right-click `MovePawn`.

## Known Limitations

- No grow-zones, rooms, roofs, power, stockpile settings UI, or 1–4 priorities.
- Wizard still does not change the start kit.

## Follow-ups

SURV-001 harvest + downed/death (done in this roadmap).
