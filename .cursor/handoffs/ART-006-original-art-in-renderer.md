# Task Handoff

Task ID: ART-006
Status: complete

## Summary

The ebiten renderer now draws the original generated art (ART-005)
instead of the vendor pack and flat rectangles: soil terrain variants
with grass decals, 16-mask autotiled walls, blueprint + scaffold frames,
a 1×1 cot for beds, berry bushes (± harvestable), wood/food items,
stockpile/blueprint/designation/selection overlays, and layered colonists
(body, pants, shirt, optional vest, one of 3 hairstyles) facing their last
step with a walk bob and EntityID-stable tints. HUD portraits use the same
layers. A dev capture mode renders a scripted colony to PNG from the real
renderer.

## Files Changed

- `internal/adapters/inbound/ebiten/art.go` (new: metadata/tint parsing,
  catalog, tint baking, wall mask, facing, pawn look, placement)
- `internal/adapters/inbound/ebiten/draw_art.go` (new: draw path)
- `internal/adapters/inbound/ebiten/capture.go` (new: `-capture`)
- `internal/adapters/inbound/ebiten/art_test.go` (new)
- `internal/adapters/inbound/ebiten/{renderer,app,hud,app_test}.go`
- `internal/adapters/inbound/ebiten/AGENTS.md`
- `cmd/game/main.go` (`-capture`, `-capture-ticks`, `-capture-zoom`)
- `assets/source/blender/artlib/families/{furniture,terrain}.py` (cot;
  more terrain variety), regenerated `assets/generated`, `assets/metadata`
- `Makefile` (`art-capture`), `assets/AGENTS.md`,
  `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`

## Public Contracts Changed

`AppConfig` gains `LoadArt` and `Capture`; `App.CaptureErr()`. No
simulation, command, or save contract changed.

## Invariants / Decisions

- Presentation-only: art, tints, facing, bob and look never reach the
  domain; look/variant choice is hash(EntityID / tile), not RNG.
- Domain beds are single-cell, so the in-game bed is the 1×1 cot; the
  1×2 bed waits for building footprints.
- The domain has no terrain types, so only soil is drawn (no stone
  patches that would imply unbuildable ground).
- Capture mode uses only Enqueue/Step and is deterministic per seed.

## Verification

- `gofmt -l .` clean, `go vet ./...` clean, `go test -race ./...` pass.
- `staticcheck`: only pre-existing SA4000 in `pack_test.go`.
- `make art-check`: 108 sprites, 0 errors.
- `TestGeneratedArtCoversRenderer` checks every key the draw path uses.
- In-game: `assets/generated/ingame_capture.png` (seed 1, 5400 ticks).

## Determinism Impact

None (presentation only; capture uses ordinary commands).

## Save / Mod Impact

None.

## Art / Visual Impact

The colony view now matches the signed-off prototype style.

## Screen / UI Impact

Colony viewport and HUD colonist portraits. Grid lines are no longer
drawn when original art is loaded.

## Known Limitations / Next

- No terrain types, floors, doors, trees, crops or furniture in the domain
  yet; their sprites are ready.
- No sleeping/downed posture, carried-item presentation, or far-zoom LOD.
- Tint bake is per (sprite, tint) on first draw; fine for small colonies,
  consider an atlas + shader tint for large ones.
- Vendor pack fallback can be removed once nobody needs it.
