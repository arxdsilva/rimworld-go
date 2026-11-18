# internal/adapters/inbound/ebiten/plugins

## Purpose
The Plugin Manager (PLG-070, MAIN_UI §45), its load-order explanation (PLG-071, §46) and the content problems view (PLG-041, §48), reached from the main menu. Feature package: it reads one read model and changes nothing but the player's settings.

## Owns
- `View`, `Row`, `Problem`: the run's plugins as plain values (every installed plugin, the loaded ones in load order with their position and reason, every problem with its provenance, the enabled set the run started with, the plugin folders). `internal/compose/game` (`runContent.view`) fills it once at start; captures get an empty one. `View.Late` (PLG-060) adds the problems found while playing, the scripts the game turned off; the root's hosts read `View.Live()`.
- `Screen` built by `New(sh Shell)`: `Declare(scr, sw, sh)` and `Draw(dst, scr, sw, sh)` for `ports.ScreenPlugins` (list with checkboxes, detail, restart banner, empty state) and `ports.ScreenPluginProblems` (problem list, Show plugin, Disable <plugin>, Back).
- `Toggle`, `Pending`, `StateOf`: the enabled-set edit, the restart test and a row's state.
- `Host` (`Plugins`, `SetSettings`); `Shell` = `Settings + Widgets + Texts + Dispatcher + Host`.

## Depends on
- Leaves: `gfx`, `ports`, `uikit`; `platform/config` (`Settings.Plugins`).

## Must not depend on
- Other feature packages, `application/modding` (compose maps it to `View`), domain, the simulation, the file system.

## Contracts
- A toggle rewrites `Settings.Plugins.Enabled` through `SetSettings` (a fresh sorted set; ids of uninstalled plugins stay); nothing restarts. The banner shows while the saved set differs from `View.Enabled`, the set the run started with (not the loaded ids, so a plugin that failed to load raises no banner).
- States: loaded, `not loaded: <first error>`, off, on after restart, off after restart.
- Widget ids are by row index (`plugins.on.N`, `plugins.list.N`): manifest ids may be blank or repeated. Checkboxes sit left of the list rect, so a press never lands on both.
- Disable is refused, with the reason, for a problem naming no plugin, core, or a plugin already off. Copy is not offered: the client has no system clipboard.
- Navigation goes through `ports.Push`/`ports.Back`; Escape is the root's Back. Show plugin goes back when the problems view sits on the list, else pushes the list.

## Tests
- `screen_test.go`: `fakeShell` with a kit driven frame by frame and a navigator; toggles, states, empty state, Disable rules, Show plugin, the §46 detail.
- Root journeys `TestJourneyPluginsToggle`, `TestJourneyPluginsProblemsNotice`, `TestJourneyPluginsEmpty` (`journey_plugins_test.go`).
