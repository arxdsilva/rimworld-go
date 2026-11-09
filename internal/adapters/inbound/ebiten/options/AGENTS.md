# internal/adapters/inbound/ebiten/options

## Purpose
The Options screen and the settings it edits: the tabs (graphics, audio, controls with key capture, the story tab's difficulty form, RAM notice), loading, applying and saving `config.Settings`. It holds the saves book (`Saves *saves.Book`, sub-package `saves/`), which the root builds. `Screen` reaches the program through `Shell`; the root applies a changed setting to the renderer, window and sound (`ports.Dispatcher` / `Host`). Feature package (ARCH-005 wave 5.3, agent A4).

## Owns
- `Screen` built by `New(sh Shell) *Screen`: tabs, `ApplyOption`, controls key capture, the story tab over `uikit/form.go`, `optionLock` (a commitment colony locks developer mode). The save slots, autosave and the Load screen are `saves/` (`saves/AGENTS.md`).
- Pure store functions: `LoadSettings`, `SaveSettings(h Hooks, s) string`, `ApplyDetectedTier`; `Hooks` (the side effects a changed setting needs) and `Detect` (the detected hardware tier).
- `OptionRow`/`OptionRows(tab)`: the row table for each tab; `RebindState`, `PendingBind`: the key capture.
- `Env` (with `SavesDir`, which the root fills from `SaveHooks.Dir`, SAV-023).
- Interfaces: `Host` (7 methods: `Env`, `SetSettings`, `Cue`, `Toast`, `Send`, `PaintMenu`, `Rebind`), `Story` (7: `StoryView`, `PaintCard`, `CardTip`, `PresetTip`, `SaveMode`, `OpenCustom`, `Rules`).
- `Shell` = `Nav + Sizer + Settings + Widgets + World + Host + Story`.

## Depends on
- Leaves: `gfx`, `ids`, `ports`, `uikit`; its own sub-package `saves` (the type of `Screen.Saves`).
- Application read model: `application/game` (`StoryView`, slot data).
- Domain value types: `difficulty`, `gameplay` (the Habits tab), `sim/clock`, `sim/command`, `storyteller`.
- `platform/config` (Settings, Gain defaults), `platform/input` (actions), `platform/sysinfo` (RAM and tier detection).

## Must not depend on
- Other feature packages, including `wizard`: options may not import wizard, which is why the `Story` interface exists.
- `render/*`: the renderer, window and sound effects of a changed setting stay in the root.

## Contracts
- `New(sh Shell)`; the caller pushes `Screen` through the navigator.
- Settings changes go to `Host.SetSettings`; commands go through `ports.Commands`.
- The settings file (`LoadSettings`/`SaveSettings`, `config.Settings`) is user data: changing a field's name or meaning needs a migration.

## Invariants
- The RAM estimate is monotone in map size (`TestEstimatedRAMGrowsWithTheMap`); the minimum RAM is one and a half plus two GiB, rounded up (`TestMinimumRAMIsOneAndAHalfPlusTwoGiBRoundedUp`).
- Low-RAM text names both amounts (`TestLowRAMTextNamesBothAmounts`).
- `Host.Save` stores changed settings normalised and without re-applying them (the `Host` doc comment).

## Hot paths
- Controls are declared once per `Update`; `Draw` paints the active tab. No allocation claim.

## Tests
- 8 source files, 4 test files, 15 `Test*` functions.
- Coverage 41.4 % (ARCH-005 row).

## Notes
- `App.settings` and `App.rebind` stay root: every role reads the first, and `input_frame.go` and `shell.go` read the second.
- The renderer, window and sound side effects of a changed setting stay in the root's `options_compat.go`.
- The save screen and its `time` use moved to `saves/`.
