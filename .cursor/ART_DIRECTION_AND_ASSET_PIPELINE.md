# rimworld-go --- Art Direction & Asset Pipeline

> **Purpose:** Canonical visual-production specification for human and
> AI agents creating `rimworld-go` art in parallel.
>
> **Target:** A highly readable, top-down colony-simulator aesthetic
> close to RimWorld's look (style only), with strong silhouettes,
> bold outlines, a muted palette, composited characters, consistent
> materials, and excellent readability at gameplay zoom.
>
> **Clean-room rule:** Never trace, extract, recolor, vectorize, or
> closely reconstruct proprietary RimWorld sprites, textures, icons, UI
> art, source files, or other copyrighted assets. Do not use extracted
> RimWorld assets as image-to-image inputs. Reproduce broad design
> principles and gameplay readability---not proprietary artwork.
>
> **Agent loading:** This file is *not* default context. Route via
> `.cursor/skills/rimworld-go-router/SKILL.md` and follow
> `.cursor/skills/rimworld-go-art-assets/SKILL.md`. Load §1, §2, §35, and
> only the category/pipeline sections that skill names for the task.
> Tick ART wave / prototype-gate / acceptance checkboxes in this file
> using `[x]` / `[-]` / `[!]` when work lands. For UI chrome that belongs
> to a named screen, also follow `MAIN_UI_SCREENS_SPECIFICATION.md`.

## 1. Agent Definition of Done

An art task may be checked off only when it is original, readable at
normal gameplay zoom, uses canonical scale/pivots/palette, has correct
metadata, passes asset validation, works on light and dark terrain, is
inspected in the real renderer, and includes editable source where
applicable.

Agents should read `AGENTS.md`, skill `rimworld-go-art-assets`, this
file (named sections only), the relevant gameplay/UI specification, and
only the asset-category references needed for their task.

## 2. Core Visual Principles

Priority is:

``` text
silhouette → orientation → gameplay state → material/category → detail → decoration
```

Use broad forms, restrained texture, large color regions, limited
internal lines, subtle dimensional shading, and exaggerated functional
features when useful. Avoid photorealism, noisy textures, excessive
gradients/specular highlights, tiny decoration, and inconsistent
perspective.

The visual identity is close to RimWorld's readable top-down look, with
entirely original assets: broad forms, bold dark outlines, a muted warm
palette, limbless round-headed pawns, consistent shadows, original
iconography, and a UI designed to remain readable at very large colony
sizes. The clean-room rule above still governs every asset.

ART-025 (2026-10-26, user decision): the look moves close to RimWorld's
readable style (style only, no traced or referenced art). It replaces the
earlier "distinctive pawn proportions / geometric industrial" identity.

## 3. Canonical Scale and Coordinates

Use a logical tile independent of simulation and texture resolution.

Initial art-production reference:

``` text
1 tile = 64 × 64 source pixels at @1x
@2x = 128 × 128 pixels per tile
+X = east/right
+Y = south/down
```

Keep vector/high-resolution editable sources when practical. PNG
dimensions never define collision or footprint; metadata does.

Objects may visually exceed their logical cells.

## 4. Pivots, Anchors and Attachments

World assets require deterministic anchors. Buildings anchor to their
logical footprint; pawns use a common ground/body origin; weapons define
grip and muzzle; beds can define sleeper anchors; workstations can
define interaction anchors.

Example:

``` yaml
id: core:building/bed_basic
footprint: [1, 2]
pivot: [0.5, 1.0]
rotatable: true
attachments:
  sleeper: [0.5, 0.52]
```

Use normalized coordinates where practical. All composited pawn layers
must share canonical anchors.

## 5. Camera and Perspective

Use a primarily orthographic top-down presentation with no strong
perspective convergence. Objects may show subtle top/front thickness to
communicate volume while remaining grid-aligned.

Contracts (`artlib/contracts.py`, ART-025): oblique shear `SHEAR_K = 0.35`
(a flatter camera; the renderer's `projShearK` in `light.go` matches it),
upright figures (pawns) leaned `FIGURE_LEAN_DEG = 82`, animals
`ANIMAL_LEAN_DEG = 70`. Previously 0.5, 55 and 40.

Never mix strongly isometric furniture, pure top-down structures, and
side-view objects without an explicit category rule. A sprite that looks
good alone but inconsistent beside existing assets fails review.

## 6. Lighting, Shadows and Outlines

Base sprites need restrained baked lighting because world lighting is
dynamic.

Canonical dimensional cue:

``` text
light: upper-left
shadow: lower-right
```

Use subtle highlights/contact shadows, not dramatic baked environmental
light.

At 64 px/tile, target roughly 2--3 px major outer contours and 1--2 px
internal separators. Use dark neutral/chromatic outlines rather than
pure black everywhere. Internal lines should be weaker than silhouettes.

Outline contract (ART-025; was 4 px, darken 0.32, saturate 1.25):
- `OUTLINE_PX = 6` at @2x (about 3 px at 1x), `OUTLINE_DARKEN = 0.18`, `OUTLINE_SATURATE = 1.05`: the outline is the neighbouring colour darkened, still chromatic.
- `OUTLINE_INNER_PX = 2` for the `pawn` family: layers are outlined against a figure holdout so hair, hat and pants outlines never spill onto the face or body.
- Toon bands use `RAMP_INTERP = "EASE"` (`RAMP_SOFT` 0.06) so band edges are soft; `SHADOW_T` 0.30 and `LIGHT_T` 0.80 set the band thresholds.
- Grass and plants (ground plants, bushes, crops, trees) are never outlined.

Sprites carry **no baked drop shadow** (ART-010): shadows are cast at
runtime by the sun and moon (§6.1). The upper-left toon banding stays as
the restrained baked form cue.

### 6.1 Dynamic light and shadows (normative, ART-010)

Normal maps are generated with `pkg/spritenormals`, a Go port of
[spritefusion-sprite-normals](https://github.com/Hugo-Dz/spritefusion-sprite-normals)
by Hugo Duprez (MIT; see `THIRD_PARTY_NOTICES.md`).
- **Method:**
  - Each opaque island becomes a soft bevel dome: an exact distance transform, then `R·ln(1 + inset/R)`, blurred.
  - Oklab-lightness detail is added on top.
  - Sobel slopes become normals.
- **Output:** `make normals` (part of `make art`) writes `<sprite>_n.png` (OpenGL green-up) next to every world sprite.
- **Settings:** spritefusion's defaults, with radius 8, softness 2.3 and detail scale 1.2, because our art is @2x.

The light comes from the sim clock only, so it is presentation and deterministic.
- **Sun:**
  - Rises in the east at 06:00, crosses the southern sky and sets in the west at 18:00.
  - Peak elevation: Summer 65°, Spring/Fall 50°, Winter 32°.
  - In screen space the light vector is `normalize(cos az·cos el, 0.5·sin az·cos el, sin el)`.
  - Colour is warm at dawn and dusk, near white at noon. A flat surface at a summer noon renders at its own colour (≈1.0).
- **Moon:**
  - Crosses the night sky the same way, peaking at 40°.
  - Cool `(0.55, 0.65, 0.90)` at 0.22 intensity.
  - The night stays readable: about 0.25–0.35 under open sky.
- **Ambient:** a sky ambient under open sky, and a dimmer indoor ambient under roofs. Roofed tiles get no sun, moon or shadows.
- **Lamps:** lit rooms, and tiles within 3 tiles of a burning campfire, add a warm lamp term `(0.55, 0.42, 0.25)`.
- **Shader:**
  - Every world sprite is drawn by `lit.kage`, with sky exposure and lamp light as per-vertex values so draws still batch.
  - A sprite without a normal map is lit flat.
  - Zone and marker overlays, weather particles, UI and HUD are unlit.
- **Shadows:**
  - Casters are described in metadata as `shadow: {kind: blob|box|none, w, h, d}` in tiles.
  - They fall away from the active light, length `clamp(h·cot el, 0.25·h, 3)` tiles: long at dawn and dusk, short at noon.
  - **Alpha is physical:** a shadow removes the light's direct share of flat ground, `0.95·S/(A+S)`. `S` is the light's colour times its height `L.z`, and `A` is the sky ambient.
    - Floor 0.34 (moon 0.12), scaled by how much weather lets through. Cap 0.62 (moon 0.30).
    - It fades over the first and last half hour of the light.
  - Colour `(18, 20, 30)`.
  - All shadows go into one offscreen layer with max blending, composited once, so overlaps never double-darken.
  - **No sun or moon shadows** are cast on or from roofed tiles.
  - Textures: `overlay/shadow_blob` and `overlay/shadow_box`, for buildings, plants, items and rocks.
- **Creature shadows (ART-012):** pawns and animals cast **their own silhouette**: every pawn layer (body, apparel, hair, held weapon or tool, carried item), and the animal's current trot, attack or graze frame.
  - **How they're drawn:** by the same figure code with a cast flag. The figure code is pure, with no state writes.
  - **Projection:** affine, about the feet pivot. Height above the feet maps to `ShadowDir·m·z`, where `m` is the light's length per tile of height. Width runs across the shadow. Lying poses project at a quarter of the height.
  - **Softness:** the silhouette encodes height (0 at the feet, 1 at the top), and a contact-hardening blur (`shadow_blur.kage`) softens it with distance from the feet. It's crisp at contact and soft at the tip.
  - **Contact pools:** a soft dark pool sits under every creature, also indoors, at night and in storms.
  - **Point lights:** burning campfires (height 0.5 tile, reach 4.5) and lamps in lit rooms (1.6, 6) cast shadows away from the light.
    - Each creature takes its 2 nearest lights.
    - Length follows point-light geometry `d·h/(H−h)`, clamped `[0.3h, 3]`.
    - Alpha is `0.30·falloff·(1−daylight)`; campfire light flickers.

## 7. Palette and Materials

Maintain semantic shared palettes for wood, steel, dark/light metal,
stone, cloth, leather, soil, vegetation, water, ice, glass, electronics,
ceramic, flesh/organic, and bone.

Use restrained saturation. Strong saturation is primarily for gameplay
signaling, faction accents, emissives, danger, selection, and
exceptional items.

ART-025: the palettes get a muted warm pass through
`tools/palette_mute.py --contrast 0.8 --outline 0.18,3`: saturation and
light/shadow spread x0.8 toward base, light band value x0.96, shadows up
to 8 degrees warmer. Emissive entries and neutral tint greys are skipped,
and a header marker stops it being applied twice. Do not hand-saturate
palette entries back up.

Prefer neutral source art plus explicit runtime tint masks for clothing,
hair, faction accents, fabrics, and paintable objects.

Material language: - **wood:** warm, broad minimal grain; - **metal:**
cooler, planar highlights, harder edges; - **stone:** muted, broad
irregular variation; - **cloth:** soft color blocks/minimal folds; -
**electronics:** geometric housing plus sparse emissive indicators.

Do not encode essential state by hue alone.

## 8. Terrain

**NG-1 water (normative):** terrain `shallow_water` (walk 300, unbuildable), `deep_water` (impassable), `mud` (walk 140, fertility 70), `track` (dirt road, walk 90, not farmable); 4 variants each (`terrain/<type>_v0..3`), 16-mask edges for mud-over-soil and shallow-over-deep, shore-foam masks; `ebiten/water.kage` ripple + caustic glints on the presentation clock (4-frame fallback; Reduced Motion freezes it). See `NG1_NEW_GAME_PLAN.md` §4.3, §7.3.


Initial terrain families: soil, rich soil, stony soil, sand, gravel,
mud/marsh, shallow/deep water, ice, rough stone, constructed floors, and
snow.

Shipped (ART-013): sand, marsh and ice with transition edges alongside the earlier families; grass is tinted per biome at runtime.

Terrain uses low-frequency texture and should not compete with pawns.
Natural terrain should have deterministic visual variants selected from
stable coordinates/visual seed. Use transition masks, edge decals, or
shaders to avoid checkerboard boundaries.

Separate deterministic decals can provide pebbles, cracks, grass tufts,
debris, blood, scorch marks, snow patches, and similar variation.

## 9. Rocks and Plants

Rock walls must connect visually, read as solid, remain distinct from
constructed walls, and support material families. Chunks use several
compact silhouettes and material tinting. Ore/resource cues should be
embedded and restrained rather than arcade-bright.

Plants prioritize silhouette and growth state. Support
seedling/young/mature/harvestable/dying states where useful. Trees may
exceed one tile visually but retain a clear ground origin. Crop species
should be recognizable at maturity; wild plants can use more silhouette
variation.

Vegetation set (family `plant`; trees in `families/trees.py`):

- Trees: `tree_{oak,birch,pine,jungle}_00..02`. Oak and birch also have
  `_autumn` and `_bare`; pine has `_snow`. Every tree sprite also exists
  as `…_trunk` and `…_canopy` parts on the **same canvas and origin**
  (§9.2). Metadata `sway: {trunk, canopy}`.
- Ground plants: `<key>_<stage>_v<n>` with stage `young` or `mature`,
  plus `<key>_dormant_v<n>` (leafless, dry or frozen look). Berry bushes
  add `_harvestable`. Keys: `bush`, `berry_bush`, `shrub`, `reeds`,
  `cactus`, `healroot`, `bramble`, `moss`, `agave`, `ball_cactus`,
  `elephant_ear`, `fire_lily`, `dandelion`.
- Grass: `grass_short_v0..5`, `grass_tall_v0..3`,
  `grass_short_dormant_v0..2`, composed many per tile by the renderer
  (§9.1).
- Crops: `<species>_<stage>_v0..v2` with stages `sown`, `sprout`,
  `young`, `mature`, `harvestable` and `wilted`. Variants change leaf
  count, angle and height, not just colour.
- **No contour, ever:** grass and every plant (ground plants, bushes,
  crops, trees and their parts) render with `outline_width=0`. Shapes
  separate by colour value alone: darker recesses, lighter tips.
- `flat_normal: true` in metadata makes `cmd/normalgen` write a flat
  normal map (low cover: grass, moss, dandelion) instead of the dome
  relief, so the sprite may be mirrored without wrong lighting.

### 9.1 Ground vegetation (normative, PLAY-010)

**Identity.** Ground vegetation is many small, separate, flat-shaded
sprites laid over an earthy terrain texture, never one big painting of
a field. Up close each plant reads by a few chosen shapes; at colony
zoom the shapes merge into tufts and shrubs; far out they only tint the
ground. A single tile stays sparse (soil shows between blades); a
landscape reads dense because many small sprites overlap. Gameplay
comes first: colonists, animals, items and buildings must stay easy to
pick out over any vegetation.

**Style.** Flat colour shapes with two value bands, modest internal
shading, no strong highlights, no wet or translucent leaves, no
photographic detail, no pixel-art blockiness, no black outlines (no
outlines at all, see above). Neutral, upright rest pose so wind can
lean the sprite. Fully transparent background with real gaps between
blades and leaves; never a square patch.

**Palette** (`assets/palettes/materials_plants.yaml`): dark olive
#414E32, forest olive #556442, grass green #708052, sage #87936C, dry
green #91915F, khaki #A49B70, stem brown #766247, dried foliage #857651,
plus accents used sparingly: berry blue-violet, fire-lily orange,
dandelion pale yellow. About two thirds of a plant sits in its middle
green; darker olive and brown separate overlaps; light yellow-green
touches only exposed tips. Biome tint (`grassTint`) shifts the whole
set warmer or cooler; never neon, never cyan.

**Silhouettes** (each must read at 64 px):

| Plant | Silhouette |
|---|---|
| grass (short) | a loose fan of 4–10 thin tapered blades from a narrow base, some leaning, a few lighter for overlap |
| grass (tall) | the same blades, about 1.6× taller and fuller, a jagged top line |
| bush | a low, slightly wider-than-tall mound with a scalloped edge, mottled leaf clusters, darker base |
| berry bush | the bush mound with small blue-violet berry dots that never dominate |
| shrub | a low star rosette of 6–12 broad pointed leaves with gaps at the tips |
| reeds | thin upright stems with small brown heads |
| cactus | an upright column cactus, clearly not the ball cactus |
| healroot | a low herb, medium-width leaves round a centre |
| bramble | a low horizontal tangle of crossing brown woody stems with scattered olive leaves |
| moss | a flat mottled grey-green cushion hugging the ground |
| agave | a rosette of 8–14 thick blue-green pointed leaves |
| ball cactus | a squat ribbed ball, faint grooves, no long spines |
| elephant ear | 6–10 large arrow-shaped leaves radiating out with gaps between |
| fire lily | a fan of strap leaves with 2–3 small orange flower clusters above |
| dandelion | a tiny green rosette with one or two pale yellow dots |

**Growth and seasons.** Young plants are smaller with more empty space;
mature ones are fuller, never a new design. Dormant looks keep the
plant's structure with brown, dried, sparser foliage (bramble keeps its
stems, grass shrivels to straw). Snow hides low cover; only trees get
autumn colours.

**Composition (renderer).** Grass is drawn as up to nine tuft copies per
tile in a jittered 3×3 grid; edge copies may sit up to 0.15 tile past
the tile edge so neighbours interleave. Copies pick a variant, may be
mirrored and scale with growth (short 0.45–0.65, tall 0.7–0.9). Other
ground plants are one sprite per plant, varied by `tileHash`. Ground
vegetation never uses linked autotiles.

**Acceptance.** Blades readable at medium zoom; bush, rosette and tangle
clearly different; terrain visible through grass and open foliage;
palette consistent across the set; scale order grass < bush < tree;
repeated sprites show no obvious pattern; alpha only, no square edges;
no contour; creatures and items stay readable on top.

### 9.2 Trees (normative, PLAY-010)

- **Trunk:** visible from the ground to at least 55% of the tree's
  height, a slight flare at the root, bark materials `bark` /
  `bark_birch`.
- **Canopy:** loose leaf clusters with gaps, no solid core. It sits on
  the upper trunk, overhangs about half a tile into the tile above and
  at most 0.2 tile to each side; total height about 1.5 tiles above the
  root. Outer clusters are soft and irregular so neighbouring canopies
  overlap into one forest edge; variants differ in canopy height
  (±0.1) and lean.
- **Pine:** visible trunk under tiered boughs starting near 35% height.
  **Jungle:** tall trunk, broad flatter crown.
- **Parts:** `_trunk` (trunk and lowest limbs) and `_canopy` (leaves and
  upper limbs) share the full sprite's canvas and origin. The renderer
  draws trunk then canopy; the canopy (never the trunk or shadow) fades
  to 40% opacity while a creature stands on the tree's tile, the tile
  above or the two tiles diagonally above, or the hovered/selected
  thing is there, easing over about 0.2 s (instant with Reduced
  Motion).

### 9.3 Crops (normative, PLAY-016)

Crops belong to the §9.1 family: Blender geometry from the
`plant_base.py` primitives, the muted two-band palette (`pl_crop_*` in
`assets/palettes/materials_plants.yaml`), no outline, no baked shadow,
the bushes' light direction; low crops (lettuce, radish, potato, beans,
ashleaf, healroot, every sprout and sown mark) carry flat normals so the
renderer may mirror them.

-   **Unit, not tile.** A crop sprite is one small plant about the size
    of a grass tuft to a small bush. The renderer composes several per
    tile (`crop_batch.go`): corn 2; wheat and brewgrain 4 clumps; cotton,
    potato, beans, healroot, ashleaf 3; lettuce and radish 4; pumpkin 1
    sprawl. Units are jittered, mirrored and scaled by growth, with a
    slight row alignment along the field.
-   **Root and containment (PLAY-019).** A unit's root is the art's
    tile-centre point (`origin.y + 64` atlas px, the shared plant
    convention in `plant_base.py`), never the footprint's bottom. Every
    unit stays on its own tile: one whose top would cross the north
    edge moves south (root at most 0.95 of the tile), then shrinks until
    it fits (`cropFit`). So tall crops show fewer, full-height plants
    per tile (ripe corn reads as one stalk). Sway pivots at the root.
    `TestCropUnitsStayOnTheirTile` checks every crop, stage and pattern.
-   **Zone fill (PLAY-020).** A growing zone's wash and stripes are not
    drawn on cells holding the plant it grows (its crop, or a tree in a
    tree zone): the furrows and plants show there, at every zoom; empty
    cells, the outline and the label keep the zone visible.
-   **No soil mound.** Tilled furrows are the ground decal
    `overlay/tilled_0..3` under cells holding a crop or sown seed.
-   **Silhouettes (ours):** corn a tall thin stalk with arching leaves,
    tassel and a cob when ripe; wheat a clump of fine stems whose heads
    bow when ripe; brewgrain like wheat with heavier bronze heads; cotton
    a branching shrub with cream bolls; potato a leafy mound with small
    pale flowers and tubers peeking at harvest; beans a twining sprawl
    with hanging pods; lettuce a rosette closing into a head; radish a
    short leaf fountain with a red shoulder at the soil line; pumpkin a
    sprawling vine with big leaves and one or two fruits; healroot a low
    herb with pale blue-white flowers; ashleaf a silvery broad-leaf
    rosette.
-   **Stages** (keys unchanged, `plant/<crop>_<stage>_v0..3`): sown (seed
    specks only), sprout (2-3 tiny leaves), young (half size, leaves
    only), mature (full foliage), harvestable (produce visible),
    wilted (drooping, browned: dying). The `<crop>_{young,mature,
    harvestable}` aliases stay for fallbacks.
-   **Palette accents:** corn muted gold, wheat straw, brewgrain bronze,
    pumpkin burnt orange, radish dusty rose-red, cotton warm cream,
    healroot pale blue-white, ashleaf silver-sage, potato ochre, beans
    sage, lettuce pale jade, berries deep plum-red; none more saturated
    than the dandelion or fire-lily accents.
-   **Cold and heat:** a damaged crop shows a rime (cold) or scorch
    (heat) tint, then the wilted sprite while its units shrink and fade
    out over the hours it takes to die (renderer presentation of
    `PlantView.Damage`).
-   **Produce items** keep the item outline rule but share the crop
    palette and softer, rounder forms.

## 10. Pawn Visual System

**NG-1 builds (normative):** one parametrised rig (`families/pawn.py` `BUILDS`) gives six builds — `m_thin, m_avg, m_heavy, f_thin, f_avg, f_heavy` — for body and every apparel layer, every facing; keys `pawn/<layer>_<build>_<facing>`, the old `pawn/body_…` keys alias `m_avg`. Hair styles short, crop, bun, long, braid, shaved, curls, tail; beards short, full, moustache (males, 40 %); a grey-hair palette entry. The pawn family is atlas-packed by `cmd/atlasgen` (≤4096² pages, `assets/metadata/atlas/<family>.yaml`). Looks are domain data (`pawn/look.Look`); `look.Legacy(id)` reproduces pre-NG-1 looks. See `NG1_NEW_GAME_PLAN.md` §4.6, §7.1.


Pawns are compositional, not one flattened sprite per colonist.

Conceptual layers:

``` text
ground shadow
body
skin/head
hair-back
lower apparel
torso apparel
outer apparel/armor
head
hair
facial hair
headgear
equipment
damage/status effects
selection/status UI
```

Create an original silhouette (ART-025): a round head on a rounded body
capsule, dot eyes (two facing south, one east, none north), and **no
arms, legs or hands**. Identity comes from build, hair, beard and apparel.
- Six builds are capsule widths and head scales; apparel layers are thin shells over the body.
- Rendered facings are south, east and north; west is east mirrored by the renderer. There are no walk or work frames (§27).
- Body attachments: `head` (hair and hats; with the feet it sets the body centre held items are placed from, §15), `hold` (the punch trail's point) and `back` (a rescued patient).

Support reusable body types, multiple head shapes, tintable skin/hair,
separate facial hair, layered apparel, headgear, and separate rotatable
equipment. All layers share attachment conventions.

Renderer handles orientation, downed/sleeping posture, carried objects,
aiming, fire/status, damage, and selection where possible.

## 11. Animals

Species must be identifiable by silhouette at normal zoom: body length,
head, ears/horns, tail, and proportions matter more than detail. Prefer
a small direction set (`south/east/north`, ART-025) and mirror east for
west. Life stages may use scale or dedicated juvenile
silhouettes.

### 11.1 Roster and builder (normative, ART-011)

- **Roster:** 61 species, listed in `assets/source/blender/artlib/families/animal_roster.py`.
  - It mirrors `internal/domain/animal` (same keys and order).
  - Each row's `features` text is the binding visual brief: silhouette, proportions, palette and markings.
  - The species exaggerate their defining traits: the elephant's trunk and ears, the ibex's horns, the fennec's ears, the squirrel's tail.
- **Originals rule:** RimWorld's invented creatures are never reproduced. Ten roster entries are **original** creatures filling the same roles, with their own names and looks:
  - emberbuck, cinder_rat, woolhorn, glimmerstag, timbergnaw
  - hive_beetle, hive_brute, hive_crawler
  - ground_sloth and dire_wolf (both real extinct animals)
- **Sizes** are body lengths in tiles: XS 0.44 (raised from 0.35 in PLAY-010; XS+ stays below S-), S 0.6, M 0.95, L 1.4, XL 2.0, with `+`/`-` meaning ±15%. Large animals overhang their tile.
- **One parametric builder:** `families/animals/` (`rig.py` plus archetype modules). Archetypes:

  | Archetype | Species |
  |---|---|
  | quadruped | most mammals |
  | primate | monkey |
  | bird | |
  | snake | |
  | lizard | |
  | tortoise | |
  | beetle, brute, crawler | hive insects |

- **Per species and rendered facing (south/east/north; west is east mirrored by the renderer; canvases are centred on the tile, lean `ANIMAL_LEAN_DEG` 70):**

  | Sprite | Frames |
  |---|---|
  | standing | 1 |
  | trot | `walk01..08` |
  | rest (lying) | `rest` |
  | attack (by the species' natural weapon) | `bite1..4`, `claw1..4`, `kick1..4`, `gore1..4`, `headbutt1..4` or `peck1..4` |
  | graze | `graze1..2` |
  | flinch | `flinch1..2` |

  - Corpses are `item/corpse_<key>_v0..v2`, three lying poses.
  - The icon is `ui_icon/<key>`.
- **Dimorphic species** (deer, elk, chicken) add a full female set, `animal/<key>_f_<facing>[_frame]`. The renderer picks it by `AnimalView.Female`.
- **Style:** fixed palette materials (no tint masks), dark outlines, toon bands, restrained 3–6 tones per species, tiny eyes. No baked shadow: shadows are cast at runtime (§6.1).
- **QA:**
  - A 64 px silhouette sheet of all 61 species, where each must be identifiable by its signature features.
  - A gameplay-zoom contact sheet.

## 12. Buildings and Construction

Function dominates decoration. A bed visibly communicates sleeping
direction; a workstation communicates its production category; a
generator looks mechanically energy-producing.

Every building declares footprint, pivot, rotation behavior,
material/tint regions, and attachment points. Runtime rotation is
preferred when visually sound; asymmetric assets can use authored
directional variants.

Construction presentation should generally be:

``` text
blueprint → frame/scaffold → completed object
```

Use shared crack/scorch/smoke damage overlays instead of bespoke damaged
versions of every building.

## 13. Walls, Doors, Floors and Furniture

Walls need adjacency-aware isolated/straight/corner/T/cross
connectivity. Material changes appearance without changing topology.

Doors clearly communicate orientation and open/closed state. Floors stay
visually subordinate to pawns/furniture and use subtle tileable
variation.

Furniture uses simple functional silhouettes. Initial families include
beds, tables, chairs, shelves, lamps, medical furniture, recreation
objects, and storage. Separate geometry from material/tint whenever
possible.

## 14. Production and Power

Workstations must communicate category by shape rather than color alone:
stove, butcher table, tailoring, smithing, machining, fabrication,
research, etc.

Power producers have clear mechanical identity. Conduits remain subtle
normally and become legible in the power overlay. Powered/unpowered
state should preferably use emissive masks, lights, animation, or
overlays instead of duplicate sprite libraries.

## 15. Weapons, Apparel and Items

Weapon silhouettes must clearly distinguish
pistol/revolver/SMG/rifle/shotgun/long
rifle/launcher/knife/club/sword/spear. At world scale, silhouette beats
realistic mechanical detail.

Weapons define rotation pivot, grip, and muzzle:

``` yaml
grip: [0.31, 0.55]
muzzle: [0.94, 0.48]
```

Held gear (family `gear`: `weapon/*`) is one east-pointing sprite per
item, rotated by the renderer, with image-px attachments: `grip`,
`support` (towards the working end), `muzzle` (firearms) and `tip`
(knife, spear, club). Melee set: `weapon/knife`, `weapon/spear`,
`weapon/club`; firearms: `weapon/rifle_basic` (+ `stock`).

Held items follow RimWorld's observable behaviour with our own values
(ART-027, 2026-10-26, user decision):

- A weapon is drawn only on duty: drafted, a raider on the attack, or
  aiming or fighting (a hunt aims). Off duty nothing is held: no slung
  rifle, no weapon in hand, no tool, no utensils.
- The placement is centre-based: the sprite's centre (canvas centre on
  its `grip` line) from the body centre (the `head` x, halfway from the
  head to the feet). Carried at rest: 0.18 tile below the body centre,
  forward end 0.72 rad down; facing east 0.14 tile forward, west mirrored;
  behind the body facing north. Aimed: 0.35 tile out from the body centre
  towards the target, turned to it, flipped when aiming left so it stays
  upright, behind the body only when aiming into the north half.
- A hauled item is its own sprite at 0.5 tile, centred on the body at the
  chest (0.07 tile below the body centre), in front (behind facing north).
- No tool sprites exist: work reads as the pawn facing the job (lean, dip,
  squash), the job's particles at the work spot and an in-world progress
  bar (§27.2).

Apparel uses shared body templates, body layers, tint regions, covered
regions, and offsets.

Resource/item icons need compact recognizable silhouettes. Stacks should
not draw one sprite per item. Quality is primarily UI information rather
than unique art.

## 16. Food, Damage and Effects

Food categories should be distinguishable without dominating the scene:
raw plants, meat, meals, preserved food, animal feed,
medicine/consumables.

Blood, scorch, debris, and structural damage should be reusable decal
systems with deterministic variation.

Fire, smoke, rain, snow, explosions, muzzle flashes, projectiles,
impacts, and similar visuals are presentation effects. Keep them
brief/readable and never let animation timing determine simulation
outcomes.

Weather (family `weather`, driven by `WorldView.Weather`,
`WeatherIntensity`, `WindX/WindY`, `SnowDepth`, `Wetness`):

- Rain: thin streaks angled by the wind, density by intensity, small
  splash sprites where drops land, ripples on puddles. Storms darken the
  scene and flash rare lightning.
- Snow: flakes of a few sizes that flutter and drift with the wind,
  settling into snow cover; blizzards cut visibility.
- Ground: `snow_cover_00..05` and `snow_drift_00..03` by `SnowDepth`,
  `puddle_00..05` by `Wetness`, `frost_00..03` on freezing mornings, each
  picked per tile by hash.
- Particle textures: `raindrop`, `splash_00..02`, `snowflake_00..03`.

Roofs and interiors (§16.1, SIM-018): roofs are never drawn as tiles.
- A roofed tile reads as indoors through light (§6.1): indoor ambient, lamp warmth, and no sun, moon, shadows, rain or snow.
- Soft eave shadows run inside walls that bound roofed tiles (6 px, 10 px under top walls).
- The Roof overlay (R or the HUD button, forced on with a roof tool) shows:

  | Roof | Overlay style |
  |---|---|
  | Built | blue `{90,140,210,70}` with a lighter edge |
  | Natural (mined rock) | slate `{110,110,125,85}`, stippled |
  | Planned | amber dashes |
  | Planned removal | red hatch |
  | Unsupported, under the tool | red with ×s |

- Collapse dust puffs where a roof falls.
- Icons: `roof_build`, `roof_remove`, `overlay_roof`.

Corpses: animal corpses `item/corpse_<species>_v0..v2` (different lying
poses); human corpses are drawn from the deceased's own pawn layers,
lying, with the dead tint and the rot tint as they spoil; fully rotted
bodies become `item/bones_00..01`. Graves (`furniture/grave_dug_*`,
`grave_filled_*`, `headstone_00..03`) and the funeral pyre
(`furniture/pyre`, `pyre_burning`) belong to the furniture family.

### 16.2 Blood and stains (normative, SIM-020 / ART-013)

- **Sim state** (deterministic, saved): each tile holds a stain with an amount from 0 to 1000 and the tick of its last fresh blood.
  - **Hit:** `40 + 25·damage` on the target's tile, plus 40% of that on the tile beyond it, away from the attacker, for hits of 6 damage or more.
  - **Death:** a pool of 700.
  - **Bleeding:** creatures drip 80 on every 3rd tile they walk. Downed, untended colonists pool by 60 an hour, up to 600.
  - Hive insects leave **ichor**.
- **Fading and washing:**
  - A full stain fades linearly to nothing in **7 in-game days**.
  - Outdoors, it washes off in **2 h of full rain** or **3 h of snow**, proportionally slower in lighter weather.
  - Under roofs it only fades.
  - At most 2000 stains; the faintest go first.
- **Drawing:**
  - Procedural splat, pool and drip textures, chosen by amount (under 0.2 drips, under 0.6 splats, otherwise pools).
  - Rotated and offset by a hash of the tile.
  - **Alpha = amount**, so fading is smooth.
  - Colour dries over a day: blood #7a1414 to #4a1d1a, ichor #a8a030 to #5c5a1c.
  - Drawn lit, after the ground cover and floors and before shadows.
- **On creatures:**
  - Blood blotches spread with lost health (`lit.kage` Custom3 coverage). For colonists it's stronger while a wound is open, and it clears as they heal.
  - A fresh hit adds a wet sheen for 2 s.
  - It covers bodies and clothes, not hair or held items.
- **Sprays** scale with damage (`4 + damage/2` particles), with a few droplets flicking back toward the attacker.

### 16.3 Dirt, carpets, cotton and the Home area (normative, SIM-021)

- **Tracked dirt on the ground** (`WorldView.Dirt`, amount 0..1):
  - Procedural scuffs, smudges and boot prints, from one atlas through `drawLit`.
  - Drawn after floors and **before** blood (`dirt_draw.go`).
  - Colour: soil brown, #6b5238 (light) to #4a3826 (heavy). **Alpha = 0.15 + 0.6·amount**, so a light trail reads as faint prints.
  - Under 0.25 the tile shows 1–3 boot prints, oriented by a tile hash. Above that, a smudge grows with the amount.
- **Carpets** (`furniture/carpet_<material>`; materials straw, cloth, leather, wood):
  - A flat 1×1 mat about 0.8 tile wide, with a slight pile and a 2 px darker border.
  - No facing. It is drawn in the floor-marking pass, under pawns and furniture.
  - Looks by material:
    - **straw:** woven tan #c8a868, with a visible weave
    - **cloth:** a dyed pile with a border stripe, using the `cloth` tint preset
    - **leather:** a stitched brown hide, #7a4e2c
    - **wood:** slats with gaps, #8a6a44
  - Rolled item sprites: `item/carpet_<material>_rolled`.
- **Cloth and straw items:** `item/cloth_bolt` (an off-white bolt) and `item/straw_bundle`.
- **Cotton crop:** `plant/cotton_{sprout,young,mature,harvestable}` with `_v0.._v2`, like the other crops. Bushy green stems; when harvestable, white bolls.
- **Cleaning:** the low reach pose with dust particles; no broom is drawn (ART-027).
- **Home overlay:** a translucent blue tint (#4a7fd4, alpha 0.18) with a 1 px edge on its outline.
  - Shown while a Home tool is held, or when toggled from the overlay row next to Roof.
- **Icons** (`ui_icon/`, underscores): `cloth`, `straw`, `straw_carpet`, `cloth_carpet`, `leather_carpet`, `wood_carpet`, `carpet` (the Architect tool), `home`, `home_clear`.

## 17. UI Art

Use an original high-information-density interface: neutral charcoal
surfaces with a warm gold accent (§33), restrained borders, clear
typography, modest texture, strong hover/selected states. No copied
panel art, icons, typography, text or visual layouts.

Create coherent icon families for architect categories, orders, work,
speed, pawn status, health, needs, research, factions, trade, overlays,
alerts, plugins, and developer tools. Icons need consistent stroke/fill
language, transparent backgrounds, semantic IDs, and small-size
readability.

Pawn portraits should initially derive from the compositional pawn
system.

### 17.1 Setup screens, font and storyteller portraits (normative, NG-1)

- **Font:** Noto Sans Regular/SemiBold (OFL 1.1, embedded from `ebiten/fontdata/`); type scale Small 11, Body 13, Label 13 SemiBold, Large 16, Title 22, Huge 32; UI scale 90–130 %.
- **Setup chrome:** centred dialog frame (≤1240×760, title 44 px, footer 56 px; Back bottom-left, forward bottom-right) over the dimmed diorama + baked starfield + a distant planet disc; widget tokens in `assets/palettes/ui.yaml` (text, fields, sliders, scrollbars, tooltips, modal dim).
- **Storyteller busts** (painted raster sources in `source/raster/ui/portraits/` with their prompts in `briefs.md`, cut by `make portraits` / `tools/portrait_cut.py` into 512×640 busts + 120×150 head-and-shoulders thumbnails, transparent, dark contour; the style matches the main-menu colonist: heroic comic bust, bold black cel outlines, flat cel shading, slight painterly texture; ART-023): Orla Vane (late 50s, close-cropped silver hair, dispatcher's headset round the neck, quilted navy jacket with a reflective stripe, calm half-smile); Bram Holloway (60s, broad, ruddy, short white beard, knitted cardigan over an apron, steaming mug); Kestrel Quay (30s, angular, asymmetric dark hair with one shaved side, long coat with brass buttons, a card between two fingers, one raised eyebrow). Never reference RimWorld storyteller art; silhouettes, palettes and props must differ clearly.
- **Icons:** `ui_icon/scenario_<key>` + `scenario_custom`, `passion_minor|major`, `skill_<key>`, `trait`, `factions`, `minimap`, `components`, `industrial_medicine`, `tool_cabinet`, `hospital_bed`.
- Details: `NG1_NEW_GAME_PLAN.md` §1, §2, §7.

## 18. Factions and Overlays

Factions are data-driven through emblem, accent palette, optional
apparel palette, and optional architecture palette. Prefer shared world
art plus faction identity layers over entirely unique asset libraries.

**NG-1 factions (normative):** emblems `ui_icon/faction_<settlers|clans|drifters|freebooters|swarm|player>`, settlement icons `ui_icon/settlement_<neolithic|industrial>` tinted by the faction accent; the globe is vertex-coloured from `assets/palettes/world.yaml` (one colour per biome mixed toward `mountain_tint` by hilliness 0/12/25/40 %), rivers, dashed roads, faction-tinted settlement emblems, hover/selected tile outlines, starfield. See `NG1_NEW_GAME_PLAN.md` §2.7, §4.1, §7.3.

Renderer-owned reusable overlays include selection, hover, hostile
target, blueprint, forbidden, hunt, harvest, cut, mine, deconstruct,
cancel, route/path, reservation, and debug markers.

## 19. Repository Layout and Naming

Recommended structure:

``` text
assets/
├── source/
│   ├── vector/
│   ├── raster/
│   ├── blender/
│   ├── aseprite/
│   └── masks/
├── generated/
│   ├── world/
│   ├── ui/
│   ├── effects/
│   └── atlases/
├── metadata/
├── palettes/
└── AGENTS.md
```

Use lowercase stable paths and namespaced IDs:

``` text
core:building/bed_basic
core:item/steel
core:weapon/rifle_bolt
core:pawn/hair/short_01
```

`source/` is canonical editable art. `generated/` is reproducible
output. Never manually patch generated art when canonical source exists.

## 20. Preferred Toolchain

Recommended baseline:

``` text
Vector                 Inkscape / Affinity Designer
Raster cleanup         Krita / Photoshop
Sprites/animation      Aseprite
3D-assisted source     Blender
Automation             Go tools + editor CLIs
```

Prefer reproducible formats such as SVG, PNG, ORA, ASE/ASEPRITE, BLEND,
and YAML/JSON metadata. A proprietary editor must not be required merely
to build the game from committed generated assets.

## 21. Vector, Raster and Blender Workflows

**Vector:** brief → silhouette → canonical template → material regions →
outline → restrained shading → metadata → automated raster export →
in-game review.

**Raster:** canonical canvas → broad forms → low-frequency variation →
edge cleanup → transparency validation → metadata → in-game review.

**Blender:** use one canonical `.blend` template containing orthographic
camera, lighting, world background, reference tile plane, origin, and
render resolution. Use simple stylized materials, render transparent,
then normalize outline/palette and optionally paint over. Agents must
not invent independent camera/light setups. Whole scenes (the main-menu
diorama) compose family builders into one render through the same rig
(`stage.render_scene`), never a scene-specific camera or light.

## 22. AI-Assisted Art

AI is best used for silhouette ideation, concept sheets,
furniture/plant/building variations, material ideas, and original icon
concepts.

Never use extracted proprietary assets as image-to-image inputs or
request pixel-perfect recreation.

Preferred pipeline:

``` text
asset brief
→ art-bible constraints
→ multiple original concepts
→ choose silhouette
→ redraw/vectorize/paint
→ normalize palette and outline
→ fit canonical template
→ metadata
→ in-game test
```

Base prompt:

``` text
Create an ORIGINAL concept for a minimalist top-down science-fiction
colony simulation game.

Asset: <DESCRIPTION>
Gameplay purpose: <PURPOSE>
Material: <MATERIAL>
Logical footprint: <W> × <H> tiles

Requirements:
- orthographic near-top-down presentation
- extremely readable silhouette
- simple geometric forms
- restrained saturation
- minimal internal detail
- subtle upper-left form lighting
- isolatable/transparent background
- readable at small gameplay size
- original design, not a recreation of an existing game's asset

Do not include text, UI framing, photorealism, dramatic perspective,
complex backgrounds, or copyrighted game assets.
```

## 23. Canonical Templates

Create and freeze templates before bulk production:

-   [x] building 1×1
-   [x] building 1×2
-   [ ] building 2×2
-   [ ] building 3×1 / 3×2
-   [x] furniture
-   [x] weapon
-   [x] pawn body/head/hair/apparel
-   [x] animal
-   [x] plant
-   [x] item
-   [-] UI icon (cut from world renders; no dedicated icon template yet)
-   [x] terrain
-   [ ] decal

Each template marks logical tile bounds, safe visual bounds, pivot,
attachment anchors, reference outline, palette, and light direction.

## 24. Metadata and Tint Masks

Use versioned sidecar metadata.

``` yaml
id: core:building/bed_basic
type: building_sprite
source: assets/source/vector/buildings/bed_basic.svg
footprint: {width: 1, height: 2}
pivot: {x: 0.5, y: 1.0}
rotatable: true
material_regions: [frame, fabric]
attachments:
  sleeper: {x: 0.5, y: 0.52}
export:
  world_scale: 1
  padding: 8
```

Tint channels may include primary/secondary material, fabric, skin,
hair, faction accent, and emissive. Do not encode undocumented semantics
into arbitrary RGB channels.

## 25. Automated Asset Pipeline

Production path:

``` text
editable source
→ validator
→ raster exporter
→ trim/padding
→ atlas packer
→ manifest
→ game
```

Create a Go tool:

``` text
go run ./cmd/assettool validate
go run ./cmd/assettool build
go run ./cmd/assettool atlas
go run ./cmd/assettool report
```

Validation checks IDs, paths, formats, alpha, dimensions, pivots,
footprints, attachment points, masks, palettes, and references. Reports
include counts, missing assets, oversized textures, duplicates, memory
estimates, and atlas utilization.

Atlas packing should be deterministic where practical, use padding
against texture bleeding, and keep stable content IDs independent of
atlas coordinates.

Light pipeline (ART-010):
- World sprites render without a baked drop shadow.
- `stage.render_asset` writes each sprite's `shadow:` caster block from its staged bounds.
- `make normals` (`cmd/normalgen`, after Blender in `make art`) writes `<sprite>_n.png`. Masks, normal maps and `ui/` are skipped, and up-to-date maps are kept.
- `validate_assets.py` (`make art-check`) requires a normal map with the sprite's size and alpha, and a `shadow:` block, for every world sprite.

Housekeeping (PLAY-010, normative):
- Remove unused assets and resources in the same change that makes them unused: sprites with their `_n` maps and metadata, sounds, fonts, palette entries, vendor packs, generated files and the code that loads them. Never keep a fallback asset nobody draws. An alias may stay only while something still reads it.
- `make art-check` runs an orphan check: every generated sprite, sound and metadata file must be referenced by a renderer key table, a palette, a recipe or other metadata, or be listed with a reason in `assets/metadata/orphan_allow.txt`.
- Plant sprites never carry an outline (`outline_width=0`); the check fails if one does.

### 25.1 Audio assets (SND-002)

Sound follows the same rules as art: original, generated reproducibly,
with sidecar metadata.

-   **Effects** are made in code: `cmd/audiogen` renders the recipes in
    `cmd/audiogen/recipes.go` through `pkg/sfxsynth` (oscillators, noise,
    envelopes, filters, grains) to 48 kHz mono 16-bit WAV with three
    variants, byte-identical on every machine (`make audio`).
-   **Music** is composed in code: `cmd/audiogen/music.go` scores (theme,
    calm, tension, A minor, frontier colours) played by `pkg/musicsynth`
    instruments (plucked guitar, string pads and violins, horn, bass,
    frame drum, taiko, shaker) through a stereo reverb, mastered and
    encoded to Ogg Vorbis (`make music`). Byte-identical renders.
-   **Ambience beds** may be generated with ElevenLabs from the
    prompts in `assets/source/audio/prompts.json`, within the project's
    credit budget, in priority order (what the player hears longest
    first). Each take is reviewed by a person before import, and every
    take is recorded in `assets/source/audio/elevenlabs/ledger.json`.
-   **Animal voices** (call, attack, hurt, die per species, the 9 voice
    groups and the hive burst) are generated with ElevenLabs by
    `cmd/sfxgen` from `assets/source/audio/animal_prompts.json`, processed
    to mono 48 kHz Ogg with a ±4% pitched B variant, and listed only after
    a person approves them (SND-003). `animal.generic.*` stays the
    code-made last fallback.
-   **The sound list** `assets/metadata/audio/audio.json` records, per id,
    the bus, loop flag, volume, files and provenance (recipe and seed, or
    tool, prompt, credits, license and reviewer).
-   Loudness: music about -16 LUFS, ambience about -20 LUFS, effects
    peak-normalized to -1 dBFS and balanced by their list volume.
-   Budget: 20 MB of audio in the repository (`make audio-check`).
-   Never import recordings or music from other games, and never prompt a
    generator to imitate a named game's soundtrack.

## 26. Plugin Assets

Plugins use the same validation and namespaced IDs.

``` text
my-plugin/
├── manifest.yaml
├── defs/
├── patches/
├── assets/{world,ui,effects}/
└── localization/
```

Example:

``` text
author.mod:building/fusion_generator
```

Overrides, if supported, are explicit and load-order-resolved. Developer
diagnostics must expose resolved asset, source plugin, overridden asset,
and load-order reason. Plugins never mutate generated core files.

## 27. Animation and Zoom LOD

Animate only when it communicates operation, danger, weather, combat, or
state change. Animation is presentation-only.

At near zoom show full detail. At normal zoom optimize for canonical
readability. At far zoom omit tiny details, reduce effects/vegetation,
collapse pawn rendering, aggregate markers, and use icons/impostors when
beneficial.

For 1,000+ pawns use atlases, bounded texture counts, tinting, culling,
animation LOD, effect-density limits, and cached composited pawn
appearances when appearance has not changed.

### 27.1 Movement and body motion (normative)

ART-025 (2026-10-26, user decision): pawns are limbless and have no
animation frames; all pawn motion is renderer transforms (walk bob, sway,
lean, squash, rotation) applied to the standing sprites. Animal frames
are kept.

| Set | Frames | Notes |
|---|---|---|
| Pawn | none | standing sprite per layer and facing (S/E/N, west mirrored); the renderer moves the whole figure |
| Animal trot | `walk01`..`walk08` | per species; diagonal leg pairs, body bob, head nod |
| Animal rest | `rest` | lying pose, all animal sets |

Pawn attachments are `head` (hair, hats; the body centre for held items,
§15), `hold` (the punch trail's point; no hands are drawn) and `back` (a
rescued patient), in canvas px per facing.
Weapons and loads are drawn behind the body when facing north (aimed weapons when aiming north).

Clocks:

- `animT` (sim ticks plus the elapsed fraction of the current frame):
  movement, walk bob/sway, work motion, combat reactions. It freezes when
  paused and scales with game speed.
- `present` (real frames): breathing, wind, fire, particles, UI pulses.
- Tracers, recoil, hit flashes and floaters stay visible for at least
  `max(sim duration, 8 frames)` so they read at 3x.
- Draw interpolates between simulation updates; screenshots and capture
  sequences use no interpolation so they stay deterministic.

Path and gait:

- Positions follow the simulation's step timing exactly (step start tick
  and duration, diagonal steps longer than straight ones), so speed is
  constant on screen at every game speed.
- Corners are drawn as a quadratic curve between the midpoints of the
  incoming and outgoing steps, through the shared tile centre.
- Starting from rest and stopping (arrival or blocked) ease with
  smoothstep; the walk pose blends in and out over 0.3 tiles.
- The walk phase advances with distance travelled along the drawn path:
  one cycle per 1.6 tiles, so the bob never slides against the ground on
  diagonals (animal trot frames advance with the same stride).
- The pawn bob peaks once per footfall; sway (north/south facing) and
  forward lean (east/west) scale with speed; a short lean-back on
  deceleration.
- Facing uses the smoothed velocity with 20% axis hysteresis; a
  north/south reversal passes through a side facing for 4 frames.
- A blocked creature eases to a stop, faces the creature in its way and
  shows the idle pose.
- Getting into bed and being picked up ease over 12 frames.

### 27.2 Action animation (normative)

Pawn actions are transforms of the standing figure and its held item, not
pose sets (ART-025):

| Action | Motion |
|---|---|
| aim / fire | weapon aimed at the target from the body centre (§15); muzzle kick and a pull back along the aim on the shot, body recoil |
| melee (stab, thrust, swing, punch) | weapon aimed at the target; knife/spear jab along the aim, club a short arc about it; fists draw nothing; body lunge, then recover |
| chop, mine, hammer | wind back and drive in with a lean and squash on the strike; no tool |
| plant, pick, tend, bench, eat | lean and dip toward the job; nothing in hand |
| carry | the hauled item on the chest (§15); a rescued patient over the `back` |
| hit | flinch: short recoil away from the attacker |
| fall | knees-buckle dip, then rotation to lying, falling away from the last hit |

Animals (ART-011) keep frames: every species has its natural weapon's set
(`bite`, `claw`, `kick`, `gore`, `headbutt` or `peck`, `1..4`), and all
have `graze1..2` and `flinch1..2`.

Holding (ART-027): see §15. Only weapons are held, only on duty; no tool
is ever drawn. A flinch keeps what the pawn holds (the aimed weapon, the
load). West uses the mirrored east sprites, and the lit shader inverts the
normal X for mirrored draws.

Work progress: a pawn at a work job whose `PawnView.ActionProgress` is in
(0, 1) gets a bar 0.6 × 0.06 tile along the bottom edge of the work tile
(under the worker's feet, clear of its head), `uiAccent` on
`uiSliderTrack` with a dark edge, drawn in the overlay pass only at tile
≥ 24 px, one bar per work tile (the furthest progress). Eating, recreation
and research sessions show none.

Timing (with §27.1 clocks):

- The weapon is aimed for the whole attack cycle; the strike (muzzle
  kick, club arc or lunge peak, animal bite and kick frames) lands on the
  tick the attack's trace appears (`TraceView.Attacker`, age 0).
- A hit (`PawnView.HitTick`, `HitFromX/Y`) plays the pawn flinch and
  staggers 0.12 tile away from the attacker over 10 sim ticks; blood
  sprays away from the attacker. A pawn going down buckles, then rotates
  to lying, falling away from the last hit.
- Work loops run on `animT` with a per-action period; each strike throws
  that action's particles: chop wood chips, mine sparks and rock dust,
  build sawdust, sow soil, harvest leaves, cook steam, butcher blood, at
  the work spot (the worked tile's face towards the pawn).
  Each shot throws muzzle flash and smoke at `muzzle` and a shell casing.
- Every transform motion lasts at least 8 visible frames.

### 27.3 Environment motion (normative)

- Wind field: world wind (`WindX/WindY`) plus two travelling gust fronts
  (wavelength 9-14 tiles, moving downwind at 1.2x the wind) plus a small
  per-plant phase, so visible waves roll across grass and crops.
- Each plant leans with the local wind through a damped spring
  (stiffness k=38, damping ratio 0.35). Lean limits: grass 0.35 rad,
  crops 0.22, bushes 0.12, tree canopy 0.06 over a 0.02 trunk with the
  canopy lagging the trunk.
- Contact: a creature moving within 0.6 tile pushes a plant away from it
  (0.9 x its speed, capped); the plant springs back when it has passed.
  Trees shake briefly when brushed and, in fall, drop a leaf or two.
- Clocks: ambient waving on `present` (the world keeps breathing while
  paused); contact reads creature positions from the motion API.
- Budget: only visible plants; contact only within 1.5 tiles of a moving
  creature; no per-frame string building; weather particles in their own
  capped pool.

## 28. Deterministic Visual Variation

Visual randomness should be stable using keys such as world seed, tile
coordinate, EntityID, and variant category. Terrain variants, plant
rotation, rock variants, and decals must not shuffle between
frames/loads. Visual RNG must never feed back into authoritative
simulation.

Plant and decal variants are picked by `tileHash(x, y)`; seasonal looks
by the world's season; human corpses take the deceased's look from its
EntityID.

## 29. Tests and Visual QA

Automate metadata schema, duplicate IDs, missing sources/references,
pivot bounds, footprint validity, alpha, max dimensions, atlas
overlap/padding, plugin namespaces, and deterministic manifests.

Create fixed-seed screenshot scenes for:

``` text
art-room-furniture
pawn-composition-grid
weapons-grid
terrain-biomes
plant-growth-grid
wall-connectivity
power-buildings
combat-effects
ui-icon-sheet
plugin-assets
```

Generate category contact sheets. Large asset batches are not accepted
until reviewed together.

Create an in-game art test map containing every
terrain/material/furniture/workbench/plant stage plus representative
pawns, apparel, weapons, animals, weather, day/night, fire, and damage.

## 30. Art Agent Task Contract

Every task should specify:

``` text
Task ID
asset family
gameplay purpose
template
logical footprint
required variants
material regions
palette
attachments/pivots
allowed files
forbidden files
existing project references
acceptance tests
```

Acceptance includes normal-zoom readability, correct footprint/pivot,
originality, palette compliance, validator pass, contact-sheet review,
and in-game screenshot review.

## 31. Parallel Agent Ownership

Safe parallel work after shared contracts are frozen:

``` text
Agent A → furniture
Agent B → plants
Agent C → weapons
Agent D → UI icons
Agent E → terrain
Agent F → pawn hair
```

Do not let multiple agents independently redefine pawn proportions,
outlines, tile scale, palette, camera, pivots, or building perspective.

Use one Art Integration Agent to validate sources, regenerate
exports/contact sheets, inspect regression scenes, reject style drift,
resolve metadata conflicts, and update the catalog.

## 32. Implementation Order

### ART Wave 0 --- Freeze contracts

-   [x] tile reference scale (64 px/tile @1x, exported @2x; `artlib/contracts.py`)
-   [x] camera/perspective (orthographic top-down + oblique shear k=0.35; figures leaned 82°, animals 70° — ART-025, category rule in `contracts.py`)
-   [x] pivots (`origin_px` = footprint top-left; named `attachments`)
-   [x] outlines (post-pass chromatic contour, 6 px @2x, darken 0.18; ART-025)
-   [x] base/material palettes (`palettes/base.yaml`, `palettes/materials.yaml`)
-   [x] light direction (upper-left key, 3 toon bands, lower-right contact shadow)
-   [x] naming (`core:<family>/<name>`, lowercase snake_case, facing/variant suffixes)
-   [x] metadata schema (`assets/metadata/<family>/<name>.yaml`, version 1)
-   [x] source/generated rules (`assets/source/blender` → `make art` → `assets/generated`; never hand-patch)

### ART Wave 1 --- Tooling/templates

-   [x] validator (`assets/source/tools/validate_assets.py`, `make art-check`; Go port pending)
-   [x] raster export (`make art`, headless Blender; reproducible to ±1/255 GPU noise)
-   [-] atlas/manifest (NG-1 ART-014: `cmd/atlasgen` packs the pawn family, about 15k sprites with masks and normals, into six pages with `assets/metadata/atlas/pawn.yaml`; other families are not packed)
-   [x] contact sheets (`make art-preview`)
-   [x] screenshot test scene (`bin/game -capture out.png` / `make art-capture` renders a scripted colony in the real renderer; `compose_scene.py` for the full §34 set)
-   [-] building/item/terrain/icon/pawn templates (`template.blend` + per-family builders; animal family with trot frames, pawn without frames; 2×2/3×N and decal not yet)

### ART Wave 2 --- First playable

-   [x] soil/stone/water (domain terrain drawn in-game: soil/rich soil/gravel/stone/smooth stone variants with soft edge transitions, natural rock walls with baked corners tinted per stone (ART-015); NG-1 adds shallow and deep water with a ripple shader, mud and dirt tracks, ART-014)
-   [x] wall/door/floor (wood/stone/steel walls with 16 masks plus baked inner corners, 47 per material; wood and steel doors; wood and stone floors; all drawn in-game, ART-015)
-   [x] blueprint/frame (blueprint overlay on every frame, scaffold once wood is delivered)
-   [x] pawn base: original compositional pawns (ART-025: limbless round-headed figure, body/pants/shirt/vest/hair layers, S/E/N rendered with west mirrored, EntityID-stable tints; walk, work, lying/in-bed/carried motion are renderer transforms, no frames)
-   [x] first hair/apparel (3 hairstyles, pants/shirt/vest, runtime tint masks, in-game)
-   [-] basic resources/items (wood/food drawn in-game; steel/stone generated, no domain items yet)
-   [x] selection/designation markers (original selection brackets, harvest designation, stockpile zone in-game; cut/mine glyphs ready for when those orders exist)

### ART Wave 3 --- Survival

-   [-] bed/table/chair (1×1 cot drawn in-game for the current single-cell bed; 1×2 bed, table, chair generated in 4 facings for when the domain has footprints/furniture)
-   [x] food/crops/wild plants/trees (PLAY-010: restyled to §9.1/§9.2 without outlines; oak, birch, pine and jungle trees with visible trunks and _trunk/_canopy parts; grass tufts composed up to 9 per tile; 13 ground plants with young/mature/dormant looks incl. 7 new ones; PLAY-016 §9.3: 11 crops rebuilt as units in the §9.1 style (6 stages × 4 variants, no mound, muted pl_crop palette) composed several per tile over tilled furrows, cold/heat damage shown as tint, wilt, shrink and fade)
-   [ ] stove/butcher workstation
-   [x] medicine (NG-1: industrial medicine stacks and the hospital bed; ART-026: nanomed kit, servo/implant parts, valve and pump hearts, synth stomach, organ jars, vitals monitor with glow, care-level icons)
-   [x] blood/injury effects

### ART Wave 4 --- Production

-   [-] material variants (wood, five tinted stones and steel for walls; wood/stone floors; wood/steel doors; furniture wood only)
-   [ ] furniture families
-   [ ] production/research benches
-   [ ] storage
-   [x] generators/batteries/conduits/lamps (PLAY-021: `artlib/families/power.py`: a linked 16-mask conduit and a hidden-conduit overlay marking, power switch on/off, wood-fired, solar, wind (eight rotor frames) and geothermal generators, battery, heater, in-wall cooler; torch lamp and electric benches in furniture; lit parts as `<name>_glow` layers drawn only while powered; review sheet `generated/power_buildings.png`)

### ART Wave 5 --- Combat

-   [x] melee family (knife, spear, club; stab, thrust, swing and punch pose sets)
-   [-] pistol/rifle/shotgun/long-range weapon (rifle carried, aimed and firing, ART-027; no pistol, shotgun or long rifle)
-   [ ] armor/helmets
-   [-] projectiles/muzzle flash/impact/explosion (tracer, muzzle flash, smoke, casing, impact puff and blood; no explosions)
-   [ ] fire/smoke
-   [ ] turret/defenses

### ART Wave 6 --- Pawn variety

-   [-] body/head variants (NG-1 ART-014 / ART-025: six body builds as capsule widths, shared round head; the sexes read weakly at 1x)
-   [-] hair/facial hair library (NG-1: eight hair styles, three beards; beards are nearly invisible facing north)
-   [ ] apparel/armor library
-   [ ] tint validation
-   [x] sleeping/downed/dead presentation

### ART Wave 7 --- Animals/biomes

-   [x] domestic animals (ART-011 roster: 61 species incl. domestic; ART-013 adds a rest pose for all 64 animal sets)
-   [x] wildlife/predators (ART-011 roster; ART-013 rest pose)
-   [x] juvenile variants (scaled at runtime from the adult set, not separate art, per section 11)
-   [x] biome plants (ART-013: pine with snow, jungle tree, cactus with dry, shrub with dry, reeds; terrains sand, marsh, ice with edges; 12 biome icons)
-   [x] rock/resource variants (rough and smoothed rock and steel/silver/gold ore, 47 linked variants each, tinted per stone; stone chunk, stone blocks, steel, silver and gold items; ART-015)
-   [x] snow/weather (rain, storm and snow particles; snow cover, drifts, puddles and frost; wind-driven plant motion)

### ART Wave 8 --- World/factions

-   [x] faction emblems (`ui_icon/faction_<settlers|clans|drifters|freebooters|swarm|player>`)
-   [-] settlement/caravan/quest icons (settlement icons for neolithic and industrial factions; no caravan or quest icons)
-   [ ] trade icons
-   [-] world terrain (the globe is vertex-coloured from `assets/palettes/world.yaml` with no sprite terrain; storyteller busts, scenario, skill, passion and trait icons added alongside, ART-014)

### ART Wave 9 --- Endgame

-   [ ] advanced production/power
-   [ ] ship components
-   [ ] endgame effects
-   [ ] victory presentation

### ART Wave 10 --- Plugin/developer

-   [ ] missing/placeholder assets
-   [ ] plugin UI icons
-   [ ] developer/debug icons
-   [ ] debug overlays
-   [ ] example plugin asset pack

### ART Wave 11 --- Polish

-   [x] material/outline/palette consistency (ART-025: one outline contract, muted palettes)
-   [ ] pivot/scale audit
-   [-] normal/far zoom audit (ART-025 re-render reviewed at normal zoom; far zoom not audited)
-   [-] light/dark terrain contrast (muted palette brings wall tops, rich soil and mud close in value; walls rely on the outline and darker south face)
-   [ ] large-colony performance
-   [x] full contact-sheet review (ART-025)

## 33. Palette Deliverables

Before bulk production create:

``` text
assets/palettes/base.yaml
assets/palettes/materials.yaml
assets/palettes/ui.yaml
assets/palettes/factions.yaml
```

Use semantic names (`steel.base`, `wood.shadow`) rather than `gray1`.
Exact colors must come from original visual prototyping rather than
copying another game's palette.

ART-024 (2026-10-26, user decision: the look moves close to RimWorld's
readable chrome, values stay original): the UI chrome palette
(`assets/palettes/ui.yaml`, mirrored in `widgets.go`) is neutral
translucent charcoal with a warm gold accent. It replaces PLAY-018's
slate charcoal with a sea-glass accent.
- panels `#26292EE0`, neutral grey borders `#5A5D61`;
- buttons `#3C3F43`, hover `#4B4E52`, selected a warm `#5E5440`;
- text `#E8E4DA`, dim `#B8B4AA`, headings `#F0D9A0`;
- accent `#D9B45B`;
- semantic good/warning/danger, letter-kind and mood colours, power-net
  colours, and the warm learning-helper set are kept.

The colours come from CIELAB lightness steps and WCAG contrast, not from
sampling another game. Tests pin:
- neutral, translucent surfaces (chroma C* ≤ 6, panel alpha 0xB0–0xE6;
  `TestUIPanelsAreNeutralTranslucent`);
- text contrast ≥ 4.5:1 on panels over dark and light maps, on buttons
  and on tooltips (`TestUITextContrast`);
- no near-copies: dE ≥ 3 from a reference set of another game's UI
  colours (`TestUITokensAreNotCopies`).

UI badge icons sit on a neutral grey disc (`#303236`, edge `#0F1012`)
from the ART-0xx re-render on.

-   [-] `assets/palettes/ui.yaml` (menu/pause chrome tokens; Go adapter
    mirrors the hexes), `base.yaml` (outline/shadow/overlay tokens +
    runtime tint presets) and `materials.yaml` (3 toon bands per material)
    are in use by the Blender pipeline. `factions.yaml` has placeholder
    accents only.

## 34. Mandatory Style Prototype Gate

Before hundreds of assets are created, build one vertical-slice scene
containing:

-   [x] one pawn
-   [x] three hairstyles
-   [x] two apparel layers
-   [x] one rifle
-   [x] one tree and crop
-   [x] soil and stone
-   [x] wall, door, floor
-   [x] bed, table, chair
-   [x] workbench and lamp
-   [x] representative UI icons
-   [x] selection/designation overlays

Render everything together in one representative colony screenshot.

> Status: every element exists and is composed in
> `assets/generated/prototype_scene.png` (offline, renderer placement
> rules). Owner signed off on the visual identity 2026-10-05. The
> in-game subset (terrain, walls, blueprint/frame, cot, bushes, items,
> layered pawns, overlays) is drawn by the real renderer
> (`make art-capture`, ART-006). Bulk production may proceed.

**Do not begin bulk art production until this scene establishes a
coherent original visual identity.**

## 35. Asset Acceptance Checklist

Before accepting any asset:

-   [ ] identifiable at normal zoom
-   [ ] function communicated by silhouette
-   [ ] belongs beside existing assets
-   [ ] perspective/light/outline consistent
-   [ ] material matches shared material language
-   [ ] sufficient terrain contrast
-   [ ] no unnecessary detail
-   [ ] correct pivot and footprint
-   [ ] rotation works where required
-   [ ] tinting used instead of needless duplicates
-   [ ] original design
-   [ ] no proprietary source art used
-   [ ] automated validation passes
-   [ ] reviewed in-game, not only in editor

## 36. Final Directive

The goal is not to reproduce another game's asset library. It is to
reproduce the **clarity of thought** behind excellent colony-simulator
presentation:

``` text
simple enough to read instantly
+ distinct enough to recognize
+ consistent enough to feel authored by one team
+ modular enough to generate enormous variety
+ efficient enough to render enormous colonies
```

Canonical production loop:

``` text
GAMEPLAY REQUIREMENT
→ ORIGINAL BRIEF
→ SHARED TEMPLATE + PALETTE
→ CONCEPT / SILHOUETTE
→ EDITABLE SOURCE
→ NORMALIZATION
→ METADATA VALIDATION
→ AUTOMATED EXPORT
→ ATLAS
→ IN-GAME TEST
→ CONTACT-SHEET / VISUAL REVIEW
→ ACCEPT
```

If a contributor can add a new bed, weapon, plant, hairstyle, animal,
building, icon, or plugin asset without asking how scale, pivots,
palette, lighting, naming, metadata, or export work, this pipeline is
doing its job.

## 37. Social & host systems asset list (ENG-003)

Normative key list for the social and host-system feature set
(`.cursor/SOCIAL_SYSTEM_CONTRACT.md`). None of these exist yet: until each
is generated the renderer draws its vector stand-in (`social_art.go`,
`drawIconFallback`) and the Architect keeps the tool in
`archToolsPendingArt`. When an asset lands, move its tool to `archTools`
(TestPendingArtToolsLackArt fails until you do), add its keys to the
renderer's sprite lookups, and only then let `TestGeneratedArtCoversRenderer`
cover it. Every family follows §12-§17 (pivots, palette, outline rules) and
§24 metadata; all designs original.

- **Furniture / markings** (rotatable ones in four facings):
  - `furniture/brewing_bench_<facing>` (2x1 bench, barrels and a mash tub),
    `furniture/drug_table_<facing>` (2x1, mortar, drying leaves).
  - `overlay/marriage_spot_<facing>` (2x1 floor marking, ribbon arch
    feel), `overlay/party_spot` (1x1 floor marking, lantern ring).
  - Icons `ui_icon/brewing_bench`, `ui_icon/drug_table`,
    `ui_icon/marriage_spot`, `ui_icon/party_spot`.
- **Crops (never an outline or contour, like every plant):**
  `plant/brewgrain_<young|mature|harvestable>`,
  `plant/ashleaf_<young|mature|harvestable>`, their item sprites
  `item/brewgrain`, `item/ashleaf` and icons `ui_icon/brewgrain`,
  `ui_icon/ashleaf`. Adding them to `plant.Crops()` is what puts them in
  the growing-zone menu.
- **Items:** `item/beer`, `item/ashleaf_tea`, `item/spark_tablet`,
  `item/peg_leg`, `item/prosthetic_leg`, `item/prosthetic_arm`,
  `item/wooden_hand`, `item/denture`, `item/hearing_trumpet`,
  `item/prosthetic_foot`, `item/glass_eye`, `item/wooden_nose` (the item
  names have spaces: map them to these keys in the renderer's
  `itemSprite` when the art lands).
- **Social interaction icons** (log lines, speech bubbles):
  `ui_icon/social_chitchat`, `social_deep_talk`, `social_slight`,
  `social_insult`, `social_kind_words`, `social_romance`, `social_rebuff`,
  `social_proposal`, `social_breakup`, `social_fight`, `social_lovin`.
- **Order and tab icons:** `ui_icon/capture`, `ui_icon/arrest`,
  `ui_icon/prisoner`, `ui_icon/caravan`, `ui_icon/history`,
  `ui_icon/surgery`.
- **Overlays:** a social-fight lightning indicator over both brawlers
  (`overlay/fight_lightning`), and a face-scar portrait overlay for
  disfigured colonists (`pawn/face_scar_<facing>`, drawn over the head on
  portraits and in the world at portrait LOD).
