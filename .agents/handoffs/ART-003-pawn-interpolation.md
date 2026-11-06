# Task Handoff

Task ID: ART-003
Status: complete

## Summary

Pawn sprites (and their selection marker) now lerp from the previous tile to the current snapshot cell over the presentation wait between ticks. Pause freezes mid-stride. Domain positions stay discrete.

## Files Changed

- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/renderer_test.go`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`

## Public Contracts Changed

None.

## Invariants / Decisions

- Interpolation is adapter-only. `Snapshot()` coordinates remain integer tiles.
- After a hop, origin is the previous logical cell; a later tick on the same cell settles origin so the next idle frame does not snap backward.
- Click-select prefers the interpolated cell, then the logical tile.
- Walk plays while in transit (and still uses the existing sustain window).

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/`

In-game window was not visually QA'd in this session. Run `go run ./cmd/game` at speed 1.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

Movement reads as a slide across one tile per tick instead of a teleport.

## Screen / UI Impact

Selection crosshair follows the interpolated sprite.

## Known Limitations

- No easing; linear lerp only.
- Very-fast still covers a tile every 2 frames.

## Follow-ups

- Domain movement that occupies multiple ticks per tile.
- Optional ease-in/out if linear still feels mechanical.
