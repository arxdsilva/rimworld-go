# CTL-001..020: mouse controls

User request (2026-11-04): double click on furniture or an item takes the same type on screen, and any action applies to them all; and everything else missing from the mouse. Rows, sources and decisions: `.cursor/MOUSE_CONTROLS_GAP_ANALYSIS.md`.

## What changed

- **Selection:** `selection.Group` is `{Members []Cand}`, its `Kind()` is the common one or `Mixed`. `Multi` says which buildings group. `Alike` and `Boxed` find the members. `PickID` names the pane's rows. A double click with nothing alike stays a click, so quick clicks still cycle a tile's stack.
- **Pane and actions:** titles with the count, "Various (n)". Commands merge by key; menus merge to the options all share; `Button.Single` and `groupSingle` drop Move, Copy, Settings and the like. F flips stacks and doors together.
- **Windows:** `windows/paint.go` is the shared brush. Work: right click, Shift on a header, drag. Assign: drag on Drugs, Medical and Food. Wild row: an arrow. The food policy editor has no per-colonist column, so nothing paints there.
- **Counters:** `uikit.Stepper` multiplies by `uikit.QtyStep`; trade's 1 and 10 buttons and the comms gift too.
- **HUD:** right click dismisses a message; right drag on the bar reorders it, a right click opens the world.
- **Orders:** Forbid, Allow, Hunt and Tame have a whole-map option (`architect.Card.Options`).
- **Camera:** a middle tap rotates the held tool or the moved building; a drag still pans.
- **Escape:** clears a lone animal, item or plant.
- **Sim:** `command.SetColonistOrder`, `game/roster/bar.go`, `saveState.ColonistOrder`. `g.pawnIDs` never moves, so AI and work cannot see the order. Nil until used: no `SaveVersion` bump, and every pinned hash holds. Used, it enters `StateHash`.

## Found during integration

- The bar sent its place in the list grouped by map while the sim ranks across all maps, so a drag in a later group could do nothing. It now sends the target's `BarOrder` (`TestRightDragInALaterGroupSendsTheRank`).
- With a group selected and the Architect open, tool cards took no clicks: an old bug that box-selecting furniture made easier to hit.
- The journeys caught two more: Shift on bare ground cleared the selection, so a Shift+box replaced it instead of adding; and the Architect's order cards lost their options on the way to the grid (`archToolCommands`).
- `TestJourneyAX8EscapeShip` clicked a tile twice at once, which now groups the two caskets. It waits out the window; its assertions did not change.
- `TestGroupInspectorAtScale1_5` ignores options arrows when checking overlap, because a merged menu draws one inside its own cell.

## Tests

`journey_multiselect_test.go` and `journey_mouse_test.go`: 30 journeys through the real mouse, one per gesture. The slowest is the caravan counter, about 2.7 s under `-race`, since it needs a wizard colony with a world.

## Budgets

`App` fell to 243 methods: `toggleWorkCell` went to `windows.Work.Click`. `game` stays under its line budget: `passionPct` and `totalXP` moved to `game/roster`.

## Not done

CTL-101..103 stay open (`MOUSE_CONTROLS_GAP_ANALYSIS.md`). A colonist away during the first ever move holds no slot, so comes back at the end of the bar. Sources are the wiki, tips and reports: nothing here was checked against the game itself.
