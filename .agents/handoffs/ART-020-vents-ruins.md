# Task Handoff

Task ID: ART-020 (PLAY-013, MAPGEN-2 §3.4–3.5)
Status: complete

## Summary
Original art for the map's natural features:
- `structure/steam_vent`: a 2×2 low mineral-crusted mound with a dark fissure on damp ground.
- `effect/steam_puff_0..3`: a soft plume, no outline. It plays in bursts every 4–7 s per vent, each frame 150 ms, fading out, with the phase hashed from the position. Reduced Motion holds one frame.
- `structure/wall_ruin_overlay_0..2`: cracks and broken chunks drawn over ruined walls of any material, variant by position.
- `sfx.steam_vent`: a procedural hiss with two variants (cmd/audiogen), cued once per burst while the vent is drawn.

## Gameplay tie-ins (game package)
- Vents are `construction.SteamVent` (Placed and Natural). They can't be deconstructed, moved, swapped or built over. Each warms the air +6 °C outdoors in a 6×6 patch around it and raises its room's settling temperature by 1.5 °C. No power use yet.
- Ruins are ordinary walls and floors flagged `Ruin`, already damaged. They aren't colony wealth or Home anchors, builders don't repair them, and they salvage for half (rounded up). The inspector titles them "Ruined granite wall" and similar; a vent's inspector says it warms the air and can't be removed.
- Hovering deep rock says "Thick rock overhead (permanent)". Thick roofs can't be removed and drop pods avoid them.

## Notes
- `make art FAMILIES="structure effect"` re-renders the whole family with ±1/255 GPU noise. Only the new files were committed; the noisy re-renders were restored.
