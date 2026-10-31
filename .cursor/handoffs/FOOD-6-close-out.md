# FOOD-6: close-out

Final phase of the FOOD programme (ENG-003 extended to food). It closes the rows FOOD-1 to FOOD-5 left partial. Agents: S1 (pawn food health), S2 (world and animals), F (food balance and year proof), U2 (UI).

## What changed
- **Pawn food health (S1)**
  - Food poisoning has blood filtration and an eating speed factor (`body.FoodPoisonEatSpeed`, read by the eat step in `tick.go`).
  - A synth stomach halves the poisoning chance (`body.FoodPoisonChance`; wiki: the chance, not the duration).
  - Immunity gain is x0.9 when ravenous and x0.7 when malnourished.
  - The hunger rate uses the wiki formula, rate x (1 + offsets) x multipliers (`body.NeedRates`): gut worms, malnutrition, and stoned +30 % via ashleaf tea (the smokeleaf stand-in). Hypothermia -10 / -40 / -95 % is the insects' hypothermic slowdown in the wiki, and insects have no exposure here, so it is not modelled.
- **World and animals (S2)**
  - Growing zones: roofed cells are sown only within a powered sun lamp's reach; trees never under a roof.
  - Butcher table: work 450, the spot yields 70 % at full speed, default bills butcher animals only (wiki: Butcher table).
  - Breeding (`animal/breeding.go`): wiki gestation, litter, juvenile and maturity ages and lifespan per species; the hive brute is original.
  - Eclipse and toxic fallout (`incident/sky.go`, wiki Events weights x4): eclipse light to 0, crops stop; fallout kills open-sky plants, builds toxic buildup at 40 %/day for pawns and 20 %/day for animals, roofs protect, -5 thought, instant rot on death at chance = severity. Views: `WorldView.SkyEvent` / `SkyEventEnds`, `PawnView` / `AnimalView.ToxicBuildup`.
- **Food balance (F)**
  - Binge: one meal per sitting, at most one sitting per hour (original), 5-19 meals per binge.
  - `EatWorkTicks` 500 (original; the wiki has no ingest time), scaled by the poison eat speed.
  - Deterioration rain x5 and wet x3 confirmed against the wiki.
  - The grown colony's non-food deaths are pre-existing (infections, colonist fights, raids, ridgewolf-explosion fires).
- **Year proof**: `internal/application/scenario/year_test.go` (run with `RIMWORLD_PROBE=1`): 4 colonists survive 60 days on their own farm; food 227 to 3305 to 2024; 439 cooked meals against 40 survival meals. This is 60 days, not the full year the plan named.
- **UI (U2)**: a Freshness rot bar on the item pane, a HUD sky chip with time left, a procedural sky veil, and "Toxic buildup N%" lines.

## Docs
Gap doc: all rows done, phase table gains FOOD-6. Checklist rows: Roofs, Growing zones, Food need, Food poisoning, Butchering, Disease classes, Eclipse, Toxic fallout, Solar output, Mating/pregnancy.

## Open
- A full-year run was not executed.
- Eating capacity and any wiki value flagged "original" in the FOOD-1 to FOOD-5 handoffs.
