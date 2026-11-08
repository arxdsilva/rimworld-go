# internal/adapters/inbound/ebiten/render/motion

## Purpose
The renderer's per-creature motion: tracks interpolated between simulation ticks, the presentation animation clock, poses, gaits and easing. It is pure math: nothing here draws or reaches the `Renderer`, and nothing feeds back into the simulation (ART §27). Layers read the clocks from a `*render.View` (`AnimT`, `Alpha`, `Present`) and sample a creature with `State.MotionOf` at `View.AnimT`. ARCH-004 wave 4.1 (agent R1).

## Owns
- `State[B]` (11 methods) and `Track[B]` (14 methods): the generic tracks and the per-creature state. `B` is the animal body player type; the root type is resolved by `render/creature.Body`.
- `MoveState`, `MoView` (`PawnMoView`, `AnimalMoView` build one from a read model), `Pose` (`LerpPose`, `WalkPose`), `Vec`.
- Gait, bob, sway, lean and easing functions.

## Depends on
- Leaves: `art` (the facing type `art.Facing`).
- `application/game` read models (`PawnView`, `AnimalView`); domain `sim/clock`, `sim/entity`.
- `pkg/mathx` (`Abs`), `pkg/randx` (`Mix64`).

## Must not depend on
- Every `render/*` package and every feature package: motion sits below all layers.
- Any draw call. The package has no `*ebiten.Image` drawing.

## Contracts
- `State[B].MotionOf(id)` returns a `MoveState` at `View.AnimT`; `Track[B]` interpolates between ticks.
- `WalkPose(base, facing, m)` and `LerpPose(a, b, k)` are pure.
- `State[B].Pace(tick, dt, nominal, tps)` (`pace.go`, GAR-091) is the drawn clock. App.Draw calls it after `SetAlpha` with the seconds since the previous Draw; `AnimT(tick)` then returns it. It advances by the measured tick rate and pulls toward `Tick - (1-Alpha)*Stepped`. It never goes back, never passes the tick, and never trails it by more than two Updates. Captures never call it, so their `AnimT` stays the deterministic interpolation point. The wall clock stays in the root; this package only receives `dt`.

## Invariants
- Bob peaks at contact (`TestBobPeaksAtContact`); walk bob follows the stride (`TestWalkBobFollowsStride`); idle blends to zero (`TestWalkBlendIdle`); sway and lean scale with speed (`TestSwayAndLeanScaleWithSpeed`).
- No value changes the simulation (ART §27). Time is the view's animation clock (`AnimT`), not the wall clock.

## Hot paths
- `MotionOf` and `WalkPose` run once per visible creature per frame. Value types, no allocation claim.

## Tests
- 3 source files, 3 test files, 13 `Test*` functions. The root's `TestWalkSmoothWithZeroOrTwoUpdates` drives the whole Update and Draw schedule at 60, 59, 53 and 42 fps.
- Coverage 61.8 % (ARCH-004, 1,038 lines).

## Notes
- `B` remains a type parameter: the root's animal body player cannot be imported here. The creature layer's `Body` resolves it. Removing the parameter needs the body type moved first.
- `animState`, particles, bursts and the sprite-pose draws stay in the root `anim.go` for `render/creature`.
