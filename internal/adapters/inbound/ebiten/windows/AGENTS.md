# internal/adapters/inbound/ebiten/windows

## Purpose
The colony's main-tab windows: Work, Schedule, Assign, Animals (with the wildlife list), Research, History and Factions, and the window chrome they share. `Set` owns which window is open and its mode, declares the widgets, draws and handles clicks; it reaches the simulation and the rest of the client through `Shell`. Feature package (ARCH-005 wave 5.3, agent A5).

## Owns
- `Set` (30 methods, 24 exported; under the 40-method limit): the kit window, `Want`, `Rect`, `Anchor`, `Buttons`, `Click`, `Show`, `Declare`, `Draw`, and the wild-row focus cycle (`focus`).
- Seven window types, each `struct{*Set}`: `Work`, `Schedule`, `Assign`, `Animals` (14 methods), `Research` (16: the RSCH-2 tech tree; its state is `researchState` in `research_tree.go`, the layout `layoutTree` in `research_layout.go`, the info panel in `research_panel.go`), `History`, `Factions`. The launch method is `Launch`, not `Open`, so it does not hide the field.
- Interfaces: `Reader` (`Wildlife`, `History`, `IncapableReason`), `Labels` (7: `PawnLabel`, `AnimalLabel`, `AnimalHealth`, `CareShort`, `Factions`, `DrawAnimalIcon`, `DrawPassion`), `Host` (6: `Env`, `WindowRect`, `CenterOnMap`, `AreaDropdown`, `ManageAreas`, `Float`).
- `Shell` = `Chrome (Sizer + Dispatcher + Texts) + Commands + World + PawnSel + Widgets + Labels + Host`. Every label goes through `Shell.T`/`lang.Tf` (OPT-001); the exported text helpers (`AnimalDoing`, `FollowCell`, `DrawGoodwillChart`, ...) keep their English signatures and wrap an unexported `tr lang.Tr` variant; the Work header texts live in `Work.help`/`colName` (`WorkHelp`/`WorkColumns` stay the English reference).
- Mouse painting (CTL-013..015, 019), `paint.go`: one `painter` on `Set` (one window is open at a time): a first click arms it with its value, and `Drag` spreads that value over the cells the held mouse enters, once each. Schedule, Work and Assign use it; `Assign.Drag` covers Drugs, Medical and Food. Work: `RightClick` lowers a cell or runs a header with the right button, `HeadClick` needs Shift, incapable colonists are skipped silently. `Animals.WildArrow` is the arrow to a hovered wild row; it needs the shell's `WorldToScreenF` through the consumer-owned `mapProjector`.

## Depends on
- Leaves: `art`, `gfx`, `ids`, `inputframe`, `ports`, `uikit`.
- `application/game` read models; domain `ai/work`, `building/construction`, `faction`, `pawn/drug`, `pawn/medcare`, `pawn/schedule`, `pawn/skill`, `sim/clock`, `sim/command`, `sim/entity`.

## Must not depend on
- Feature packages (`hud`, `architect`, `inspector`, `orders`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`, `selection`).
- `render/*`: windows draw UI, not the map.

## Contracts
- `Shell` is composed in the root's `windows_compat.go`; `Set` never reaches `App`.
- Window actions leave as `command.Command` through `ports.Commands`, or as `ports.Intent` (`ShowWindow`, `CloseWindows`) for navigation.
- The window mode and preset are per-session UI state.

## Invariants
- Research tab (RSCH-2): the tree's layout and texts are rebuilt only when `World().Research` hands out a different slice or the projects' set changes (`TestResearchTreeCachesAndDoesNotAllocatePerFrame`); a box click selects (`research.pick.<id>`), only Start research (`research.start`) sends SetResearch; everything draws clipped to the window (`TestResearchTreeDrawsInsideAtEveryOffset`, root).
- Every window declares its widgets once per `Update` (`TestKitWindowOpensTogglesAndCloses`).
- Animal columns drop to fit the width (`TestAnimalColumnsDropToFit`); presets cycle (`TestNextPresetCycles`).

## Hot paths
- `Draw` paints the open window per frame from read models. Widget declaration runs once per `Update`. No allocation claim.

## Tests
- 12 source files, 2 test files, 6 `Test*` functions; `fakeShell` (11 methods) and `fakeHost` (8 methods).
- Coverage 18.1 % (ARCH-005 row); the windows run mostly through the root journey tests.

## Notes
- `App.ng` stays on App because `Env.Planet` reads it.
- The wizard-to-factions glue stays in the root.
- Lowest coverage among the feature packages: add tests with any change to a window's click handling.
