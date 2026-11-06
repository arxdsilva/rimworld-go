# Task Handoff

Task ID: PLAY-011
Status: complete

## Summary

The 1x1 cot is gone; every sleeping place is bed-sized.

- **Sleeping spot** (replaces the cot):
  - A 1x2 marking painted on the floor, 2x1 when rotated (Q/E).
  - Free and instant: designating it places it, with no frame and no hauling. Deconstructing erases it on the spot.
  - Mood: sleeping on it counts as sleeping on the ground. For rescue, doctoring, claiming and healing it counts as a bed.
  - The sleeper lies along it with the head on the painted pillow. On north-south spots the figure is drawn flatter.
- **Bed:** the 1x2 built upgrade, with the blanket overlay.
- **Sleep cue:** floating "z"s over every sleeping colonist.
- **Construction:** `Kind.Instant()` marks no-cost, no-work kinds. `placeFrameStuff` completes them at once and `markDeconstruct` erases them (collected, then removed after the loop).

## Public Contracts Changed

- construction `Cot` is renamed `SleepingSpot` ("sleeping spot"): Size 1x2, Rotatable, Wood 0, Work 0. Adds `Kind.Instant()`.
- Art: `overlay/sleeping_spot_<facing>` (bed-sized canvases) and `ui_icon/sleeping_spot`. The `furniture/bed_cot_*` sprites, blankets and the cot icon are retired (moved to `_to_delete/retired_cot`).
- Architect: the furniture tool "Cot" is now "Sleeping spot".

## Verification

- `go test -race ./...`, vet and staticcheck are clean.
- New test: a spot is placed instantly without a frame, a tired colonist sleeps on it, and deconstruct erases it.
- Asset validator: 343 sprites, 0 errors.
- Night captures checked: a spot in each facing (south, west, north) and the bed, each with a sleeper.

## Follow-up

- A 2x2 double bed needs two sleepers per bed (multi-claimant beds); not built yet.
