# Allow system gap analysis (STK-AL)

Scope: RimWorld's forbid/allow system for things and buildings, plus allowed areas (pawn area restriction), vanilla core only. The user asked on 2026-11-01: "do a gap analysis on the allow system and add it to this change". Load only for allow, forbid or allowed-area work. Handoff: `handoffs/STK-AL-allow-system.md`. Sister documents: `STORAGE_GAP_ANALYSIS.md` (dumping, hauling) and `ARCHITECT_GAP_ANALYSIS.md` (the Orders cards).

## 1. Decision record

| Item | Decision |
|---|---|
| Numbers | ENG-003 (programme brief STK) allows published wiki numbers for forbidding. The only number here is the allowed-area cap (wiki: Allowed area, eight, Home not counted). |
| Sources | Read on 2026-11-01: wiki Allowed area, Zone/Area, Home area, Orders, Door, Menus, Assign. The wiki's Forbidden and Forbid pages returned 404, so the auto-forbid and order-allows-target rows rest on documented game behaviour (the outside-home rule for what a death drops; orders and the Haul things designator allowing their target). Those rows say "game behaviour". |
| Clean room | No wiki text or game code is copied. Names and prose are ours. |
| Package | `internal/application/game` is at its SOLID budget. The rules live in the new stateless sub-package `game/allow` (`allow.Forbids` over a consumer-owned `allow.Host` of three roles); `game/forbid.go` holds the adapter, so no Game field or file is added and the moved code pays for the call sites. |
| Saves | No new saved field: `item.Stack.Forbidden` and `construction.Building.Forbidden` already exist. Saves with nine or ten areas keep them; no new one can be made above eight. |

## 2. Gap table

State: `done` (before this change), `closed` (this change), `open` (left, with the reason). Paths are under `internal/application/game` unless they start with `internal/` or `adapters/`, which means `internal/adapters/inbound/ebiten`.

### 2.1 Forbidding things

| # | Vanilla behaviour | Our evidence | Gap | State |
|---|---|---|---|---|
| F1 | The player forbids or allows a single item; forbidden items are skipped by every autonomous job (haul, eat, use, bills, refuel). | `allow/allow.go` `Forbids.Set`; `game.go` `Stack.Forbidden`; work-giver filters (`food.go` `bestFood`, `tick.go` haul searches, `craft/gather.go`, `burial.go`, `surgery.go`, `trough_feed.go`, `drugs.go`). | None. | done (SIM-028) |
| F2 | Architect Orders: Forbid and Allow drag over items. | `allow.Forbids.Rect` (was `forbid.go` `markForbidden`); `architect/catalog.go` cards `forbid`, `allow`. | None. | done |
| F3 | Toggle gizmo with hotkey F on the selection; mixed state on multi-select. | `inspector/cmds_thing.go` `item.forbid`; `inspector/multiselect.go` `group.forbid`/`group.allow`; `hotkeys.go` `forbidSelection`. | F did nothing on a selected door or bed; the building toggle showed no key and kept the label Forbid on a forbidden bed. | closed: `inspector/building_stats.go` `ForbidSelection`, `Hotkey: input.Forbid`, Allow label and icon when forbidden |
| F4 | Forbidden marker: a red X over a forbidden thing, buildings included. | Items: `render/world/draw_art.go` `objForbidden`, `fence.go` `DrawForbiddenMark`. | No marker on forbidden doors and beds; the building pane did not say forbidden. | closed: `render/world/objects.go` `DrawBuildingObj`, `inspector/cmds_build.go` `buildingLines` |
| F5 | Game behaviour: a pawn's body and what it drops when it dies (weapon, inventory) start forbidden when they land outside the home area, whatever its faction; inside Home they stay allowed. | Before: `forbid.go` `forbidRaidBody` and `raids.go` `dropRaiderGear` forbade raiders' bodies and weapons anywhere, nobody else's. | Faction rule instead of the home-area rule. | closed: `burial.go` `spawnCorpse`, `raids.go` `dropRaiderGear` (weapon, spare kept by combat, a thief's haul), `craft_host.go` `dropStackOf` keeps the flag; rule `allow.Forbids.DropForbidden` |
| F6 | Game behaviour: an animal's carcass outside Home starts forbidden, except a kill its hunter carries or a predator eats. | `animals.go` `killAnimal`, `animal_life.go` (old age), `wildlife.go` `dropCorpseAs`. | Carcasses were never forbidden. | closed: `allow.Forbids.CorpseForbidden`, `forbid.go` `killTaken` |
| F7 | Game behaviour: loot generated with the map (ruins, ancient vaults, caskets) starts forbidden. | `setup_mapgen.go`, `vault.go`, `casket.go` `spawnUnclaimed(..., false)`: found (PLAY-017 haul mark) but allowed. | Not forbidden. | closed: those three call sites pass `true`; stone chunks stay allowed (vanilla too) |
| F8 | Drop pods: event cargo pods forbid their contents; orbital-trade purchases arrive allowed. | `trade_orbital.go` drops purchases allowed. No cargo-pod or quest-reward incident exists (`internal/domain/incident`). | No content for cargo pods or quest rewards. | done for trade; no content otherwise |
| F9 | A dead trader's goods are dropped forbidden. | `trade_caravan.go` `traderLost` drops them forbidden. | None (kept forbidden anywhere; a trader dies near the colony's edge). | done |
| F10 | The Haul things designator allows a forbidden item and marks it for hauling. | Before: `haul_claim.go` `markHaul` marked found stacks only and skipped forbidden ones. | Forbidden stacks stayed forbidden. | closed: `allow.Forbids.HaulRect`, `Forbids.Mark` (the item's Mark for hauling allows too; its disabled "allow it first" state is gone, `inspector/cmds_thing.go`) |
| F11 | Game behaviour: an order given to a colonist on a forbidden item (equip, wear, pick up/consume, haul) allows it. | `orders.go` `forcedJob` accepted forbidden targets but left them forbidden. | Flag not cleared. | closed: `orders.go` `prioritize` calls `allow.Forbids.Ordered` for Eat, Haul, Equip (Equip covers apparel) |
| F12 | A colonist in a mental break ignores forbidding (eats forbidden food on a binge). | `food.go` `bestFood` skipped forbidden food for everyone. | Binge ignored nothing. | closed: `st.Forbidden && !g.breaking(id)` |
| F13 | Non-colony pawns ignore forbidding: thieves steal forbidden items, wild animals eat forbidden food. | `raids.go` steal search has no forbidden filter; wild animals hunt live prey only (`animal_needs.go` `findTameFeed` is the tame search and honours forbidding). | None. | done |
| F14 | Items outside a colonist's allowed area count as forbidden to it. | `areas.go` `areaAllows`, `inArea` used by the givers. | None. | done (PLAY-015) |
| F15 | A forbidden stack mid-haul is not interrupted. | Haul steps check the flag only when gathering more (`haul/step.go`). | None. | done |

### 2.2 Forbidding buildings

| # | Vanilla behaviour | Our evidence | Gap | State |
|---|---|---|---|---|
| B1 | Doors and beds (sleeping spots, animal beds) can be forbidden; benches cannot (bills are suspended instead). | `inspector/building_stats.go` `forbiddable`: sleeping places and doors only. | None. | done (BED-1, CBT-5) |
| B2 | A forbidden bed is nobody's: no claiming, no sleeping. | `rest/beds.go`, `rest/animals.go`, `rest/seats.go`. | None. | done |
| B3 | A forbidden door: colonists and colony animals do not pass, even when it is open (wiki: Door). | `combat/door.go` `Barred`, `bridge.go` `cellWalkable`. | None for colonists. | done (DEF-3) |
| B4 | Visitors, traders, enemies and colonists in a mental break ignore a forbidden door (wiki: Door). | `cellWalkable` makes the door cell unwalkable on the one shared path grid; hostiles use their own shut-door set (`combat/door.go` `HeldOpen`). | Visitors and breaking colonists path around a forbidden door. | open: needs a per-faction door passability layer in pathing and regions (a DEF follow-up, not an allow rule) |
| B5 | Forbidden doors not owned by the player can be forbidden (1.3). | Any door building takes the toggle. | None. | done |

### 2.3 Allowed areas

| # | Vanilla behaviour | Our evidence | Gap | State |
|---|---|---|---|---|
| A1 | Manage allowed areas: create, rename, delete, invert (wiki: Allowed area, Zone/Area). | `areas.go` `cmdCreateArea`, `cmdRenameArea`, `cmdDeleteArea`, `cmdInvertArea`; `architect/area_manager.go`. | None. | done (PLAY-015) |
| A2 | At most eight allowed areas; Home is not one of them (wiki: Allowed area, Menus). | `internal/domain/world/area/area.go` `MaxAreas` was 10. | Cap ten. | closed: `MaxAreas = 8`; `architect/area_manager.go` `AreasMax = area.MaxAreas` |
| A3 | Copy an area. | Not on the wiki's list (create, delete, rename, invert). | Not vanilla per the wiki. | not a gap |
| A4 | Expand and clear tools, one per area; areas show only while painting; areas overlap and may be disjoint. | Architect Zone submenus (`architect/area_menu.go`, `area_select.go`); overlay while armed. | None. | done |
| A5 | Each colonist has one area or Unrestricted (default); set in the Schedule tab's area column. | `windows/schedule.go` `SchedAreaW` column; `areas.go` `cmdAssignArea`, `AreaOf`. | None (the manager is also reachable from Assign). | done |
| A6 | Home area is a selectable restriction. | `command.AreaHome`, `areas.go` `refContains`. | None. | done |
| A7 | Tame animals (not penned livestock) take an area in the Animals tab, with a Manage areas button; a drafted master frees them. | `windows/animals.go` `colArea`; `areas.go` `restrictedRef`, `areaAnimal`. | None. | done |
| A8 | Drafted pawns, colonists in a mental break, fleeing and attacking pawns ignore the area. | `areas.go` `restrictedRef` (drafted, breaking, downed); `area_dest.go` (orders, rescue, fights, fleeing). | None. | done |
| A9 | Pawns path freely but take the shortest path within the area, and do no work or joy outside. | `area_soft.go` soft path cost; `area_dest.go` destination rule. | None. | done |
| A10 | Hunters do not hunt outside their area; firefighters need Home and the area. | `area_dest.go` `huntGivenUp`; `firefight.go`. | None. | done |
| A11 | A pawn outside its area walks back in. | `areas.go` `areasTick`, `areaAnimalsTick`. | None. | done |

## 3. Constants

| Constant | Value | Source |
|---|---|---|
| `area.MaxAreas` | 8 | wiki: Allowed area |

## 4. Left open

- B4: per-faction door passability (visitors, traders and breaking colonists through a forbidden door). Pathing has one colonist grid; this needs a region layer, which belongs to a defence or pathing task.
- Downed pawns dropping their weapon (vanilla): a combat rule, not an allow rule; when it lands, its drop uses `allow.Forbids.DropForbidden`.
- Building multi-select (several doors at once): the selection model groups pawns, animals and items only (`selection/selection.go`); F works on one selected door or bed.
