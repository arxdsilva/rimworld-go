# render/creature package

## Package Purpose
Draws pawns and animals: figure layers and poses, the animal body player, held items, work and combat animation, bursts and particles, graves and pyres, the overlays above creatures (hunt marks, chase lines, name plates, health bars), floating text and the attack traces. It reads the frame's `*render.View`, the motion state, the painter and the light, never the renderer (SOLID_GO.md §7). Its root needs (a pawn's look and blood, its visual record, sounds, marks) come through `Host`. ARCH-004 wave 4.3 (agent R5).

## Owned Concepts
- `Set` (93 methods, 72 exported): the whole creature layer. Built over a `Host` of `Figures` + `Marks` + `Sounds`.
- `Host` (embeds `Figures`, `Marks`, `Sounds`); `Figures` (8 methods: `FigureLayers`, `PawnBlood`, `AnimalBlood`, `LayerBlood`, `Visual`, `ColonyCloths`, `ItemSprite`, `StoneItem`); `Marks` (5: `HostileMarker`, `HealthColor`, `TurretMuzzle`, `BuildingAt`, `Spray`); `Sounds` (3: `WorkCue`, `BurstCue`, `TraceSound`).
- `Body` and `FigurePose`: resolve the motion and light type parameters of the shared layers.
- Types `Layer`, `Part`, `Vec`, `Visual`.

## Allowed Dependencies
- Leaves: `art`, `camera`, `gfx`, `uikit`.
- Layers: `render` (the view), `render/light`, `render/motion`, `render/sprite`.
- `application/game` read models; domain `sim/entity`.
- Ebitengine (`ebiten`, `vector`); stdlib (`cmp`, `fmt`, `image`, `image/color`, `math`, `slices`, `strconv`, `strings`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages (`hud`, `architect`, `inspector`, `windows`, `wizard`, the rest).
- `render/world` and the other sibling layers (`env`, `overlay`, `cue`, `minimap`): creature is below the world pipeline, which imports it.

## Public Contracts
- `New` builds `Set` from a `Host` and the frame's `*render.View`.
- `Set` draws in the order `render/world` asks for; the world's pipeline owns the draw order.
- `Host` is the only way a creature reads root data (look, blood, visual record, sounds, marks).

## Invariants
- `Set` has 93 methods, the moved debt budgeted at `render/creature.Set 93` in `internal/arch/solid_budget.txt`; no new method is added without a split.
- Breathing shows on every size (`TestBreathingShowsOnEverySize`); corpses fly when rotting (`TestCorpseFliesWhenRotting`); attack reactions are tested (`TestAttackReactions`).

## Hot Paths
- Per frame, `Set` draws every visible creature: figure layers, poses, held items, overlays. This is the hot loop of the renderer.
- Never pass a pointer to a frame local through an interface method (ARCH-004 finding); escape analysis moves it to the heap. Pass slices or values.
- The `DrawArt500` benchmark in the root is the gate (allocs/op unchanged, draw calls identical).

## Determinism Rules
- Presentation only. Animation time comes from the view's clocks; the package reads no RNG and no wall clock.
- Sounds leave through `Sounds`; the audio device is never touched here.

## Save/Mod Impact
- None.

## Tests
- 14 source files, 6 test files, 17 `Test*` functions; `fakeHost` (16 methods) drives them.
- Coverage 23.8 % (ARCH-004): the draw paths run through the root's tests.

## Benchmarks
- None in the package.

## Known Design Constraints
- `Set` is 93 methods; the seams to split next are: pawn figure (20+), animal body, combat and work animation, bursts and particles, traces and floaters.
- Root keeps `syncAnimals`, `settleVisuals` (over `pawnVisual`), the threat state, `drawPawnArt` and the portraits.
