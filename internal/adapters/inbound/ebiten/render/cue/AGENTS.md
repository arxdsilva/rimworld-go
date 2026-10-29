# render/cue package

## Package Purpose
The sound cues the world raises: one-shot cues from world diffs, animal calls and hurts, arrivals, thunder, the machine ambience and what is on screen. The cues are handed to the App's sound director each frame. It reads the frame's `*render.View`, never the renderer or the audio device; the director in the root decides what is heard. ARCH-004 wave 4.3 (agent R7).

## Owned Concepts
- `Bus` (15 methods, 13 exported) built by `New() *Bus`: `Cue`, `Global`, `Take`, `ListenWorld`, `ListenAnimals`, `ListenArrivals`, `ListenThunder`, `Machines`, `WaterOnScreen`, `Audible`, `TraceSound`.
- `Cue` (one queued sound), `Voice` (an animal's voice group entry), `Loop` (a running ambience loop), `Memo` (the last-seen world state for diffs).
- `Host` (6 methods): `Terrain`, `IsWater`, `PlantsOn`, `Chopping`, `AnimalAt`, `ListenCaskets`.
- `animal_sounds.go` keeps its name: `cmd/audiogen` reads `VoiceGroups` from it as source (a text read, not an import).

## Allowed Dependencies
- Leaves: `art`, `camera`.
- Layer: `render` (the view).
- `application/game` read models; domain `building/construction`, `building/power`, `sim/clock`, `sim/entity`.
- Standard library: `math`, `sort`, `strconv`, `strings`, `testing`.

## Forbidden Dependencies
- The ebiten root package and `cmd/` (the audiogen reader is a file read, not an import).
- Feature packages and `render/world` and the other layer siblings.
- The audio device: no Ebitengine audio import; `internal/adapters/outbound/ebitenaudio` is reached only through the root's director.

## Public Contracts
- `Bus.Take` returns and clears the cues queued this frame; the root's director consumes them.
- `Bus.Audible` and `WaterOnScreen` answer what is on screen, so the director can drop off-screen cues.

## Invariants
- Every roster voice has a group (`TestEveryRosterVoiceHasAGroup`); the fallback chain resolves (`TestFallbackChain`).
- Call intervals scale and rate caps hold (`TestCallIntervalScales`, `TestCallRateCaps`).

## Hot Paths
- `Bus` runs per frame over the world diff and the visible creatures; `Memo` holds the last-seen state the diff compares against. No benchmark.

## Determinism Rules
- Presentation only. The package reads no simulation RNG and no wall clock; it queues cues for the director.

## Save/Mod Impact
- None.

## Tests
- 4 source files, 2 test files, 13 `Test*` functions.
- Coverage 81.5 % (ARCH-004).

## Benchmarks
- None.

## Known Design Constraints
- The App's sound director stays root; this package only queues.