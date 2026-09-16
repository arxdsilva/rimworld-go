# Task Handoff

Task ID: TIME-005
Status: complete

## Summary

Colony viewport no longer applies a warm sky grade during morning or afternoon. Dusk and night keep the existing veil. The main-menu colonist PNG was cropped so the bust meets the bottom edge (20 empty transparent rows removed).

## Files Changed

- `internal/adapters/inbound/ebiten/daylight.go`
- `internal/adapters/inbound/ebiten/daylight_test.go`
- `internal/adapters/inbound/ebiten/splash_test.go`
- `assets/source/raster/ui/main_menu_colonist.png`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`

## Public Contracts Changed

None. `skyOverlay` remains adapter-private.

## Invariants / Decisions

- Day periods (`Morning`, `Afternoon`) draw with alpha 0. Evening and night still use outdoor light × `skyOverlayMaxAlpha`.
- Domain `clock` and `lighting` are unchanged.
- Splash layout constants are unchanged; `drawMenuSplash` still bottom-aligns the PNG.
- Source PNG only; no generated atlas patch.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/`

## Determinism Impact

None. Presentation-only.

## Save / Mod Impact

None.

## Art / Visual Impact

Playable map is untinted from 06:00–16:59. Menu bust sits on the window bottom with no transparent gap under the torso. Side alpha cutout kept.

## Screen / UI Impact

Main menu splash crop only. Colony HUD unchanged.

## Known Limitations

- No indoor/roofed darkness or lamps.
- Overlay is still a fullscreen grade, not per-tile lighting.

## Follow-ups

- Roof/indoor and artificial lights in `world/lighting`.
