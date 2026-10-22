# ART-012 Realistic pawn and animal shadows

## What changed
- **Silhouettes instead of blobs:** pawns and animals no longer cast textured blobs. They cast **their own silhouette**: every pawn layer (body, apparel, hair, held weapon or tool, carried item), and the animal's current frame. Walking legs, swings and lunges all move in the shadow.
- **Cast mode:**
  - `drawLit` routes to `castSilhouette` when `r.cast` is set.
  - Figure draws are pure: `drawPawnFigure` (`pawn_anim.go`) and `drawAnimalFigure` (`animal_figure.go`).
  - A small Kage shader draws white silhouettes with height in R.
- **Projection** (`castProjection`), affine about the feet pivot. Height above the feet goes along `ShadowDir·m`, where `m = ShadowLen(h)/h` for the sun and moon and `tip/h` for point lights. Width runs across the shadow. `κ = h·artTilePx/opaquePx` is calibrated per sprite from its alpha bounds. Lying poses use κ × 0.25.
- **Blur:** `shadow_blur.kage` is a contact-hardening blur. The radius grows with the encoded height and with a low sun, so shadows are crisp at the feet and soft at the tip.
- **Contact pools:** under every creature, also indoors, at night and in storms.
- **Point lights:**
  - Burning campfires (H 0.5, reach 4.5) and lamps in lit rooms (H 1.6, reach 6); the 2 nearest per creature.
  - Length is `d·h/(H−h)`, clamped `[0.3h, 3]`. With pawns taller than the fire, campfire shadows are always 3 tiles.
  - Alpha is `0.30·falloff·(1−daylight)`, with campfire flicker.
- **Physical shadow strength** (`light.go` `shadowStrength`): `0.95·S/(A+S)`, the light's share of flat ground's light.
  - Floor 0.34 (moon 0.12), scaled by the weather. Cap 0.62 (moon 0.30).
  - Summer noon is about 0.45 (was 0.38); dawn and dusk are 0.34 (was 0.24).
  - Rain and storms fade shadows through the light itself.
- **Race fix:** all debug text is serialised through `debugPrintAt`. Ebiten's DebugPrint glyph cache raced between parallel UI tests.

## Verification
- **Tests:** projection, the noon/dawn/dusk cases, lying poses, point lights, indoor and night contact, purity of figure draws, routing, height encoding, the blur shader and calibration; `TestShadowFields` for the physical strength.
- **Benchmarks:**

  | Benchmark | Result | Budget |
  |---|---|---|
  | Shadows | 0.44–0.57 ms | 0.9 |
  | CreatureShadows | 0.17–0.23 ms | 0.5 |

  DrawArt is unchanged within noise (about 2.3–2.7 ms).
- **Captures reviewed:**
  - walkers at 07, 12 and 17
  - a drafted armed line at dawn
  - a campfire room at night
  - a layer dump confirming legs and weapons in the silhouettes

## Known limitations
- No lying pose for downed animals; `AnimalView` has none.
- One shadow layer: point lights and contact pools are drawn after the roof clear.
- Trees still cast swept blobs; the same machinery could take them on later.
