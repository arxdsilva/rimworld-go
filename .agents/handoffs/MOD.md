# MOD handoffs

Families: MOD, CSE. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## MOD-000-mods-programme
RimHUD, Neat Storage and Common Sense programme · 2026-11-03 · f7631b72a
- Changed: asked 2026-11-03 to gap-analyse the three mods and add everything they add (RimHUD as complete as possible). Gap docs: `.agents/gaps/RIMHUD_GAP_ANALYSIS.md` (RHUD, DOS-n), `NEAT_STORAGE_GAP_ANALYSIS.md` (NST, WP-A…), `COMMON_SENSE_GAP_ANALYSIS.md` (CSE, WP0…). Released as v0.1.25 on 2026-11-04 (v0.1.24 failed CI and published no archives, see CI-002).
- Outcome (rows / done / excluded): RimHUD 504 / 429 / 75; Neat Storage 27 / 27 / 0; Common Sense 55 / 45 / 10. Each gap doc lists under Counts where a done row differs from the mod. DOS-9 records the HUD flip to on and the bugs it exposed.
- Decisions (lead, on the docs' open questions):
  - NST Architect placement: a thirteenth category `storage`; `hudlayout.ArchCatsH` grows. Sculptures admirable from the first change; Admire reuses `JoySolitary` so no saved array grows.
  - `SaveVersion` 12 to 13, one bump for all three, no migration (new fields additive, zero-safe); `testdata/v12.sav` must keep loading. CSE's "no bump" overridden because NST adds kinds an older binary can't read.
  - CSE-002 on by default as in the mod, said in the release notes as a balance change. CSE-010 opt-in `drug.MoodLift` preset ships. Pack capacity: 40 kg times Manipulation (original tuning).
  - `HUD.Enabled` stayed false until the last wave, then flipped on for new and existing settings (user asked for the feature in the game); the switch stays in Options.
  - Numbers: NST sizes and per-cell capacities follow the mod's table (new exception next to ENG-003); all other NST numbers and CSE thresholds are ours.
- Contract as written (W0-C) where it differs from the CSE doc: `sense.Sense` lives on `worldState` via `senseOf(g)` (`game/sense_host.go`), plain fields saved by `sense/save.go`, no component adds a field. The doc's `Packs` component is `Gear` (`sense/gear.go`); `Packs` is the map. `Tidy.After` true when it restarted the interrupted job. `TopOff.Wants` = "wants recreation", true by default. Roles per component (`TidyRoles`, `GearRoles`, `JoyRoles`) with adapters in `game/sense_*_host.go`; `craft`, `care`, `snow`, `recreate` roles are declared by the agents using them. A command kind the client doesn't emit yet needs no owed journey entry; the change that first emits one ships its journey.
- Process: waves 0 to 4 (contract and room; foundations; behaviour and card; card settings; lead finish), at most 15 agents, `make agent-budget` before each wave, lead alone edits `go.mod`, `Makefile`, `cmd/**`, `command.go`, `job.go`, `save.go`, goldens, `solid_budget.txt`, checklists, specs, README, handoffs. `solid_budget.txt` lowered only at the end by the lead.
- Perf: tick mean ms before and after (grown fixture, seed 1): 120 pawns 0.176 / 0.188; 250 pawns 0.252 / 0.243; 500 pawns 0.292 / 0.308. Card frame cost (`BenchmarkDossierDraw`): about 0.1 ms, 555 allocs per draw.
- Notes carried to later packages: DOS-1 must fill `Card.Stats` and `Card.BreakAt`; `Card.WorkType` is -1 for none; no per-condition pain or per-wound tone in `PartView`. NST-C: `windows/storage.go` and `architect/zone_select.go` call `game.ProfileFilterRows(z.Profile, z.Filter)` when `z.Profile != ""`.
- Open: not touched, still open: the `newMapState` hash window, `SaveInfo.Day`, `RIMWORLD_UPDATE_SAVE_FIXTURE`.

## MOD-W0-P-budget
Room in package `game` · 2026-11-03 · f7631b72a
- Changed: wave 0 of MOD-000. Code moved out of `internal/application/game` with no behaviour change (hash identical): `game/sense` (`Cleaning`, `Wander`; roles `Filth`, `Ground`, `Walks`, `Labor`, `Strolls`), `game/dose` (`Drugs` and `Line`; roles `Pawns`, `Minds`, `Bodies`, `Stock`), `game/roster` (`CandidatePool`, `BioOf`, bio and team read models, Health tab text), `game/mapfields` (`Generate`, `Roofs`, `Fog`, `Regions`, `Layer`).
- Decisions: `savedDrug` stays in `game` because `testdata/save_schema.txt` names it. Components are values built where needed (as `snowOf`); adapters are one pointer each, so no allocation, no new `Game` field or method. `dose.New` takes `idleRecheck`. `Cleaning.Assign` reads Home cells as a slice; its one interface call (`CleanFree`) stays after the distance filter, as before. `DrugRolls` returns the stream's draw at the same moment as the old code, so a lazily created stream appears at the same point.
- Doc mapping: old references `game/drugs.go:NNN`, `game/clean.go`, `game/idle.go:109` mean `game/dose/drugs.go`, `game/sense/clean.go`, `game/sense/wander.go`; `assignDrugUse`, `nearestDrug`, `ingestDose` are now `Assign`, `nearest`, `Ingest`. CSE-010 and CSE-011 land in `dose.Drugs.Assign`.
- Limits: `game` was at its ceiling (176 files, 53224 lines; `Game` 1837 methods, 64 fields). Not moved: `wildlife_habitat.go` (per-tile forage scan), `chunk_view.go` and `minimap_view.go` (read root `*WorldView`). `power*.go` (about 1,300 lines, low coupling) is the next big payer but its saved types and hot paths need their own change.
- UI: none, nothing observable changed.

## CSE-W1-habits
Options Habits tab (CSE-049, CSE-023) · 2026-11-03 · f7631b72a
- Changed: `options/habits_tab.go`: 27 toggle checks in four groups (Free time, Food and drugs, Tidying, Hauling and crafting) with one-line tips, three limit steppers (0..`MaxLimit`, TidyTend 0 reads "Unlimited"), "Back to defaults", scroll area. Values from `Habits.Rules()`; clicks send `SetGameplayToggle` / `SetGameplayLimit` via `Host.Send`, applied at once (colony is paused behind Options). Outside a colony the tab is listed but shows one sentence saying why it is off.
- Decisions: consumer-owned role `Habits{Rules() gameplay.Settings}` on `Shell`; root side `optionsHabits` in `options_compat.go`. `optTabW` 150 to 112 so 7 tabs fit; habits sit before controls, after story. Journeys assert after `update`, not `tick` (paused game runs 0 ticks).
- Open: `options/AGENTS.md` should list `domain/gameplay` as an allowed dependency. `TestEveryInputActionHasAJourney` failed on `journeyKindTODO` entries MarkUnload, SetBenchClean, SetName (not from this change).

## CSE-W2-pack
The pack (CSE-036) · 2026-11-03 · f7631b72a
- Changed: each pawn can carry stacks off the map. `domain/pawn/inventory/pack.go`: `Add` (joins a `SameLoad` entry, merging rot/poison/wear via `MergeFood`; clears X, Y, Carrier, flags), `Take`, `Grams`, `Find`; an emptied entry goes, IDs never reused in a pack. `game/sense/pack.go` on `senseOf(g).Gear`: `Pack`, `Has`, `Capacity` (40 kg times Manipulation, ours), `Room` (`1<<30` with `RespectCapacity` off), `Stow`, `TakeOut` (sets `FromPack`), `Unstow(id, kind)`, `Spill`, `Rot`, `Wealth`. Callers clear/set `j.Carry` around `Stow`/`TakeOut`. `Stow` refuses corpses, so a corpse ingredient stays on `HaulIngredients`.
- Decisions:
  - A pack counts in `Game.Wealth` (per map; a pack on the road isn't counted) and nowhere else; stock totals, stockpile counts, hauling and search don't see it.
  - Rot runs on `phaseClimate`'s 250-tick pass, not staggered by id (a stagger would scan every tick), walking the map's `pawnIDs`. Packs off the map don't rot, like caravan goods; a casket is stasis.
  - No threshold behaviour (losing Manipulation drops nothing), so no no-flap test.
  - An empty pack is deleted from `Packs`, so such a colony saves and hashes as one that never used packs.
  - `FromPack` is set by `TakeOut`, cleared only by `Spill`, so `BackToPack` must clear it on landing. New needs go in a new role embedded in `GearRoles` (a composite counts one per embed).
- Where a pawn's pack goes: death on a map (`dieBy`): beside the body; on the road, in a casket or pod: caravan goods / where it boarded; downed then stripped, arrested, captured, raider or visitor leaves, left aboard ship: where it lies/stood; kidnapped: map edge; closing map: lost; final wipe: deleted; caravan/casket/pod (`stashPawn`): keeps the pack (on `worldState`). A lost caravan's cargo takes the pack's share with it. `SetDown` keeps the whole stack (unlike `dropStackOf`).
- Guarantees: `TestPackNeverVanishes` (item balance over maps, packs and caravans across each path), `TestPackRoundTrip` (save, load, hash equal, equal after 600 ticks), `TestPackInWealth`, `TestPackSpillsOnDeath`.
- Save: no format change (`sense.Save.Packs` and `FromPack` already existed). Hash with no pack in use unchanged (grown 120, 20000 ticks, seeds 1 and 2).
- UI: Gear tab shows `Pack  1.5 / 20.0 kg` and one line per entry only when the pack holds something; rows past the tab bottom are cut.
- Limits: about 113 lines added to `game`, one file. `TestSOLID` failed on `options.Shell` (9 methods, another agent's file).

## CSE-W3-tidy
Tidying a room around a job (CSE-017…027) · 2026-11-04 · b4830ea8d
- Changed: `sense/tidy.go` and `tidy_route.go` (gate, `After`, `Forget`, `SetBench`, `TidyBench`, `reclaim`, `prune`, detour, route, `Do`), roles in `TidyRoles`, adapters `game/sense_tidy_host.go`, `care.Doctor.Urgent`, `CleanOff` in `fillBuildingView`, bench switch in `inspector/cmds_build.go` (id `cmd.bench_clean`; `bldg.*` ids never read `Cmds`). CSE-023's steppers are the Habits tab (CSE-W1).
- Decisions:
  - `TidyAt` is the cooldown (1500 ticks, ours), set before any search, so a pawn whose dirt is taken/unreachable retries after it, not each tick; it holds on the way back through `startJob`.
  - `Before` allocates nothing and looks at no tile (rule, cooldown, queue length, `roomSum`); a meal's seat scan (`AtTable`) runs once per meal start and only with dirt queued.
  - A job cut by `interrupt` or replaced by another `startJob` never returns (`Forget`); a held load goes down there.
  - `reclaim`: what a giver claims after `startJob` stays held through a `Before` swap and is reclaimed on resume. A load in hand sits outside the `CleanRoom` record (`Carry` 0) so `dropCarry` leaves it.
  - A held meal: `noFood` holds the hunger check at most 600 ticks (`tidyMealHold`), then hunger cuts the tidy.
  - Numbers (ours): `tidyCooldown` 1500, `tidyMealHold` 600, `maxTidyCells` 64, `hugeRoomCells` 160, `hugeRoomReach` 7, `detourFar` 40 steps, `detourFarLine` 10 tiles, ratio 0.7; path length stands in for the mod's path cost.
  - Deviations: `Sleep` out of the kinds (phaseAI cuts anything but sleep when tired/bedtime); `Stargaze`, `Horseshoes`, `SocialRelax` in; `senseOf` uses `newSenseTidy(g)` since the adapter embeds cleaning adapters.
- Limits: door filth missed (doors are `room.Wall` in `roomGrid`); outdoor jobs never tidy; a pawn's own claim makes `CleanFree` false so a held dirty work spot isn't swept; `Gather`'s tidy lasts until the next pull; `UseComms` is always forced so never tidies.
- Open: `action_view.go:127,196` and `rooms.go:293` have no `job.CleanRoom` case (no sweep animation or progress bar; the job line reads "clean room"). `Shop.pending` doesn't count a pawn in a tidy with a bill behind it.

## CSE-W4-bills
Bills gather in the pack (CSE-029…035, 040, bill call of CSE-018) · 2026-11-04 · b4830ea8d
- Changed: `craft/packgather.go`, `job.PackIngredients`; the plan saves in the job record (`Route[:Count]` pairs: from `RouteI` on stack and count, before it pack entry and count or 0,0 once laid; `Route[Count:]` loads laid; `SpareX` is the recipe, since bills can be reordered/removed mid-walk, `billOf`). Roles `Prep` and `Packs` under composite `Supply{Stock; Prep; Packs}` (`Host` already had 8 embeds). Spoil key before distance (3 days, whole hours; never `work.Doctor` recipes). `Shop.plan` chooses pack or hands; `stackFirst` before `start`, `tidyFirst` on first work tick.
- Decisions:
  - `Shop.start` starts a pack job as `HaulIngredients` and flips the kind after `StartJob` (`claimJobTarget` in `food.go` only count-claims that kind).
  - `tidyFirst` hands `TidyBench` the bill with `Target` = the bench (a plain bill's `Target` is the stack its load was split from).
  - Every stowed entry gets `Unload = true`; what a cut job leaves is taken by W5's unload. A kept spare of an ingredient kind would be unloaded too (rare, not handled).
  - `Settled` clears `FromPack` at the bench so a load taken for a bill doesn't go back via `BackToPack`.
  - No room for one of each pick, or a corpse among them: `packable` false, hands do the bill. No room mid-job fails `Blocked`; next plan goes by hand (no flap).
  - Deviations: CSE-029 no timed pickup/progress bar; `Unstow(id, entry, n)` and `Stow` returns the entry (exact counts); CSE-035 bill not stored as `Resume` (route stale after haul), replans at next Assign, `HaulAt` stops a second haul; CSE-034 no recipe takes mixed medicine, closes with CSE-033 (`TestPlanPotencyFirst`).
- Perf: 2000 stacks, `PlanManyStacks` rules off 634 us, on 637 us, 1 alloc; `PackGatherManyPicks` 80 us. `TestRulesOffKeepTodaysPath` (frozen oracle) and `TestPackBillRulesOffGoesByHand`.
- Goldens: `TestScaleGolden` passes with defaults and with all rules off (never reaches a bill). `TestGrownGolden` moved, not from these rows (all rules off gives the same moved hash, also on a tree with this agent's files restored); not re-pinned. Save impact: none beyond existing fields.
- Limits: 64 lines in `game/craft_host.go` (budget about 60).
- Open: `craft/view.go` `current()` must add `job.PackIngredients` to both conditions (until then the bill row shows no worker during a pack gathering); `game/bills_settings_test.go:69` should accept `PackIngredients`; `game/reservation_share_test.go:115` should list it; `craft/settings.go:73` reorder and `RemoveBill` don't adjust `BillIndex` (pack jobs covered by `billOf`); `craft/AGENTS.md` to list `packgather.go` and roles.

## CSE-W5-gear
Spare meal, put back, manual unload (CSE-005, 037, 038, 039) · 2026-11-04 · b4830ea8d
- Changed:
  - Spare meal (CSE-005): in free time, rested, food above 30 %, no meal in pack, `SpareMeal` on: `job.TakeSpare` to the nearest allowed meal via the food index (40 tiles), one meal into the pack. Hungry with a meal in the pack: `Gear.Assign` takes it out and runs `job.Eat` in place (`Count` 2, no table walk). `SpareAt` waits 2,500 ticks after a failed look, a meal taken, a meal eaten. Meal kinds: Meal, FineMeal, LavishMeal, SurvivalMeal (`sense.IsMeal`).
  - Put back (CSE-037): `BackToPack(id, carry)` in `dropCarry`: the `FromPack[id]` stack returns to the pack (re-marked if an unload was putting it away) when `PutBack` is on and `Room` allows; else normal drop. `FromPack` cleared either way.
  - Manual unload (CSE-038): `Mark` validates pawn, mode, index; bad input does nothing; `ManualUnload` off ignores it. Mode 0 sets `Entry.Unload`; Cancel on an entry in hand stops the job and the load goes back (or down with no room).
  - Unload (CSE-039): `job.UnloadPack`: wait 10 ticks, take the first marked entry (up to 100), `haulDest` finds a store cell, walk, `Lay`; no cell: down where it stands. `UnloadAt` is 500 ticks after beginning, cleared on completion. CSE-040's `Entry.Unload = true` is hauled with no button.
- Decisions: `Assign` is one line in `game/tick.go` `phaseAI` before the hungry branch, reached only with no job; allocates nothing (`BenchmarkAssignNothingMarked` 18 ns/op, 0 allocs). Marks for worn things (modes 1 to 3: garment, weapon, spare): sense has no field, so they start the unload job at once (`Count` = mode, `BillIndex` = slot) and Cancel ends it; the mark is the job; no Cancel shown. Buttons one line high. Roles added: `GearJobs` (7), `GearFood` (6), `GearWorn` (7).
- Guarantees: no-flap tests `TestSpareMealDoesNotFlap`, `TestUnloadDoesNotFlap`; `TestUnloadMarkedInTheGame` (steel balanced every tick), `TestSaveMidUnload` (hash equal, equal after 300 ticks).
- Save/goldens: no schema change. Default-on, so colonies with a meal on the ground move: `TestGrownGolden`, `TestScaleGolden` and state hashes expected to move; not re-pinned.
- UI: Gear tab Unload buttons on the weapon, each worn garment, the spare weapon and each pack row (Cancel while marked); journey `TestJourneyGearUnload`.

## CSE-W6-food
Food (CSE-012…015) · 2026-11-04 · b4830ea8d
- Changed: CSE-012/013: `item.MealRule{OddNormal, Mix}`, `SameMeal`, `CanMergeMeal`, `RuleOf(gameplay.Settings)` in `domain/economy/item/meal.go`; sites `game/tick.go` (planGather, tile merge), `haul` (`Host.MealRule`), `travel` (`Goods.MealRule`, `Hold.addMeal`/`merge` take the rule; `addItem` keeps the default for tests) ask the colony's rule through `item.RuleOf(senseOf(g).Rules)` adapters. `TestGatherMealRule`: refused with SeparateMeals on, merges off.
- CSE-014: `recipe/flavour.go` (neutral tables), `craft.IngredientNamesFor` (pure function of stack id and kind, `randx.SplitMix64`), one line in `game/item_view.go`. Display only; no recipe-view line (flavour is a stack fact; item inspector already shows the guess).
- CSE-015: `diet.Picker.Under(Ruler)`, `diet.SpoilBonus`; `dietEaters.Rules()` in `game/food.go`. The Picker carries the flag, not `Book.Spoil`. `BenchmarkPickerScore` 0 allocs.
- Save: none. No UI line for the spoil score (internal weighting).

## CSE-W7-joy
Joy and drugs (CSE-001, 002, 007, 008, 010, 011) · 2026-11-04 · b4830ea8d
- Changed (all numbers not in the mod's table are ours):
  - CSE-001 `recreate/seek.go`: `Seek` keeps the best outdoor pieces, tries them first. Saved craving on below 40 %, off from 60 %; bed rest keeps craving, skips outdoor pass.
  - CSE-002 `sense/joy.go` `TopOff`: in a free hour `Wants` false sends a colonist to `assignWorkByPriority` before idling. `TopOffOn` set above 95 %, cleared below 80 %.
  - CSE-007 `care/gift.go` `Giving`: after `Visits.Assign` picks a patient, a hash of tick and pair may turn the visit into `job.FeedPatient` with a dose (pack first, else nearest stack). Weights drug `4*(100-tolerance)`, visit `6*100`; dose gap in `Allows`.
  - CSE-008 `sense/joy.go` `Relax`: `idleSocialRelax` asks `Relax.Assign` first; the `SocialRelax` job fetches a drink, carries it, finishes at `Full*99% - dose joy` or when linger ends; at least `relaxSit` (240 ticks) seated.
  - CSE-010 opt-in `drug.MoodLift` (beer only, mood below 50; with `DrugPotentialMood` target mood also below 50; a craving ignores the gate); 12-hour dose gap. CSE-011 `dose/habits.go` `fromPack`: `Assign` looks in the pack before `nearest`.
- Decisions:
  - `TopOffOn` means "topped off"; the comment in `sense/sense.go` says the reverse and needs fixing. The flag follows joy when `Wants` is read (free-time decisions), not at each job end/start as the row says; a mark passed in a Work hour counts from the next free hour.
  - The Sleep block is included (a colonist who woke rested works until it tires; `bedtime` still interrupts at once). `Wants` returns true while a colonist is sitting or could seek a seat (else topped-off work starves comfort).
  - Tolerance scales only the drug's weight, not the visit's. With the rule off, a gift comes from a stack only. `DrinkFor` skips drugs with an overdose limit. `TopOffOn` entries of the dead stay (bounded by roster); `Craving` cleared by `recreate.Forget`, which has no root caller today.
- Measured CSE-002 (3 colonists, 5 days): seed 1 with chess table and horseshoe pin, 4.9 % of Recreation-hour colonist-ticks at work off, 13.5 % on; seed 2 6.9 % both (joy peaks 86.6 %); no recreation piece unchanged. `TestRecreationBlockWorksWhenFull` (97 % entering): 23.9 % off, 78.1 % on. Total work fell 2.4 % and sleep rose 8.5 % in seed 1; cause not investigated.
- Guarantees: `TestHabitsOffKeepTheHash` pins seeds 1 and 2, 30000 ticks, with every toggle off, equal to f7631b72a. `TestTopOffWantsAllocatesNothing`, `TestTopOffKeepsBedtime`, `TestRelaxDrinkBalance` (5 events), `TestPackDoseInterrupted`. Bench: `Wants` 31 ns/op 0 allocs; `FreeTimeStep` off 14640 / on 14598 ns/tick.
- Save: `recreate.Save.Craving` only (additive, sparse); `save_schema.txt` gained those lines. Goldens: `TestGrownGolden` moved (not re-pinned; other agents' default-on rows share the tree); `TestScaleGolden` passes.
- UI: MoodLift is a fourth button on the Assign tab's drug policy with a tip; toggles in Habits tab; gifts show as the feed-patient job and "fed by" log lines.
- Limits: `sense_joy_host.go` 99 lines plus about 10 hook lines in `game`.
- Open: a work giver whose `startJob` fails during a relax has already put the drink down; the colonist can lose a held drink (counts balance, `TestRelaxDrinkBalance/work_giver_fails`). Whole `game` package under `-race` had two failures outside these rows: `TestUnstowIsAnOrdinaryCarriedStack` (`dropCarry` now calls `BackToPack`, CSE-W5) and `TestBillGathersWoodFromSeveralStacks` (CSE-W4 area).

## CSE-W8-wander
Wander, snow sky, cold store (CSE-028, 041, 042, 043) · 2026-11-04 · b4830ea8d
- Changed: CSE-028 `snow.Sky.SkipClear` checked first in `AssignClear`; `snowHost.SkipClear` = `Kind.Falling()` && `SnowWaitsForSky`; a started job is not dropped, only new assignments wait (`TestClearWaitsForClearSky`, `TestClearSkyNoFlap`). Sand: none exists.
- CSE-041/042: `sense.Wander` carries `gameplay.Settings` plus `Shelter` and `Manners` roles; `senseCourtesy` in `sense_host.go`. Both off: identical draws and picks (`TestWanderOffDrawsSameNumbers`). Sheltered: roofed Home goal with roofed quarter points of the straight line; under open sky `seekRoof` rings out to radius 12 (at most 6 reachability tries, no random draw), else stays put. Jog: no urgency field, walks normally.
- CSE-043: `room.ColdStore` (appended), `FullContents.ColdSides`, `Tally.RoleCold`, `thermo.Model.ColdSides(r)` (cooler thermostat target < 0).
- Perf: `Wander.Pick` off 44 ns, on-fair 48, on-rain 60, 0 allocs.
- Limits: `Rude` calls `countFacts` (pass over stacks and buildings) per candidate with PoliteWander on (off by default); a per-room cached fact would fix it. `ShouldHide` ignores toxic resistance (no pawn stat) and lord duty/home-map.
- Open: `rest_host.go:RoomQuality` and `rooms.go` still call `Tally.Role`, so RoomView and room_mood don't see ColdStore yet (`Rude` computes the cold role itself; label shows via `Role.String()` once wired).

## CSE-W9-recipes
Recipe ingredient kinds named (CSE-046) · 2026-11-04 · b4830ea8d
- Changed: `recipe.Input.Kinds()` (kinds `Accepts` takes, in kind order); `craft/view.go` `Recipes()` fills `RecipeView.InputKinds`; `Bills.pickTip(r)` in `inspector/bill_config.go` returns "Takes: wood, steel, ..." when `FullRecipeList` is on, "" otherwise. Tests `TestInputKindsMatchPlanner` (stateless inputs only; the planner's per-stack state: dryable, smelt yield, wreck, is not in `Kinds`), `TestRecipeRowNamesKinds`. `Def.InputKinds` unchanged.
- Open: the picker row is not wired: `openBillPicker` in `inspector/building_inspector.go` needs `Tip: func(i int) string { return a.Bills().pickTip(recipes[i]) }` on its `uikit.ListSpec`.
