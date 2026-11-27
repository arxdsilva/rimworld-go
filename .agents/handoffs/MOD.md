# MOD handoffs

Families: MOD, CSE, PLG. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

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

## PLG-0-plugin-engine
Plugin system: discovery to save header (PLG-001…050) · 2026-11-17 · e266b7a9f
- Changed: a folder in `mods/` (shipped) or `<config>/rimworld-go/mods` with `plugin.json`, `defs/`, `patches/` loads when its id is in `Settings.Plugins.Enabled` (next start). `application/modding` resolves the order (requires/after/before/conflicts, reasons), builds defs with templates and patches, and freezes per-class books; `modloader` reads through `os.Root` with caps. Backstory is the first moddable class; plugin backstories are drawn in pawn generation. `cmd/contentlint` lints headless. `mods/example_frontier` ships (off by default) and is built by a test.
- Decisions: JSON via the standard library (no dependency). No package registry changes at run time: `game.Config.Content` carries the books (zero = core), package `backstory.DefOf/All` are gone. Core defs keep their slots whatever a patch does (saves keep indexes). Captures load no plugin. Gap doc "Decision record" and "Decisions made while building" hold the rest.
- Save: `SaveVersion` unchanged; the header's `Content.Plugins` is the load order and `contentid.Of` hashes the run's books, so the SAV-025 missing-plugin and content-mismatch notices now fire on real saves. With no plugin the fingerprint is unchanged (`TestOfCoreIsCurrent`).
- Limits: `game` line budget 51644 → 51666 for the book threading (user-approved ceilings rise when perf holds; no tick-path change).
- UI: the Plugins screen and Content Errors view are PLG-070/071/041 (next section); until then problems go to the log.
- Open: PLG-025 recipes, PLG-026 more classes, PLG-055 translations, PLG-072 source lines, PLG-003 `-plugins` flag, PLG-060 scripting (needs a user decision on Starlark/WASM).

## PLG-1-plugins-screens
Plugins screen and content problems view (PLG-070, 071, 041) · 2026-11-17 · 011fd4e68
- Changed: main menu › Plugins lists every installed plugin in load order (`#N`), with an on/off box, state (loaded, not loaded: <first error>, off, on/off after restart) and problem counts; the detail says what it requires, loads after/before, conflicts with and "Position N: <reason>". Toggling rewrites `Settings.Plugins.Enabled`; a banner says changes apply at the next start. Errors raise a main-menu notice that opens the Problems view (plugin › file › def › field, Show plugin, Disable).
- Decisions: read model `plugins.View` filled once by compose (`runContent.view`), so the feature never imports `modding`. The banner compares the saved set with the set the run started with, not the loaded ids. Host and wiring sit in `mainmenu_compat.go` (the root package is at its 60-file ceiling). The screens draw through `Host.PaintMenu`, like Options.
- Review: frames shot with `-capture-ui plugins` and `plugins-problems` (shipped `mods/` plus an in-memory fixture that cannot load, in-memory settings); the review found the bare backdrop and a lone "…" by each unlabelled checkbox (`gfx.Ellipsize("")`), both fixed.
- Limits: root ebiten package 11,958 of 12,000 lines; no `App` field or method added.
- Open: Copy diagnostics (no clipboard), search/filter, plugin settings PLG-073, source lines PLG-072.

## PLG-2-recipes-and-saved-keys
Recipes moddable; saves keep definition keys (PLG-025, PLG-044) · 2026-11-17 · e0450f1be
- Changed: plugins add recipes (offered by their bench's bill picker, made like core ones) and patch core recipes' plain fields; `recipe.Book` (per-bench index, no allocation in work discovery) held by `craft.Bills`, package `recipe.DefOf/All/For/Upgrade` gone. Saves write `ContentKeys` (backstory, recipe) and a load remaps saved IDs by key: a moved plugin definition keeps its meaning, a gone one becomes none (backstory) or drops its bill (recipe). `Build` is a per-class table; a broken `plugin.json` reports line and column.
- Decisions: behaviour-bound recipe flags are core-only (gap doc PLG-025). `ContentKeys` is excluded from `StateHash` (content, not state): every pin holds. Recipe IDs stay a byte (256 per run).
- Perf: tick benchmarks within noise, allocs equal (ColonyHour 193.78 → 192.98 ms; TickGrown/500 3.129 → 3.031 ms). `game` line budget 51666 → 51696 (key remap 13, recipe hand-off 17; user-approved, perf holds).
- Tests: `TestSavedBackstoriesFollowTheirKeys`, the recipe twin, `TestBillPickerOffersPluginRecipes`, round trips straight and through JSON.
- Found: `go test -race ./internal/application/game/` exceeds 10 min as one local binary at baseline too (gap doc leftovers); CI runs it alone.
- Open: plugin products are not smeltable (`SmeltYield`), PLG-026 more classes, PLG-055 translations, PLG-072 source lines, PLG-060 scripting (user decision).


## PLG-3-classes-scripts-translations
Traits, research, incidents and items as plugin classes; Starlark scripts; translations; source lines; `-plugins` · 2026-11-18 · f24dcc152 (contract), 2b2e74c24 (`-plugins`), the wave commit after it
- Changed: plugins add and patch traits, research projects (gating plugin recipes), nature incidents (`like` a core incident) and goods items (own PNG, value, category, smelt yield); run Starlark hooks (`on_hour`, `on_day`, `on_incident`, `on_colonist_died`) that post messages/letters and spawn items through typed commands; ship `lang/<locale>.json` translations; the bio tab, bill picker and bill title say "Added by"/"Changed by"; `-plugins a,b` sets one run's plugins. The example plugin shows each.
- Decisions (user, 2026-11-18): Starlark (`go.starlark.net`, BSD-3), not WASM; all four classes at once. Lead: each class keeps core's IDs and caps at 255/256 (byte IDs, as recipes); plugin items take Kinds after core with neutral `Kind` methods, their data through `Content.Items`; mass and stack limit stay core-only (read straight from the kind in ~30 places); items decode before recipes so recipes can name them; scripts keep no state across saves (PLG-051 deferred); a failing script is turned off, never stops the game.
- Saves: every class's keys are saved; load remaps traits, research progress/selection, incident cooldowns/queue and every item kind by key, and now also world people's backstories (a PLG-044 gap). Core-only saves and the save schema are unchanged.
- Limits: `game` 176 files, 52571 lines, `Game` 65 fields; `ebiten` 12176 lines (SPDX headers + translations/textures); `inspector.Pane` kept at 39 by moving the source helpers onto a small `sources` reader.
- HUD: research tab lists plugin projects; bio shows plugin traits; storage, inspector and bills show plugin items with their texture; dev menu fires plugin incidents; Plugins detail lists locales; Problems lists turned-off scripts.
- Open: PLG-026 remaining classes and item gaps (gap row); PLG-060 turned-off set not saved, threats that aren't incidents; PLG-072 lines for research, items and incidents; PLG-055 backstory description templates.

## PLG-4-plugin-items-and-vanilla-fixes
Plugin items with their own mass, stack, trade, shelves and info card; four vanilla mismatches fixed · 2026-11-18 · fb2e63c0c (contract), d6b2b5fd4
- Changed: item defs take `mass` (g, 1..100000), `stack_limit` (1..1000) and `trader_stock` (0..1000), defaults 1000/1000/0; every core read of mass, stack limit and value (haul, stacking, packs, bill gathering, raid theft, caravans, pods, encounter demands, trade) goes through `item.Book`. Plugin goods sell to traders whose categories want them and are stocked (1..`trader_stock`, half the visits, drawn after the crate pools) only on opt-in. New shelves (`zone.KindShelf`; not bookcases or egg boxes) admit plugin kinds. The info card shows a plugin item's label, description, category, mass, value, stack limit and a Source row. Example plugin: trail bell 250 g, stack 75, stock 6.
- Vanilla fixes (contributor report, checked against the raw wiki pages; the report had ours and vanilla swapped): fueled stove and smithy 5 → 50 logs and burn per worked tick (1 log per 375 / 750 worked ticks = 160 / 80 a day), not per work unit; hostile at ≤ -75 and neutral again at ≥ 0 (was -50/-10), Drifters start -90..10 so ~16 % still start hostile; orbital trader stays 40000 ticks (was 90000); trade beacon reach is a 15-tile-wide disc (dx²+dy² ≤ 56) filled 4-connected that rock and built walls cut (doors, columns, frames don't), cached per `structVer` (was radius 6, walls ignored). Wiki: Fueled_stove, Fueled_smithy, Factions, Trade, Orbital_trade_beacon. New CLEAN_ROOM row for the trade numbers.
- Decisions: core items keep refusing mass/stack/stock patches; STK-ST (stacks 1 or 1000) holds for core, a plugin `stack_limit` is the exception; recipe work stays scaled, so fuel per item is below vanilla.
- Proof: `bin/benchmark -hash` of the plugin-item part alone (Part B sim files reverted) equals 46184ef77 on grown/raid 120/250; full hashes move only for the four fixes. Tick means equal or lower than base. Game package passes under the race detector in 126 s alone.
- Limits: `game` lines 52586 → 52628, `Game` methods 1769 → 1770 (`collectGoods`), `ebiten` lines 12176 → 12180 (user rule: raises OK while perf holds).
- HUD: info card rows; fuel line "12 / 50 wood"; faction window and wizard tips show the thresholds; beacon ghost ring 7.5 tiles and build text; orbital letter "about 16 hours".
- Open: ally band (wiki 75, kept until 0; ours 50/30); beacon needs power on the wiki (40 W); existing shelves don't auto-admit a newly enabled plugin's items; `Settle` accepts a `math.MinInt` offer quantity (pre-existing); `TestScaleGolden` was not run (it skips under the race detector; run it once without it); CI's `TestPainterAllocatesOnlyEbitengine` (aashape) flaked on 46184ef77; go1.27.4 fixes GO-2026-6859 (net/http).

## PLG-5-follow-ups-ally-beacon-power-offers
The PLG-4 open items: ally band, beacon power, offer quantities, Go 1.27.4 · 2026-11-18 · 02b37c56b
- Changed: factions become allies at >= +75 and stay allies until goodwill reaches 0 (wiki: Factions; `FriendlyEndAt`, `FriendlyKeep` gone); start ranges unchanged, so none starts allied. The orbital trade beacon is a 40 W consumer (wiki: Orbital trade beacon): only powered beacons give trade reach, gift and drop-pod spots; the reach cache key is `structVer + pwc.vers` (a power flip alone refreshes it). `DecodeOffer` and `Settle` reject quantities beyond `MaxOfferQty` (1,000,000), `math.MinInt` and overflowing totals ("impossible quantity"). `go.mod` pins `toolchain go1.27.4` (GO-2026-6859; `make vuln` clean).
- HUD: faction tips show all four thresholds; the build text, the no-beacon reason ("needs a powered trade beacon"), the alert and the beacon glow follow power.
- Saves: an old save's beacon simply needs power; `saveV12Hash` unchanged.
- Flaky-test audit and the golden: OPS.md CI-006.

## PLG-6-cadence-hooks-batched-reads
Scripting port: batched colonist reads, 60-tick cadence hooks, Starlark vs gopher-lua baseline · 2026-11-20 · 25bf1383b (contract), 3b0f16b93, 9b169e7f7, 2801c7f27, 1046d7b5b
- Changed: scripts read `ctx.pawns()` (every living home colonist, one call, read-only tuple) and `ctx.pawn(id)` (fields id, name, x, y, food, rest, mood, downed, drafted: the HUD's bars and smoothed mood). `on_pawns(ctx, pawns)` gets the colonists due this tick in one call per file; `on_pawn(ctx, pawn)` runs per due colonist. Each colonist is due once every 60 ticks (`cadence.Due`, offset `OffPawnScripts`), from `idindex.Buckets`; no tick scans every pawn. The example plugin uses `on_pawn`.
- Decisions (user 2026-11-20): runtime stays Starlark (`MASTER_SPEC` §49 now holds the criteria for any second runtime: same port, deterministic step budget, memory budget, per-mod cost, no CGo; not speed alone); ship both cadence hook shapes; gopher-lua v1.1.2 (MIT) only in `scriptbench` tests (`TestGopherLuaOnlyInBenchTests`). Lead: `ctx.pawns()` is home only while `ctx.colonists` keeps home + caravans; `on_pawn` shares one `MaxSteps`/`MaxCommands` per file per tick, all or nothing; its `ctx.rand` is keyed by pawn id; `pawnIDs` grows only through `addPawnID` (source test), the index is verified on visit and rebuilt on load, so save/load and continuous runs see the same visits.
- Baseline (`scriptbench`, median ns/op, starlark / lua-lazy / lua-eager): batched n=50 6.5k / 14.0k / 52.6k; n=500 55k / 131k / 605k; per call n=50 11.3k / 15.9k / 56.7k; n=500 107k / 157k / 556k. Empty hook: Starlark full Fire 359 ns, Lua bare call 152 ns. Record shape dominates (eager tables 4-5x lazy); with lazy records Starlark is 1.4-2.4x faster. gopher-lua has no deterministic step budget and its `math.random`/`pairs` order break determinism, so it cannot ship as is.
- Perf (no plugins, A/B vs db3ab95f5, 8 runs): `BenchmarkColonyHour` median 34.27 → 34.73 ms (ranges overlap, 3946 allocs both); `TickGrown/120` 143.0 → 140.4 µs. Cadence path: 600 colonists cost ~8.7x 60 per 60 ticks (O(N/60) per tick).
- Limits: `game` lines 52628 → 52769 (user rule: raises OK while perf holds); no new `game` file, `Game` field or method.
- HUD: none new. The hooks act only through the existing message, letter and spawn commands, which already show; a turned-off script still lands in Problems.
- Saves: schema unchanged (the index is not saved). Determinism: `TestPawnScriptsSaveLoadSameVisits`, `TestPawnHooksDeterministic`.
- Boxes: feature checklist 38.3 **Choose scripting runtime** `[x]`, **Read-only world query API** and **Event subscription** `[-]`; MODDING gap row PLG-060 `[-]`. No `spec-backlog` issues existed yet.
- Open: per-mod script cost in developer diagnostics (§49); a memory budget per script; reads beyond colonists (animals, visitors); the turned-off set is still not saved.

## PLG-7-wasm-second-runtime-candidate
Second scripting runtime: WebAssembly on wazero named as the candidate · 2026-11-20 · cdb16a849
- Changed: docs only. `MASTER_SPEC` §49 names Wasm on wazero as the second-runtime candidate for mods that keep their own state and touch many entities; Starlark stays the default. New gap PLG-063 and backlog item 38.3 "Second runtime spike" hold the six spike criteria.
- Decisions (user 2026-11-20): from a contributor benchmark (100 pawns, hourly work review: Rust/TinyGo Wasm with fuel ~1.1-1.2x native Go per pawn decision, Luau 1.2-1.9x, Starlark ~7x, 11.3 vs 1.66 µs). Wasm on wazero meets the §49 criteria on paper (pure Go, deterministic fuel, memory cap from max pages, one instance per mod). gopher-lua (no deterministic budget, slower than Starlark in `scriptbench`) and CGo Luau are dropped as candidates.
- Spike gate: host-inserted fuel; same state hash on amd64 and arm64; per-mod fuel and memory; versioned size-capped save hook (PLG-051); same port with batched data; harness in `scriptbench`.
- Boxes: none ticked; added the MODDING gap row PLG-063 and the feature checklist 38.3 spike item, both open.
- Open: PLG-063 spike; ask the contributor what the projectile column (~96 ms for every runtime) measures and which tool inserted fuel.

## PLG-8-wasm-wazero-spike
WebAssembly on wazero spike: second scripting runtime behind the same port, not wired · 2026-11-20 · a779d925a, 54eb729d5, 32e7a0fd9, c3175582e, b0b19a45d, 7da749bdc, b997ef70b
- Changed: new adapter `internal/adapters/outbound/wasmhost` (`*Host` implements `scripting.Host` plus Problems, Stats, Save/Load, Close) with `wasmrewrite` (pure-Go decode, check, rewrite) and `wasmtest` (test module builder); Rust fixtures under `wasmhost/testdata/rust` (`make wasm-fixtures`); `scriptbench` Wasm case and compile benchmark; a Starlark vs Wasm colony test in `compose/game` (test only). wazero v1.9.0 (Apache-2.0) only in `wasmhost`.
- Decisions: fuel charged at function entry, loop header and per bulk-memory byte, 1,000,000 per call (per file per tick for `on_pawn`), all or nothing; stack depth is a weighted budget (2^22, instruction weight = 1 + values pushed, at most 2^17 per function: the arm64 wazero compiler refuses functions over 16 MiB of machine code, so a bigger one would load on some platforms only); types capped at 64 params/results; `Check` bounds every index against the original module so appended fuel globals cannot be reached; NaN results canonicalised; start sections rejected; memory min or max over 256 pages rejected at load (wazero would clamp silently); modules over 1 MiB rejected; guest text lengths checked before any copy; wazero compile panics recovered into a load Problem; runtime creation serialised (wazero v1.9.0 races on concurrent `NewRuntime`).
- Proofs: golden hash 0xa3bc661840bdc5fc equal on compiler and interpreter, arm64 and amd64 (Rosetta; CI runs amd64, where 0/0 is non-canonical); Starlark and Wasm twins give one colony hash; `FuzzPrepare` (219 seeds, ~730k execs) never panics or validates an invalid module; save → fresh host → load equals a continuous run.
- Perf (M2 Max, idle, median): 500 colonists batched 5.7 µs Wasm vs 57 µs Starlark; per call 30 µs vs 107 µs; compile 0.8 ms vs 9 µs; a 500-colonist hook uses 1.5% of its fuel. No game code changed.
- Limits: no SOLID budget change; new packages well under the limits.
- HUD: none. The spike is not wired, so no player can load a Wasm mod yet.
- Saves: none (the blob format is adapter-local; the save file is PLG-051's).
- Boxes: feature checklist 38.3 **Second runtime spike: WebAssembly on wazero** `[x]`; MODDING gap row PLG-063 `[-]` (spike done, wiring open, now issue #7).
- Open: wiring (manifest field, mod loader, save-file blob, per-mod cost in diagnostics) as a follow-up to PLG-063; report the wazero `NewRuntime` race and the validator panic upstream; rebuild fixtures with a current Rust when available; the contributor's harness.

## PLG-063-wasm-wiring
Wasm mods run in real games · 2026-11-21 · a9e028551, bab189b41, b7cff43f1
- Changed: `scripts/**.wasm` in a plugin loads through the mod loader (runtime by extension, no manifest field; one 1 MiB module cap); hooks interleave per plugin in load order across Starlark and Wasm with shared limits; a Wasm file turned off is on again after load (R5, as Starlark); mutable reference globals are rejected at load; a rustc 1.39 `tally` module runs through the loader in a colony test.
- Decisions: saving runs on a throwaway copy of the host, so it never changes a run (save-time notes only in the Plugins problems list); `StateHash` never runs guest code and excludes the blobs; blob cap 64 KiB, `MaxBlobs` 256, notes capped at 16.
- Limits: `application/game` line budget +39 (`solid_budget.txt`); A/B vs 010444d84: ColonyHour 196.2 to 194.8 ms (p=0.39, noise).
- HUD: the Plugins detail shows per-file fuel, calls and memory.
- Saves: `ModState` in the save, SaveVersion 18 (read for v18+; older saves load with no mod state).
- Determinism: same state hash with Starlark or Wasm twins; no wall clock or global RNG in the wiring.
- Boxes: MODDING gap row PLG-063 `[x]`; PLG-051 `[-]` (Wasm blobs shipped; Starlark has no state, general plugin blobs open); MASTER_SPEC §49 shipping note; THIRD_PARTY_NOTICES wazero row (wired); feature checklist 38.3 **Second runtime spike: WebAssembly on wazero** stays `[x]` (note updated); issue #7: Closes #7 in the docs commit.
- Open: the contributor's own Rust/TinyGo harness; Starlark plugin state and general plugin blobs (PLG-051); `contentlint` does not lint `.wasm` files; gob decodes `ModState` before its bounds apply (savefmt decompressed-size cap unaudited, as for every save field).

## PLG-026-saved-filter-identity
Historical saved plugin filter identity · 2026-11-25 · this commit
- Changed: the item remap walks saved filter bits in their original key namespace, including slots now occupied by new core kinds. Identity/no-key paths and the None sentinel retain their prior policy.
- Evidence: exact upstream v20 fixture, removed/reordered content, native storage admission and re-save/reload; no schema bump or enum renumbering.
- Boxes: vanilla **Storage filters** [x] annotated; PLG-044 remap guarantee clarified.
- Open: none within this historical filter migration scope.

## PLG-064-stable-plugin-extension-api
Stable plugin extension API: versioned manifest, capabilities and extension points · 2026-11-27 · this commit
- Changed: `plugin.json` gains `api` (1) and `capabilities` (closed set of 13; legacy plugins default to query, commands, events with a warning); new packages `application/pluginapi`, `application/scripting/gameread`, `domain/pawn/stat/statmod`. Scripts get world queries (`ctx.animals()`, `ctx.buildings()`, `ctx.map`, `ctx.world`), `register_defs`, `validate_defs`, `validate_command`, `EVENTS` + `on_event` (quadrum, year, research_done, recipe_done, map_generated), `panel`/`on_action`, `run_incident`, `ctx.order`, `ctx.place` and `stat_mod` defs. Script-writer reference: `internal/application/scripting/doc.go`.
- Decisions: legacy default caps keep old mods running; deny by default, enforced in `starlarkhost/caps.go` (`mayUse` kept as defence in depth); script orders at most once per pawn per 600 ticks (`TestExtScriptOrderNoFlap`); queries read the home map and cap at 256 rows; `statmod` and `gameread` import their parent, allowed in `internal/arch`; lang catalog regenerated; contentlint unchanged.
- Limits: `solid_budget.txt` `application/game` 52650 to 52968 (+318); A/B `-race` count=8: ColonyHour p=0.80, TickGrown/120 p=0.13, /250 p=0.23, /500 +0.96% p=0.065, allocs/op equal. ebiten root 12179, within its 12180 ceiling (no raise).
- CI: `TestStartWiresExtensions` and `TestExtE2E/AC5_quadrum_year` jump with `SkipTo` to the hour before each quadrum instead of simulating 16 and 60 days; `compose/game` `-race` 50 s to 9 s locally (base 5 s), same assertions plus a no-early-event check.
- HUD: Plugins screen detail shows API and capabilities; in-colony "Plugins" main tab shows plugin panels and actions.
- Saves/determinism: no save field; registrations, stat mods and panels rebuild from plugins on load (panels empty until the next in-game hour); `TestExtE2E/Deterministic` equal hashes; stat mods survive load (`TestExtStatModsSurviveLoad`).
- Boxes: feature checklist 38.1 **Plugin API version**, **Capability declaration**, **Read-only world query API**, **Definition registration hooks**, **Event subscription**, **Custom validation hooks**, **Custom UI panels/actions where safe**, **Custom incident behavior**, **Custom job/intent providers**, **Custom stat modifiers** `[x]`; **Custom health-effect behavior**, **Custom recipe/building components**, **Custom world-generation hooks** `[-]` (what is left is in the row); 38.2 **New day/quadrum/year**, **Research completed** `[x]`, **Item produced/consumed** `[-]`; MODDING gap rows PLG-060 `[-]` (extended), PLG-070 `[-]` (capabilities shown; no search or filter).
- Open (follow-up, not yet filed): contentlint does not lint script-registered defs or validations; Wasm capability enforcement and Wasm extension hooks; health conditions and per-tick health effects; building components; terrain, biome and planet generation hooks and later-generated maps; panels not persisted across load; a capability name's length is not capped in the loader (only the list length).

## PLG-026-thought-class
Thoughts: core patch only; plugin-added thoughts refused · 2026-11-26 · this PR
- Changed: `thought/core.<key>` is a moddable class. A patch sets label, value, duration (remembered thoughts only), stack_limit and stack_factor; the example plugin sets `core.slept_outside` to -2. Every game read of a thought goes through `mood.Book` (`thoughtsOf(g)`).
- Decisions: the 23 code-valued kinds (`mood.CodeValued`) cannot be patched. Plugin-added thoughts are refused with a problem: no data-driven channel gives them to pawns (F1). Bounds are in `modding/AGENTS.md`. Core thought IDs are the saved kind numbers, so a patch adds no save key; a save made with the patch loads without the plugin, and values and expiries baked at add time persist until they expire. Core thought memories stay byte-identical.
- Decisions (tooling): `TestNoStaleAgentPaths` in `internal/arch/agents_test.go` skips `.loops` (`guardSkipDirs`), because it read loops' git-ignored status file. F2: the guard should walk `git ls-files` instead.
- Limits: `application/game` non-test lines 52650 → 52649; `solid_budget.txt` unchanged. Mood benchmarks (`-race`, interleaved base and head, benchstat): no sub-benchmark slower at p<0.05; 0 allocs/op everywhere; noise about ±20%.
- Boxes: feature checklist **Definition classes** `[-]` (thought: core patch only, F1 open); MODDING gap row PLG-026 `[-]` (thought class added, F1 open). Ticket arxdsilva/rimworld-go#17 stays open: more classes remain.
- Open: finding F1 (backlog supervisor files the issue): a trigger channel for plugin thoughts and a save encoding by namespaced key for plugin thought kinds. Finding F2 (backlog supervisor files the issue): arch guard walks `git ls-files`.
