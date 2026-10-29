# orders package

## Package Purpose
The right-click order flow: the float menu over a tile, the order queue route, the drafted colonists' formation drag and the Z search box. `Controller` owns their state and turns a choice into a typed command through `Shell`. Feature package (ARCH-005 wave 5.3, agent A9).

## Owned Concepts
- `Controller` (27 methods, 19 exported) built by `New(sh Shell) *Controller`, with exported state:
  - `Float` (the float menu over a tile),
  - `Search` (the Z search box),
  - `Form` (the formation drag; its type is `Formation`).
- `Hit` (the menu hit result), `Move` (one pawn's destination).
- Free functions: `DrawRoute`, `FireOrders`, `FormationCells`, `FormationMoves`, `LineCells`, `AssignCells`.
- Interfaces: `Host` (8 methods: `Frame`, `Selection`, `Hostiles`, `ChromeHit`, `Layout`, `Icon`, `OrdersAt`, `CenterOnMap`); `Labels` (`BuildingTitle`).
- `Shell` = `Hinter + Sizer + Commands + World + View + PawnSel + Labels + Host`.

## Allowed Dependencies
- Leaves: `gfx`, `hudlayout`, `ids`, `inputframe`, `ports`, `uikit`.
- `selection`: the selection model (stated as fact: orders imports `selection`).
- `application/game` read models; domain `sim/clock`, `sim/command`, `sim/entity`.
- Ebitengine (`ebiten`, `vector`); stdlib (`sort`, `strconv`, `strings`, `unicode`, `unicode/utf8`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages: `hud`, `architect`, `inspector`, `windows`, `dialogs`, `learn`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`.
- Domain state: orders reads through `Host.OrdersAt` and `Hostiles`.

## Public Contracts
- Every order is a `command.Command` through `ports.Commands`; `FireOrders` builds the list.
- `FormationMoves` takes a `free` predicate and returns moves; it does not read the world itself.

## Invariants
- Formation cells are deterministic for the same input (`TestFormationCellsDeterministic`).
- A single colonist's formation move lands on the exact cell (`TestFormationMovesSingleColonistGoesExactly`).
- Walls and edges block the walkable function (`TestWalkableFnBlocksWallsAndEdges`).
- The pick menu calls back once and closes (`TestPickMenuCallsBackAndCloses`).

## Hot Paths
- `DrawRoute` runs per frame over the queued orders. No benchmark in the package.

## Determinism Rules
- Presentation only; formation and line cell maths are pure functions of their inputs, so the same drag yields the same commands.
- Never reaches the simulation's RNG or wall clock.

## Save/Mod Impact
- None.

## Tests
- 7 source files, 5 test files, 16 `Test*` functions.
- Coverage 76.7 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- `Renderer.drawOrderRoute` stays a 30-line root wrapper over `orders.DrawRoute`.
- The root's `float`, `search` and `form` fields moved here; the root repoints them as `a.ord.Float` and similar.
