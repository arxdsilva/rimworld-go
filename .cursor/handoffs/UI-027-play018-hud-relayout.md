# Task Handoff

Task ID: UI-027 (PLAY-018; plan `.cursor/PLAY018_PLAN.md`)
Status: complete

## Summary
**User decisions (2026-10-25):**
- The colony screen takes the familiar colony-sim arrangement on our own grid, with no other game's numbers or colours.
- A new original charcoal palette on every screen.
- A learning helper.
- Art agents always run on the best model (PARALLEL_WORK §6).

**Layout** (`hud_layout.go`): `computeHUDLayout` is a pure function that places every region in logical px.
- `hud.go` keeps only the dispatchers; each region lives in its own file.
- The map fills the screen and the HUD floats over it.
- Each frame's rects come from `a.hud()`. Accessors that feed `hud()` never call it.

| Region | Where | Contents |
|---|---|---|
| Resources | Top-left | Stored goods from `WorldView.Stored`, categorised and collapsible, scrolls |
| Colonist bar | Top-centre | Bust cards with mood fill and status marks; scales 1 / 0.75 / 0.5 and wraps to 3 rows, then "+N" |
| Right column | Right edge | Learning helper; alerts (severity order, hover card, click/right-click cycling, at least 3 rows kept); letter stack (kinds, right-click dismiss, saved); minimap; lower-right global stack (8 toggles, 4 speed buttons, paused reason, date, weather, cursor temperature, conditions) |
| Messages | Under the bar | Right-aligned, fading |
| Inspect pane | Lower-left | Header, bars, chips, scrolling lines; tabs above it open a panel upward (Escape closes the tab first) |
| Command buttons | Right of the pane | 64×64, wrap upward; hotkeys by declaration order; options on right-click or the corner arrow; a group merges by key (mixed, disabled for all) |
| Architect | Lower-left | 2 category columns, description box above, tools as commands |
| Windows | Bottom-left above the 11 tabs (Wildlife is new) | Never cover the bar, alerts or time controls |
| Cell info | Lower-left | Floating text when nothing is selected |

**Game side:**
- `WorldView.Letters` is saved and capped at 40, closed with `DismissLetter`.
- Every major threat is a red letter (`g.threat(title, …)`).
- `WorldView.Stored` and `item.Category()`.
- `Simulation.TemperatureAt` on the UI interface.

**Palette:** slate charcoal with a sea-glass accent. Tests pin text contrast and a minimum distance from a reference set of another game's UI colours. Badge icons sit on a cool disc; 25 new icons.

## Notes
- **Command ids:**
  - Legacy ids (`pawn.draft`, `item.forbid`, `arch.tool.*`) carry no Run and go through `applyClick`.
  - New ones are `cmd.<kind>.<name>`.
  - Commands keep their legacy ids, so click-by-id tests still work.
- **Learning helper** (`learn_ui.go`, `lessons.go`): 10 lessons, done by the player's own commands (`Renderer.onEnqueue`) or "Got it". It folds itself when the right column is cramped. Progress is saved in settings (v4).
- **Bugs fixed on the way:**
  - the Plans and Rooms toggles shared one rect;
  - clicks fell through the animal, item and plant panels;
  - hidden Work cells sat under kit windows (a History click changed priorities);
  - captures no longer opened inspector tabs;
  - a file named `hud_windows.go` built on Windows only.
- **Tests:**
  - `hud_layout_test.go` checks no region overlaps at 3 sizes × 3 UI scales, with the minimap and learning helper on and off and 6 or 30 colonists, plus grid wrap, chrome, right-click and alert room.
  - The AST draw guard covers `hud_*.go` and `cmds_*.go`.
  - `TestFrameInvariance` checks portraits in `BarSlots`.
- **Performance:** fps probe 119.8 average / 99.9 1% low (was 119.4 / 93.8). The game package takes about 3 min under `-race`.
- **Left open:**
  - When nothing is stored yet, the resource list falls back to all loose stacks (so the starting cargo shows).
  - Messages keep ageing while a wide window hides them.
  - There is no inspired or caravan mark on the bar.
  - Dragging to reorder colonists is not implemented.
