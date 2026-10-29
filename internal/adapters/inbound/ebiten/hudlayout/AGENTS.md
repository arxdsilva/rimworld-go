# hudlayout package

## Package Purpose
The colony HUD's pure layout: region rectangles from the screen size and the regions' wanted sizes. No state, no drawing. Moved in ARCH-003 wave 3.1 (370 lines).

## Owned Concepts
- `Layout` (region rectangles) computed by `Compute(sw, sh, st Inputs) Layout`.
- `Inputs` (screen size and each region's wanted size).
- `BoundsOf` and `PerfRect` (the performance overlay's rectangle), `LearnHeaderH`.
- Every HUD metric constant (`HudMargin`, `ResourceRowH`, and the rest).

## Allowed Dependencies
- `gfx` (the `Rect` type).
- Standard library: `math`.

## Forbidden Dependencies
- Everything else in the ebiten tree that draws or holds state (`hud`, `uikit`, `ports`).
- Domain, application, the root package and `cmd/`.

## Public Contracts
- `Compute` is a pure function: same inputs, same `Layout`.
- Callers (stated as fact from the import lists): `hud`, `architect`, `inspector`, `orders`, `learn`, `devtools`, `render/minimap`.

## Invariants
- Empty rectangles when a region is off (`TestPerfRectEmptyWhenOff`).
- Colonist-bar rows wrap inside the bar (`TestLayoutColonistBarWraps`).
- `BoundsOf` matches the region rectangles that `Compute` returns (`TestBoundsOf`).

## Hot Paths
- `Compute` runs on resize and when a region's wanted size changes; callers may cache its result. No allocation (value types).

## Determinism Rules
- Pure function of its inputs. No RNG, no clock, no state.

## Save/Mod Impact
- None.

## Tests
- 1 source file, 1 test file, 3 `Test*` functions.
- Coverage 63 % (ARCH-003).

## Benchmarks
- None.

## Known Design Constraints
- The root's `perfRect` and `learnHeaderH` are now thin aliases over this package.
