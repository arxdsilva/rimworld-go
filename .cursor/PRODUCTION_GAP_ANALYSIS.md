# Production gap analysis (PRD-0..PRD-4)

Scope: bring benches, bills, recipes, item quality, materials, deterioration, art and stockpile filters to RimWorld-wiki fidelity (vanilla core only, no DLC). Load only for PRD-* tasks. Phase handoffs go in `.cursor/handoffs/PRD-N-*.md`. Sister documents: `BEDS_GAP_ANALYSIS.md` and `DEFENCE_GAP_ANALYSIS.md`. Armor and weapon recipes are shared with DEF-2.

## Status (2026-10-29)

Wave C done for PRD-2 (quality, stuff, condition, inspiration; handoff `handoffs/PRD-2-quality-stuff-condition.md`). Wave B done (PRD-1/BED-1/DEF-1). Wave A done earlier: the `game/craft` extraction, the `quality` domain package and the wiki constants (PRD-0). PRD-3..4 have not started. Every row in section 3 is `open` unless it says `done` or `partial`. Rows are ticked per phase by the lead, in the same commit that ships the phase.

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for the beds, production and defence systems "so we can implement what is missing", at RimWorld fidelity. This document covers the bench to bill to product loop. |
| Scope | All of vanilla core, no DLC. Ideology, Royalty, Biotech and Anomaly recipes and benches are in section 7. |
| Numbers | ENG-003 in `AGENTS.md` was extended on 2026-10-29 to beds, production and defence. Published RimWorld wiki numbers are used as tuning data. Names and prose stay original. |
| Clean room | No wiki text, code, art or sprite data is copied. Each constant is a named constant with a `// wiki: <page>` comment, or `// original tuning` where the wiki gives none. Section 4 lists them. Sculpture descriptions are generated from original grammar, never copied. |
| Art and sound | Art ships with each phase that adds a bench or item (skill `rimworld-go-art-assets`). Sound generation for this effort is pre-approved by the user; the lead states the count before each run. Benches get work sounds only if the existing set has none. |
| New package | `internal/application/game` is at its SOLID ceiling. Bills, recipes, bench work, drying and the tool-cabinet bonus move into the concise sub-package `internal/application/game/craft` (`.cursor/SOLID_GO.md` section 10): a consumer-owned `deps.go` with roles of at most 8 methods, adapters in game, one Game field, saved state keeping its gob names (bills keep theirs). New behaviour goes on component types, never on the package root type. |
| Quality | Item quality (awful to legendary) is a pure domain package, `internal/domain/economy/quality`: levels, the skill roll curve (the caller supplies the roll), stat and price multipliers. Beds, armor and weapons read it. |
| Budget | PRD-0 moves more than its adapters, so package game shrinks first. No later phase may leave package game larger (lines, files, `Game` methods) than the previous commit. |
| Hysteresis | Ingredient search retries, inspiration triggers and any bill threshold (pause and unpause) use different start and stop values or a retry cooldown, with a no-flap test. |
| Determinism | No new global RNG and no wall clock. The quality roll takes its random number from the caller, drawn from an existing seeded stream. |

## 2. Reading the tables

Legend for the `State` column: `open`, `done`, `out of scope (needs X)`. `Today` evidence gives a path and a function name; line numbers drift while extractions run, so find the function. Paths are under `internal/application/game` unless a full path is given. Every reference was checked against the code on 2026-10-29.

## 3. Gap tables

### 3.1 Bills and the bench loop

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| A bench holds an ordered list of bills; the first runnable bill is worked. | `Game.bills[bid]`, `assignBill` walks the bills in order (`production.go`). `recipe.Bill` is `Recipe`, `Mode`, `Target`, `Done`, `Suspended`, `Waiting` (`internal/domain/economy/recipe/recipe.go`). | None. | - | done |
| Bill modes: do X times, do forever, do until you have X, with a resume level. | `recipe.DoCount`, `Forever`, `UntilHave`; `Bill.Update`, `Bill.Wants`, `Bill.ResumeAt`; `billPending` stops two cooks starting the last meal. | None. | - | done |
| Counting "have X" uses stocks in storage, not loose stacks. | `craft_host.go` `stockOf` still sums every stack of the kind by scanning `g.itemIDs`; `Bills.stock` (`craft/bills.go`) calls it through `Host.Stock`. | Count only stored stacks from an index (not done in PRD-1). | PRD-1 | partial |
| A recipe may need several ingredients, each from its own stack; the pawn hauls them to the bench. | `recipe.Def.Inputs` (`domain/economy/recipe`, `AppendInputs`); `planner.plan`/`takeFor` (`craft/gather.go`) pick stacks across several piles; `job.HaulIngredients` built in `craft/shop.go`, carried by `Shop.gather`/`pickUp` (`craft/haul.go`); retry cooldown after a failed search (`shop.go`). | None for the vanilla case. | PRD-1 | done |
| Ingredient radius limits how far from the bench stacks are taken. | `Bill.Radius` applied in `fits` (`craft/gather.go`) with `pathing.Dist` from the bench spot; 0 means unlimited. | None. | PRD-1 | done |
| Ingredient filter, allowed stuff, quality range, hit-point range on the bill. | `Bill.Forbid` (kinds the bill refuses) and `SetBillIngredient`, checked in `fits`/`usable` (`craft/gather.go`); `BillView.Ingredients`. The filter is a flat list of kinds, not a category tree. No quality or hit-point range yet. | Category tree; quality and hit-point ranges (PRD-2). | PRD-1, PRD-2 | partial |
| Store mode: best stockpile, drop on the floor, or a specific stockpile. | `Bill.Store` (`recipe.BestStockpile`, drop on floor, `StockpileZone` with `Bill.Zone`), set by `ConfigureBill`; `Shop.store` (`craft/haul.go`) delivers the product. | None. | PRD-1 | done |
| Worker restrictions: skill range, specific worker. | `Bill.SkillMin`/`SkillMax` and `Bill.Worker` set by `ConfigureBill`; `Bills.noWorker` (`craft/view.go`) names the reason. | None. | PRD-1 | done |
| Pause and unpause thresholds, count equipped and tainted apparel. | `Bill.UnpauseAt` via `ConfigureBill`; `BillView.ResumeAt`/`Waiting`. Equipped and tainted apparel counting is not done. | Equipped and tainted count rules (PRD-2). | PRD-1, PRD-2 | partial |
| Reorder, copy and paste bills; bill config dialog. | `command.ReorderBill`, `ConfigureBill`, `SetBillIngredient` (`domain/sim/command`); `Bills.reorder`/`applySetting` (`craft/settings.go`); dialog, reorder and copy/paste buttons in `inspector/bill_config.go` (clipboard is local to the UI). | None. | PRD-1 | done |
| Bench pane shows the current worker, progress, and why a bill cannot run. | `BillView.CurrentWorker`, `Progress`, `Gathering`, `NoWorker` filled by `Bills.AppendViews`/`current`/`noWorker` (`craft/view.go`); shown in `inspector/bill_config.go`. | None. | PRD-1 | done |
| Bills need the right research. | Research locks recipes and benches through `lockedBy` and `lockedKeys` (`research.go`), shown as "Needs research: X". | None. | - | done |
| Bills can require fuel, power or a free rack. | `fuel[bid]` for the campfire, `electricBench` with `powered`, racks hold one batch (`production.go`, `drying.go`). | Fueled stove and smithy come with PRD-3. | PRD-3 | open |

### 3.2 Quality, stuff and condition

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Crafted items roll a quality from the worker's skill, with a chance of inspiration-boosted results. | `craft/quality.go` rolls on completion with the worker's skill through `domain/economy/quality`; `item.Stack` carries Quality and Stuff; apparel and weapons roll (grenades do not); constructed beds, bedrolls, animal bed, end table, dresser, table, chair and horseshoe pin roll at construction. | Art (PRD-3) and the remaining craftables (PRD-4) roll once they exist. | PRD-0, PRD-2 | partial |
| Quality changes item stats and market value. | `quality` multipliers read from the stored quality by armor (`armor.AppendPiece`), insulation (`craft.Outfit.Bands`), beds (`construction.FurnitureOf`) and `item.StackValue`. | Weapon damage/accuracy by quality is not applied to wielded weapons (see 3.7). | PRD-2 | partial |
| Inspirations from creativity let a worker produce better work for a while. | `game/mind/inspiration.go`: mood >= 50 and Crafting or Construction >= 3, hourly check with mean time 410 - 4 x mood days (original tuning), lasts 8 days or until the next quality roll uses it, 1-day cooldown; mood never ends it (no flap test). `PawnView.Inspiration`/`InspirationTicks`. | Other inspiration types (checklist 10.2); `InspirationTicks` is not shown in the UI. | PRD-2 | partial |
| Choose the stuff (steel, wood, cloth, leather, stone) for a product; stats depend on it. | `item.StuffOf`, `MadeFrom`, `DefaultStuff`, `StuffFactor`, `Stuffs`, `Insulation` (`domain/economy/item/stuff.go`); `Bill.Stuff` pinned with `ConfigureBill` and the allowed-stuff Forbid filter; `armor.Piece` scales plate and helmets by stuff; `BillView.Stuff`/`StuffKind`/`Quality`. | Melee weapons are not scaled by stuff; the wielded weapon keeps only a kind (see 3.7). | PRD-2 | partial |
| Items have hit points and lose them outdoors, unroofed, over time; tattered apparel works worse; tainted apparel gives a mood penalty. | `item/condition.go` and `craft/condition.go`: 2 HP/day outdoors (original tuning, the wiki gives none), rain x5, water or marsh x3, roofed 0, metal and stone immune, raw resources exempt, quality factor. Worn apparel loses 1 HP on 40 % of days (`craft/garment.go`); ratty at 50 % HP (-3), tattered under 20 % (-5; Apparel page used over the 25 % on the Deterioration page); tainted -5/-8/-11/-14 by count; dead pawns drop a tainted outfit and auto-dress skips it. | Shelves (PRD-4); tattered apparel does not yet reduce armor beyond its HP. | PRD-2 | done |
| Wielded weapons keep their quality and stuff; melee weapons scale with stuff. | `weaponKindOf`/`doEquip` store a kind only, so the crafted quality and stuff are lost on equip; melee weapons are not stuff-scaled. | Carry quality and stuff on the equipped weapon. | PRD-4 / DEF-4 | open |
| Colony wealth counts item and building value. | `Wealth()` (raid points) is separate from `room.BuildingValue`/`item.StackValue`. | Reuse the shared value functions. | PRD-4 | open |
| Stock filters and bills by quality and hit-point range. | `Bill.Forbid` is a flat kind list. | Quality and HP ranges on bills and stockpiles. | PRD-4 | open |

### 3.3 Benches and facilities

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Benches: crafting spot, hand and electric tailor, fueled and electric stove, fueled and electric smithy, machining table, fabrication bench, art bench, electric smelter, biofuel refinery, drug lab, butcher spot and table, fermenting barrel, stonecutter. | 12 bench kinds flagged `Bench: true` in `construction.go`: workbench, stove, butcher table, tailoring bench, campfire, drying rack, funeral pyre, stonecutter table, brewing bench, drug table, electric stove, electric tailoring bench. | Missing: crafting spot, fueled stove, both smithies, machining table, fabrication bench, art bench, electric smelter, biofuel refinery, drug lab, butcher spot, fermenting barrel. | PRD-3 | open |
| Work speed depends on light and room cleanliness at the bench. | `workProgressPct` uses work-speed stat, skill and a percentage argument (`skills.go`); the bench passes `100 + cabinetBonus + electricPct` (`doBill`). No light or cleanliness term. | Light and cleanliness factors in the percent argument. | PRD-3 | open |
| Facilities linked to benches raise speed; the link is visible. | Tool cabinets within `cabinetReach` add `cabinetPct` per cabinet, up to `cabinetMax` (`cabinetBonus`, `items_ng1.go`). No link UI. | Link lines in the inspector; further facility kinds only where the benches need them. | PRD-3 | open |
| Art bench produces sculptures with quality and a generated description. | No art (checklist 14.5 `[ ]`). | Art bench, sculpture item, an original description grammar, beauty hook for BED-3. | PRD-3 | open |
| Electric benches run only on power. | `electricBench` and `electricPct` (`production.go`), `power.ClassBench`. | None. | - | done |

### 3.4 Recipe catalogue

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Meals: simple, fine, lavish, pemmican, packaged survival meal; kibble. | `SimpleMeal`, `CampfireMeal`, `DryFood` only. | The other meals and kibble, with multi-ingredient inputs. | PRD-4 | open |
| Weapons and apparel including armor (flak, plate, marine, helmets, shirts, pants). | About 44 recipe ids (two legacy). Weapons: club, knife, spear, crude rifle. Apparel: leather parka, tuque, duster, wool parka. No armor. | Full list; armor values come from DEF-1/DEF-2 defs. | PRD-4 (with DEF-2) | open |
| Drugs: beer, tea, tablets, hard drugs, medicine. | `BrewBeer`, `BrewTea`, `PressTablet`, `MakeNumbroot`, `PressProphyl`, `RefineMedicine`. | Hard drugs, wort to beer, drug lab recipes. | PRD-4 | open |
| Smelting, chemfuel, advanced components, steel and plasteel work. | `FabricateComponent` only (12 steel into 1 component at the workbench, Crafting 6). | Smelter, refinery, component tiers. | PRD-3, PRD-4 | open |
| Butchering, stonecutting, prosthetics, implants, carpets. | `ButcherAnimal`, five `cut()` stone recipes, prosthetics and implants through `MakeServoArm` and others, carpets. | None. | - | done |

### 3.5 Stockpiles and shelves

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Full item-category filter tree with special filters, quality and condition ranges. | `zone.Zone` has `Food`, `Wood`, `Stone` booleans plus `Corpses` and `Chunks` for dumping zones (`internal/domain/building/zone/map.go`). | Category tree, special filters (allow rotten, fresh only), quality and HP ranges. | PRD-4 | open |
| Storage priority and re-hauling. | `Zone.Priority`, `haul_priority.go`. Checklist 6.3 **Storage priorities** is now `[x]`. | None. | - | done |
| Stockpile names, copy and paste settings. | `Zone.Name` exists; no copy and paste. | Clipboard commands and UI (UI 6.2). | PRD-4 | open |
| Shelves hold items. | No shelf building. | Shelf kind, storage slot rule. | PRD-4 | open |

### 3.6 Already at fidelity (no work)

Bill queue, the three bill modes with resume, suspend, research gating, power-gated benches, drying racks, tool-cabinet bonus, butchering, stonecutting, carpets, prosthetics, work-type and minimum-skill gating, storage priorities. State: `done`.

## 4. Wiki constants

Filled in wave A from the wiki research. Each row becomes a named constant with `// wiki: <page>`. A row that the wiki does not publish records the decision.

| Constant | Value | Source | Used by |
|---|---|---|---|
| Quality levels (awful to legendary) and their order | Awful 0, Poor 1, Normal 2, Good 3, Excellent 4, Masterwork 5, Legendary 6; Legendary only via inspiration or special (in `internal/domain/economy/quality`) | https://rimworldwiki.com/wiki/Quality | PRD-0 |
| Skill-to-quality roll curve, including the chance of a rare jump | Per-skill (0..20) probability table Awful..Masterwork, e.g. skill 0: 64.62/30.18/5.00/0.20/0/0; skill 10: 0.02/3.33/40.22/43.77/12.22/0.44; skill 20: 0/0.01/2.44/37.42/50.61/9.52; Legendary 0 in base. Inspired creativity raises the result 2 levels (skill 20: Normal 0, Good 0.01, Excellent 2.44, Masterwork 37.42, Legendary 60.13). Rare-jump chance beyond the table: not published (in `internal/domain/economy/quality`, matches research) | https://rimworldwiki.com/wiki/Quality | PRD-0, PRD-2 |
| Quality multipliers on HP, armor, damage, beauty, comfort, work speed | Awful..Legendary: beauty -0.1/0.5/1/2/3/5/8; deterioration rate 2/1.5/1/0.8/0.6/0.3/0.1; melee damage and AP 0.8/0.9/1/1.1/1.2/1.45/1.65; ranged accuracy 0.8/0.9/1/1.1/1.2/1.35/1.5 (capped at 100%); ranged damage and AP 0.9/1/1/1/1/1.25/1.5; apparel protection 0.6/0.8/1/1.15/1.3/1.45/1.8; insulation 0.8/0.9/1/1.1/1.2/1.5/1.8; comfort 0.76/0.88/1/1.12/1.24/1.45/1.7; rest effectiveness 0.86/0.92/1/1.08/1.14/1.25/1.6; surgery success 0.9/0.95/1/1.05/1.1/1.15/1.3; recreation 0.76/0.88/1/1.12/1.24/1.4/1.8 (all in `internal/domain/economy/quality`, matches research). HP and work speed: not published (the Quality page has no such factor); decision: not modelled | https://rimworldwiki.com/wiki/Quality | PRD-0 |
| Quality multipliers on market value | 0.5/0.75/1/1.25/1.5/2.5/5, Good and above also capped at +500/+1000/+2000/+3000 (factors in `internal/domain/economy/quality`; the additive caps are not coded, decision pending PRD-0) | https://rimworldwiki.com/wiki/Quality | PRD-0 |
| Inspiration: creative inspiration trigger, duration, effect | Inspired creativity raises the quality result by 2 levels (minimum Normal; Legendary reachable); trigger chance and duration: not published here, decision pending (PRD-2). Production specialist +1 level is Ideology, ignored. Roll shift coded in `internal/domain/economy/quality` | https://rimworldwiki.com/wiki/Quality | PRD-2 |
| Stuff table: HP, sharp and blunt armor, beauty, value, work factor per stuff | Not published in the researched pages (per-stuff HP, armor, beauty, value and work factor were not extracted); decision pending (PRD-2) | https://rimworldwiki.com/wiki/Quality (stuff table not retrieved) | PRD-2 |
| Item deterioration per day by stuff, roofed and unroofed, rain factor | Roofed or indoors 0 HP/day; shelf 0; outdoors base per item (e.g. survival meal 0.25, rice 6; full list not extracted); rain x5; water or marsh x3; immune: metal, stone, cores, artifacts, minified buildings. Worn apparel deteriorates, weapons do not. Quality deterioration factor 2/1.5/1/0.8/0.6/0.3/0.1. Value vs HP: 100% above 90% HP, about 50% at 60%, about 10% at 50%, slow to 0. Pemmican rots in 70 days | https://rimworldwiki.com/wiki/Deterioration, /Pemmican | PRD-2 |
| Tattered stage thresholds and the apparel mood thoughts (tattered, tainted, dead man's) | Mood: ratty -3, tattered -5; tainted 1: -5, 2: -8, 3: -11, 4+: -14 (persists while worn); naked -6. Conflict on thresholds: tattered below 25% and ratty up to 50% vs tattered below 20% and ratty 20-50%; decision pending (PRD-2) | https://rimworldwiki.com/wiki/Deterioration, /Apparel | PRD-2 |
| Bench list: cost, work, size, power, fuel, research, work speed factor | Size / power / cost / build ticks: crafting spot 1x1, speed 0.5; butcher spot 1x1, speed 0.7; butcher table 3x1, 75 stuff + 20 wood, 2000, HP 180; fueled stove 3x1, 80 steel, 2000, 1 wood per 375 work ticks, cap 50; electric stove 3x1, 350 W, 80 steel + 2 comp, Electricity; fueled smithy 3x1, 100 steel, 3000, Smithing; electric smithy 3x1, 210 W, 100 steel + 3 comp; machining table 3x1, 350 W, 150 steel + 5 comp, 3000, Machining; fabrication bench 5x2, 250 W, 200 steel + 12 comp + 2 adv, 5000; hand tailor bench 3x1, speed 0.5, 75 stuff; electric tailor 3x1, 120 W, speed 1.0 (0.5 unpowered), 75 stuff + 50 steel + 2 comp; art bench 3x1, 75 stuff + 50 steel, 2500; stonecutter's table 3x1, 75 stuff + 30 steel, 2000; drug lab 3x1, 50 stuff + 75 steel + 6 comp, 3500 (power not listed); brewery 3x1, 120 wood + 30 steel, 2000; electric smelter 3x1, 700 W, 170 steel + 2 comp, 3500; biofuel refinery 3x2, 170 W, 150 steel + 3 comp, 2000; fermenting barrel 10 steel + 30 wood, cap 25 wort (footprint not published); nutrient paste dispenser 3x4, 200 W, 125 steel + 3 comp. Temperature penalty bands: stove below 5 or above 35 C, butcher below 10 C; magnitudes not published | https://rimworldwiki.com/wiki/Crafting_spot, /Butcher_table, /Fueled_stove, /Electric_stove, /Machining_table, /Fabrication_bench, /Art_bench, /Drug_lab, /Brewery, /Electric_smelter, /Biofuel_refinery, /Nutrient_paste_dispenser (one page per bench) | PRD-3 |
| Work speed by light level and by room cleanliness at the bench | Light below 30% is dark: work speed 0.8 at 0% light (max 20% loss; ramp between 0 and 30 not published); lit 30-89, bright 90-100. Cleanliness bands <-1.1 / <-0.4 / <-0.05 / <0.4 / >= 0.4 (very dirty..sterile) (in `internal/domain/world/room/stats.go`); floor values sterile 0.6, steel/silver/gold 0.2, stone and most floors 0, natural -1, dirt or rubble -5, blood -10, vomit -15. Cleanliness-to-work-speed factors: not published; decision pending (PRD-3). Bench outdoors (no roof) 0.8; bench outside its room role 0.8 (wiki does not explain it clearly) | https://rimworldwiki.com/wiki/Light, /Cleanliness | PRD-3 |
| Facility reach and bonus per facility kind | Tool cabinet +0.06 work speed each, max 2 per bench (total +12%), affects 14 benches including all core ones; 2x1, 200 steel, 1800 ticks, HP 100, research Complex furniture. Reach distance: not published here; decision pending (PRD-3) | https://rimworldwiki.com/wiki/Tool_cabinet | PRD-3 |
| Recipe list: ingredients, work amount, skill, minimum skill, bench, research | Simple meal 0.5 nutrition any, 300 ticks, no skill; fine meal 0.25 veg + 0.25 meat, 450, Cooking 6; lavish 0.5 + 0.5, 800, Cooking 8; pemmican 0.25 + 0.25 -> 16, 700; packaged survival meal 0.3 + 0.3, 450, Cooking 8 (batch time conflict 1440 vs 1800 ticks); kibble 1 veg + 1 meat -> 50 (table) / 35 (spot), 450; nutrient paste 0.3 -> 0.9, no bill; wort 25 hops -> 5, 1000; beer 1 wort -> 1, ferments 6 days at 7 to 32 C; smokeleaf joint 4 leaves, 450; psychite tea 4 leaves, 400, Cooking 2; flake 4 leaves 250; yayo 8 leaves 350; go-juice 2 neutroamine + 1 yayo 600; wake-up 2 neutroamine 900; penoxycyline 2 neutroamine 600; medicine 3 cloth + 1 herbal + 1 neutroamine 700; chemfuel 70 wood -> 35 in 2000 or 3.5 nutrition -> 35 in 2500; component 12 steel, 5000, Crafting 8 (yield not published); advanced component 1 comp + 20 steel + 10 plasteel + 3 gold, 10000, Crafting 8; slag 1 chunk -> 15 steel 400; smelt weapon or apparel 1600, 25% back; destroy 400; stone blocks 1 chunk -> 20, 1600. Batch x4 multiplies ingredients and work. Luciferium not craftable | https://rimworldwiki.com/wiki/Fueled_stove, /Fine_meal, /Lavish_meal, /Pemmican, /Packaged_survival_meal, /Kibble, /Brewery, /Fermenting_barrel, /Drug_lab, /Biofuel_refinery, /Component, /Advanced_component, /Electric_smelter, /Stonecutter%27s_table | PRD-1, PRD-4 |
| Art bench: sculpture work, quality curve, beauty by size and quality | Bench 3x1, 75 stuff + 50 steel, 2500 ticks (research not listed). Sculpture work amount and size-based beauty: not published; quality curve is the shared table above and beauty factor -0.1/0.5/1/2/3/5/8 (in `internal/domain/economy/quality`); decision pending (PRD-3) | https://rimworldwiki.com/wiki/Art_bench, /Quality | PRD-3 |
| Bill defaults: ingredient radius, store modes, pause thresholds | Ingredient search radius default and maximum: not published (a mod page claims 32, unverified; likely unlimited); store modes and pause thresholds: not published; decision pending (PRD-1) | https://rimworldwiki.com/wiki/Bill | PRD-1 |
| Stockpile filter tree: category names and special filters | Not published in the researched pages; decision pending (PRD-4) | https://rimworldwiki.com/wiki/Bill (not retrieved) | PRD-4 |

## 5. Phase plan

**PRD-0, extraction and quality domain.** No behaviour change. `production.go`, `drying.go` and the tool-cabinet bonus (`items_ng1.go`) move into `internal/application/game/craft` behind consumer-owned roles; bills and the save keep their gob names. `internal/domain/economy/quality` lands with levels, roll curve, stat and price multipliers, and tests; nothing calls it yet. Golden hashes and old saves stay byte-identical (`bin/benchmark -hash`). Moves: no checklist item; it prepares 14.4.

**PRD-1, multi-ingredient bills.** Recipes take a list of inputs. Gathering across stacks with a haul-ingredients job, reservations and a retry cooldown (no-flap test). The bill gains store mode, ingredient radius and filter, allowed stuff, skill range, specific worker, pause and unpause levels; bills reorder and copy. Stock counting uses an index. The bill config dialog, current worker and progress, and the no-worker reason ship in the same phase. Moves: vanilla 14.1 Recipe definition, 14.2 Ingredient radius, Ingredient filters, Worker restrictions, Product destination; UI 14.1..14.4.

**PRD-2, quality, stuff, condition.** Quality rolls at craft and at construction, inspirations from creativity, stuff choice with stats by stuff, item HP with roof-aware deterioration on an index, tattered and tainted apparel with their thoughts. Quality is read by bed stats (BED-1), armor and weapons (DEF-1/2). Item quality and HP show in the inspector and tooltips. Moves: vanilla 6.1 Item quality, Item hit points/deterioration, 13 Outdoor deterioration, 14.4 Craft quality generation, 16.2 Apparel deterioration and Tainted apparel.

**PRD-3, benches and art.** The missing benches with research gating, fuel for stoves and smithies, light and cleanliness work speed, a facility link UI, the art bench and sculptures with original generated descriptions. Art for every bench. Moves: vanilla 14.1 Workstation, 14.3 Smithing/weapon production, Machining/advanced fabrication equivalents, Drug production, 14.5 Artistic work and Art descriptions, 12.4 Light can affect work/mood/accuracy where appropriate (work part).

**PRD-4, catalogue and stockpiles.** The full recipe catalogue (meals, weapons, apparel and armor from DEF-2 defs, drugs, smelting, chemfuel, advanced components) with item art. The stockpile filter tree, special filters, quality and HP ranges, copy and paste, names and shelves. Moves: vanilla 6.3 Storage filters, 14.3 entries above; UI 6.2.

## 6. Checklist cross-reference

Each phase ticks these items when its acceptance criteria are met. Titles are quoted exactly. The lead edits the checklists; this file does not.

### 6.1 `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 6.1 Item system | **Item quality** (`[ ]`) and **Item hit points/deterioration** (`[ ]`) | PRD-2 |
| 6.3 Stockpile zones | **Storage filters** (`[-]`) | PRD-4 |
| 10.2 Inspirations | **High-mood inspiration chance** and **Inspiration types** (`[ ]`; creativity part only, note the rest) | PRD-2 |
| 12.4 Lighting | **Light can affect work/mood/accuracy where appropriate** (`[-]`; work part) | PRD-3 |
| 13 Cleaning, Filth, Deterioration | **Outdoor deterioration** (`[ ]`) | PRD-2 |
| 14.1 Generic workbench system | **Workstation** (`[-]`) | PRD-3 |
| 14.1 Generic workbench system | **Recipe definition** (`[-]`; research and quality) | PRD-1, PRD-2 |
| 14.2 Bills | **Ingredient radius**, **Ingredient filters**, **Worker restrictions**, **Product destination** (all `[ ]`) | PRD-1 |
| 14.3 Crafting | **Smithing/weapon production**, **Machining/advanced fabrication equivalents**, **Drug production** (all `[-]`) | PRD-3, PRD-4 |
| 14.4 Quality | **Craft quality generation** (`[ ]`) | PRD-2 |
| 14.5 Art | **Artistic work**, **Art descriptions** (`[ ]`) | PRD-3 |
| 16.2 Apparel | **Apparel deterioration**, **Tainted apparel** (`[ ]`) | PRD-2 |

### 6.2 `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 6.2 Stockpile configuration panel | **Stockpile name**, **Allowed-items filter tree**, **Quality range**, **Hit-point/condition range**, **Special filters**, **Copy/paste settings** (`[ ]`) | PRD-4 |
| 14.1 Workbench inspector | **Current worker/progress** (`[ ]`) and **Interaction status** (`[-]`) | PRD-1 |
| 14.2 Bill stack | **Reorder bills**, **Copy/paste bills** (`[ ]`) and **Add bill** (`[-]`) | PRD-1 |
| 14.3 Bill configuration | **Pause/resume thresholds if supported**, **Ingredient filter tree**, **Ingredient search radius**, **Allowed material/stuff**, **Quality/condition restrictions**, **Worker skill range**, **Specific worker**, **Product destination** (all `[ ]`) | PRD-1, PRD-2 |
| 14.4 Bill explanation | **No valid worker reason**, **Output-count explanation** (`[ ]`) | PRD-1 |

## 7. Out of scope

| Item | Missing prerequisite | State |
|---|---|---|
| Ideology rituals, relic and precept-driven recipes | DLC (Ideology) | out of scope (DLC) |
| Royalty psycasts, royal apparel and bladelink weapons | DLC (Royalty) | out of scope (DLC) |
| Biotech xenogerm, mech gestator and mechanitor recipes | DLC (Biotech) | out of scope (DLC) |
| Anomaly entity-based production | DLC (Anomaly) | out of scope (DLC) |
| Mods and plugin-defined recipes | Plugin system (checklist 36) | out of scope (needs the plugin loader) |
