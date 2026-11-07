# internal/adapters/inbound/ebiten/hudlayout

## Purpose
The colony HUD's pure layout: region rectangles from the screen size and the regions' wanted sizes. No state, no drawing. Moved in ARCH-003 wave 3.1 (370 lines).

## Owns
- `Layout` (region rectangles) computed by `Compute(sw, sh, st Inputs) Layout`.
- `Inputs` (screen size and each region's wanted size).
- `BoundsOf` and `PerfRect` (the performance overlay's rectangle), `LearnHeaderH`.
- Every HUD size token, in `metrics.go`: each reference size names its row in `.agents/gaps/UI_SCALE_GAP_ANALYSIS.md` and `TestMetricsMatchReference` pins it; "ours" marks a size with no reference measure. Workers read the tokens and never add one (the lead does, UIS-1).

## Depends on
- `gfx` (the `Rect` type); `math`.

## Must not depend on
- Everything else in the ebiten tree that draws or holds state (`hud`, `uikit`, `ports`).

## Contracts
- `Compute` is a pure function: same inputs, same `Layout`.
- Callers (stated as fact from the import lists): `hud`, `architect`, `inspector`, `orders`, `learn`, `devtools`, `render/minimap`.

## Invariants
- Empty rectangles when a region is off (`TestPerfRectEmptyWhenOff`).
- Colonist-bar rows wrap inside the bar at the reference steps (`TestLayoutColonistBarWraps`, `TestBarStepsMatchReference`).
- Commands are never dropped: past the grid's room the last cell stands for the rest (`GridMore`, `TestCommandsNeverDropped`).
- A command's hanging label (`CmdLabelBand`) never lies on the row under it or on a panel beside the grid: rows are `CmdRowPitch` apart (`TestCommandLabelsClearTheNextRow`).
- Messages lie over the colonist bar (UIS-029) and keep only the newest rows that fit above the lower-left stack.
- `BoundsOf` matches the region rectangles that `Compute` returns (`TestBoundsOf`).

## Hot paths
- `Compute` runs on resize and when a region's wanted size changes; callers may cache its result. No allocation (value types).

## Tests
- 2 source files (`layout.go` placement, `metrics.go` tokens), 1 test file.
- Coverage 63 % (ARCH-003).

## Notes
- The root's `perfRect` and `learnHeaderH` are now thin aliases over this package.
