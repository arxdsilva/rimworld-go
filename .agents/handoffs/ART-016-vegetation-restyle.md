# Task Handoff

Task ID: ART-016 (PLAY-010)
Status: complete

## Summary
All ground vegetation follows the new ART §9.1 direction: small, flat, muted-olive sprites with transparent gaps and no outline, composed many per tile by the renderer. Grass tufts `grass_short_v0..5`, `grass_tall_v0..3`, `grass_short_dormant_v0..2`; every wild plant has young/mature/dormant looks (`<key>_<stage>_v0..3`, berry bush also `_harvestable`; wild healroot is `healroot_wild_*` to keep the crop keys); seven new plants. Crops re-rendered without outlines.

## Files
`families/plant.py`, shared primitives in `families/plant_base.py`, palette `assets/palettes/materials_plants.yaml` (`pl_*`), `cmd/normalgen` `flat_normal` (low cover gets a flat normal map so it can be mirrored). Renderer keys: `biome_art.go` ground tables, `plantSprite`, inspector "Slows walking", far-zoom colours, hover "Grass (growing, N%)".

## Rules added
AGENTS.md / ART §25: remove unused assets in the same change (`make art-check` runs an orphan check with `assets/metadata/orphan_allow.txt`), and plants never have a contour (checked by `TestPlantRecipesHaveNoOutline`).

## Follow-ups
The old `grass_00..07`, `grass_tall_00..03`, `bush_0N`, `berry_bush_0N`, `shrub_0N`, `cactus_0N`, `reeds_0N` keys are still renderer fallbacks; delete them (and their fallback entries) in one change once nothing reads them.
