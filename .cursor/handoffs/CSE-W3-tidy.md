# CSE-W3: tidying a room around a job (CSE-017…027)

CSE-023's steppers are the Habits tab (CSE-W1); the rest is here.

## Shape

`sense/tidy.go` (gate, `After`, `Forget`, `SetBench`, `TidyBench`, `reclaim`, `prune`), `sense/tidy_route.go` (`detour`, `list`, `route`, `Do`, `next`), roles in `TidyRoles`, adapters in `game/sense_tidy_host.go`, `care.Doctor.Urgent` in `care/tend.go`, `CleanOff` in `fillBuildingView`, the bench switch in `inspector/cmds_build.go` (id `cmd.bench_clean`: `bldg.*` ids never read `Cmds`).

## Invariants

- `TidyAt` is the cooldown (1500 ticks, ours). It is set before any search, so a pawn whose dirt is taken or out of reach tries again after it, not each tick, and it holds on the way back through `startJob`.
- `Before` allocates nothing and looks at no tile on its way past: the rule, the cooldown, the queue length, `roomSum`. A meal's seat scans buildings (`AtTable`), once per meal start and only with dirt queued.
- A job cut by `interrupt`, or replaced by another `startJob` (forced orders), never returns: `Forget`. A load held for it goes down there.
- `reclaim`: what a giver claims after `startJob` stays held through a `Before` swap and is reclaimed on resume; the seam released it all. A load in hand sits outside the `CleanRoom` record (`Carry` 0) so the seam's `dropCarry` leaves it.
- A held meal: `noFood` holds the hunger check at most 600 ticks (ours), then hunger cuts the tidy.

## Numbers, all ours

`tidyCooldown` 1500, `tidyMealHold` 600, `maxTidyCells` 64, `hugeRoomCells` 160, `hugeRoomReach` 7, `detourFar` 40 steps, `detourFarLine` 10 tiles, ratio 0.7. Path length stands in for the mod's path cost.

## Deviations from the doc

- `Sleep` is out of the kinds: phaseAI cuts anything but sleep when a pawn is tired or at bedtime, so a tidy would die on the next tick.
- `Stargaze`, `Horseshoes` and `SocialRelax` are in: recreation jobs the doc's list predates.
- `senseOf` uses `newSenseTidy(g)`, not `senseTidy{g}`: the adapter embeds the cleaning adapters.

## Known limits

- Door filth is missed: doors are `room.Wall` in `roomGrid` (verified; the doc left it open). Outdoor jobs never tidy.
- A pawn's own claim makes `CleanFree` false, so a dirty work spot it already holds isn't swept in that tidy.
- `Gather`'s tidy lasts until the next pull; `UseComms` is always forced, so it never tidies.

## Not in owned files, for the lead

- `action_view.go:127,196` and `rooms.go:293` have no `job.CleanRoom` case: no sweep animation or progress bar. The job line does read "clean room".
- `Shop.pending` doesn't count a pawn in a tidy with a bill behind it.

## Tests

`game/sense_tidy_test.go`, `sense/tidy_test.go`, `care/tend_urgent_test.go`, `ebiten/journey_tidy_test.go`.
