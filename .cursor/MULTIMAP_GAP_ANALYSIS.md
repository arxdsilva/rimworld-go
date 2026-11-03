# Multiple maps gap analysis (MAP-0..MAP-4)

Scope: several maps open and ticking at once (the colony, mineral sites, attacked settlements, founded colonies and caravan encounters), the caravan and pod rules that open them, and the UI that switches between them, compared with core vanilla RimWorld 1.5 (no DLC). Load only for MAP-* tasks. Phase handoffs go in `.cursor/handoffs/MAP-N-*.md`. AX-9 (transport pods, caravans, the mineral scanner) refused all of this as needing "a second map". Sister document: `WORLDGEN_GAP_ANALYSIS.md` (WG-03 supplies the movement difficulty that MAP-09 uses). Sources: rimworldwiki.com pages Caravan, Colony, Faction base, Long-range mineral scanner, Transport pod, Ending, Goodwill (2026-11-02). Names and prose are ours.

## Decisions

| Item | Decision |
|---|---|
| Numbers | ENG-003 is extended to MAP-* (approved 2026-11-02 and recorded on the `AGENTS.md` ENG-003 line): published wiki numbers, original names and prose. Each constant gets `// wiki: <page>`, or `// original tuning`. |
| Features | The user chose all four: site maps, attacking settlements, founding a second colony, encounter maps. |
| Ticking | Every open map ticks every tick, as vanilla does. |
| Architecture | The per-map half of `Game` moves into a `mapState` bundle that `Game` embeds, so field promotion keeps `g.grid` compiling; lifecycle lives in the new sub-package `game/maps` behind consumer-owned hosts of at most 8 methods. `application/game` is at its SOLID ceiling, so added lines are paid for by moving the caravan state machine out. |
| Saves | Map 0 stays at the top level of the save; extra maps are additive (`ExtraMaps`); `SaveVersion` 11 to 12 with a no-op migration. A single-map game hashes as today. |
| Open questions (settled by the user, 2026-11-02) | Abandoning the last colony ends the game with a modal (MAP-14). Settlement turrets fire on colonists and the player can't power or claim them (MAP-13). Caravans attack neutral bases as vanilla does (MAP-15). |
| Original tuning (wiki silent) | Ticks per tile and winter curve (WG-03), encounter MTB by biome, map sizes for sites, settlements and encounters, defender points, defender flee threshold, mass multiplier curve, settlement loot, pack-up pacing. |

## Scope notes

- Trade, visitors, gatherings, the ship and climate events stay on the first colony map in this delivery. They are single-colony services; making them per map is a later task.
- The meeting encounter (trade with a passing caravan) is deferred: caravan trading does not exist yet. Only ambush, manhunter ambush and demand ship.
- Splitting a caravan stays a disabled gizmo ("Not yet").
- Max colonies defaults to 1, so a new game behaves as today until the player raises it (1-5).
- Abandoning needs a confirm modal (our UX choice; vanilla asks nothing).
- New wanderers joining after a game over (vanilla 1.5, 8 in-game hours) are deferred: the modal offers Keep watching, Load and Main menu.
- Visitors already at home don't react when their faction turns hostile (`visitors.go` never reads the relation; a trade session does close). A gap for any faction turning hostile, not only through an attack; deferred.
- Attacking a neutral base asks for a confirmation first (our UX choice; the cost is large and can't be undone in one click).

## Phases

| Phase | Delivers | State |
|---|---|---|
| MAP-0 | This analysis; the inert contract (`handoffs/MAP-0-contract.md`) | done |
| MAP-1 | Site maps, all maps tick, pack-up, mineral site map, colonist bar and map switching, camera per map (`handoffs/MAP-1-site-maps.md`) | done |
| MAP-2 | Settlement attack, defenders, pod modes, storyteller target map (`handoffs/MAP-2-attack-and-pods.md`) | done |
| MAP-3 | Founding, abandoning, caravan speed, encounters (`handoffs/MAP-3-founding-and-encounters.md`) | done |
| MAP-4 | The three open questions: hostile turrets, everyone-gone ending, neutral attacks, and the goodwill rule (`handoffs/MAP-4-endings-turrets-neutral-attacks.md`) | done |

## Items

Columns: id, vanilla behaviour (wiki page), ours today (the state before MAP-0, evidence verified against the code on 2026-11-02), gap, fix (with what shipped), phase, status (current).

| ID | Vanilla (wiki page) | Ours today (evidence) | Gap | Fix | Phase | Status |
|---|---|---|---|---|---|---|
| MAP-01 | Several maps exist at once and all tick (Colony) | One map held flat on `Game` (`application/game/game.go:704-903`; `New` at `:906`) | No second map can exist | `mapState` bundle, one phase loop over all maps, tagged version counters, per-map ids and inbox | MAP-0, MAP-1 | done |
| MAP-02 | A caravan arriving at a site or tile can enter a map (Caravan) | Mining is abstract after 2 days (`application/game/travel/launch.go:174-187` `Mine`); other arrivals only wait (`application/game/world_state.go:474-494` `caravanArrived`) | Arrival never opens a map | Arrival routing: existing map, mineral site map, settlement map, friendly visit, empty tile (`EnterMap` opens a site map) | MAP-1 | done |
| MAP-03 | Temporary maps: 24 h to pack up once the job is done, then whatever is left is lost (Caravan) | Nothing | No temporary maps | 24 h countdown chip; then the colonists still there auto-form a caravan and the map closes. Close and defeat need 3 consecutive hourly census results; a started countdown never restarts | MAP-1 | done |
| MAP-04 | Mineral site: expires if not reached in 30 days; 10 days to mine, then hostiles (Long-range mineral scanner) | Sites exist with the 30-day expiry (`travel/scanner.go:28` `SiteDays`, `:232` `siteTick`) but mining is abstract | No map, no raid | Site map with the lump; expiry timer kept; a raid at day 10 of the visit | MAP-1 | done |
| MAP-05 | Attack a base: -200 goodwill and hostile; destroyed when the defenders flee (turrets ignored); +20 from that faction's enemies; loot is what is on the map; no bases left means the faction is gone (Faction base) | Refused: "attacking a settlement is not in this game" (`travel/launch.go:60`) | No settlement map | Settlement maps (section "Attack and founding"): generator, `raid.Defend` strategy, goodwill rules, letter "Settlement destroyed" Shipped end to end (`maps_settle_test.go`). Compound doors are held open so defenders can move, so colonists can walk through them (our simplification). Turrets: MAP-13; neutral bases: MAP-15. | MAP-2, MAP-4 | done |
| MAP-06 | Settle from a caravan; not next to a settlement; Max colonies default 1, up to 5 (Caravan, Colony) | No command | Cannot found a colony | `SettleHere` plus Options, Gameplay, "Max colonies" (1-5, default 1, passed in through `Setup`) `SettleHere`, `SettleReason` and the cap are done: `command.SetColonyCap` (Count) is sent by the App at start and load when the setting is above 1 and on each change, clamped to `maps.ColonyCapLimit` 5, never saved (exception noted in `ebiten/AGENTS.md`). The Settle gizmo is in the caravan inspector (`TestJourneyWorldSettleHere`). | MAP-3 | done |
| MAP-07 | Abandon a colony; carried items are lost; vanilla has no confirmation | Nothing | No abandon | `AbandonMap` with a confirm modal; abandoning map 0 while another colony exists promotes the next colony; none left shows the game-over flow `AbandonMap` works and abandoning map 0 promotes the next colony. Abandoning the last colony wipes colonists, prisoners and items and keeps the empty map; the ending is MAP-14. | MAP-3, MAP-4 | done |
| MAP-08 | Caravan incidents: ambush, manhunter ambush, demand (refuse and it is a fight), meeting or trade; visibility 1 pawn 20 %, 2 pawns 36 % (Caravan); MTB by biome is not published | None | No encounters | Hourly roll in `stepCaravans` on a named stream; ambush via `raidFrom` (`application/game/raids.go:108`) at the edges, manhunter ambush via `manhunterPack` (`application/game/handling.go:416`) with an edge-spawn argument, demand letter with `EncounterAnswer`; 100x100 encounter map; 2-day cooldown per caravan. Meeting deferred (no caravan trading) Ambush, manhunter ambush and demand are done; Give and Refuse sit in `hud/letters.go letterFooter` through `LetterView.Demand`. The meeting encounter stays deferred (no caravan trading). | MAP-3 | done |
| MAP-09 | Caravan speed: about 0.06 days per difficulty point; travel 06:00-22:00; mass x2 empty to about x0.5 full; roads halve difficulty; mounts 1.3-2.0 (Caravan) | Flat 3 tiles a day: `caravan.BaseTilesPerDay` (`domain/world/caravan/caravan.go:17`), `SpeedFor` (`:36`), `Advance` (`:53`) | Terrain, night, mass and mounts ignored | `caravan.Advance` over `planet.MoveDifficulty` (WG-03) with night rest, mass and mounts; `planet.Route` uses the same cost; `TravelEstimate` for the UI. Existing caravan tests re-baselined with the reason stated Empty caravan about 22 tiles a day; a caravan with nobody walking moves at full speed; hours use the colony clock. | MAP-3 | done |
| MAP-10 | Pods: 2.25 chemfuel per tile, max 66; onto a manned base always an attack (edge or centre); onto a loaded map pick the exact spot (Transport pod) | Fuel and range are done (`domain/world/caravan/pods.go:12-18`, `PodRange`, `PodFuel`); drop, gift and visit work (AX-9); landing elsewhere becomes a caravan (`travel/launch.go:139` `land`) | No attack, found or spot | New pod modes `PodArriveAttackEdge`, `PodArriveAttackCentre`, `PodArriveFound`, `PodArriveSpot`; `LaunchReason` loses the refusal; `land` asks the map registry; range ring and anchored landing menu in the world view. Done: Drop, DropCentre, Spot, Attack edge and centre, Found, gift, visit (Visit is refused without a settlement), all from the world view (`TestJourneyWorldPodModes`, `PodPickSpot`, `PodFound`). Caravans and pods follow the same rule since MAP-15. | MAP-2 | done |
| MAP-11 | Colonist bar groups pawns by map; the world button switches the view | One map; the World tab is modal (`ebiten/windows/world.go`, `world_ax9.go`; F8 `tab.world` in `platform/input/defs.go:147`); `hud.Host.CenterOnMap` takes bare coordinates (`ebiten/hud/deps.go:69`) | No map groups or switching | `SimMaps` port, `mapnav` leaf (keys `]` and `[`), colonist bar groups per map and caravan (`hud/colonistbar.go`), `Jump(Spot)`, per-map camera sidecar, scene per map in the renderer, full-screen `worldview` with inspector Colonist bar groups, `]` and `[`, `Jump(Spot)`, per-map camera and renderer scenes are done; the full-screen `worldview/` (F8, picking with cycle and double-click enter, markers, inspector with disabled reasons, route and ETA, search, legend, grid) replaced `windows/world*.go`; 14 journeys in `journey_world_test.go`; Split stays disabled ("Not yet"). | MAP-1, MAP-3 | done |
| MAP-12 | Raids can target any colony | One target (the single grid) | Threats always hit map 0 | The storyteller picks a target map from its own named stream, weighted by colony wealth; cross-map "Threat on X" alert Storyteller target on `streamStory`; no draw with one colony; sites and encounters are never picked. | MAP-2 | done |
| MAP-13 | Settlement turrets stay hostile, aren't needed to destroy the base and stay hostile afterwards (Faction base) | Inert, and `Claim` accepted them (`orders/claim.go`), so the player could own and wire them | Free turrets, and no danger from them | `Building.Hostile`: fires on colonists and tame animals, needs no power and joins no grid, can't be claimed or ordered; drafted colonists can shoot it; defeat still ignores it | MAP-4 | done |
| MAP-14 | When all colonists are dead, kidnapped or gone the colony is over and a popup says so; play can continue watching (Ending) | No flow at all, even when everyone dies; abandoning the last colony only posted a letter | The game never ends | `AX.EveryoneGone` (no colonist on any map, none on the road except prisoners, beasts and the kidnapped); modal with Keep watching, Load and Main menu. New wanderers: MAP-19 | MAP-4 | done |
| MAP-15 | Non-hostile settlements can be attacked; -200 goodwill and the faction turns hostile (Faction base) | The simulation and pods allowed it; only the caravan inspector refused, and its tip wrongly said the people turn hostile for good | One UI refusal, one wrong tip | Refusal removed, confirm modal on a non-hostile base, tip fixed, relation shown | MAP-4 | done |
| MAP-16 | Goodwill with a settlement's faction can't rise during an attack (Faction base) | The daily drift, radio and pod gifts, released prisoners and given animals all raised it | Goodwill rose during a siege | `Faction.Besieged`, kept in step by `Lifecycle.Flush`: `Adjust` and `DailyDrift` ignore rises, the radio refuses the gift with a reason, a pod gift lands for no goodwill (`TestGoodwillCannotRiseDuringAttack`) | MAP-4 | done |
| MAP-17 | "Doors owned by an enemy must be claimed (which requires defeating said enemy) in order for colonists to open them" (Door) | `Settle` held every settlement door open, so colonists walked in | No break-in | Doors lie shut with `Building.Hostile`. Defenders open them, colonists and animals are shut out (`colonistGrid`, region `Enemy` flag). A drafted Attack order breaks one; Claim works once no defender stands. Defenders go out when the compound is hit, held by `defendHold` (`settlement_door_test.go`) | MAP-5 | done |
| MAP-18 | Visitors whose faction turns hostile stop visiting and leave, defending themselves, as enemies (game behaviour; the Visitors page is missing) | They carried on visiting | Peace with an enemy | `visit/turn.go`: they become raiders already leaving, in a group of their own that costs no goodwill; trade closes; a letter says so; a guest in bed stays until recovered (`visitors_hostile_test.go`) | MAP-5 | done |
| MAP-19 | "The option to create new wanderers to start the colony again after eight in-game hours… After seven new groups have been created, the option will no longer be available" (Endings) | Deferred at MAP-14 | No way back after a game over | `command.NewWanderers`, a picker, walking in from an edge; `AX.Wanderers*` read model; a legacy colony with no setup gets no option (`wanderers_test.go`, `journey_wanderers_test.go`) | MAP-5 | done |
| MAP-20 | A mini-turret needs a hauler after 60 rounds (Mini-turret); nothing says an enemy reloads one | A spent settlement turret goes silent for good | None: checked, same result | Kept and pinned (`TestSpentSettlementTurretStaysSilent`); the inspector says "Out of ammo". The wiki is silent on enemy turrets, so this rests on game behaviour | MAP-5 | done |

## Counts

| | Count |
|---|---|
| Rows MAP-01 to MAP-20 | 20 |
| Missing | 0 |
| Partial | 0 |
| Done | 20 |

## Attack and founding

- **Settlement layout.** A new pure generator in `domain/world/mapgen/settlement*.go` (original): walls and doors in a compound sized by faction tech, beds, a bench and a storage area with seeded loot, turrets for industrial factions. Placed the way `placeRuin` is.
- **Defenders.** `raidFrom` takes a spawn-area argument; a new `raid.Defend` strategy (appended before `NumStrategies` in `domain/raid/raid.go`) holds inside the compound, engages within sight and flees at 50 % losses (original) using the existing flee code. Points scale with faction tech and difficulty (original).
- **Rules (wiki: Faction base).** Entering with attack intent costs 200 goodwill and the faction turns hostile; the settlement is defeated once no standing defender remains (fled, downed or dead; turrets ignored, and they stay hostile on the map afterwards); goodwill can't rise until then; factions hostile to it give +20 goodwill; the settlement leaves the faction's list; with none left the faction is defeated; the map becomes a site and the 24 h pack-up starts.
- **Founding.** `SettleHere` (or the Found pod mode) is refused at the colony cap or next to a settlement; otherwise it opens a colony map (or converts the temporary one) and runs `startHome` with wildlife on.

## Numbers

Source column: wiki (rimworldwiki.com page) or original tuning (the wiki publishes nothing).

| Number | Value | Source |
|---|---|---|
| Caravan pace | about 0.06 days per movement-difficulty point | wiki: Caravan |
| Travel hours | 06:00 to 22:00 | wiki: Caravan |
| Mass multiplier | x2 empty down to about x0.5 full; linear between | wiki: Caravan (ends); original tuning (linear curve) |
| Roads | x0.5 movement difficulty | wiki: Caravan |
| Mounts | best 1.3-2.0 when there is one per human | wiki: Caravan |
| Caravan visibility | 1 pawn 20 %, 2 pawns 36 %, then `1 - 0.8^n` | wiki: Caravan (first two); original tuning (fit beyond) |
| Encounter MTB by biome | table | original tuning (not published) |
| Encounter cooldown | 2 days per caravan | original tuning |
| Pod fuel | 2.25 chemfuel per tile per pod | wiki: Transport pod |
| Pod range | 66 tiles at most | wiki: Transport pod |
| Settlement attack | -200 goodwill, faction hostile | wiki: Faction base |
| Enemy-of-faction reward | +20 goodwill | wiki: Faction base |
| Goodwill during an attack | can't rise | wiki: Faction base |
| Defeat rule | defenders fled, downed or dead; turrets ignored | wiki: Faction base |
| Defender flee threshold | 50 % losses | original tuning |
| Defender points | by faction tech and difficulty | original tuning |
| Mineral site expiry | 30 days if not reached | wiki: Long-range mineral scanner |
| Mineral site raid | 10 days of mining, then hostiles | wiki: Long-range mineral scanner |
| Site distance | 6 to 25 tiles | wiki: Long-range mineral scanner |
| Temporary map pack-up | 24 hours | wiki: Caravan |
| Max colonies | default 1, up to 5 | wiki: Colony |
| Map sizes | colony uses setup size; site and settlement 150; encounter 100 | original tuning |
| Census hysteresis | 3 consecutive hourly results to close or declare defeat | original tuning |
| Storyteller target weight | by colony wealth | original tuning |
