# selection package

## Package Purpose
What the player has selected: one pawn, animal, item, plant, building, frame or zone, or a group of several; the click and box-select rules that set it; and the group's bookkeeping. It is the single owner of selection state, reached by the inspector, orders and HUD through `Model` (ARCH-005 wave 5.1, agent A1).

## Owned Concepts
- `Model` (27 methods, all exported) built by `New(sh Shell, host Host) *Model`: `Select`, `Clear`, `ClickSelect`, `ShiftClick`, `ToggleInSet`, `SetGroup`, `ApplyBox`, `Selected*`, `Selectables`, `HandleMapMouse`, and the cycle and press handling.
- `Kind` (uint8: the selectable kinds), `Cand` (one selectable candidate; 109 literals in the root are keyed), `Group`, `Cycle`, `Press`.
- `SelectablesAt(w, pawns, tx, ty)` and `SelectablesSplit` (the candidate list for a tile, with the solid count).
- Interfaces: `Shell` = `World + PawnSel + View + Sizer + Commands`; `Host` (8 methods: `AnimalsCovering`, `PawnUnderCursor`, `SelectZoneLabel`, `NoteCell`, `SetZone`, `PowerShown`, `HandleOrders`, `Frame`); `Mouse` (4: `Clicked`, `Take`, `Down`, `Shift`).

## Allowed Dependencies
- `ports` (the roles `Shell` embeds).
- `application/game` read models; domain `sim/clock`, `sim/command`, `sim/entity`.
- Standard library: `slices`, `strconv`, `strings`, `testing` (tests).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages (`inspector`, `orders`, `hud` read `selection`; it imports none of them).
- Ebitengine and every `render/*` package: selection is pure state and hit rules.

## Public Contracts
- Every selection change is a method call on `Model`; the readers call `Selected*` and never mutate the fields.
- Zone and power state changes go out through `Host.SetZone` and `PowerShown`, not through the simulation.

## Invariants
- Clicking cycles through candidates and clears the others (`TestClickCyclesAndClearsOthers`); group set and toggle are consistent (`TestGroupSetToggleAndClicks`).

## Hot Paths
- `HandleMapMouse` and `ApplyBox` run per mouse event; `Selectables` runs per click. No per-frame work beyond the `Update` input poll.

## Determinism Rules
- Presentation state only. Selection never reaches the simulation; orders read it through the inspector and orders packages.

## Save/Mod Impact
- None.

## Tests
- 3 source files, 1 test file, 2 `Test*` functions. Coverage 47.8 % (ARCH-005 row).
- The interaction is covered mostly through the root's journey tests.

## Benchmarks
- None.

## Known Design Constraints
- Wired as a sole agent: `App.sel` and 38 root files repointed to `a.sel.X`.
- The group pane, its commands, the box-select and group-mark draws and the renderer hit tests stay in the root.
- Read by 52 root files; a field rename is a serial, root-wide repoint.
