# TEMP-4: layered clothing (agent A, art by AR)

Wiki fidelity under ENG-003 (temperature). Gap rows: `.agents/gaps/TEMPERATURE_GAP_ANALYSIS.md` §3.6. Art handoff: `TEMP-art-climate-and-clothing.md`.

## What changed
- 8 layer slots (`item.Slot`, `craft.Slots`), including SkinTorso, SkinLegs, Middle and Outer (the old Body slot) from the contract. The jacket is Outer (wiki). Occupancy (`item.Kind.Occupies`): tribalwear takes SkinLegs; plate and marine armour take Middle and Legs. `craft.Outfit.Conflicts` stops layer pairs sharing a region.
- Garments: T-shirt, button-down shirt, pants, jacket, cowboy hat, bowler hat, tribalwear, with tailoring recipes (also at the crafting spot) from any fabric or leather, quality and stuff. Existing garments are retuned to the wiki factors.
- Dressing (`craft/dress.go`): `craft.Best` ranks covered first, then discomfort, pieces, surplus and walk. The 1 C dead band and per-slot cooldown stay; nothing is taken off just to spare insulation.
- Naked thought: -6. Starting kits: colonists in a T-shirt and pants; tribal origins in tribalwear; NoClothes scenarios naked; raiders get a T-shirt and pants under their kit. The v8 migration dresses old colonists and moves a saved flak vest from Body to Middle.
- Rendering: `portrait.go` `figureLayers`, `worn.go` and `world/keys.go` are wired to the new sprites.

## Wiki vs original numbers
- **Wiki:** apparel factors (cold/heat), stuff insulation, garment hit points, market values (66/77/66/130/44/37/97; parka 197, duster 205), recipe work divided by 10.
- **Original tuning:** trader stock counts, tuque leather value 46, advanced helmet 15/0, wool parka 520/0, armour rows (0.2 of the stuff's armour, jacket 0.3; a cloth T-shirt rates 72/0/36).

## Tests
Dress plan and no-flap tests (`craft/dress_test.go`, `apparel_test.go`), conflict and migration tests, save round trip (see the test files for the full list).

## UI still owed (TEMP-5)
Gear tab showing layers, per-garment insulation lines in the inspector, the "Need warm clothes" alert. The comfort range and the Naked thought already show.

## Open
- No player apparel policy screen.
- Garments beyond the seven listed (capes, further hats) are not added.
