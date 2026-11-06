# MAP-0: multiple maps, gap analysis and inert contract

User request (2026-11-02): all four map features (site maps, attacking settlements, founding a second colony, encounter maps), with every open map ticking every tick. Plan Part 2; analysis in `.cursor/MULTIMAP_GAP_ANALYSIS.md`.

## What changed
- `mapState` bundle embedded in `Game` (field promotion keeps `g.grid`); `Game` fields 144 to 64, methods 1889 to 1837, lines 53537 to 53328.
- `Command.Map` and `MapSet`; kinds appended after `LeaveShip` (`EnterMap`, `AttackSettlement`, `SettleHere`, `AbandonMap`, `CaravanTravel`, `CaravanReform`, `EncounterAnswer`, `SetColonyCap`) and pod modes (`PodArriveAttackEdge/Centre`, `PodArriveFound`, `PodArriveSpot`).
- One Step loop over all maps with `primary` and `owns` gating of global tables.
- `game/maps` skeleton (Atlas, hosts), view types (`MapView`, `Maps`, `ViewMap`, `Colonists`), ports, `Gameplay.MaxColonies`.
- Saves: `ExtraMaps`, `SaveVersion` 12 (a v12 save is refused by older binaries). A single-map game hashes as before (`TestSaveSingleMapHasNoExtraMaps`); `TestSaveTwoMapsRoundTrip`; mirror tests prove a quiet extra map leaves map 0's hash unchanged (`maps_mirror_test.go`).
- ENG-003: approved in the plan; the AGENTS.md line is pending the user.

## Scope notes
Trade, visitors, gatherings, the ship and climate events stay on the first colony map. The meeting encounter is deferred (no caravan trading).
