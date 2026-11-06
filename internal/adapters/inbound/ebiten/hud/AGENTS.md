# internal/adapters/inbound/ebiten/hud

## Purpose
The colony HUD's regions: resource bar, colonist bar, right column (letters, messages, alerts), global controls (speed, date, overlays), cell info box, main tabs, status chips, climate, biome and gathering banners, toasts and the room stats overlay. `Chrome` owns their state and draws them from read models; the root composes them with the inspector and the windows. Feature package (ARCH-005 wave 5.5, agent A8).

## Owns
- `Chrome` (17 methods, holds the toasts) over `Resources` (13), `ColonistBar` (15), `Right` (32: letters, messages, alerts), `Alerts` (9: the alert rules), `Global` (21, status chips folded in), `CellInfo` (4), `Tabs` (3), `Banner` (5), `RoomStats` (2). Each is `struct{*Chrome}` or a field of it.
- The endings (`ending.go`): the ship's modal and the everyone-gone modal `ending.gone` (MAP-14, Keep watching, Load game, Main menu), which opens when `AX.EveryoneGone` turns true and re-arms only after it goes false.
- `alertExplanations` is a literal table; the rules are data, not `init()` registration.
- `Shell` = `Sizer + World + View + Overlays + Speed + Settings + Host + Peers`.
- `Host` (8 methods: `Widgets`, `Commands`, `Env`, `Layout`, `ChromeHit`, `SpeedLabel`, `Jump`, `SetSettings`).
- `Peers` (8 methods: `PawnSel`, `Pick`, `Picked`, `TabOn`, `Power`, `StoneItem`, `DrawPawnBust`, `DrawWeather`).
- `Reader` (`Snapshot`, `TemperatureAt`, `Factions`, `Activity`) and the small source interfaces in `alerts_allies.go`, `alerts_trade.go`, `gathering.go`.

## Depends on
- Leaves: `art`, `gfx`, `hudlayout`, `ids`, `ports`, `uikit`.
- Layers: `render/creature`, `render/env`, `render/light`, `render/overlay`, `render/sprite`, `render/world`.
- `selection`: a read model of the selection (`selection.Cand`, `selection.Kind`). This is the one feature-to-feature import, stated as fact.
- `application/game` read models; domain value types (`economy/item`, `pawn/schedule`, `scenario`, `sim/clock`, `sim/command`, `sim/entity`, `world/biome`).
- `platform/config`, `platform/input`.

## Must not depend on
- Other feature packages: `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`. (`selection` is the exception above.)
- Direct domain state: `hud` reads through `Snapshot()`-style read models and `Reader`.

## Contracts
- `Chrome` is the composition unit; the root calls its layout and draw methods each frame and routes clicks through `ids` prefixes.
- Clicks become `ports.Commands` or `ports.Intent` values; the HUD never writes the simulation. `SetSettings` sends a changed setting back through `Host`; the root writes it.
- `Peers` returns read-only answers about the inspector and power overlays.

## Invariants
- Alerts are ordered by severity, then key (`TestAlertsAreOrderedBySeverityThenKey`); every alert carries an explanation (`TestEveryAlertCarriesAnExplanation`). Alert order and row content are functions of the read models.
- A hostile alert names the chased colonist (`TestHostileAlertNamesTheChasedColonist`).
- No type over 40 methods; `Right` (32) is the largest.

## Hot paths
- Widgets are declared once per `Update`; `Draw` paints the regions each frame. The colonist bar and resource bar redraw each frame. Not benchmarked.

## Tests
- 19 source files, 5 test files, 35 `Test*` functions; `fakeShell` (14 methods) drives them.
- Coverage 36 % (ARCH-005 row); the rest runs through the root's journey tests.

## Notes
- The colony composition stays root: `colony_compose.go` (`colonyUI`, `colonyButtons`, `drawColonyHUD`, `colonyChromeHit`, `hudRightClick`, the `hud()` layout memo). Renderer-side toggles live in `hud_global_render.go`.
- Converting shims (`alert`, `letterRow`, `messageRow`, `statusChip`, `toast`) remain in root tests that read lowercase fields.
- New wanderers (MAP-19): `ending.go`: "Nobody left" (`GoneAlertKey`, counts down to new wanderers; a click reopens the everyone-gone screen via `Alerts.Focus`). The gone screen's New wanderers button (`GoneWanderersID`) is off with "In 6h" or "No more groups" and sends `ports.OpenWanderers`.
- Fish tank (FISH): `alerts_tank.go`: "Fish dying" (`FishDyingAlertKey`, a stocked tank with `Tank.Trouble`).
- Mouse on the bar and the messages (CTL-010..012): `bar_drag.go`: `ColonistBar.HandleRight` owns the right button from a press on a portrait until the release. It sends `SetColonistOrder` with the target's `BarOrder` (one rank across all maps, not the place in the grouped list), `ports.OpenWorld` on a click, a hint across maps. The "+N" slot takes no part. `rightState.dismissed` hides a message while the game still lists it; presentation only, never saved.
