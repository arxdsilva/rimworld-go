# Task Handoff

Task ID: ART-001
Status: complete

## Summary

Wired the CC0 Rgsdev character pack under `assets/` into the Ebitengine adapter. Pawns use full-body idle/walk sprites, ground/rocks tile the map, and selection uses the pack crosshair. Domain state is unchanged: appearance, facing, and animation stay presentation-only.

## Files Changed

- `assets/AGENTS.md`
- `cmd/game/main.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/pack.go`
- `internal/adapters/inbound/ebiten/pack_test.go`
- `internal/adapters/inbound/ebiten/sprites.go`
- `internal/adapters/inbound/ebiten/sprites_test.go`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`

## Public Contracts Changed

None in domain/application. Renderer gained `LoadPack` / `LoadDefaultPack` and exported `TileSize` (64). `cmd/game` accepts `-assets`.

## Invariants / Decisions

- Vendor pack is immutable source; do not hand-edit PNGs.
- Only colonist idle/walk, ground, rocks, and crosshair are decoded (cropped + scaled). Layered body parts, enemies, weapons, bullet/muzzle remain inventoried for later use.
- Sprite assignment is `EntityID` modulo colonist count. Walk vs idle and horizontal flip are inferred from last snapshot position in the adapter. Frame index comes from simulation tick, never written back.
- Pack is side-view; it does not satisfy the long-term top-down art bible. Wave 0 contracts stay open.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/ ./internal/application/game/`
Benchmarks: none.

In-game window was not visually QA'd in this session (native Ebitengine, no browser). Run `go run ./cmd/game` from the repo root.

## Determinism Impact

None. Simulation RNG and positions unchanged.

## Save / Mod Impact

None.

## Art / Visual Impact

First-playable pawns and ground use the vendor pack. Original orthographic art still required.

## Screen / UI Impact

Main colony HUD still debug text, plus a small idle portrait when a pawn is selected.

## Known Limitations

- 2048×2048 sources make first `LoadPack` slow; textures are downscaled to 128/64 px.
- No compositional pawn layers, no enemy/weapon/projectile drawing yet.
- Ground is visual-only (no terrain defs).

## Follow-ups

- Freeze ART Wave 0 and replace this pack with original top-down art.
- Optional: decode remaining extras (bullet/muzzle) when projectiles exist.
