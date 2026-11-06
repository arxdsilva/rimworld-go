# Task Handoff

Task ID: ART-022 (PLAY-016)
Status: complete

## Summary
**Crops** are rebuilt to ART spec §9.3, in the §9.1 family: Blender primitives, the muted `pl_crop_*` palette, no outline, no shadow, no soil mound.
- Each sprite is one small unit, `plant/<crop>_<stage>_v0..3`, for 6 stages (sown specks, sprout, young, mature, harvestable, wilted). Every variant keeps its structure through all the stages.
- Grains (`crops_grain.py`): corn, wheat, brewgrain, cotton.
- Leafy (`crops_leafy.py`): potato, beans, lettuce, radish, pumpkin, healroot, ashleaf, plus the wild healroot herb.
- `plant.py` keeps a `_crop` wrapper for the menu scene.
- The old builders and alias sprites (`crop_sown`, `<crop>_{young,mature,harvestable}`) are deleted.

**Other art:**
- **Berry bush:** small plum berries set into the foliage.
- **Produce items** (`item.py`): restyled to the crop palette with softer forms; they keep the item outline. The 12 UI icons follow them.
- **Tilled furrows:** `overlay/tilled_0..3`. Three rows per tile; tileable both ways with seams matched, so rotating 90° works.
- **Palette:** `pl_crop_*` in `materials_plants.yaml`. The unused `cloth_white` was removed.

**Renderer** (`crop_batch.go`):
- **Composition:** units per tile (corn 2, wheat/brewgrain 4, cotton/potato/beans/healroot/ashleaf 3, lettuce/radish 4, pumpkin 1), with:
  - row patterns along the zone's long axis;
  - mirroring;
  - 4 variants;
  - a continuous growth height;
  - per-unit sway.
- **Batching:** a lazy atlas, one lit batch per run of crops; LOD tiers below 48 px and at map zoom.
- **Furrows** under crop and sown cells.
- **Damage:**
  - 0–49: a rime or scorch tint;
  - from 50: the wilted sprite, shrinking to 0.6;
  - from 70: units fade out one by one.

  Trees and bushes get the whole-plant version. Shadows thin out with the units.
- **Hover and inspector:** "Freezing: N% damaged".
- **Performance:** `BenchmarkCropField500` runs 0.13 ms at default zoom.

## Notes
- `stage._render_raw` writes to a shared `$TMPDIR/rwg_render.png`: render concurrently only with a private TMPDIR.
- `PlantView.Wilted` no longer changes the sprite; damage does.
