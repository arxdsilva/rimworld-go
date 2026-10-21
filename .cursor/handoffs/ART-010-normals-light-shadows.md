# ART-010 Normal maps, sun and moon light, moving shadows

## Credit
Normal maps are generated with a Go port of
[spritefusion-sprite-normals](https://github.com/Hugo-Dz/spritefusion-sprite-normals)
by Hugo Duprez (MIT, (c) 2026 Hugo Duprez), studied at commit `d14b067`.
- The licence is in `third_party/spritefusion-sprite-normals/LICENSE`.
- The notice is in `THIRD_PARTY_NOTICES.md`.
- Every `pkg/spritenormals` file carries the attribution header.

## What changed
- **`pkg/spritenormals`:** a faithful port of upstream's relief pipeline.
  - 8-connected islands, padded.
  - Oklab lightness, centred.
  - An exact Felzenszwalb distance transform turned into a log bevel dome.
  - Separable Gaussian and masked Gaussian.
  - A Sobel correlation to OpenGL normals.
  - Divergence: float64 transcendental functions rounded to float32, so a byte may occasionally differ by 1.
- **`cmd/normalgen`, `make normals`** (runs after Blender in `make art`):
  - writes `<sprite>_n.png` using upstream's defaults, with radius 8, softness 2.3 and detail 1.2 for our @2x art
  - parallel; 3165 sprites in about 9 s
  - skips masks, UI, terrain and floors (lit flat)
- **Art:**
  - Every world family was re-rendered with no baked drop shadow (`post.process` and `render_asset` now default to `shadow=False`).
  - Each sprite has a `shadow:` caster block (`kind` blob/box/none, `w`, `h`, `d` in tiles) measured from the staged geometry. Pawns and animals are measured upright; structures are clamped to their footprint.
  - New sprites: `overlay/shadow_blob`, `shadow_box`, `roof_built`, `roof_natural`, `roof_plan`, `roof_remove`. New icons: `roof_build`, `roof_remove`, `overlay_roof`.
  - The validator requires the normal map (same size and alpha) and the shadow block by default (`--no-light` skips them). It rejects normal maps on terrain and floors.
- **Light** (`light.go`, `lit.kage`), from the sim clock (`WorldView.Hour`, `DayOfYear`), so it's deterministic:
  - **Sun:** rises east at 06:00, crosses the south and sets west at 18:00. Peak elevation: Summer 65°, Spring/Fall 50°, Winter 32°.
  - **Sun colour:** warm keys at dawn and dusk; noon is calibrated so a flat normal comes out at about 1.0 (the old palette).
  - **Moon:** cool, intensity 0.22, peaking at 40°.
  - **Ambient:** sky ambient under open sky, indoor ambient under roofs.
  - **Lamps:** warm lamp light in lit rooms and within 3 tiles of a burning campfire. Wall tiles take sky from their sides and lamp light from all 8 neighbours.
  - **Weather** dims the sun and moon and greys the ambient.
- **Draw path:**
  - Every world sprite goes through `drawLit`: `DrawTrianglesShader32`, with the sprite and its normal map packed into one image.
  - Per-vertex values: Custom0 = sky, Custom1 = lamp, Custom2 = normal offset (0 = flat).
  - Uniforms change once per frame, so draws batch. Bilinear sampling happens in the shader.
  - Terrain, floors, overlays, particles, UI and HUD are lit flat or unlit.
- **Night:** the global night veil (`drawDaylightRooms`) is retired. `skyOverlay` remains only for the non-art fallback path.
- **Shadows** (`shadow.go`):
  - Every caster on open-sky ground draws into one offscreen layer with max blending. Roofed tiles are cleared from the layer, which is composited once in (18, 20, 30).
  - Blobs are used for creatures, plants and items; buildings get swept boxes with soft edges.
  - Direction is away from the active light. Length is `clamp(h·cot el, 0.25h, 3)`.
  - Alpha:

    | Condition | Alpha |
    |---|---|
    | Noon | 0.38 |
    | Low sun | at least ~0.24 |
    | First and last 30 min of daylight | fades in and out |
    | Moonlight | 0.14 |
    | Storm or blizzard | × (1 − 0.6·intensity) |

- **Interiors and weather** (`roof_draw.go`, `environment.go`):
  - soft eaves inside walls bounding roofed tiles
  - rain and snow never land on roofed tiles; puddles, frost, snow cover and snow-buried grass are skipped there
  - collapse dust, emitted once per stamp

## Verification
- **Tests:**
  - `light_test.go` (17 tests, plus `BenchmarkDrawArtLitAB`)
  - `shadow_test.go`, `roof_draw_test.go`, `environment_test.go` additions
  - `spritenormals`: 100% coverage, including an EDT brute-force test ported from upstream, plus an independent float64 reference implementation. `cargo` was unavailable, so goldens are self-consistent.
  - `normalgen`: 85%.
- **Benchmarks:**

  | Benchmark | Before | After | Gate |
  |---|---|---|---|
  | DrawArt | 2.0 ms | 2.5 ms (+25%) | +35% |
  | DrawPlants | 0.63 ms | 0.72 ms | 1.2 ms |
  | Shadows | — | 0.32 ms | 0.6 ms |

- `make art-check`: 3165 sprites, 0 errors.
- **Reviewed by eye:**
  - the late game at 07, 12, 17 and 22: warm dawn, palette noon, cool readable night
  - sunny 7/12/16: tree shadows swing west → north → east
  - roofs at 10:00, with the overlay, and at 21:00: the roofed room is warm and fire-lit; the unroofed one is dark, with its fire doused by snow; the porch is shaded
- **README media:** regenerated, with new `daycycle` and `roofs` clips.

## Known limitations
- No mipmaps on the shader path, so heavy zoom-out shimmers like before.
- Pawn layers each carry their own shadow block; only the body is a sensible caster.
- Light levels have no gameplay effect yet (work speed, plant growth).
- The ebiten package's coverage (62%) is below the 80% bar. The new files' own functions are mostly 85–100%.
