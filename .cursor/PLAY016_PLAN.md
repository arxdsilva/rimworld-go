# PLAY-016: Crops redone in the grass and bush style, plus visible cold death

## Context
You like the PLAY-010 grass and bushes and want the food-production plants to match them. Today the crops are the older art, with only their outlines removed:
- the saturated toon palette (`materials.yaml` vegetation, foliage, leaf_light…);
- a **dark soil mound under every plant**;
- scaled up 1.3–1.6×;
- drawn as **one big centred sprite per tile**, with no jitter, mirroring or growth scaling.

So a field reads as a grid of potted toys next to soft, muted, ground-level grass.

Grass and bushes, by contrast, are:
- Blender geometry from the `plant_base.py` primitives (`_limb`, `_leaf`, `_ball`, `_cluster`);
- the muted two-band `pl_*` palette (`materials_plants.yaml`);
- no outline or shadow, flat normals for small plants;
- grass composed by the renderer as several jittered, mirrored tufts per tile, scaled by growth (`grass_batch.go`).

Your choices:
- **Per tile: several small plants**, composed like grass.
- **Soil:** the mound is removed; **tilled furrows** show in the ground under planted crops.
- **Scope:**
  - all 11 crops at every stage;
  - **berry bush and wild healroot**;
  - **harvested produce items**;
  - **a cold look**, with your rule: below a per-crop temperature the crop is **destroyed over a few hours, visibly disappearing**.

**Originality:** our own shapes, palette and numbers; no RimWorld art traced or copied.

---

## 1. Art direction (lead, before any art: a new spec section `ART_DIRECTION_AND_ASSET_PIPELINE.md` §9.3 "Crops")
**Same family as §9.1 ground vegetation:**
- Blender geometry from the `plant_base.py` primitives;
- the muted two-band `pl_*` palette;
- no outline, no baked shadow;
- flat normals for the low crops (lettuce, radish, potato, beans, ashleaf, healroot, sprouts) so units can be mirrored;
- light from the same direction as the bushes.

**Crop palette.** New `pl_crop_*` entries in `materials_plants.yaml`:
- leaf greens in two bands, slightly yellower than wild foliage (cultivated, well-fed);
- produce accents desaturated toward the grass palette:

| Accent | Colour family |
|---|---|
| Corn | muted gold |
| Wheat and brewgrain | straw and bronze |
| Pumpkin | burnt orange |
| Radish | dusty rose-red |
| Cotton | warm cream |
| Healroot | pale blue-white flowers |
| Ashleaf | silver-sage |
| Potato | ochre tubers (only peeking at harvest) |
| Beans | sage pods |
| Lettuce | pale jade |
| Berries | deep plum-red |

No colour may exceed the saturation of the dandelion or fire lily accents.

**Unit plant.** Each sprite is **one small plant** (not a tile's worth), about the size of a grass tuft to a small bush. The renderer places several per tile.

**Silhouettes** (original), read at gameplay zoom:

| Crop | Silhouette |
|---|---|
| Corn | tall thin stalk, long arching leaves, tassel, a cob at harvest |
| Wheat | clump of fine stems, heads that bow when ripe |
| Brewgrain | as wheat, with heavier bronze heads |
| Cotton | branching shrub with cream bolls |
| Potato | leafy mound, small pale flowers, tubers peeking |
| Beans | twining sprawl with hanging pods |
| Lettuce | rosette, a dense head at harvest |
| Radish | short leaf fountain, a red shoulder at the soil line |
| Pumpkin | sprawling vine with big leaves, one or two fruits at harvest |
| Healroot | low herb with pale flowers |
| Ashleaf | silvery broad-leaf rosette |

**Stages** (keys unchanged):
- **sown:** no plant; only the furrow decal and a few seed specks;
- **sprout:** 2–3 tiny leaves;
- **young:** half size, leaves only;
- **mature:** full foliage, no produce;
- **harvestable:** produce visible, the plant fuller;
- **wilted:** drooping, browned, used for dying.

Variants v0..3 per stage.

**Tilled ground:** `overlay/tilled_0..3` decals, soft furrows of darker turned soil with no hard edges, tileable along rows. They are drawn under cells holding a crop, and over the ground in cells sown but not yet sprouted.

**Berry bush:** keep the bush body; the berries become smaller, plum-red clusters set into the foliage. **Wild healroot:** the healroot herb in the ground-plant style.

**Produce items:** potatoes, corn pile, wheat sheaf, beans sack, lettuce heads, radish bunch, pumpkins, brewgrain, ashleaf, berries basket, healroot's herbal medicine bundle and the cotton → cloth bolt keep their item rules (items keep the item outline per the art spec), restyled to the muted crop palette and softer, rounder forms. UI icons follow them automatically (`overlay.py` ICONS).

## 2. Renderer: crops composed like grass
**Unit layout per species** (table in `crop_batch.go`): units per tile and an arrangement in a jittered pattern, deterministic from a tile hash, mirrored at random:

| Crops | Units per tile |
|---|---|
| Corn | 2 |
| Wheat and brewgrain | 4 |
| Cotton, potato, beans, healroot, ashleaf | 3 |
| Lettuce and radish | 4 |
| Pumpkin | 1 sprawl (plus fruit at harvest) |

- **Row feel:** units keep a slight row alignment along the grow zone's long axis so a field reads in rows.
- **Scale:** each unit's scale comes from growth within its stage (no pop between stages), with a 0.9–1.1 per-unit trim.
- **Sway:** per unit (tall crops sway more), phase from the unit hash.
- **Performance:** drawn as one batch through a runtime atlas like `grass_batch.go`, with LOD:
  - far zoom draws one representative unit per tile;
  - very far zoom draws a tinted colour mark.
- **Budget:** a 500-crop field costs no more than 1 ms of draw at default zoom (fps probe).
- **Furrows:** tilled decals under crop cells (and sown-but-empty cells) drawn in the ground pass, below grass and stains.
- **Inspector and hover:** unchanged (one plant entity per cell).

## 3. Cold and heat kill plants over hours, visibly
**Domain** (`plant`):
- `Plant.Damage uint8` (0..100, saved additively);
- the `Frostbitten`/`Killed` checks become a **damage rate**:
  - below the species' kill temperature (crops: `FrostKill`; wild plants: `ColdKill`; heat: `HeatKill`), damage grows by **25% per in-game hour at the threshold**, plus 5% per °C further below or above, capped at 60%/h;
  - so a crop just below its threshold dies in about 4 hours, and a hard freeze in under 2.

**Hysteresis:**
- damage stops only once the air is 2 °C past the threshold back on the safe side;
- below 60%, damage then heals at 10%/h;
- **from 60% on, the plant is dying and keeps dying** even if it warms;
- a no-flap test covers temperature hovering at the threshold.

**Game** (`plant_growth.go`):
- the staggered per-chunk frost pass now **adds damage** instead of deleting;
- at 100% the plant is removed, with today's batched notice per species ("The cold killed 14 corn plants");
- a new one-per-day message when frost first starts hurting a species: "Frost is damaging the corn";
- harvesting a damaged crop yields `(100 − Damage)%`.

**Read model:** `PlantView.Damage` and `PlantView.DamageKind` (cold or heat).

**Visual** (renderer):
- 0–49%: the normal sprite with a growing rime tint (cold: pale blue-white frosting on the leaves) or a scorch tint (heat: brown-yellow);
- 50–100%: the **wilted** sprite, units shrinking toward 60% and fading out over the last 30%, so a field visibly thins out and disappears over the hours;
- tiles lose units progressively (units drop out one by one as damage passes thresholds);
- a hover and inspector line: "Freezing: 40% damaged" / "Scorching".

**Also applies to:** wild plants and trees (with ColdKill/HeatKill) and the berry bush. It does not apply to grass, whose snow and frost pause logic is unchanged.

## 4. Execution (parallel-work rules; ≤ 15 agents)
### Phase 0: lead contract (one commit)
- The §9.3 spec text: palette table, silhouettes, stages, unit layout table, furrow and frost visuals.
- `pl_crop_*` palette entries.
- `Plant.Damage`, the `PlantView.Damage/DamageKind` fields and the save field.
- Asset ids: unchanged crop keys plus `overlay/tilled_0..3`.
- `crop_batch.go` stub.

### Phase 1 (parallel)
| Agent | Model | Owns |
|---|---|---|
| **ART-GRAINS** | opus (looks matter most) | `plant.py` crop builders for corn, wheat, brewgrain, cotton (all stages, v0..3), without mounds, in the `pl_crop` palette; render the plant family; restore incidental re-renders |
| **ART-LEAFY** | opus | potato, beans, lettuce, radish, pumpkin, healroot, ashleaf builders + berry-bush berries + wild healroot (a separate builder file or clearly separated section to avoid conflicts with ART-GRAINS: the lead splits `plant.py`'s crop section into `crops_grain.py` / `crops_leafy.py` in Phase 0) |
| **ART-ITEMS** | sonnet | `item.py` produce items restyle, `overlay.py` tilled decals; icons follow |
| **RENDER-CROPS** | sonnet | `crop_batch.go` (layout, jitter, mirroring, growth scale, sway, atlas batch, LOD), furrow decals, the frost/scorch tint, shrink and fade, unit drop-out, hover/inspector lines |
| **PLANT-DAMAGE** | opus (hysteresis, staggered pass, saves) | `plant` damage rates, the `plant_growth.go` frost pass, messages, yield, saves, tests |

### Phase 2: lead review and verification
1. **Visual review** (iterate with the art agents until it matches the grass):
   - a contact sheet of every crop at every stage next to grass and bushes on a soil background;
   - in-game captures: a mixed field at four growth stages; a frost night killing corn (frames over the hours); a far-zoom field.
2. **Checks:** `make art-check` (no outlines on plants, no orphans: removed mound or legacy sprites deleted in the same change), `go test -race ./...`, determinism.
3. **Goldens:** winter scenario goldens may move (plants now die over hours instead of instantly). Re-pin with a field-by-field note. `TestTemperateGenerationUnchanged` and the map goldens don't move (art only).
4. **Performance:** the fps probe on the grown colony (large fields).
5. **Docs:**
   - handoffs ART (crops, produce) and SIM (plant damage);
   - ART spec §9 checklist line corrected (crops truly restyled);
   - vanilla checklist (cold and heat plant death);
   - the README crop/plants line.

## Critical files
- `assets/source/blender/artlib/families/{plant.py → crops_grain.py, crops_leafy.py}`, `plant_base.py`, `item.py`, `overlay.py`
- `assets/palettes/materials_plants.yaml`
- `internal/domain/world/plant/plant.go`
- `internal/application/game/{plant_growth.go, plant_view.go, save.go}`
- `internal/adapters/inbound/ebiten/{crop_batch.go, draw_art.go, environment.go (buildPlantKeys), plant_inspector.go}`
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` §9.3
