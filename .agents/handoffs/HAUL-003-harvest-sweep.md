# HAUL-003: harvest the field before hauling its piles

User request (2026-11-04): "when plants are ready to harvest the colonist should prioritize harvesting all nearby plants before hauling any of the harvested — similar to what the hauling is now" (HAUL-002's gather-then-haul).

## Behaviour

- `doHarvest` (tick.go) marks the colonist when a harvest completes, fumbled or not (`haul.Sweep.Arm`).
- `assignWorkType`'s `work.Haul` case (priorities.go) tries `g.haul.sweep.Next(haulHost{g}, id)` first: a marked colonist with Grow on starts the nearest ripe marked plant (through the marked-plant index, `NearestRipe` = `nearestPlant(id, job.Harvest, haul.WantsHarvest)`) when it lies within `haul.Radius` (8, Chebyshev, the measure the plant index sorts by) of where the colonist stands. Otherwise the mark goes and the haul options follow as before; `haul.Plan` still gathers the piles in one load.
- Hysteresis: only a finished harvest marks again, so a plant that ripens during the haul never pulls the colonist back. Hunger, sleep, fire, draft and forced orders are decided before work assignment and keep interrupting; after them the sweep resumes only if a ripe plant is still within 8 tiles of where the colonist then stands. Firefight, Doctor and Warden at the same priority still come before Haul in `work.Order`. A colonist with Haul off never reaches the case.
- No toggle (as HAUL-002). No new UI: the job reads "Harvesting"; the sweep is the order of jobs, not a state the player sets.

## Code

- `game/haul/sweep.go`: `Sweep` (sorted id slice; `Arm`, `Armed`, `Disarm`, `Next`, `Export`, `Import`), `SweepHost` = `Harvester` + `GrowOn` + `Position`.
- `game/haul/harvest.go`: `Harvest` and `WantsHarvest` moved out of `game` (`assignHarvest`, `wantsHarvest`) behind the `Harvester` role, with the goldens proven unchanged after the move alone; pays the budget: `game` is 52927 non-test lines / 175 files (was 52928 / 175).
- `haulHost` (tick.go) gains `GrowOn`, `NearestRipe`, `StartHarvest` (32 methods). `haulState` gains `sweep haul.Sweep`; `Game` gains no field.
- Save: `saveState.HarvestSweep []entity.EntityID`, ascending, nil when empty; no `SaveVersion` bump (an older save loads unmarked); `testdata/save_schema.txt` refreshed.

## Tests

`harvest_sweep_test.go`: five ripe bushes harvested before the first Haul job; a bush 20 tiles away hauls first; Grow turned off mid-chain stops it; hunger and draft interrupt; Haul 1 / Grow 3 still finishes a started sweep; no-flap; same seed → same hash; save and load mid-sweep → same hash. `haul/sweep_test.go`, `haul/harvest_test.go`: the branches, the radius edge (diagonal 8 in, 9 out), sorted export and normalising import. Goldens (`TestScaleGolden`, `TestHabitsOffKeepTheHash`, `TestGrownGolden`, every SameSeed/Hash/Determin test) unchanged on arm64 and amd64: the pinned runs never have a grower finish a marked harvest and then reach Haul at the same or a more urgent priority.
