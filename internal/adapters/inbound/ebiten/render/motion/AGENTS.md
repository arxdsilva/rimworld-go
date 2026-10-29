# render/motion package

## Package Purpose
The renderer's per-creature motion: tracks interpolated between simulation ticks, the presentation animation clock, poses, gaits and easing. It is pure math: nothing here draws or reaches the `Renderer`, and nothing feeds back into the simulation (ART §27). Layers read the clocks from a `*render.View` (`AnimT`, `Alpha`, `Present`) and sample a creature with `State.MotionOf` at `View.AnimT`. ARCH-004 wave 4.1 (agent R1).

## Owned Concepts
- `State[B]` (9 methods) and `Track[B]` (14 methods): the generic tracks and the per-creature state. `B` is the animal body player type; the root type is resolved by `render/creature.Body`.
- `MoveState`, `MoView` (`PawnMoView`, `AnimalMoView` build one from a read model), `Pose` (`LerpPose`, `WalkPose`), `Vec`.
- Gait, bob, sway, lean and easing functions.

## Allowed Dependencies
- Leaves: `art` (the facing type `art.Facing`).
- `application/game` read models (`PawnView`, `AnimalView`); domain `sim/clock`, `sim/entity`.
- Ebitengine (`ebiten`); stdlib `fmt`, `math`.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Every `render/*` package and every feature package: motion sits below all layers.
- Any draw call. The package has no `*ebiten.Image` drawing.

## Public Contracts
- `State[B].MotionOf(id)` returns a `MoveState` at `View.AnimT`; `Track[B]` interpolates between ticks.
- `WalkPose(base, facing, m)` and `LerpPose(a, b, k)` are pure.

## Invariants
- Bob peaks at contact (`TestBobPeaksAtContact`); walk bob follows the stride (`TestWalkBobFollowsStride`); idle blends to zero (`TestWalkBlendIdle`); sway and lean scale with speed (`TestSwayAndLeanScaleWithSpeed`).
- No value changes the simulation (ART §27).

## Hot Paths
- `MotionOf` and `WalkPose` run once per visible creature per frame. Value types, no allocation claim; no benchmark in the package.

## Determinism Rules
- Presentation only. Time is the view's animation clock (`AnimT`), not the wall clock. The package never reads the simulation's RNG.

## Save/Mod Impact
- None.

## Tests
- 2 source files, 2 test files, 9 `Test*` functions.
- Coverage 61.8 % (ARCH-004, 1,038 lines).

## Benchmarks
- None.

## Known Design Constraints
- `B` remains a type parameter: the root's animal body player cannot be imported here. The creature layer's `Body` resolves it. Removing the parameter needs the body type moved first.
- `animState`, particles, bursts and the sprite-pose draws stay in the root `anim.go` for `render/creature`.
