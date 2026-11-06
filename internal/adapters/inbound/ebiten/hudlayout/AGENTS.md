# internal/adapters/inbound/ebiten/hudlayout

## Purpose
The colony HUD's pure layout: region rectangles from the screen size and the regions' wanted sizes. No state, no drawing. Moved in ARCH-003 wave 3.1 (370 lines).

## Owns
- `Layout` (region rectangles) computed by `Compute(sw, sh, st Inputs) Layout`.
- `Inputs` (screen size and each region's wanted size).
- `BoundsOf` and `PerfRect` (the performance overlay's rectangle), `LearnHeaderH`.
- Every HUD metric constant (`HudMargin`, `ResourceRowH`, and the rest).

## Depends on
- `gfx` (the `Rect` type); `math`.

## Must not depend on
- Everything else in the ebiten tree that draws or holds state (`hud`, `uikit`, `ports`).

## Contracts
- `Compute` is a pure function: same inputs, same `Layout`.
- Callers (stated as fact from the import lists): `hud`, `architect`, `inspector`, `orders`, `learn`, `devtools`, `render/minimap`.

## Invariants
- Empty rectangles when a region is off (`TestPerfRectEmptyWhenOff`).
- Colonist-bar rows wrap inside the bar (`TestLayoutColonistBarWraps`).
- `BoundsOf` matches the region rectangles that `Compute` returns (`TestBoundsOf`).

## Hot paths
- `Compute` runs on resize and when a region's wanted size changes; callers may cache its result. No allocation (value types).

## Tests
- 1 source file, 1 test file, 3 `Test*` functions.
- Coverage 63 % (ARCH-003).

## Notes
- The root's `perfRect` and `learnHeaderH` are now thin aliases over this package.
