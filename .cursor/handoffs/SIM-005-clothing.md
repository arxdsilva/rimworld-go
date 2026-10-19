# Clothing

## Items and production
- New item kinds: Leather, Parka, Tuque, Duster.
- Butchering also gives leather through the new recipe `Extra`/`ExtraQty` by-product: 5 per dustbuck, 4 per ridgewolf.
- Tailoring bench (`construction.TailoringBench`): 2x1, 7 wood, a bench under Architect > Production. Its recipes use Craft work:

  | Recipe | Leather | Work (ticks) |
  |---|---|---|
  | Sew leather parka | 14 | 900 |
  | Sew leather tuque | 4 | 300 |
  | Sew leather duster | 10 | 700 |

- Stockpiles always accept goods (leather, apparel, weapons).

## Wearing (`application/game/apparel.go`)
- `g.apparel[id]` is a `worn` array indexed by `item.Slot`: Body (parka or duster) and Head (tuque). Garments go over everyday clothes.
- Insulation is cold/heat in tenths of a degree (`item.Kind.Insulation`):

  | Garment | Cold | Heat |
  |---|---|---|
  | Parka | +20C | -3C (hot to wear) |
  | Tuque | +6C | 0 |
  | Duster | +4C | +8C |

- `exposure.Bands` replaces the fixed comfort constants. `bandsOf(id)` is `Everyday` (8 to 28C) widened by what the colonist wears. Danger, freezing and sweltering lines are offsets from the band, and everything (exposure, thoughts, seeking warmth) uses each colonist's own band.
- Orders: "Wear X" appears in the right-click menu for apparel (`OrderEquip`, so it can be queued). Wearing a garment drops whatever was in that slot.
- Autonomy (`assignWear`, during work and free time, before idling):
  - A colonist puts on a free garment if it fits the season better by at least 3C. Fit is measured against the day's mean ±10C.
  - It takes off a garment that makes the season worse, for example a parka in summer.
- Read model: `PawnView` gains Apparel, ComfortMin and ComfortMax.

## Presentation
- Art:
  - Item sprites: `item/leather_hides`, `item/parka_folded`, `item/tuque`, `item/duster_folded`.
  - Furniture: `furniture/tailoring_bench_<facing>` and `ui_icon/tailoring_bench`.
  - Worn pawn layers: `pawn/parka_leather_*` and `pawn/duster_leather_*` (with walk and work frames), and `pawn/tuque_knit_*` (cloth tint, drawn over the hair).
- `pawnLayers(l, facing, apparel...)` adds the garments after shirt and vest, and the hat after the hair.
- Inspector Gear tab (enabled): weapon, garments with what each does, comfortable range, and a hint.

## Dev
- The winter scenario now adds a tailoring bench, 2 parkas, 3 tuques and 20 leather.
- `-capture-ui dressed`: the first colonist with the Gear tab open.

## Tests
- `apparel_test.go`:
  - Butchering gives leather.
  - A colonist dresses for winter and survives a day and night outdoors.
  - The parka comes off in midsummer.
  - The "Wear duster" order works.
- `exposure_test.go`: parka bands.
