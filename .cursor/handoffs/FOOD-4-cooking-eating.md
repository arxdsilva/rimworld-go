# FOOD-4: cooking, preservation and eating at RimWorld fidelity

Wave 2 of the FOOD programme (ENG-003 extended to food). Two agents: C1 (cooking and preservation) and C2 (eating). Code: `internal/domain/economy/recipe/food4.go`, `internal/application/game/diet` (new), `game/craft`, `game/store`.

## What changed: cooking and preservation (C1)
- **Recipes by nutrition** (`recipe/food4.go`, wiki: Meals): a food input counts its nutrition per item (two chicken eggs make a simple meal). Vegetarian and carnivore fine (0.75) and lavish (1.25) meals; batch x4 bills for simple, fine, lavish and survival meals; pemmican batch 64; kibble's plant side takes hay. Wiki fixes: chemfuel from food 70 units (3.5 nutrition), campfire meal work 240.
- **Ingredients**: the cook writes `Stack.Ingredients` / `IngredientSpecies` on every meal, pemmican, kibble and paste meal. `item.MergeFood` merges rot, poison and wear and unions ingredients (used in `tick.go`, `haul/step.go`, craft); carry and `SplitLoad` keep them.
- **Stoves and benches**: a fueled stove burns wood like the smithy (old saves' stoves start empty; haulers refuel). Benches outside 10-35 C work at 70 % (`TestBenchTemperatureNoFlap`).
- **Haulers**: haulers fill barrels; hoppers are filled by `job.FillHopper` records (`Kind.Hauls()`) by haulers and cooks (wiki: Hopper; `TestHaulerFillsHopper`).
- **Preservation**: wiki days to rot and food HP; food deteriorates unroofed, shelves protect; meals 50 HP, so blasts can destroy them. The legacy "food" ration loads as a survival meal (`item.ConvertLegacy`, idempotent) and is never spawned.
- **Original tuning**: fine bulk work 1440, lavish bulk 2560, hay, milk and egg HP and decay (wiki is silent; Rot, Deterioration are 404 or stubs).

## What changed: eating (C2, `game/diet`)
- **Policies** (`policy.go`): Lavish, Fine, Simple, Paste, Raw, Nothing, and "Animal feed" for tame animals (original: allows no meals). Commands `SetFoodPolicy` / `EditFoodPolicy` (Allow, Create, Rename, Delete). Save field `Diet *diet.Save`.
- **Choice** (`choose.go`): score 300 - distance + mood worth, kibble and corpse penalties, a near-rot bonus. All original numbers (the Food_restrictions page is 404).
- **Thoughts** (`thoughts.go`, wiki: Food): fine +5, lavish +12, raw -7, kibble -12, paste -4, without a table -3; the original simple +4 and table +3 are removed. Human meat, insect meat and butchered-human thoughts; the cannibal trait flips the human-meat thoughts (+20, +15) and skips the butchery ones.
- **Poisoning** (`poison.go`, wiki: Ailments): room-cleanliness and cook-skill rolls and the three stages; stored as permille (rounded).
- **Feeding others**: doctors and wardens feed by the patient's policy, fetch paste when it is best; patients get eat thoughts and the poisoning roll.
- **Days of food** (`days.go`): `WorldView.FoodDays`; the low-food alert fires below 2.5 days at 4 nutrition per colonist per day (the vanilla threshold is unconfirmed: the Alerts page was down). No actor threshold, so no no-flap test.
- **Views**: `PawnView.FoodPolicy`, `AnimalView.FoodPolicy`, `WorldView.FoodPolicies` (name lists cached, rebuilt on edit), `ItemView.Ingredients`.
- **Tests**: recipe, item, craft (96.7 %), store (95.5 %), `game/food4a_test.go`, `diet/diet_test.go` (95.8 %). Scale and grown goldens re-pinned; determinism gate passes; the grenade test was updated for meal HP.

## UI representation
Shipped in FOOD-5 (see `FOOD-5-food-ui.md`).

## Open
- Synth stomach poisoning halving; eating capacity.
- Immunity effects of the Ravenous and Malnourished bands, smokeleaf and hypothermia hunger factors (carried over from FOOD-1).
- Real-game proof not run: "a colony feeds itself through a year with farms, hydroponics and livestock" was not executed; only the unit and journey tests above ran.
