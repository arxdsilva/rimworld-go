# assets

Presentation-only art. Nothing in this tree is authoritative simulation
state. The renderer and other inbound adapters consume these files;
`internal/domain` must not.

## Vendor pack (first playable)

`Free 2D Animated Vector Game Character Sprites/` is a CC0 pack by
Raphael Gonçalves (Rgsdev). License: `License.txt`.

It is a **side-view** compositional character kit (idle/walk/jump/hit/
death plus environment tiles). It is not the long-term top-down art
bible in `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`. The ebiten
adapter uses it so pawns and ground are readable now; original
orthographic colony art still has to be produced under Wave 0/§34.

IDs the renderer understands:

```text
core:pawn/vendor_char_1 .. core:pawn/vendor_char_4
core:pawn/vendor_enemy_1 .. core:pawn/vendor_enemy_4
core:terrain/vendor_ground
core:overlay/vendor_crosshair
```

Appearance is assigned from `EntityID` in the renderer (stable, not
RNG). Animation frame is derived from simulation tick in the adapter
and never written back to the domain.

## Original art pipeline (ART Wave 0/1, §34 prototype)

Original world art is rendered from code in Blender. One command
rebuilds everything:

```text
make art           # headless Blender -> generated/ + metadata/, then validate
make art-check     # validator only (python3 + Pillow)
make art-preview   # generated/contact_sheet.png + generated/prototype_scene.png
make art-menu      # main-menu colony diorama (scenes/menu.py), not part of make art
make audio         # procedural sound effects (cmd/audiogen) + the sound list
make music         # composed music (cmd/audiogen/music.go) -> Ogg, needs ffmpeg
make audio-check   # sound list vs files, 48 kHz, 20 MB budget
```

Rules:

- Edit `source/blender/artlib/families/*.py` or the palettes, then
  `make art`. Never hand-edit `generated/` or `metadata/`.
- Camera, light, outlines, scale and canvas math live only in
  `source/blender/artlib/contracts.py` + `stage.py`. Families build
  geometry and call `stage.render_asset()`; they never add cameras/lights.
- Colours come only from `palettes/materials.yaml` (three toon bands per
  material). Cloth/skin/hair are neutral greys with a `_mask.png`; the
  renderer multiplies a tint from `palettes/base.yaml` `tints`.
- Placement: draw a sprite at `tile * 64 - origin_px / 2` (sprites are
  @2x). Draw order: terrain → floors → zones → objects by footprint
  bottom row (north → south, then west → east) → overlays.
- Walls: pick `wall_<mat>_<mask>` with mask N=1, E=2, S=4, W=8 (doors
  count as wall neighbours).
- Linked families (`rock/rock_wall`, `rock/rock_smooth`, `rock/ore_<metal>`,
  `structure/wall_<wood|stone|steel>`; ART-015) bake their inner corners:
  47 sprites each, `<prefix>_<NN>` plus `<prefix>_<NN>_c<h>` where `h` is
  the hex of the filled corners (NE=1, SE=2, SW=4, NW=8). A corner is
  filled when both of its cardinals and its diagonal link; an unfilled
  one is notched so it lines up with the neighbours' free edges. Walls,
  doors and rock link to each other; fences link one way into walls.
  Natural rock links past the map edge, built walls don't.
- Stone is drawn in neutral greys (`stone_neutral*`) and multiplied at
  draw time by the cell's or material's stone tint (`base.yaml`
  `tints.stone_tint`); ore sprites mask only their rock body
  (`stone_tint`) so the metal flecks keep their colour.
  `cmd/normalgen` extends linked edges before beveling, so joined tiles
  light as one surface.
- Pawns: draw `body`, `pants_*`, `shirt_*`, `vest_*`, `hair_*` for the
  same facing and frame at the same anchor; held weapons attach their `grip`
  to the body's `hand` attachment (behind the body when facing north), and
  hair/hats shift by the body frame's `head` attachment. Frames per ART
  §27.1: walk `walk01..walk12`, work `work1..work6`, animal trot
  `walk01..walk08`, animal rest pose `rest` (all animal sets).
- Renders are reproducible to ±1/255 per channel (GPU float noise).

## Layout

```text
assets/
├── AGENTS.md
├── palettes/
│   ├── base.yaml        # outline/shadow/overlay tokens, runtime tint presets
│   ├── materials.yaml   # 3-band toon colours per material (ART §7, §33)
│   ├── factions.yaml    # faction accents (placeholder)
│   └── ui.yaml          # menu/pause chrome tokens (ART §33)
├── source/
│   ├── blender/
│   │   ├── build.py     # entry point (headless or live via Blender MCP)
│   │   ├── template.blend
│   │   └── artlib/      # contracts, stage, post-pass, metadata, families/, scenes/
│   ├── tools/           # validate_assets.py, contact_sheet.py, compose_scene.py
│   └── raster/ui/main_menu_colonist.png
├── generated/
│   ├── world/<family>/<name>.png (+ _mask.png)   # @2x sprites
│   ├── ui/icons/*.png
│   ├── ui/menu/menu_diorama.png (+ menu_glow_lamp/fire.png)  # main-menu scene
│   ├── contact_sheet.png, prototype_scene*.png, pawn_sheet.png  # review aids
├── metadata/<family>/<name>.yaml   # footprint, origin_px, attachments, tint
│   └── ui_menu/menu_diorama.yaml    # menu scene: canvas, glow layers, fire/lamp px (not a sprite)
├── Free 2D Animated Vector Game Character Sprites/   # vendor source
└── .gitkeep
```

`source/raster/ui/main_menu_colonist.png` is original title art (cel-shaded
colonist bust with raven, RGBA cutout). It is not vendor pack art and is
not a reconstruction of proprietary RimWorld screenshots. The brief that
produced it lives at repo-root `ai-bg-rimworld`.

`generated/ui/menu/` is the main-menu background: one scene composed from
the family builders and rendered through the same stage (`stage.place`,
`stage.render_scene`). Edit `source/blender/artlib/scenes/menu.py`, then
`make art-menu`; live, `menu.preview()` writes a quarter-size render.

The renderer draws the original set (ART-006). The vendor pack is only a
fallback when `metadata/` is missing. Do not hand-patch vendor PNGs; treat
the pack as immutable source. `generated/ingame_capture.png` comes from
`make art-capture` (real renderer).

## Audio (SND-002)

```text
generated/audio/{ui,sfx,ambience,music}/   # 48 kHz: WAV effects, Ogg Vorbis music/ambience
metadata/audio/audio.json                  # the sound list: id, bus, loop, volume, files, provenance
source/audio/prompts.json                  # ElevenLabs prompts in spending order + credit budget
source/audio/elevenlabs/ledger.json        # every ElevenLabs take, approved or not, with its cost
source/audio/elevenlabs/raw/               # downloads before review (git-ignored)
```

- Every sound is original. Effects are made in code (`cmd/audiogen`,
  `pkg/sfxsynth`): edit `cmd/audiogen/recipes.go`, then `make audio`.
  Output is byte-identical everywhere.
- Music is composed in code too (`cmd/audiogen/music.go`, `pkg/musicsynth`:
  scores played by synthesized guitar, strings, horn, bass and drums
  through a reverb): edit a score, then `make music`.
- Ambience beds come from ElevenLabs within a 10,000-credit
  budget, in `prompts.json` priority order: one retry per sound at most,
  every take in the ledger. Import approved takes with
  `scripts/audio_import.sh` (loudness, 48 kHz, seamless loops, Vorbis,
  sound-list entry with prompt, credits and license).
- Ids: `ui.*` (interface bus), `sfx.*` (effects), `amb.*` (ambience),
  `music.*` (music). Several files on one id are variants (effects pick
  one at random; music plays them in turn).
- Total audio stays under 20 MB (`make audio-check`).

