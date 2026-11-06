# CSE-W5: spare meal, put back, manual unload (CSE-005, 037, 038, 039)

## What a colonist does now
- **Spare meal (CSE-005).** In free time, rested, food above 30 %, no meal in its pack, `SpareMeal` on: `job.TakeSpare` to the nearest allowed meal through the food index (40 tiles), one meal into the pack. Hungry with a meal in the pack: `Gear.Assign` takes it out and runs `job.Eat` in place (`Count` 2, so no table walk). `SpareAt` waits 2,500 ticks after a failed look, a meal taken and a meal eaten.
- **Put back (CSE-037).** `BackToPack(id, carry)` in `dropCarry`: the stack in `FromPack[id]` goes back into the pack (marked again if an unload was putting it away) when `PutBack` is on and `Room` allows; else the normal drop. `FromPack` is cleared either way.
- **Manual unload (CSE-038).** `Mark` validates pawn (a colonist on this map), mode and index; bad input does nothing; `ManualUnload` off ignores it. Mode 0 sets `Entry.Unload`. Cancel on an entry whose load is in hand stops the job and the load goes back (or down if there is no room).
- **Unload (CSE-039).** `job.UnloadPack`: wait 10 ticks, take the first marked entry (up to 100) to hands, `haulDest` finds a store cell, walk, `Lay`; no cell or unreachable: down where it stands. Next marked entry starts at once; `UnloadAt` is 500 ticks after beginning and cleared on completion, so an interrupted or unable colonist retries on the cooldown. CSE-040's `Entry.Unload = true` is hauled with no button.

## Assign call
`game/tick.go` `phaseAI`, one line before the hungry branch (`senseOf(g).Gear.Assign(id)`); reached only for a pawn with no job. It reads `FromPack`, `Packs`, `SpareAt`, `UnloadAt` for that pawn and allocates nothing; benchmark `BenchmarkAssignNothingMarked` 18 ns/op, 0 allocs.

## Deviations
- **Marks for worn things.** Sense has no field for them, so modes 1 to 3 (garment, weapon, spare) start the unload job at once (`Count` = mode, `BillIndex` = slot) and Cancel ends it; the mark is the job. They use `Drafts.DropWeapon`/`DropSpare` and one slot of the StripGear code. No Cancel is shown for them.
- Buttons are one line high (not 24 px), so they align with the text rows. One line added to `inspector/gear_ui.go` to call `gearUnloadButtons`.
- `sense/pack_test.go`: `gearScene` now builds the habits on the errand fake (`spare_test.go`).
- Meal kinds: Meal, FineMeal, LavishMeal, SurvivalMeal (`sense.IsMeal`).
- Roles added: `GearJobs` (7), `GearFood` (6), `GearWorn` (7); adapters in `game/sense_gear_host.go`.

## Tests
`sense`: TestSpareMealTakenOnce, TestSpareMealWaitsOnItsRules, TestSpareMealDoesNotFlap, TestEatsFromPack, TestMarkUnloadCommand, TestMarkWornThingsStartTheirUnload, TestUnloadStoresMarked, TestUnloadNoStoreDrops, TestUnloadCancelStops, TestInterruptReturnsToPack, TestGroundLoadStillDrops, TestCancelOverCapacityDrops, TestUnloadDoesNotFlap, TestUnloadTakesEveryMarkedEntry, TestAssignAllocatesNothingWhenNothingIsMarked. `game` (`sense_errand_test.go`): TestUnloadMarkedInTheGame (steel balanced every tick), TestSaveMidUnload (hash equal, equal after 300 ticks), TestEatsFromPackInTheGame, TestSpareMealInTheGame. Journey `TestJourneyGearUnload`.

## Save and goldens
No schema change. Default-on, so any colony with a meal on the ground now moves: `TestGrownGolden`, `TestScaleGolden` and the state hashes are expected to move; not re-pinned.

## UI
Gear tab: Unload on the weapon, each worn garment, the spare weapon and each pack row (Cancel while marked).
