# Options gap analysis (OPT-*)

Scope: the Options screen and what each option changes, compared against the vanilla 1.6 options dialog, plus the translation layer behind a Language option. Load only for OPT-* tasks. Handoffs go in `.agents/handoffs/OPT-N-*.md`. The screen's layout rules are in `MAIN_UI_SCREENS_SPECIFICATION.md` §42.

## Decision record

| Item | Decision |
|---|---|
| Goal | Every vanilla option that means something in this game has a row, and the row changes what it says, live. |
| Language | Build the translation layer and a Language row now (user, 2026-11-06). English is the only language; output in English is byte-identical. |
| Names | Our own labels and tooltips. Only behaviour is compared. |
| Settings stay presentation | As before (`ebiten/AGENTS.md`): nothing here reaches the simulation, except Max colonies (a command). Preferred names are applied by the wizard before the Setup, so they don't either. |
| Not applicable | Texture compression, Steam Deck keyboard, mod settings, the DLC simulation flags, gravship cutscenes, quick-start options, controller config. |

## Sources

The wiki's Options page returns 404 (2026-11-06). The option set comes from the field names in a 1.6.4871 `Prefs.xml` and the key names in the installed game's `Menu_Options.xml`. Only names were read, and no text from them enters the repo. [Menus](https://rimworldwiki.com/wiki/Menus) and [User interface](https://rimworldwiki.com/wiki/User_interface) confirm the learning helper switch and developer mode.

## Already matching

Volumes (master, game, music, ambient, UI), fullscreen, UI scale, run in background, edge scroll, autosave interval, Celsius or Fahrenheit, max colonies, developer mode, the learning helper and its reset, key bindings, storyteller change (Story tab). Ours only: VSync, frame cap, quality preset and detail rows, HiDPI, world detail, performance overlay, reduced motion and flashing, colonist card, Habits, pan speed, usage data.

## Rows

The phase is in brackets. `open` is owed; `kept` differs on purpose.

| Id | Vanilla | Today | State |
|---|---|---|---|
| OPT-001 | Language | No translation layer; about 3,566 UI literals in 428 files; the simulation sends English (`MessageView`, `LetterView`, `Reason`, `item.Kind.String`) | partial: catalog, Language row, every UI string, and the simulation's messages and letters as format and arguments done [OPT-1, Wave 3, OPT-4, OPT-001-w4-simtext]. Missing: 27 posts keyed on finished text (mental breaks, raid log text, frost, weddings, research letters), and item and species names inside arguments |
| OPT-002 | Restore defaults | None | done [OPT-1] |
| OPT-003 | Open save and log folders | None | done [OPT-1]: `cmd/game` writes `game.log` (the last run kept as `game-prev.log`) |
| OPT-004 | Pause on load | A load starts at normal speed, `renderer.go:191` | done [OPT-1] |
| OPT-005 | Automatic pause: never, major threat, any threat, any letter | One toggle on `MessageView.Danger`, `threat_ui.go:13`; some caution letters carry `Danger` | done [OPT-1] |
| OPT-006 | Autosaves kept | One slot, `options/saves.go:59` | done [OPT-1] |
| OPT-007 | Temperature in Kelvin | C and F only | done [OPT-1] |
| OPT-008 | Realtime clock, 12-hour clock | 24 h only, `hud/global.go:411` | done [OPT-1] |
| OPT-009 | Disable tiny text | Small is 11 px, `gfx/text.go:45` | done [OPT-1] |
| OPT-010 | Animal name display | Named tame animals only, `render/creature/traces.go:88` | done [OPT-1] |
| OPT-011 | Weapons under portraits | None, `hud/colonistbar.go` | done [OPT-1] |
| OPT-012 | Visible mood | Always drawn, `colonistbar.go:551` | done [OPT-1] |
| OPT-013 | Categorised resources | In the HUD menu only, `hud/global.go:123` | done [OPT-1] |
| OPT-014 | Hats only on the map | To check where headgear is drawn | done [OPT-1]: the colonist bar portrait leaves the hat off |
| OPT-015 | Menu background | Diorama, else splash, `mainmenu_compat.go:87` | done [OPT-1] |
| OPT-016 | Pawn highlight at far zoom (none, hostiles, all; dots, silhouettes, both) | Nothing at the map tier, `render/world/lod.go:40` | done [OPT-1] |
| OPT-017 | Tooltip delay | Fixed 24 frames, `uikit/frame.go:12` | done [OPT-1] |
| OPT-018 | Colour-vision support | Fixed palette, `uikit/widgets.go:34`, `assets/palettes/ui.yaml` | partial [OPT-1]: the kit follows the palette; mood, letter and alert colours outside the kit do not yet |
| OPT-019 | Confirmations | Deconstruct and release animal enqueue at once, `building_inspector.go:324`, `animal_inspector.go:121` | done [OPT-1] |
| OPT-020 | Speed after unpausing | Always normal, `renderer.go:290` | done [OPT-1] |
| OPT-021 | Zoom to mouse | Always on, `renderer.go:351` | done [OPT-1] |
| OPT-022 | Smooth camera jumps | Follows reduced motion, `options_compat.go:420` | done [OPT-1] |
| OPT-023 | Map drag sensitivity | 1:1, `renderer.go:354` | done [OPT-1] |
| OPT-024 | Zooming out switches to the world | No | done [OPT-1] |
| OPT-025 | Remember draw styles | Reset on each tool, `architect/tool.go:368` | done [OPT-1] |
| OPT-026 | Plant wind sway | Only reduced motion stops some, `render/env/tree_fade.go:80` | done [OPT-1] |
| OPT-027 | Screen shake intensity | No camera shake at all | done [OPT-1] |
| OPT-028 | Custom cursor | System cursor; no art | done [OPT-1, OPT-2 art] |
| OPT-029 | Resolution, borderless fullscreen | Free resize and fullscreen only | done [OPT-1] |
| OPT-030 | Verbose log, open log on warnings, close on Escape, pause on error | Dev log only, `devtools/panel.go:138` | done [OPT-1] |
| OPT-031 | Test map sizes | No | done [OPT-1] |
| OPT-032 | Names you want to see | No | done [OPT-1] |
| OPT-033 | Saved but never read | `Gameplay.RunOnStart`, `BirthdayLetters`, `config/settings.go:146-147` | done [OPT-1]: both settings deleted (read nowhere) |
| OPT-034 | Spec and checklist drift | §42 marks the learning helper `[ ]`; checklist §34 marks shipped rows `[ ]` (1683–1687, 1694, 1696, 1709, 1712) | done [OPT-1]: UI checklist §34 ticked |

## Translation layer

- **Keys** are the English source strings (gettext style), so English needs no catalog and every test keeps its text.
- **No package state.** The catalog is a value held by `App` and served through `ports.Texts` (`T`, `Tf`); pure helpers take a `lang.Tr`.
- **Files:** `assets/lang/<code>.json`, generated for English by an extractor test that fails while it is stale. `TestUITextIsTranslatable` reports literals drawn without `T`; it turns into a failure once the migration is done.
- **Simulation text:** messages and letters carry `Format` and `Args` beside `Text`; reasons and item names are translated by exact key.
- **Fonts:** Noto Sans covers Latin, Greek and Cyrillic. A CJK language needs a font of its own (open until one is added).

## Proposed phases

- **OPT-1:** every row, and the hooks behind them.
- **OPT-2:** the art: cursor, the colour-vision palette values.
- **OPT-3:** UI strings through the catalog, package by package; the guard turns into a failure.
- **OPT-4:** simulation text carries its format; the Language row lists the catalogs found.

## Checklist cross-reference

UI §34 (1667–1716): 1671 resolution `[ ]` (OPT-029), 1679 far-zoom `[ ]` (OPT-016), 1695 speed `[ ]` (OPT-020), 1697 confirmations `[ ]` (OPT-019), 1711 colour vision `[ ]` (OPT-018), 1713 text size `[-]` (OPT-009), 1714 tooltip delay `[ ]` (OPT-017); stale rows in OPT-034. Spec §42: far-zoom detail, speed behaviour, confirmations, learning helper, colour vision, tooltip delay, other accessibility options.
