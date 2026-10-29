# hud package

## Package Purpose
The colony HUD's regions: resource bar, colonist bar, right column (letters, messages, alerts), global controls (speed, date, overlays), cell info box, main tabs, status chips, climate, biome and gathering banners, toasts and the room stats overlay. `Chrome` owns their state and draws them from read models; the root composes them with the inspector and the windows. Feature package (ARCH-005 wave 5.5, agent A8).

## Owned Concepts
- `Chrome` (17 methods, holds the toasts) over `Resources` (13), `ColonistBar` (15), `Right` (32: letters, messages, alerts), `Alerts` (9: the alert rules), `Global` (21, status chips folded in), `CellInfo` (4), `Tabs` (3), `Banner` (5), `RoomStats` (2). Each is `struct{*Chrome}` or a field of it.
- `alertExplanations` is a literal table; the rules are data, not `init()` registration.
- `Shell` = `Sizer + World + View + Overlays + Speed + Settings + Host + Peers`.
- `Host` (8 methods: `Widgets`, `Commands`, `Env`, `Layout`, `ChromeHit`, `SpeedLabel`, `CenterOnMap`, `SetSettings`).
- `Peers` (8 methods: `PawnSel`, `Pick`, `Picked`, `TabOn`, `Power`, `StoneItem`, `DrawPawnBust`, `DrawWeather`).
- `Reader` (`Snapshot`, `TemperatureAt`, `Factions`, `Activity`) and the small source interfaces in `alerts_allies.go`, `alerts_trade.go`, `gathering.go`.

## Allowed Dependencies
- Leaves: `art`, `gfx`, `hudlayout`, `ids`, `ports`, `uikit`.
- Layers: `render/creature`, `render/env`, `render/light`, `render/overlay`, `render/sprite`, `render/world`.
- `selection`: a read model of the selection (`selection.Cand`, `selection.Kind`). This is the one feature-to-feature import, stated as fact.
- `application/game` read models; domain value types (`economy/item`, `pawn/schedule`, `scenario`, `sim/clock`, `sim/command`, `sim/entity`, `world/biome`).
- `platform/config`, `platform/input`; Ebitengine; stdlib.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages: `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`. (`selection` is the exception above.)
- Direct domain state: `hud` reads through `Snapshot()`-style read models and `Reader`.

## Public Contracts
- `Chrome` is the composition unit; the root calls its layout and draw methods each frame and routes clicks through `ids` prefixes.
- Clicks become `ports.Commands` or `ports.Intent` values; the HUD never writes the simulation.
- `Peers` returns read-only answers about the inspector and power overlays.

## Invariants
- Alerts are ordered by severity, then key (`TestAlertsAreOrderedBySeverityThenKey`); every alert carries an explanation (`TestEveryAlertCarriesAnExplanation`).
- A hostile alert names the chased colonist (`TestHostileAlertNamesTheChasedColonist`).
- No type over 40 methods; `Right` (32) is the largest.

## Hot Paths
- Widgets are declared once per `Update`; `Draw` paints the regions each frame.
- The colonist bar and resource bar redraw each frame. No per-frame allocation claim: the package has no benchmark.

## Determinism Rules
- Presentation only. Alert order and row content are functions of the read models.
- Never reaches the simulation's RNG or wall clock.

## Save/Mod Impact
- None. `SetSettings` sends a changed setting back through `Host`; the root writes it.

## Tests
- 19 source files, 5 test files, 35 `Test*` functions; `fakeShell` (14 methods) drives them.
- Coverage 36 % (ARCH-005 row); the rest runs through the root's journey tests.

## Benchmarks
- None.

## Known Design Constraints
- The colony composition stays root: `colony_compose.go` (`colonyUI`, `colonyButtons`, `drawColonyHUD`, `colonyChromeHit`, `hudRightClick`, the `hud()` layout memo). Renderer-side toggles live in `hud_global_render.go`.
- Converting shims (`alert`, `letterRow`, `messageRow`, `statusChip`, `toast`) remain in root tests that read lowercase fields.
