# STO-3: stockpile UI

UI checklist: storage filter tree, search, allow/clear all, specials, shelf priority, link, capacity readouts, zones overlay toggle and colours, bill take-from, drop weapon.

## What changed
- Storage window: Allow all / Clear all (`FilterRoot`), search box, five priority buttons, quality names, `StorageSpecials` (grave toggles only on graves), tree from `StorageFilterRows`; tree first, specials after.
- Shelves, hoppers, sarcophagi and graves: `cmd.shelf.storage`, `cmd.shelf.link` (picks one other shelf at a time; buildings cannot be multi-selected), `cmd.shelf.unlink`; group naming via `RenameZone`.
- Readouts: "Stored x of y stacks" on zone and building panes; the Accepts line reads the real filter; the dispenser pane lists hoppers and fill.
- Overlay: input action `overlay.zones` (Shift+Z) and `hud.zones`; zones are shown by default; storage zones are outlined in a fixed palette by zone id.
- Bills: the store target lists dumping stockpiles and shelves; `bill.cfg.source` float menu sends `SetStorageSource`; float menus above a modal now take clicks (`floatOverModal` in `ui_state.go`).
- Architect: hopper, nutrient paste dispenser, sarcophagus, haul urgently tool; BuildSprite keys, paste meal key, icons; sarcophagus stone mask.
- Gear: `cmd.pawn.dropweapon`.
- Art: `families/paste.py` (hopper, dispenser and glow, meal), `families/sarcophagus.py` (4 facings, stone tint mask), icons, palette `materials_sto.yaml`; validator 5965 sprites, 0 errors.

## Numbers
No wiki numbers. The zone palette is original.

## Tests
Journeys in `journey_wave2_test.go`: StorageAllowAllAndSearch, LinkShelves, HaulUrgently, NutrientPasteDispenser, BillTakesFromStockpile, DropWeapon, ZonesOverlayToggle.

## Gaps
No shift multi-select link. `PawnView.Spare` is not exposed to the Gear tab. Per-zone colours are a fixed palette, not user-chosen.
