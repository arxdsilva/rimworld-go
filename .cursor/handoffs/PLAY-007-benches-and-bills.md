# Task Handoff

Task ID: PLAY-007
Status: complete (phase 3)

## Summary

Multi-tile buildings (rotatable footprints with interaction spots), a
two-tile bed (cot stays as a one-tile kind), workbench and stove that
block movement, recipes (cook simple meal: 2 raw food -> 1 meal; wooden
club: 4 wood), bills (do X / forever / until you have X, suspend,
remove) and the Bill job (fetch one ingredient stack, carry it to the
work spot, work, drop product). Clicking a building selects it; the
building inspector shows bills with -/+/mode/pause/delete and add-recipe
buttons, plus Deconstruct. Work panel gains Craft. Art: 2x1 stove in 4
facings, club and rifle items, new icons.

## Public Contracts Changed

- construction: `Size/Spot/HasSpot/Bench` defs, `Footprint`, `Covers`, kinds `Cot/Workbench/Stove`; Bed is 1x2.
- recipe package; job `Bill` + `Record.BillIndex`; work `Craft`; item `Club/Rifle`.
- commands `AddBill/SetBill/RemoveBill`, fields `Recipe/Index/Mode/Count`, `WorkCraft`.
- read model: `BuildingView.W/H/HasSpot/SpotX/SpotY/Bench/Bills`, `FrameView.W/H`, `game.Recipes()`, `Game.CanPlaceAt(kind, stuff, facing, x, y)`.
- ebiten `Simulation.CanPlace` replaced by `CanPlaceAt`.

## Verification

go test -race ./... green. New tests: footprint rotation, multi-tile placement/blocking/work-spot rules, stove cooks meals end-to-end, bill modes/suspend/remove, waiting-for-ingredients, bills panel UI. In-game: assets/generated/ingame_bills.png.

## Known Limitations

Single-stack ingredients only, products drop on the work spot (haulers
store them), no ingredient radius/filters, no bill reordering.
