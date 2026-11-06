# WRK-001, HRV-001..010: work order and harvesting

User request (2026-11-04): gap analyses of the Work tab and of harvesting against vanilla, and the "harvested piles are hauled at once" report. Findings and every row: `.cursor/WORK_PRIORITIES_GAP_ANALYSIS.md`, `.cursor/HARVEST_GAP_ANALYSIS.md`.

## What changed

- `work.Order` is the wiki's: Firefight, Doctor, Warden, Handle, Cook, Hunt, Construct, Grow, Mine, PlantCut, Craft, Haul, Clean, Research. `work.Type` keeps its values, so saves mean the same.
- Work tab: columns follow `work.Order` (`windows.workSlot`); everything else stays indexed by type, so cell ids and tests with column indexes did not move. Header says "ties: left first". Tooltips corrected.
- Construct delivers to frames (`assignDelivery`, also first in `assignHaul`). A forced Supply order takes Haul or Construct.
- Inner order per type: `priorities.go`, one line each. Hoppers are Haul only; a cook with Haul off leaves them empty.
- Sweep (`assignWorkByPriority`): a marked colonist tries a fire, then a patient (`care.Doctor.Assign`), then the next plant, before the priority loop. Operations, animal care and Doctor bench bills wait for the field. Radius and mark are unchanged. `NearestRipe` filters by the kind of work and the allowed area, so a plant outside the area no longer ends the sweep.
- Wild plants are Plant cut, crops Grow: `haul.RipeCrops`, `haul.RipeWild`, `wildPlant`; `OrderHarvest` and `ReasonNoPlantCut` follow.
- `domain/world/plant/yield.go`: wiki yield table, Manipulation and Sight, level speed for plant work. Below 100 %: chance of losing everything, no smaller amount. A wasted harvest drops no by-product. Damage cuts at most half. `skillYield` and `SpeedPermil` still serve the other skills.

## The hauling report

Three routes take a fresh pile: a cook's bill and another free colonist (both vanilla), and the harvester leaving for more urgent work (ours, fixed: the sweep was only tried in the Haul slot). A cook with a bill still takes ingredients from a field pile, and a second colonist still hauls.

## Tests changed, and why

- `TestRecreationBlockWorksWhenFull`: its bound on all hungry ticks failed (1497 to 2406). Measured: every colonist starts for food on the tick it gets hungry, eats once, and 909 extra ticks are 302 of walking each, because with the rule on they now mine at 42,9 when the meal comes due, while with it off they idle by the food. Replaced with two checks: hungry ticks not at or on the way to a meal (0 to 0, same bound) and longest hunger, at most 600 ticks more (499 to 839). Stricter on not eating, looser on the walk. Reported to the user.
- `TestJourneyHaulerFillsBarrelCookDoesNot`, `TestHopperIsHaulWorkOnly`: pinned the cook filling the hopper; now pin the opposite (WRK-008).
- `TestNeutralPawnTimingsUnchanged`: Plants has its own curve now, so it checks Mining.
- `TestGrownGolden` re-pinned to `0x7623a063b8e7b5be`, reason in the test.
- Forced supply test: both works off to refuse, and each alone accepted.
- New: `work_order_test.go`, `plant/yield_test.go`, `TestOrderIsTheWikis`, `TestWorkSlotsCoverEveryType`, `TestJourneyWorkColumnsFollowOrder`. `TestHarvesterFinishesTheFieldThenRefines` was confirmed failing against the whole Doctor slot.

## Year proof

`RIMWORLD_PROBE=1 go test -race -run TestYearProofColonyFeedsItself ./internal/application/scenario/` is skipped by default; run by hand on this code and on the commit before. Both: 4 alive, 0 malnourished, no deaths, 440 meals and 40 survival meals eaten. Food at day 60: 666 here, 632 before. More potatoes and less rice remain (day 40: 1711 and 483, was 1240 and 920).

## Not done

- **Work tab capture:** the display was on 6720x3780, the scaled mode behind the kernel panics, so no window was opened. `TestJourneyWorkColumnsFollowOrder` checks positions and clicks, not the look, and the wider header hint was never seen. To do with the monitor on its native mode: `bin/game -capture-ui work`.
- Not checked in the real game: a colony with a ripe field and a cook with a bill; building sites moving with Haul at 3.
- Amd64 hashes were run for `internal/application` and `internal/domain`; the ebiten packages need a cgo cross toolchain.
- `planDelivery` runs twice per assign for a colonist with Construct and Haul on when nothing can be delivered. Cost not measured.
- Feeding a downed animal lives in `AssignTame`, so it now comes after slaughter and collecting.
- Open rows: WRK-101..107, HRV-101..103.

## Budget

`game` stays at its ceiling, 175 files and 52928 lines. `solid_budget.txt` is unchanged.
