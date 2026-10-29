# globemath package

## Package Purpose
The pure geometry of the landing-site globe: quaternions, the orbit camera, the perspective projection, ray-sphere picking and the horizon test. It has no Ebitengine dependency, so picking and camera rules are unit-tested headless. Moved in ARCH-003 wave 3.1 alongside `globe`.

## Owned Concepts
- `Vec3` (`[3]float64`), `Quat` (`AxisAngle`, `Identity`, conjugate, rotate), `Cross`, `Norm`.
- `Camera` (21 methods, 20 exported) built by `NewCamera()`: orbit, zoom and orientation. `Dist` is measured in planet radii.
- `Viewport` (`NewViewport(x, y, w, h, panX, panY)`) and the perspective projection.
- Picking: ray-sphere intersection and the horizon test.

## Allowed Dependencies
- Standard library only: `math`. (`math/rand` appears in the test file only.)

## Forbidden Dependencies
- Ebitengine, `gfx`, `art` and every other ebiten package: this package is headless by design.
- Domain, application, the root package and `cmd/`.

## Public Contracts
- Conventions are documented in the package comment: world space is the planet's (unit sphere, north +Z); view space has +X right, +Y up, +Z toward the viewer; the eye sits at `(0, 0, Dist)` looking down -Z.
- Picking returns a point on the sphere or a miss; the horizon test returns visibility of a point.

## Invariants
- `Vec3` is an array, copied by value; no pointer state.
- `Camera` methods do not allocate.
- Round trips hold: `TestPickRoundTrip`, `TestQuatRotateAndConj`, `TestOrientationCentresAndNorthIsUp`, `TestPickMissesOutsideTheLimb`.

## Hot Paths
- `Camera` orbit and projection run per frame in `globe`; picking runs per mouse move over the globe. Pure math, no allocation, no benchmark in this package.

## Determinism Rules
- Pure functions of their inputs; no RNG, no clock. Tests use fixed inputs.

## Save/Mod Impact
- None.

## Tests
- 1 source file, 1 test file, 13 `Test*` functions. Headless: no Ebitengine device.
- Coverage: not recorded in the handoffs.

## Benchmarks
- None.

## Known Design Constraints
- `Camera` has 21 methods, under the 40-method limit; if picking grows, split it into a projection type rather than adding methods.
