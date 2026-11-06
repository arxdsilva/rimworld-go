# UI-011 Inspectors, paused actions, move/copy, grass, tree zones

## What changed
- **Paused actions apply at once.**
  - `Game.ApplyPending` runs queued commands (the first phase of `Step`). `Renderer.enqueue` calls it while paused, through the consumer-owned `pendingApplier`.
  - Zones, blueprints, designations, roofs, bills, research, schedules, work priorities, drafting and orders now show without unpausing. Work priorities were changing all along; the panel only showed it after the next tick.
  - The clock doesn't move, so the state equals what the next `Step` would reach (`TestApplyPendingMatchesStep`).
- **`Undesignate` command:** clears one named mark (harvest, cut, hunt, tame, mine, deconstruct). Previously the animal panel and Animals tab cancelled with `DesignateCancel`, which also deleted any pen, grow zone or stockpile on that tile.
- **Selection** (`selection.go`):
  - Precedence is pawn → animal → ground item → building → plant → floor. Clicking the same tile again cycles.
  - Selecting one kind clears the others.
  - `colonistActions` builds per-colonist chips from `OrdersAt`, with disabled reasons.
- **Panels:**
  - **Plant** (`plant_inspector.go`), **item** (`item_inspector.go`) and **pawn action bar plus Equip nearby** (`inspector.go`).
  - **Building panel:** Move (`move_mode.go`: a ghost validated by `CanMoveTo`, Q/E to rotate, right click or Esc to cancel, a pending marker and Cancel move) and Copy (an Architect preset of kind, stuff and facing).
- **Move (sim, `move.go`):**
  - `MoveBuilding` places a reinstall frame (`Frame.Reinstall`): no materials, `max(60, work/2)`.
  - On completion the **same building entity** moves, keeping bills, fuel, contents, owners, HP and ID. Old cells are freed and new ones blocked; pawns on them step aside and its users are interrupted.
  - Not movable: walls, doors, floors, graves. A second move replaces the first; cancelling or losing the building drops the move.
  - Saved with the frame.
- **Grass is simulated** (`plant.GrassAt`, `game/grass.go`):
  - Seeded from terrain with the old renderer rule, so maps look the same.
  - Removed by completing a building or floor, by sowing, and by terrain change.
  - Cut plants marks grass too; growers cut it after any marked plants (work 25).
  - Views `Grass` and `GrassCut`. Saved; older saves reseed bare under buildings.
  - The renderer also hides grass at once under buildings, frames and crops.
- **Tree growing zones:** Tree is `Sowable` (`ZoneSowable`) and listed in the crop picker. Growers plant saplings, chop grown zone trees for wood (in the Grow chain before sowing) and replant. Wild trees still need Cut.

## Tests
- **New:**
  - `pending_test.go`, `paused_test.go`, `undesignate_test.go`
  - `selection_test.go`, `plant_inspector_test.go`, `item_inspector_test.go`, `inspector_test.go` additions
  - `move_test.go`, `move_mode_test.go`
  - `grass_test.go` (game and plant), `grass_rule_test.go`
  - `tree_zone_test.go` (game and plant), `tree_zone_ui_test.go`
  - `TestCancelHuntKeepsPen`
- **Gates:** gofmt, vet, staticcheck (only the old pack_test warnings), full `-race` suite, determinism ×3.
- **Coverage:** game 86.8%, plant 83%.
- **Benchmarks** (quiet machine):

  | Benchmark | Result | Baseline |
  |---|---|---|
  | ColonyHour | 17.5 ms | 17.5 ms |
  | ColonyRaid | 20.5 ms | 20.5 ms |
  | DrawArt | 2.6 ms | 2.5 ms |
  | DrawPlants | 0.71 ms | 0.72 ms |

- **Review:** capture UIs `plant`, `item`, `pawnactions`, `paused-stockpile`, `move` and `copy`, all reviewed by eye.

## Known limitations
- `CanMoveTo` repeats the placement rules of `placement.go` instead of sharing them, so the two can drift.
- Cancel move cancels the destination's origin tile, which would also remove a zone there.
- Grass never regrows.
- Trees take one in-game day to grow, as wild trees do.
- The plant panel shows at most 3 colonist chips per row.
- The pawn panel's tab row overflows slightly ("Social" label).
- No forbid, set-owner or multi-selection actions yet.
