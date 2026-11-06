# internal/adapters/inbound/ebiten/render/env

## Purpose
The colony view's ground cover and weather: grass tufts and the grass batch, crop furrows and the crop batch, see-through tree canopies, water edges and the water shader, puddles, the snow cache and veil, cave plants, map features, wind and contact springs, the weather drops and lightning. Its one type, `Layer`, reads the frame's `*render.View`, the sprite painter and the light layer, never the renderer. What it needs from the renderer's chunk index and ground pipeline comes in through small interfaces. ARCH-004 wave 4.3 (agent R4).

## Owns
- `Layer` (29 methods, 22 exported) embeds `Sky` (18 methods: wind, weather, plant pose, cull). Holds `Water`, `SnowC`, `Trees`, `Grass`, `Crops`, `Keys`.
- `drawBatch` (12 methods): the batched draw helper shared by the grass and crop batches.
- Interfaces (deps.go): `Chunks` (4: `ChunkView`, `PlantsOn`, `FrameLists`, `Current`), `Stamp` (4: `DrawBakedColor`, `DrawChunkFlat`, `DrawTile`, `DrawSpritePose`), `Movers` (3: `InView`, `PawnMotion`, `AnimalMotion`).
- `Lists` and `ListsOf`: the frame's chunk lists passed by value.
- Five art-key tables that were `init()`-filled in the root are literal vars.

## Depends on
- Leaves: `art`, `camera`, `gfx`.
- Layers: `render` (view), `render/light`, `render/motion`, `render/sprite`.
- `application/game` read models; domain `sim/clock`, `sim/entity`, `world/roof`.
- `platform/config` (settings read).
- `embed` (the water shader); `pkg/randx` (`Mix64`).

## Must not depend on
- Siblings `render/world`, `render/overlay`, `render/creature`, `render/cue`, `render/minimap` (`world` imports them).

## Contracts
- `Layer` is built by the world pipeline and drawn once per frame in the world's order.
- The three interfaces are consumer-owned: the root's chunk index and the sprite stamp satisfy them.

## Invariants
- Crop units are deterministic and row-aligned (`TestCropUnitsDeterministic`, `TestCropUnitsRowAlignment`); mirror variants trim (`TestCropUnitsMirrorVariantTrim`).
- Grass and plants never have an outline (the `art` package test `TestPlantRecipesHaveNoOutline` guards the recipes).
- `Layer` never stores the simulation; it reads `*render.View` each frame. Weather and wind come from the read model; placement randomness is presentation-side.

## Hot paths
- Grass, crop and canopy batches run per frame over the visible chunks. `FrameLists` passes the lists by value (ARCH-004 finding: a pointer to a frame local through an interface moved `w` to the heap; allocs stayed at 3,828 until fixed to 3,826). The root's `DrawArt500` and `ChunkLayer500` measure it.

## Tests
- 13 source files, 5 test files, 35 `Test*` functions.
- Coverage 79.5 % (ARCH-004).

## Notes
- The root's `envState` embeds `Layer` and keeps the shadow, roof, home and snow-clear states until their owners move.
- Dust, leaves and the weather HUD label stay root.
- ARCH-004 recorded 15 files for this package; the tree has 13 source files and 5 test files.
