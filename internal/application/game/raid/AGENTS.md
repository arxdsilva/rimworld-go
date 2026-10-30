# raid package

## Package Purpose
Threats: the legacy storyteller's die and raid plans, the raider AI (arrive, assault, siege camp, flee), predator packs, hive swarms and the rock sappers dig through. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by DEF-4 step 1 as a pure extraction of the game's `raids.go`, `raid_plans.go` and `hive.go`; the gap backlog is `.cursor/DEFENCE_GAP_ANALYSIS.md`.

## Owned Concepts
- `Raid` (raid.go): the root, built by `New(Host, Roles)`; components as exported fields, no methods of its own.
- Types the game stores and saves: `Raider` (the save's `Raiders` map values), `Group` (`RaidGroups`), `Plan` (`Assault`, `Siege`, `Sappers`; `String`), `Mode` (`Arriving`, `Assaulting`, `Fleeing`). Field names are gob names: never rename.
- `Raiders` (raiders.go): `Think(id, now)` (the threat phase, per raider: prisoners handed to the host, dead and downed raiders, flight, siege camp and dig-in, breach, assault, turret targeting), `Camp(ex, ey)` (a siege's camp tile).
- `Threats` (threats.go): `PickThreat` (pure: the die's faces), `Fire(now)` (the legacy storyteller; returns the next threat tick), `Plan(day)`, `WolfPack(n)`.
- `Hives` (hive.go): `Swarm`, `SpawnNear` (also the game's incidents), `Exit` (cached per grid, grid version, structure version and home).
- `Rock` (rock.go): `Dig` (combat's breach calls it through the game), `Export`/`Import` (the save's `RockDamage`).
- DEF-4 (R1): `Plan` is the domain's `raiddef.Strategy` (Assault, Siege, Sappers, Stage, Steal, Burn; saved byte unchanged). `Threats.Plan` picks by `raiddef.PickStrategy` and `Threats.Settle` (called by the game's `raidFrom`) rolls the group's flee share and, for a storyteller pick only, fits the strategy to tech and points and rolls the arrival. `Threats.Major` splits a raid face into a crashed ship part (wiki 4.0 : raid 7.4), a mechanoid raid (1 against each hostile human faction from 300 points) or people; `MechRaid` calls R2's `Mechs.Raid`.
- `Pods` (pods.go, edge.go): `Drop` plans one pod per raider (centre, edge, scattered), `Step` lands them (`Arrive.Land`: roof break, shell, raider), `Views` (hostile `ArrivalView`s), `Left`; `EdgeTile` (moved from the game). `NearestEdge`, `Warning`, `Announce` are pure helpers.
- Raider behaviours: staging (stage.go), thieves, burners and rival factions (mischief.go), siege mortars (`crew`, `mortarNear` in raiders.go), retreat at `raiddef.Broken` (logged once), the combat log through `Chronicle`.
- CBT-4 (tactics.go): threat-weighted targets (`target`: distance less a danger bonus, gun 8, turret 6, melee weapon 4 tiles), cover under fire (`cover`, `advance`, `covering`; `Raider.CoverAt/CoverX/CoverY`), grenade dodging (`dodge`, combat's `Tactics.Danger`), loot grabbed on the way out (`grab`, within 8), a prisoner breaking out (`escape`, `Raider.Escaping`: fights a colonist beside it, bashes through, flees), siege resupply pods (`resupply`, `Group.Supply`: a day apart, 12 survival meals and full mortar racks). Burners go for sown crops too (`crop`, mischief.go). Staged raids storm on sight or at their 10-15 h assault timer (stage.go).
- CBT-4 mechanoids (mech.go): bash through walls and shut doors on their way to a target or into the colony (`bash`, `MechRecord.Bash/BashX/BashY`, combat's `Tactics.Block/Bash`); mechanoid raids by drop pod (`podRaid`, `landPods`, `Pod.Mech`, `Arrive.Crash`; stepped by `Pods.Step`); they never retreat (wiki: Raider, mechanoids attack indefinitely).
- CBT-4 hive mounds (nest.go): `Hives.List`, `Step` (from `Pods.Step`); an infestation in a dark, warm, roofed cave raises a `construction.HiveMound` whose swarm is its brood; it breeds (a dormant insect a day, up to 3), spreads (every 3-4 days within 10 tiles, never beside another, at most 30), wakes its brood when a colonist comes within 10; destroyed, it is forgotten. Saved in `MechSave.Hives` (and `MechSave.Pods`).
- Kidnapping (kidnap.go, moved from the game's kidnap.go by CBT-4): `Kidnaps.Try`, `Step`, `End` (`KidnapRange`, `KidnapScan`, `KidnapCooldown`).

## Allowed Dependencies
- Domain value types (`ai/job`, `animal`, `combat/weapon`, `pawn/health`, `incident`, `sim/clock`, `sim/entity`, `world/grid`, `world/temperature`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game`, adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go`: `Host` embeds eight value-only roles, each ≤ 8 methods: Roster (7), Captives (4), Bodies (7), Jobs (5), Fight (8), Ground (8), Events (2), Dice (1). `Roles` carries Wild (6), Story (6), Mech (R2), the DEF-4 roles in roles.go: Arrive (4, with `Crash`), Mischief (5), SiegeWorks (3), Chronicle (1), and the CBT-4 roles Tactics (8, tactics.go), Nest (3, nest.go), Kidnap (8, kidnap.go). The game implements them in `internal/application/game/raid_host.go` (Jobs and Dice reuse combat's `combatJobs` and `combatDice`); none adds a method to `game.Game`.
- On the game, behind roles: the raider records and raid groups (`g.raiders`, `g.raidGroups`, `g.raiderIDs`; prisoners, visitors, kidnappers, caskets and factions write them too), `g.nextThreat`/`g.threatsOn` (story.go writes them), spawning raids (`raidFrom`: people, factions, relatives), `edgeTile`, `nearestColonist`, `removeRaider`, the read model (`raiderViews`), wealth and raid kits, and the sapper path maps.
- Numbers: original tuning (raid gaps, siege setup, camp range, sight range, rock hit points, hive swarm sizes, pod fall and roof crush, steal minimum, burn count, strategy bands; CBT-4: danger bonuses, cover radius 12, margin 150, hold 600, look 60, bounding advance, retreat loot reach 8, resupply interval, hive breeding/spreading intervals and brood, dark below 30 % light). Wiki (ENG-003, `domain/raid/rules.go`): map entry weights, flee share 40-70 %, siege storm 1.5-3 days and 8 % per hit, two siege mortars, big-threat weights; CBT-4: staged assault timer 10-15 h, siege resupply 12 meals, hive 130 HP, spread within 10 tiles, cap 30, aggro radius 10, -17 °C.

## Invariants
- Every roll comes from `Dice.Roll` (the game's main stream), in the order the game rolled before the extraction: the benchmark state hashes did not move.
- No map iteration; raiders are visited in the game's `raiderIDs` order.
- Hysteresis (CBT-4): a raider moves to cover only for `coverMargin` more and then holds `coverHold` (`TestRaiderCoverDoesNotFlap`); a mechanoid keeps bashing its one block until it is down; hives look every 60 ticks.
- Hysteresis: a raider that found no way in waits `RaiderRetry`; a fleeing raider never returns to the fight (`TestRetreatAtFleeShareNoFlap`); a siege holds its camp until its storm; a staged raid storms once and a raider walks back only past `StageWander`, and the group storms on sight of a colonist or at its 10-15 h assault timer (`TestStagedRaidStormsOnceNoFlap`); thieves and burners retry on `RaiderRetry` cooldowns.

## Tests
- Unit tests here: `raid_test.go` (threat odds, plan names, rock damage with save round trip).
- Whole-game tests stay in the game: `raids_test.go`, `raid_plans_test.go`, `raid_def4_test.go` (pods, staging, retreat, sieges with mortars, steal, burn, rivals, IEDs, combat log, saves), `raid_cbt4_test.go` (cover and its no-flap, threat weighting, grenades, retreat loot, crops, resupply, escapes, mechanoid bashing and pods, hive mounds, threat response), `hive_test.go`, `kidnap_test.go`, `prisoner_test.go`, `turret_test.go`. They cover this package to about 85 % (`-coverpkg`).

## Hot Paths
- `Raiders.Think` runs per raider per tick: value host calls only (one map lookup for the record, one for the vital record, one job read). The rivalry pass is once a tick over the raiders; a rival scan per raider only while rival factions share the map; staging checks every 30 ticks per raider. `Pods.Step` loops the groups once a tick.
