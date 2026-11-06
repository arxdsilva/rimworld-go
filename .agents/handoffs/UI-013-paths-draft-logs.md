# Task Handoff

Task ID: UI-013
Status: complete

## Summary

- **Paths:** every colonist's remaining walk is drawn on the map, cell by cell, with a ring at the destination and a square at a haul's drop point. Drafted colonists' routes are red, thicker and solid; the selected colonist's is white and drawn on top. Zoomed far out, only the destination rings are drawn.
- **Drafting:** a drafted colonist right-clicked onto a raider or animal now opens the order menu (Shoot or Melee attack). A right-click on ground still moves at once.
- **Log UI:**
  - Colonist inspector: a Log tab with wrapped, scrollable lines.
  - Animal inspector: taller, with a Recent list of 4 entries and a Log/Less toggle.
  - Disabled inspector tabs no longer print "(disabled)", which overflowed; the reason shows on hover.

## Files Changed

- `internal/adapters/inbound/ebiten/`: `paths_draw.go`, `log_ui.go`, `zones_draw.go` (ZONE-001), `orders.go`, `inspector.go`, `animal_inspector.go`, `app.go`, `capture.go`, and tests.
- `internal/application/game`: `PawnView.Path`, `HasGoal`/`GoalX`/`GoalY` and `HasDrop`/`DropX`/`DropY`, filled in `fillPawnMotion`.
- `cmd/game/main.go`: `-capture-ui zones|paths|log`.

## Verification

The UI package passes at 65.1% coverage. Tests cover path planning (every colonist, off-screen and hostile pawns skipped, zoomed out), the drafted right-click on ground versus an animal, log wrapping and scrolling, the animal Recent list and the Log toggle. Captures were checked.

## Determinism Impact

None (presentation only; the path fields are derived from saved job state).
