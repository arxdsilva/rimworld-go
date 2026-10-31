# FOOD-0: food production gap analysis and contract

User request (2026-10-30): a gap analysis of food production, with every element added as close to RimWorld as possible, carried through until complete. ENG-003 is extended to food production (AGENTS.md).

## What changed
- `.cursor/FOOD_GAP_ANALYSIS.md`: the full gap analysis (growing, animals, cooking, preservation, eating, hunger, UI). Each row has the wiki behaviour, evidence, gap, phase (FOOD-1..5) and status, plus a decision record:
  - the vanilla nutrition scale;
  - original crops, troughs and the drying rack kept;
  - drug crops and devilstrand out of food scope;
  - the 404 and stub wiki pages.
- **Inert contract:**
  - item kinds Rice, Strawberries, Hay, Hops, HumanMeat, InsectMeat, AgaveFruit (filed in the filter tree, kept out of scenario choices until their art lands; `Kind.RawPenaltyFree()`, `Kind.IsMeat()`);
  - plants Rice, Strawberry, Haygrass, Hops (not sowable yet), Agave yields AgaveFruit with yield 0 until FOOD-2;
  - HydroponicsBasin and SunLamp (hidden), research Hydroponics;
  - commands SetFoodPolicy, EditFoodPolicy (modes FoodPolicyAllow/Create/Rename/Delete), SetAutoSlaughter (no-ops); job Slaughter;
  - moods AteFineMeal, AteLavishMeal, AteKibble, AteHumanMeat, AteInsectMeat, ButcheredHuman, ObservedButchery, Ravenous, Malnourished;
  - view fields: PawnView FoodPolicy/Malnutrition, AnimalView FoodPolicy, PlantView Fertility/Light/Lifespan/Blighted, ItemView Ingredients (with `sameItemView`), WorldView FoodDays/FoodPolicies/AutoSlaughter.
- `plantLabel` moved to `plant.Species.DisplayName()`, and `biome_info.go` moved to `game/landing` (−70 game lines, −1 file).
- Checklist corrections: 11 crops, growing period done, fertility overlay partial, the food need's hungry thought.

## Proof
- Benchmark state hashes match HEAD.
- TestGrownGolden re-pinned. There is no behaviour change; the sandbox completes the new research and new stockpiles' filters gain the new kinds' bits, and hiding both brings back the old pin.
- The full race suite and the determinism gate pass.
