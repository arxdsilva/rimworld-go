# rimworld-go --- Art Direction & Asset Pipeline

> **Purpose:** Canonical visual-production specification for human and
> AI agents creating `rimworld-go` art in parallel.
>
> **Target:** A highly readable, minimalist, top-down sci-fi
> colony-simulator aesthetic with strong silhouettes, restrained detail,
> composited characters, consistent materials, and excellent readability
> at gameplay zoom.
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

The visual identity should be original: slightly geometric industrial
construction, clean modern sci-fi forms, distinctive pawn proportions,
consistent shadows, stronger material differentiation, original
iconography, and a UI designed to remain readable at very large colony
sizes.

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

Prefer neutral source art plus explicit runtime tint masks for clothing,
hair, faction accents, fabrics, and paintable objects.

Material language: - **wood:** warm, broad minimal grain; - **metal:**
cooler, planar highlights, harder edges; - **stone:** muted, broad
irregular variation; - **cloth:** soft color blocks/minimal folds; -
**electronics:** geometric housing plus sparse emissive indicators.

Do not encode essential state by hue alone.

## 8. Terrain

Initial terrain families: soil, rich soil, stony soil, sand, gravel,
mud/marsh, shallow/deep water, ice, rough stone, constructed floors, and
snow.

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

Vegetation set (family `plant`):

- Trees: `tree_{oak,birch,pine}_00..02`. Oak and birch also have
  `_autumn` (amber/red/gold canopy) and `_bare` (winter branches with a
  light snow cap); pine is evergreen. Metadata `sway: {trunk, canopy}`.
- Bushes: `bush_00..03`; berry bushes `berry_bush_00..02` with
  `_harvestable` (berries visible).
- Grass: `grass_00..07` (tufts, some with small flowers or clover) and
  `grass_tall_00..03` (meadow grass on rich soil).
- Crops: `<species>_<stage>_v0..v2` with stages `sown`, `sprout`,
  `young`, `mature`, `harvestable` (ripe produce visible) and `wilted`
  (frost or stalled growth). Variants change leaf count, angle and
  height, not just colour.
- Cozy rule: warm, slightly desaturated greens; soft value steps; no
  pure saturated primaries.

## 10. Pawn Visual System

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

Create an original silhouette: compact/geometric body, large readable
head, simplified limbs, minimal world-scale facial detail, and
apparel-driven identity.

Support reusable body types, multiple head shapes, tintable skin/hair,
separate facial hair, layered apparel, headgear, and separate rotatable
equipment. All layers share attachment conventions.

Renderer handles orientation, downed/sleeping posture, carried objects,
aiming, fire/status, damage, and selection where possible.

## 11. Animals

Species must be identifiable by silhouette at normal zoom: body length,
head, ears/horns, tail, and proportions matter more than detail. Prefer
a small direction set (`north/east/south/west`) and mirror only where
asymmetry permits. Life stages may use scale or dedicated juvenile
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
- **Sizes** are body lengths in tiles: XS 0.35, S 0.6, M 0.95, L 1.4, XL 2.0, with `+`/`-` meaning ±15%. Large animals overhang their tile.
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

- **Per species and facing (south/east/north/west):**

  | Sprite | Frames |
  |---|---|
  | standing | 1 |
  | trot | `walk01..08` |
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

Held gear (family `gear`: `weapon/*`, `tool/*`) is one east-pointing
sprite per item, rotated by the renderer, with image-px attachments:
`grip` (main hand), `support` (off hand, so two-handed items sit in both
hands), `muzzle` (firearms), `tip` (blades: knife, spear) and `head`
(tools). Melee set: `weapon/knife`, `weapon/spear`, `weapon/club`;
firearms: `weapon/rifle_basic` (+ `stock`) and `weapon/rifle_slung` (worn
on the back). Tools: axe, pickaxe, hammer, trowel, sickle, bandage, bowl,
spoon.

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
- **Broom:** the held tool `tool/broom`, a stick with a straw head. The cleaning pose reuses the low `_pick` work frames, with dust particles.
- **Home overlay:** a translucent blue tint (#4a7fd4, alpha 0.18) with a 1 px edge on its outline.
  - Shown while a Home tool is held, or when toggled from the overlay row next to Roof.
- **Icons** (`ui_icon/`, underscores): `cloth`, `straw`, `straw_carpet`, `cloth_carpet`, `leather_carpet`, `wood_carpet`, `carpet` (the Architect tool), `home`, `home_clear`.

## 17. UI Art

Use an original high-information-density interface: neutral/dark
surfaces, restrained borders, clear typography, modest texture, strong
hover/selected states. Do not duplicate RimWorld's exact panel art,
iconography, typography, or visual layouts.

Create coherent icon families for architect categories, orders, work,
speed, pawn status, health, needs, research, factions, trade, overlays,
alerts, plugins, and developer tools. Icons need consistent stroke/fill
language, transparent backgrounds, semantic IDs, and small-size
readability.

Pawn portraits should initially derive from the compositional pawn
system.

## 18. Factions and Overlays

Factions are data-driven through emblem, accent palette, optional
apparel palette, and optional architecture palette. Prefer shared world
art plus faction identity layers over entirely unique asset libraries.

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

Frames (rendered in Blender, one set per facing):

| Set | Frames | Layers | Notes |
|---|---|---|---|
| Pawn walk | `walk01`..`walk12` | body, pants, shirt, vest, parka, duster (+ masks) | one full gait cycle: 01 right heel contact, 02 loading, 03 mid-stance, 04 terminal stance, 05 pre-swing, 06 swing; 07-12 mirror for the left foot |
| Pawn work | `work1`..`work6` | same layers | 1-2 wind-up, 3-4 swing, 5 strike, 6 recover; smoothstep eased |
| Animal trot | `walk01`..`walk08` | per species | diagonal leg pairs, body bob, head nod |

Every pawn body frame carries `hand` and `head` attachments (canvas px).
Hair and hats are drawn at the body's `head` offset for that frame; held
tools and weapons at its `hand` point.

Clocks:

- `animT` (sim ticks plus the elapsed fraction of the current frame):
  movement, walk cycle, work swings, combat reactions. It freezes when
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
- The walk cycle advances with distance travelled along the drawn path:
  one 12-frame cycle per 1.6 tiles, so feet don't slide on diagonals.
- Bob peaks once per footfall (frames 01 and 07); sway (north/south
  facing) and forward lean (east/west) scale with speed; a short
  lean-back on deceleration.
- Facing uses the smoothed velocity with 20% axis hysteresis; a
  north/south reversal passes through a side facing for 4 frames.
- A blocked creature eases to a stop, faces the creature in its way and
  shows the idle pose.
- Getting into bed and being picked up ease over 12 frames.

### 27.2 Action animation (normative)

Pose sets (Blender, every facing; body, pants, shirt, vest, parka and
duster layers; hair and hats follow `head`):

| Set | Frames | Pose |
|---|---|---|
| `ready` | 1 | rifle at low ready, both hands, elbows bent |
| `aim` | 1 | rifle shouldered, cheek on stock, support arm out, slight lean in |
| `fire1..3` | 3 | recoil: shoulder driven back, muzzle rise, recover |
| `stab1..6` | 6 | knife: guard, chamber, step-thrust, full extension, withdraw, guard |
| `thrust1..6` | 6 | spear, two hands: rear hand at the butt, lunge, full reach, recover |
| `swing1..6` | 6 | club, one hand: cock over the shoulder, twist, diagonal strike, follow-through |
| `punch1..4` | 4 | guard, jab, cross, guard |
| `chop1..6` | 6 | axe, two hands: raise over the far shoulder, diagonal drive, rebound |
| `mine1..6` | 6 | pickaxe, two hands: high overhead, vertical drive, knees bent, pry |
| `hammer1..4` | 4 | hammer in the right hand, left hand steadying, short arcs |
| `kneel`, `plant1..4` | 5 | one knee down; dig, place the seed, press the soil |
| `pick1..4` | 4 | crouch, both hands into the plant, pull, drop into the bag |
| `bench1..4` | 4 | hands at waist height working a bench (craft, cook, tailor, research) |
| `tend1..4` | 4 | kneeling, both hands bandaging low in front |
| `eat1..4` | 4 | bowl in the left hand at the chest, spoon to the mouth and back |
| `carry`, `carry_walk01..12` | 13 | forearms forward and level holding a load; legs as in the walk |
| `flinch1..3` | 3 | hit: torso snaps back, arms up, head turns away |
| `fall1..4` | 4 | knees buckle, torso folds, hands out; the renderer finishes lying down |
| `offer1..2` | 2 | crouched, hand held out low (taming) |

Animals (ART-011): every species has its natural weapon's set (`bite`, `claw`, `kick`, `gore`, `headbutt` or `peck`, `1..4`), and all have `graze1..2` and
`flinch1..2`.

Every pawn body frame carries `hand` (right), `hand_l` (left) and `head`.

Holding: a two-handed item puts its `grip` on `hand` and turns so its
`support` lies on the line to `hand_l`; one-handed items hang from `hand`
along the forearm. Off duty (not drafted, not attacking) the rifle is
slung on the back, the knife rides on the belt (not drawn), the spear is
carried upright in the right hand and the club stays in hand. Working
swaps the weapon for the job's tool.

Timing (with §27.1 clocks):

- Aiming blends `ready` to `aim` over the first 60% of the attack cycle
  (`PawnView.AttackCycle`); strike frames (`fire1`, the stab, thrust,
  swing and punch peaks, animal bite and kick) land on the tick the
  attack's trace appears (`TraceView.Attacker`, age 0).
- A hit (`PawnView.HitTick`, `HitFromX/Y`) plays `flinch1..3` and
  staggers 0.12 tile away from the attacker over 10 sim ticks; blood
  sprays away from the attacker. A pawn going down plays `fall1..4`, then
  lies down falling away from the last hit.
- Work loops run on `animT` with a per-action period; each strike throws
  that action's particles: chop wood chips, mine sparks and rock dust,
  build sawdust, sow soil, harvest leaves, cook steam, butcher blood.
  Each shot throws muzzle flash and smoke at `muzzle` and a shell casing.
- Everything keeps the §27.1 minimum of 8 visible frames.

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
-   [x] camera/perspective (orthographic top-down + oblique shear k=0.5; figures leaned 55° — category rule in `contracts.py`)
-   [x] pivots (`origin_px` = footprint top-left; named `attachments`)
-   [x] outlines (post-pass chromatic contour, 2 px @1x)
-   [x] base/material palettes (`palettes/base.yaml`, `palettes/materials.yaml`)
-   [x] light direction (upper-left key, 3 toon bands, lower-right contact shadow)
-   [x] naming (`core:<family>/<name>`, lowercase snake_case, facing/variant suffixes)
-   [x] metadata schema (`assets/metadata/<family>/<name>.yaml`, version 1)
-   [x] source/generated rules (`assets/source/blender` → `make art` → `assets/generated`; never hand-patch)

### ART Wave 1 --- Tooling/templates

-   [x] validator (`assets/source/tools/validate_assets.py`, `make art-check`; Go port pending)
-   [x] raster export (`make art`, headless Blender; reproducible to ±1/255 GPU noise)
-   [ ] atlas/manifest
-   [x] contact sheets (`make art-preview`)
-   [x] screenshot test scene (`bin/game -capture out.png` / `make art-capture` renders a scripted colony in the real renderer; `compose_scene.py` for the full §34 set)
-   [-] building/item/terrain/icon/pawn templates (`template.blend` + per-family builders; animal family with trot frames; 2×2/3×N and decal not yet)

### ART Wave 2 --- First playable

-   [-] soil/stone/water (domain terrain drawn in-game: soil/rich soil/gravel/stone variants with soft edge transitions, 16-mask natural rock walls; water missing)
-   [-] wall/door/floor (16-mask wood wall autotile drawn in-game; doors and wood floor generated but the domain has no doors/floors yet)
-   [x] blueprint/frame (blueprint overlay on every frame, scaffold once wood is delivered)
-   [x] pawn base: original compositional pawns (body/pants/shirt/vest/hair layers, 4 facings, EntityID-stable tints) with an 8-frame Blender walk cycle synced to tile progress, idle breathing, work swings, lying/in-bed/carried poses; vendor pack kept as fallback
-   [x] first hair/apparel (3 hairstyles, pants/shirt/vest, runtime tint masks, in-game)
-   [-] basic resources/items (wood/food drawn in-game; steel/stone generated, no domain items yet)
-   [x] selection/designation markers (original selection brackets, harvest designation, stockpile zone in-game; cut/mine glyphs ready for when those orders exist)

### ART Wave 3 --- Survival

-   [-] bed/table/chair (1×1 cot drawn in-game for the current single-cell bed; 1×2 bed, table, chair generated in 4 facings for when the domain has footprints/furniture)
-   [x] food/crops/wild plants/trees (oak, birch and pine in 3 variants with autumn and bare winter looks; bushes and berry bushes; 12 grass tufts; 8 crops × 6 stages × 3 variants)
-   [ ] stove/butcher workstation
-   [ ] medicine
-   [x] blood/injury effects

### ART Wave 4 --- Production

-   [-] material variants (wood/stone walls and floors; doors/furniture wood only)
-   [ ] furniture families
-   [ ] production/research benches
-   [ ] storage
-   [ ] generators/batteries/conduits/lamps

### ART Wave 5 --- Combat

-   [x] melee family (knife, spear, club; stab, thrust, swing and punch pose sets)
-   [-] pistol/rifle/shotgun/long-range weapon (rifle with ready/aim/fire poses and a slung variant; no pistol, shotgun or long rifle)
-   [ ] armor/helmets
-   [-] projectiles/muzzle flash/impact/explosion (tracer, muzzle flash, smoke, casing, impact puff and blood; no explosions)
-   [ ] fire/smoke
-   [ ] turret/defenses

### ART Wave 6 --- Pawn variety

-   [ ] body/head variants
-   [ ] hair/facial hair library
-   [ ] apparel/armor library
-   [ ] tint validation
-   [x] sleeping/downed/dead presentation

### ART Wave 7 --- Animals/biomes

-   [ ] domestic animals
-   [ ] wildlife/predators
-   [ ] juvenile variants
-   [ ] biome plants
-   [-] rock/resource variants (natural rock walls 16-mask, rock chunks, stone blocks item; no ore)
-   [x] snow/weather (rain, storm and snow particles; snow cover, drifts, puddles and frost; wind-driven plant motion)

### ART Wave 8 --- World/factions

-   [ ] faction emblems
-   [ ] settlement/caravan/quest icons
-   [ ] trade icons
-   [ ] world terrain

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

-   [ ] material/outline/palette consistency
-   [ ] pivot/scale audit
-   [ ] normal/far zoom audit
-   [ ] light/dark terrain contrast
-   [ ] large-colony performance
-   [ ] full contact-sheet review

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
