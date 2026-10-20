# SIM-014 Movement timing, diagonal cost and creature blocking

## What changed
- **Per-creature step timing** (`motion_store.go`): each colonist, raider and
  animal has a `motionRec` (`NextMoveAt`, `StepTick`, `StepTicks`,
  `BlockedTicks`, `BlockedBy`) instead of the global `(now+id)%15` slot. A
  step costs `TicksPerCell` (15) straight or `TicksPerCellDiag` (21) diagonal;
  animals cost `Step()` or `round(Step()*1.414)` (`animal.StepCost`).
- `phaseMovement` moves colonists, raiders **and animals** in ascending ID,
  applying each move at once (animals no longer step in `phaseAnimals`).
- **Read model**: `PawnView`/`AnimalView` expose `StepTick`, `StepTicks`,
  `NextX/NextY`, `HasNext`, `BlockedBy` for presentation.
- **Blocking** (`occupancy.go`): an occupancy index of standing creatures
  (downed, sleeping and carried pawns don't block; lower ID keeps a shared
  cell). `enter(id, from, to)` returns a verdict, in order:
  1. free cell: go;
  2. next cell holds my attack target: wait (melee from adjacent);
  3. a ring of walkers (head-on swap = 2-ring): the highest ID yields every
     15 ticks: sidestep off both paths towards its goal, else repath, else
     back off along the other's path (1-wide corridors), else wait;
  4. occupant walking elsewhere: wait;
  5. idle occupant and the walker blocked >= 30 ticks: the occupant gets a
     1-step make-way job (raiders, drafted and breaking pawns never yield);
  6. last cell held by a stationary occupant: exact-cell jobs wait (`Sleep`,
     `Relax`, `Bill`, `Research`, `Horseshoes`; `Eat`, `Haul`, `Rescue` while
     carrying); move orders and animals take the nearest free cell; other jobs
     arrive next to the work cell;
  7. otherwise repath at >= 45 blocked ticks and every 45 after
     (`pathAround`: +40 on occupied cells, +20 next to stationary ones); a
     colonist behind a stationary creature repaths at once.
  After 600 blocked ticks the job fails as `job.Blocked` (`movePawn` only;
  animals drop their path). Haulers that detour keep walking to their
  drop-off (`walkTarget`).
- **Pathing** (`domain/ai/pathing`): octile costs 10/14 with an octile
  heuristic, no diagonal step past a wall corner, optional `Coster`
  (`ExtraCost(x, y)`), pooled search buffers (no per-call allocation).
  Tie-break unchanged (f, g, y, x).
- Dev: `-scenario corridor|crowd`, `-capture-ui corridor|crowd`,
  `-capture-speed 1|2|3` (`cmd/game`).

## Tests
- Timing: straight/diagonal step cost, animal diagonal scaling, snapshot step
  timing, save round trip, `TestOldSaveWithoutMotionLoads`
  (`testdata/premotion_v1.sav`), verdict handling via `enterHook`,
  `TestSidestepWhileCarryingReplansToDestination`,
  `TestRepathOrWallWhileCarryingKeepsDestination`.
- Pathing: octile prefers straight, no corner cutting, `Coster`, octile
  optimal vs brute force on seeded random grids (99.2% coverage).
- Occupancy units: blocked by a standing pawn; downed/carried don't block;
  idle steps aside; head-on swap resolves; arrive-near; exact-cell jobs wait.
- Scenarios (`blocking_scenarios_test.go`): corridor head-on (both arrive;
  budget is uncontested time + 300 ticks because the yielder backs out),
  crowd spreads, door chokepoint queues, three-ring no deadlock, raider blocks
  a doorway, `TestNoSharedCellInvariant` (seeds 1-5 x 20k ticks with a raid
  and animals; `-short` 2 x 5k), `TestBlockingSameSeedSameRun`.
- Coverage: `application/game` 83.0% -> 85.1%, `pathing` 91.7% -> 99.2%,
  `domain/animal` 68.4% -> 73.8%.

## Benchmarks
`go test -run '^$' -bench Colony -benchmem ./internal/application/game/`
(new `bench_test.go`): Hour 16.5 -> 17.3 ms/op, Raid 18.6 -> 19.5 ms/op
(+5% each); allocations 5.6 MB -> 0.31 MB and 12 MB -> 0.54 MB per op.
Movement phase ~0.2 -> ~1.6 ms per hour (mostly the per-tick motion loop);
`doCombat` path searches roughly double in raids because fights last longer
once raiders can't stack.

## Determinism and saves
Deterministic (ID order, fixed neighbour order, no map iteration, no RNG or
wall clock), but outcomes for a given seed differ from earlier builds.
`motionRec` is saved; saves from before load with "due now, straight step".
`SaveVersion` stays 1. The occupancy index is transient (rebuilt).

## UI
Presentation in ART-007: blocked creatures ease to a stop and face whoever is
in the way; the colonist inspector shows "waiting for <name>".

## Known limitations / follow-ups
- Simultaneous move orders to one cell still fail as Reserved (tile
  reservation in `cmdMove`); drafted groups need a formation/spread order.
- No movement speed stat or terrain cost yet (checklist "Movement cost").
- No region graph; repaths are still full A* (rate-limited).
