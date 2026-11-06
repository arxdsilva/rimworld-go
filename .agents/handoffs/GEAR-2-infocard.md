# GEAR-2 (infocard): the read-only Info window

GEAR-006 in `GEAR_GAP_ANALYSIS.md`; UI checklist "Inspect item" (12.2). Presentation only: the state hash is unchanged.

## What changed
- New feature package `ebiten/infocard`:
  - `Window.Open(ports.OpenInfoCard)` pushes a `uikit.Modal` (id `colony.infocard`) with a scroll view and a Close button. Esc and the title X close it through the kit; the inspector and the selection are not touched.
  - `Build` reads `PawnView` (`Apparel`, `Worn`, `ApparelArmor`, `Weapon`, `WeaponStats`, `Spare`, `Pack`) and `WorldView.Items`, plus pure catalogs: `armor.Piece`/`MoveOffset`/`SpeedPct`/`Region.Names`, `item` (mass, value, stack limit, nutrition, insulation, category), `quality`, `weapon.Stats`.
  - Rows per item: category, count, mass, value, quality, HP, material; apparel armour (Sharp/Blunt/Heat), covers, layers, insulation, move speed; food nutrition, raw, shelf life; weapon damage, penetration, range, burst, aim time, accuracy, cooldown. One or two original sentences per category (`words.go`).
  - Modes follow `command.DropGear`: 0 pack entry (Index is the entry ID), 1 worn slot, 2 weapon, 3 spare.
- `shell.go`: the `ports.OpenInfoCard` case builds the window over `a.shell` and opens it. It keeps no state, so `App` gains no field.
- Tests: `infocard/infocard_test.go` (content for apparel, weapon, food, drug, resource; missing items; open/replace/close; fit at 75-200 %), `journey_infocard_test.go` (opens through the shell over a selected pawn, Esc closes without clearing the selection, Close button, vanished item).

## Deviations
- No `infocard_compat.go`: the root package is at its 60-file ceiling (`make arch`), so the one-line constructor sits in the `shell.go` case.
- It is a modal, not a window composed in `colony_compose.go`: the kit gives the draw order, the hit swallowing, Esc and Close. Nothing was added to the compose file or `hotkeys.go`.
- No `Host`: `Shell` is `ports.World` + `ports.Widgets`; no icons.

## Gaps
- Weapon numbers are the base profile; the held weapon's quality tuning is not applied (quality is shown).
- Pack entries carry no quality or HP in the read model.
- No trigger yet except the shell intent: the Gear tab's Info button (W2-GEAR) emits it.
