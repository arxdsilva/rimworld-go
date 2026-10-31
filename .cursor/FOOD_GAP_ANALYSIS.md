# Food production gap analysis (FOOD-0..FOOD-5)

Scope: growing, animals and butchering, cooking, preservation, eating, and hunger and malnutrition, vanilla core only (no DLC). Load only for FOOD-* tasks. Phase handoffs go in `.cursor/handoffs/FOOD-N-*.md`. The loops exist (zones, sowing, harvest, hunting, milk and eggs, recipes, rot, eating, poisoning, starvation), but the numbers and many mechanics are not vanilla. Sister documents: `PRODUCTION_GAP_ANALYSIS.md`, `MEDICAL_GAP_ANALYSIS.md`, `BEDS_GAP_ANALYSIS.md`, `COMBAT_GAP_ANALYSIS.md`.

## Status (2026-10-30)

FOOD-0 (this analysis, the ENG-003 extension, checklist corrections) is done. FOOD-1 to FOOD-5 are open. The worst gaps: the nutrition scale is not vanilla (a meal is 6000 of 10000, a full bar empties in 2 days), there is no malnutrition (death one hour after reaching 0), four vanilla crops are missing, there is no hydroponics basin or sun lamp, no blight, butchering uses five size buckets, recipes count items rather than nutrition, and there are no food policies.

| Phase | Delivers | State |
|---|---|---|
| FOOD-0 | This analysis; ENG-003 extended; inert contract (kinds, species, buildings, commands); stale checklist rows corrected | done |
| FOOD-1 | Nutrition and hunger: vanilla 1.0 / 0.9 / 0.05 scale, 1.6 per day, saturation bands, eating at 30 %, malnutrition, animal hunger by size and age | open |
| FOOD-2 | Growing: rice, strawberries, haygrass, hops; wiki growth model; partial harvest; lifespan; hydroponics and sun lamp; blight; trees; wild plants; Plant cutting work type | open |
| FOOD-3 | Animals: butchering formula, meat kinds, human and insect meat, slaughter job, auto-slaughter, per-species products, taming cost, hunter carry, egg and milk eating | open |
| FOOD-4 | Cooking and eating: food policies, nutrition recipes, batch bills, ingredients, moods, choice, poisoning, fed patients, deterioration, stove fuel, bench temperature | open |
| FOOD-5 | UI: policy tab, animals columns, days of food, fertility overlay, plant and building panes, tooltips | open |

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | Make food production as similar to RimWorld as possible, carried through until complete, ranked by how wrongly the colony feeds itself. |
| Numbers | ENG-003 in `AGENTS.md` is extended to food production (2026-10-30) at the user's request ("as similar as possible to RimWorld"): published wiki numbers are tuning data; names and prose stay original (generic crop names such as rice are fine). Each constant gets `// wiki: <page>`, or `// original tuning` where the wiki gives none. |
| Nutrition scale | Changes to the vanilla scale: full bar 1.0 (10,000 units), cooked meal 0.9, raw food 0.05 per unit, adult need 1.6 per day. Stored hunger values get a save migration. |
| Original crops | Wheat, beans, lettuce, radish, pumpkin, brewgrain and ashleaf stay beside the vanilla crops; they are not vanilla and are not removed. Brewgrain keeps the hops role until hops exist, ashleaf the drug-crop role. Existing potato, corn, cotton and healroot are retuned to wiki numbers. |
| Originals kept | Feeding troughs (`game/troughs.go`, `trough_feed.go`) and the drying rack are not vanilla and stay as original features. The non-vanilla simple-meal +4 and "ate at a table" +3 thoughts are removed in FOOD-4. |
| Out of scope | Drug crops (smokeleaf, psychoid) belong to the drug gap analysis; devilstrand and cotton fibre belong to the textile gap. Neither is a food. Ashleaf stays as the drug stand-in. DLC foods are excluded. |
| Wiki gaps | Pages that are 404: Rot, Butchering, Food_restrictions. Stubs: Food, Deterioration, Food_types, the Growing_zone summary, Butcher_table butchering, Temperature creatures. Food poisoning numbers come from a summary of the Ailments page. The crop table came from a flattened rendering, so alignment is to be verified. Where a number is missing the row says so and the phase picks an original tuning, recorded in its handoff. |
| Clean room | No wiki text, code, art or sound is copied. New crops, items and buildings get original art through `rimworld-go-art-assets`. No new sound-making events exist, so no new sounds. |
| Budget | `internal/application/game` is at its SOLID ceiling (54,182 lines, 179 files, `Game` 1922 methods). New behaviour goes in sub-packages (`game/crops`, `game/forage`, `game/craft`, `game/store`, `game/care`, a new diet package) behind consumer-owned roles. Never raise a `solid_budget.txt` line. |
| Determinism and hysteresis | Rolls come from the existing streams, no global RNG or wall clock, no per-tick global scans (animals every 60 ticks through the bucket index, plants per chunk every 250 ticks). Every threshold behaviour (eat at 30 %, auto-slaughter, blight spread) starts and stops at different thresholds or has a cooldown, with a no-flap test. |
| Saves | New saved state keeps gob names, gets Export/Import and a round-trip test, and changed units get a migration (`save.go` `migrateSave`). Goldens are re-pinned only in behaviour phases, and the handoff says so. |
| Player visibility | Each phase ships read-model fields and the HUD/UI representation in the same change (FOOD-5 collects the screens) or states in the handoff why none fits. |

## 2. Reading the tables

State legend: `done`, `partial`, `missing`. `Today` evidence is `file:function` with line numbers from the 2026-10-30 inventory; line numbers drift, so find the function. `P` is `internal/domain/world/plant/plant.go`, `G` is `internal/application/game/`, `item.go` is `internal/domain/economy/item/item.go`. Wiki page names in the Vanilla column are rimworldwiki.com pages. About ten rows were spot-checked by grep on 2026-10-30; section 4 lists what was found stale.

## 3. Gap tables

### 3.1 Growing

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Crop table (grow days / yield / fertility sensitivity): potato 5.8 / 11 / 0.4, rice 3 / 6 / 1.0, corn 11.3 / 22 / 1.0, strawberry 4.6 / 8 / 1.0, haygrass 7 / 18 / 0.6, cotton 8 / 10 / 1.0, hops 5 / 8 / 0.7, healroot 7 / 1 / 1.0 (wiki: Plants). | 11 sowable crops (`P:156-193`, sowable list `P:236-238`): potato, corn, wheat, beans, lettuce, radish, pumpkin, cotton, healroot, brewgrain, ashleaf. Original GrowTicks ("real days / 12") and yields per plant (corn 7, potato 8). | Add rice, strawberries, haygrass, hops; retune potato, corn, cotton, healroot to the table. | FOOD-2 | partial |
| Real growth days = growDays / 0.5417: plants rest 19:00-05:00 and grow 13 h of 24 (wiki: Plants). | Plants grow 06:00-20:00 unroofed (`P:108-121`); any lamp or campfire at 50 % light lets them grow all night (`P light.go:11-22,36-42`; `G plant_light.go:99-113`). | Rest 19:00-05:00 in all light. | FOOD-2 | partial |
| Fertility factor GRF(F) = (Fert - 1) x sensitivity + 1; soil 100 %, rich soil 140 %, hydroponics 280 %, stony 70 %, sand 10 %; plants need 50 %, trees 30 % (wiki: Plants, Growing zone). | Formula matches (`P:242-251`); terrain fertility soil 100, rich 140, gravel 70, marsh 60, mud 70, sand 10 (`terrain.go:87-104`). | Marsh and mud values are original; no hydroponics value. | FOOD-2 | partial |
| Temperature factor: optimal 6-42 C, below 6 GRF = T/6, above 42 GRF = (58-T)/16, none outside 0-58 C (wiki: Plants). | `TempFactor` (`P:206`, `P:198-217`): none at or below 0, ramp to 10, full 10-35, 0 at 45 C. No heat death. | Move to 6-42 optimal and 58 C limit. | FOOD-2 | partial |
| Cold kills plants below a random -18 to -10 C threshold; healroot is cold-resistant (wiki: Plants). | `FrostKill` -1 to -8 per species (`damage.go`). | Wiki threshold. | FOOD-2 | partial |
| Light: crops need at least 51 %, GRF(L) = (Light - min) / (optimal - min); standing lamps give 50 % and are not enough (wiki: Plants, Sun lamp). | Lamp light 0.7, radius 6 (`G light.go:32-39,217-233`) lets crops grow; no 51 % minimum. | 51 % minimum and the light factor; plain lamps fall below it. | FOOD-2 | missing |
| Lifespan = growDays x 5 (most crops); a plant fully unlit dies (wiki: Plants). | Ripe crops never age or die (`G plant_growth.go:96`). | Lifespan death. | FOOD-2 | missing |
| Harvest from 65 % growth at half yield, linear to full; yield x health x Plant Harvest Yield; cutting a crop gives its yield (wiki: Plants). | Harvest needs 100 % (`P:319`, `tick.go:94`, `orders.go:187`); `doCut` drops only wood (`world.go:308-336`); damage cuts yield (`damage.go:97-104`); skill yield +-4 %/level clamp 60-140 % (`skills.go:476-482`). | Partial-growth harvest; cutting returns food. | FOOD-2 | partial |
| Sow and harvest work are per-crop hundreds of ticks; minimum sowing skill per crop (wiki: Plants). | Harvest work 15-40, sow work 15-35 ticks (about 10x fast); no minimum sowing skill. Failed harvest below Plants 4 exists (`skills.go:43,520-526`). | Per-crop work and minimum skill. | FOOD-2 | partial |
| Nutrition per day = yield x 0.05 / real grow days; raw crops are 0.05 per unit (wiki: Plants, Nutrition). | Raw berries 1500, meat 2200 on a 10000 bar (`item.go:263-269`). | Unit nutrition goes to the vanilla scale. | FOOD-1 | partial |
| Growing zones: fertile unroofed terrain, not sand, mud, ice, stone, water or floors; sowing only in the growing period; harvested tiles replant; changing the crop re-sows after cutting (wiki: Growing zone). | `addGrowZone` (`G food.go:27-41`), `cmdSetZoneCrop` (`zone_cmds.go:104-112`), research gate `cropLocked` (`research.go:102`), NoSow and NoCut toggles, `assignZoneClear` (`food.go:83-130`); sowing needs 0-50 C daily mean (`climate.go:52-66`). | Terrain rules and temperature window to verify against the wiki; otherwise present. | FOOD-2 | partial |
| Plant cutting is its own work type, separate from Growing (wiki: Plants). | One Grow work type covers sow, harvest and cut (`priorities.go:122-126`, `work.go:7-24`). | Separate Plant cutting work type. | FOOD-2 | missing |
| Hydroponics basin: research Hydroponics, 100 steel + 1 component, work 2,800, Construction 4, 1x4, 70 W, fertility 280 %, plants die quickly unpowered, no corn, devilstrand, haygrass, trees or decorative, default rice (wiki: Hydroponics basin). | None (no basin, no research, no plant pot). | Building, research, power rule, crop restriction. | FOOD-2 | missing |
| Sun lamp: research Electricity, 40 steel, work 330, 2,900 W only 06:00-19:12, 100 % light within 5.5 tiles, 50 % at 6-10, 30 % to 11, heat 3/s, rain short-circuits an unroofed lamp (wiki: Sun lamp). | None. | Building, schedule, light falloff. | FOOD-2 | missing |
| Blight: domesticated plants with grow days under 15; starts at 20 % severity on one species in a circle from a random point (one room); spreads within 4 tiles from 28 %; blighted plants yield nothing; trees (except cocoa), wild plants, decorative and devilstrand are immune; cut ("Cut all blight") or burn (wiki: Blight). | None (only the ship part kill radius, `raid/shippart.go`). | Incident, spread, cut command. | FOOD-2 | missing |
| Wild plants regrow and spread by biome; berry bushes and healroot grow wild; agave gives fruit (wiki: Plants). | Berry bushes (`P:124-125`); agave is a "desert rosette" with no yield (`P:147`); no wild healroot; wild plants never regrow or spread (only grass); `spawnPlant` only from mapgen, sowing and dev. | Regrowth, spread, agave fruit, wild healroot. | FOOD-2 | partial |
| Trees: slow growth, 30-46 wood, need 30 % fertility, no sowing under roofs, one tile clearance (wiki: Plants). | GrowTicks 60-90k sun ticks (about 2 days); wood tree 8, pine 10, jungle 12; sowable Tree, Pine, JungleTree (`crops/crops.go:55-66`). | Wiki growth and yield. | FOOD-2 | partial |
| Eclipse, toxic fallout and other plant incidents. | None. | Not planned; outside the food core. | none | missing |

### 3.2 Animals

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Butcher yield scales with body size, butchery efficiency, maturity, damage and lost parts; the Butchering page is 404, so the formula is original tuning (wiki: Butcher table, Animals). | Five size classes XS 2/0, S 6/2, M 14/5, L 30/10, XL 55/16 meat/leather (`animal.go:455-461`), +-15 % trim (`:530`), `Yield()` (`:549-553`), applied in `craft/shop.go:303-307`; cooking skill +-4 %/level (`skills.go:473-488`). No age, no missing parts. | Body-size formula, age, missing parts, damage. | FOOD-3 | partial |
| Butcher table work is 450 ticks; spot gives 70 % speed; bills default to animals (wiki: Butcher table). | Table and spot (speed 0.7, `station.go:13-16`); kibble 50 at table, 35 at spot (`recipe.go:387-388`). | Work time and bill default to verify. | FOOD-3 | partial |
| Rotten corpses give leather only. | Rotten corpses are refused (`craft/gather.go:52`); corpses decay at 2x shelf (`condition.go:54`). | Leather from rotten corpses. | FOOD-3 | partial |
| Meat is named by species; human meat and insect meat exist (wiki: Meat). | One generic `Meat` (`item.go:19`) and `Leather` (`:34`). No human or insect meat. | `Stack.Animal` species, HumanMeat, InsectMeat kinds. | FOOD-3 | missing |
| Butchering a human: -6 mood to all colonists, stacking -6 to the butcher; cannibal, bloodlust and psychopath immune (wiki: Butcher table). | No human butcher recipe (only cremation, `recipe.go:275`). | Human butcher recipe and thoughts. | FOOD-3 | missing |
| Hunting: ranged weapon, hunter carries the kill to storage, revenge chance shown (wiki: Hunting). | `markHunt` (`animals.go:827-839`, now at 827), `orders.go:291-300`, hunt job `combat/hunt.go` (needs a non-explosive ranged weapon); the hunter does not carry the kill (`colonyKill` `wildlife.go:502-515` marks the corpse for haulers); revenge per behaviour class (`animal.go:473-485`), not shown in UI. | Hunter carry; show revenge chance. | FOOD-3 / FOOD-5 | partial |
| Slaughter is a Handling melee kill with no weapon (wiki: Animals). | Slaughter reuses the hunt flag (`windows/animals.go:186-194`, `cmds_thing.go:296-300`) and runs as Hunt work with a gun. | Slaughter job on Handling. | FOOD-3 | partial |
| Auto-slaughter per species: max males, females, young (wiki: Animals). | None. | Command SetAutoSlaughter and the settings. | FOOD-3 | missing |
| Milk: adult females, cow 14 per day (0.7 nutrition), 0.05 per unit; production x1.0 Fed, x0.5 Hungry, x0.25 Ravenous, x0 Malnourished (wiki: Milk, Saturation). | Adult females, 2-day interval, amount by size 2/4/6/10/15 (`ecology.go:250-265`); fill follows food (`life.go ProductPerDay`); Milk job (`animal_products.go:37-92`). | Per-species amounts and the saturation multiplier. | FOOD-3 | partial |
| Eggs: chicken 0.25, goose 0.5, turkey 0.5, duck 0.25, cassowary 0.5, emu 0.5, ostrich 0.6, cobra, iguana, tortoise 0.25; fertilized eggs ruined outside 0-50 C (wiki: Eggs). | Chickens, ducks, geese daily when food at least 30 %; fertile eggs; hatching 0-45 C (`life.go:20-41`, `animal_life.go:363-498`); `FertileEgg` is not food (`item.go:304`, `wave_e.go:117`). | Species nutrition; fertile eggs edible; hatch window. | FOOD-3 | partial |
| Wool: per-species amount and interval (wiki: Wool). | Every 12 days, amount by size (`ecology.go:254-258`). | Per-species table. | FOOD-3 | partial |
| Breeding and gestation by species, growth stages baby / juvenile / adult (wiki: Animals). | `life.go:8-18`, `animal_life.go:145-313`, gestation 5-25 days, litter by size (`ecology.go:242-248`), growth 6-30 days, old age (`animal_life.go:122-141`). | Per-species values (wiki Gestation numbers still to fetch). | FOOD-3 | partial |
| Taming: food cost, wildness, minimum skill (wiki: Taming). | `TameChance` x skill, downed x2 (`handling.go:62-156`); no food cost, no wildness, no minimum skill. Training is Obedience, Guard, Release only. | Food cost, wildness, minimum skill. | FOOD-3 | partial |
| Animal feed: hay, kibble, meat, plants by diet; eggs and milk eaten by omnivores (wiki: Animals, Hay, Kibble). | Hay is "straw" from wheat and reeds (`plant.go:136,172-173`), worth 900 to plant eaters (`ecology.go:471,484-487`); FEED-001 diet table (`ecology.go:474-506`), feed first (`:508-524`), forage (`forage/pick.go`); animals never eat eggs or milk (`ecology.go:479`). | Hay kind from haygrass; omnivores eat eggs and milk. | FOOD-2 / FOOD-3 | partial |
| Animals and stores: tame animals eat stockpiled food; wild animals do not (wiki: Animals). | `findTameFeed` (`animal_needs.go:519-618`); wild never take stores. No per-animal food restriction. | Food restriction per animal. | FOOD-4 | partial |
| Handlers feed downed animals. | Downed animals are not fed (`animal_health.go:290-325`). | Feed downed animals. | FOOD-3 | missing |
| Feeding troughs. | Original feature (`G/troughs.go`, `trough_feed.go`); not vanilla. | Kept. | none | done |
| Animal hunger by body size; baby x0.4, juvenile x0.75 (wiki: Saturation). | Size-based need rates in `ecology.go`; stages exist. | Rescale to the nutrition model. | FOOD-1 | partial |

### 3.3 Cooking

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Simple meal: 0.5 of any food, gives 0.9, 300 ticks, no skill (wiki: Meals). | Recipe takes 2 of any raw item, work 120 (`recipe.go:239-256`); meal 6000 of 10000. | Nutrition inputs, 300 ticks, 0.9. | FOOD-4 | partial |
| Fine meal: 0.25 meat + 0.25 vegetable, gives 0.9, Cooking 6, 450 ticks; carnivore 0.75 meat; vegetarian 0.75 vegetable (wiki: Meals). | 1 plant + 1 meat, Cooking 6 (`recipe.go:383`, `wave_e.go:42-46`); no variants. | Nutrition inputs, variants. | FOOD-4 | partial |
| Lavish meal: 0.5 meat + 0.5 vegetable, gives 1.0, Cooking 8, 800 ticks; carnivore 1.25 meat; vegetarian 1.25 vegetable (wiki: Meals). | 2 + 2, Cooking 8 (`recipe.go:384`). | Nutrition inputs, variants, 1.0. | FOOD-4 | partial |
| Packaged survival meal: 0.6 requirement, gives 0.9, Cooking 8, 450 ticks, never rots (wiki: Meals). | `recipe.go:386`. Legacy "food" ration kind (never rots, 4000). | Wiki numbers; fold the legacy ration into the survival meal. | FOOD-4 | partial |
| Pemmican: 16 x (0.25 meat + 0.25 vegetable), gives 0.8 each, 700 ticks (wiki: Pemmican). | 1 + 1 gives 16 (`recipe.go:385`). | Nutrition inputs. | FOOD-4 | partial |
| Kibble: 50 from 1 non-vegetarian + 1 vegetable or hay nutrition (2 required), 450 ticks (wiki: Kibble). | Table 50, spot 35 (`recipe.go:387-388`); preference 0. | Spot figure and inputs to verify. | FOOD-4 | partial |
| Nutrient paste: 0.3 per meal, 0.9, awful taste, never poisoned (wiki: Nutrient paste dispenser). | `game/store/paste.go` (2000 nutrition per meal, scale to be checked), hoppers; `FillHopper` job unused. | Vanilla amount; haulers use FillHopper. | FOOD-4 | partial |
| Batch bills (x4): a poisoned batch poisons the whole batch; stacks share poison probability (wiki: Meals, Food poisoning). | No batch bill. `Stack.Poison` (`item.go:475`), `MergePoison` (`:578`). | Batch x4. | FOOD-4 | missing |
| Ingredients are tracked on a meal and shown in the tooltip. | No ingredient tracking (`Stack` `item.go:442-485`). Items are counted, not nutrition. | Ingredient tracking; nutrition accounting. | FOOD-4 | missing |
| Poisoning: corpses 5 %, raw 1-4 % (most 2 %); cooked: room roll (Cleanliness + 2) x 0.05 / 3 capped 0-5 %, blocked above -2, outdoors 2 %; skill roll 0:5 % 1:4 % 2:3 % 3:2 % 4:1.5 % 5:1 % 6:0.5 % 7:0.25 % 8:0.15 % 9-20:0.1 % (wiki: Ailments, summary-derived). | `food_poison.go:30-76`: skill curve 5 % to 0.3 % times cleanliness (clean 0.5 to filthy 3, outdoors 1.5), campfire at least 1.5 %. | Two-roll wiki model. | FOOD-4 | partial |
| Food poisoning condition: 24 h in three stages (0-4 h, 4-20 h, 20-24 h) with vomit MTB 0.3 / 0.2 / 0.4 days, pain +20 % / +40 %, consciousness x0.6 / x0.5, moving x0.8 / x0.5, manipulation x0.9 / x0.8, eating x0.5 / x0.3; bionic stomach halves it (wiki: Ailments). | `cond_food.go:19-45`: 24 h, original capacities, vomit filth, bed rest, sick -8. | Stage capacities. | FOOD-4 | partial |
| Stoves: electric, fueled, campfire; fueled stoves burn wood. Benches below 10 C or above 35 C work at 70 % (wiki: Electric stove, Temperature). | Stoves burn no fuel (`craft/craft.go:72-74`); no bench temperature penalty; electric stove +40 % speed. | Fuel, temperature penalty. | FOOD-4 | missing |
| Fermenting barrel and hoppers: haulers fill them. | `craft/ferment.go`: cooks fill the barrel; vanilla uses haulers. | Haulers fill. | FOOD-4 | partial |
| Cook skill speed and minimum skill. | Present (`shop.go:211`, `skills.go:286-319`, `MinSkill`). | None. | none | done |

### 3.4 Preservation

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Spoilage above 0 C; at 0-10 C the rate is x T/10; at or below 0 C it stops (wiki: Temperature, Food). | `RotRate` (`item.go:627-636`): 0 at or below 0 C, linear to 10 C, applied every 250 ticks (`craft/condition.go:35-58`, `climate.go:101`); rotten items destroyed. | None known. | none | done |
| Freezer is a room cooled below 0 C by coolers; a passive cooler holds 17 C (wiki: Cooler). | Coolers make fridges and freezers (`power_heat.go`). | None known. | none | done |
| Days to rot: meals 4, nutrient paste 0.75, survival meal never, eggs 15, meat about 2 (wiki: Food; the Rot page is 404). | `shelfDays` (`item.go:585-595`): meat 2, meals 4, pemmican 70, potato 30; berries 10 vs 14, milk 3 vs 14, egg 10 vs 15, corn 40 vs 60 (second figure is the best published value). | Correct berries, milk, egg and corn; check paste. | FOOD-4 | partial |
| Deterioration (HP loss, separate from rot): rice 6 HP/day outdoors and unroofed, x5 in rain, x3 on marsh or water, none indoors or on shelves (wiki: Deterioration, a stub). | Food never deteriorates outdoors (`item/condition.go:15-20`). | Outdoor deterioration for food. | FOOD-4 | missing |
| Meals: 10 HP/day deterioration; fertilized eggs ruined outside 0-50 C (wiki: Food, Eggs). | Egg hatch window 0-45 C. | Window. | FOOD-3 | partial |

### 3.5 Eating

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Pawns eat the tastiest allowed food: Lavish > Fine > Simple > Raw > Awful > Terrible, balanced against distance (wiki: Food, Food policy). | `bestFood` (`food.go:240`): preference first, then nearest; paste when best ranks below paste (`tick.go:761-765`); preference table `item.go:273-278`. | Optimality by preference and distance. | FOOD-4 | partial |
| Food policies per colonist and prisoner (Assign tab); defaults Lavish, Fine, Simple, Paste, Raw, nothing; animals feed-first (wiki: Food policy; Food_restrictions is 404). | None; drug policies exist (`drugs.go:68-76`) and are the model. | SetFoodPolicy, EditFoodPolicy, per-animal restriction. | FOOD-4 | missing |
| Moods: fine +5, lavish +12, ate raw food -7 (milk, berries, insect jelly, strawberries exempt), kibble -12, nutrient paste -4, eating without a table -3 (wiki: Food, Meals). | `rooms.go:241-259`: without table -3, at table +3 (not vanilla), simple cooked meal +4 (not vanilla), raw -5, paste -4; fine and lavish give nothing. | Wiki set; remove the two originals. | FOOD-4 | partial |
| Tables and chairs for meals; carry food to a table. | `dining.go` carries to a table within 30 tiles. | Distance rule to verify. | FOOD-4 | partial |
| Cannibalism and insect meat: eating human meat or insect meat gives thoughts; cannibal trait immune; rules not captured by the wiki notes (wiki: Cannibalism, Insect meat). | No human or insect meat, no cannibal handling. | Kinds and thoughts, original tuning for numbers the wiki lacks. | FOOD-3 / FOOD-4 | missing |
| Feeding others: wardens feed prisoners, doctors feed patients, who fetch paste; fed pawns get thoughts and the poisoning roll. | `care/feed.go`, `prison/warden.go` via `careTreat.Feed` (`care_host_prison.go:39-66`) skip `afterEat` (no thoughts, no poisoning); no paste fetch. | Run `afterEat`; paste fetch. | FOOD-4 | partial |
| Poisoning rolled once per meal eaten. | `rollFoodPoison` (`food_poison.go:96-115`, `rooms.go:258`). | None beyond the model above. | FOOD-4 | done |
| Food binge mental break. | Exists. | None. | none | done |

### 3.6 Hunger and malnutrition

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Adult need 1.6 nutrition per day; bar maximum 1.0 (wiki: Nutrition, Saturation). | `needs.go:7-10`: 1 point per 12 ticks, full bar (10,000) empties in 2 days. | Rate 1.6 per day on the 1.0 scale. | FOOD-1 | partial |
| Bands: Fed above 25 % (mood 0), Hungry 12.5-25 % (-6), Ravenously hungry 0-12.5 % (-12, immunity 90 %), Malnourished at 0 % (-20, immunity 70 %) (wiki: Saturation). | Hungry -8 at 25 % and below (`rooms.go:194-195`, `mood.Hungry`); no ravenous or malnourished thoughts. | Four bands and immunity effects. | FOOD-1 | partial |
| Pawns eat at 30 % saturation; overeating wastes the excess (wiki: Saturation). | Eat threshold per `bestFood` callers; no overeating rule. | 30 % threshold with a no-flap test. | FOOD-1 | partial |
| Malnutrition: severity +1.51-2.27 % per hour at 0 % saturation, recovers at the same rate; stages trivial 0-0.19 (-20 mood, hunger +50 %), minor 0.2-0.39 (-26, +60 %, consciousness -10 %), moderate 0.4-0.59 (-32, -20 %), severe 0.6-0.79 (-38, -30 %), extreme 0.8-0.99 (-44, consciousness max 10 %), death at 1.0; wounds do not heal; malnourished pawns eat anything including corpses unless forbidden (wiki: Malnutrition). | None. At 0 the pawn is downed (`tick.go:337-339`) and dies after `StarvationTicks` 2500 (`health/health.go:119`, `tick.go:320-325`), one hour. | Whole condition. | FOOD-1 | missing |
| Hunger multipliers: gourmand x1.5; smokeleaf +30 %; gut worms +100 %; hypothermia -10 / -40 / -95 % (wiki: Saturation). | Not modelled. | Multipliers that exist as sources (gourmand, hypothermia). | FOOD-1 | missing |
| Saves keep stored hunger. | Hunger stored on the old 10,000 bar at the old rate. | Migration of stored values. | FOOD-1 | missing |

### 3.7 UI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Resources panel shows food and a days-of-food readout; low-food alert by days. | Resources `hud/resources.go`; `FoodByKind` is built (`item_view.go:82-114`) but unused; the low-food alert counts items, under 4 per colonist (`hud/alerts.go:48-50,183-184`). | Days of food by nutrition; alert by days. | FOOD-5 | partial |
| Spoiling-soon alert and rot lines in the inspector. | Present (alerts, inspector). | Rot progress bar. | FOOD-5 | partial |
| Food policy tab in Assign; animals tab shows food and auto-slaughter. | Animals window has Slaughter and Release (`windows/animals.go`); no restriction or auto-slaughter columns. | Policy tab, columns. | FOOD-5 | missing |
| Fertility and growing-zone overlays. | Fertility is hover text only (`hud/cellinfo.go:157-166`; the dev layer `mapfields.go:119` is noise). Checklist row 34.2 had it done; corrected to partial. | Painted overlays. | FOOD-5 | partial |
| Plant inspector: growth, fertility, light, lifespan; hydroponics and sun lamp panes. | `plant_view.go:86-88` Growth and Stage; `plant_inspector.go` shows state, ripe days, damage, marks. | Fertility, light, lifespan, new panes. | FOOD-5 | partial |
| Crop picker shows wiki numbers. | Picker in `architect/panel.go:331-345`, `tool.go:275-296`, zone inspector `cmds_zone.go:102-133`, `crops/crops.go:36-52`. | Numbers. | FOOD-5 | partial |
| Meal tooltip with ingredients; last meal; revenge chance. | `LastMeal` (`game.go:107`) is unused. | Tooltips and readouts. | FOOD-5 | missing |
| World growing period. | The landing screen shows the growing period (`wizard/screen_landing.go`). Checklist row 12.2 was stale; corrected. | None. | none | done |

## 4. Stale or wrong evidence

Found by grep on 2026-10-30: `item.go` is at `internal/domain/economy/item/item.go` (the inventory said `internal/domain/item`); `TempFactor` is at `P:206`; `doHarvest` is at `tick.go:1435` and `markHunt` at `animals.go:827`, as listed; `Hunting.Assign` is the hunt job entry at `combat/hunt.go:32`. Checklist rows corrected in this change: Food need (a Hungry thought exists), Crop definitions (11 crops, not 7), Growing period (done on the landing screen), Fertility overlay (partial, hover text only).

## 5. Phase order

FOOD-1 first (every other phase uses the nutrition scale), then FOOD-2 and FOOD-3 in parallel, then FOOD-4 and FOOD-5. See the plan for the wave and worker split.
