# wizard package

## Package Purpose
The New Colony flow (NG-1): scenario choice and the Scenario Editor, storyteller and difficulty, world generation and the landing site on the globe, the factions, the starting colonists and the loading step. `Flow` owns the draft and reaches the shell through a small interface (`Shell`); the colony itself is built and installed by the shell. Feature package (ARCH-005 wave 5.2, agent A2).

## Owned Concepts
- `Flow` (37 methods, 19 exported): the flow state machine and the page dispatch. Six page types, each `struct{*Flow}`: `storyPage` (11), `worldPage` (12), `pawnsPage` (15), `editorPage` (16), `landingPage` (16), plus the loading step.
- `Draft` (14 methods, 42 fields; budget `wizard.Draft 42` in `solid_budget.txt`): the new game's draft, the moved `newGameState` debt.
- `Shell` = `Hinter + Settings + Widgets + Dispatcher + Host`.
- `Host` (8 methods): `Env`, `KeepDraft`, `SaveSettings`, `SliderTick`, `Load` (returns a `Loader`), `DrawPortrait`, `DrawDiorama`, `MapSizeNote`.
- `ScenarioStore` (`List`, `Load`, `Save`, `Delete`; scenario files by name) and `Loader` (`Step`, `Finish`).
- `candidatePool` (8 methods, `screen_pawns.go`): the candidate colonists the pawns page edits.
- `Form` helpers come from `uikit/form.go` (the difficulty widgets shared with `options`).

## Allowed Dependencies
- Leaves: `art`, `gfx`, `ids`, `inputframe`, `ports`, `uikit`.
- `globe`: the landing-site globe view.
- `application/game` read models and setup (`game.Setup`); domain: `animal`, `building/construction`, `difficulty`, `economy/item`, `faction`, `pawn/look`, `pawn/pawngen`, `pawn/trait`, `research`, `scenario`, `sim/clock`, `storyteller`, `world/biome`, `world/planet`, `world/plant`, `world/site`, `world/temperature`.
- `platform/config` (settings), `platform/input`.
- Ebitengine (`ebiten`); stdlib (`bytes`, `errors`, `fmt`, `hash/fnv`, `image/color`, `math`, `math/rand/v2`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Feature packages: `options` (wizard's story tab uses `uikit/form.go`, not options), `windows`, `hud`, `inspector`, and the rest.
- `render/*`: the diorama and portrait draw through `Host`.
- Simulation mutation: the draft is written as data; the shell builds the colony from it.

## Public Contracts
- `Host.Load(s *game.Setup)` returns a `Loader`; the shell runs the loading step by calling `Loader.Step` until `done`, then `Finish`.
- `ScenarioStore` is the only file access; the adapter behind it is the root's.
- `Host.KeepDraft` mirrors the draft into the App's planet (`a.ng` stays on App).

## Invariants
- Every screen is reachable from the start (`TestFlowWalksEveryScreenToStart`).
- The backdrop prefers the given planet (`TestBackdropPrefersTheGivenPlanet`) and fits its render (`TestBackdropDiscFitsItsRender`); it builds across frames (`TestBackdropShowcaseBuildsAcrossFrames`).
- No type over 40 methods; `Draft` is 42 fields and budgeted.

## Hot Paths
- The page draws each frame and the globe backdrop draws each frame. Widget declaration runs once per `Update`. No benchmark for the flow.

## Determinism Rules
- Setup-time randomness only: `RandomSeed` and the scenario editor's seed use the package-level `math/rand/v2` functions (`world_words.go`, `screen_scenario_editor.go`). These are not the simulation's RNG, but they are global RNG calls and the rule forbids adding more. New setup randomness takes a seeded source.

## Save/Mod Impact
- Scenario files (`ScenarioStore`: list, load, save, delete by name). Saved through the store interface only; the file format belongs to `scenario` and the adapter.

## Tests
- 16 source files, 3 test files, 9 `Test*` functions; `fakeShell` (18 methods).
- Coverage 73.4 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- The largest package by lines (5,529 source lines in 16 files).
- `a.ng` stays on App: `world_ui.go` reads the planet, and the flow mirrors the draft into it.
- The difficulty widgets (`diffSlider`, `diffCheck`, `diffStepper`) were lifted into `uikit/form.go` and shared with `options`.
