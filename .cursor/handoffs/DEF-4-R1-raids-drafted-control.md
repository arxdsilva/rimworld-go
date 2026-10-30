# DEF-4 (R1): raids, drafted control, IED leftovers, combat log

Date: 2026-10-29. Wave E, agent R1. Backlog `.cursor/DEFENCE_GAP_ANALYSIS.md` §3.1, §3.5, §3.7, §3.8. Built on R1a's `game/raid` extraction; mechanoids and ship parts are R2's.

## What changed
- **Domain** `internal/domain/raid/rules.go`: `PickArrival`, `PickStrategy`, `Fit`, `FleeLoss`, `Broken`, siege, staging, pod and target-priority constants (100 % covered).
- **Drop pods** (`game/raid/pods.go`, `edge.go`): one pod per raider, centre (around the colonist nearest home), edge or scattered; 240-tick fall, roof break with crush, pod shell, raider steps out; hostile `ArrivalView`s (same Tick/Landed fields as the colony's pods). `edgeTile` moved into the raid package. Dev hook `RaidDropPods(kit ...item.Kind)`.
- **Strategies**: staged assault (`stage.go`), siege with two raider-crewed mortars and the wiki storm (`raiders.go`, `combat/mortar.go` `Raise`/`Crew`/`Man`), Steal and Burn (`mischief.go`), rival hostile factions fight each other, retreat at the rolled flee share, give-up day from the storm. `Threats.Settle` fits a storyteller pick to tech and points and rolls the arrival.
- **Big threats** (lead request): a raid face of either storyteller splits into a crashed ship part (`FireIncident("crashed_ship_part")`), a mechanoid raid (`Mechs.Raid` with a rolled arrival) or people, by `Threats.Major`.
- **Drafted control** (`combat/draft.go`): fire at will, hold position, grenade belts and `ThrowGrenade`; `PawnView.FireAtWill/HoldPosition/Grenades`.
- **IEDs**: bullets light a charge, blasts chain, sappers know 30 % of charges (`sapperTraps`).
- **Combat log** (`combat/log.go`): 64-entry ring, saved, `WorldView.CombatLog []CombatEventView{Tick, Text}`.

## Wiki vs original
Wiki: map entry 45+15/30 (+10 haywired, mechs only), flee share 40-70 %, siege 1.5-3 days, 8 % per hit, two mortars, IED bullet trigger, sapper 30 % trap knowledge, grenade warmup and forced miss, big-threat weights (raid 7.4, ship parts 2.0 + 2.0, human factions 1 each, mechanoids 0 below 300 points). Original: pod fall 240, roof crush 15, pod split, staging 10 tiles / 3 h / 6-14 h, strategy bands, siege 300 points and 32-tile camp, shell split 15/5, steal 50 silver, 3 fires.

## UI
Read models only (U5 owns the screens): `PawnView.FireAtWill`, `HoldPosition`, `Grenades`; `WorldView.CombatLog`; hostile `ArrivalView`; `PawnView.RaidPlan` names the strategy. UI checklist items 13.2 and 24.3 are left to U5.

## Tests
`domain/raid/rules_test.go`; `game/raid_def4_test.go` (13 tests, including the no-flap tests for staging, retreat and fire at will); `combat/draft_test.go` (5). Coverage: domain/raid 100 %, game/combat 95.2 %, game/raid 32 % on its own tests and about 85 % from the game tests (`-coverpkg`). Bench x3 frame p99: grown 1.54/1.48 ms, raid/250 1.81-1.91, raid/500 1.86-2.16 ms (R1a: 1.80).

## Gaps
Opportunistic theft on retreat, burning crops, the 10-15 h assault timer, siege resupply, mech groups landing by the pod hook (they use R2's arrival spot), walking into grenade range, threat-weighted targeting and cover seeking.

## Integration notes (lead, wave E)
- Journey fixes: the fire-at-will journey runs on a 48x48 map so the raider spends time in rifle range before melee; the mech journey arms the colonist before the mech lands and expects a wreck, not steel. All wave E journeys pass (`journey_wave_e_test.go`).
- The wave E contract inertness test became `TestDraftDefaults` (`draft_defaults_test.go`): fire at will on, no hold position, once the commands were live.
- `TestGrownGolden` re-pinned (wave E reason). Game budget lowered to 54474 lines, 180 files, 1935 `Game` methods. Full `-race` suite green.
- `BenchmarkDrawArt500` unchanged against wave D (407/548/266 draw calls, 3228/2983/2223 allocations).
- UI for drafted controls, the combat log and the hostile pods landed in U5 (UI checklist 13.1, 13.2, 24.3); mechanoids are in `DEF-4-R2-mechanoids-ship-part.md`.
