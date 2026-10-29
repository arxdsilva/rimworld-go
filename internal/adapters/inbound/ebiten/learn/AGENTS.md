# learn package

## Package Purpose
The learning helper (PLAY-018): the lessons, which are done, the card and chrome that teach the next one, and the observation of the player's own commands that marks a lesson learned. `Helper` reaches the program through `Shell`. Feature package (ARCH-005 wave 5.3, agent A11).

## Owned Concepts
- `Helper` (24 methods, 16 exported) built by `New(sh Shell) *Helper`; owns the former `hst.learn` state.
- `Lesson` and `ByID(id)`: the lesson catalogue.
- `State`, `Memo` (the draft memo), `Row`, `Layout`, `Frame`, `Sensed` (what the helper senses about the colony).
- `KeyFunc func(input.Action) string`: renders a live key binding into lesson text.
- `Host` (7 methods): `Colony`, `Chain`, `Frame`, `Sense`, `Layout`, `Updates`, `Save`.
- `Shell` = `Hinter + Sizer + Settings + Widgets + Host`.

## Allowed Dependencies
- Leaves: `gfx`, `hudlayout`, `inputframe`, `ports`, `uikit`.
- `domain/sim/command` (the observed command type).
- `platform/config` (settings), `platform/input` (actions and key names).
- Ebitengine; stdlib (`fmt`, `math`, `slices`, `strings`).

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `devtools`, `perf`, `options`, `wizard`, `mainmenu`, `selection`).
- `application/game` is not imported here; the colony arrives through `Host.Colony`.

## Public Contracts
- `Host.Chain(observe func(command.Command))` hooks the command chain once; the helper watches commands and never emits gameplay commands.
- `Host.Save(config.Settings) error` persists a changed setting; the root performs the write.

## Invariants
- A lesson completes only after its draft exists (`TestDraftMemoNeedsTheDraftFirst`).
- The hook is installed once (`TestHelperCompletesAndHooksOnce`).
- Lesson text shows live key names and survives an unbound key (`TestLessonTextShowsLiveKeysAndSurvivesNoBinding`).

## Hot Paths
- Lesson state updates on each observed command and once per `Update`; the card draws each frame. No benchmark.

## Determinism Rules
- Presentation only. The helper observes commands; it never reads the simulation's RNG or clock.

## Save/Mod Impact
- Writes settings only through `Host.Save`; the package writes no files itself.

## Tests
- 6 source files, 2 test files, 4 `Test*` functions (`TestLessonCatalog`, `TestLessonTextShowsLiveKeysAndSurvivesNoBinding`, `TestDraftMemoNeedsTheDraftFirst`, `TestHelperCompletesAndHooksOnce`).
- Coverage 34 % (ARCH-005 row); the rest runs through the root's learn tests.

## Benchmarks
- None.

## Known Design Constraints
- `keyName` and `learnSelection` stay root: `shell.go` and a HUD test call them.
- The journey emitter scan skips `learn/`.
