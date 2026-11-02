# worldview package

## Package Purpose
The colony's full-screen world view (MAP-2, MAIN_UI_SCREENS §33, §35, §36): the planet on the globe with every world object marked, picking, the world inspector with its gizmos, the caravan route preview, the transport pods' range ring and landing menu, and the pick-spot cursor. It replaced the modal World tab (`windows/world*.go`). The simulation ticks under it and the HUD stays up; the root skips the map draw, the map keys and the map mouse while it is open.

## Owned Concepts
- `View`: open state, chart (the planet, a slice per Update when it must be rebuilt), the globe, the objects (`Object`, `Kind`) and their markers (`mark`), the selection (`selState`, keys as in `ports.OpenWorld`), the modes (`routeState`, `podState`, `spotState`), the toolbar's toggles, the inspector's caches (`card`, `etaCache`).
- `Reader` (`WorldMap`, `Maps`, `ViewedMap`, `Factions`) with optional type assertions `WorldSource`, `Estimator`, `Launcher`, `Attacker`.
- `Host` (`Env`, `Layout`, `ChromeHit`, `FormCaravan`, `TileAt`, `TilePx`, `WorldToScreen`, `SettleReason`); `Shell` = `Sizer + Commands + World + Settings + Panels + Widgets + Dispatcher + Host`.

## Allowed Dependencies
- Leaves: `art`, `gfx`, `globe`, `globemath`, `hudlayout`, `inputframe`, `ports`, `uikit`, `units`, `worldinfo`.
- `application/game` read models and `game/axview`; domain `sim/clock`, `sim/command`, `sim/entity`, `world/caravan`, `world/planet`; `platform/config`, `platform/input`.
- Ebitengine; stdlib.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages (`hud`, `windows`, `dialogs`, `inspector`, `orders`, `wizard`, ...): `FormCaravan` and the intents reach them through the shell.
- `render/*`.

## Public Contracts
- Root calls: `Show`, `Toggle`, `Close`, `Open`, `Escape` (pick spot, pod menu, targeting or route, selection, then the view), `LaunchFor`, `Declare` (once per Update, in `colonyUI`, before the HUD's widgets), `DrawGlobe` (in the map's place), `HandleSpot`, `Spotting`, `DrawSpot`.
- Widget ids: `world.obj.<map|caravan|settlement|site|pod>.<id>`, `world.act.<jump|form|abandon|reform|travel|return|settle|enter|attack|split|confirm|cancel>`, `world.pod.<drop|dropcentre|attackedge|attackcentre|found|gift|visit|spot|cancel>`, `world.pod.target.<tile>`, `world.search`, `world.search.field`, `world.legend`, `world.grid`, `world.close`, modal `world.abandon`.
- Everything leaves as `command.Command` (`ports.Commands`) or `ports.Intent`; a launch names the pods' own map (`MapSet`), so a switch to the target map does not move the command.

## Invariants
- Objects are read when the clock has moved `refreshTicks` or the view sent a command, never per frame (`TestObjectsAreReadOnlyWhenNeeded`); estimates are cached (`TestEstimateIsCached`); the range ring is one breadth-first search per targeting start.
- Every gizmo carries why it is disabled; the simulation's own answer wins where the game offers one (`LaunchReason`, `AttackReason`, `TravelEstimate`).
- Draws go through the canvas (Rule 3 of `ui_canvas_guard_test.go`); only `spot_draw.go:DrawSpot` draws raw, on the map.
- Another tab or the Architect opening closes the view; a new game resets it (`follow`).

## Determinism Rules
- Presentation only: it reads views and sends typed commands.

## Save/Mod Impact
- None.

## Tests
- `view_test.go`, headless over a fake shell and a small real planet.

## Known Design Constraints
- Settlement emblems are the globe's own marks; the view adds only their click widgets and selection ring.
- Objects on one tile share a spot: a click takes the first in pick order, a slower click cycles, a double click on our map enters it.
