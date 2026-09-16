# Task Handoff

Task ID: SURV-001
Status: complete

## Summary

Survival gate: harvestable wild plants + Harvest designation (Grow work type, not a grow-zone). Hunger at 0 downs the pawn (`no food`) then kills after `health.StarvationTicks`. Rest at 0 downs (`no bed`) and forces Sleep on the current tile; rest-downed pawns revive when rest rises above 0 and do not starve from exhaustion. Start kit adds plants. First-playable fixture still passes.

## Files Changed

- `internal/domain/world/plant/plant.go`
- `internal/domain/pawn/health/health.go`
- `internal/domain/sim/command/command.go` (`DesignateHarvest`)
- `internal/application/game/{game.go,tick.go,play_test.go}`
- `internal/adapters/inbound/ebiten/{architect,inspector,renderer}.go`

## Public Contracts Changed

`PawnView.Status` / `Reason` (`no food`, `no bed`). `WorldView.Plants`. `command.DesignateHarvest`. `Game.SpawnPlant`, `VitalOf`.

## Invariants / Decisions

- Harvest is Grow work, last in AI work order after Haul/Construct.
- Hunger 0 downs even if the pawn is already in Sleep Do (starvation is not skipped at night). Rest-downed pawns that reach hunger 0 convert to `no food` and can starve.
- Downed pawns skip movement and non-sleep jobs. Exhaustion sleeps in place and revives when rest reaches `needs.CriticalRest`.
- Starvation death is hunger-downed only; rest-downed does not kill.
- Dead pawns release bed `Claimant`.
- Original harvest yield/work ticks; not copied from RimWorld.

## Verification

`TestFirstPlayableHaulBedEatSleep`
`TestHarvestYieldsFood`
`TestNoFoodReachesDownedThenDeath`
`TestExhaustionForcesSleepOnGround`
`TestStarvationDownsWhileSleeping`
`TestDeadPawnReleasesBed`

## Determinism Impact

Plant spawn and harvest designation are command/rect based. Death uses tick counters from `DownedTick`, not wall-clock.

## Save / Mod Impact

None.

## Art / Visual Impact

Vector plant cells + harvest designation outline. Inspector status/reason text.

## Screen / UI Impact

Architect Harvest tool. Inspector downed/dead + reasons. No grow-zone UI (out of scope).

## Known Limitations

- No plant growth, sowing, cooking, corpses, rescue, or mood breaks.
- Grow-zones remain out of this roadmap.

## Follow-ups

Later vanilla phases: rooms, cooking, grow-zones, combat, saves.
