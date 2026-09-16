# Task Handoff

Task ID: ART-004
Status: complete

## Summary

The post-boot main menu now shows an original cel-shaded colonist bust on the left over the existing warm backdrop. Title and actions sit on a right rail. The PNG was generated from the written `ai-bg-rimworld` brief (colors, pose, raven, rifle), not from the RimWorld colony screenshot and not by decoding the ASCII grid.

## Files Changed

- `assets/source/raster/ui/main_menu_colonist.png`
- `assets/AGENTS.md`
- `internal/adapters/inbound/ebiten/splash.go`
- `internal/adapters/inbound/ebiten/splash_test.go`
- `internal/adapters/inbound/ebiten/screen_menu.go`
- `internal/adapters/inbound/ebiten/app.go`
- `internal/adapters/inbound/ebiten/app_test.go`
- `internal/adapters/inbound/ebiten/AGENTS.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`

## Public Contracts Changed

`AppConfig.LoadSplash` is optional. The composition root may omit it; `NewApp` locates `assets/source/raster/ui/main_menu_colonist.png` and decodes it. A missing or failed splash still shows the menu.

## Invariants / Decisions

- Clean-room: original bust with alpha cutout over the existing menu backdrop. No proprietary screenshot as image-to-image.
- Menu presentation does not start a simulation.
- Buttons stay 300px wide; `x = sw - 40 - 300` so they sit right of `sw/2` at the default 1280 width.
- Splash is UI raster, not a pawn sprite: full PNG decode, no 128px downscale.
- Boot screen layout is unchanged.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/`
Benchmarks: none.

Headless: right-rail `menu.*` buttons; splash PNG decodes as a portrait. Disabled Continue/Plugins still do not navigate.

In-game window was not visually QA'd in this session. Run `go run ./cmd/game`, skip boot, confirm bust left and buttons right.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

One original RGBA PNG under `assets/source/raster/ui/` (padded bust, keyed background). Vendor pack unchanged. §34 prototype gate still blocks bulk gameplay-sprite production.

## Screen / UI Impact

Main Menu (§2): alpha splash left, centered title, actions right. Wave UI-1 Main Menu note updated. Background item marked `[-]`.

## Known Limitations

- Splash is a still image, not an animated colony scene.
- A thin dark-red cel fringe may remain from chroma-keying the generator's pink field.
- No metadata sidecar yet.

## Follow-ups

- Optional idle animation later, still without a live sim.
