# Blender art source

Everything under `assets/generated/world` and `assets/metadata` is
rendered from this folder. See `assets/AGENTS.md` for the rules and the
ART spec (`.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`) for intent.

```text
build.py                 entry: blender -b --factory-startup -P build.py -- [family ...]
template.blend           canonical stage, saved by a full build (open it to inspect, never render from it by hand)
artlib/contracts.py      frozen Wave 0 numbers: scale, shear, figure lean, light, outline, shadow
artlib/stage.py          camera, light, toon materials, geometry helpers, render + export
artlib/post.py           numpy post-pass: chromatic outline, contact shadow
artlib/palette.py        reads palettes/materials.yaml
artlib/meta.py           writes metadata/<family>/<name>.yaml
artlib/families/         terrain, structure, furniture, plant, item, pawn (+ weapon), overlay (+ UI icons)
```

Live iteration with Blender MCP: in Blender's Python,
`p = ".../build.py"; exec(open(p).read(), {"__file__": p, "__name__": "rwg_live"}); build(["furniture"])`.
Use the Blender version that wrote `template.blend` (5.2); older versions
name the EEVEE engine differently and shade differently.

Scenes (`artlib/scenes/`) compose many builders into one render, through
the same rig: `stage.place(builder, x, y)` builds a piece alone (inside
`stage.isolate()`, with self-rendering builders muted) and moves it to a
tile; `stage.render_scene(...)` shears, frames and renders the whole
stage. `make art-menu` (or `build(["menu"])`) renders the main-menu
diorama; `menu.preview()` writes a quarter-size check render.

Adding an asset: write a builder in the right family using `stage.box/cyl/
ellipsoid/blob/prism` and palette material names, call
`stage.render_asset(family, name, footprint=(w, h), zmax=..., extra={...})`,
then `make art && make art-preview` and look at the contact sheet and the
prototype scene before committing.
