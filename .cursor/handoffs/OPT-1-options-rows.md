# OPT-1: Options rows (W1-OPTS)

Every OPT-002…032 row is on the Options screen and saves through `Host.SetSettings`; the hooks behind them belong to the other W1 agents. `options.Screen` gained no methods (free functions in `rows_*.go`).

## Tabs and ids (`options/rows_general.go`, `rows_interface.go`, `rows_helpers.go`)
- Order: audio, graphics, gameplay, [story], habits, controls, access, **general, interface**, [**developer**, with developer mode]. New tabs go last so the default tab (audio) and Story at index 3 keep working in root tests.
- General: `language` (OPT-001, `Env.LangDir`), `pauseonload`, `autopause` (replaces `pausethreat`), `autosaveskept` (step 1–25), `customcursor`, `names` (dialog), `savesfolder`, `logfolder`, `restore` (confirm dialog).
- Interface: `clockrealtime`, `clock12h`, `animalnames`, `barweapons`, `barmood`, `colonistbar`, `categorized`, `tempunit` (moved from Gameplay; C, F, K), `confirm`, `unpausespeed`, `farpawns`, `farstyle`, `menubg`, `plantsway`.
- Gameplay (second column, added): `zoomtomouse`, `smoothjumps`, `dragpct`, `zoomoutworld`, `rememberstyles`, `shake`.
- Graphics (added): `windowsize`, `borderless`. Accessibility (added): `tooltipdelay`, `colorvision`, `largersmall`, `hatsonmap`.
- Developer: `verboselog`, `logonwarn`, `logesc`, `pauseonerror`, `testmaps`.
- Tabs narrow to fit the width (`tabW`); the Gameplay note now sits beside Back.
- `OptionRow.run` makes a row a one-shot action; `Env` gained `OpenFolder`, `SavesDir`, `LogDir`, `LangDir` (all optional; the root may fill them).

## Behaviour
- Restore defaults: confirm, then `config.Defaults()` keeping key bindings (Controls has its own reset), install id, learned lessons, RAM notice, last colony choices, and developer mode in a commitment colony; graphics are detected again.
- Autosave rotation (`saves.go autosaveSlot`): kept = 1 writes `<slot>-autosave` as before; more writes `<slot>-autosave-1…N`, first free else oldest by modified time (day-based fallback if the list fails). Loading strips either suffix. The Load list pages (8 saves + a "More saves" row, id `load.slot.-1`, handled by `LoadSlot(-1)`, no root change).
- Log folder: nothing writes log files yet; the default is `<user config dir>/rimworld-go/logs` (created on open).
- Kelvin: `units.Temp{K}`, `units.UnitK`.

## Findings
- OPT-033: `Gameplay.RunOnStart` and `BirthdayLetters` are read nowhere (only declared); wire or delete in config.
- OPT-034: UI checklist §34 shipped but `[ ]`: 1683–1687 (volumes), 1694 (autosave interval), 1696 (edge scrolling), 1709 (UI scaling), 1712 (reduced flashing). Still owed after this: 1671, 1679, 1695, 1697, 1711, 1714 need the hooks.
- OPT-014 applies: the colonist bar bust draws apparel including hats (`portrait.go`), so `hatsonmap` needs the HUD/portrait hook.

## Root tests that now fail (not mine)
`threat_ui_test.go` (`options.pausethreat`), `screen_landing_wg_test.go` and `journey_globe_test.go` (`options.tempunit` is on the Interface tab, three-way), `graphics_settings_test.go TestOptionsGraphicsLabelsFitTheirColumn` (check after the label rename).

## Journeys wanted
Autopause cycle, restore defaults confirm, names dialog, folder buttons, autosave rotation then Load paging, Developer tab with dev mode, Kelvin.
