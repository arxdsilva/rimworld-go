# ART-MEAT: meat reads as steaks

Asked 2026-11-03: sliced meat should read as steaks, as in vanilla, and ours read as a blob.

## What changed

- `artlib/families/item.py` has `_steak` and `_meat`: a pile of three kidney-shaped steaks with a cream fat rim and marbling.
- Meat: red with cream marbling and a marrow bone. Human meat: pallid pink with bruise-purple veins and a bone. Insect meat: olive with pale-green marbling and no bone.
- Butcher table (`furniture.py`): the block, laid-out and hook cuts are steaks in all four rotations, with its icon recut.
- Fine meal (`food.py`): a seared steak instead of a blob, with its icon recut.

## Decisions

- Ids are unchanged, so metadata, `keys.go` and palettes were not touched.
- Lavish meal's roast (a whole joint), simple meal's stew chunks and the drying rack's thin strips stay: they aren't steaks in the dish they show.

## Verification

- `make art-check`: no errors. Targeted `-race` tests pass.
- Contact sheet and in-game crops reviewed by the lead: ground stacks, butcher table in two rotations, drying rack.

## UI

No new behaviour. Rack loads and HUD icons use the same sprites; trade and caravan lists have no item art.
