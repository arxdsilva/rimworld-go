# Enemy AI gap analysis (EAI-0..EAI-4)

Scope: how animals, raiders, visitors, factions and the storyteller choose targets, hold their ground, and are sized and scheduled, at RimWorld-wiki fidelity (vanilla core only, no DLC). Load only for EAI-* tasks. Phase handoffs are the `EAI-0..4` sections of `.agents/handoffs/DEF.md`. Sister documents: `DEFENCE_GAP_ANALYSIS.md` (weapons, cover, turrets, the raid plans this builds on) and `COMBAT_GAP_ANALYSIS.md`.

## Status (2026-11-09)

Every row in section 3 is `done` at HEAD, with one qualification: EAI-007's two journey tests were not written; the whole-game test and the unit tests carry the proof (section 4). Performance: section 5.

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for a gap analysis of "enemies AI (animals attacking, raids and anything else)" and then for every gap found to be implemented. |
| What existed before | Revenge and tame-fail rage; manhunter and predator packs; mad animals and charges; hives; seven raid plans and four arrivals; cover and retreat; kidnap, steal and arson; allied and turned visitors; mechanoids; a storyteller with a threat budget and a nature pool. |
| What was missing: animals | Animals never broke doors. Manhunters chased only colonists, by map-wide distance. Herds and packs did not defend each other. Predators skipped downed colonists and nobody defended livestock. Pack sizes ignored threat points. A grieving tame manhunter was never engaged. |
| What was missing: raiders and factions | Raiders knew where every colonist was without sight. Neutral visitors did not fight back. Nobody ran from a sparking turret. Threat points had no cap. Wealth was a global scan with no history. NPC factions had no enemies of their own. The stage, steal and burn plans had no alert label. |
| What was missing: cost | The standing check was O(R*(R+V+P)) a tick and the rival check O(R^2). Target picks ran every tick. `NearestThreat`, thief and burner scans were linear. |
| What was missing: storyteller | No incident queue, no letter choices, no repeat limit on messages, no positive-event bias, no debug panel; infestations searched near one rock face only. |
| Kept as not-vanilla | Flanking, kiting, executing downed colonists, a morale stat, mech retreat, and DLC breach raids. Vanilla core does none of them; they are recorded here as kept out, not as gaps. |
| Numbers and clean room | ENG-003 and the DEF exception in `.agents/specs/CLEAN_ROOM.md` allow wiki numbers as tuning data. This programme did not need them: the pack-size cost (`animal.ThreatCost`), the colonist wealth curve and `PointsCap` are formulas of our own, marked original tuning. Names and prose are original. |
| Determinism | No new RNG stream. `faction.SetEnemies` hashes factions' ids, types and names. The incident queue is saved and fires in sequence order. The wealth sweep's cursor, hour and partial sum are saved, so a mid-sweep save loads into the same hash. |
| Hysteresis | Every threshold or timer behaviour starts and stops apart (lock and give-up timers, switch margins, goodwill cooldown, hold-and-release bands) and has a no-flap test; section 3 names it per row. |
| Save | Format v17: `migrateSave` seeds the wealth tracker's first map and fills `Faction.Enemies` for older saves. Schema golden refreshed. |
| Package budget | Game-package growth stayed small (36e7840ec: 51148 -> 51217 lines, 1751 -> 1753 `Game` methods; 5c8c4ca07: 33 lines, no new `Game` methods). New code went to `game/rage`, `game/raid`, `game/combat`, `game/wealth` and `domain/*`. |

## 2. Sources and reading the tables

Read in our own words, with no quoted text and no proprietary numbers: the RimWorld wiki pages on Raids, Raid points, Manhunter, Animals, Predator, Infestation, Siege, Factions and Storytellers. What was taken from them is behaviour (what a manhunter, predator, raid or storyteller does and in what order), not data. Our numbers are our own: `animal.ThreatCost`, the colonist wealth curve (`difficulty.ThreatPoints`) and `PointsCap`.

`State` is `done`, `partial`, `open` or `out of scope`. Evidence names a path and a function or test; line numbers drift, so find the symbol. Every test name below was checked with grep against HEAD on 2026-11-09. Paths are under `internal/application/game` unless a full path is given.

Commit key: contract 36e7840ec; animals 5c8c4ca07; raids and perf c334e2e9e; points, wealth, factions, visitors f8b8c4a79 and ab42be706; storyteller and HUD 26621c22c, 2f4bc917f, 44af54b65, 61967cbd7; lead wiring 505e52d8c, 992629ff5, 1a5e82070, 24c080e11, cc8ba7a0d, 3cee1b30a.

## 3. Gap tables

### 3.1 Animals (EAI-001..007)

| ID | Missing | Ships (evidence) | Commit | State |
|---|---|---|---|---|
| EAI-001 | Animals never broke doors or walls. | `rage/bash.go`: a shut-out manhunter, hunting predator or hive insect finds the first door on its way (an insect also a wall), holds it in `Memory.Bash`, stands by it and strikes it; gives up after `BashGiveUp` (2500) and waits for the path retry; a door held open and unforbidden drops the bash. Tests `TestAnimalBashNoFlapOnHeldDoor`, `TestManhunterBashesShutDoor`, `TestInsectBashesWallWithoutDoorRoute` (`eai_bash_test.go`). HUD: `AnimalView.Bashing` in the inspector. | 36e7840ec, 5c8c4ca07 | done |
| EAI-002 | Manhunters chased only colonists, by map-wide distance. | `rage/target.go`: standing people (colonists, guests, raiders) and tame animals through the creature index in widening rings, reachable first; locked for a time and switched only for a foe a set margin closer. Tests `TestManhunterRetargetNoFlap`, `TestManhunterTargetsGuestsRaidersLivestock`, `TestManhunterPrefersReachable`. HUD: the hostile alert names any target. | 5c8c4ca07 | done |
| EAI-003 | Herds did not defend each other. | `rage/herd.go`, called from `animalHurt`: a hurt wild animal calls its herd once per call interval; fighter mates within the herd radius join, grazers flee. Tests `TestHerdDefenceNoFlap`, `TestTerritorialHerdJoins`, `TestGrazerHerdFlees` (`eai_herd_test.go`). HUD: `AnimalView.Defending`. | 5c8c4ca07 | done |
| EAI-004 | Predators skipped downed colonists; nobody defended livestock. | Predators take a downed colonist where the difficulty allows hunting people; the shared `hostile()` rule from the contract makes a predator hunting tame animals a foe for colonists, guard animals and turrets. Tests `TestPredatorTakesDownedColonist`, `TestColonistDefendsLivestock`, `TestGuardAnimalDefendsLivestock`, `TestLivestockDefenceNoFlap`. HUD: the "X is attacking Bessie" message. | 36e7840ec, 5c8c4ca07 | done |
| EAI-005 | Pack sizes ignored threat points. | `domain/animal/pack.go`: `ThreatCost` (our formula from the `Def` alone) and `PackSize(points, species)`; `ManhunterPack` and `WolfPack` pick one species of the map's biome and season and send as many as the points buy (a starving pack two at least). Tests `TestPackSizeScalesWithPoints` (`domain/animal/pack_test.go`), `TestManhunterPackByBiome` (`eai_pack_test.go`). HUD: the letter's animal count. | 5c8c4ca07, ab42be706 | done |
| EAI-006 | A grieving tame manhunter was never engaged. | `guardFoe` and `Strikes.foe` count a tame manhunter as a foe; turrets engage it too (`TestTurretTargetsTameManhunter`, `combat/eai_b_test.go`); the rage ends on `ManhunterTicks`. Tests `TestGrievingPetIsEngaged`, `TestGrievingPetCalmsNoFlap`. HUD: `AnimalView.Grief`. | 36e7840ec, 5c8c4ca07, c334e2e9e | done |
| EAI-007 | No whole-game proof of the animal chain. | `TestFailedTamingRage` (`eai_pack_test.go`) runs a whole game through a failed taming. The planned journeys `TestJourneyEAIManhunterPack` and `TestJourneyEAIPredatorCharge` do not exist at HEAD; the unit tests above stand in. | 5c8c4ca07 | done (journeys not written) |

### 3.2 Raids (EAI-010..016)

| ID | Missing | Ships (evidence) | Commit | State |
|---|---|---|---|---|
| EAI-010 | Raiders knew where every colonist was. | `raid/sight.go` (`Senses`): a raider knowing of no foe walks on `Home()` in legs until it sees one; it remembers a seen foe for 600 ticks and its group the foe any mate saw for 1200; a raider wounded by a colonist or ally turns on the shooter. `Raider.Foe/FoeAt/Felt/Advancing` and `Group.Foe/FoeAt` are saved. Tests `TestRaiderTargetMemoryNoFlap`, `TestRaidersAdvanceUntilSeen`, `TestGroupMateSightShared`, `TestShotRaiderTurnsOnShooter` (`raid_eai_test.go`), `TestRaiderMemoryWindows` (`raid/eai_b_test.go`). HUD: `PawnView.Advancing`, "Advancing on the colony" in the sheet activity line (`dossier/sheet/lines_misc.go`). | 36e7840ec, c334e2e9e, 24c080e11, 61967cbd7 | done |
| EAI-011 | Neutral visitors and trader guards did not fight back. | `Rec.Defend` (saved): visitors and guards engage a raider or hostile animal within 10 tiles (16 when just hurt; the trader only when hurt), hold at least 600 ticks, let go when none is within 16, and look staggered by id. Tests `TestVisitorDefenceNoFlap`, `TestDefendStaggered` (`visit/defend_test.go`), `TestVisitorsFightNearbyRaiders`, `TestTraderGuardsDefend` (`visit_defend_test.go`). HUD: the "joining the fight" message. | ab42be706 | done |
| EAI-012 | Nobody ran from a sparking turret. | `combat/turret_step.go`: a sparking turret is `Danger` (reach+1); raiders and colonists in its blast step out to reach+2; one idle in the band holds and a job into the blast is dropped. Tests `TestTurretDangerStepNoFlap` (`combat/eai_b_test.go`), `TestRaidersStepFromSparkingTurret`, `TestColonistsStepFromSparkingTurret`. HUD: the "Turret about to explode" danger alert (`BuildingView.Sparking`, `hud/alerts_power.go`). | c334e2e9e, 61967cbd7 | done |
| EAI-013 | Threat points had no cap and ignored wealth. | `difficulty.ThreatPoints` caps at `PointsCap` (10000) and, unless the difficulty ignores wealth, weights each colonist by a curve of our own (20 points at no wealth, 25 at 20k, 40 at 100k, 60 from 300k, linear between); with `IgnoreWealth` a colonist stays flat 25. Tests `TestThreatPointsCapped`, `TestColonistPointsByWealth` (`domain/difficulty/difficulty_test.go`); `TestThreatPointsFormula` re-derived. HUD: story debug panel. | f8b8c4a79 | done |
| EAI-014 | Wealth was a global scan with no history. | `game/wealth.Tracker`: chunked sweep every `Stride` (25) ticks, one full pass per in-game hour, 60 daily samples kept, partial sum saved; `Wealth()` returns `Value()`. Tests `TestWealthTrackerMatchesFullScan`, `TestWealthSaveMidSweepHash` (`wealth_track_test.go`), `wealth/wealth_test.go`. HUD: the History window's Wealth tab. | 36e7840ec, ab42be706, 2f4bc917f, cc8ba7a0d | done |
| EAI-015 | NPC factions had no enemies of their own. | `domain/faction/wars.go`: `SetEnemies` from a hash of ids, types and names (no stream draw), wars mutual and between human peoples only; older saves get the same rule in the v<17 migration. Beating a raid of faction X thanks each standing enemy of X once, then not again for `EnemyCooldownDays` (3). A faction's military aid queues a rival raid later through the EAI-030 queue. Tests `TestFactionEnemiesDeterministic`, `TestEnemyGoodwillCooldown` (`domain/faction/wars_test.go`), `TestRivalIsAnEnemy`, `TestRivalRaidFromEnemy`, `TestEnemyRaidBeatenThanks`, `TestNewWorldHasWars` (`faction_wars_test.go`). HUD: the Factions window's Enemies line. | 36e7840ec, ab42be706, 992629ff5, 1a5e82070 | done |
| EAI-016 | Stage, steal and burn plans had no alert label. | The raid alert names the plan; the sheet's raiding line localises the plan word. Test `TestRaidAlertNamesPlan` (`hud/alerts_plan_test.go`). Catalog keys in 3cee1b30a. | 26621c22c, 3cee1b30a | done |

### 3.3 Raid and combat cost (EAI-020..023)

| ID | Missing | Ships (evidence) | Commit | State |
|---|---|---|---|---|
| EAI-020 | `broken()` was O(R*(R+V+P)) and `rival()` O(R^2) per tick. | `raid/tally.go`: standing raiders per group and faction, and whether rivals share the map, counted once a tick. Test `TestBrokenCountsOncePerTick` (`raid/eai_b_test.go`). Flee stays one-way. | c334e2e9e | done |
| EAI-021 | Target picks every tick. | Picks every 15 ticks offset by id; `nearestColonist` walks index rings past 32 colonists (`spatial.Nearest`). Equivalence test `TestNearestColonistRingsMatchScan` (`raid_eai_test.go`). | c334e2e9e | done |
| EAI-022 | Linear `NearestThreat`. | Asks `Near(..., Raiders)`, lower id on ties. Test `TestNearestThreatIndexMatchesScan` (`combat/eai_b_test.go`). | c334e2e9e | done |
| EAI-023 | Linear thief, burner and kidnap-guard scans. | Thieves pick loot through the food grid, burners through a wood index keyed on the structure version, kidnap guards through `Colonists(r=1)`; cooldowns kept. Tests `TestThiefPicksSameAsScan`, `TestBurnerIndex`. | c334e2e9e | done |

### 3.4 Storyteller (EAI-030..035)

| ID | Missing | Ships (evidence) | Commit | State |
|---|---|---|---|---|
| EAI-030 | No incident queue. | `incident.State.Enqueue/PopDue` keep the saved queue soonest first, ties in queue order; `phaseIncidents` fires each due entry; `Game.QueueIncident` and `Game.QueueRaid`. Tests `TestIncidentQueueFiresAtTick`, `TestQueueSurvivesSave` (`incident_queue_test.go`). HUD: story debug panel. | 36e7840ec, 26621c22c | done |
| EAI-031 | No letter choices. | `letters.Choice` rendered as `AnswerLetter` buttons by `letterFooter`; `command.AnswerLetter` routed (505e52d8c); the `RefugeeChased` event (Nature pool, setup colonies with a hostile faction): accept gives a joiner and a queued raid half a day on, decline sends them away, a plea lapses after a day. Tests `TestLetterChoiceAccept`, `TestLetterChoiceDecline`, `TestJourneyEAILetterChoice` (`ebiten/journey_eai_test.go`). | 36e7840ec, 26621c22c, 505e52d8c | done |
| EAI-032 | No repeat limit on messages. | `post` merges a repeated plain notice within 600 ticks into one with `MessageView.Repeat` (the HUD shows xN); danger messages and letters never merge. Tests `TestRepeatedMessageMerged`, `TestDangerNotLimited` (`message_limit_test.go`); `TestCasketEmpty`-style counts adjusted by Repeat (44af54b65). | 26621c22c, 44af54b65 | done |
| EAI-033 | No positive-event bias. | `incident.Def.Helpful`, `Odds.Helpful`; `pickNature` draws the storyteller's `PositiveBias` once more on a setup colony and then only among helpful events; a legacy colony draws as before. Tests `TestPositiveBiasApplied`, `TestLegacyNoExtraDraw` (`story_bias_test.go`). | 26621c22c | done |
| EAI-034 | No debug panel. | `devtools/story.go`: queue, next threat, points, wealth, and a button to fire an incident, from `Game.StoryDebug`. Test `TestStoryPanelLists` (`devtools/story_test.go`). | 36e7840ec, 26621c22c | done |
| EAI-035 | Infestations searched near one rock face only. | `Hives.Spots` lists every hive spot of the map nearest home first (cached by structure, home and day); `Hives.Infest` raises a hive there or reports false so the story sends a raid instead; `Swarm` keeps the rock-face burst. Tests `TestInfestationDarkRoofedWarm`, `TestInfestationSkipsFreezing` (`infest_test.go`). | 26621c22c, ab42be706 | done |

## 4. Known leftovers

- `rage.Memory.Call` keeps one entry per herd (`rage/memory.go`). It is bounded by the number of herds, so it is left as is.
- The legacy storyteller (`raid/threats.go`, `Threats.Fire`) keeps its old draws, including `(2 + Roll(2)) * 100` points for its wolf pack; the points-based sizing applies to the new storyteller and the scenario.
- EAI-007: `TestJourneyEAIManhunterPack` and `TestJourneyEAIPredatorCharge` were planned but are not in the tree; `ebiten/journey_eai_test.go` holds only `TestJourneyEAILetterChoice`.

## 5. Performance

- B measured `BenchmarkTickRaid` about 20-30% faster per tick, and target picks went from every tick to one in 15.
- cc8ba7a0d: the wealth tick is within noise on the `BenchmarkTick*` runs.
- Perf (lead): TBD

## 6. Benchmark and pin moves (intentional)

| Commit | Move | Cause |
|---|---|---|
| 36e7840ec | Seed 1 benchmark hashes and seed 7's 120 fixtures | A wild predator hunting tame animals is now hostile, so colonists, guards and turrets answer it (bisected to the tame-animal target alone). |
| 5c8c4ca07 | `TestHabitsOffKeepTheHash` re-pinned | A colony with every habit off now sees a herd call (seed 1, tick 46605, saved in `Wildlife.Rage`); the run diverges from there. |
| cc8ba7a0d | `TestHabitsOffKeepTheHash` re-pinned again | The wealth tracker's cursor, hour and partial sum are saved state. |
| 1a5e82070 | `saveV12Hash` 0x2a89537689b1cc80 -> 0x3c352207ecddf6c2 | The v<17 migration now fills factions' wars. |
| c334e2e9e | Raid fixture (planned) | Sight-limited advance and 1-in-15 picks change raider movement. The commit body does not record a pin change. |
| ab42be706 | Packs sized by threat points | Storyteller and scenario packs no longer roll 200..500; wealth and war state join the save. |

## 7. Out of scope and kept as vanilla

| Item | Reason | State |
|---|---|---|
| Flanking, kiting, executing downed colonists, a morale stat, mech retreat | Vanilla core does none of these | kept (not a gap) |
| Breach raids and breach weapons | DLC | out of scope (DLC) |
| Mech clusters and mechanitor content | DLC (Biotech) | out of scope (DLC) |
| Plugin-driven storytellers | No mod system exists | out of scope |
