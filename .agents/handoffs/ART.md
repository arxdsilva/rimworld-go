# ART handoffs

Families: ART, SHADOW, CURSOR, VIEW. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## ART-000-agent-pipeline-integration
Art spec wired into the agent pipeline · 2026-09-08 · a9ccd60f8
- Changed: docs and skills only. `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` is recognised from `AGENTS.md` and the (since removed) Cursor always-on rule, and loaded by section through skill `rimworld-go-art-assets`.
- Decisions: the spec stays canonical, the skill routes sections and DoD; full spec load is task-gated, never default context for headless work. ART wave checkboxes use the same `[x]`/`[-]`/`[!]` rule as the other checklists. Pixels, texture handles, camera and animation stay in presentation adapters.
- Open: freeze ART Wave 0 (scale, camera, pivots, palettes, metadata) before bulk production (done in ART-005); `assets/AGENTS.md` and `assettool` did not exist yet.

## ART-001-vendor-pack-renderer
CC0 vendor character pack in the renderer · 2026-09-08 · c118d1134
- Changed: the ebiten adapter drew the CC0 Rgsdev pack (idle/walk pawns, ground, rocks, crosshair); `LoadPack`/`LoadDefaultPack`, exported `TileSize` (64), `cmd/game -assets`. No domain change.
- Decisions: vendor PNGs immutable; sprite chosen by `EntityID` modulo colonist count; walk/idle and flip inferred from last snapshot position; frame index from sim tick, never written back. Side-view pack did not satisfy the top-down art bible.
- Limits: 2048x2048 sources made `LoadPack` slow (downscaled to 128/64 px).
- Superseded (PLAY-010, 2026-10-23): the pack (203 MB), `pack.go`, the `atlas`, `AppConfig.LoadPack` and `-assets` were deleted. Without generated art the renderer draws plain coloured terrain and pawn squares (`TestRendererWithoutArtDrawsFallback`). AGENTS.md now requires deleting unused assets in the change that leaves them unused.

## ART-002-presentation-animation-clock
Presentation animation clock · 2026-09-08 · a6bed3ced
- Changed: simulation no longer steps 1:1 with render frames; normal/fast/very-fast wait 15/6/2 Update frames per tick (~4/10/30 TPS at 60 Hz). Idle/walk frames run on a presentation frame counter; walk holds 20 frames (`walkSustainFrames`) after a tile hop. Speed keys (`Space`, `1`/`2`/`3`) unchanged.
- Decisions: the clock lives only in the ebiten adapter; speed changes the rate of processed ticks, never rules. Headless `Step(n)` unchanged. No determinism or save impact.
- UI: selected-pawn portrait plays idle on the presentation clock.
- Open: interpolate draw position (done in ART-003); multi-tick tile movement in the domain.

## ART-003-pawn-interpolation
Pawn interpolation between ticks · 2026-09-08 · dc6412f4c
- Changed: pawn sprites and their selection marker lerp from the previous tile to the snapshot cell over the wait between ticks; pause freezes mid-stride; click-select prefers the interpolated cell, then the logical tile.
- Decisions: adapter-only, `Snapshot()` stays integer tiles. After a hop the origin is the previous logical cell; a later tick on the same cell settles the origin so idle does not snap backward. Walk plays while in transit.
- Limits: linear lerp, no easing; very-fast covers a tile every 2 frames.
- Open: multi-tick tile movement in the domain; optional ease-in/out (largely done in ART-007).

## ART-004-animation-pass
Animation pass: pawns, animals, combat, work, world · 2026-10-06 · a11321a63
- Changed: presentation-only animation, visual randomness is `mix64` of stable keys.
  - Pawns: 8-frame Blender walk cycle in 4 facings (one stride per tile, feet planted), hand attachment follows the arm; idle breathing with per-pawn phase; work swings with per-job particles (wood chips, stone dust, leaves, soil, shavings, blood, steam plus burner glow); rifle recoil, melee lunge, knockback, red hit flash; falling when downed/killed (dead grey, downed breathe); in-bed pose with blanket overlay; carried across the rescuer's shoulders.
  - Work arm frames `work1..work3` for body and shirt; tool sprites `tool/hammer|axe|pickaxe`. Tools by job: hammer construct/deconstruct/workbench, axe cut, pickaxe mine, bare-hand chop butcher; low reach for harvest/sow/stove/tend. The tool replaces the held weapon; swing snaps to strike at 48% of the swing and particles land there.
  - Animals: trot frames (dustbuck, ridgewolf), bounce when fleeing/attacking, bite lunges, step dust. World: wind sway, ease-out-back item pop, pulsing selection and alerts. Combat: blood spray and muzzle smoke once per trace.
- Decisions: simulation fixes found while animating: fully rested sleepers stay in bed during scheduled sleep (they used to re-start sleep every tick); doctors stand beside the patient/bed; `PawnView.Acting/ActX/ActY` exposes hands-on work.
- Dev tooling: `-capture-frames N` (lock-stepped), `-capture-ui follow`, `-scenario rescue|beds`, `Game.Injure`, `Game.BuildNow`.
- Limits: no animal body-part animation; particles drawn under the night veil; no animation LOD at far zoom.

## ART-004-main-menu-colonist
Main menu colonist bust · 2026-09-15 · 9ff99e3b6
- Changed: Main Menu (§2) shows an original cel-shaded colonist bust (alpha PNG `assets/source/raster/ui/main_menu_colonist.png`) left over the warm backdrop; title and actions on a right rail (buttons 300 px wide, `x = sw - 40 - 300`). Background item marked `[-]`.
- Contract: `AppConfig.LoadSplash` optional; `NewApp` locates and decodes the PNG; a missing/failed splash still shows the menu.
- Decisions: clean-room, generated from the written `ai-bg-rimworld` brief, not from a RimWorld screenshot or the ASCII grid. Splash is UI raster (full decode, no 128 px downscale). Menu presentation starts no simulation. Boot screen unchanged.
- Limits: still image, cutout torso, no metadata sidecar.
- Open: optional idle animation later, still without a live sim.

## ART-005-blender-pipeline-style-prototype
Blender pipeline and style prototype (ART Wave 0 frozen) · 2026-10-05 · 12916c5f2
- Changed: original top-down art rendered from code in Blender (`make art` headless, Blender MCP for iteration). Wave 0 contracts frozen in `assets/source/blender/artlib/contracts.py`; palettes `assets/palettes/{base,materials,factions}.yaml`; full §34 slice: terrain, rocks, autotiled walls, doors, floor, furniture, plants, items, 4-facing layered pawn with tint masks, rifle, overlays, 12 UI icons (107 sprites + 12 icons, validator clean). New Makefile targets `art`, `art-check`, `art-preview`; Blender MCP add-on copy in `tools/blender-mcp/`.
- Contracts: sprite metadata v1 (`id`, `image`, `scale`, `canvas`, `footprint`, `origin_px`, `attachments`, `tint{group,mask}`, family extras); wall autotile mask N=1 E=2 S=4 W=8; pawn layer order; @2x export. Ids namespaced `core:` for plugin packs.
- Decisions: orthographic top-down with oblique shear (height z drawn 0.5*z tiles up), footprints grid-aligned. Pawns/animals lean 55 deg toward the viewer about their feet (§5); everything else plain shear. Light upper-left; exactly three palette bands per material; outline is a 2 px @1x chromatic post-pass, never pure black; contact shadow lower-right. Apparel/hair render with the body as holdout. Terrain tiles share border lattices. Clean-room: geometry and colours authored in code, no reference images, libraries or AI 3D.
- Decisions: owner signed off the §34 visual identity on 2026-10-05. Renderer variant choice must be coordinate-hashed, never sim RNG.
- Limits: two headless runs differ by at most 1/255 (GPU noise). No terrain transitions/water/snow, only wood walls/doors/floor, no walk/downed/sleep presentation yet (later ART tasks).

## ART-006-original-art-in-renderer
Generated art in the ebiten renderer · 2026-10-05 · 8c2d59745
- Changed: renderer draws the ART-005 art: soil terrain variants with grass decals, 16-mask autotiled walls, blueprint/scaffold frames, 1x1 cot for beds, berry bushes (harvestable or not), items, overlays, layered colonists (body, pants, shirt, optional vest, 3 hairstyles) with walk bob and EntityID-stable tints. HUD portraits use the same layers. Grid lines hidden when art is loaded.
- Contract: `AppConfig` gains `LoadArt` and `Capture`; `App.CaptureErr()`; `cmd/game -capture`, `-capture-ticks`, `-capture-zoom`; `make art-capture`. No sim, command or save change.
- Decisions: look/variant is hash(EntityID / tile), not RNG. Domain beds are single-cell so the 1x1 cot is used until building footprints exist. Only soil is drawn because the domain had no terrain types. Capture uses only Enqueue/Step, deterministic per seed. `TestGeneratedArtCoversRenderer` checks every key the draw path uses.
- Limits: tint bake per (sprite, tint) on first draw (atlas + shader tint suggested for large colonies); no carried-item presentation or far-zoom LOD.

## ART-007-smooth-animation
Smooth movement and body animation (ART §27.1) · 2026-10-19 · 3176ecff3
- Changed: pawn walk is a 12-pose gait (`walk01..walk12`) for body, pants, shirt, vest, parka, duster; work is a 6-pose smoothstep swing (`work1..work6`); animals trot in 8 poses; every body frame has `head` beside `hand` (pawn PNGs 304 to 792, animal 40 to 72). Validator checks sets and head anchor.
- Motion (`motion.go`): `pawnMotion`/`animalMotion` build `moveState` from `StepTick/StepTicks/NextX,NextY/HasNext/BlockedBy` (SIM-014): exact sim step timing, quadratic curve through tile centres, smoothstep start/stop (first/last step peaks at 1.33x cruise so it still lasts its sim duration), facing from a two-step window with 20% hysteresis, N/S reversals via a side facing for 4 frames.
- Decisions: `animT()` = sim tick plus elapsed fraction of the update (wall clock, adapter-only, 0 in captures), one update behind so only happened steps are drawn; `present` drives ambient effects. Walk frame = stride/1.6 tiles x 12 (8); blend over 0.3 tiles; bob on footfalls; hair and hats follow the `head` delta; footstep dust every 0.8 tiles; bed entry and carry lift ease over 12 frames; blocked creatures face the blocker; fleeing animals bob 1.6x.
- Camera (`camera_smooth.go`): pan accelerates and coasts (~200 ms), zoom eases (~150 ms) at the cursor; captures set it directly.
- UI: inspector shows "waiting for <name>" while `BlockedBy` is set. Dev: `-capture-speed`, `-scenario corridor|crowd`, `-capture-ui corridor|crowd`.
- Limits/Open: lean-back on deceleration is a heuristic; bed exit and carry release not eased; no far-zoom animation LOD; no aiming pose; mouse edge/drag pan not implemented.

## ART-008-action-animations
Action animations: holding, combat, hits, work (ART §27.2) · 2026-10-19 · dc16a8353
- Changed: pawn rig gained elbows, wrists, shoulder twist, torso lean, head yaw/pitch, two-bone IK. Pose sets in all facings (pawn PNGs 792 to 4232, 28 MB): combat ready/aim/fire1-3/stab/thrust/swing/punch; work chop/mine/hammer/kneel+plant/pick/bench/tend/eat; carry, carry_walk01-12, flinch, fall, offer. Every body frame has `hand`, `hand_l`, `head`.
- Gear: new Blender family `gear` (split from `pawn.py`): `weapon/rifle_basic`, `rifle_slung`, knife, spear, club; tools axe, pickaxe, hammer, trowel, sickle, bandage, bowl, spoon; attachments `grip`, `support`, `muzzle`/`stock`, `tip`, `head`; ground items `item/knife`, `item/spear`. Animals: dustbuck `kick1-4`, ridgewolf `bite1-4`, `graze1-2`, `flinch1-2`.
- Renderer: `anim.go` split into `combat_anim.go`, `work_anim.go`, `held.go`; `standingPose` tries hit reaction, combat, carry, work. Two-handed items put `grip` on `hand` with `support` on the line to `hand_l`; off duty the rifle is slung, knife on belt (not drawn), spear upright, club in hand; off-hand slot carries the bowl while eating. Aim blends ready to aim and tracks the target; strike frames land on the attack trace (first sighting, `Age <= 1`); hit gives flinch, 0.12-tile stagger, blood away from attacker; pawns turn to the target before the first shot (`targetFacing`).
- Work: one action table (chop axe/chips, mine+breach pickaxe/sparks, build/deconstruct/repair hammer/sawdust, sow trowel/soil, harvest sickle/leaves, craft/cook/tailor/butcher/research bench pose, tend bandage, tame open hand, eat spoon+bowl, refuel bench/chips). Hauling and rescue show a carry pose. New particles `partSpark`, `partCasing`, `partSawdust`, `burstDir`. Validator checks every action set and grip, support, end order.
- Limits: chibi proportions: chop/mine hands cover the face in south view; north poses foreshortened; `horseshoes` uses generic work frames; no pistol/shotgun/explosions; floating damage text can overlap. The `hits` capture never centres on a blow: use `-scenario armory -capture-ui raid`.

## ART-009-living-world
Living world: wind, weather, vegetation, burial (ART §9, §16, §27.3, §28) · 2026-10-19 · d09965b21
- Changed: vegetation 34 to 217 sprites: oak/birch/pine x3 variants (oak/birch `_autumn`, `_bare`), bushes, berry bushes (+`_harvestable`), grass 00..07 and tall 00..03, 8 crops x 6 stages x 3 variants; old ids kept as aliases. Animal corpses in 3 poses, bones, graves (dug/filled), headstones, pyre, `tool/shovel`. New `weather` family (snow cover, drifts, puddles, frost, rain/splash/snow textures).
- Environment (`environment.go`): wind field with two gust fronts at 1.2x wind; lean is the exact steady-state of a damped spring (k=38, zeta=0.35), no per-plant state, 0.3 s canopy lag; creature contact pushes plants within 0.6 tile (settle 1.08 s, per-tile map pruned); rain, snow (four sizes), storm darkening with deterministic lightning, blizzard fog, snow cover banked against walls, puddles, frost. Keys from prebuilt tables (no per-frame strings). Weather particle pool capped 1400 rain / 1100 snow.
- Burial (`burial_anim.go`): human corpses drawn from the deceased's layers lying (`fall4`) with dead tint, rot tint and flies; pyre flame/glow/embers/smoke emitted per advance, never in Draw; dig uses `mine` frames with shovel, bury `pick`, cremate `bench`; carried body is a wrapped bundle. Grass denser (1 in 5 soil tiles).
- UI: HUD shows weather and a wind arrow. Benchmark `BenchmarkDrawPlants` ~0.54 ms for 1000 plants and 6 movers (budget 1.2 ms).
- Limits/Open: snow cover needs ~3 in-game hours of snowfall (snow clip starts at tick 12000); no Architect icons for grave/pyre (borrow stone/campfire); concurrent `make art` runs share Blender's temp render file (use a private `TMPDIR`); no biome plants or vegetation far-zoom LOD.

## ART-010-normals-light-shadows
Normal maps, sun/moon light, moving shadows · 2026-10-20 · 9d64cde88
- Credit: normals use a Go port of spritefusion-sprite-normals by Hugo Duprez (MIT, 2026), studied at commit `d14b067`; licence in `third_party/spritefusion-sprite-normals/LICENSE`, notice in `THIRD_PARTY_NOTICES.md`, attribution header on every `pkg/spritenormals` file.
- Changed: `pkg/spritenormals` (relief pipeline; float32 rounding means a byte may differ by 1); `cmd/normalgen`, `make normals` after Blender in `make art` writes `<sprite>_n.png` (radius 8, softness 2.3, detail 1.2 for @2x; skips masks, UI, terrain, floors). World sprites re-rendered with no baked shadow (`shadow=False`); each sprite has a `shadow:` caster block (`kind` blob/box/none, `w`,`h`,`d`). New sprites `overlay/shadow_blob`, `shadow_box`, `roof_*`; icons `roof_build`, `roof_remove`, `overlay_roof`. Validator requires the normal map and shadow block (`--no-light` skips) and rejects normals on terrain/floors.
- Light (`light.go`, `lit.kage`) from sim clock (`WorldView.Hour`, `DayOfYear`), deterministic: sun rises east 06:00, sets west 18:00, peak 65 deg summer, 50 spring/fall, 32 winter; noon calibrated so a flat normal is ~1.0; moon intensity 0.22 peaking 40 deg; sky ambient open, indoor ambient under roofs; warm lamp light in lit rooms and within 3 tiles of a burning campfire; weather dims sun/moon.
- Draw path: world sprites go through `drawLit` (`DrawTrianglesShader32`, sprite + normal packed; Custom0 sky, Custom1 lamp, Custom2 normal offset); terrain, floors, overlays, particles, UI lit flat/unlit. Night veil `drawDaylightRooms` retired; `skyOverlay` only for the non-art fallback.
- Shadows (`shadow.go`): one offscreen layer with max blend, roofed tiles cleared, composited in (18,20,30); blobs for creatures/plants/items, swept boxes for buildings; length `clamp(h*cot el, 0.25h, 3)`; alpha noon 0.38, low sun >= ~0.24, fade in/out over first/last 30 min, moonlight 0.14, storm x(1-0.6*intensity). Interiors: soft eaves, no rain/snow/puddles/frost on roofed tiles, collapse dust once per stamp.
- Benchmarks: DrawArt 2.0 to 2.5 ms (+25%, gate +35%); DrawPlants 0.72 ms (gate 1.2); Shadows 0.32 ms (gate 0.6).
- Limits: no mipmaps on the shader path; each pawn layer carries its own shadow block (only body is a sensible caster); light has no gameplay effect yet; ebiten coverage 62% below the 80% bar.

## ART-011-animal-roster
Animal roster: 61 species from one parametric rig · 2026-10-21 · e21011daa
- Changed: `families/animal_roster.py` mirrors the Go roster; ten RimWorld-invented creatures are original designs (ART §11.1 originals rule). Rig in `families/animals/` (`rig.py` Body spec + 2-bone IK Skel + poses; `parts.py`; `quadruped.py` 48 species incl. monkey; `bird.py`, `reptile.py`, `insect.py` 13 with `materials_animals_extra.yaml`). Per species and facing: standing, `walk01..08`, attack set (`bite/claw/kick/gore/headbutt/peck` 1..4), `graze1..2`, `flinch1..2`, corpses `item/corpse_<key>_v0..v2`, icons `ui_icon/<key>`; female sets for deer, elk, chicken (hen). Old dustbuck/ridgewolf art removed. Outlines 2 px (XS) and 3 px (S). 7556 sprites, validator requires every roster species' full set.
- Renderer: attack frames by `AnimalView.Attack`, `_f` art for females, size-aware hunt/tame markers and health bars, 61 original descriptions (`animal_blurbs.go`), icon fallback (initial on a disc), corpse sprites by species, burst effects for explosive deaths. Tests: `TestEverySpeciesHasArt` plus frame keys, female fallback, blurbs, corpses.
- Limits: animal lean is 40 deg while `contracts.FIGURE_LEAN_DEG` says 55 (kept); slim animals narrow from N/S; XS birds are blobs at far zoom; emberbuck throat gland faint; cobra hood edge-on E/W. Animal family renders ~8.5 min (private TMPDIR per worker); repo grew ~60 MB.

## ART-011-menu-colony-diorama
Main menu dusk colony diorama · 2026-10-21 · 217a4a8f5
- Changed: menu background is a Blender-built dusk colony scene (lit room with beds/table/lamp, campfire with four colonists, workbench lamp, stockpile, crops, autumn trees, grazing dustbuck, rock ridge) rendered through the canonical stage (`stage.render_scene`), drawn with slow pan/zoom, flickering fire glow, breathing lamp glow, embers, dust motes and a scrim keeping title, buttons and bust readable. Sub-menus show it at 55% dim (`fillMenu`). Main Menu §2 Background.
- Contract: `AppConfig.LoadMenuScene` optional; nil makes `NewApp` find `assets/metadata/ui_menu/menu_diorama.yaml`; load failure falls back to the gradient. `-capture-ui menu` captures without a game; `-capture-ticks` picks the moment. `make art-menu`.
- Decisions: one canonical rig (ART §21), no scene camera/light. Figures cast no sun shadow, numpy blob shadows stand in. Dusk grade is baked and position-independent; all screen-relative darkening is in Go (cover-fit crops differ per aspect ratio). Motion runs on `menuTick`, never the simulation. `ui_menu` metadata family is skipped by the sprite catalog. Full 3200x1800 render takes 16 s.
- Limits/Open: one render, so overlapping objects lose their own outlines (optional object-ID contour pass); seasonal variants optional; only checked by eye at 1280x720. `validate_assets.py` not run (no Pillow).

## ART-012-creature-shadows
Pawn and animal silhouette shadows · 2026-10-21 · e21011daa
- Changed: pawns and animals cast their own silhouette (every pawn layer incl. held item and carried item; the animal's current frame), so walking legs, swings and lunges move in the shadow. `drawLit` routes to `castSilhouette` when `r.cast` is set; figure draws are pure (`drawPawnFigure`, `drawAnimalFigure`); a small Kage shader draws white silhouettes with height in R. `shadow_blur.kage` is a contact-hardening blur (radius grows with height and low sun).
- Projection (`castProjection`): affine about the feet pivot; height goes along `ShadowDir*m`, `m = ShadowLen(h)/h` for sun/moon, `tip/h` for point lights; `kappa = h*artTilePx/opaquePx` calibrated per sprite from alpha bounds; lying poses use kappa x 0.25. Contact pools under every creature, also indoors, at night and in storms.
- Point lights: burning campfires (H 0.5, reach 4.5) and lamps in lit rooms (H 1.6, reach 6), the 2 nearest per creature; length `d*h/(H-h)` clamped [0.3h, 3] (campfire shadows are always 3 tiles); alpha `0.30*falloff*(1-daylight)` with campfire flicker.
- Shadow strength (`light.go` `shadowStrength`): `0.95*S/(A+S)`; floor 0.34 (moon 0.12), cap 0.62 (moon 0.30); summer noon ~0.45 (was 0.38), dawn/dusk 0.34 (was 0.24); rain and storms fade shadows through the light itself.
- Decisions: all debug text goes through `debugPrintAt` because Ebiten's DebugPrint glyph cache raced between parallel UI tests.
- Benchmarks: Shadows 0.44-0.57 ms (budget 0.9); CreatureShadows 0.17-0.23 ms (budget 0.5); DrawArt unchanged (~2.3-2.7 ms). Guarantee test: `TestShadowFields`.
- Limits/Open: no lying pose for downed animals (`AnimalView` has none); point lights and contact pools drawn after the roof clear; trees still cast swept blobs.

## ART-013-biome-terrain-plants-rest-pose
Biome terrain, plants, items, rest pose · 2026-10-22 · 04d46d084
- Changed: terrain sand, marsh, ice (4 variants + 15 edge masks); plants pine (snow looks), jungle tree, cactus (dry), shrub (dry), reeds; feeding trough (empty/full, 4 facings), wool carpet; items milk, wool, egg, fertile egg, ember oil, wool parka, wool carpet roll with icons; a `_rest` pose for all 64 animal sets (incl. female), shown with snore Zs; 12 biome icons `assets/generated/ui/icons/biome_*.png`; palette `materials_biomes.yaml`.
- Renderer (`biome_art.go`, `grass_draw.go`): grass tint by biome; juveniles are scaled adult art (no separate sprites).
- Limits: worn wool parka reuses the tinted leather parka layer; no dedicated juvenile art; no seasonal cactus/reed states beyond dry. The validator needs a Python with Pillow (`/usr/local/bin/python3`).

## ART-014-fences-forbid-icons
Fences, gates, forbidden overlay, icons · 2026-10-22 · fc49cab16
- Changed: new family file `families/fence.py`: wood and stone fences with 16 autotile masks (`fence_<mat>_00..15`), gates `fence_gate_<facing>` closed/open (`fence_gate_open_<facing>`), `world/overlay/forbidden` (red marker), icons `order_forbid`, `order_allow`, `fence`, `fence_gate`, `overlay_rooms`; all with normals.
- Limits: single gate kind, no stone gate art.

## ART-014-pawn-builds-portraits-water
Pawn builds, storyteller busts, water, NG-1 world art · 2026-10-22 · f33df2d86
- Context: part of NG-1 (`.agents/handoffs/NG.md#ng1_new_game_plan` §7; decisions D5, D9, D12). Commits `ece5b5e8`, `f7bebeb1`, `42cb19d2`, `24836829`.
- Changed: six builds `m_thin, m_avg, m_heavy, f_thin, f_avg, f_heavy` from one parametrised `BUILDS` table driving body and all clothing layers, frames and facings; keys `pawn/<layer>_<build>_<facing>_<frame>`; old `pawn/body_...` keys alias male average so `look.Legacy` renders as before. Hair: short, crop, bun (legacy) plus long, braid, shaved, curls, tail; beards short/full/moustache (~45% of males); grey-hair palette entry.
- Atlas (`cmd/atlasgen`): ~15,000 pawn sprites (with tint mask and normal) packed into six pages of at most 4096x4096 (`assets/generated/atlas/pawn_0..5{,_mask,_n}.png`), index `assets/metadata/atlas/pawn.yaml`; catalog serves atlas keys; `atlasgen -family pawn -check` is part of `make art-check`; `make atlas` repacks; pages decoded once then dropped (ENG-001). Committed artifact = atlas + metadata; per-sprite renders and metadata of the five non-legacy builds are build intermediates, git-ignored (rebuild: `make art` then `make atlas`).
- Storyteller busts (`families/portrait.py`): original Orla Vane, Bram Holloway, Kestrel Quay, 512x640 + 120x150 thumbs; keys `ui_portrait/steady|hearth|gambler` (+`_thumb`) held in the profile's `Portrait` field; a test follows the catalog key (`24836829`); descriptions in ART §17.1.
- Water/sites: terrain `shallow_water`, `deep_water`, `mud`, `track` (4 variants), edge masks (mud over soil, shallow over deep), shore-foam masks (ripple/glint shader: SIM-033); `rock/cave_mouth_*`, `rock/ore_machinery_*`; `effect/pod_fall`, `pod_dust`, pod shell building.
- Items/icons: components and industrial medicine (3 stack sizes), tool cabinet, hospital bed; icons for four scenarios + `scenario_custom`, eleven skills, minor/major passion, trait, faction emblems `faction_<settlers|clans|drifters|freebooters|swarm|player>`, `settlement_neolithic|industrial` (tinted at run time), minimap. The globe is vertex-coloured from `assets/palettes/world.yaml`, not sprite art.
- Limits/Open: male/female and thin/average/heavy read weakly at 1x (widen silhouettes or add a clothing cut); beards nearly invisible facing north; only the pawn family is atlas-packed; no distinct head shapes; keep checking busts and icons against the original-art rule. Budget: ~12.6k new sprites per build.

## ART-015-baked-corner-linked-rock-stone-tints
Linked walls and rock: baked corners, stone tints · 2026-10-22 · f7bce7011
- Changed: linked families (`families/linked.py` `_variants`) bake inner corners, 47 sprites per family (`<prefix>_<NN>`, `_c<h>`; NE=1, SE=2, SW=4, NW=8). A corner is filled when both cardinals and the diagonal link (corner-filler rule from the rendering reference); unfilled concave corners are notched so they meet neighbours' free edges. Eight families: `rock/rock_wall`, `rock_smooth`, `ore_{steel,silver,gold}`, `structure/wall_{wood,stone,steel}`.
- Neutral stone: rock, stone walls, fences, floors, stone terrain/edges and `stone_chunk`/`stone_blocks` drawn in neutral greys (`stone_neutral*`); the renderer multiplies in the stone tint. Ore keeps unmasked metal flecks on a `stone_tint` rock body. New: steel doors, `terrain/stone_smooth_00..03`, `item/stone_chunk|silver|gold`, `furniture/stonecutter_table_*`, `overlay/designation_smooth`, icons `order_smooth`, `stonecutter_table`.
- Decisions: seamless rock: lumps touching a linked edge are centred on it (two per edge at 0.28 and 0.74, seeds fixed per axis/position), so neighbouring tiles render the same blob cut in half. `cmd/normalgen` extends linked edges (metadata `autotile` + `mask`) before beveling so joined tiles light as one surface (`TestLinkedEdgeHasNoBevel`, `TestLinkedMaskFromMetadata`).
- User decision: `base.yaml` `tints.stone_tint` values for sandstone, granite, limestone, slate, marble are the user-supplied rendering-reference RGBs. Palette gains `stone_neutral`, `stone_neutral_dark`, `stone_tint`, `silver`, `gold`.
- Dev: `-scenario walls`, saved by `make art-capture` as `ingame_walls.png`.
- Limits: doors link as walls (repo convention; the reference treats doors separately); no mountain-roof or fog-of-war art.

## ART-016-vegetation-restyle
Vegetation restyle (PLAY-010) · 2026-10-23 · cf43235d7
- Changed: ground vegetation follows ART §9.1: small, flat, muted-olive, transparent gaps, no outline, many per tile. Keys `grass_short_v0..5`, `grass_tall_v0..3`, `grass_short_dormant_v0..2`; every wild plant has young/mature/dormant looks `<key>_<stage>_v0..3` (berry bush also `_harvestable`; wild healroot is `healroot_wild_*` to keep crop keys); seven new plants; crops re-rendered without outlines. `cmd/normalgen` `flat_normal` gives low cover a flat normal so it can be mirrored. UI: inspector "Slows walking", far-zoom colours, hover "Grass (growing, N%)".
- Decisions: AGENTS.md / ART §25 rules added: remove unused assets in the same change (`make art-check` orphan check with `assets/metadata/orphan_allow.txt`); plants never have a contour (`TestPlantRecipesHaveNoOutline`). Palette `materials_plants.yaml` (`pl_*`).
- Open: old keys `grass_00..07`, `grass_tall_00..03`, `bush_0N`, `berry_bush_0N`, `shrub_0N`, `cactus_0N`, `reeds_0N` remain renderer fallbacks; delete with their fallback entries in one change once nothing reads them.

## ART-017-trees
Trees (PLAY-010) · 2026-10-23 · cf43235d7
- Changed: oak, birch, pine, jungle (autumn, bare, snow looks) have a visible trunk with slim root flare, a loose gapped canopy overhanging into the tile above, muted §9.1 colours, no outline; heights 1.5-1.7 tiles (bare taller). Each tree also renders `_trunk` and `_canopy` parts on one canvas/origin; the renderer draws trunk then canopy. Own build family: `make art FAMILIES=trees`.
- Decisions: canopy fades to 40% while a creature stands on the tree's tile, the tile above or the two diagonals above, or the hovered tile, selection or Architect ghost is there (eases ~0.2 s, instant with Reduced Motion); trunks and shadows never fade.
- Open: the menu diorama needs `make art-menu` to pick up the new trees.

## ART-018-xs-animal-size
XS animal size (PLAY-010) · 2026-10-23 · cf43235d7
- Changed: XS body length 0.44 tiles (was 0.35): rat, cinder rat, chinchilla, fennec fox, guinea pig, squirrel, Yorkshire terrier, chicken, duck, hive beetle (XS+, 0.506). 822 sprites + normals re-rendered.
- Decisions: largest XS stays below smallest S (S-, 0.51: cat, hare, snowhare); pinned by `TestXSBetweenOldAndS`. Size reaches only the read model (`BodyLength` to marker and bar height), so simulation and goldens are unaffected.

## ART-019-social-host-assets
Social and host-system art (ENG-003) · 2026-10-24 · 141eccc86
- Changed: furniture brewing bench, drug table, marriage spot, party spot; crops brewgrain and ashleaf (no outline); 11 items, 11 social icons, fight lightning; pawn atlas repacked.
- Limits/Open: 6 order icons (capture, arrest, prisoner, caravan, history, surgery) and the face scar overlay are not wired yet; wire the order icons with M8/M9 and the scar in the pawn renderer. Unwired assets need a reader or removal in the deciding change; allowed orphans are in `assets/metadata/orphan_allow.txt`.

## ART-020-vents-ruins
Steam vents and ruins art (PLAY-013, MAPGEN-2 §3.4-3.5) · 2026-10-24 · e5f89f8e6
- Changed: `structure/steam_vent` (2x2 mineral mound), `effect/steam_puff_0..3` (bursts every 4-7 s per vent, 150 ms frames, phase hashed from position, Reduced Motion holds one frame), `structure/wall_ruin_overlay_0..2` (cracks over ruined walls of any material, by position), procedural `sfx.steam_vent` (2 variants, cmd/audiogen, cued once per burst while the vent is drawn).
- Gameplay tie-ins: vents are `construction.SteamVent` (Placed and Natural); cannot be deconstructed, moved, swapped or built over; warm outdoor air +6 C in a 6x6 patch and raise their room's settling temperature +1.5 C; no power use yet. Ruins are ordinary walls/floors flagged `Ruin`, already damaged; not colony wealth or Home anchors, not repaired by builders, salvage half (rounded up). UI: inspector "Ruined granite wall" etc., vent inspector says it warms the air and can't be removed; hovering deep rock says "Thick rock overhead (permanent)" (thick roofs can't be removed; drop pods avoid them).
- Notes: `make art FAMILIES="structure effect"` re-renders with +-1/255 GPU noise; only new files were committed.

## ART-021-bridge-duskmoss-casket
Bridge, duskmoss, sleeper casket art (PLAY-014, MAPGEN-3) · 2026-10-24 · b6b0ecfd5
- Changed: `structure/bridge_00..15` (128x128 timber deck, boards perpendicular to walking direction, no baked shadow; renderer adds one on water), `structure/sleeper_casket_{closed,open}` (1x2), `plant/duskmoss_{young,mature,dormant}_v0..3` (no outline), icons `ui_icon/bridge`, `open_casket`, palette `materials_mapgen3.yaml`, procedural `sfx.casket_open`.
- Notes: `make art` re-rendered 807 untouched files with GPU noise; restored. Duskmoss reads a little spidery at full zoom.

## ART-022-crops-restyle
Crops restyle (PLAY-016, ART §9.3) · 2026-10-25 · ce02a68bb
- Changed: crops rebuilt as Blender primitives in the muted `pl_crop_*` palette (`materials_plants.yaml`; unused `cloth_white` removed), no outline, no shadow, no soil mound; one small unit `plant/<crop>_<stage>_v0..3`, 6 stages (sown, sprout, young, mature, harvestable, wilted). Grains (`crops_grain.py`): corn, wheat, brewgrain, cotton; leafy (`crops_leafy.py`): potato, beans, lettuce, radish, pumpkin, healroot, ashleaf, wild healroot. Berry bush gained plum berries; produce items and 12 icons restyled (items keep outline); tilled furrows `overlay/tilled_0..3` (tileable, rotatable). Old builders and aliases (`crop_sown`, `<crop>_{young,mature,harvestable}`) deleted.
- Renderer (`crop_batch.go`): units per tile (corn 2; wheat/brewgrain 4; cotton/potato/beans/healroot/ashleaf 3; lettuce/radish 4; pumpkin 1), row patterns along the zone's long axis, mirroring, continuous growth height, per-unit sway; lazy atlas, one lit batch per crop run; LOD tiers below 48 px and at map zoom; furrows under crop and sown cells.
- Damage: 0-49 rime or scorch tint; from 50 the wilted sprite shrinking to 0.6; from 70 units fade out one by one (trees and bushes get the whole-plant version); shadows thin with the units. UI: "Freezing: N% damaged". `PlantView.Wilted` no longer changes the sprite; damage does.
- Benchmark: `BenchmarkCropField500` 0.13 ms at default zoom. `stage._render_raw` writes a shared `$TMPDIR/rwg_render.png`: render concurrently only with a private TMPDIR.

## ART-023-storyteller-portraits-raster
Storyteller portraits as painted busts · 2026-10-26 · 7f5bb946c
- Changed: the three busts (Orla Vane, Bram Holloway, Kestrel Quay) were far below the menu colonist, so they are now painted rasters `assets/source/raster/ui/portraits/{steady,hearth,gambler}.png` (1288x1600, magenta field) made with Canva's image generator from `briefs.md`; the only style reference was our own `main_menu_colonist.png`, no RimWorld art. Character briefs unchanged (ART §17.1).
- Tool: `assets/source/tools/portrait_cut.py`, `make portraits`, deterministic: keys out magenta and spill within 6 px, inks the fringe, fits bottom-flush into 512x640 with 4 px contour, 120x150 head-framed thumbnail with 2 px contour, writes `metadata/ui_portrait/*.yaml` with the same keys (minus unused `projection`).
- Removed: `families/portrait.py` and its `FAMILIES` entry, 18 bust-only colours in `materials_ng1.yaml`, the dead `ui/portrait/` fallback in `screen_storyteller.go` `portraitImage`. No behaviour change beyond the art.
- Notes: only `/usr/local/bin/python3` has Pillow and numpy.
- Open (for the user): repo-root `ai-bg-rimworld` describes itself as a "reconstruction" of a 700x412 source crop; the menu art's clean-room status deserves a check (out of scope here).

## ART-024-ui-chrome-charcoal
UI chrome to neutral charcoal with gold accent · 2026-10-26 · 17a676638
- Changed: `widgets.go` tokens and `assets/palettes/ui.yaml` (in sync via `TestUITokensMatchPaletteYAML`): panel `#26292EE0`, panel border `#5A5D61`, button `#3C3F43`, hover `#4B4E52`, selected `#5E5440`, disabled `#24262B`, button border `#66696E`, accent/focus/slider knob `#D9B45B`, text `#E8E4DA`, dim `#B8B4AA`, disabled text `#6D6A64`, heading `#F0D9A0`, fields `#1F2022`, tooltip `#222428F5`, neutral black scrims, menu space/horizon/dust `#0E0F12`/`#24262A`/`#8C8576`, danger lightened to `#FFA094` for contrast. Primary button blends 0.6 toward the accent (label 5.1:1). Other semantic colours unchanged.
- Decisions: user asked for art direction very close to RimWorld's, style only, clean-room (plan §3.10). Replaced `TestUITokensStayClearOfReferencePalette` with `TestUIPanelsAreNeutralTranslucent`, `TestUITextContrast` (text 11.5:1, dim 7.0:1 on panel over a dark map) and `TestUITokensAreNotCopies` (dE >= 3 from the reference hexes, near-copies forbidden).
- Open: icon badge disc (`overlay.py` ~L178) to `#303236` with `#0F1012` edge when icons are re-rendered (done in ART-025); the menu diorama grades with `ui.space`, so its next `make art-menu` is neutral-dark (wanted).

## ART-025-rimworld-look
RimWorld-style look: camera, outlines, palette, limbless pawns · 2026-10-26 · 3ec8d9574
- Decision (user, 2026-10-26): art very close to RimWorld's readable top-down look; style only, clean-room, nothing traced, sampled or referenced. Contracts (`artlib/contracts.py`) picked from a 3x3 prototype grid: `SHEAR_K` 0.5 to 0.35; `FIGURE_LEAN_DEG` 55 to 82; `ANIMAL_LEAN_DEG` 40 (hard-coded twice) to 70 as a contract; `OUTLINE_PX`/`DARKEN`/`SATURATE` 4/0.32/1.25 to 6/0.18/1.05; `RAMP_INTERP` EASE (+-`RAMP_SOFT` 0.06); `SHADOW_T`/`LIGHT_T` 0.38/0.86 to 0.30/0.80; new `OUTLINE_INNER_PX` 2 (figure holdout pass so hair/hat/pants outlines don't spill). Env overrides `RWG_OUT_ROOT`, `RWG_PALETTE_DIR`.
- Palettes: `tools/palette_mute.py --contrast 0.8 --outline 0.18,3` (saturation and light/shadow x0.8, light band x0.96, shadow hue up to 8 deg warmer; emissive and neutral tint greys skipped; header marker prevents double application).
- Pawns (`families/pawn.py`, 1061 to ~480 lines): round head on a capsule body with dot eyes, no arms/legs/hands (RimWorld draws none; test hand dots read as flails). 6 builds kept as capsule widths; thin apparel shells; 8 hair styles, 3 beards, tuque, face scar kept. South/east/north rendered, west mirrored; no frames. Attachments `head`, `hold`, `back`. 147 sprites (was 40,472, ~470 MB); atlas is 1 page.
- Renderer: `figureLayers(art, Look, facing, dress)` serves colony, portraits and colonist bar (builds and beards now show in the colony). `facingKey` mirrors west for pawns, held items, face scar, shadows, animals; the lit shader inverts normal X for mirrored sprites (`TestMirroredDrawInvertsNormalX`). All frame machinery is gone; motion is transforms (walk bob/sway/lean, per-action tool arcs, lunges, recoil, buckle then rotation when falling; strike timing still lands at trace age 0). Items grip on `hold`, slung on `back`, behind the body facing north. `projShearK = 0.35` (`light.go`) matches metadata (`TestProjShearMatchesMetadata`). This supersedes the frame sets of ART-007/008.
- Re-rendered: pawn, animal (S/E/N only, centred canvases, corpses), structure, furniture, power, fence, terrain, rock, item, weapon, tool, effect, plant, trees (no outline), overlay, weather, UI icons (grey `#303236` disc), menu diorama (plants excluded from scene outline). 5367 sprites, 0 validator errors. `BenchmarkDrawArt500` z1 16.9 to 17.2 ms (within 10% budget).
- Open: muted palette makes wall tops, rich soil and mud close in value (walls rely on outline and darker south face); odd `origin_px` y values give half pixels at 1x (pre-existing); long-legged animals narrow facing S/N at lean 70; normals not rotated for lying sprites (pre-existing); `grave_dug_*` clipped before this pass, not fixed.

## ART-026-medical-items
Medical item art, vitals monitor, care icons (MED-05/06) · 2026-10-26 · 2feec5ec6
- Changed: item sprites `nanomed_kit` (one sprite), `servo_arm`, `servo_leg`, `optic_implant`, `cochlear_implant`, `spinal_brace`, `pump_heart`, `synth_stomach`, `valve_heart`, and `kidney`/`lung`/`heart`/`liver` in a squat glass jar with teal seal; `furniture/vitals_monitor_{south,east,north,west}` plus `_glow_<facing>` layers (emissive `med_teal_glow`); icons for the 13 items and `vitals_monitor`; disc glyphs `care_none|nomeds|herbal|industrial|best` now drawn on the Health tab care buttons. Palette additions only ("MED items" block in `materials_ng1.yaml`). All original, in the ART-025 look.
- Fix: `itemSprite` fallback maps spaces to underscores so "servo arm" finds `item/servo_arm`; peg leg, glass eye etc. now find sprites too (`TestItemSpriteFallbackUsesUnderscores`). Re-render changed 0 existing files; 21 new normal maps.
- Open: facing east/west the vitals monitor reads as a tall slab.

## ART-027-held-items-rimworld
Weapons and items held the RimWorld way · 2026-10-26 · aa5e3bf71
- Decision (user choice): after ART-025 pawns have no hands and held items floated, so handling copies RimWorld's observable behaviour with our own values (no code copied). Placement measured from the body centre (head x, halfway head to feet); weapons turn about their own centre.
  - Off duty (idle, walking, working, eating): nothing in hand, no slung rifle, no tools, no spoon/bowl.
  - On duty at rest (drafted, hostile and not fleeing, hunting): weapon 0.18 tile below centre, muzzle 0.72 rad down; east +0.14 tile, west mirrored; behind the body facing north.
  - Aiming or melee: weapon 0.35 tile out toward the target, flipped upright aiming left, behind the body only aiming north. Hauling: 0.5-tile item at the chest (0.07 below centre), behind facing north.
  - Shot kicks the muzzle 0.3 rad and pulls the weapon back 0.06 tile; knife and spear jab along the aim, club swings +0.8 to -0.45 rad; fists draw nothing but the body lunges; a flinch keeps the load or aimed weapon. Work keeps lean/dip/squash; particles come from the worked tile's face toward the pawn.
- UI: in-world work progress bar (`work_progress.go`) while `ActionProgress` in (0,1), colonists only, not eating/research/horseshoes; 0.6 x 0.06 tile, accent fill on slider track, only at >= 24 px per tile, along the worked tile's bottom edge. UI checklist "Progress bar" `[-]` (in-world bar, not a shared widget).
- Removed: every `tool/*` sprite (axe, bandage, bowl, broom, hammer, pickaxe, shovel, sickle, spoon, trowel) with masks, normals, metadata, and `weapon/rifle_slung`; `gear.py` keeps rifle, knife, spear, club.
- Guarantee tests: `TestNoWeaponWhenOffDuty`, `TestCarriedWeaponOverlapsBody`, `TestRifleAimsAtTarget`, `TestWorkDrawsNoTool`, `TestLoadKeptDuringFlinch`, `TestWorkProgressBar*`. `BenchmarkDrawArt500` z1 934 to 952 us (noise).

## ART-028-menu-art-original
Main-menu hero replaced with original art (clean-room fix) · 2026-10-26 · 3b83c45a3
- Changed: the old menu colonist (red-bearded man, raven, rifle) came from `ai-bg-rimworld`, an ASCII "reconstruction" of a 700x412 crop of an existing illustration, which broke the clean-room rule. New hero: a scout woman with goggles, shearling coat, rifle and sled dog, generated with Canva from an original prompt (`assets/source/raster/ui/menu/briefs.md`); only reference was our own `portraits/gambler.png` for style; the user picked it from three takes.
- Pipeline: `menu/hero.png` (1288x1600 magenta source); `portrait_cut.py` `cut_menu` (run by `make portraits`) keys, inks, outlines and fits it bottom-flush into the 904x1104 `main_menu_colonist.png`. Splash contract and release packaging unchanged. `ai-bg-rimworld` deleted; `assets/AGENTS.md` and ART §17.1 updated. `make portraits` re-cuts the busts byte-identical.
- Open: the ART-023 busts used the old menu image as a style reference only (comic cel-shading, nothing else carried); for a fully clean chain regenerate them with the new hero as the reference (cheap, same pipeline; waits on the Canva quota, see ART-029).

## ART-029-menu-hero-extended
Menu hero extended, cut tool hardened · 2026-10-27 · cd4b6c44c
- Changed: the ART-028 hero had the scout's right arm and rifle muzzle past the frame's right edge, so the menu showed a vertical cut through the sleeve (plus a 4 px dark inked band at x 900-903). `hero.png` is now an extended take (1200x1600, 3:4): Canva with the ART-028 hero as the only reference, user picked from two takes (a third refused by the credit quota); prompt in `menu/briefs.md`. `main_menu_colonist.png` (904x1104) re-cut: content x 36-867, last columns transparent, bottom row flush.
- `portrait_cut.py`: `clipped_sides(img)`; `contour(img, width, clipped)` skips the contour on a clipped side (also re-cut busts `gambler` and `steady`, which lost the dark line on the cut side); `cut_menu` refuses a source whose figure touches the right or top edge (`SystemExit`).
- Guarantee test: `TestMenuSplashKeepsItsRightEdgeFree` (splash PNG's last two columns fully transparent).
- Open (user): delete the Canva scratch design "Blank magenta portrait scratch canvas" (`DAHZHun9Dn8`, 7 pages; no delete tool); Canva generation quota is used up, the storyteller-bust restyle from ART-028 waits for it.

## ART-030-play022-animals-puddles
Animal size, animation and puddles (PLAY-022) · 2026-10-27 · d839757b9
- Plan/contract: `.agents/handoffs/PLAY.md#play022_plan`. User requests (2026-10-27): animals too small (XS about 1 tile, XL up to 6), unnatural animation, bad puddles. User decisions: size is the AREA a body covers (XS ~1 to XL ~6 tiles squared) while the sim footprint stays one tile; every animal re-rendered with the longest bodies at a capped density the renderer scales up.
- Sizes: lengths XS 1.45, S 1.65, M 1.8, L 2.2, XL 3.0 tiles (measured to hit area targets; median drawn areas 0.97/1.48/1.98/3.75/5.90). Go `sizeLength` mirrors Python `SIZE_LENGTH`/`SIZE_AREA` (test); `validate_assets.py` fails a class more than 15% off its area. One canvas per facing (union of that facing's poses); `ppt` 128 or lower where a facing would pass 384 px (L 80-128, XL 64-128, corpses 88-128); animal+corpse pixels 0.88x before. `largeSize` is 1.87; markers, bars, snores follow.
- Motion: every sprite carries `gait` (ground per walk cycle: rat 0.23, deer 0.82, horse 0.98, elephant 1.14 tiles; slip <= 1.3 px); stride 1.7x wider, body pitch 0-4 deg twice a cycle, head nod +-9 deg; bird/reptile/insect get the thin outline. Sim: a step fixes its gait; wandering/grazing/milling amble at 2x the walking step, errands walk, fleeing runs; hunting and charging report run but keep the walking step, so no fight changes; slow class +6 to +12 ticks (tortoise 26 vs deer 16). `AnimalView.Gait`; grazing shows only on grass or an edible plant. Render: walk frames advance by distance / gait, start/stop on standing-height frames (`walk03`/`walk07`), graze bouts 3-8 s down then 1-2 s up, no animal renderer bob, animals face sideways on diagonals, cycle advances at most 0.3 per drawn frame; sprites placed/mirrored/culled/cast at their own `ppt`. Picking: click or box select on any tile of a body picks the animal, after whatever stands on that tile.
- Puddles: one chunk-baked veil (`puddle_veil.go`, `snow_cache.go`) at every zoom tier; a low-frequency per-map relief holds water below a level rising with `Wetness`, so puddles grow and shrink in the same hollows with soft shores and no rim; only open soil, rich soil and gravel; under grass tufts, lit flat with a sky tint; fades under snow; ripples only on water. Puddle sprites, normals, metadata, builder deleted; snow cover and frost lost dome normals (ground decals lit flat). Dev: zoo spaces animals 5 tiles apart.
- UI: the map itself shows sizes, gaits, grazing; no text line for gait.
- Save/golden: scale golden (6 pins) and grown golden re-pinned for slower idle wandering and two new saved fields (`Errand`, `StepGait`); biome golden unchanged. Perf (grown 500x500, 3 ticks/frame): fps 116.7 to 117.1 at zoom 0.8, Draw p99 3.48 to 3.58 ms; zoom 0.1 Draw p99 4.25 to 5.10 ms, peak 2165 to 2275 MB.
- Open: XS/S read big (XS covers ~2x a colonist's drawn area, the requested target; lower `SIZE_AREA` XS/S and re-render if wrong); animal PNGs 79 to 143 MB, corpses 5 to 11 MB (a lossless re-save shrinks ~20%, pipeline-wide, not done); far-zoom draw p99 +0.85 ms (veil ~0.3 ms, rest unexplained); normalgen's fixed 8 px bevel is softer on 64-ppt sprites; cow, pig, ground sloth chase a little slower; social scenario test clears wildlife; stale git-ignored pawn PNGs (2026-10-22) fail `make art-check`, `TestAtlasCoversKeys`, `TestNoOrphanSprites`, `TestIndexCatalogMatchesLoad` on this disk only; `make lint` has 17 older findings.

## ART-MEAT-steaks
Meat reads as steaks · 2026-11-03 · e924244d3
- Changed (asked 2026-11-03): `artlib/families/item.py` `_steak`/`_meat`: a pile of three kidney-shaped steaks with cream fat rim and marbling. Meat red with marrow bone; human meat pallid pink with bruise-purple veins and a bone; insect meat olive with pale-green marbling, no bone. Butcher table cuts (block, laid-out, hook) are steaks in four rotations, icon recut; fine meal is a seared steak, icon recut.
- Decisions: ids unchanged, so metadata, `keys.go` and palettes untouched. Lavish meal's roast, simple meal's stew chunks and drying-rack strips stay (not steaks in those dishes).
- UI: no new behaviour; rack loads and HUD icons use the same sprites; trade and caravan lists have no item art.

## SHADOW-creatures-cast-none
Creatures cast no shadow (agent SH) · 2026-11-01 · c2a9c3683
- Decision (user, 2026-10-31): pawns, animals and mechanoids cast no shadow at all. This retires ART-012's silhouettes, point-light shadows and contact pools. Buildings, rock, plants and items keep box and blob shadows; the Options Shadows toggle still covers them.
- Changed: `render/light/shadow_cast.go` deleted (projection, `Plan`/`PlanCreature`, `PointLight`s, `castCreature`, `creatureLights`, contact batch, cast shader); `Grid.lampReaches` and `Rect` deleted. `Shadows.Draw(v, lit, host, screen, w, sw, sh, tile)` draws only boxes and blobs and returns early when nothing casts; `Casters` no longer generic (`Objects`, `VisibleTiles`, `White`); `Cast` is an empty bake marker; `shadow_blur.kage` lost the `R1` height term and blocker search (blurs by `R0` only, `BlurRadius`). `Lit.Draw` and `DrawGround` lost their `cast` parameter. `render/creature`: every `pass.Cast` branch and `FigureLying` gone. `render/world`: `Shared.Cast` became `base.Cast`, set only by `WithBake`; water's baked-frame flag reads `LOD.Baking`. Root: `shadowHost.CastFigure`/`Creatures` and `Renderer.cast` deleted. `art`: `Sprite.OpaqueHeightPx` deleted (`OpaqueBox` remains). The dead `Shadows[F]` type parameter is gone (`light.Shadows` everywhere).
- Art: pawn and animal sprites carry no `shadow:` blocks; `stage.NO_SHADOW_FAMILIES` skips them, `validate_assets.py` rejects one, and 4014 existing blocks were stripped by script (images unchanged, no re-render).
- Guarantee tests: `TestCreaturesCastNoShadow` (root; noon, low sun, moonlight with lamps outdoors and in a roofed room: creatures alone produce no layer and no composite; no figure gets a lit draw). Deleted: root `shadow_cast_test.go`, `render/light/shadow_cast_test.go`, `BenchmarkCreatureShadows`; `shadow_pass_test.go` rewritten.
- Perf: `DrawArt500/z1` 414 draw calls both ways, allocs 3798 to 3749, ns/op 1.02-1.17 ms to 1.01-1.05 ms. UI: none; removing the shadows is the visible change.

## CURSOR-000-migrate-agent-to-cursor
Moved `.agent/` into `.cursor/` · 2026-09-08 · a9ccd60f8
- Changed: agent docs only. The whole `.agent/` tree moved to `.cursor/` (specs, skills, handoffs, Cursor rules in one project location); thin Cursor skill stubs for `rimworld-go-art-assets` and `rimworld-go-ui-screens` replaced by the full playbooks; all `.agent/` path references retargeted. Go packages and `docs/` unchanged.
- Decisions: root `AGENTS.md` stays the always-applied workspace rule; skills are native Cursor project skills with no duplicate stub layer; historical handoffs rewritten to the new paths.
- Note: superseded by the AI-001 move into `.agents/`.

## VIEW-001-camera-restored-on-load
Camera restored when a game loads · 2026-10-30 · 9bbc29ca7
- Changed (user request 2026-10-30; loads used to always centre): `camera.View{CX, CY, Zoom}` holds world centre in tiles plus apparent zoom, never pixel offsets, so it survives a different window size or device scale; `(Camera).View(vp)` captures, `(*Camera).Resume(v)` stores a pending view (ignores NaN, infinite, zero-zoom). `Clamp`'s unplaced branch uses it, clamped to zoom limits and map, and sets `Placed`.
- Save: `filesave.Store.SaveView` writes `<slot>.rwview` (JSON, atomic temp file + rename) beside the save; `LoadView(savePath)` returns false for a missing, corrupt or invalid file (today's centring kept); `List` ignores it. `SaveHooks.SaveView`/`LoadView` optional, nil-safe, wired in `cmd/game/main.go`. `optionsSaver.Save` writes the view after a successful save (failure only logged); `Load` resumes it after `startSession`, covering manual save, autosave, save-and-quit and window close. Adapter-only: no save-format, application or domain change; `App`/`Renderer` didn't grow.
- Guarantee tests: `TestSaveWritesViewAndLoadResumesIt`, `TestJourneyCameraRestoredOnLoad` (view applies on the first `Update` after load); camera round trip over three viewport sizes and device scales.
- Limits: the store has no delete, so a view file stays if its save is removed by hand.
- MAP-1 follow-up: the sidecar is per map: `filesave.ViewFile.Maps` (omitempty, keyed by decimal map id) holds every other open map's camera, top-level fields stay map 0's, old sidecars still load. See `.agents/handoffs/WORLD.md#map-1-site-maps`.
