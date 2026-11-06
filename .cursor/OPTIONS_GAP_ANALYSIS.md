# Options gap analysis (OPT-*)

Scope: the Options screen and what each option changes, compared against the vanilla 1.6 options dialog, plus the translation layer behind a Language option. Load only for OPT-* tasks. Handoffs go in `.cursor/handoffs/OPT-N-*.md`. The screen's layout rules are in `MAIN_UI_SCREENS_SPECIFICATION.md` §42.

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
| OPT-001 | Language | No translation layer; about 3,566 UI literals in 428 files; the simulation sends English (`MessageView`, `LetterView`, `Reason`, `item.Kind.String`) | open [OPT-1 infrastructure, OPT-3 migration, OPT-4 simulation text] |
| OPT-002 | Restore defaults | None | open [OPT-1] |
| OPT-003 | Open save and log folders | None | open [OPT-1] |
| OPT-004 | Pause on load | A load starts at normal speed, `renderer.go:191` | open [OPT-1] |
| OPT-005 | Automatic pause: never, major threat, any threat, any letter | One toggle on `MessageView.Danger`, `threat_ui.go:13`; some caution letters carry `Danger` | open [OPT-1] |
| OPT-006 | Autosaves kept | One slot, `options/saves.go:59` | open [OPT-1]: 1–25, rotating |
| OPT-007 | Temperature in Kelvin | C and F only | open [OPT-1] |
| OPT-008 | Realtime clock, 12-hour clock | 24 h only, `hud/global.go:411` | open [OPT-1] |
| OPT-009 | Disable tiny text | Small is 11 px, `gfx/text.go:45` | open [OPT-1] |
| OPT-010 | Animal name display | Named tame animals only, `render/creature/traces.go:88` | open [OPT-1] |
| OPT-011 | Weapons under portraits | None, `hud/colonistbar.go` | open [OPT-1] |
| OPT-012 | Visible mood | Always drawn, `colonistbar.go:551` | open [OPT-1] |
| OPT-013 | Categorised resources | In the HUD menu only, `hud/global.go:123` | open [OPT-1]: also a row |
| OPT-014 | Hats only on the map | To check where headgear is drawn | open [OPT-1], N/A if no portrait draws it |
| OPT-015 | Menu background | Diorama, else splash, `mainmenu_compat.go:87` | open [OPT-1] |
| OPT-016 | Pawn highlight at far zoom (none, hostiles, all; dots, silhouettes, both) | Nothing at the map tier, `render/world/lod.go:40` | open [OPT-1]; closes §42 "far-zoom detail" |
| OPT-017 | Tooltip delay | Fixed 24 frames, `uikit/frame.go:12` | open [OPT-1]; §42 row |
| OPT-018 | Colour-vision support | Fixed palette, `uikit/widgets.go:34`, `assets/palettes/ui.yaml` | open [OPT-1]; §42 row |
| OPT-019 | Confirmations | Deconstruct and release animal enqueue at once, `building_inspector.go:324`, `animal_inspector.go:121` | open [OPT-1]; §42 row |
| OPT-020 | Speed after unpausing | Always normal, `renderer.go:290` | open [OPT-1]; §42 "speed behaviour" |
| OPT-021 | Zoom to mouse | Always on, `renderer.go:351` | open [OPT-1] |
| OPT-022 | Smooth camera jumps | Follows reduced motion, `options_compat.go:420` | open [OPT-1] |
| OPT-023 | Map drag sensitivity | 1:1, `renderer.go:354` | open [OPT-1] |
| OPT-024 | Zooming out switches to the world | No | open [OPT-1] |
| OPT-025 | Remember draw styles | Reset on each tool, `architect/tool.go:368` | open [OPT-1] |
| OPT-026 | Plant wind sway | Only reduced motion stops some, `render/env/tree_fade.go:80` | open [OPT-1] |
| OPT-027 | Screen shake intensity | No camera shake at all | open [OPT-1]: shake on blasts and lightning, 0–200 % |
| OPT-028 | Custom cursor | System cursor; no art | open [OPT-1, art] |
| OPT-029 | Resolution, borderless fullscreen | Free resize and fullscreen only | open [OPT-1]; UI checklist 1671 |
| OPT-030 | Verbose log, open log on warnings, close on Escape, pause on error | Dev log only, `devtools/panel.go:138` | open [OPT-1] |
| OPT-031 | Test map sizes | No | open [OPT-1] |
| OPT-032 | Names you want to see | No | open [OPT-1]: applied by the wizard, deterministic |
| OPT-033 | Saved but never read | `Gameplay.RunOnStart`, `BirthdayLetters`, `config/settings.go:146-147` | open [OPT-1]: wire or delete |
| OPT-034 | Spec and checklist drift | §42 marks the learning helper `[ ]`; checklist §34 marks shipped rows `[ ]` (1683–1687, 1694, 1696, 1709, 1712) | open [OPT-1] |

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
