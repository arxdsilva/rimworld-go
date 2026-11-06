# FOOD-2: growing at RimWorld fidelity

Wave 1 of the FOOD programme (ENG-003 extended to food: wiki numbers, original names and prose). Code: `internal/application/game/farm` (new), `internal/domain/world/plant/farm.go` (new).

## What changed
- **Crops** (`plant.Crops()`, `// wiki: Plants`): rice, strawberry, haygrass and hops added; potato, corn, cotton and healroot retuned to the wiki grow days, yield and fertility sensitivity. Original crops (wheat, beans, lettuce, radish, pumpkin, brewgrain, ashleaf) stay. Hay now comes from haygrass; the wort recipe accepts hops or brewgrain.
- **Field work**: sow work 170, harvest 200 (wiki-scale hundreds of ticks); minimum sowing skill: healroot 8 (work 800 / 400), strawberry 5, hops 3, trees 6.
- **Trees**: tree 30 days / 46 wood, pine 20 / 27, jungle tree as teak 32.5 / 60.
- **Growth model**: plants rest 19:00-06:00 in any light; 51 % light minimum (lamp light is capped at 50 %, so plain lamps no longer grow crops); the wiki temperature curve (optimal 6-42 C, limit 58 C) and cold death (healroot -29 C).
- **Harvest and lifespan**: harvest from 65 % growth with a partial yield; lifespan is 5 x grow days.
- **Hydroponics basin** (280 % fertility, 70 W, research Hydroponics gates it): basin plants are ordinary per-cell plants read as `PlantView.Fertility == 280`. Unpowered plants die in 20 h (original tuning: the wiki says "quickly").
- **Sun lamp**: 2,900 W, 06:00-19:12, radius 11 (`shining` in `light.go`); `BuildingView.Glow` and `GlowRadius`.
- **Blight**: incident `crop_blight` (weight 10, cooldown 30 days). It strikes zone-sown domesticated crops under 15 grow days, spreads within 4 tiles, and is cleared by cutting. Origin radius and severity thresholds follow the wiki where published; weight and cooldown are original tuning.
- **Wild plants**: regrowth, agave fruit, wild healroot.
- **Work types**: new Plant cutting work type (`woodLow` removed).
- **Views**: `PlantView.Fertility`, `Light`, `Lifespan`, `Blighted` filled.
- **Goldens**: map goldens re-pinned (plants now spawn with a share of the wiki grow time); TestScaleGolden and TestGrownGolden re-pinned with FOOD-1..3.
- **Tests**: `food2_test.go`, `plant/farm_test.go`, `farm/farm_test.go`, `farm/field_test.go`.
- **Art**: plant stages for the four new crops, agave harvestable, basin and sun lamp (see the art handoff notes in the wave report).

## UI representation
Done: crop batch counts (rice 4, haygrass 4, strawberry 3, hops 2), raised basin drawing, agave ground pick, build keys for basin and sun lamp, sun lamp glow. The plant inspector fields exist in the read model but the panes, overlays and crop picker numbers are FOOD-5.

## Open
- FOOD-5: a work-tab Plant cutting column (`command.WorkPlantCut` plus a `PawnView` flag); a "Cut all blight" command; fertility and growing-zone overlays; basin and lamp panes; picker columns.
- Blight against ship-part kill radius is separate and unchanged.
- Not done: eclipse, toxic fallout, devilstrand, smokeleaf and psychoid (out of food scope).
