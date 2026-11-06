# ART-007 Smooth, realistic movement and body animation

## What changed
- **Spec**: ART §27.1 (normative): frame sets, clocks, path/gait rules,
  blocked presentation.
- **Art** (Blender, `pawn.py`/`animal.py`): pawn walk is one 12-pose gait
  cycle (`walk01..walk12`: heel contact, loading, mid-stance, terminal
  stance, pre-swing, swing, mirrored) for body, pants, shirt, vest, parka
  and duster; work is a 6-pose smoothstep swing (`work1..work6`), now also
  for pants and vest; animals trot in 8 poses (`walk01..walk08`). Every body
  frame has a `head` attachment beside `hand`. Pawn PNGs 304 -> 792, animal
  40 -> 72; `make art FAMILIES=pawn` ~1m40s. The validator checks the sets
  and the head anchor.
- **Motion** (`motion.go`): `pawnMotion`/`animalMotion` -> `moveState`
  (position, velocity, speed, stride, walk blend, facing) from the snapshot's
  `StepTick`/`StepTicks`/`NextX,NextY`/`HasNext`/`BlockedBy` (SIM-014).
  Exact sim step timing, quadratic curve through tile centres at corners,
  smoothstep start/stop (a first/last step of a longer walk peaks at 1.33x
  cruise so the step still lasts its sim duration), facing from a two-step
  travel window with 20% hysteresis, north/south reversals through a side
  facing for 4 frames. `interpWorld`/`interpAnimal` delegate to it.
- **Clocks**: `animT()` = sim tick plus the elapsed fraction of the current
  update (wall clock, adapter-only; 0 in captures) and runs one update behind
  so only steps that happened are drawn; drives movement, walk cycle, work
  swings and combat reactions. `present` drives ambient effects.
- **Body** (`anim.go`, `pawn_anim.go`): walk/trot frame = stride / 1.6 tiles
  x 12 (8); walk blend in/out over 0.3 tiles; bob peaks on footfalls (frames
  01, 07) scaled by speed and blend; sway (N/S) and lean (E/W) with speed;
  hair and hats offset by the `head` delta; tools on the per-frame `hand`;
  work on `animT` (freezes when paused, 3x at 3x); bed entry and carry lift
  ease over 12 frames; footstep dust every 0.8 tiles at the feet; recoil,
  lunge and flash visible >= 8 frames; blocked creatures face the blocker;
  fleeing animals bob 1.6x.
- **Camera** (`camera_smooth.go`): pan accelerates and coasts to a stop
  (~200 ms); zoom eases (~150 ms) anchored at the cursor. Captures set the
  camera directly.
- **Inspector**: "waiting for <name>" on the colonist status line while
  `BlockedBy` is set.
- **Dev**: `-capture-speed`, `-scenario corridor|crowd`,
  `-capture-ui corridor|crowd` (SIM-014).

## Verification
- Unit: `motion_test.go` (constant speed at 1/3/6 ticks per frame, diagonal
  vs straight speed ratio 1.01, continuous corners, ease start/stop, blocked
  stop, zig-zag facing, N/S turn, pause freeze, capture alpha 0, animal pace
  change, `TestRealSimStraightWalkConstantSpeed` on a real colony),
  `anim_test.go` (stride walk cycle, blend, bob at contact, sway/lean, hair
  follows head, six-frame work on animT, bed/carry ease, dust, combat min
  frames, trot), `camera_smooth_test.go`, `inspector_test.go`,
  `TestGeneratedArtCoversRenderer` against the real art. ebiten coverage
  36.5% -> 43.4%.
- Captured (fixed camera, `-scenario corridor`, 240 frames, head x per
  frame): before, linear lerp at 4.2 px/frame with instant reversals and
  pawns passing through each other; after, steady cruise with the same
  frame-to-frame smoothness (jerk p90 0.16 vs 0.13-0.18 of mean speed),
  eased stops and starts at the meeting point, one pawn backing out, no
  overlap. A per-step walk-blend dip found this way was fixed (792e869).
- README media regenerated with `make screenshots clips`.

## Known limitations / follow-ups
- Lean-back on deceleration is a heuristic (moveState has no acceleration).
- Bed exit and carry release are not eased.
- No far-zoom animation LOD (ART §27, UI/UX "Far-zoom detail").
- No aiming pose; mouse edge/drag pan not implemented.
- Frame-based head-x measurement shows small one-frame offsets when a pawn
  turns (the sprite's head shape changes with facing); not a motion error.
