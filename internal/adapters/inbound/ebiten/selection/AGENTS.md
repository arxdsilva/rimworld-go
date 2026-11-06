# internal/adapters/inbound/ebiten/selection

## Purpose
What the player has selected: one pawn, animal, item, plant, building, frame or zone, or a group of several; the click and box-select rules that set it; and the group's bookkeeping. It is the single owner of selection state, reached by the inspector, orders and HUD through `Model` (ARCH-005 wave 5.1, agent A1).

## Owns
- `Model` (34 methods) built by `New(sh Shell, host Host) *Model`: `Select`, `Clear`, `ClickSelect`, `ShiftClick`, `ToggleInSet`, `SetGroup`, `SetMembers`, `CanGroup`, `ApplyBox`, `Selected*`, `Selectables`, `HandleMapMouse`, and the cycle and press handling.
- `Kind` (uint8: the selectable kinds), `Cand` (one selectable candidate; 109 literals in the root are keyed), `Group`, `Cycle`, `Press`.
- `SelectablesAt(w, pawns, tx, ty)` and `SelectablesSplit` (the candidate list for a tile, with the solid count).
- Who groups (`group.go`, CTL-001..007): `Multi` (a building's rule), `Alike` (the double click on a building, blueprint, zone or a zone's plant), `Boxed` (a box after colonists, animals and items), both over `InView`, the renderer's test passed in. `PickID` is a group row's button id.
- Interfaces: `Shell` = `World + PawnSel + View + Sizer + Commands`; `Host` (8 methods: `AnimalsCovering`, `PawnUnderCursor`, `SelectZoneLabel`, `NoteCell`, `SetZone`, `PowerShown`, `HandleOrders`, `Frame`); `Mouse` (4: `Clicked`, `Take`, `Down`, `Shift`).

## Depends on
- `ports` (the roles `Shell` embeds).
- `application/game` read models; domain `building/construction`, `sim/clock`, `sim/command`, `sim/entity`.
- `pkg/mathx` (`Abs`).

## Must not depend on
- Feature packages (`inspector`, `orders`, `hud` read `selection`; it imports none of them).
- Ebitengine and every `render/*` package: selection is pure state and hit rules.

## Contracts
- Every selection change is a method call on `Model`; the readers call `Selected*` and never mutate the fields. Selection never reaches the simulation; orders read it through the inspector and orders packages.
- Zone and power state changes go out through `Host.SetZone` and `PowerShown`, not through the simulation.

## Invariants
- Clicking cycles through candidates and clears the others (`TestClickCyclesAndClearsOthers`); group set and toggle are consistent (`TestGroupSetToggleAndClicks`).
- A group's members may be of different kinds (`Mixed`): compare `Cand`s, never an id alone, because a zone's id can equal an entity's. A box takes one kind; Shift mixes.
- Walls, hoppers, floors, carpet, fences, conduits, floor blueprints, zones a building laid and plants never join a group; a plain click selects them.

## Hot paths
- `HandleMapMouse` and `ApplyBox` run per mouse event; `Selectables`, `Alike` and `Boxed` per click or box. The root's group marks call `Selected*` each frame while a group is up: `members` reads nothing for a kind the group lacks and keys its map on the ids, never the views.

## Tests
- 4 source files, 2 test files. Coverage 76 %. The root's `group_kinds_test.go` drives the gestures through the real mouse.
- The interaction is covered mostly through the root's journey tests.

## Notes
- Wired as a sole agent: `App.sel` and 38 root files repointed to `a.sel.X`.
- The group pane, its commands, the box-select and group-mark draws and the renderer hit tests stay in the root.
- Read by 52 root files; a field rename is a serial, root-wide repoint.
