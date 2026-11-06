# DOS handoffs

Families: DOS. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## DOS-0A-art
Colonist card colours and icons · 2026-11-03 · f7631b72a
- Changed: 13 new `ui.dossier_*` tokens in `assets/palettes/ui.yaml`, mirrored in `uikit/widgets.go` (off, excellent, bar_tick, skill_*, side_*, tree_*). `uikit.DossierColor(key) (color.RGBA, bool)` and `DossierColorKeys()` (31 keys, editor order, `tree_*` last and not player-editable); no `Kit` method or field. New Blender family `dossier.py`: `dossier_gear`, `dossier_toggle` badge icons. RHUD-036, 039, 399, 403, 406-409, 411, 414.
- Decisions: `text_off` uses `ui.dossier_off` (#93918A), not `TextDisabled` (RHUD-394): `TextDisabled` is 1.84:1 on the panel over a white map. RHUD-417/418: `tree_widget` = `Text`, `tree_core` = `TextDim`, `tree_bad` = `Danger`. Skill, side and tree inks are at least 4.5:1 over white. The bar tick is only 1.71:1 on `bar_low`; DOS-3 may add a dark 1 px side.
- Open: add `"dossier"` to `FAMILIES` in `assets/source/blender/build.py` (lead-only) or `make art` won't rebuild the icons; `uikit/AGENTS.md` omits the theme table and `dossier_colors*.go`. In-game contact-sheet check falls to DOS-3/DOS-4.
- No visual or save/sim impact until DOS-3..5 draw them.

## DOS-1-card
The pawn card's data · 2026-11-03 · f7631b72a
- Changed: `Game.Dossier(id)` fills `dossierview.Card` for colonists and animals, no scan, no writes. `command.SetName` renames colonists and tame animals. `Game.LearnedThisWeek` is gone; read `Card.LearnedWeek`. Nothing added to `Card`.
- Changed: every `ExtraLines` row has a real formula (228 training chance, 235 cook poison, 236 butchery, 245 surgery, 249/250 melee hit/dodge, 251 mining, 255 harvest), whole percents. `Saturated` is true one point below the cap. `hitHarm.Hold` (`game/body.go`) is the one writer of the stun record (`skillState.stun`, unsaved).
- Decisions: a colonist's "" name restores `bioName(personBio)`. A pet's generated name came from `g.rng`, so its first rename keeps the old one in `PetNames` (new additive field, nil until a rename; `SaveVersion` stays 13). A rename moves the state hash by design; restoring brings it back (`TestSetNameRoundTripsASave`). `dossierview.CleanName` mirrors private `roster.checkName`; the rename dialog should call it.
- Decisions (RHUD): 119 excluded (no pack flag); 182 partial (sleepers skip `decayNeeds`); 228, 244 partial; 298 via `WorkTypeOf`.
- Limits: `StunTicks` does not cover EMP-stunned mechs (`raid.Mechs` exposes a bool). One `Game` method freed; lead lowers `solid_budget.txt`.
- Open: `inspector/AGENTS.md:12` still names `LearnedThisWeek`; `journey_mods_owed_test.go` still owes SetName.

## DOS-2-sheet
`sheet`, the card's pure logic · 2026-11-03 · f7631b72a
- Changed: package `ebiten/dossier/sheet` (no ebiten import): targets (`WhoOf`, letter `p` = the player's creature, prisoners/visitors/allies/raiders are `s`), tree ops (`Prune`, `Measure`, `Heights`, `Validate`, `Move`, `Add`, `Remove`), `Catalog()` (`clothes_pick` Reserved), defaults and presets (`DefaultDocked/Floating`, `BuiltIns`, `Encode/DecodePreset` with `ErrPresetTooBig/Newer/Invalid`, `Capture`, `Apply`), line builders, cells, tip builders (`[]TipLine`), colour helpers, `WarnsInWords` (spec §59 guard: tone and words agree).
- Decisions: `Prune` keeps an off-target widget inside a `line` as a `gap`; elsewhere drops it. `TestEveryRHUDRowIsClassified` parses the gap doc.
- Limits: 94.6 % coverage; `FuzzPresetDecode` 20 s clean.
- Open/best guess: RHUD-119 (DOS-1 settled: none); 228, 235, 236, 245, 251, 255 only via `Card.ExtraLines`; 244, 253 share the skill speed line; 286 `FormatStat` lives in `sheet`, not `stat.ID.Format`; 441/457 thought stacks merged by label in `MindTip`.
- Deviations: `Catalog` is a function; tree edit functions take `[]int` paths; `StandingLine`/`IdentityLine` take `BioView`; `Line.Tip` is plain text.

## DOS-3-panel
`dossier.Panel` · 2026-11-04 · b4830ea8d
- Changed: `New(Reader, Actions, Prefs, ports.Sizer, *uikit.Kit, *art.Catalog, Scene) *Panel`. `Scene` (3 methods: `AreaOf`, `FoodPolicies`, `Hour`) is the one addition beyond `deps.go`. PaneBody `Claims/Declare/Draw/Title`; floating `DeclareFloating/DrawFloating`; `Select`, `Reset` (root must call on map switch/return to menu, RHUD-503/504), `Invalidate`, `Height`, `OpenStats`, `SetOnSetup`. Order per Update: `kit.Begin`, `Declare*`, then `Draw*`; the gear press is taken before any card widget.
- Decisions: refresh gate `HUD.RefreshTenths * 6` Updates, immediately on new id/`Invalidate`/settings change; tips build after 20 Updates at rest, up to 280 px; selectors use `uikit.Picker` with a "Manage..." entry (so `Actions.AreaPicker` is unused); unknown tree key draws a marker box and `Hint`s once. Hour backs blend `picker_back` 35 % toward tone/side tokens (no schedule tokens exist). `notes` is built in `build.go`, not `sheet`.
- Open: faction emblem and the older-row click (DOS-4, DOS-7).
- Perf: `BenchmarkDossierDeclare` 0 allocs ~220 ns; Refresh ~50 us/495 allocs (at refresh only); Draw ~100 us (allocs inside Ebitengine). Coverage 73.7 %.

## DOS-4-pane
The card in the game · 2026-11-04 · b4830ea8d
- Changed: the card runs docked in the pane or floating over the map; `HUD.Enabled` false by default (with it off nothing changes, `TestJourneyDossierOffIsIdentical`). Pane is `TabW * max(MinTabs, tabs)` wide, tabs fixed and right-aligned, header buttons `pane.head.*` (name, emblem tip, next, info, rename, threat, care, self-tend). `hud.dossier` toggle listed only while `HUD.Enabled`. Captures `dossier | dossier-float | dossier-log | dossier-animal`.
- Decisions: `CardHost` asserted on the Shell (like `bedPrisonReader`), `deps.go` untouched. RHUD-011 `selectNext` in `pane_header.go`. RHUD-502 floating card draws after `lrn.Draw` so grid/right column don't paint over it. RHUD-003 `Pane.Height` applies only while the card is the body (plain pane keeps 188). Fix: `build.go`/`data.go` set `.A` on opaque premultiplied `color.RGBA` (bars drew near white); now `uikit.Fade`.
- Limits: `shield_feed.go` merged into `draw_combat.go`; `lastTick, lastDraw` folded into `App.stamp`; `App` stays 57 fields / 244 methods; `solid_budget.txt` untouched.
- Open: threat button uses "!" text (no `order_attack` art); hover can show under a floating card; `inspector/AGENTS.md` should learn `CardHost` and `pane.head.*`; `cmd/game/main.go:53` help string lacks the dossier capture states; `TestPawnTabBodiesDrawInsideExpandedPanel/gear/x1.5` fails (Unload row, `inspector/gear_rows.go`, not this work).

## DOS-5-setup
The card's setup dialog · 2026-11-04 · c1b25230f
- Changed: `dossier.Setup` is a `uikit.Modal` (`dossier.setup`; Escape/X close via `RunModals`). Look tab (card, pane, floating, text groups; typed entry plus slider; greyed rows give a reason line and tooltip). Colours tab (grouped list, `#RRGGBB[AA]`, HSL + opacity, reset one/all, preview of the four bar styles; only differences stored). Options > Accessibility second column: Colonist card on/off, Card sits, Pane height (step 20), Floating card at (nine places); `optionLock` now also stops `ApplyOption` for a greyed row. Captures `dossier-setup | -sections | -presets | -colours`. Rows 001-003, 028-030, 032-033, 054-056, 383-431, 476 finished.
- Decisions: "Back to defaults" keeps `Enabled` (the card must not vanish); "Changed" tag ignores `Enabled`. Controls greyed, not hidden. Offset sliders use `hudlayout.CardOffsetBounds`; changing the anchor clamps the nudge. Typed entries commit on Enter/leaving field; sliders live. Only `uikit` tokens and `DossierColor`; no RimHUD name, text or colour.
- Limits: `App` still 57 fields / 244 methods.
- Open: no select-all on focus in text fields; Options second column clipped at 742 logical px (as existing ones); `cmd/game/main.go:55` help string; checklists not ticked here.

## DOS-6-editor
Sections tab, Presets tab, preset store, folder opener · 2026-11-04 · c1b25230f
- Changed: Sections tab edits the current mode's tree (fold, move, remove, three add menus, fill/target checkboxes with last-target-on disabled, definition/bar-style pickers, "Use the built-in layout"); each edit is applied to a clone, refused if `sheet.Validate` finds more problems (hint says why), stored via `Setup.Edit`; a tree equal to the built-in is stored nil. Presets tab: built-ins plus player files (in use, Use, Delete with confirm, name + six save flags, "Keep as a look", "Show card files"); unreadable files stay listed; cap 100 listed.
- Changed: new `outbound/cardpresetfile` (name 1..64 of `[A-Za-z0-9_-]`, else `ErrName` and touches nothing; `<name>.json` in `<config>/rimworld-go/dossier`; atomic writes; 256 KiB cap = `sheet.MaxPresetBytes`) and `platform/openfolder` (`open`/`explorer`/`xdg-open`, own code, no new module). `AppConfig.CardFiles`; captures get no store (`cardFiles(capturing)`). `setup_stub.go` deleted.
- Rows done: RHUD-058, 330-335, 338, 344, 349-353, 358-369, 371-378, 380. Partial/notes: 348 (key shown, no mod source); 332 collapse is a button; 333 `tree_mine` marks widgets the built-in tree lacks; 349 label read-only; 368/369 manage list is the Presets tab; 375 fires only for a file older than `PresetVersion`.
- Tests: `TestJourneyDossierEditAddBar`, `TestJourneyDossierPresetSaveLoad` (hostile file). Open: no `-capture-ui` case for Sections/Presets yet.

## DOS-7-letters
Letters and the floating card · 2026-11-04 · c1b25230f
- Changed: `hudlayout` `Inputs.LetterGap` (0 = `HudGap`) spaces rows; `LetterAvoid` (set only while the card floats and `CompressLetters` is on) puts the stack in the larger free band when the card overlaps the column; otherwise layout identical. `hud/letters.go`: the "+N older" row is a button (`letter.<id>` of the newest folded letter, `LetterRow.Opens`).
- Decisions: root `TestLettersPlusNOlder` changed to require the button opening letter 3 (spec change RHUD-474).
- Open: alerts above a card-avoiding stack are not moved off the card.

## DOS-8-lessons
Colonist card lessons (RHUD-043..047) · 2026-11-04 · c1b25230f
- Changed: done by the lead. `learn.Lesson.Needs func(config.Settings) bool`; `Frame` fields `CardRead`, `SetupOpened`, `LookRead`, `LayoutsSeen`, `SetupClosed`; lessons `dossier_hover`, `dossier_gear`, `dossier_look`, `dossier_sections`, `dossier_close`. `Sensed.CardHover/SetupOpen/SetupLook/SetupLayout` filled from `dossierHost.pointerOnCard` and `dossier.Setup.Showing`.
- Decisions: `watchCard` needs 45 frames on the card and 150 on the Look tab before a lesson counts as read; a modal already up at helper start is not an opening. Lessons are gated on the card being enabled (`TestCardLessonsNeedTheCard`).

## DOS-9-card-on
The colonist card on by default, and the audit · 2026-11-04 · c1b25230f
- Changed: `config.HUD.Enabled` true (old settings files have no `hud` key, so existing players get the card too); switch in Options > Accessibility. RHUD-042: the developer inspector lists each widget's prepare/declare/draw time (`dossier/timing.go`, `devtools.Host.CardTiming`), measured only while asked. New capture `arch-storage`.
- Decisions: fixes the flip exposed: `uikit.BtnOpts.NoFocus` on pane head buttons (Tab was stolen from the Architect; `TestJourneyDossierLeavesTabToTheArchitect`); `hudlayout.placeExpanded` narrows the expanded panel; `dossier.cardGutter` adds 8 px between side-by-side children; `world.heapBody` scales `LyingPose` offsets with the body. Tests reach the plain pane through `dossierOff`, others set `HUD.Enabled = false`; no assertion weakened.
- Audit: all three gap documents re-read against code. RHUD 429 done / 75 excluded; NST 27 done; CSE 45 done / 10 excluded. Found RHUD-092, 100 missing and 057, 284 partial; all four built afterwards. Read code only; journeys and package tests ran it.
- Open: the map's extreme lower-left corner (~4x5 tiles at the camera limit) stays behind the docked card; float it or turn it off.
- UI: the card, its setup modal and Options rows; spec §10 and §42 updated.

## DOS-10-compact
The docked card in 30 % of the screen · 2026-11-04 · 12f9913bc
- Changed (user request 2026-11-04: card took half the screen, make it half that height, every function, no scrolling, never over 30 %): `config.HUDPane.Height`, `TabW`, `HUDText.BodySize`, `LabelW`, `ValueW` are measures on a 720-tall screen; defaults 168, 96 (6 tabs), 12 pt, body line 89 % (16 px), 100, 36; limits 120..192, line pct floor 85. `sheet.DockedFor(hud, sh)` fits them to the real screen (text and pane grow with `sh/720` but pane stays within `30 % - tabs`; below 720 text shrinks to 9 pt; only under 550 tall does the pane pass 30 %). Results: 550 -> 9 pt/141 px; 720 -> 12/168 (26.7 %); 1080 -> 18/252; 1440 -> 24/336. Applied in `CardHUD()` and the panel's `hud()`.
- Changed: `sheet.DefaultDocked` is now line, line, rule, strip of columns (eight rows + rule = 132 px of the 134 left). The old tall tree is gone; a test pins its widget list and that nothing is dropped for colonist, prisoner, tame, wild. The floating card keeps `skillBlock`. Gear sits in the upper-right corner (`TestGearCorners`).
- Decisions: `SettingsVersion` 9: files on old defaults take the new ones, custom heights clamp to 192, custom docked trees keep values and get 192. Presets: Standard 168/6x96/12 pt; Roomy 192/7x96/13 pt (over 30 % only under 604 tall); Snug 152/6x80/10 pt; Snug with traits 168. A saved layout taller than the card warns once. `hudlayout.paneSize` card floor 120 so commands keep room.
- Decisions: VSync on by default (user decision, PERF-003): third kernel panic of 2026-11-04 came with the client pacing frames on an external display in a scaled mode; presets set it, the v9 migration turns it on unless the player chose off; frame cap stays 120 for VSync off.
- Tests: `TestDockedForTable`, `TestEachBuiltInFitsEveryScreen`, `TestJourneyDossierCompact` (resizes 1280x720 -> 1920x1080 -> 1024x768: pane + tabs <= 30 %); geometry journeys re-pinned.
- Open: captures are always 1280x720 logical, so 1024x768 and true 1080p are checked by hand only; the gear covers the last 21 px of the identity line under the pointer (tooltip kept). Animal card's two tabs sit right-aligned by design (`inspector.PaneSpec.TabW`).
