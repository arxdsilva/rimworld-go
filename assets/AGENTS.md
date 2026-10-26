# assets

Presentation-only art. Nothing in this tree is authoritative simulation
state. The renderer and other inbound adapters consume these files;
`internal/domain` must not.

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
- Pawns (ART-025): limbless figures with no walk/work frames. Draw `body`,
  `pants_*`, `shirt_*`, `vest_*`, `hair_*` for the same facing at the same
  anchor; S/E/N are rendered and west is east mirrored by the renderer.
  Attachments are `head` (hair/hats; with the feet it gives the body
  centre held items are placed from), `hold` (the punch trail's point) and
  `back` (a rescued patient). Motion is renderer transforms (ART §27.1).
- Gear (`families/gear.py`): `weapon/rifle_basic`, `knife`, `spear`, `club`
  only, flat and pointing east with `grip`/`support`/`muzzle`|`tip`
  (+ `stock`). ART-027 (2026-10-26, user decision): no tool sprites and no
  slung rifle; the renderer draws a weapon only on duty and never a tool. Animals keep frames: trot
  `walk01..walk08` and a `rest` pose (all animal sets), S/E/N only.
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
│   ├── tools/           # validate_assets.py, contact_sheet.py, compose_scene.py, portrait_cut.py
│   └── raster/ui/       # main_menu_colonist.png, portraits/<key>.png + briefs.md
├── generated/
│   ├── world/<family>/<name>.png (+ _mask.png)   # @2x sprites
│   ├── ui/icons/*.png
│   ├── ui/portraits/<key>.png (+ _thumb.png)   # storyteller busts (make portraits)
│   ├── ui/menu/menu_diorama.png (+ menu_glow_lamp/fire.png)  # main-menu scene
│   ├── contact_sheet.png, prototype_scene*.png, pawn_sheet.png  # review aids
├── metadata/<family>/<name>.yaml   # footprint, origin_px, attachments, tint
│   └── ui_menu/menu_diorama.yaml    # menu scene: canvas, glow layers, fire/lamp px (not a sprite)
└── .gitkeep
```

`source/raster/ui/main_menu_colonist.png` is original title art (cel-shaded
colonist bust with raven, RGBA cutout). It is not a reconstruction of proprietary RimWorld screenshots. The brief that
produced it lives at repo-root `ai-bg-rimworld`.

`source/raster/ui/portraits/{steady,hearth,gambler}.png` are the storyteller
busts (ART-023), painted on a flat magenta field from the prompts in
`briefs.md`. `make portraits` (`tools/portrait_cut.py`, needs Pillow + numpy)
keys the field, inks the fringe, fits the bust bottom-flush into 512×640,
cuts the 120×150 thumbnail and writes `metadata/ui_portrait/`. Never
hand-edit those outputs.

`generated/ui/menu/` is the main-menu background: one scene composed from
the family builders and rendered through the same stage (`stage.place`,
`stage.render_scene`). Edit `source/blender/artlib/scenes/menu.py`, then
`make art-menu`; live, `menu.preview()` writes a quarter-size render.
The house counts as roofed (PLAY-020): inside it there is no sun shading
or sun shadow, and its lamp is a point light (`stage.point_light`) that
shades the furniture toward it; the dusk grade applies outdoors only.

A lamp's `light_radius` metadata is read by the game (PLAY-020): its light
falls off over that many tiles, inside its room or, outdoors, in its line
of sight.

The renderer draws the original set (ART-006); without `metadata/` it
draws plain coloured fallbacks. The old vendor sprite pack was removed in
PLAY-010 (unused assets are deleted, see AGENTS.md). `generated/ingame_capture.png` comes from
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

