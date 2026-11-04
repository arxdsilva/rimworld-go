# RimHUD gap analysis (RHUD)

Scope: every feature of the RimWorld mod RimHUD, compared with our game at commit `e924244d3`. Sources are the mod's source, resource and language files (read to learn what it does; there is no licence on that repository, so nothing is copied), its readme and its Workshop description, and our code. `MAIN_UI_SCREENS_SPECIFICATION.md` §0, §9–§16, §39–§48, §58 and §59 were read for the screen rules. Names, proposed UI strings, default layouts and default colours here are ours. Mod element ids such as `NeedMood` appear only so a row can be traced; no player sees them.

Four source classes are absent from the reference, so those features are known only from their strings and the code that calls them: the setup dialog, the preset save dialog, the tutorial and the text style type. Rows that depend on them say what was seen. `rh/cs_strings.xml` belongs to another mod and was ignored.

Path prefixes: `ebiten/` is `internal/adapters/inbound/ebiten/`, `game/` is `internal/application/game/`, `domain/` is `internal/domain/`, `config/` is `internal/platform/config/`. A row with `file.go:N` and no directory continues the directory named earlier in that cell or the row above. `sheet` is the new pure package `ebiten/dossier/sheet`. `DOS-n` is a work package from [Proposed split](#proposed-split), `Card` is `dossierview.Card`, and tests are listed by package there unless a row names one. `UNCLASSIFIED` in a row marks a fact the code reading did not settle; its status is the best guess, and the package named settles it.

## Decisions

| Item | Decision |
|---|---|
| Name | The feature is the **dossier**. Packages: `ebiten/dossier`, `ebiten/dossier/sheet`, `game/dossierview`, `internal/adapters/outbound/cardpresetfile`, `internal/platform/openfolder`. The mod's name appears in this file only, never in code or UI. The player sees "Colonist card". |
| Layer rule | The card lives in the adapter. It reads `Snapshot()`, `World()`, `Bio`, `Health`, `Prisoner`, `Areas`, `Factions` and one new query, `Dossier(id)`, and emits existing `command.Command` kinds plus one new kind, `SetName`. |
| Settings | All of it goes in `config.Settings.HUD` (player-side, never in a save), `SettingsVersion = 8`. Colours store only differences, as `Controls.Bindings` does. |
| Default state | `HUD.Enabled` is false, so the current pane, every journey, every golden and every capture stay as they are until the feature is whole. DOS-9 flips it in one commit, after its gate. Captures get no settings hooks (`ebiten/AGENTS.md`), so they stay on the plain pane either way. The user may prefer otherwise; the flip is one line in `config.Defaults`. |
| Off and on | The mod has no off state, because removing the mod is its off state. We need `Enabled`. With `Enabled` on: `Restyle` on and `Docked` on is the mod's default; `Restyle` off forces floating with our plain pane beside the card. |
| Which selections | One pawn or one animal gets the card. Groups, buildings, plants, items, zones and cell info keep today's pane in every mode. |
| Mechs | Mechs are `AnimalView`s here (`game/animals.go:104`), so a mech counts as the wild target, `w`. |
| Slavery, ideoligion, genes, psyfocus | Grepped for `slave`, `ideoligion`, `ideology`, `ideo`, `xenotype`, `gene`, `psyfocus`, `psychic`, `meditat` over `internal/`: no hits. All excluded. `mutant` and `ghoul` hit one flavour line in `domain/animal/ecology.go`; also excluded. |
| Trainables | Ours are obedience, guard and release (`domain/animal/training/training.go:7`). Tameness, rescue, haul and all Odyssey rows are excluded. Guard has no mod row and **is added** to our catalog as an extra, def `guard`. |
| Artistic skill | We have eleven skills, none artistic. The default layout's skill pairs are rearranged so the odd one is not left beside a blank. |
| Apparel policy | Excluded here, because the system is missing (open in both checklists). It is core gameplay, not DLC, so its key `clothes_pick` stays reserved. When `SetApparelPolicy` lands, that task adds the widget. |
| Stat lines with no stat | A tooltip stat line with no formula in our game is excluded, not faked. Where a pure domain formula exists the line calls it. |
| Selector menus | `uikit.Picker` stands for the mod's float menu; it is what the game uses everywhere. |
| Refresh rate | Kept as a setting, on the presentation clock. Only the card's built rows wait for it. The sim is not touched, and a selection change rebuilds at once. |
| Hysteresis | The card reports state and starts no behaviour, so the no-flap rule does not apply. `SkillCell`'s active colour uses the sim's own expiry, `workingTicks`. |
| Colour alone | Spec §59 forbids colour-only signals. Every warning tone sits on a line whose words state the same fact, and DOS-2 tests that tone and words agree. |
| Third-party libraries | None. `os/exec` opens the folder; colour maths is about 30 lines. |
| Plugins | No format exists, so custom widgets are excluded. They should be re-opened with spec §45. |

## Counts

| | Count |
|---|---|
| Rows | 504 |
| Done | 43 |
| Partial | 159 |
| Missing | 226 |
| Excluded | 76 |
| Marked UNCLASSIFIED (status is a best guess) | 0 |

Status by family:

| Family | Rows | Done | Partial | Missing | Excluded |
|---|---|---|---|---|---|
| A. Display modes and the inspect pane | 60 | 4 | 15 | 36 | 5 |
| B. Layout tree | 16 | 0 | 2 | 14 | 0 |
| C. Value widgets | 240 | 29 | 100 | 60 | 51 |
| D. Targets and default layouts | 13 | 0 | 4 | 9 | 0 |
| E. Layout editor | 24 | 0 | 1 | 22 | 1 |
| F. Presets | 27 | 1 | 1 | 25 | 0 |
| G. Theme | 52 | 0 | 19 | 32 | 1 |
| H. Tooltips | 33 | 5 | 11 | 11 | 6 |
| I. Visual warnings | 5 | 0 | 3 | 2 | 0 |
| J. Letter stack | 8 | 2 | 0 | 6 | 0 |
| K. Modders and integrations | 11 | 0 | 0 | 0 | 11 |
| L. Game hooks the mod patches, mapped to our seams | 15 | 2 | 3 | 9 | 1 |

## Items

Columns: id, mod feature, what we have today, status, work needed.

### A. Display modes and the inspect pane

#### A1. Docked in the pane

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-001 | The pane can host the card (`DockedMode`, default on) | Pane body is fixed text and bars, `ebiten/inspector/hud_inspect.go:351`, `ebiten/inspector/cmds_pawn.go:155` | missing | DOS-4 route the body through `inspector.PaneBody`; `config.HUD.Docked`; DOS-5 row; `TestJourneyDossierDocked` |
| RHUD-002 | Switch that lets the mod restyle the pane at all (`Modify`); off forces floating and greys its controls | No such switch | missing | `config.HUDPane.Restyle`; `Normalize`: `!Restyle` forces `Docked=false`; DOS-5 greys the group with a reason |
| RHUD-003 | Pane height, slider plus typed entry (`Height`) | `PaneH = 188` constant, `ebiten/hudlayout/layout.go:17`, `:236` | missing | DOS-4 `hudlayout.Inputs.PaneH`; `config.HUDPane.Height` (188..800, clamped to the screen by `Compute`); DOS-5 slider and entry; golden test at 1113x626 |
| RHUD-004 | Least tab count the pane must fit (`MinTabs`) | `PaneW = 400`, tabs squeezed to fit, `ebiten/hudlayout/layout.go:16`, `ebiten/inspector/hud_inspect.go:272` | missing | `config.HUDPane.MinTabs` (6..12); DOS-4 `Inputs.PaneW = TabW * max(MinTabs, tabs)` |
| RHUD-005 | Tab button width (`TabWidth`) | Widths measured from labels, `ebiten/inspector/hud_inspect.go:272` | missing | `config.HUDPane.TabW` (72..150); DOS-4 `paneTabRects` takes a fixed width when above 0 |
| RHUD-006 | Pane width follows tabs (`PaneWidthFor`) | Fixed width | missing | DOS-4, same `Inputs.PaneW`; the command grid already keys on `l.Pane.Right()`, `ebiten/hudlayout/layout.go:246` |
| RHUD-007 | Tab strip right-aligned with the pane | Left to right, `ebiten/inspector/hud_inspect.go:272` | missing | DOS-4 `paneTabRects` right-aligns when `Restyle`; order kept so journeys press the same ids |
| RHUD-008 | Pane header row: name in the large style, tinted by faction relation | Title and subtitle plain, `ebiten/inspector/hud_inspect.go:351` | partial | DOS-4 `inspector.Spec.TitleColor`, filled by the card; tint from family G faction colours |
| RHUD-009 | Bio tooltip on the header name | Absent | missing | DOS-4 `Spec.TitleTip`; text from `sheet.BioTip` (DOS-2) |
| RHUD-010 | Click on the header name toggles the Social tab | Absent; `OpenPaneTab` exists, `ebiten/inspector/hud_inspect.go:467` | missing | DOS-4 `Spec.TitleTab = "inspector.social"` |
| RHUD-011 | Header button: select the next thing on this cell | Repeat click on the map cycles, `ebiten/selection/selection.go:206`; no button | partial | DOS-4 header button using `selection.Model.Selectables`, `:106`; a new method on `selection.Model`, no sim change |
| RHUD-012 | Header button: info card with full stats | No stat screen; spec §11 unbuilt; checklist line 178 open | missing | DOS-3 read-only stat sheet modal from `dossierview.Card.Stats`, labels from `domain/pawn/stat/stat.go:86`; no breakdown (§11 stays open) |
| RHUD-013 | Header button: xenotype icon | No genes or xenotypes: grep over `internal/` finds nothing | excluded | No gene system |
| RHUD-014 | Header icon: faction emblem (`ShowFactionIcon`) | `FactionView.Emblem`, `game/ng1_types.go:149`; art `assets/generated/ui/icons/faction_*.png`; not in the pane | missing | DOS-4 header icon; `config.HUDPane.FactionIcon`; `dossierview.Card.FactionEmblem` (DOS-1) |
| RHUD-015 | Faction icon tooltip with name and relation | `FactionView` has `Name`, `Relation`, `game/ng1_types.go:140` | missing | DOS-4 tooltip from `Factions()`, no new read model |
| RHUD-016 | Header icon: ideoligion (`ShowIdeoligionIcon`) | No ideoligion: grep finds nothing | excluded | No ideoligion system |
| RHUD-017 | Ideoligion tooltip: certainty and former beliefs | Same | excluded | No ideoligion system |
| RHUD-018 | Header button: hostility response | A command button in the grid, `ebiten/inspector/cmds_combat.go:65`; `command.SetThreatResponse`, `domain/sim/command/command.go:287` | partial | DOS-4 add it to the header when `Restyle`; export `inspector.ThreatResponses` for the label; the grid button stays |
| RHUD-019 | Header selector: medical care level, flesh only | Health tab row, `ebiten/inspector/health_ui.go:198`; animal command, `ebiten/inspector/animal_inspector.go:452`; `command.SetMedicalCare`, `command.go:199` | partial | DOS-4 header button opening `uikit.Picker`, `ebiten/uikit/form.go:202`; mechs hide it |
| RHUD-020 | Care selector tooltip: who and what level now | Level tips only, `ebiten/inspector/health_ui.go:102` | partial | DOS-4 prepends the pawn's name and current level |
| RHUD-021 | Header toggle: self-tend, colonists only | Health tab checkbox, `ebiten/inspector/health_ui.go:231`; `command.SetSelfTend`, `command.go:201` | partial | DOS-4 header toggle, same command |
| RHUD-022 | Self-tend disabled when the pawn cannot doctor | Disabled only when dead, `ebiten/inspector/health_ui.go:233`; `IncapableReason`, `game/skills.go:272` | missing | DOS-4 uses `IncapableReason(id, work.Doctor)` as the reason; fix the Health tab too |
| RHUD-023 | Warning message when self-tend is on and doctoring is off | Absent; `PawnView.WorkDoctor`, `game/game.go:99` | missing | DOS-4 `Hint("… has doctoring switched off.")` on enable; original text |
| RHUD-024 | Self-tend tooltip states the efficiency number | "at lower quality", `ebiten/inspector/health_ui.go:236`; `SelfTendFactor = 700`, `domain/pawn/body/tend.go:26` | partial | DOS-4 prints `body.SelfTendFactor / 10` percent |
| RHUD-025 | Header button: rename, for animals and mechs without a training tab | No rename command; only areas, zones and food policies are renamable, `command.go:128`, `:420`; checklist 11.3 open | missing | New `command.SetName{Pawn, Name}` (DOS-0, handled in DOS-1); DOS-4 rename dialog on `uikit.TextField`, `ebiten/uikit/textfield.go:470`; we offer it for colonists and tame animals; `TestJourneyDossierRename` |
| RHUD-026 | Character log in the pane's body while floating (`AddLog`); not available docked | Log tab only, `ebiten/inspector/log_ui.go:139` | partial | `config.HUDPane.LogBody`; DOS-4 body draws `DrawPawnLogRows`, `ebiten/inspector/log_ui.go:146`; `Normalize` clears it when docked |
| RHUD-027 | That log merges combat and social lines, newest kept to a cap, cached | 30 pawn lines, `game/eventlog.go:21`, `PawnView.Log`, `game/game.go:268`; social log in `SocialView`, `game/views_social.go:13` | partial | DOS-4 merge the two by tick in `inspector`; the 30-line store is kept; no cache: both views are already cached per tick |

#### A2. Floating card

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-028 | Nine anchor positions (`Anchor`) | None | missing | `config.HUDCard.Anchor` (0..8); DOS-4 `hudlayout.CardRect` pure helper; nine-position picker in DOS-5; test all nine |
| RHUD-029 | Horizontal offset | None | missing | `config.HUDCard.OffsetX`; DOS-5 slider, DOS-4 reads it |
| RHUD-030 | Vertical offset | None | missing | `config.HUDCard.OffsetY`; DOS-5 slider, DOS-4 reads it |
| RHUD-031 | Offset limits recomputed from anchor and screen | None | missing | DOS-4 `hudlayout.CardOffsetBounds(sw, sh, anchor, w, h)`; `Compute` clamps at draw, so a smaller window never loses the card |
| RHUD-032 | Card width (`Width`) | None | missing | `config.HUDCard.W` (100..1000); DOS-5 slider, DOS-4 reads it |
| RHUD-033 | Card height (`Height`) | None | missing | `config.HUDCard.H` (100..1000); DOS-5 slider, DOS-4 reads it |
| RHUD-034 | Window background and inner margin when floating | `uikit.Kit.Panel`, `ebiten/uikit/kit.go:358` | partial | DOS-3 `Panel` plus `sheet` content inset; the widget exists, the card does not |
| RHUD-035 | Card shown only for a single pawn or creature; other selections keep the plain pane | `shownPane`, `ebiten/inspector/hud_inspect.go:134`; group pane, `:101` | missing | DOS-4 `PaneBody.Claims(selection)` is true only for one pawn or animal |

#### A3. General display

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-036 | Gear button on hover, only while the mouse is over the card | None | missing | DOS-3 gear widget; new icon `ui_icon/dossier_gear` (DOS-0A); opens the setup modal |
| RHUD-037 | Clicks under the gear do not reach card widgets | Immediate-mode kit gives the top widget the press, `ebiten/uikit/kit.go:127` | partial | DOS-3 declares the gear last; test that a press on it opens setup and no picker |
| RHUD-038 | Gear corner: lower right docked, upper right floating | None | missing | DOS-3 `gearRect(body, docked)` |
| RHUD-039 | Toggle among the play-settings icons | Toggle row pattern, `ebiten/hud/global.go:107` | missing | DOS-4 toggle entry `hud.dossier` flips `HUD.Docked`; icon from DOS-0A; `TestJourneyDossierToggle` |
| RHUD-040 | Refresh rate, tenths of a second (`RefreshRate`) | Pane rebuilds each frame, `ebiten/inspector/hud_inspect.go:134` | missing | `config.HUD.RefreshTenths` (1..30); DOS-3 rebuild gate on the injected clock (`App.now`, `ebiten/app.go:228`); presentation only |
| RHUD-041 | Rebuild at once when the selected pawn changes | `lastTitle` check, `ebiten/inspector/hud_inspect.go` `PaneUI` at `:477` | partial | DOS-3 keys the cache on the entity id, so a stale id is dropped |
| RHUD-042 | Developer timing overlay per widget | `ebiten/perf/probe.go` measures frames, not widgets | missing | DOS-9 `dossier` timing behind `Settings.Gameplay` developer mode, shown in `ebiten/devtools/panels.go` |

#### A4. First-use tutorial

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-043 | Stage: asks the player to hover the pane | `ebiten/learn/lessons.go:19` lesson model; no such lesson | missing | DOS-8 lesson `dossier_hover`, finished from `Lesson.OnUI`, `ebiten/learn/lessons.go:31` |
| RHUD-044 | Stage: asks the player to click the gear | Same | missing | DOS-8 lesson `dossier_gear` |
| RHUD-045 | Dialog note 1: what the settings do | Lessons have a detail card, `ebiten/learn/lessons.go:24` | missing | DOS-8 lesson `dossier_look` body, original text |
| RHUD-046 | Dialog note 2: sections tab and presets | Same | missing | DOS-8 lesson `dossier_sections` |
| RHUD-047 | Dialog note 3: close the dialog | Same | missing | DOS-8 lesson `dossier_close` |
| RHUD-048 | End note: no more prompts | "Panel hides itself once every lesson is done", `ebiten/learn/helper.go` header | done | Our helper already ends itself; no work |
| RHUD-049 | Completion remembered across games | `Gameplay.LearnedLessons`, `config/settings.go:127` | done | Lesson ids above are saved by the helper; no new field |
| RHUD-050 | Reset tutorial button with a description | Learning helper row resets lessons, `ebiten/options/screen_options.go:289` | done | Covers the new lessons with no change; verify in DOS-8 |
| RHUD-051 | Gear hidden until the tutorial completes | None | excluded | Our helper teaches without locking controls (spec §43); a hidden gear would strand a player who turned lessons off |
| RHUD-052 | Tutorial variant of the gear icon | None | excluded | Same; the helper highlights instead |
| RHUD-053 | Tutorial drawing hooked into the game's tutor | Helper panel, `ebiten/learn/draw.go` | done | Lessons draw in the existing panel |

#### A5. Setup dialog

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-054 | Dialog with three tabs: design, colours, content | None; `uikit.Kit.Push`, `ebiten/uikit/modal.go:82`, and tabs, `ebiten/uikit/tabs.go` | missing | DOS-5 modal `dossier.setup` with tabs "Look", "Colours", "Sections"; Escape through `RunModals`, `modal.go:136` |
| RHUD-055 | Use-default button resets everything, with a confirm | Options resets are per screen | missing | DOS-5 "Back to defaults", confirm modal, `config.Defaults().HUD` |
| RHUD-056 | Marker that the settings differ from defaults | None | missing | DOS-5 "Changed" tag from `reflect.DeepEqual` against defaults (the setup is not a hot path) |
| RHUD-057 | Alert when a version bump forced a reset | Unreadable file goes to `.bad`, `internal/adapters/outbound/settingsfile/settingsfile.go:33`; migrations in `Normalize`, `config/settings.go:216` | partial | `HUD` decodes over defaults, so no reset is needed; DOS-0 adds `Hint` when a stored layout fails `sheet.Validate` |
| RHUD-058 | Button that opens the config folder | No helper; `exec.Command` only in `cmd/` | missing | DOS-6 `internal/platform/openfolder.Open(path)` (`open`, `xdg-open`, `explorer`), behind `dossier.FolderOpener`; disabled with a reason when it fails |
| RHUD-059 | Settings persisted between sessions | `settings.json`, `internal/adapters/outbound/settingsfile/settingsfile.go:23` | partial | `config.Settings.HUD`, `SettingsVersion = 8`; test an old v7 file decodes to defaults (DOS-0) |
| RHUD-060 | Layouts persisted in their own files | Would be `settings.json` | partial | `config.HUD.Layouts` holds both trees; one file is simpler and atomic; size capped by `Normalize` (256 nodes, depth 8) (DOS-0) |

### B. Layout tree

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-061 | Root element, filling its area (`Layout`) | Fixed `PaneLayout`, `ebiten/inspector/hud_inspect.go:210` | missing | DOS-2 `sheet.Node` root `column` with `Fill` |
| RHUD-062 | Vertical stack (`VStack`) | None | missing | DOS-2 kind `column`; height is the sum, fill shares the rest |
| RHUD-063 | Horizontal stack, equal widths (`HStack`) | Bars split in up to 4 columns, `paneBarCols`, `ebiten/inspector/hud_inspect.go:80` | partial | DOS-2 kind `strip`; height is the tallest child |
| RHUD-064 | Panel holding rows only (`Panel`) | None | missing | DOS-2 kind `block`, validated: only `line` children |
| RHUD-065 | Row holding widgets, evenly spread (`Row`) | None | missing | DOS-2 kind `line` |
| RHUD-066 | Filled flag on stacks and panels (`FillHeight`) | None | missing | `config.HUDNode.Fill`; DOS-2 measure returns the fill marker; several fills share evenly |
| RHUD-067 | Empty widgets collapse and take their row with them | Lines simply absent, `ebiten/inspector/cmds_pawn.go:230` | partial | DOS-2 `Measure`: height 0 on an empty value |
| RHUD-068 | Off-target widget keeps its slot as a blank | None | missing | DOS-2 returns a `gap` with the widget's height |
| RHUD-069 | Off-target container hides entirely | None | missing | DOS-2 `Prune` |
| RHUD-070 | Targets attribute on any element (`Targets`) | None | missing | `config.HUDNode.Who`, letters `c s p w` (family D) (DOS-2) |
| RHUD-071 | Definition attribute for generic widgets (`DefName`) | None | missing | `config.HUDNode.Def`, the skill, stat, need, training or record key (DOS-2) |
| RHUD-072 | Bar colour style attribute (`BarColorStyle`) | None | missing | `config.HUDNode.BarStyle` (family G) (DOS-2) |
| RHUD-073 | Width, height and tab count on a preset's roots | None | missing | `sheet.Preset.{Dock,Float}{W,H,Tabs}` (family F) (DOS-2) |
| RHUD-074 | Marker box for an unknown element, logged once | None | missing | DOS-3 draws a danger-tinted box with the key; `sheet.Validate` reports; one `Hint` per load |
| RHUD-075 | Blank element (`Blank`) | None | missing | DOS-2 kind `gap` |
| RHUD-076 | Separator element (`Separator`) | `gfx.FillRect` rules elsewhere | missing | DOS-2 kind `rule`, tinted by the line colour key |

### C. Value widgets

#### C1. Name and selectors

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-077 | Name header (`NameHeader`) | `Spec.Title`, `ebiten/inspector/hud_inspect.go:63`, built at `ebiten/inspector/cmds_pawn.go:161` | partial | DOS-3 widget `title`; large style, faction tint |
| RHUD-078 | Name header bio tooltip | None | missing | DOS-3 `sheet.BioTip` |
| RHUD-079 | Name header click opens Social | None | missing | DOS-3 `Actions.OpenTab("social")` |
| RHUD-080 | Selector look: small text on a tinted back | Chip look, `ebiten/inspector/hud_inspect.go:37` | partial | DOS-3 `drawSelector`; colour keys `picker_text`, `picker_back` |
| RHUD-081 | Selector hover box | Kit hover on buttons, `ebiten/uikit/kit.go:297` | done | `uikit.Button` gives it |
| RHUD-082 | Selector menu ends with a manage entry | `uikit.Picker`, `ebiten/uikit/form.go:202`, has no action row | partial | DOS-3 appends a last option "Manage…" mapped to the action; no Kit method added (Kit is at 73 methods) |
| RHUD-083 | Food selector (`Food`) | Line only, `ebiten/inspector/cmds_pawn.go:385`; cycling button, `ebiten/windows/foodpolicy.go:91`; `command.SetFoodPolicy`, `command.go:298` | partial | DOS-3 widget `diet_pick` lists `World().FoodPolicies`, `FoodPolicyView` at `game/game.go:702` |
| RHUD-084 | Food selector only where the pawn's food can be set | `PawnView.FoodPolicy`, `game/game.go:282`; prisoners and animals get policies too, `ebiten/windows/animals.go:262` | partial | DOS-3 hides it for strangers, hostiles and wild animals |
| RHUD-085 | Food selector manage opens the policy editor | Button in Assign, `ebiten/windows/assign.go:152` | partial | DOS-3 `Actions.ShowWindow(ids.TabAssign)`; a way to land in the editor is DOS-3's call into `windows` through the host |
| RHUD-086 | Rules selector replacing food (`Rules`, Pawn Rules) | Third-party mod | excluded | Third-party mod |
| RHUD-087 | Outfit selector (`Outfit`) | No policy: colonists dress by best fit, `game/apparel.go:157`; checklist open at UI line 997, vanilla line 1342; `ebiten/windows/assign.go:22` waits for the system | excluded | No apparel policy system; widget key `clothes_pick` reserved and hidden until `SetApparelPolicy` exists; see Decisions |
| RHUD-088 | Outfit selector manage entry | Same | excluded | No apparel policy system |
| RHUD-089 | Area selector (`Area`) | Chip plus picker, `ebiten/architect/area_select.go:95`, `area_menu.go:178`; `command.AssignArea`, `command.go:131` | partial | DOS-3 widget `area_pick` through `Actions.AreaPicker` |
| RHUD-090 | Area list starts with "no area" | Picker offers it with `base=true`, `ebiten/inspector/animal_inspector.go:124` | done | Reuse `AreaPicker` |
| RHUD-091 | Only assignable areas listed | `areaRef` list in `ebiten/architect/area_menu.go` | done | Reuse `AreaPicker` |
| RHUD-092 | Hovering an area draws it on the map | `Overlays().Area.Hover`, `ebiten/architect/area_menu.go:159`, `ebiten/render/overlay/area_overlay.go:34` | done | Reuse `AreaPicker` |
| RHUD-093 | Area manage entry opens the manager | `Areas.OpenManager`, `ebiten/architect/area_manager.go:40` | partial | DOS-3 `Actions.ManageAreas`, already on `windows` host, `ebiten/windows/deps.go:83` |
| RHUD-094 | Selector back tinted by the area's colour | `AreaTint`, `ebiten/architect/area_menu.go:73`; a swatch only, `:185` | partial | Root host exposes `AreaTint` for the current ref; DOS-3 fills the back |
| RHUD-095 | Only player-faction pawns get an area selector | `areaChip` rules, `ebiten/architect/area_select.go:95`; penned animals, `ebiten/inspector/animal_inspector.go:123` | done | Reuse the chip's `ok` |
| RHUD-096 | Timetable only for a pawn with a schedule | `PawnView.Schedule`, `game/game.go:181` | done | Target filter `c` in the default layouts |
| RHUD-097 | Timetable selector: label is this hour's assignment (`Timetable`) | Chip, `ebiten/inspector/cmds_pawn.go:217` | partial | DOS-3 widget `hour_pick`; `PawnView.Activity`, `game/game.go:86` |
| RHUD-098 | Timetable menu sets the current hour for this pawn | Only the Schedule window paints hours, `ebiten/windows/schedule.go:165`; `command.SetSchedule`, `command.go:45` | missing | DOS-3 options "Hour 14: work" emit `SetSchedule{Pawn, Index: hour, Mode}` |
| RHUD-099 | Timetable uses the pawn's local hour | One map clock, `WorldView.Hour`, `game/game.go:624` | done | Single-clock rule: pawns on another map use `SimMaps`; note in DOS-3 test |
| RHUD-100 | Timetable manage entry opens Schedule | Chip opens it, `ids.TabSchedule`, `ebiten/ids/ids.go:49` | done | Same id |
| RHUD-101 | Timetable back tinted unless unrestricted | `schedColor`, `ebiten/windows/schedule.go:42` | partial | Export it from `windows` is forbidden across features: move the mapping to `sheet` keys with palette tokens (DOS-3) |

#### C2. Standing line (`RelationKindAndFaction`)

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-102 | Adjective: prisoner | `PawnSubtitle`, `ebiten/inspector/cmds_pawn.go:177` | done | Reused wording in `sheet.StandingLine` |
| RHUD-103 | Adjective: slave | No slavery: grep finds nothing | excluded | No slavery system |
| RHUD-104 | Adjective: factionless person | Stranger: `guestLine`, `ebiten/inspector/pawn_state_ui.go:49` | partial | DOS-2 "drifter" wording, `Card.Relation = RelationNone` |
| RHUD-105 | Adjective: wild creature | `AnimalStateLine`, `ebiten/inspector/animal_inspector.go:138` | done | `AnimalView.Tame`, `game/animals.go:46` |
| RHUD-106 | Adjective: hostile person or mech | `Hostile`, `game/game.go:189`; `Mech`, `game/animals.go:104` | partial | DOS-2 wording from the flags |
| RHUD-107 | Adjective: allied | `Ally`, `game/game.go:278` | done | `PawnSubtitle` wording |
| RHUD-108 | Kind, person: job title or kind label | `PawnView.Title`, `game/game.go:112` | done | Same field |
| RHUD-109 | Kind, other person: trader type | `Visitor` plus `trade.TraderKind`, `domain/economy/trade/trade.go:16`; not on the pawn | missing | `dossierview.Card.Role` (DOS-1), empty for most |
| RHUD-110 | Kind, mech unit | `AnimalView.Mech`, `game/animals.go:104` | done | `sheet` label |
| RHUD-111 | Kind, wild: stray (very pet-like) | No petness value: grep for petness is empty | excluded | No pet-likeness stat |
| RHUD-112 | Kind, wild: predator | `Predator`, `PackPredator`, `domain/animal/animal.go:158` | partial | DOS-2 reads `animal.DefOf(species)` |
| RHUD-113 | Kind, wild: insect | `body.Insect`, `domain/animal/animal.go:128` | partial | DOS-2 same |
| RHUD-114 | Kind, wild: plain | Default | done | `sheet` label |
| RHUD-115 | Kind, tame: otherworldly entity | Anomaly DLC | excluded | No such creatures |
| RHUD-116 | Kind, tame: pet (petness above half) | No petness | excluded | No pet-likeness stat; a bonded animal is the nearest, `AnimalView.Bond` |
| RHUD-117 | Kind, tame: exotic pet | No petness | excluded | No pet-likeness stat |
| RHUD-118 | Kind, tame: hunting | `AnimalView.Hunts`, `game/animals.go` | partial | DOS-2 label from the flag |
| RHUD-119 | Kind, tame: pack | No pack flag: `animal.Pack` is pack predators; a caravan takes any tame animal at body size x 35 kg, `game/travel/pack.go:42` | excluded | The label would name every tame animal; dropped (DOS-1) |
| RHUD-120 | Kind, tame: farm | `AnimalView.Product`, `game/animals.go` | partial | DOS-2 label when it gives a product |
| RHUD-121 | Kind, tame: herd | `AnimalView.Herd`, `game/animals.go` | partial | DOS-2 label |
| RHUD-122 | Kind, tame: insect | `body.Insect` | partial | DOS-2 same as wild insect |
| RHUD-123 | Kind, tame: general | `AnimalTitle`, `ebiten/inspector/animal_inspector.go:69` | done | `sheet` label |
| RHUD-124 | Former-title note for mutants | Anomaly DLC; grep's one hit in `domain/animal/ecology.go` is flavour | excluded | No mutants |
| RHUD-125 | "Of faction" suffix | `GuestFaction`, `game/game.go:213`, guests only, `game/visitors.go:163` | partial | `dossierview.Card.Faction` also for raiders, from the raid group's `Faction`, `game/raid/mischief.go:141` (DOS-3) |
| RHUD-126 | Tooltip: bio | Bio tab only | missing | DOS-3 `sheet.BioTip` |
| RHUD-127 | Click opens Bio | Bio tab id `inspector.bio`, `ebiten/inspector/cmds_pawn.go:358` | missing | DOS-3 `Actions.OpenTab("bio")` |

#### C3. Identity line (`GenderRaceAndAge`)

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-128 | Gender | `PawnView.Sex`, `game/game.go:105`; `AnimalView.Female` | partial | DOS-2 `sheet.IdentityLine`; shown in Bio, not in the pane |
| RHUD-129 | Race label | Species for animals, `AnimalLabel`, `ebiten/inspector/animal_inspector.go:59`; people are all one kind | partial | DOS-2 omits the word for people |
| RHUD-130 | Xenotype variant | No genes | excluded | No gene system |
| RHUD-131 | Ideoligion-member variant | No ideoligion | excluded | No ideoligion system |
| RHUD-132 | Mutant variant | No mutants | excluded | No mutants |
| RHUD-133 | Age in years | `PawnSubtitle`, `ebiten/inspector/cmds_pawn.go:185` | done | Reused |
| RHUD-134 | Birthday today wording | `AgeDays`, `game/skills.go:464`, not in the view; `AgeYears` whole, `game/game.go:107` | missing | `dossierview.Card.AgeDayOfYear`; `sheet.IdentityLine` "Age 31, birthday today" (DOS-3) |
| RHUD-135 | One day past a birthday | Same | missing | "Age 31 and 1 day" (DOS-3) |
| RHUD-136 | N days past a birthday | Same | missing | "Age 31 and 12 days"; animals use `AgeDays`, `ebiten/inspector/animal_inspector.go:172` (DOS-3) |
| RHUD-137 | Faction tint on the line | None | missing | DOS-3 tint from `Card.Relation` |
| RHUD-138 | Tooltip: bio | None | missing | DOS-3 `sheet.BioTip` |
| RHUD-139 | Click opens Bio | None | missing | DOS-3 `Actions.OpenTab("bio")` |

#### C4. Body state line (`HealthCondition`, first match wins)

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-140 | Dead, critical | `Status`, `game/game.go:90`; line `ebiten/inspector/cmds_pawn.go:232` | partial | DOS-2 `sheet.BodyLine`, tone `critical` |
| RHUD-141 | Bleeding with under a day to live, "in {period}", critical | `Bleeding`, `BleedDeathHours`, `game/game.go:141`, `:143`; line `ebiten/inspector/cmds_pawn.go:264`; animals `game/animals.go:137` | partial | DOS-2 "Bleeding (death in 3h)" as today, tone `critical` below 24 hours |
| RHUD-142 | Untended: one condition by name | `ConditionView.Due`, `game/views_social.go:79`; `Wounded`, `game/game.go:131` | partial | DOS-2 first `Due` row from `Health(id)`; tone by next row |
| RHUD-143 | Untended: count of many | Same | missing | DOS-2 "3 wounds need tending" |
| RHUD-144 | Untended critical when any is life-threatening, else warning | `ConditionView.Losing`, `game/views_social.go:82`; `DeathInHours`, `:64` | missing | DOS-2 tone rule; uses `HealthView`, cached per tick; **visual warning** |
| RHUD-145 | Life-threatening: worst one by name | `Losing`, `Infection` lines, `ebiten/inspector/cmds_pawn.go:268` | partial | DOS-2 smallest `DueInHours`/`DeathInHours`; tone `critical` |
| RHUD-146 | Life-threatening: count of many | Same | missing | DOS-2 |
| RHUD-147 | Affected: one condition by pain | `Conditions`, `Pain`, `game/views_social.go:48`; no per-condition pain | partial | DOS-2 highest `SevPct` stands for worst pain; noted difference |
| RHUD-148 | Affected: two | Same | missing | DOS-2 "Cut, bruise" |
| RHUD-149 | Affected: many | Same | missing | DOS-2 "4 conditions" |
| RHUD-150 | Downed or incapacitated, warning | `Status "downed"`, `domain/pawn/health/health.go:16` | partial | DOS-2 tone `warning` |
| RHUD-151 | Health below 30 percent: critical | `HealthColor`, `ebiten/inspector/inspector.go:183` | partial | DOS-2 same cut, words original |
| RHUD-152 | Health below 60 percent: poor | Our cut is 70, `inspector.go:187` | partial | DOS-2 uses 60 |
| RHUD-153 | Above 90 with nothing bad: good | None | missing | DOS-2 |
| RHUD-154 | Otherwise: stable | None | missing | DOS-2 |
| RHUD-155 | Tooltip: health | `healthStateTip`, `ebiten/inspector/health_ui.go:271` | partial | DOS-2 `sheet.HealthTip` (family H) |
| RHUD-156 | Click opens Health | Tab `inspector.health`, `ebiten/inspector/cmds_pawn.go:373` | missing | DOS-3 `Actions.OpenTab("health")` |

#### C5. Mind state line (`MentalCondition`, first match wins)

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-157 | Stunned for N seconds, critical | Pawn stun only moves `NextMoveAt`, `game/body.go:582`; no flag; mechs `Stunned`, `game/animals.go:104` | missing | `dossierview.Card.StunTicks`; DOS-1 records the until tick in a transient map beside `hitHarm.Hold`; not saved; see Risks |
| RHUD-158 | Psychic trance variant | No psychic system: grep finds nothing | excluded | No psychic system |
| RHUD-159 | Staggered for N seconds, warning | Bool only, `game/game.go:146`, `game/body.go:528` | partial | `Card.StaggerTicks` from `motion.StaggerUntil` (DOS-3) |
| RHUD-160 | In a mental state: its label | `MentalBreak`, `game/game.go:161`; `mentalBreakLine`, `ebiten/inspector/pawn_state_ui.go:22` | done | Text reused |
| RHUD-161 | Mental state critical if aggressive or extreme, else warning | `TierOf`, `domain/pawn/mentalbreak/mentalbreak.go:127`; `MentalBreak` is a string | partial | `Card.BreakTier`, from the kind (DOS-3) |
| RHUD-162 | Extreme break imminent, critical | Thresholds never reach a view, `game/mind/risk.go:11`, `mentalbreak.go:118` | missing | `Card.BreakAt`; DOS-2 `TierAt`, "Near collapse"; **visual warning** |
| RHUD-163 | Major break imminent, warning | Same | missing | DOS-2 "Near a serious break" |
| RHUD-164 | Minor break imminent, warning | Same | missing | DOS-2 "Close to a minor break" |
| RHUD-165 | Inspired, with a period | `Inspiration`, `InspirationTicks`, `game/game.go:176`, `:177`; `InspirationLine`, `ebiten/inspector/gear_facts.go:105` | partial | DOS-2 adds the time left |
| RHUD-166 | Mood above 90: top wording | `MoodLabel`, `game/game.go:124` | partial | DOS-2 our own `mood.Label` text, tone `excellent` |
| RHUD-167 | Mood above 65: good wording | Same; tone cuts at 55, `ebiten/inspector/inspector.go:176` | partial | DOS-2 tone `good` at 65 |
| RHUD-168 | Otherwise: neutral wording | Same | partial | DOS-2 tone `neutral` |
| RHUD-169 | Tooltip: mood | Thoughts in Needs tab, `ebiten/inspector/inspector.go:290` | partial | DOS-2 `sheet.MindTip` |
| RHUD-170 | Click opens Needs | Tab `inspector.needs`, `ebiten/inspector/cmds_pawn.go:370` | missing | DOS-3 `Actions.OpenTab("needs")` |

#### C6. Need bars

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-171 | Bar widget: label, bar, value grid | `drawPaneBar`, `ebiten/inspector/hud_inspect.go:401`; `drawNeedBar`, `ebiten/inspector/inspector.go:127` | partial | DOS-3 `drawBar(label, fill, value, ticks, style)` |
| RHUD-172 | Health bar (`Health`) | `pawnBars`, `ebiten/inspector/cmds_pawn.go:199`; animals `game/animals.go:37` | done | Data ready; widget `vigour_bar` in DOS-3 |
| RHUD-173 | Health bar tooltip | None on bars | missing | DOS-3 `sheet.HealthTip` |
| RHUD-174 | Health bar click opens Health | None | missing | DOS-3 |
| RHUD-175 | Mood bar (`NeedMood`) | `pawnBars`, `:200` | done | Widget `mood_bar` |
| RHUD-176 | Mood bar ticks at the three break levels | None | missing | `Card.BreakAt`; DOS-3 ticks; test a trait shift moves them |
| RHUD-177 | Mood bar tooltip | Thoughts, `inspector.go:290` | partial | DOS-3 `sheet.MindTip` |
| RHUD-178 | Mood bar click opens Needs | None | missing | DOS-3 |
| RHUD-179 | Food bar (`NeedFood`) | `:201`; animals `Food`, `game/animals.go:81` | done | Widget `hunger_bar` |
| RHUD-180 | Food tooltip: diet | `AnimalDietLine`, `ebiten/inspector/animal_inspector.go:188`; people omnivores, `FoodPolicy`, `game/game.go:282` | partial | DOS-2 `sheet.FoodTip` |
| RHUD-181 | Food tooltip: eating speed | `foodEatSpeed`, `domain/pawn/body/cond_food.go:69`; not in a view | missing | `Card.EatSpeedPct` (DOS-1) |
| RHUD-182 | Food tooltip: hunger rate in bed | `stat.HungerRate`, `domain/pawn/stat/stat.go:14`; no bed effect: the Sleep job skips `decayNeeds`, bed or not, `game/tick.go:368` | partial | `Card.Stats[stat.HungerRate]`; the bed clause is dropped, since a bed changes nothing here (DOS-3) |
| RHUD-183 | Food bar click opens Needs | None | missing | DOS-3 |
| RHUD-184 | Rest bar (`NeedSleep`) | `:202`; animals `Rest`, `game/animals.go:81` | done | Widget `rest_bar` |
| RHUD-185 | Rest tooltip: rest rate | `stat.RestRate`, `stat.go:15` | missing | `Card.Stats`; DOS-2 `sheet.RestTip` |
| RHUD-186 | Rest bar click opens Needs | None | missing | DOS-3 |
| RHUD-187 | Energy bar (`NeedEnergy`) | Biotech | excluded | No energy need |
| RHUD-188 | Energy tooltip | Biotech | excluded | No energy need |
| RHUD-189 | Recreation bar (`NeedRecreation`) | `Recreation`, `game/game.go:160`; Needs tab, `inspector.go:269` | done | Widget `fun_bar` |
| RHUD-190 | Recreation tooltip line: beauty | `Beauty`, `inspector.go:269` | partial | DOS-2 `sheet.FunTip` |
| RHUD-191 | Recreation tooltip line: comfort | `Comfort`, same | partial | DOS-2 |
| RHUD-192 | Recreation tooltip line: outdoors | `Outdoors`, same | partial | DOS-2 |
| RHUD-193 | Recreation bar click opens Needs | None | missing | DOS-3 |
| RHUD-194 | Suppression bar (`NeedSuppression`) | Ideology | excluded | No slavery |
| RHUD-195 | Suppression tooltip | Ideology | excluded | No slavery |
| RHUD-196 | Suppression click opens the slave tab | Ideology | excluded | No slavery |
| RHUD-197 | Generic bar for any need, by definition (`Need`) | Comfort, beauty, outdoors bars in Needs, `inspector.go:269` | partial | DOS-2 catalog `need_bar` keys: comfort, beauty, outdoors |
#### C7. Skills

Our eleven skills are in `domain/pawn/skill/skill.go`. All data is on `PawnView.SkillLv`, `Passion` (`game/game.go:110`, `:111`) and `BioView.Skills` (`game/ng1_types.go:87`). The Bio tab table is `ebiten/inspector/bio_ui.go:225`. Each row below needs the same widget, DOS-3 `skill` with `Def` set to the skill key.

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-198 | Skill row: shooting | Bio table | partial | Widget `skill`, def `shooting` (DOS-3) |
| RHUD-199 | Skill row: melee | Bio table | partial | Def `melee` (DOS-3) |
| RHUD-200 | Skill row: construction | Bio table | partial | Def `construction` (DOS-3) |
| RHUD-201 | Skill row: mining | Bio table | partial | Def `mining` (DOS-3) |
| RHUD-202 | Skill row: cooking | Bio table | partial | Def `cooking` (DOS-3) |
| RHUD-203 | Skill row: plants | Bio table | partial | Def `plants` (DOS-3) |
| RHUD-204 | Skill row: animals | Bio table | partial | Def `animals` (DOS-3) |
| RHUD-205 | Skill row: crafting | Bio table | partial | Def `crafting` (DOS-3) |
| RHUD-206 | Skill row: artistic | No art skill: `skill.go` has eleven, none artistic | excluded | No artistic skill |
| RHUD-207 | Skill row: medicine | Bio table | partial | Def `medicine` (DOS-3) |
| RHUD-208 | Skill row: social | Bio table | partial | Def `social` (DOS-3) |
| RHUD-209 | Skill row: intellectual | Bio table | partial | Def `intellectual` (DOS-3) |
| RHUD-210 | Passion indicator on the label, one mark per level | `DrawPassion`, `ebiten/inspector/bio_ui.go:307`; `passion_minor`, `passion_major` icons | done | Export is fine inside `inspector`; DOS-3 draws the same `ui_icon/passion_*` keys |
| RHUD-211 | Colour: skill disabled | `Incapable`, `game/ng1_types.go:92` | partial | DOS-2 `sheet.SkillCell`, key `text_off` |
| RHUD-212 | Colour: learning saturated today | Cap exists: `learnCapDay = 4000`, `game/skills.go:40`, `lg.today`, `:348`; not in any view | missing | `Card.Saturated [skill.NumSkills]bool`; key `skill_full`; tip says the cap; test at the cap (DOS-3) |
| RHUD-213 | Colour: skill in use right now | `working`, `game/skills.go:295`, `workingTicks`, `lovesWork` at `:541`; not in any view | missing | `Card.ActiveSkill`; key `skill_busy`; same expiry as `lovesWork` (DOS-3) |
| RHUD-214 | Colour: minor passion | `Passion`, `game/ng1_types.go:91` | partial | Key `skill_like` (DOS-3) |
| RHUD-215 | Colour: major passion | Same | partial | Key `skill_love` (DOS-3) |
| RHUD-216 | Colour priority: off, saturated, active, passion, plain | None | missing | DOS-2 `SkillCell`, table test over every pair |
| RHUD-217 | Value shows a dash when totally disabled | Bio row, `ebiten/inspector/bio_ui.go:225` | partial | DOS-2 |
| RHUD-218 | Value shows the level | `SkillView.Level`, `LevelLabel` | done | Reused |
| RHUD-219 | Decimal progress when `ShowDecimals`, capped at 99 | `XPPct`, `game/ng1_types.go:93`, `domain/pawn/skill/skill.go:222` | partial | `config.HUDText.Decimals`; DOS-2 "12.45" |
| RHUD-220 | Tooltip: skill description | `BioSkillTip`, `ebiten/inspector/bio_ui.go:94`; `SkillView.Affects` | done | Text reused through `sheet.SkillTip` |
| RHUD-221 | Tooltip: global work speed line | `stat.WorkSpeed`, `domain/pawn/stat/stat.go:12`; `SpeedPct`, `game/ng1_types.go:94` | partial | `Card.Stats`; DOS-2 |
| RHUD-222 | Tooltip: general labour speed line | No such stat | excluded | One work speed stat here |
| RHUD-223 | Tooltip omits stats when disabled | `BioSkillTip`, `:97` | done | Same rule |
| RHUD-224 | Click opens Bio | None | missing | DOS-3 |

#### C8. Skill tooltip stat lines

Each line is one more line in `sheet.SkillTip`. Where a pure domain formula exists, DOS-2 calls it with the pawn's level. Where none exists the line is excluded rather than invented.

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-225 | Animals: gather speed | `SpeedPct`, handling work via `skill.ForWork`, `domain/pawn/skill/skill.go:108` | partial | DOS-2 from `SpeedPct` |
| RHUD-226 | Animals: gather yield | `animal.GatherYield`, `domain/animal/husbandry.go:279` | missing | DOS-2 calls it |
| RHUD-227 | Animals: tame chance | `animal.TameChance`, `domain/animal/husbandry.go:250` | missing | DOS-2 for a typical wildness; say which |
| RHUD-228 | Animals: train chance | `training.StepChance`, `domain/animal/training/training.go:47`, rolled at `game/train_animals.go:165` | partial | DOS-1 line in `Card.ExtraLines`, unbonded chance; `sheet` draws it |
| RHUD-229 | Animals: hunting stealth | No stealth: grep empty | excluded | No stealth stat |
| RHUD-230 | Construction: success chance | No failed builds: grep empty | excluded | Building never fails here |
| RHUD-231 | Construction: speed | `SpeedPct` | partial | DOS-2 |
| RHUD-232 | Construction: repair success | No such stat | excluded | Repair never fails here |
| RHUD-233 | Construction: smoothing speed | Same speed: `game/smooth.go:20`; `skillAffects`, `game/bio_build.go:30` | partial | DOS-2 shares the speed line |
| RHUD-234 | Cooking: speed | `SpeedPct` | partial | DOS-2 |
| RHUD-235 | Cooking: food poison chance | `CondFoodPoisoning`, `domain/pawn/body/cond_kinds.go:54`; chance formula not traced | partial | DOS-1 finds the formula or drops the line |
| RHUD-236 | Cooking: butchery efficiency | "butcher yield", `game/bio_build.go:31`; difficulty scale, `domain/difficulty/difficulty.go:74` | partial | DOS-1 |
| RHUD-237 | Cooking: butchery speed | `SpeedPct` | partial | DOS-2 |
| RHUD-238 | Cooking: drug cooking speed | No such stat | excluded | One cooking speed here |
| RHUD-239 | Crafting: smelting speed | `SpeedPct`; `recipe.SmeltPct`, `domain/economy/recipe/benches.go:93` | partial | DOS-2 |
| RHUD-240 | Crafting: mech butchery speed | No such recipe found | excluded | No mech butchery |
| RHUD-241 | Crafting: mech butchery efficiency | Same | excluded | No mech butchery |
| RHUD-242 | Intellectual: research speed | `SpeedPct`, `game/bio_build.go:37` | partial | DOS-2 |
| RHUD-243 | Intellectual: hacking speed | No hacking: grep hits are rock digging | excluded | No hacking |
| RHUD-244 | Medicine: operation speed | Surgery runs on `workProgress(id, skMedicine, true)`, `game/surgery.go:679`, so `SpeedPct` is the operation speed too | partial | DOS-2 shares the speed line |
| RHUD-245 | Medicine: surgery success | `OperationView.ChancePct`, `game/views_social.go:128` per operation | partial | DOS-1 a base chance at this level, or the line is dropped |
| RHUD-246 | Medicine: tend speed | `SpeedPct`; no tend-speed stat | partial | DOS-2 |
| RHUD-247 | Medicine: tend quality | `body.TendQuality`, `domain/pawn/body/tend.go:91` | missing | DOS-2 calls it with no medicine |
| RHUD-248 | Melee: damage per second | `PawnView.WeaponStats`, `game/game.go:1377`, a text line | partial | DOS-2 prints that line |
| RHUD-249 | Melee: hit chance | `attack.HitChanceOf`, `domain/combat/attack/melee.go:60`; `stat.MeleeAcc` | missing | `Card.Stats` plus DOS-1 helper |
| RHUD-250 | Melee: dodge | `movingDodge`, `domain/combat/attack/melee.go:82` | missing | DOS-1 exports the chance |
| RHUD-251 | Mining: yield | "ore and chunk yield", `game/bio_build.go:30`; formula not traced | partial | DOS-1 |
| RHUD-252 | Mining: speed | `SpeedPct` | partial | DOS-2 |
| RHUD-253 | Mining: deep drilling speed | Deep drill runs, `game/axview/axview.go` `Cycle`; skill link not traced | partial | DOS-2 shares the speed line |
| RHUD-254 | Plants: work speed | `SpeedPct` | partial | DOS-2 |
| RHUD-255 | Plants: harvest yield | "harvest yield", `game/bio_build.go:32`; `HarvestYield`, `domain/world/plant/farm.go:25` | partial | DOS-1 skill factor |
| RHUD-256 | Plants: pruning | Ideology | excluded | No pruning |
| RHUD-257 | Shooting: accuracy | `stat.ShootAcc`, `domain/pawn/stat/stat.go:20` | missing | `Card.Stats`; DOS-2 |
| RHUD-258 | Shooting: aiming delay | `stat.AimCooldown`, `stat.go:22` | missing | `Card.Stats`; DOS-2 |
| RHUD-259 | Social: negotiation ability | `stat.NegotiationAbility`, `stat.go:28` | missing | `Card.Stats` (DOS-3) |
| RHUD-260 | Social: trade price improvement | `stat.TradePriceImprovement`, `stat.go:29` | missing | `Card.Stats` (DOS-3) |
| RHUD-261 | Social: pawn beauty | Our `stat.Beauty` is furniture, `stat.go:34` | excluded | No pawn beauty |
| RHUD-262 | Social: social impact | `stat.SocialImpact`, `stat.go:27` | missing | `Card.Stats` (DOS-3) |
| RHUD-263 | Social: conversion power | Ideology | excluded | No ideoligion system |
| RHUD-264 | Social: suppression power | Ideology | excluded | No slavery |
| RHUD-265 | Artistic: base lines only | No art skill | excluded | No artistic skill |

#### C9. Training (creature target)

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-266 | Training row: tameness | Taming is a one-off, `AnimalView.Tame`; `training.go` has no such step count | excluded | No tameness training |
| RHUD-267 | Training row: obedience | `training.Obedience`, `domain/animal/training/training.go:7`; `drawAnimalTrainingTab`, `ebiten/inspector/animal_inspector.go:340` | partial | DOS-3 widget `train`, def `obedience` |
| RHUD-268 | Training row: release | `training.Release`, `:9` | partial | Def `release` (DOS-3) |
| RHUD-269 | Training row: rescue | No such trainable: three skills only, `training.go:7`; grep on `domain/animal` empty | excluded | No rescue training |
| RHUD-270 | Training row: haul | Same | excluded | No haul training |
| RHUD-271 | Training row: attack target (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-272 | Training row: comfort (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-273 | Training row: dig (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-274 | Training row: egg spew (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-275 | Training row: forage (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-276 | Training row: sludge spew (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-277 | Training row: terror roar (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-278 | Training row: thrumbo roar (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-279 | Training row: war trumpet (Odyssey) | DLC | excluded | Odyssey trainable |
| RHUD-280 | Hidden unless the species can learn it | `animalTrainable`, `ebiten/inspector/animal_inspector.go:245` | done | `AnimalTrainingView.Allowed`, `game/animals.go:158` |
| RHUD-281 | Colour: not wanted | `trainState`, `animal_inspector.go:328`; `On`, `game/animals.go:159` | partial | DOS-2 `sheet.TrainCell` |
| RHUD-282 | Colour and bold: learned | `Learned`, `:160` | partial | DOS-2 |
| RHUD-283 | Value: steps of total | `trainLabel`, `animal_inspector.go:256` | done | Reused |
| RHUD-284 | Tooltip: animal summary plus what it teaches | `drawAnimalTrainingTab`, `:340` | partial | DOS-2 `sheet.AnimalTip` |
| RHUD-285 | Click opens Training | Animal tab exists, `ebiten/inspector/cmds_thing.go:238` | missing | DOS-3 `Actions.OpenTab("training")` |

#### C10. Other values

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-286 | Stat widget, "label: value" (`Stat`) | No view of the sheet: `statOf`, `game/skills.go:217`; `Sheet`, `domain/pawn/stat/stat.go:39` | missing | `Card.Stats stat.Sheet`; DOS-2 `stat` widget, keys from `stat.ID`; add `stat.ID.Format` in the domain for units |
| RHUD-287 | Stat hidden when disabled for the pawn | No such concept beyond incapable | excluded | Our stats never disable; the widget hides only `-1` |
| RHUD-288 | Record widget, "label: value" (`Record`) | `Kills`, `DamageDealt`, `game/game.go:198`; `windows/records.go:26` | partial | DOS-2 `tally`, keys `kills`, `damage`; no other records are kept |
| RHUD-289 | Time records print as periods | No time records | excluded | No record of that type; add when `game` keeps one |
| RHUD-290 | Master line (`Master`) | `AnimalBondRows`, `ebiten/inspector/animal_inspector.go:226`; `Master`, `game/animals.go:116` | partial | DOS-3 widget `keeper` |
| RHUD-291 | Master tinted when a bond exists | `Bond`, `game/animals.go` | missing | DOS-2 key `skill_like` when `Bond != ""` |
| RHUD-292 | Master tooltip: animal | None | missing | DOS-2 `sheet.AnimalTip` |
| RHUD-293 | Master click opens Social | Animals have no Social tab | partial | DOS-3 opens Training, where the master command sits; noted difference |
| RHUD-294 | Activity line (`Activity`) | "Job: …", `ebiten/inspector/cmds_pawn.go:305`; animals `AnimalStateLine`, `animal_inspector.go:138` | partial | DOS-3 widget `doing` |
| RHUD-295 | Activity from a group order (lord report) | `RaidPlan` on raiders, `game/game.go`; `Fleeing`, `Acting` | partial | DOS-2 uses them ahead of `Job` |
| RHUD-296 | Activity names the attack target | `TargetID` on animals, `game/animals.go`; pawn target not in the view | missing | `Card.AttackTarget` (DOS-1) |
| RHUD-297 | Activity prefix and bold | None | missing | DOS-3 bold style |
| RHUD-298 | Activity tooltip: work type | `Card.WorkType`, new: `dossierview.WorkTypeOf` from the job kind, following `assignWorkType`, `game/priorities.go:62`; a bill by its recipe's skill | missing | DOS-1 fills it; `sheet` draws it |
| RHUD-299 | Activity tooltip: relevant skills | `skill.ForWork`, `domain/pawn/skill/skill.go:96` | missing | DOS-2 from `Card.WorkType` |
| RHUD-300 | Activity click opens Work | `ids.TabWork`, `ebiten/ids/ids.go:48`; `ports.ShowWindow`, `ebiten/ports/ports.go:147` | missing | DOS-3 |
| RHUD-301 | Queued line: first order plus how many more (`Queued`) | Every order listed, `ebiten/inspector/cmds_pawn.go:315`; `QueuedOrderView`, `game/order_queue.go:31` | partial | DOS-2 "Next: … (+2)" |
| RHUD-302 | Equipped line: restraints when held (`Equipped`) | Prisoner state; no restraint item | partial | DOS-2 "Held captive" wording when `Prisoner` |
| RHUD-303 | Equipped line: weapon | `Weapon`, `game/game.go:193`; `WeaponTitle`, `ebiten/inspector/gear_ui.go:26` | done | Reused |
| RHUD-304 | Equipped click opens Gear | Tab `inspector.gear`, `ebiten/inspector/cmds_pawn.go:363` | missing | DOS-3 |
| RHUD-305 | Carrying line (`Carrying`) | `Carrying`, `game/game.go:245`; `carryName`, `cmds_pawn.go:286` | partial | DOS-3 widget `hauling` |
| RHUD-306 | Carrying click opens Gear | None | missing | DOS-3 |
| RHUD-307 | Traits, comma list (`Traits`) | `BioView.Traits`, `game/ng1_types.go:112`, Bio tab | partial | DOS-3 widget `traits` |
| RHUD-308 | Traits tooltip: each with a description | `TraitView.Desc`, `game/ng1_types.go:100` | partial | DOS-2 `sheet.TraitsTip` |
| RHUD-309 | Component info: emerging and recharge lines (`CompInfo`) | DLC | excluded | DLC-only comps |
| RHUD-310 | Component info: quest strings | No quest read model found in `game/game.go`; spec §31 | excluded | No quest strings on pawns |
| RHUD-311 | Component info: comp strings joined | Status lines, `ebiten/inspector/cmds_pawn.go:230`; animal lines, `animal_inspector.go:178` | partial | DOS-3 widget `notes` reusing `AnimalPregLine`, `AnimalHuntLine`, `ExposureLines` |
| RHUD-312 | Prisoner line: resistance (`PrisonerInfo`) | `PrisonerView.Resistance`, `game/views_social.go:141`; tab, `ebiten/inspector/prisoner_ui.go:146` | partial | DOS-3 widget `captive` |
| RHUD-313 | Prisoner line: unrecruitable | No such state in `PrisonerView` | excluded | Every prisoner is recruitable |
| RHUD-314 | Prisoner line: no-faction variant | `GuestFaction`, `game/game.go:213` | partial | DOS-2 wording |
| RHUD-315 | Prisoner line: will level | Ideology | excluded | No will stat |
| RHUD-316 | Prisoner line click opens Prisoner | `PrisonerTabID`, `ebiten/inspector/cmds_pawn.go:366` | missing | DOS-3 |

### D. Targets and default layouts

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-317 | Target: player humanlike (H) | Colonist branch, `ebiten/inspector/hud_inspect.go:94` | partial | `sheet.Who` value `c`; `sheet.WhoOf(PawnView)` (DOS-3) |
| RHUD-318 | Target: other humanlike (h) | `Prisoner`, `Visitor`, `Ally`, `Hostile`, `game/game.go:189`, `:276` | partial | `s` (DOS-3) |
| RHUD-319 | Target: player creature (C) | `AnimalView.Tame`, `game/animals.go:46` | partial | `p` (DOS-3) |
| RHUD-320 | Target: other creature (c) | Wild, mechs, `game/animals.go:104` | partial | `w`; a mech reads as `w` (DOS-3) |
| RHUD-321 | Colour by relation: own, hostile, ally, prisoner, none | None | missing | DOS-2 `sheet.Relation` from `Card.Relation`, table test |
| RHUD-322 | Default docked layout, player humanlike | None | missing | DOS-2 `sheet.DefaultDocked` original arrangement that fits `PaneH` |
| RHUD-323 | Default docked layout, other humanlike | None | missing | Same tree, filtered (DOS-3) |
| RHUD-324 | Default docked layout, player creature | None | missing | Same tree, filtered (DOS-3) |
| RHUD-325 | Default docked layout, other creature | None | missing | Same tree, filtered (DOS-3) |
| RHUD-326 | Default floating layout, player humanlike | None | missing | DOS-2 `sheet.DefaultFloating` |
| RHUD-327 | Default floating layout, other humanlike | None | missing | Same tree, filtered (DOS-3) |
| RHUD-328 | Default floating layout, player creature | None | missing | Same tree, filtered (DOS-3) |
| RHUD-329 | Default floating layout, other creature | None | missing | Same tree, filtered (DOS-3) |

### E. Layout editor

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-330 | Tree of the current mode's elements | None | missing | DOS-6 `setup_sections.go` on `uikit.ScrollArea`, `ebiten/uikit/scroll.go:64`; edits a copy of the tree |
| RHUD-331 | Selectable rows with indent | `uikit` list, `scroll.go:140` | missing | DOS-6 |
| RHUD-332 | Collapse and expand | None | missing | DOS-6 local state |
| RHUD-333 | Row colour per element type | None | missing | DOS-6, colour keys `tree_*` |
| RHUD-334 | Row shows attributes: bar style, fill, targets | None | missing | DOS-6 |
| RHUD-335 | Move up | None | missing | DOS-2 `sheet.Move(path, -1)`; DOS-6 button |
| RHUD-336 | Move down | None | missing | DOS-2 `sheet.Move(path, +1)` |
| RHUD-337 | Remove | None | missing | DOS-2 `sheet.Remove(path)` |
| RHUD-338 | Add stack with vertical or horizontal menu | None | missing | DOS-2 `sheet.Add`; DOS-6 `uikit.Picker` |
| RHUD-339 | Add panel | None | missing | DOS-2 |
| RHUD-340 | Add row | None | missing | DOS-2 |
| RHUD-341 | Description tooltip on each add entry | None | missing | DOS-2 `sheet.Catalog[key].Tip`, original text |
| RHUD-342 | Add common value widget menu | None | missing | DOS-2 `sheet.Catalog`, group `values` |
| RHUD-343 | Add third-party widget menu | No mods | excluded | No plugin format (family K) |
| RHUD-344 | Add bar, with a colour-style submenu | None | missing | DOS-6 two pickers; sets `BarStyle` |
| RHUD-345 | Add skill or training | None | missing | DOS-2 catalog from `skill.ID`, `training.Skill` |
| RHUD-346 | Add stat | None | missing | DOS-2 catalog from `stat.ID` |
| RHUD-347 | Add record | None | missing | DOS-2 catalog `tally` |
| RHUD-348 | Definition tooltip shows its key and source mod | None | partial | Shows the key; no mod source until plugins exist (DOS-6) |
| RHUD-349 | Selected block: label | None | missing | DOS-6 |
| RHUD-350 | Selected block: fill checkbox for stack or panel | `uikit.Kit.Check`, `ebiten/uikit/check.go:27` | missing | DOS-6 |
| RHUD-351 | Selected block: four target checkboxes, last one locked | None | missing | DOS-2 `sheet.SetWho` refuses empty; DOS-6 disabled reason |
| RHUD-352 | Any edit rebuilds the card live and clears the current preset | None | missing | DOS-6 writes `config.HUD.Layouts` and clears `Preset` through `SetSettings`, `ebiten/options/deps.go:45` |
| RHUD-353 | Editor follows the current mode | None | missing | DOS-6 reads `HUD.Docked` |

### F. Presets

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-354 | Built-in default preset | None | missing | DOS-2 `sheet.Standard`; display "Standard" |
| RHUD-355 | Built-in classic-style preset | None | missing | DOS-2 "Roomy": every row for every target |
| RHUD-356 | Built-in compact preset with smaller text | None | missing | DOS-2 "Snug": smaller `HUDText`, food, rest, activity narrowed by target; our own sizes |
| RHUD-357 | Built-in preview preset adding a traits row | None | missing | DOS-2 "Snug with traits" |
| RHUD-358 | Preset select menu with source label | None | missing | DOS-6 picker: built-in or player's |
| RHUD-359 | Load a preset | None | missing | DOS-6 `sheet.Apply(preset, *config.HUD)` |
| RHUD-360 | Save current | None | missing | DOS-6 "Keep as preset…" dialog; `cardpresetfile.Store.Save` |
| RHUD-361 | Save flag: include docked layout | None | missing | `sheet.SaveFlags.Docked`; checkbox "Pane sections" (DOS-6) |
| RHUD-362 | Save flag: include floating layout | None | missing | `SaveFlags.Floating`; "Card sections" (DOS-6) |
| RHUD-363 | Save flag: include width | None | missing | `SaveFlags.Width`; "How wide" (DOS-6) |
| RHUD-364 | Save flag: include height | None | missing | `SaveFlags.Height`; "How tall" (DOS-6) |
| RHUD-365 | Save flag: include tab count | None | missing | `SaveFlags.Tabs`; "Tab count" (DOS-6) |
| RHUD-366 | Save flag: include text sizes | None | missing | `SaveFlags.Text`; "Type sizes" (DOS-6) |
| RHUD-367 | Name validation: no reserved names, length limit | `scenariofile.ValidName`, `internal/adapters/outbound/scenariofile/scenariofile.go:47` | partial | DOS-6 same slug rule plus built-in names |
| RHUD-368 | Manage dialog: list of the player's presets | None | missing | DOS-6 modal "Presets…" |
| RHUD-369 | Delete selected, with confirm | `scenariofile.Store` pattern | missing | DOS-6 `Store.Delete` |
| RHUD-370 | Close button | `uikit.Modal`, `ebiten/uikit/modal.go:40` | done | Kit gives it |
| RHUD-371 | Alert: preset loaded | `Hint`, `ebiten/inspector/deps.go` `Host` | missing | DOS-6 |
| RHUD-372 | Alert: default loaded | Same | missing | DOS-6 |
| RHUD-373 | Alert: invalid preset, falls back to default | Same | missing | DOS-6 `sheet.Validate` |
| RHUD-374 | Alert: saved | Same | missing | DOS-6 |
| RHUD-375 | Warning for an older preset version | None | missing | `sheet.PresetVersion`; DOS-6 hint |
| RHUD-376 | Back to the default layouts from the preset menu | None | missing | DOS-6 picker entry "Standard" clears `Layouts` and sets `Preset` |
| RHUD-377 | Load by dropping a file in the presets folder; list refreshes on open | Scenario folder precedent, `scenariofile.go:37` | missing | DOS-6 `<config>/rimworld-go/dossier/*.json`; `Store.List` on open |
| RHUD-378 | Open the config folder | See family A5 | missing | DOS-6 "Show card files" |
| RHUD-379 | Preset file format with version, text sizes, roots | None | missing | DOS-2 JSON `sheet.Preset`; golden file test; malformed files never panic |
| RHUD-380 | Layouts kept while no preset is current | None | missing | `config.HUD.Layouts` with `Preset == ""` (DOS-6) |

### G. Theme

#### G1. Text and sizes

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-381 | Regular text size | `gfx.Body`, `ebiten/gfx/text.go:46`; shared const | missing | `config.HUDText.BodySize` (7..20); DOS-3 local styles, global `gfx` tokens untouched |
| RHUD-382 | Regular line height percent | `gfx.LineH`, `ebiten/gfx/text.go:179` | missing | `BodyLinePct` (100..250); DOS-3 `step = LineH * pct / 100` |
| RHUD-383 | Large size as an offset | `gfx.Large`, `text.go:48` | missing | `LargeDelta` (0..24) (DOS-5) |
| RHUD-384 | Large line height percent | Same | missing | `LargeLinePct` (DOS-5) |
| RHUD-385 | Small size as an offset | `gfx.Small`, `text.go:45` | missing | `SmallDelta` (-5..0) (DOS-5) |
| RHUD-386 | Small line height percent | Same | missing | `SmallLinePct` (DOS-5) |
| RHUD-387 | Label column width | 100 px constant, `ebiten/inspector/inspector.go:144` | missing | `config.HUDText.LabelW` (30..150) (DOS-5) |
| RHUD-388 | Value column width | Measured | missing | `config.HUDText.ValueW` (30..150) (DOS-5) |
| RHUD-389 | Show decimals | None | missing | `config.HUDText.Decimals` (DOS-5) |
| RHUD-390 | Letter padding | `HudGap` constant, `ebiten/hudlayout/layout.go` | missing | `config.HUDCard.LetterGap` (0..12) (DOS-5) |
| RHUD-391 | Compress notifications | See family J | missing | `config.HUDCard.CompressLetters` (DOS-5) |

#### G2. Colours

Every row is one colour key. A key maps to a `uikit` token by default and to `config.HUD.Colors[key]` when the player changed it, following `config.Controls.Bindings` (only differences are stored). Default tokens are picked by DOS-0 art from our palette; none are taken from the mod. Palette tokens are `ebiten/uikit/widgets.go:16`, `assets/palettes/ui.yaml`.

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-392 | Colour: line | `PanelBorder`, `widgets.go:20` | partial | Key `rule` (DOS-5) |
| RHUD-393 | Colour: main text | `gfx` default ink | partial | Key `text` (DOS-5) |
| RHUD-394 | Colour: disabled | `TextDisabled`, `widgets.go:31` | partial | Key `text_off` (DOS-5) |
| RHUD-395 | Colour: critical | `Danger`, `widgets.go:34` | partial | Key `tone_critical` (DOS-5) |
| RHUD-396 | Colour: warning | `Warning`, `widgets.go:33` | partial | Key `tone_warning` (DOS-5) |
| RHUD-397 | Colour: neutral | `TextDim`, `widgets.go:30` | partial | Key `tone_neutral` (DOS-5) |
| RHUD-398 | Colour: good | `Good`, `widgets.go:35` | partial | Key `tone_good` (DOS-5) |
| RHUD-399 | Colour: excellent | No token | missing | New token `ui.dossier_excellent`; key `tone_top` (DOS-5) |
| RHUD-400 | Colour: bar background | `SliderTrack`, `widgets.go:39` | partial | Key `bar_back` (DOS-5) |
| RHUD-401 | Colour: bar main | `SliderFill`, `widgets.go:40` | partial | Key `bar_main` (DOS-5) |
| RHUD-402 | Colour: bar low | `Danger` | partial | Key `bar_low` (DOS-5) |
| RHUD-403 | Colour: bar threshold tick | No token | missing | New token; key `bar_tick` (DOS-5) |
| RHUD-404 | Colour: selector text | `TextDim` | partial | Key `picker_text` (DOS-5) |
| RHUD-405 | Colour: selector back | `Field`, `widgets.go:36` | partial | Key `picker_back` (DOS-5) |
| RHUD-406 | Colour: minor passion | No token | missing | New token; key `skill_like` (DOS-5) |
| RHUD-407 | Colour: major passion | No token | missing | New token; key `skill_love` (DOS-5) |
| RHUD-408 | Colour: learning saturated | No token | missing | New token; key `skill_full` (DOS-5) |
| RHUD-409 | Colour: active skill | No token | missing | New token; key `skill_busy` (DOS-5) |
| RHUD-410 | Colour: own faction | `Good` | partial | Key `side_own` (DOS-5) |
| RHUD-411 | Colour: allied | No token | missing | New token; key `side_ally` (DOS-5) |
| RHUD-412 | Colour: independent | `TextDim` | partial | Key `side_free` (DOS-5) |
| RHUD-413 | Colour: hostile | `Danger` | partial | Key `side_foe` (DOS-5) |
| RHUD-414 | Colour: wild | No token | missing | New token; key `side_wild` (DOS-5) |
| RHUD-415 | Colour: prisoner | `Warning` | partial | Key `side_held` (DOS-5) |
| RHUD-416 | Colour: slave | No slavery | excluded | No slavery system |
| RHUD-417 | Editor-only colours for stack, panel, row, widget | None | missing | Keys `tree_stack`, `tree_block`, `tree_line`, `tree_widget`; not player-editable (DOS-5) |
| RHUD-418 | Editor-only colours for missing, built-in, player's, external, selected | `BtnSelected`, `widgets.go:23` | partial | Keys `tree_bad`, `tree_core`, `tree_mine`; the external one is dropped (DOS-5) |

#### G3. Colour editor

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-419 | Colours grouped: card, faction, skill | None | missing | DOS-5 `setup_colours.go` |
| RHUD-420 | Sample swatch | None | missing | DOS-5 |
| RHUD-421 | Hex RGBA entry | `uikit.TextField`, `ebiten/uikit/textfield.go:470`; `DigitsOnly` shows filters, `:39` | missing | DOS-5 hex filter; `config` parses `#RRGGBBAA` |
| RHUD-422 | Hue slider and entry | `Kit.SliderInt`, `ebiten/uikit/slider.go:165` | missing | DOS-5; `sheet.RGBToHSL`, `HSLToRGB` pure with round-trip test |
| RHUD-423 | Saturation slider and entry | Same | missing | DOS-5 |
| RHUD-424 | Lightness slider and entry | Same | missing | DOS-5 |
| RHUD-425 | Alpha slider and entry | Same | missing | DOS-5 |
| RHUD-426 | Reset one colour | Stored only as a difference | missing | DOS-5 deletes the map key |
| RHUD-427 | Reset all colours | See family A5 | missing | DOS-5 clears the map |

#### G4. Bar colour styles

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-428 | Style: low fades to main | `needColor`, stepped, `ebiten/inspector/inspector.go:196` | partial | `sheet.BarStyle` `fade_up`, `uikit.LerpRGBA` (DOS-5) |
| RHUD-429 | Style: low colour only | None | missing | `low_only` (DOS-5) |
| RHUD-430 | Style: main fades to low | None | missing | `fade_down` (DOS-5) |
| RHUD-431 | Style: main colour only | `drawNeedBar` default, `inspector.go:135` | partial | `main_only` (DOS-5) |
| RHUD-432 | Thresholds on any bar | None | missing | DOS-3 `drawBar` ticks; only mood fills them, as in the mod |
### H. Tooltips

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-433 | Bio tooltip: title and faction | `BioView`, `game/ng1_types.go:104`; Bio tab, `ebiten/inspector/bio_ui.go:114` | partial | DOS-2 `sheet.BioTip` |
| RHUD-434 | Bio tooltip: ideoligion line | No ideoligion | excluded | No ideoligion system |
| RHUD-435 | Bio tooltip: childhood | `BioView.Childhood`, `:108` | partial | DOS-2 |
| RHUD-436 | Bio tooltip: adulthood | `BioView.Adulthood`, `:110` | partial | DOS-2 |
| RHUD-437 | Bio tooltip: traits list | `BioView.Traits`, `:112` | partial | DOS-2 |
| RHUD-438 | Bio tooltip: incapable list, subdued | `BioView.Incapable`, `:114`; `BioIncapable`, `bio_ui.go:70` | partial | DOS-2 |
| RHUD-439 | Bio tooltip: psychic sensitivity | No psychic system | excluded | No psychic system |
| RHUD-440 | Bio tooltip: meditation | Royalty | excluded | No psyfocus |
| RHUD-441 | Bio tooltip: ideoligion stats | Ideology | excluded | No ideoligion system |
| RHUD-442 | Bio tooltip on a creature gives the animal one | `AnimalBlurb`, `ebiten/inspector/deps.go` `Labels` | partial | DOS-2 |
| RHUD-443 | Animal tooltip: trainability | `MinHandling`, `Wildness`, `game/animals.go`; no trainability class | partial | DOS-2 prints minimum handling instead |
| RHUD-444 | Animal tooltip: training decay interval | `training.DecayTicks`, `domain/animal/training/training.go:25` | missing | DOS-2 |
| RHUD-445 | Animal tooltip: tameness decay note | No tameness decay | excluded | Tame is permanent here, save for `ReleaseAnimal` |
| RHUD-446 | Animal tooltip: petness | No petness | excluded | No pet-likeness stat |
| RHUD-447 | Animal tooltip: diet | `AnimalDietLine`, `ebiten/inspector/animal_inspector.go:188`; `AnimalFeedLine`, `:199` | done | Reuse |
| RHUD-448 | Animal tooltip: master | `AnimalBondRows`, `:223` | done | Reuse |
| RHUD-449 | Animal tooltip: training description | No descriptions | missing | DOS-2 original text per `training.Skill` |
| RHUD-450 | Health tooltip: per condition, grouped by part in order | `PartView`, `ConditionView`, `game/views_social.go:76`, `:106`; `healthParts`, `ebiten/inspector/health_ui.go:428` | partial | DOS-2 `sheet.HealthTip`; animal rows `AnimalPartRows`, `animal_inspector.go:406` |
| RHUD-451 | Health tooltip colour: not bad | `uikit.Kit.Tip` is plain text, `ebiten/uikit/tooltip.go:74` | missing | DOS-3 own tooltip drawing with per-line tone |
| RHUD-452 | Health tooltip colour: permanent or immune | `ImmPct`, `Status` | missing | DOS-2 tone rule |
| RHUD-453 | Health tooltip colour: addiction or tended | `DrugLineView`, `game/drugs.go`; `TendPct`, `game/views_social.go:78` | missing | DOS-2 tone rule |
| RHUD-454 | Health tooltip colour: otherwise critical | Same | missing | DOS-2 tone rule |
| RHUD-455 | Health tooltip: nothing wrong | None | missing | DOS-2 "Nothing wrong" in `text_off` |
| RHUD-456 | Mood tooltip: thoughts in order | `Thoughts`, `game/game.go:125`; cap of five, `inspector.go:291` | partial | DOS-2 `sheet.MindTip` shows all |
| RHUD-457 | Mood tooltip: stack multiplier | `ThoughtView{Label, Value}`, `game/rooms.go:47`; no count | missing | `Card.ThoughtStacks` or merge equal labels in DOS-2; merging chosen, no sim field |
| RHUD-458 | Mood tooltip: value tones at the four cuts | `+N` plain, `inspector.go:294` | missing | DOS-2 tones at -10, 0, 10 |
| RHUD-459 | Mood tooltip: inspiration line | `InspirationLine`, `gear_facts.go:105` | done | Reuse |
| RHUD-460 | Trait tooltip with title and description | `TraitView`, `game/ng1_types.go:99` | partial | DOS-2 |
| RHUD-461 | Compact tooltip style, regular text and a narrow width | Card: bold title, small body, 360 wide, `ebiten/uikit/tooltip.go:33` | missing | DOS-3 draws its own card through `Kit.Queue`, `ebiten/uikit/kit.go:127`; no new Kit field or method (Kit is at 30 fields, 73 methods) |
| RHUD-462 | Tooltip drawing patch: inner text | Same | missing | DOS-3 `drawTip` |
| RHUD-463 | Tooltip drawing patch: box rect | `TipLayout`, `tooltip.go:33` | partial | DOS-3 reuses the placing maths with its own width |
| RHUD-464 | Empty tooltip shows nothing | `TipVisible`, `tooltip.go:69` | done | Same rule |
| RHUD-465 | Multiple tips kept apart by id | Single tip per frame, `tooltip.go:74` | done | One hover at a time here |

### I. Visual warnings

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-466 | Life-threatening condition in the critical colour | Pane lines: `PaneLineWarns`, `ebiten/inspector/cmds_power.go:118`; bars `HealthColor`, `inspector.go:183` | partial | DOS-2 tone on `BodyLine`; DOS-3 draws it |
| RHUD-467 | Needs tending in the warning or critical colour | "Wound untended", `cmds_pawn.go:273`; `untended` alert, `ebiten/hud/alerts.go:691` | partial | DOS-2 |
| RHUD-468 | Near a mental break in the warning or critical colour | `unhappy` alert, `alerts.go:718`, `break`, `:705`; no per-pawn warning | missing | DOS-2 with `Card.BreakAt` |
| RHUD-469 | Bar low colour by fill | Stepped, `inspector.go:196` | partial | DOS-3 per style |
| RHUD-470 | Mood bar ticks | See family C6 | missing | DOS-3 |

### J. Letter stack

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-471 | Compress option on while floating | Fixed rows, `ebiten/hudlayout/layout.go:175` | missing | `config.HUDCard.CompressLetters`; `Inputs.LetterAvoid gfx.Rect` (DOS-7) |
| RHUD-472 | Count letters that would overlap the card | `room`, `layout.go:181` from the column only | missing | DOS-7 subtract the rect when the card's anchor is on the right |
| RHUD-473 | Fold the oldest into one bundle | "+N older" row, `ebiten/hud/letters.go:96`, `StackSplit`, `ebiten/hud/alerts.go:425` | done | Our fold already exists; DOS-7 reuses it |
| RHUD-474 | Bundle row clickable | `Older` row has no button, `letters.go:143` | missing | DOS-7 button opens the newest folded letter; `TestJourneyLetterOlderOpens` |
| RHUD-475 | Padding between letters | `HudGap` | missing | `Inputs.LetterGap` from `HUDCard.LetterGap` (DOS-7) |
| RHUD-476 | Slider only active when compress is on and floating | None | missing | DOS-5 disabled with a reason |
| RHUD-477 | Top of the stack published for other UI | `l.Letters`, `layout.go:187`; `stackBottom` | done | `Layout` carries it |
| RHUD-478 | Only while floating | None | missing | DOS-7 `LetterAvoid` empty when docked |

### K. Modders and integrations

`mods/` and `content/` hold only `.gitkeep`. `internal/adapters/outbound/modloader/doc.go:1` is a stub, `cmd/contentlint/main.go:3` is a TODO, and the menu's Plugins entry is disabled, `ebiten/mainmenu/screen.go:74`. Spec §45–§48 is unbuilt. There is no format to map to, so custom widgets are excluded; the preset folder is the one open surface.

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-479 | External widget base with API version | No plugin format | excluded | Blocked on §45; no Go plugin ABI is proposed |
| RHUD-480 | Custom drawn widget (`CustomWidgetDef`) | Same | excluded | Would run foreign draw code |
| RHUD-481 | Custom value (`CustomValueDef`) | Same | excluded | No plugin format |
| RHUD-482 | Custom bar (`CustomBarDef`) | Same | excluded | No plugin format |
| RHUD-483 | Custom selector (`CustomSelectorDef`) | Same | excluded | No plugin format |
| RHUD-484 | Custom need (`CustomNeedDef`) | Same | excluded | No plugin format |
| RHUD-485 | Presets shipped inside a mod's folder | Same | excluded | The player's preset folder is the honest equivalent (family F) |
| RHUD-486 | Preset `Requires` attribute | Same | excluded | Nothing to require |
| RHUD-487 | Multiplayer sync for selector commands | Not relevant: no multiplayer; every selector already emits a typed command | excluded | No multiplayer |
| RHUD-488 | Pawn Rules integration | Third-party | excluded | Third-party mod |
| RHUD-489 | Credits entry | Menu credits not traced | excluded | The mod's own credits |

### L. Game hooks the mod patches, mapped to our seams

| id | mod feature | today | status | work needed |
|---|---|---|---|---|
| RHUD-490 | `DoTabs`: tab drawing | `paneTabRects`, `ebiten/inspector/hud_inspect.go:272` | missing | DOS-4 direct change |
| RHUD-491 | `InspectPaneOnGUI`: pane drawing | `DrawPane`, `hud_inspect.go:351`; `colony_compose.go:167` | missing | DOS-4 `PaneBody` |
| RHUD-492 | `PaneSizeFor`: pane size | `Compute`, `ebiten/hudlayout/layout.go:134` | missing | DOS-4 |
| RHUD-493 | `PaneWidthFor`: pane width | Same | missing | DOS-4 |
| RHUD-494 | `GeneUIUtility.DrawGenesInfo`: gene tab sizing | No genes | excluded | No gene system |
| RHUD-495 | `PaneTopY`: pane top | `l.Pane`, `ebiten/hudlayout/layout.go:236` | missing | DOS-4 |
| RHUD-496 | `DoPlaySettingsGlobalControls` | `ebiten/hud/global.go:107` | missing | DOS-4 |
| RHUD-497 | `Tutor.TutorOnGUI` | `ebiten/learn/draw.go` | done | Lessons draw there |
| RHUD-498 | `ActiveTip.DrawInner` | `paintTooltip`, `ebiten/uikit/tooltip.go:79` | missing | DOS-3 draws its own |
| RHUD-499 | `ActiveTip.TipRect` | `TipLayout`, `tooltip.go:33` | partial | DOS-3 |
| RHUD-500 | `Game.FinalizeInit`: rebuild after load | `Settings` live in `App`, not per game | done | No per-game state |
| RHUD-501 | `LetterStack.LettersOnGUI` | `LetterRows`, `ebiten/hud/letters.go:109` | missing | DOS-7 |
| RHUD-502 | `MapInterfaceOnGUI_AfterMainTabs`: floating draw point | `colony_compose.go:167` | missing | DOS-4 one call after `DrawPane` |
| RHUD-503 | `Notify_SwitchedMap` | `ports.SwitchMap`, `ebiten/ports/ports.go:190` | partial | DOS-3 cache keyed on entity id; test a map switch drops it |
| RHUD-504 | `ClearAllMapsAndWorld` | `ports.ReturnToMenu`, `ebiten/ports/ports.go:285` | partial | DOS-3 `Reset` in the root's teardown path |

## Budget constraints

Measured at `e924244d3` against `internal/arch/solid_budget.txt` and `.cursor/SOLID_GO.md` §8. Budget lines only shrink.

| Package or type | Now | Limit | Consequence |
|---|---|---|---|
| `game` lines | 53224 | 53224 (budget) | **No room.** Every added line is paid for in the same commit. |
| `game` files | 176 | 176 (budget) | No new file. Adapters go beside the code they wrap (§10). |
| `game.Game` methods | 1837 | 1837 (budget) | `Dossier` needs one removed. DOS-1 removes `LearnedThisWeek`, `game/skills.go:409`, and moves the numbers onto `Card`. The `SetName` handler must not be a `Game` method either: it goes on a component, or DOS-1 pays with a second removal. |
| `game.Game` fields | 64 | 64 (budget) | No new field; the transient stun map goes on the `skills` struct, `game/skills.go:59`. |
| root `ebiten` files | 60 | 60 | No new file. DOS-4 merges `shield_feed.go` (33 lines) into `draw_combat.go`, then adds `dossier_compat.go`. |
| `ebiten.App` fields | 57 | 57 (budget) | No new field. DOS-4 folds one pair (`lastTick`, `lastDraw`, `ebiten/app.go:229`, into a small struct is the suggestion) to fit `dossier`. |
| `ebiten.App` methods | 244 | 244 (budget) | No new method; the host is its own type in `dossier_compat.go`. |
| `uikit.Kit` | 73 methods, 30 fields | budget | No new method or field. Helpers are package functions taking `*Kit`, as `uikit.Choice` does. Tooltip work stays in `dossier`. |
| `Simulation` | 8 embeds | 8 (`iface`) | No ninth role. `Dossier` goes in `SimTabs` (6 methods today, 7 after). `SimColony` is at 8 and loses `LearnedThisWeek`. |
| `inspector` | 47 files, 10206 lines | 60, 12000 | About 1800 lines of room. Only hooks go here: `pane_body.go`, `pane_header.go`. |
| `hudlayout` | 1 file, 477 lines | 60, 12000 | Room. |
| `hud` | 22 files, 4959 lines | 60, 12000 | Room. |
| `learn`, `options`, `config` | small | 60, 12000 | Room. |
| new `dossier`, `dossier/sheet` | 0 | 60, 12000 | Estimated 5000 and 2500 lines. Entry points go on small types (`Panel`, `Setup`, `Editor`), each 40 methods or fewer, per §10. |
| `command.Kind` | about 153 kinds | `uint8` | Room; the count is a text-level estimate. |

No file in `game/dossierview` counts toward `game`, since sub-packages are measured on their own.

## Risks

| Risk | Why | Handling |
|---|---|---|
| Stun has no state on people | `hitHarm.Hold`, `game/body.go:582`, only pushes `NextMoveAt`, which also holds ordinary walking. Reading it would show every walker as stunned. | DOS-1 notes the until tick in a transient, unsaved map. A load loses a running stun's line only. Must stay out of the state hash; prove with `bin/benchmark -hash`. |
| Pane changes are wide | `pawn_pane_test.go` and the journeys assert today's pane; 9 files read `Pane`. | `Enabled=false` by default; old paths stay intact; a parallel journey set runs with it on. |
| Small screens | At 1113x626 a tall pane meets the bar, and floating cards can cover letters or alerts. | `hudlayout.Compute` clamps; golden layouts at the smallest supported size; DOS-7 only moves letters. |
| Frame cost | The card draws many tooltips and lines for one selection. | Rows built on the refresh gate; `Dossier(id)` by id, no scan; `BenchmarkDossierDraw` with `-benchmem`, as `ebiten/AGENTS.md` asks. |
| Stored layout grows or breaks | The tree is player data in `settings.json`. | `Normalize` caps it at 256 nodes, depth 8; a failing tree falls back with a hint. A bad tree never moves the settings file to `.bad`. |
| Preset files are foreign input | Players share them. | Slug names only, `sheet.Validate`, fuzz test on the decoder. |
| New colour tokens | `widgets_test.go` keeps tokens neutral and distinct from reference hexes; `TestUIPaletteMatchesYAML` ties code to YAML. | Art agent on `opus` per `PARALLEL_WORK.md` §6. |
| Unclassified rows | None left; DOS-1 settled them. | A new one is listed in Counts and settled by the package it names. |

## Proposed split

Multi-area task, so this follows `.cursor/PARALLEL_WORK.md` §0. File ownership is disjoint within a wave. Sizes assume `make agent-budget` allows three or four at once.

### Contract (lead, before any agent)

`config` (DOS-0, new `config/hud.go`, one field and the version in `config/settings.go`):

```go
const SettingsVersion = 8 // 8: the dossier (RHUD)

type Settings struct {
	// ...
	HUD HUD `json:"hud"`
}

type HUD struct {
	Enabled       bool              `json:"enabled"`
	Docked        bool              `json:"docked"`
	RefreshTenths int               `json:"refresh_tenths"` // 1..30
	Pane          HUDPane           `json:"pane"`
	Card          HUDCard           `json:"card"`
	Text          HUDText           `json:"text"`
	Colors        map[string]string `json:"colors,omitempty"` // key -> "#RRGGBBAA", differences only
	Layouts       HUDLayouts        `json:"layouts,omitempty"`
	Preset        string            `json:"preset,omitempty"` // "" once edited
}

type HUDPane struct {
	Restyle     bool `json:"restyle"`
	Height      int  `json:"height"`   // 188..800
	MinTabs     int  `json:"min_tabs"` // 6..12
	TabW        int  `json:"tab_w"`    // 72..150
	FactionIcon bool `json:"faction_icon"`
	LogBody     bool `json:"log_body"` // floating only
}

type HUDCard struct {
	Anchor          int  `json:"anchor"` // 0..8, row by row from the top left
	OffsetX         int  `json:"offset_x"`
	OffsetY         int  `json:"offset_y"`
	W               int  `json:"w"` // 100..1000
	H               int  `json:"h"` // 100..1000
	CompressLetters bool `json:"compress_letters"`
	LetterGap       int  `json:"letter_gap"` // 0..12
}

type HUDText struct {
	BodySize     int  `json:"body_size"`      // 7..20
	BodyLinePct  int  `json:"body_line_pct"`  // 100..250
	LargeDelta   int  `json:"large_delta"`    // 0..24
	LargeLinePct int  `json:"large_line_pct"` // 100..250
	SmallDelta   int  `json:"small_delta"`    // -5..0
	SmallLinePct int  `json:"small_line_pct"` // 100..250
	LabelW       int  `json:"label_w"`        // 30..150
	ValueW       int  `json:"value_w"`        // 30..150
	Decimals     bool `json:"decimals"`
}

type HUDLayouts struct {
	Docked   *HUDNode `json:"docked,omitempty"`   // nil: the built-in default
	Floating *HUDNode `json:"floating,omitempty"` // nil: the built-in default
}

type HUDNode struct {
	Kind     string    `json:"k"`             // column strip block line, or a widget key
	Kids     []HUDNode `json:"c,omitempty"`
	Fill     bool      `json:"fill,omitempty"`
	Who      string    `json:"who,omitempty"` // letters c s p w; "" all
	Def      string    `json:"def,omitempty"`
	BarStyle string    `json:"bar,omitempty"` // fade_up low_only fade_down main_only
}
```

`Normalize` clamps every range, drops malformed colours, caps the tree at 256 nodes and depth 8, and forces `Docked=false` when `!Pane.Restyle` and `LogBody=false` when `Docked`.

Read model (DOS-0 inert, DOS-1 fills, new `game/dossierview/view.go`):

```go
package dossierview

type Relation uint8 // RelationOwn, RelationAlly, RelationNone, RelationFoe, RelationHeld, RelationWild

type BreakLevels struct{ Minor, Major, Extreme int } // mood 0..100, trait-shifted

type Card struct {
	AgeDayOfYear int                       // AgeDays % DaysPerYear; 0 is a birthday
	Relation     Relation
	Faction      string
	FactionEmblem string                   // ui_icon key
	Role         string                    // a trader's type, "" otherwise
	BreakAt      BreakLevels
	BreakTier    mentalbreak.Tier          // of the running break
	StunTicks    int
	StaggerTicks int
	ActiveSkill  int                       // skill.ID, -1 none
	Saturated    [skill.NumSkills]bool
	LearnedWeek  [skill.NumSkills]int      // replaces Game.LearnedThisWeek
	Stats        stat.Sheet
	EatSpeedPct  int
	WorkType     int                       // work.Type, -1 none
	AttackTarget string
	ExtraLines   [skill.NumSkills][]StatLine // sim-side formulas for the skill tooltip; DOS-1 drops a line it cannot back
}

// StatLine is one tooltip fact, such as melee hit chance or butchery yield.
// Formulas that are pure domain functions stay in sheet and do not use it.
type StatLine struct {
	Label string
	Pct   int
}
```

Query and command:

```go
func (g *Game) Dossier(id entity.EntityID) (dossierview.Card, bool) // one id, no scan; animals too
// SimTabs gains it (ebiten/sim_ports.go).

command.SetName // appended: Pawn, Name ("" restores the generated name). Colonists and tame animals.
```

Layout (`ebiten/hudlayout/layout.go`):

```go
// added to Inputs; zero keeps today's behaviour
PaneW, PaneH float32
Card         gfx.Rect // the floating card, empty when docked
LetterAvoid  gfx.Rect
LetterGap    float32  // 0: HudGap
// added to Layout
Card gfx.Rect
// helpers
func CardRect(sw, sh, anchor, offX, offY, w, h int) gfx.Rect
func CardOffsetBounds(sw, sh, anchor, w, h int) (minX, maxX, minY, maxY int)
```

Hooks (consumer-owned):

```go
// ebiten/inspector/deps.go
type PaneBody interface {
	Claims(k selection.Kind, id entity.EntityID) bool
	Declare(r gfx.Rect)
	Draw(dst *ebiten.Image, r gfx.Rect)
	Title(k selection.Kind, id entity.EntityID) (tint color.RGBA, tip, tab string)
}
// ebiten/dossier/deps.go
// A host in dossier_compat.go fills Reader: Pawn and Animal look the id up in
// Snapshot() and World(), as inspector.pawnView does (inspector.go:151), the rest
// forward to Simulation. Selection-scoped and at the refresh rate, so no tick scan.
type Reader interface { // 8
	Pawn(id entity.EntityID) (game.PawnView, bool)
	Animal(id entity.EntityID) (game.AnimalView, bool)
	Card(id entity.EntityID) (dossierview.Card, bool)
	Bio(id entity.EntityID) (game.BioView, bool)
	Health(id entity.EntityID) (game.HealthView, bool)
	Prisoner(id entity.EntityID) (game.PrisonerView, bool)
	Factions() []game.FactionView
	Areas() []game.AreaView
}
type Actions interface { // 6
	Enqueue(command.Command)
	OpenTab(name string)
	ShowWindow(tab string)
	AreaPicker(title string, cur int, base bool, pick func(ref int))
	ManageAreas()
	Hint(string)
}
type Prefs interface { // 2
	Settings() config.Settings
	SetSettings(config.Settings)
}
type PresetStore interface { // 4; filled by cardpresetfile
	List() ([]string, error)
	Load(name string) ([]byte, error)
	Save(name string, data []byte) error
	Delete(name string) error
}
type FolderOpener func(path string) error
```

`sheet` (pure, imports `config`, `game` views, domain tables; no `ebiten`):

```go
type Who uint8
type Tone uint8 // ToneText ToneOff ToneCritical ToneWarning ToneNeutral ToneGood ToneTop
type Line struct { Text string; Tone Tone; Tip string; Tab string }
type Preset struct {
	Version int
	Name string
	Docked, Floating *config.HUDNode
	DockW, DockH, DockTabs, FloatW, FloatH int
	Text *config.HUDText
}
type SaveFlags struct{ Docked, Floating, Width, Height, Tabs, Text bool }

func WhoOf(p game.PawnView) Who
func Prune(n config.HUDNode, w Who) config.HUDNode
func Measure(n config.HUDNode, m Metrics, avail float32) float32
func Validate(n config.HUDNode) []Problem
func StandingLine(...) Line
func IdentityLine(...) Line
func BodyLine(p game.PawnView, h game.HealthView) Line
func MindLine(p game.PawnView, c dossierview.Card) Line
func SkillCell(s game.SkillView, c dossierview.Card, decimals bool) Cell
func TrainCell(t game.AnimalTrainingView) Cell
```

Widget keys: `gap rule title diet_pick clothes_pick area_pick hour_pick standing identity vigour_bar body_state mood_bar mind_state hunger_bar rest_bar fun_bar need_bar skill train stat tally keeper doing next_up in_hand hauling traits notes captive`. Stating the keys here keeps DOS-2 and DOS-3 apart.

### Wave 0

| id | model and reason | job | owned files | forbidden | tests |
|---|---|---|---|---|---|
| DOS-0 | lead | The contract above, inert and hash-identical; `ebiten/AGENTS.md` package map; `ids.DossierGear` | `config/hud.go`, `config/settings.go`, `config/settings_test.go`, `game/dossierview/`, `domain/sim/command/command.go`, `ebiten/sim_ports.go`, `ebiten/ids/ids.go`, `ebiten/hudlayout/layout.go` (fields only), `ebiten/inspector/deps.go`, `ebiten/dossier/deps.go`, `ebiten/dossier/sheet/types.go` | everything else | `go test -race ./internal/platform/config/ ./internal/arch/` |
| DOS-0A | `opus`, art always runs on the best model | New `ui.dossier_*` tokens, gear and toggle icons, per skill `rimworld-go-art-assets` | `assets/palettes/ui.yaml`, `ebiten/uikit/widgets.go`, `assets/source/` for the icons, `assets/generated/ui/icons/dossier_*` | Go outside `uikit/widgets.go` | `make art-check`, `go test -race ./internal/adapters/inbound/ebiten/uikit/` |

### Wave 1

| id | model and reason | job | owned files | forbidden | tests |
|---|---|---|---|---|---|
| DOS-1 | `opus`, sim internals, budget payer, hash proof | Fill `Card`; `SetName` handler; remove `LearnedThisWeek` and repoint its readers; settle every `UNCLASSIFIED` row in this file; pay all lines and the method | `game/dossierview/`, `game/skills.go`, `game/skills_test.go`, `game/bio_view.go`, `game/body.go`, `game/game.go` (only `SetName` dispatch), the sub-package that takes moved lines, `internal/arch/solid_budget.txt` (lower only), `ebiten/sim_ports.go`, `ebiten/app_test.go`, `ebiten/inspector/bio_ui.go`, `ebiten/inspector/deps.go` (the last four only to repoint `LearnedThisWeek`) | every other ebiten file | `go test -race ./internal/application/game/...`, `bin/benchmark -hash` on cached fixtures, `make arch`. New: `TestDossierBreakLevelsShiftWithTraits`, `TestDossierSaturatedAtCap`, `TestDossierActiveSkillExpires`, `TestDossierStunUnsaved`, `TestSetNameRoundTripsASave` |
| DOS-2 | `sonnet`, pure logic against a written contract | Tree ops, targets, catalog, defaults, built-in presets, codec, every line and tooltip builder, tones, colour maths, the `TestEveryRHUDRowIsClassified` parser | `ebiten/dossier/sheet/` except `types.go` | `ebiten` imports, every other directory | `go test -race ./internal/adapters/inbound/ebiten/dossier/sheet/`. New: rung tables for `BodyLine`, `MindLine`, `SkillCell` priority pairs, HSL round trip, `FuzzPresetDecode`, default layouts fit `PaneH` |

### Wave 2

| id | model and reason | job | owned files | forbidden | tests |
|---|---|---|---|---|---|
| DOS-3 | `sonnet`, UI against a contract | `dossier.Panel`: draw and declare the tree, bars with ticks and styles, selectors, click-throughs, own tooltip card, gear, refresh gate, stat sheet modal | `ebiten/dossier/` except `deps.go` and `setup_*.go` | `sheet`, `inspector`, root | `go test -race ./internal/adapters/inbound/ebiten/dossier/`, with a `fakeShell`; `BenchmarkDossierDraw -benchmem` |
| DOS-4 | `opus`, the composition root, the pane, field and file payments | Pane body hook, header row and its buttons, tab strip, sizes, floating rect, toggle, rename dialog, log body, `shield_feed.go` merge, `App` field fold | `ebiten/inspector/hud_inspect.go`, `pane_body.go`, `pane_header.go`, `health_ui.go`, `log_ui.go`, `ebiten/hudlayout/layout.go`, `ebiten/dossier_compat.go`, `ebiten/colony_compose.go`, `ebiten/app.go`, `ebiten/draw_combat.go`, `ebiten/shield_feed.go`, `ebiten/hud/global.go`, `ebiten/journey_dossier_test.go` | `ebiten/dossier/`, `game/` | `go test -race ./internal/adapters/inbound/ebiten/hudlayout/ ./internal/adapters/inbound/ebiten/inspector/`, `make arch`, `make test-journeys`. New: `TestJourneyDossierDocked`, `TestJourneyDossierFloating`, `TestJourneyDossierHeaderCare`, `TestJourneyDossierHourPick`, `TestJourneyDossierRename`, `TestJourneyDossierToggle`; all nine anchors; stale id test |

### Wave 3

| id | model and reason | job | owned files | forbidden | tests |
|---|---|---|---|---|---|
| DOS-5 | `sonnet`, forms against a contract | Setup modal; Look tab with every setting and its disabled reasons; Colours tab and the editor | `ebiten/dossier/setup_window.go`, `setup_look.go`, `setup_colours.go`, `ebiten/journey_dossier_setup_test.go` | `setup_sections.go`, `setup_presets.go`, `sheet` | `go test -race ./internal/adapters/inbound/ebiten/dossier/`; `TestJourneyDossierColourEdit`, `TestJourneyDossierBackToDefaults` |
| DOS-6 | `sonnet`, editor and a file adapter | Sections tab, tree editor, presets UI, preset store, folder opener | `ebiten/dossier/setup_sections.go`, `setup_presets.go`, `internal/adapters/outbound/cardpresetfile/`, `internal/platform/openfolder/`, `cmd/game` (wiring only), `ebiten/journey_dossier_edit_test.go` | `setup_window.go`, `sheet` | `go test -race ./internal/adapters/outbound/cardpresetfile/ ./internal/adapters/inbound/ebiten/dossier/`; `TestJourneyDossierEditAddBar`, `TestJourneyDossierPresetSaveLoad`; `make arch` for third-party and import rules |
| DOS-7 | `sonnet`, one small area | Letter avoidance and gap, clickable older row | `ebiten/hud/letters.go`, a new `ebiten/hud/letters_test.go`, the letters block of `ebiten/hudlayout/layout.go` (DOS-4 is done by then), `ebiten/journey_letters_test.go` | `ebiten/dossier/` | `go test -race ./internal/adapters/inbound/ebiten/hud/ ./internal/adapters/inbound/ebiten/hudlayout/`; `TestJourneyLetterOlderOpens` |
| DOS-8 | `haiku`, fully specified text and hooks | Five lessons, original text; `TestLessonsCount` if any | `ebiten/learn/lessons.go`, `lessons_test.go`, `ebiten/learn_compat.go` | everything else | `go test -race ./internal/adapters/inbound/ebiten/learn/` |

### Wave 4

| id | model and reason | job | owned files | forbidden | tests |
|---|---|---|---|---|---|
| DOS-9 | `sonnet`, verification and docs | Timing overlay; flip `Enabled`; update `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` (lines 289, 301, 596, 606, 743, 178), `MAIN_UI_SCREENS_SPECIFICATION.md` §10 and §42, `AGENTS.md` package map; handoff; a capture for visual acceptance | `ebiten/devtools/panels.go`, `config/settings.go`, `.cursor/` docs, `ebiten/capture_colony.go` | all else | `make test`, `make arch`, `make test-journeys`, inside CI's 10-minute `-race` budget |

### Dependencies

DOS-0 and DOS-0A come first. DOS-1 and DOS-2 need only the contract and run together. DOS-3 and DOS-4 need DOS-2; DOS-4 also needs DOS-1 for journeys that read `Card`, so DOS-1 lands first. DOS-5 and DOS-6 need DOS-3 for the live preview and share only `dossier/deps.go`, which neither edits. DOS-7 needs DOS-4's `layout.go` finished. DOS-8 needs DOS-3's gear id, published in DOS-0 as `ids.DossierGear`. DOS-9 is last.

### Lead gates

After each wave: `make arch`, the wave's tests, and a commit. After Wave 1 add `bin/benchmark -hash` and lower budgets. `TestGrownGolden` should not move, since `SetName` and the read model change no behaviour; if it moves, stop and find out why. Assets: only DOS-0A touches art. No creature work.

### Rows per package

| Package | Rows (RHUD-) |
|---|---|
| DOS-0 | 025, 057, 059–060 |
| DOS-0A | 036, 039, 399, 403, 406–409, 411, 414 |
| DOS-1 | 014, 025, 109, 157, 181, 228, 235–236, 245, 249–251, 255, 296, 298 |
| DOS-2 | 009, 061–073, 075–076, 104, 106, 112–113, 118, 120–122, 128–129, 140–155, 162–169, 180, 185, 190–192, 197, 211, 216–217, 219, 221, 225–227, 231, 233–234, 237, 239, 242, 244, 246–248, 252–254, 257–258, 281–282, 284, 286, 288, 291–292, 295, 299, 301–302, 308, 314, 321–322, 326, 335–342, 345–347, 351, 354–357, 379, 433, 435–438, 442–444, 449–450, 452–458, 460, 466–468 |
| DOS-3 | 012, 034, 036–038, 040–041, 074, 077–080, 082–085, 089, 093–094, 097–098, 101, 125–127, 134–139, 156, 159, 161, 170–171, 173–174, 176–178, 182–183, 186, 193, 198–205, 207–209, 212–215, 224, 259–260, 262, 267–268, 285, 290, 293–294, 297, 300, 304–307, 311–312, 316–320, 323–325, 327–329, 381–382, 432, 451, 461–463, 466, 469–470, 498–499, 503–504 |
| DOS-4 | 001, 003–011, 014–015, 018–033, 035, 039, 490–493, 495–496, 502 |
| DOS-5 | 001–003, 028–030, 032–033, 054–056, 383–415, 417–431, 476 |
| DOS-6 | 058, 330–335, 338, 344, 348–353, 358–369, 371–378, 380 |
| DOS-7 | 471–472, 474–475, 478, 501 |
| DOS-8 | 043–047 |
| DOS-9 | 042 |

Excluded and done rows need no package; a few done rows still name the package that reuses them. Settings rows name the package that draws their control; the field itself comes from DOS-0.

## Row test

`TestEveryRHUDRowIsClassified` (DOS-2, in `ebiten/dossier/sheet`) reads this file and fails if an id is out of order or doubled, a status is not one of the four, a row has other than five columns, or a row that is not done or excluded names no package. `TestEveryGapRowHasACard`, `ebiten/architect/catalog_test.go:246`, is the model. It also checks that every key in the contract's widget key list is in `sheet.Catalog`.
