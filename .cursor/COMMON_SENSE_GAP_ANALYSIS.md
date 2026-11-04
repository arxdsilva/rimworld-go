# Common Sense gap analysis (CSE-AG)

Scope: every behaviour in the 1.6 build of the Common Sense mod for RimWorld, compared with our game at commit `359f99d0f`. Load it for CSE work only. Sources: the mod's C# files, its 1.6 patch and def XML, its settings strings and its workshop description, read on 2026-11-03 to learn what it does. The mod's repository has no license: no code, text or name of it enters this repo; every name below (`Habits`, `sense`, `Tidy`, `Pack`) and all prose is ours. Mod setting keys appear only in the source column of the tables, so a reader can find the behaviour again.

Paths are relative to `internal/` unless they start with `.cursor/`. `game/` means `internal/application/game/`. Line numbers are from the commit above; other agents were working in the same checkout, so re-check them before editing.

## Decisions

| Item | Decision |
|---|---|
| Missing systems | A row that relies on something our game lacks is `excluded`, with the reason in one clause. Each absence was checked by grep over `internal/`: no `psyfocus` or `meditat`; `hygiene` and `sidearm` only as flavour text (`game/social_log.go:61`, `domain/pawn/backstory/catalog.go:104`); no sandstorm or sand rate (`Sand` is a terrain, `domain/world/terrain/terrain.go:13`); no joy-giving food; no vacuum, containment cell, ideology or slave system seen. |
| One rules record | All toggles and limits live in `gameplay.Settings` (`internal/domain/gameplay`, new): authoritative game state, carried in the save, changed only by a typed command, replayable. The simulation never reads `config.Settings` for it. Presentation-only rows (CSE-038, CSE-046) sit in the same record so there is one place and one screen. |
| Zero value means defaults | `Settings` stores differences from the defaults, so an older save, and a colony nobody changed, hold the zero value. It adds nothing to the state hash until the player flips something. Defaults follow the mod's. |
| Screen | New Options tab, **Habits** (`options/habits_tab.go`), offered only while a colony runs, like Story (`adapters/inbound/ebiten/options/screen_options.go:63`, `story_tab.go`). The existing Gameplay tab reads `config.Settings` (`screen_options.go:310`) and is the wrong source. |
| Sub-package | New `game/sense`, reached by free function `senseOf(g)` through the colony-wide `worldState` (`g.worldS().sense`), the way `shipOf` does it (`game/story.go:400`). No new `Game` field or method. Colony-wide means it survives moving between maps (MAP-0: `mapState` is per map, `worldState` is not). |
| Inventory | We have none (`domain/pawn/inventory/slot.go` is a single spare-weapon slot) and no carry capacity. Rows CSE-005, 011, 029, 030, 037 to 040 depend on CSE-036, a new per-pawn `Pack`. The pack holds stack **values**, off the map, so they join no stockpile, index, reservation or `Carrier` filter (`Stack.Carrier` has 119 readers in 58 files) and travel with the pawn. Anything leaves the pack through the pawn's hands as an ordinary carried stack. |
| No new `Stack` fields | `item.Stack` has 24 fields against the guard's 25 (`internal/arch/solid_test.go:48`). Everything a pawn or a mark needs goes in `sense`, not on the stack. |
| No job queue | A pawn holds one `job.Record`. "Do this first, then the original" is a saved `Resume` record per pawn in `sense`, started at the existing job-end seam (`game/tick.go:478`). See CSE-024. |
| Randomness | Mod rows that roll become deterministic: a hash of tick and ids (`care/visit.go:84` is the precedent, and `care/AGENTS.md` forbids randomness), or an existing `g.stream(...)`. CSE-041 must draw no extra number with its toggle off. |
| Thresholds | The mod's numbers are recorded for parity and are tuning we may change; they are a mod's, not the RimWorld wiki's, so ENG-003 does not cover them. The 80 % and 95 % of CSE-002 come from the brief. Pack capacity is original tuning (see Open questions). |
| Stale settings | `add_to_que` and `allow_feeding_with_plants` are commented out in the mod's settings and absent from its 1.6 assembly (byte search). Their strings remain. Their rows are classified anyway. |
| Duplicate patches | The mod's `FoodOptimality.xml` and `PackMeal.xml` are the same patch. One row: CSE-005. |
| Optimal patching | Not applicable: our toggles are plain branches; there is nothing to skip at start-up. CSE-050. |

## Counts

| | Count |
|---|---|
| Rows | 55 |
| Done | 5 |
| Partial | 8 |
| Missing | 32 |
| Excluded | 10 |
| Default-on rows expected to move `TestGrownGolden` | 16 (see Golden impact) |
| Toggles proposed | 27 |
| Limits proposed | 3 |
| New command kinds | 4 |
| New job kinds | 4 |

## Golden impact

`TestGrownGolden` (`application/scenario/grown_test.go:63`) and `TestScaleGolden` (`game/scale_golden_test.go`) pin the state hash, which is computed over the saved fields (`game/statehash.go:24`). A nil or zero `Sense` field and an empty hook should leave it unchanged; WP0 proves that.

- **Expected to move goldens (default on, reached by an ordinary colony):** CSE-001, 002, 005, 007, 008, 015, 017, 018, 021, 022, 028, 029, 031, 033, 035, 040.
- **Default on, but may not move them; check, re-pin only if so:** CSE-012, 037, 032.
- **No golden change:** default-off rows CSE-014, 019, 020, 041, 042; CSE-010 (acts only on a new opt-in preset); CSE-043 (nothing in the simulation reads the role until CSE-042 is on); every UI row.

The lead re-pins once per wave, with a written reason naming the CSE rows, stable over `-count=2`. Feature agents never re-pin. An agent whose WP is meant to be hash-identical proves it with `bin/benchmark -hash`.

## Non-negotiables for every package

No goroutine per pawn. No global scan: use `g.filth.cleanQ`, the `roomSum` test, chunk and food indexes, and bound what remains. No global RNG or wall clock. Maps are saved and walked in sorted id order. Every threshold-driven row names its hysteresis, minimum duration or cooldown and ships a no-flap test in the style of `game/sleep_test.go` (`TestColdAndTiredDoesNotFlap`, :262). Saved fields are additive; no field is renamed or removed here, so no `SaveVersion` bump is planned and `TestSaveV12Fixture` must keep passing; `TestSaveSchema` and `testdata/save_schema.txt` are regenerated by WP0 and by any WP that adds a saved field. Each new command kind needs a UI action and a journey (`TestEveryInputActionHasAJourney`). Player-visible state goes through read models. A checked gate (`Settings.On`) is a bit test and allocates nothing on the tick path.

## Recreation and free time

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-001 | `fulfill_outdoors` (default on). Below 40 % outdoors need, and not on medical bed rest, recreation is first looked for outdoors: joy sources are tried by weight, and the first whose target stands in a room that feels outdoors wins; otherwise the normal choice runs. | Need: `game/rest/needs.go:31`, `:51`. Choice: `game/recreate/seek.go:40`, score is distance plus tolerance, no sense of outdoors. Bed rest returns earlier, `game/tick.go:536`. | missing | **Package** `recreate`. Add `Outside(x, y int) bool` to its `Rooms` role (`recreate/deps.go`), backed by `restRooms.Outside` (`game/rest_host.go:159`), and `OutdoorsPct(id) int` to `Feelings`. `Seek` makes a first pass over outdoor pieces only, then the normal pass. **Hysteresis:** a saved craving set below 40 %, cleared at 60 % (ours). **Save:** `recreate.Save.Craving []entity.EntityID`. **Toggle** `OutdoorsJoy`, on. **Tests:** `TestSeekPrefersOutdoorsWhenCraving`, `TestOutdoorsCravingDoesNotFlap`. **Golden:** moves. |
| CSE-002 | `fun_police` (default on). A per-pawn "topped off" flag, set when any job ends with joy above 95 %, cleared when a job starts with joy below 80 %. In Recreation and Sleep blocks the wish for recreation starts at 80 % once topped off, instead of 95 %. Colonists only. | No flag. `Seek` starts below `recreation.Entertained` 85 % (`game/idle.go:84`; `domain/pawn/recreation/recreation.go:55`). A Recreation block never works, whatever the joy (`game/tick.go:606`; `needs.WorkScheduled`, `domain/pawn/needs/needs.go:230`). | partial | **Package** `sense`, component `TopOff`. `Wants(id, joy int) bool` is read at `game/tick.go:606`: a colonist that does not want recreation looks for work (`assignWorkByPriority`) before idling. **Save:** `Save.TopOff []entity.EntityID`. **Toggle** `JoyTopOff`, on. **Tests:** `TestTopOffHysteresis`, `TestTopOffDoesNotFlap`, whole-game `TestRecreationBlockWorksWhenFull`. **Risk:** this turns full colonists' Recreation hours into work; it is the largest balance change here. **Golden:** moves. |
| CSE-003 | `fun_police`, second half. In a Sleep block, recreation yields to sleep unless the pawn is topped off. | `bedtime` goes before recreation (`game/tick.go:556`, `:557`), and free time only once rested. | done | Nothing. Same order by construction. |
| CSE-004 | No toggle. A Sleep block counts as free time for a pawn that cannot sleep. | `rest.MarkRested` and `RestedFree` (`game/tick.go:553`, `game/rest/sleep.go:129`). | done | Nothing. |
| CSE-005 | `PackMeal.xml`, `FoodOptimality.xml` (no toggle, always on). The pack-food gate moves from food above 60 % to above 30 %, so pawns pick up a spare meal more readily. | No pack, no pack-food behaviour. | missing | **Needs CSE-036.** **Package** `sense`, component `Spare`. Job `job.TakeSpare`: in free time, when food is above 30 % and the pack holds no meal, walk to the nearest meal through the food index (`g.food().items.each`), put one in the pack. Eating (`game/tick.go:794`, `assignEat`) tries the pack first when hungry. **Cooldown:** failed search waits 2,500 ticks (ours). **Toggle** `SpareMeal`, on (the mod has none; every behaviour here gets one). **Tests:** `TestSpareMealTakenOnce`, `TestSpareMealDoesNotFlap`, `TestEatsFromPack`. **Golden:** moves. |
| CSE-006 | `pick_proper_amount` (default on). A recreational food job takes no more than the stomach will hold. | No joy-giving food exists. `doEat` already caps by hunger (`game/tick.go:1218`) and `doIngest` takes one dose (`game/drugs.go:359`). | excluded | No joy-giving food in our game. Reopen if one is added; apply `portionsFor`. |
| CSE-007 | `give_sick_joy_drugs` (default on). A visit to a sick friend may instead become feeding the patient a joy drug or food. Choice weighted: drug 4, the visit its own chance, each scaled by the patient's tolerance. The drug is taken from the visitor's inventory first, else the closest stack the patient's policy allows, policy order shuffled. | Visits: `game/care/visit.go:54`, no gift. Feeding patients: `game/care/feed.go:172`, `:188`, food only. Policy: `domain/pawn/drug/policy.go:37`. Dose: `game/drugs.go:376`. | partial | **Package** `care`, new component `Gifts` (`care/gift.go`), new role `Gifts` in `care.Roles`, not on `Care`: `GiftFor(visitor, patient) (stack entity.EntityID, x, y int)`, `GiveDose(patient entity.EntityID, k item.Kind)`, `Tolerance(patient, k) int`. Runs inside `Visits.Assign` after a patient is chosen. Selection by hash of tick and pair (`visitSlot` style), never `rand`. Uses `job.FeedPatient` with a drug in `Carry`; `Feeding.Do` branches on `Kind.IsDrug()`. Joy food part is excluded (CSE-006). **Cooldown:** the visit slot, plus the dose gap in `Allows`. **Toggle** `SickGifts`, on. **Tests:** `TestGiftOnlyWithinPolicy`, `TestGiftDoesNotFlap`, `TestGiftDeterministic`. **Golden:** moves. Taking from the visitor's pack waits for CSE-036. |
| CSE-008 | `social_relax_economy` (default on). A drink taken for social relax is carried to the spot and finished at the end, once joy reaches about 99 % less what the drink gives, so its joy is not wasted at full. | `idleSocialRelax` (`game/idle.go:194`) takes no drink at all. Drinks are separate jobs, gated by low joy (`game/drugs.go:276`). `ingestDose` clamps (`:383`). | missing | Base behaviour first. **Package** `sense`, component `Relax`; adapter at `idleSocialRelax`. On starting a relax, if `Allows` passes with `joyLow` true, claim the nearest allowed drink, set `Carry`, walk to the seat. The job ends when `joy >= Full*99/100 - dose joy`, then `ingestDose`. Interrupt: the drink is dropped, or returned to the pack (CSE-037). **Minimum duration:** the job's linger. **Toggle** `RelaxDrink`, on. **Tests:** `TestRelaxDrinkFinishesBeforeFull`, `TestRelaxDrinkRespectsPolicy`, `TestRelaxDoesNotFlap`. **Golden:** moves. |
| CSE-009 | `meditation_economy` (default on). Ends meditation when joy and psyfocus need nothing. | None. | excluded | No psyfocus or meditation system. |

## Drugs

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-010 | `drugs_use_potential_mood` (default on). A scheduled drug with a mood limit is refused if current mood **or** instant mood is at or above it, so a pawn whose mood is recovering does not dose. | `Preset` has `NoDrugs`, `SocialDrinking`, `Unrestricted` (`domain/pawn/drug/policy.go:9`), and `Allows` has no mood limit at all. Mood: `game/social_mood.go:79`; target `game/rooms.go:197` `thoughtLevel`. | missing | No policy has a limit, so there is nothing for the rule to act on. **Proposal:** append `drug.MoodLift`, an opt-in preset that allows social drugs only below mood 50 (ours). **Domain:** `func (p Preset) MoodGate() int` (0 none), with a new check taking `(mood, potential int, usePotential bool)` next to `Allows`. **Package** `game/drugs.go`, one call in `assignDrugUse`. UI: `AssignPresets` (`adapters/inbound/ebiten/windows/assign.go:58`) plus a label. **Toggle** `DrugPotentialMood`, on. **Tests:** `TestMoodGateUsesPotential`, `TestMoodGateCurrentOnlyWhenOff`. **Hysteresis:** the 12-hour dose gap (`policy.go`). **Golden:** none, no colonist starts on the preset. |
| CSE-011 | `ingest_any_drugs` (default on). A pawn may take from its inventory anything it could eat or dose. | No inventory. `nearestDrug` scans `itemIDs` (`game/drugs.go:316`). | missing | **Needs CSE-036.** `assignDrugUse` checks the pack before `nearestDrug`; a dose leaves the pack into hands and runs `job.Ingest`. **Toggle** `AnyDrugFromPack`, on. **Tests:** `TestDoseFromPack`, `TestPackDoseKeepsPolicy`. **Golden:** none alone, since nothing enters a pack without CSE-005 or CSE-029; counted there. |

## Meals and food

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-012 | `separate_meals` (default on). Food with ingredients stacks only when its "bad ingredient" flags match: any human meat is one flag, any ingredient with a negative thought another. | `SameLoad` ignores ingredients (`domain/economy/item/item.go:805`) and `MergeFood` unions them (`:722`), so one tainted meal spreads to a whole stack. Bad thoughts: `game/diet/thoughts.go:29`. | missing | **Domain** `item`: `type MealRule struct{ OddNormal bool }` and `func SameMeal(a, b Stack, r MealRule) bool`, comparing `mealTaint`. Human meat bit always; insect bit unless `OddNormal`. Called with `SameLoad` at its four sites: `game/tick.go:918`, `:1713`, `game/haul/step.go:87`, `game/travel/state.go:393`. **Toggle** `SeparateMeals`, on. **Tests:** `TestSameMealTaint`, `TestTaintedMealStaysApart` (whole game: a tainted and a clean meal on one tile). **Golden:** only if the run cooks human or insect meat; check. |
| CSE-013 | `odd_is_normal` (default off). Insect (low-impact) meat counts as normal for CSE-012. | As CSE-012. | missing | **Toggle** `OddMeatNormal`, off. Same function, same owner. **Test:** table row in `TestSameMealTaint`. **Golden:** none. |
| CSE-014 | `add_meal_ingredients` (default off). Meals not made by a recipe get one random allowed ingredient per recipe input, never human meat, negative-thought ingredients or composites. Recipe products, dispensers and stack splits are kept clean. The dummy recipe in `Recipes_Meals.xml` gives paste a source. | `ItemView.Ingredients` (`game/game.go:330`), `craft/view.go:251` `IngredientNames`, `IngredientsLine` (`adapters/inbound/ebiten/inspector/item_inspector.go:198`). Real ones: `craft/shop.go:369`, `store/paste.go:163`. Non-crafted meals have none (e.g. `domain/world/mapgen/settlement.go:194`). | missing | Neutral ingredients carry no thought, so this is display only here. **Decision:** derive at view time, a pure function of the stack's id and kind, with `splitmix64` from `pkg/mathx`. No state, no spawn hook, no hash change. **Package** `craft/view.go`, plus a neutral-kind table in `domain/economy/recipe`. **Toggle** `FlavourIngredients`, off. **Tests:** `TestFlavourIngredientsStable`, `TestFlavourIngredientsNeutralOnly`. **Golden:** none. |
| CSE-015 | `prefer_spoiling_meals` (default on). Food score gains `(6-2t)^2` when `0<t<3` days to spoiling, at most 36. | Flat `rotSoonBonus` 12 when under half a day (`game/diet/choose.go:21`, `:72`). | partial | **Package** `diet`. Graded bonus under three days, scaled to our score (`optimalBase` 300). Keep the flat bonus when off. New field `Book.Spoil bool` set from the rule on change (values only, no host call in `Score`). **Toggle** `SpoilingMeals`, on. **Tests:** `TestScorePrefersSpoiling`, `TestScoreOffKeepsOldBonus`. **Golden:** moves. |
| CSE-016 | `allow_feeding_with_plants`: pawns feed animals hay; animals prefer non-human food. **Stale:** strings only in 1.6. | Done by FEED-001: tiers `domain/animal/ecology.go:507`, `:512`; trough filter `game/trough_feed.go:42`; `game/animal_needs.go:538` search; hay `ecology.go:474`. | done | Nothing. |

## Cleaning

The shared design, `sense.Tidy`, is in CSE-017; the rows after it are its callers. Dirt lookup: `roomSum` (`game/filth.go:48`) answers "is this room dirty" in O(1), then `cleanQ` (sorted Home tiles at or above `filth.CleanStart`) is walked once, filtered by `g.roomGrid`, stopping at the limit. It runs only at a job start, behind the cooldown.

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-017 | `clean_before_work` (default on), limit `op_clean_num` 5. Before starting a job at a building, or a recreation job, not forced, not from a queue, not while downed or needing tending: list filth in that room, clean it, then do the job. | Cleaning is a single-tile work giver (`game/clean.go:24`, `:87`), Home only. No room-scoped clean. | missing | **Package** `sense`, component `Tidy` (`sense/tidy.go`). **Job** `job.CleanRoom`: `Route` holds cells encoded `EntityID(y*w+x)` in greedy nearest-neighbour order, `RouteI` the next; per-tile work as `doClean`. **Hook:** `Tidy.Before(id, rec) (job.Record, bool)` at the top of `startJob` (`game/tick.go:642`); on true it stores `rec` in `Resume` and starts the chain. Kinds: `Research, Bill, Sleep, Recreate, WatchTelevision, Read, Warden, OperateDrill, ScanGround, ScanMinerals, UseComms` (`tidyBefore`, the implementer confirms the list). Excludes `Forced`, `Construct` and `CleanRoom` itself. **Cooldown:** `TidyAt[id] = now + 1500` (ours), set before the chain and before resuming. **Save:** `Save.Resume`, `Save.TidyAt`. **Toggle** `CleanBeforeWork`, on. **Tests:** `TestTidyCleansRoomThenResumes`, `TestTidySkipsForced`, `TestTidyDoesNotFlap`, `TestTidyResumeSurvivesSave`, benchmark `BenchmarkStartJobTidyGate`. **Golden:** moves. |
| CSE-018 | `adv_cleaning` (default on), limit `adv_clean_num` 5. Between hauling the ingredients and crafting: list filth around the bench (respecting CSE-019), skip forbidden and non-Home tiles, clean, go to the bench, work. | `Shop.gather` hands over to a plain bill job (`game/craft/haul.go:27`); `Shop.Do` works (`game/craft/shop.go:189`). Cleanliness already affects benches (`Workshop.Cleanliness`, `craft/deps.go:178`). | missing | **Package** `craft` plus `sense`. New role `craft.Prep` (`Benches`, `Workers`, `Crafters` are full, `craft/AGENTS.md`): `TidyBench(id, bench entity.EntityID, j job.Record) bool`. Called once when the last load lands. On true the bill is the `Resume`. Laid ingredients stay on the bench and keep their claims, re-taken by `Shop` on resume (test covers it). **Toggle** `CleanBeforeBill`, on. **Tests:** `TestBillCleansBeforeWork`, `TestBillCleanRespectsLimit`, `TestBillCleanDoesNotFlap`. **Golden:** moves. |
| CSE-019 | `clean_gizmo` (default off), `CleanGizmo.xml`, `DoCleanComp`. Benches with bills get a clean / do not clean toggle, on per bench by default, saved per building. CSE-017 and CSE-018 respect it. | Nothing. Button slot: `adapters/inbound/ebiten/inspector/cmds_build.go:59`. | missing | **Command** `SetBenchClean` (Target, Enabled). **Save:** `Save.NoClean []entity.EntityID`, sorted, pruned when a building is gone. **View:** `BuildingView.CleanOff bool`. **Button** shown only when the rule is on and the building takes bills. **Toggle** `BenchCleanToggle`, off. **Tests:** `TestBenchCleanCommand`, `TestBenchCleanPruned`, journey `TestJourneyBenchClean`. **Golden:** none. |
| CSE-020 | `adv_cleaning_ingest` (default off). Before eating, and before social relax, reserve the seat, clean around it, skipping forbidden and non-Home tiles, then sit. Limit `adv_clean_num`. Only if capable of cleaning. | `carryToTable` (`game/dining.go:41`), seat `game/rest/seats.go:70`; `idleSocialRelax` (`game/idle.go:194`). | missing | **Package** `sense`. `Tidy.Before` also accepts `job.Eat` and `job.SocialRelax` under this toggle, for the seat's room. **Deviation from the mod:** the chain runs before the food is fetched, so the meal is not carried while cleaning; the claim is released and retaken on resume, and a lost claim just means choosing again. Skipped when hunger is at or below `needs.RavenousAt`. **Toggle** `CleanBeforeMeal`, off. **Tests:** `TestCleanBeforeMealOff`, `TestCleanBeforeMealResumes`, `TestStarvingSkipsClean`. **Golden:** none. |
| CSE-021 | `clean_after_tending` (default on), limit `doc_clean_num` 0 (unlimited). After a successful tend with nothing queued: if the next job would tend a patient in this room, or a patient bleeding to death within 6 hours, do not clean; else clean the room. | `Doctor.DoTend` ends at `game/care/tend.go:190`. `BleedPerDay` role (`care/deps.go:42`). | missing | **Package** `care` plus `sense`. `Doctor.Urgent(id entity.EntityID) bool` (read only, on tend end only): another tendable patient in this room, or bleeding out within 6 hours. Hook at the job-end seam (`game/tick.go:478`): `Tidy.After(id, j)`. **Toggle** `CleanAfterTend`, on. **Tests:** `TestCleanAfterTend`, `TestNoCleanWhenPatientBleeding`, `TestNoCleanWithNextPatientHere`. Cooldown shared with CSE-017. **Golden:** moves. |
| CSE-022 | `clean_before_work`, end hook. After delivering food successfully, clean the target's room, limit `op_clean_num`. | `job.DeliverFood`, `job.FeedPatient` (`game/care/feed.go:188`). | missing | `Tidy.After` for those kinds. Same toggle `CleanBeforeWork`. **Tests:** `TestCleanAfterDelivery`. **Golden:** moves. |
| CSE-023 | Three limits: `op_clean_num` 5, `adv_clean_num` 5, `doc_clean_num` 0 (0 means unlimited). | None. | missing | `gameplay.Limit` values `TidyStation`, `TidyBench`, `TidyTend`, range 0..50. "Unlimited" is capped at `maxTidyCells` 64 (ours) so a `Route` stays bounded. **Command** `SetGameplayLimit`. Stepper rows on Habits. **Test:** `TestLimitsRoundTrip`, `TestUnlimitedIsBounded`. **Golden:** none alone. |
| CSE-024 | `add_to_que`: the original task is queued behind the opportunistic one, so the pawn does not change its mind. **Stale:** the setting is gone in 1.6 and the behaviour always applies. | No per-pawn job queue. `OrderQueue` holds forced orders only (`game/order_queue.go`). | missing | `Resume` in `sense`, no toggle (matches 1.6). Started at the job-end seam `game/tick.go:478`; dropped in `interrupt` (`:417`) and when a forced order starts. **Tests:** `TestResumeDroppedOnInterrupt`, `TestResumeFailsQuietly` (a stale target fails through `startJob`). **Golden:** with CSE-017. |
| CSE-025 | `CleanOnOpportunity.xml`: always clean before recruiting, converting, enslaving, spectating a ceremony; never before placing or finishing a frame. | Warden job `job.Warden`; ceremonies `job.Gather`; frames `job.Construct`. No convert or enslave system seen. | missing | The `tidyBefore` table: `Warden` and `Gather` always; `Construct` never. Convert and enslave are excluded for lack of a system. No extra toggle. **Test:** table test `TestTidyKinds`. **Golden:** with CSE-017. |
| CSE-026 | Who and what: not for non-tool-users, non-colonists, cleaning disabled or at priority off, mental state, burning. Filth: in a huge room (over 160 cells) a radial 200 cells around the target; else the room plus doors, in the allowed area, nearest first. | `assignClean` is Home, nearest, 16 probes (`game/clean.go:24`). `g.roomGrid`, `roomSum`. Work state: `game/priorities.go:62`. | partial | `Tidy.capable(id)`: colonist, `work.Clean` priority above 0, not breaking, not burning. `Tidy.list(room, limit)` as in the section head. A room of more than 160 cells limits by distance. Doors: not walked by `roomGrid`; **unverified** whether a door cell has its own room id, so door filth may be missed, acceptable. **Tests:** `TestTidyListBounded`, `TestTidyHugeRoomUsesRadius`, `TestIncapableNeverTidies`. |
| CSE-027 | `calculate_full_path` (default on). With a fetch target, skip the clean when the detour is unreasonable: by real path costs when `start-to-fetch > 500` and `start-to-target / (start-to-fetch + fetch-to-target) < 0.7`; by straight lines with 10 when off. A failed path skips. | `pathTo` (`game/tick.go:712`), search budget `game/paths.go:69`. `pathing.Dist` for straight lines. | missing | `Tidy.detour`: on, compute with `pathTo` lengths (ours has no cost return; length stands in, up to 3 searches), and if the search budget is spent, skip the clean rather than search. Off: `pathing.Dist`. **Toggle** `FullPathCheck`, on. **Tests:** `TestDetourSkipsFarFetch`, `TestDetourDeferredSkips`. **Benchmark:** `BenchmarkTidyDetour`. **Risk:** three path searches at job start; gate after `roomSum` and the cooldown so it is rare. |
| CSE-028 | `skip_snow_clean` (default on). No clearing while snow, rain or sand is falling. | `Layer.AssignClear` (`game/snow/clear.go:115`), called `game/priorities.go:99`, ignores the sky. `weather.Kind.Falling` (`domain/world/weather/weather.go:35`). Sand excluded, none exists. | missing | **Package** `snow`. Add `SkipClear() bool` to role `Sky` (`snow/deps.go:23`); adapter returns rule and `Falling()`. Checked first in `AssignClear`. **Minimum duration:** a weather block is 6 hours, `ClearStart` hysteresis stays. **Toggle** `SnowWaitsForSky`, on. **Tests:** `TestClearWaitsForClearSky`, `TestClearSnowNoFlap` extended. **Golden:** moves where snow falls. |

## Bills and ingredients

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-029 | `adv_haul_all_ings` (default on, player humanlikes). Before crafting, walk to each ingredient and pick it up into inventory, a timed pickup `max(30, count*mass/10)` ticks with a progress bar, marked to unload if left over. Then at the bench, take each to hands and place it. | `job.HaulIngredients`: gathers by hand load, one kind at a time, several trips (`game/craft/haul.go:27`, `:73`, `:98`, `:119`; `game/craft/shop.go:134`). | partial | **Needs CSE-036.** **Package** `craft`, `craft/packgather.go`, new role `craft.Packs`: `PackRoom(id) int`, `Stow(id, stack entity.EntityID, n int) bool`, `Unstow(id entity.EntityID, kind item.Kind) (entity.EntityID, bool)`. **Job** `job.PackIngredients`: picks from `planner.plan`, then the existing `putDown` per load. Kept as a new kind so `HaulIngredients` remains the fallback. **Toggle** `HaulAllIngredients`, on. **Tests:** `TestPackGatherOneTrip`, `TestPackGatherInterruptedKeepsPack`, `TestPackGatherFailsWhenTargetGone`. **Golden:** moves; expect a large diff in ticks per bill. |
| CSE-030 | `adv_respect_capacity` (default on). Stop collecting when gear, inventory and the next item exceed capacity; deliver; come back for the rest. | No capacity. `item.CarryLimit` 100 units (`domain/economy/item/item.go:618`). `MassGrams` (`item/mass.go`, AX-9). | missing | `Pack.Grams()` and `sense.Pack.Capacity(id)`: 40 kg times Manipulation percent (ours; see Open questions). `PackIngredients` keeps its remaining picks in `Route` and loops. **Toggle** `RespectCapacity`, on. **Tests:** `TestPackGatherSplitsByCapacity`, `TestCapacityFollowsManipulation`. **Golden:** with CSE-029. |
| CSE-031 | `adv_haul_all_ings` also orders the picks by greedy nearest neighbour from the pawn. | Candidates sort by distance to the bench, then id (`game/craft/gather.go:85`); grouped by kind (`:166`). | partial | `orderPicks` in `craft/packgather.go`; deterministic ties by id. Applies to `PackIngredients` only. **Test:** `TestOrderPicksNearestNeighbour`. **Golden:** with CSE-029. |
| CSE-032 | At the bench each ingredient goes on its own free cell, never stacked (keeps material order), and the job fails if any queued target has gone. | Loads are laid on the bench tile with `putDown`, listed in `Route` (`game/craft/haul.go:27`); `laidOut` fails a bill when one has left (`game/craft/shop.go:227`). | partial | Our loads are tracked by id in `Route`, so stacking cannot lose the order, and the failure check exists. No new behaviour needed unless a merge on the tile breaks `laidOut`; add `TestLaidLoadsKeepOrder` to prove it. No toggle. **Golden:** none unless the test finds a bug. |
| CSE-033 | `prefer_spoiling_ingredients` (default on, not for bills on pawns). A rottable candidate brings its whole store into the comparison; corpses and materials count too. The mod warns of lag. | `planner.plan` reads every candidate within radius, sorted by distance to the work spot then id (`game/craft/gather.go:68`, `:85`). Rot: `Stack.FreshTicks`, `Shelf` (`item/item.go:743`). | missing | **Package** `craft`, `gather.go`. New sort key before distance: whole hours to spoiling among rottable, under a horizon of 3 days (ours), then distance. The plan already compares all candidates, so no lag and no store rule. Not for surgery recipes. **Rule passed as a value** through the new `Prep` role. **Toggle** `SpoilingIngredients`, on. **Tests:** `TestPlanPrefersSpoiling`, `TestPlanIgnoresRotForSurgery`, `TestPlanOffKeepsNearest`. **Golden:** moves. |
| CSE-034 | Allow-mix search (not recipes adding a condition): medical potency descending, then rottable first, then hours to spoiling. | Medicine picked by the doctor flow (`game/care/tend.go:128`); bills have no potency order. | missing | Second key in the same sort: for inputs where potency applies, use `item.Kind` medicine tier. **Unverified:** whether any of our recipes takes mixed medicine; if none, this row closes with CSE-033 alone. Same toggle. **Test:** `TestPlanPotencyFirst`. |
| CSE-035 | `hauling_over_bills` (default on). Before a bill, for a stackable queued ingredient outside the bench's room (or when the bench is outdoors, huge, or over 160 cells): if its stockpile destination is nearer the pawn than the bench is, haul the whole stack there first, then do the bill. | `haulToStockpile(id, pos, ids, dest)` takes a list of ids (`game/tick.go:833`), so one stack works. `Shop.start` (`game/craft/shop.go:143`). | missing | **Package** `sense` with `craft.Prep.StackFirst(id, picks) bool`. On true, the stockpile haul runs through a one-element `ids` and the bill is the `Resume`. Pawn must be able to haul. Distances are squared straight-line (`pathing.Dist`), so no search. **Cooldown:** `Save.HaulAt`, 1,500 ticks (ours). **Toggle** `StackToStockpile`, on. **Tests:** `TestBillHaulsWholeStackFirst`, `TestBillSkipsHaulWhenBenchNearer`, `TestStackFirstDoesNotFlap`. **Golden:** moves. |

## Inventory

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-036 | Base game's pawn inventory, which the mod assumes. | `domain/pawn/inventory/slot.go`, one spare weapon. `Stack.Carrier` is one job's hand load. | missing | **Domain** `domain/pawn/inventory/pack.go`, see contract. **Package** `sense`, component `Packs`: `Stow`, `Unstow` (materialises a carried stack), `Rot` (batched, every 250 ticks by id stagger, `item.RotRate` at the pawn's temperature as `craft/condition.go:50`), `Spill` on death, capture and removal, `Wealth`. **Hooks:** `Game.Wealth` (`game/raids.go:368`), death and removal sites (implementer lists them; travel keeps the pack, as the value is on `worldState`). **View:** `PawnView.Pack []PackItemView`, `PackGrams`, `PackLimitGrams`. **Gear tab** lists it (`adapters/inbound/ebiten/inspector/gear_ui.go:110`). **Save:** `Save.Packs`. **Tests:** `TestPackSpillsOnDeath`, `TestPackRots`, `TestPackTravelsWithPawn`, `TestPackInWealth`, `TestPackRoundTrip`. **Golden:** none alone. |
| CSE-037 | `put_back_to_inv` (default on). On job cleanup, a carried thing that came from the inventory returns there instead of dropping. | `dropCarry` always drops (`game/tick.go:435`), called from `interrupt` (`:422`) and job end (`:483`). | missing | `sense.Packs.BackToPack(id, carry entity.EntityID) bool` at the top of `dropCarry`; `Save.FromPack` records which carried stack left the pack. Cleared on landing. **Toggle** `PutBack`, on. **Tests:** `TestInterruptReturnsToPack`, `TestGroundLoadStillDrops`. **Golden:** check; a pack is needed first. |
| CSE-038 | `gui_manual_unload` (default on). A button per row of the Gear tab (inventory, weapon, apparel), with Unload and Cancel. Shown if the player controls the pawn and it is a colonist or away from home, not quest-locked or locked apparel. Cancel while over capacity drops the item through the normal drop. | `gearTabBody` (`adapters/inbound/ebiten/inspector/gear_ui.go:110`) draws lines with no row buttons. `command.DropWeapon` exists (`domain/sim/command/command.go:277`). | missing | **Command** `MarkUnload`: Pawn, Mode (0 pack, 1 apparel, 2 weapon, 3 spare slot), Index (`Entry.ID`, or the slot), Enabled. **UI** in `inspector`, a new `inspector/gear_rows.go`: 24 px button per row, tooltip by state. Quest locks do not exist here; dropped. **Toggle** `ManualUnload`, on (hides the buttons). **Tests:** `TestMarkUnloadCommand`, journey `TestJourneyGearUnload`, `TestCancelOverCapacityDrops`. **Golden:** none until pressed. |
| CSE-039 | Job `UnloadMarkedItems`: wait 10 ticks; find a store cell near the colony, or drop nearby; walk; for apparel wait to take it off, for a weapon 30 ticks; move to inventory, then hands; place; clear the mark. Fails if the mark was removed. Marks clear when an item is dropped; entering inventory sets "was in inventory". | `haulToStockpile` for destinations. `Spare` slot `game/combat/draft.go:71`. | missing | **Job** `job.UnloadPack` in `sense`. Takes a marked `Entry` to hands, then the haul machinery places it; no stockpile: drop next to the pawn. Apparel and weapon reuse their existing take-off or drop path (implementer finds them; only `DropWeapon` verified). Checked in `phaseAI` before work, one bit test when no mark exists. **Cooldown:** failed search waits 500 ticks. **Tests:** `TestUnloadStoresMarked`, `TestUnloadCancelStops`, `TestUnloadNoStoreDrops`, `TestUnloadDoesNotFlap`. **Golden:** with CSE-040. |
| CSE-040 | Leftover ingredients picked up by CSE-029 are marked, so they are unloaded after the bill. | None. | missing | `Entry.Unload = true` when `PackIngredients` stows, cleared when used at the bench. Hauled by CSE-039 with no button press, with no extra toggle. **Test:** `TestLeftoversUnloadAfterBill`. **Golden:** moves. |

## Wandering and rooms

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-041 | `safe_wander` (default off). A player pawn on its home map, with no lord duty, and with weather not enjoyable outside, or fallout and no full toxic resistance, or vacuum: if under open sky, jog to the closest Home roofed safe cell (region limit 100); if roofed, walk at most a random 400..800 path cost, never more than 200 under open sky, ending at the last roofed cell, not a door; stay put if nowhere is safe. | `idleWander` (`game/idle.go:109`) has no weather check. `soaking` (`game/weather.go:81`), `incident.Fallout` (`domain/incident/sky.go:65`), `outdoorsAt` (`game/weather.go:55`). | missing | **Package** `sense`, component `Wander`; `idleWander` moved in (pays budget). `ShouldHide` from `soaking` and `Fallout`. Candidates limited to roofed Home cells, then the path's open-sky run checked (paths at radius 5 to 9 are short). Unroofed pawn: spiral to radius 12 for a roofed cell, not a region flood. Cost bound from a hash of tick and id. Jog: **unverified**, no urgency field on `job.Record`; walk normally unless the implementer finds one. Vacuum excluded. **Hysteresis:** 6-hour weather blocks, plus retry on the idle recheck. **Toggle** `ShelterWander`, off. **Tests:** `TestWanderStaysRoofedInRain`, `TestWanderSeeksRoofInFallout`, `TestWanderOffDrawsSameNumbers`, `TestShelterDoesNotFlap`. **Golden:** none. |
| CSE-042 | `polite_wander` (default off; on load it takes the value of `safe_wander`). Non-hostile pawns refuse wander cells in a bedroom they do not own, a hospital, a prison cell, prison barracks, a cooled store room, or a containment cell. Plain barracks stay allowed. | Same `idleWander`: rejects bed-covered, fire-blocked, out-of-area, reserved, unreachable. Roles: `domain/world/room/room.go:78`, role from `game/rest_host.go:163`. | missing | Same component. `Wander.polite(id, x, y)`: role of `g.roomGrid` room; bed ownership from the `rest` beds. Guests: **unverified**, the visit package has no wander of its own by grep; the check covers those passing `idleWander`. Containment excluded. **Toggle** `PoliteWander`, off (own default; no coupling to the other). **Tests:** `TestWanderAvoidsOthersBedroom`, `TestWanderOwnBedroomOK`, `TestWanderAvoidsCell`. **Golden:** none. |
| CSE-043 | `RoomRoles.xml`, cooled store room: storeroom score plus one per cooler under 0 °C whose cold side is in this room. Used by CSE-042. | No store role. Coolers: `Thermostat` (`game/thermo/deps.go:54`), cold side in `site.room` (`game/thermo/cache.go:39`). Role functions: `domain/world/room/stats.go:20`, `tally.go:217`. | missing | **Domain** append `room.ColdStore` last. Given by `FullContents.ColdSides > 0` with no beds, only when the role would otherwise be General. **Package** `thermo`: `ColdRooms()` from the site cache, `FullContents` filled in `game/rest_host.go:163` and `game/rooms.go:148`. Label in `RoomView`. **Tests:** `TestRoleColdStore`, `TestRoleColdStoreWithBedIsBedroom`. **Golden:** none (`rest/room_mood.go:45` treats it as General; verify). No toggle: a name, not a behaviour. |
| CSE-044 | Containment cell role and vacuum in the wander rules. | None. | excluded | Anomaly and Odyssey content; no such room or map state. |

## Mood

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-045 | `mood_regen` (default off). While mood is frozen asleep, each interval it moves toward instant mood, up to 50 % from below and down to 50 % from above. | Mood is not frozen: `stepMood` runs for every living pawn every `moodPeriod` with no sleep check (`game/social.go:56`, `game/social_mood.go:79`). | done | Nothing, and no toggle. Our mood already follows its target asleep, all the way rather than to 50 %. Adding a toggle would need a sleep freeze we do not have. |

## Text and screens

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-046 | `gui_extended_recipe` (default on). A recipe's ingredient summary names the categories, or stuff categories, or allowed items, or thing labels, instead of a vague word. | `Input.Label` gives "raw food", "plant food", "any stone blocks" (`domain/economy/recipe/recipe.go:601`), joined by `InputsLabel` (`:630`) into `RecipeView.Input` (`game/craft/view.go:21`). The bill's own panel lists each allowed kind as a chip (`adapters/inbound/ebiten/inspector/bill_config.go:277`); the recipe list does not. | partial | **Domain** `func (in Input) Kinds() []item.Kind`, with the same filters `planner` uses. **Read model** `RecipeView.InputKinds []string`. **UI** in `inspector`: label or tooltip on the add-bill row. **Toggle** `FullRecipeList`, on. **Tests:** `TestInputKindsMatchPlanner`, `TestRecipeRowNamesKinds`. **Golden:** none. |
| CSE-047 | `TextChanges`: repairs the base game's filter so "who can make" records its list. | Inputs are flat fields, no such filter (`recipe.go:230`). | excluded | No such filter in our recipes. |
| CSE-048 | Always on, no toggle. Item info gets a Category row, "child \\ parent", with all categories in the tooltip. | `Kind.Category` (`domain/economy/item/category.go:10`), `ItemView.Category` (`game/game.go:357`), header word `ItemCategory` (`inspector/item_inspector.go:32`). Our categories are flat, one per kind. | done | Nothing: flat, so no path or list to show. |
| CSE-049 | `Settings.cs`: the mod's options page, one row per setting. | Options `Story` tab precedent. Nothing for this. | missing | **WP1.** Tab Habits: one check row per toggle with one original sentence of help, stepper rows for limits, grouped as these tables are, and a "Back to defaults" button. Shows `WorldView.Gameplay`, sends the commands. **Tests:** `TestHabitsRowsCoverEveryToggle` (walks `gameplay.AllToggles`), journey `TestJourneyHabitsToggle`, `TestJourneyHabitsLimit`. |

## Mod plumbing and other mods

| Id | Mod feature, exactly | What we have today | Status | Work needed |
|---|---|---|---|---|
| CSE-050 | `optimal_patching` (default off). A restart-only skip of runtime patches for disabled options. | No runtime patching. | excluded | No patches to skip; a toggle here is a plain branch. |
| CSE-051 | `SideArmsPatch`: tells the SimpleSidearms mod that a weapon was unloaded. | No sidearm mod. Spare slot exists and gets CSE-038's button. | excluded | Other mod; no integration needed. |
| CSE-052 | `PatchBadHy`: toilet, wash and drink priorities for the Dubs hygiene mod under `fun_police`. | No hygiene need. | excluded | No hygiene system. |
| CSE-053 | `PatchRPG`: Sandy RPG and AwesomeInventory integration, a slot icon for marked items. | Neither exists. | excluded | Other mods. |
| CSE-054 | `Recipes_Meals.xml`, dummy recipe giving paste a raw-food source for CSE-014. | No def system; paste has real ingredients (`game/store/paste.go:163`). | excluded | Not needed: CSE-014 has its own table and skips paste. |
| CSE-055 | Falls back to vanilla behaviour when a patch throws. | Go, no patches. | excluded | No patch layer. |

## Every file in the 1.6 folder

| Mod file | Rows |
|---|---|
| `CleanOnOpportunity.cs`, `1.6__Patches__CleanOnOpportunity.xml` | CSE-025 |
| `CommonSense.cs` | CSE-050 |
| `CravingForOutdoors.cs` | CSE-001 |
| `DoCleanComp.cs`, `1.6__Patches__CleanGizmo.xml` | CSE-019 |
| `DrugPolicy.cs` | CSE-010 |
| `FoodCountLimiter.cs` | CSE-006 |
| `IngredientPriority.cs` | CSE-015, 031, 033, 034 |
| `JobDriver_IngestPatches.cs` | CSE-020 |
| `JobDriver_MeditatePatches.cs` | CSE-009 |
| `JobDriver_SocialRelaxPatches.cs`, `1.6__Defs__Jobs_Misc.xml` (relax job) | CSE-008, 020 |
| `JobGiver_DoBIll_Rework.cs` | CSE-018, 029, 030, 032, 040 |
| `JoyPriority.cs`, `1.6__Patches__FunPolice.xml` | CSE-002, 003 |
| `MealStacking.cs` | CSE-012, 013 |
| `Need_MoodPatches.cs` | CSE-045 |
| `OpportunisticTasks.cs` | CSE-017, 021, 022, 024, 027, 035 |
| `PatchBadHy.cs` | CSE-052 |
| `PatchRPG.cs` | CSE-053 |
| `PointlessWanderPatches.cs` | CSE-041, 042, 044 |
| `PutBackToBackpack.cs` | CSE-037 |
| `RandomIngredients.cs`, `1.6__Defs__Recipes_Meals.xml` | CSE-014, 054 |
| `RoomRoleWorker_CooledStoreRoom.cs`, `1.6__Defs__RoomRoles.xml` | CSE-043 |
| `Settings.cs` | CSE-049, and each toggle's row; stale keys CSE-016, 024 |
| `SideArmsPatch.cs` | CSE-051 |
| `TakeAnyDrugs.cs` | CSE-011 |
| `TextChanges.cs` | CSE-046, 047, 048 |
| `UnloadChecker.cs`, `1.6__Defs__Jobs_Misc.xml` (unload job) | CSE-038, 039 |
| `Utility.cs` | CSE-026, 031, 041 |
| `WorkGiver_ClearSnow.cs` | CSE-028 |
| `WorkGiver_VisitSickPawnPatches.cs`, `1.6__Defs__JoyGivers.xml` | CSE-007 |
| `1.6__Patches__FoodOptimality.xml`, `1.6__Patches__PackMeal.xml` | CSE-005 |
| Workshop description, "sleeping assignment as recreation" | CSE-004 |
| Workshop description, error fall-back | CSE-055 |
| `1.6__Assemblies__CommonSense.dll` | Compiled form of the above; used only to confirm the stale settings. |

## Size check

Checked against `internal/arch/solid_budget.txt` and `.cursor/SOLID_GO.md` §8, §10:

| Line | Budget | Consequence |
|---|---|---|
| `files game` | 176 (176 used) | No new file in `game/`. The host goes in `game/clean.go`, whose gameplay code moves out; it is renamed `sense_host.go` in the same change, one file for one. |
| `lines game` | 53224 (53224 used) | Every line added must be paid. Known payment: `game/clean.go` body (133 lines, its code moves to `sense/clean.go`) and `idleWander` (`game/idle.go:109`, 26 lines). That is about 150 lines; hooks and adapters will cost more, likely 300 or so (an estimate). **WP-P must find the rest**, e.g. the recreational half of `game/drugs.go` (477 lines), and prove each move hash-identical first. |
| `methods game.Game` | 1837 | No new `Game` method. `senseOf(g)` is a free function; adapters are small types. Moved methods lower the line, and the lead lowers the budget. |
| `fields game.Game` | 64 | No new field. `sense` hangs on `worldState` (a struct with no methods of its own in `world_state.go`). |
| `item.Stack` | 24 of 25 fields | No field added. |
| `sense` | new | ≤ 40 methods a type, ≤ 8 an interface, ≤ 12k lines. Each feature has its own component; nothing sits on the root but `Do`, `Save`, `Load` and `Rules`. |
| `inspector` | 7,318 lines, under limit | `AGENTS.md` says split by kind before a seventh tab type; a new file for gear rows is fine. |

## Proposed split

### Parallel workers

Rules: `.cursor/PARALLEL_WORK.md`. Everyone works on `main` in the shared checkout, with disjoint files. Hooks in shared files are added by WP0 as inert calls, so feature agents never edit `game/tick.go`, `game/save.go` or `command.go`. Models are a suggestion; the lead decides.

| Wave | Packages | Gate |
|---|---|---|
| 0 | WP-P payer, then WP0 contract | `make arch`; `bin/benchmark -hash` identical; `TestSaveSchema` regenerated; `TestSaveV12Fixture` passes |
| 1 | WP1 screen, WP2 pack | Journey tests; hash identical with defaults |
| 2 | WP3 tidy, WP4 bills, WP5 pack habits, WP6 food, WP7 joy and drugs, WP8 wander and sky, WP9 text | Package tests with `-race`; no-flap tests |
| 3 | Lead | Re-pin goldens with reasons; lower budgets; checklists; doc index; handoff |

Order constraints: WP2 needs WP0. WP5 and WP4's `PackIngredients` need WP2. WP4's bench tidy call needs WP3's `Tidy` signature, which WP0 fixes in the contract. WP7's pack-first gift and dose need WP2. The rest are independent.

### Contract (WP0 writes it exactly; nobody else changes it)

`internal/domain/gameplay/settings.go` (new; domain, headless):

```go
type Toggle uint8

const (
    OutdoorsJoy Toggle = iota // CSE-001, on
    JoyTopOff                 // CSE-002, on
    SpareMeal                 // CSE-005, on
    SickGifts                 // CSE-007, on
    RelaxDrink                // CSE-008, on
    DrugPotentialMood         // CSE-010, on
    AnyDrugFromPack           // CSE-011, on
    SeparateMeals             // CSE-012, on
    OddMeatNormal             // CSE-013, off
    FlavourIngredients        // CSE-014, off
    SpoilingMeals             // CSE-015, on
    CleanBeforeWork           // CSE-017, CSE-022, on
    CleanBeforeBill           // CSE-018, on
    BenchCleanToggle          // CSE-019, off
    CleanBeforeMeal           // CSE-020, off
    CleanAfterTend            // CSE-021, on
    FullPathCheck             // CSE-027, on
    SnowWaitsForSky           // CSE-028, on
    HaulAllIngredients        // CSE-029, on
    RespectCapacity           // CSE-030, on
    SpoilingIngredients       // CSE-033, on
    StackToStockpile          // CSE-035, on
    PutBack                   // CSE-037, on
    ManualUnload              // CSE-038, on
    ShelterWander             // CSE-041, off
    PoliteWander              // CSE-042, off
    FullRecipeList            // CSE-046, on
    NumToggles
)

type Limit uint8

const (
    TidyStation Limit = iota // CSE-023, default 5
    TidyBench                // default 5
    TidyTend                 // default 0, unlimited
    NumLimits
)

// Settings holds differences from the defaults: the zero value is the
// defaults, and an older save decodes to it.
type Settings struct {
    Flipped uint64            // bit t: toggle t differs from its default
    Limits  [NumLimits]uint8  // 0: default; else the value plus one
}

func (s Settings) On(t Toggle) bool
func (s Settings) With(t Toggle, on bool) Settings
func (s Settings) LimitOf(l Limit) int
func (s Settings) WithLimit(l Limit, n int) Settings // clamped to 0..50
func AllToggles() []Toggle
func ToggleDefault(t Toggle) bool
```

`domain/sim/command/command.go`, appended after the last kind:

| Kind | Fields | Effect |
|---|---|---|
| `SetGameplayToggle` | `Index` (Toggle), `Enabled` | `Rules = Rules.With(...)` |
| `SetGameplayLimit` | `Index` (Limit), `Count` | `Rules = Rules.WithLimit(...)` |
| `SetBenchClean` | `Target` (building), `Enabled` | CSE-019 |
| `MarkUnload` | `Pawn`, `Mode`, `Index`, `Enabled` | CSE-038 |

`domain/ai/job/job.go`, appended after the last kind (`NetTank` today; saved values, never reordered): `CleanRoom`, `PackIngredients`, `UnloadPack`, `TakeSpare`. None is `IsIdle`.

`domain/pawn/inventory/pack.go` (WP0 writes the types, WP2 the methods):

```go
type Entry struct {
    ID     uint32     // stable within this pack, for commands
    Stack  item.Stack // off the map: X, Y, Carrier are zero
    Unload bool
}

type Pack struct {
    Items []Entry
    Next  uint32
}

func (p *Pack) Add(st item.Stack) uint32 // joins a SameLoad entry
func (p *Pack) Take(id uint32, n int) (item.Stack, bool)
func (p Pack) Grams() int                // item mass times quantity
func (p Pack) Find(k item.Kind) (uint32, bool)
```

`game/sense/save.go`, saved as `saveState.Sense *sense.Save` (top level, colony-wide; nil loads as defaults):

```go
type Save struct {
    Rules    gameplay.Settings
    Resume   []ResumeEntry     // {Pawn entity.EntityID; Job job.Record}, by pawn
    TidyAt   []TickEntry       // {Pawn; Until uint64}, by pawn
    HaulAt   []TickEntry       // CSE-035's cooldown
    NoClean  []entity.EntityID // sorted
    TopOff   []entity.EntityID // sorted
    Packs    []PackEntry       // {Pawn; Pack inventory.Pack}, by pawn
    FromPack []CarryEntry      // {Pawn; Item entity.EntityID}, by pawn
    SpareAt  []TickEntry
    UnloadAt []TickEntry
}
```

`recreate.Save` also gains `Craving []entity.EntityID` (WP7). No field is renamed or removed; no `SaveVersion` bump.

Read models (WP0): `WorldView.Gameplay gameplay.Settings`; `BuildingView.CleanOff bool`; `PawnView.Pack []PackItemView{ID uint32; Name string; Qty int; Unload bool}`, `PawnView.PackGrams int`, `PawnView.PackLimitGrams int`; `RecipeView.InputKinds []string`.

Inert hooks (WP0 adds them; each returns the do-nothing answer until its WP lands):

| Site | Call |
|---|---|
| `game/tick.go:642` `startJob` | `if r, ok := senseOf(g).Tidy.Before(id, rec); ok { rec = r }` |
| `game/tick.go:478` job end | `senseOf(g).Tidy.After(id, j)`, then Resume |
| `game/tick.go:417` `interrupt` | `senseOf(g).Tidy.Forget(id)` |
| `game/tick.go:435` `dropCarry` | `if senseOf(g).Packs.BackToPack(id, j.Carry) { ... }` |
| `game/tick.go:606` free time | `senseOf(g).TopOff.Wants(id, g.joy[id])` |
| `game/tick.go:1052` `phaseJobs` | one case for the four new kinds: `senseOf(g).Do(id, j)`; `PackIngredients` goes to `craft` |
| `game/idle.go:109` | `idleWander` moved: `senseOf(g).Wander.Pick(id, linger)` |
| `game/tick.go:61` `applyOne` | the four commands to `senseOf(g).Apply` |
| `game/save.go:166`, load | `Sense` export and import |

Consumer roles (`game/sense/deps.go`, each ≤ 8 methods, values only on tick paths; adapters in `game/sense_host.go`): `Bodies`, `Dirt` (`RoomDirt(room) int`, `DirtCells(room, limit, buf) []int`), `Rooms` (role and owner), `Sky`, `Starter` (`StartJob`, `Job`, `SetJob`), `Stock` (stack read, spawn carried, delete), `Workers`.

`craft/deps.go` gains `Prep` and `Packs`; `care/care.go` `Roles` gains `Gifts`; `snow/deps.go` `Sky` gains `SkipClear`; `recreate/deps.go` `Rooms` gains `Outside`, `Feelings` gains `OutdoorsPct`. WP0 declares each with a do-nothing adapter so the build stays green.

### Work packages

| WP | Rows | Model | Files owned | Forbidden | Tests | Needs |
|---|---|---|---|---|---|---|
| WP-P payer | Budget | sonnet | `game/clean.go` to `game/sense_host.go`, `game/sense/clean.go`, moved functions | Behaviour changes | Hash identical; existing cleaning tests pass unchanged | none |
| WP0 contract | All | opus | `domain/gameplay/*`, `domain/pawn/inventory/pack.go` (types), appends in `command.go`, `job.go`, `game/tick.go`, `game/save.go`, `game/testdata/save_schema.txt`, `game/sense/{deps,sense,save}.go`, read-model fields | Any behaviour | `TestSettingsZeroIsDefaults`, `TestToggleDefaultsMatchDoc`, `TestSaveSchema`, hash identical | WP-P |
| WP1 screen | 049, 023 | sonnet | `options/habits_tab.go`, `options/deps.go`, `screen_options.go` tab list, `ebiten/options_compat`, journey file | `game/` | Listed under CSE-049 | WP0 |
| WP2 pack | 036 | opus | `domain/pawn/inventory/pack*.go`, `game/sense/pack*.go`, gear list in `inspector/gear_rows.go` | Bills, eating | Listed under CSE-036 | WP0 |
| WP3 tidy | 017 to 027 except 023's UI | opus | `game/sense/tidy*.go`, `game/sense/clean.go`, `game/care/tend.go` (`Urgent`), bench button in `inspector/cmds_build.go` | `craft/*` | Listed per row | WP0 |
| WP4 bills | 018 call, 029 to 035, 040 mark | opus | `game/craft/gather.go`, `haul.go`, `shop.go`, `packgather.go`, `craft/deps.go` | `sense/tidy*`, `sense/pack*` | Listed per row | WP2, WP3 signature |
| WP5 pack habits | 005, 037, 038, 039 | sonnet | `game/sense/spare.go`, `unload.go`, `backtopack.go`, `inspector/gear_rows.go` after WP2 | `craft/*` | Listed per row | WP2 |
| WP6 food | 012 to 015 | sonnet | `domain/economy/item/meal.go`, `game/diet/choose.go`, `game/craft/view.go`, `domain/economy/recipe/flavour.go`; lead applies the four `SameMeal` lines in `game/tick.go`, `haul/step.go`, `travel/state.go` | `game/craft/gather.go` | Listed per row | WP0 |
| WP7 joy and drugs | 001, 002, 007, 008, 010, 011 | opus | `game/recreate/seek.go`, `recreate/deps.go`, `game/care/gift.go`, `game/care/feed.go`, `game/sense/topoff.go`, `game/sense/relax.go`, `domain/pawn/drug/policy.go`, the assign list in `windows/assign.go` | `craft/*` | Listed per row | WP0; 011 and the pack gift need WP2 |
| WP8 wander and sky | 028, 041, 042, 043 | sonnet | `game/sense/wander.go`, `game/snow/clear.go`, `snow/deps.go`, `domain/world/room/{room,stats,tally}.go`, `game/thermo/cache.go` | `rest/room_mood.go` | Listed per row | WP0 |
| WP9 text | 046 | haiku | `domain/economy/recipe/recipe.go` (`Kinds`), `inspector/bill_*.go` row, `RecipeView` fill in `craft/view.go` by hand-off with WP6 | Behaviour | Listed under CSE-046 | WP0 |
| Lead | Goldens, budgets | | `solid_budget.txt`, golden pins, checklists, `DOCUMENT_INDEX.md`, handoff | | `make arch`, `make test`, `-count=2` on goldens | All |

Shared-file notes: `craft/view.go` is touched by WP6 and WP9; the lead gives it to WP6 first and WP9 after. `inspector/gear_rows.go` goes WP2 then WP5. `game/care/tend.go` is WP3 only; `game/care/feed.go` is WP7 only. `game/sense/sense.go` is WP0's and stays frozen, components register as fields WP0 lists.

## Open questions for the lead

1. **Pack capacity.** 40 kg times Manipulation is original tuning. RimWorld's own figure would need ENG-003 extended by the user; this doc does not assume it.
2. **CSE-002 balance.** Colonists at full recreation will work in Recreation hours. It matches the mod's intent but changes daily rhythm the most. Default on as the brief says; the lead may want a playtest.
3. **CSE-010 preset.** Adding `MoodLift` is a new player-facing policy to give the mod's rule something to act on. If unwanted, the row stays `missing` with no gate.
4. **Doc index and handoff.** `.cursor/DOCUMENT_INDEX.md` and `.cursor/handoffs/` were outside this task's one file; the lead adds the row and the handoff.

## Not verified

- Every site that removes a pawn from play, for `Packs.Spill` (WP2 lists them).
- The take-off path for apparel in CSE-039; only `DropWeapon` was confirmed.
- Whether a door tile has its own room id in `g.roomGrid` (CSE-026).
- Whether any recipe takes mixed medicine (CSE-034).
- Whether a walking-speed urgency exists for the jog in CSE-041.
- That a nil `Sense` field leaves the state hash unchanged: expected from `normalisedFields` and from earlier additive fields, proved by WP0's hash check.
- Golden movement is predicted from reading the code; no run was made.
