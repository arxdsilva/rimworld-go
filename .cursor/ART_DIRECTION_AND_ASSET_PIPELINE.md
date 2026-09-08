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
not invent independent camera/light setups.

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

-   [ ] building 1×1
-   [ ] building 1×2
-   [ ] building 2×2
-   [ ] building 3×1 / 3×2
-   [ ] furniture
-   [ ] weapon
-   [ ] pawn body/head/hair/apparel
-   [ ] animal
-   [ ] plant
-   [ ] item
-   [ ] UI icon
-   [ ] terrain
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

## 28. Deterministic Visual Variation

Visual randomness should be stable using keys such as world seed, tile
coordinate, EntityID, and variant category. Terrain variants, plant
rotation, rock variants, and decals must not shuffle between
frames/loads. Visual RNG must never feed back into authoritative
simulation.

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

-   [ ] tile reference scale
-   [ ] camera/perspective
-   [ ] pivots
-   [ ] outlines
-   [ ] base/material palettes
-   [ ] light direction
-   [ ] naming
-   [ ] metadata schema
-   [ ] source/generated rules

### ART Wave 1 --- Tooling/templates

-   [ ] validator
-   [ ] raster export
-   [ ] atlas/manifest
-   [ ] contact sheets
-   [ ] screenshot test scene
-   [ ] building/item/terrain/icon/pawn templates

### ART Wave 2 --- First playable

-   [-] soil/stone/water (vendor ground/rock tiles drawn presentation-only; no domain terrain types)
-   [ ] wall/door/floor
-   [ ] blueprint/frame
-   [-] pawn base (vendor CC0 side-view full-body idle/walk in the ebiten adapter; original top-down compositional pawns not yet)
-   [ ] first hair/apparel
-   [ ] basic resources/items
-   [-] selection/designation markers (vendor crosshair on selected pawn; no designation overlays)

### ART Wave 3 --- Survival

-   [ ] bed/table/chair
-   [ ] food/crops/wild plants/trees
-   [ ] stove/butcher workstation
-   [ ] medicine
-   [ ] blood/injury effects

### ART Wave 4 --- Production

-   [ ] material variants
-   [ ] furniture families
-   [ ] production/research benches
-   [ ] storage
-   [ ] generators/batteries/conduits/lamps

### ART Wave 5 --- Combat

-   [ ] melee family
-   [ ] pistol/rifle/shotgun/long-range weapon
-   [ ] armor/helmets
-   [ ] projectiles/muzzle flash/impact/explosion
-   [ ] fire/smoke
-   [ ] turret/defenses

### ART Wave 6 --- Pawn variety

-   [ ] body/head variants
-   [ ] hair/facial hair library
-   [ ] apparel/armor library
-   [ ] tint validation
-   [ ] sleeping/downed/dead presentation

### ART Wave 7 --- Animals/biomes

-   [ ] domestic animals
-   [ ] wildlife/predators
-   [ ] juvenile variants
-   [ ] biome plants
-   [ ] rock/resource variants
-   [ ] snow/weather

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

## 34. Mandatory Style Prototype Gate

Before hundreds of assets are created, build one vertical-slice scene
containing:

-   [ ] one pawn
-   [ ] three hairstyles
-   [ ] two apparel layers
-   [ ] one rifle
-   [ ] one tree and crop
-   [ ] soil and stone
-   [ ] wall, door, floor
-   [ ] bed, table, chair
-   [ ] workbench and lamp
-   [ ] representative UI icons
-   [ ] selection/designation overlays

Render everything together in one representative colony screenshot.

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
