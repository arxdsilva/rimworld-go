# NG-1 — Complete new-game flow, big maps at 60 fps, generated colonists, factions, water

> Plan of record for the user's "RimWorld Vanilla — Complete New Game UI & Interaction Specification" request (pasted spec + 12 reference images), adapted to rimworld-go with **original** names, text, art and numbers. All facts below were checked against HEAD `d9646a76`. Paths are relative to the repo root; `ebiten/` = `internal/adapters/inbound/ebiten/`, `game/` = `internal/application/game/`, `domain/` = `internal/domain/`.

---

## 0. Context

### 0.1 Why
The user wants the full vanilla-style setup experience, from **New Game** to the first playable colony map. Every screen should have:
- a real layout, controls, validation, state transitions and consequences in the simulation;
- the shared desktop-style visual language of the reference screenshots: dark panels over a space backdrop, brown/gold buttons, Back bottom-left and forward bottom-right, tooltips, scroll lists.

### 0.2 What exists today (verified)
**Screens and wizard**
- The wizard screens exist as placeholders: `ebiten/nav.go:7-19` (`screenScenario`, `screenStoryteller`, `screenWorldGen`, `screenLanding`, `screenPawns`), `ebiten/screen_newgame.go`, `wizardNext` (`nav.go:142`).
- Their content is hard-coded (`screen_newgame.go:16-50`):
  - 3 scenarios: "Emergency landing", "Frontier camp", "Sole survivor"
  - 3 storytellers: "Measured tide", "Quiet years", "Wild dice"
  - 3 difficulties: Forgiving, Standard, Harsh
  - 3 stub pawns as text
- The chosen scenario, storyteller and difficulty are plain ints on `App` (`app.go:111,112,122`) and **never reach the simulation**.
- Only `NewGame{Seed, Biome}` (`app.go:57`) reaches `SimFactory` (`app.go:53`). `cmd/game/main.go:78` builds `game.NewColony(game.Config{Seed, Width:*width, Height:*height, Biome})`.
- Map size comes only from the `-width`/`-height` flags (48×40).

**Colony start**
- `game.Config{Seed, Width, Height, Biome}` (`game/game.go:52`).
- `NewColony` (`game.go:570`): `New` → `generate` → `seedGrass` → `LoadStartKit` → `startHome` → `spawnWildlife` → `scheduleThreats` → `research={}`.
- `LoadStartKit` (`game.go:629`): Ada, Bo, Cora; Food 20+10; Wood 30; Rifle; 4 berry bushes.

**Threats**
- Constants in `game/raids.go`: `firstRaidDay=5`, gap 5+rand(5), threat die out of 8 (`pickThreat` :127).
- `raidPoints = Wealth/40 + standing×30 + day×2` (`raid_plans.go`), up to 8 raiders.
- Nature incidents are in `domain/incident`.

**Pawns**
- A name only. `SpawnNamedPawn(x,y,name)` (`game.go:590`).
- `domain/pawn/skill` is a stub; `trait` and `backstory` don't exist.
- `PawnView` has no age, sex, skills or traits. Temperament comes from a hash of the name (`idle.go:17`).
- Appearance is presentation-only: `lookFor(id, skins, hairs, cloths)` (`ebiten/art.go:536`) → `pawnLook{Skin, Hair, HairStyle(short/crop/bun), Shirt, Pants, Vest}`.

**World**
- 12 biomes in `domain/world/biome`, each with `Climate{YearMean, Amplitude, PeakDay, DaySwing}`, Ground mix and Wildlife density.
- `generation.GenerateBiome` (thresholds 0.80/0.71, `StartRadius=5`).
- `geology.Generate` picks 2–3 of 5 stones. Ores: steel, silver, gold.
- Terrain types: Soil, RichSoil, Gravel, Stone, Rock, Sand, Marsh, Ice, RockSmooth, StoneSmooth. There is **no water**.
- `temperature.Season(t)` is global (peakDay 15).

**Items**
- Steel, Silver and Gold already exist (mined).
- Stone chunks and blocks per stone type, with a stonecutter table.
- Steel is a building material for walls and doors only (`MetalOK`).

**Saves**
- `SaveVersion=1`, gob+gzip.
- No seed, storyteller or difficulty is saved. The climate is re-derived from the biome on load.
- `filesave` adapter: `Save/List/Load`. One slot per colony plus `-autosave`.

**UI**
- All text is `ebitenutil.DebugPrintAt`: 162 `drawLabel` and 10 `debugPrintAt` call sites, with many 6/7-px glyph and 14/16-px line assumptions.
- No slider, checkbox, text-field widget, scroll container, modal stack, globe or minimap.
- The input system (`internal/platform/input`, `ebiten/input_frame.go`) has contexts `CtxText, CtxModal, CtxTool, CtxPanel, CtxMap, CtxGlobal, CtxDev`. Text focus is claimed through `textActive()` (`input_frame.go:81`) and `handleTextInput()` (:149).
- Stubs: `domain/faction`, `domain/economy/trade`, `application/scenario`, `application/session`, `domain/sim/statehash`, `application/benchmark`, and `cmd/benchmark/main.go` (empty main).
- Benchmarks: `BenchmarkColonyHour` and `BenchmarkColonyRaid` (48×40), wildlife at 120×120, and draw benchmarks.

### 0.3 User decisions (binding)
| # | Topic | Decision |
|---|---|---|
| D1 | Delivery | Everything in one plan, including Scenario Editor and Custom Difficulty |
| D2 | World map | **Rotating 3D hex globe** |
| D3 | Pawns | Skills, passions, traits and backstories **affect gameplay** |
| D4 | Font | **Real font everywhere** (menus, HUD, inspectors) |
| D5 | Storyteller art | Original **Blender busts** |
| D6 | Map sizes | 200–300 standard **plus up to 500×500**, **60 fps at every size** (3× speed, grown colony) |
| D7 | Factions | **Real**: types, names, leaders, goodwill, relations, settlements on the globe; raids come from a named hostile faction; Factions dialog |
| D8 | Save mode | **Commitment mode enforced** |
| D9 | Bodies | **Gender + body types** in art: 2 sexes × thin/average/heavy = 6 builds, **atlas-packed** |
| D10 | Scenarios | **Four archetypes** + custom |
| D11 | Arrival | **Drop pods + standing** (scenario chooses) |
| D12 | Water | **Water terrain** from landing-site features (rivers, coast, lakes, marsh/mud) |
| D13 | Items | Add **components** + **industrial medicine** (steel/silver/gold exist; silver becomes the currency) |
| D14 | Threats | **Wealth + time**, original numbers, difficulty multiplier, adaptation, optional time-based mode; storytellers differ in **pacing** |
| D15 | Wildlife cap on big maps | Linear up to 120², then √area, **cap 300** |
| D16 | Names | Storytellers **Orla Vane** "the Steady Hand", **Bram Holloway** "the Hearthkeeper", **Kestrel Quay** "the Gambler". Difficulties **Wanderer, Gentle, Settled, Seasoned, Grim, Merciless** + Custom. Scenarios **Broken Hull, Ember Clan, Gilded Expedition, Bare Hands** |

### 0.4 Binding rules
- **Originals only.** Never copy RimWorld names, prose, portraits, faction names, scenario text, storyteller likenesses, balance tables or layouts. Sources: AGENTS.md; `.cursor/MASTER_SPEC.md` L10, L1223, §86; ART spec clean-room §1; UI checklist L1434; vanilla checklist L1830, L1868, L239-240.
- **Structure** follows the user's reference flow. **Content** is ours.
- **Architecture** (AGENTS.md):
  - Domain stays headless and deterministic: no global RNG and no wall-clock time in gameplay; all randomness comes from `pkg/randx` with a per-package salt.
  - External mutations enter as typed commands.
  - UI reads read models and never mutates domain state.
  - Consumer-owned narrow interfaces; concrete types only at the composition root (`cmd/game`).
- **Hysteresis** (AGENTS.md): every threshold-driven behaviour gets separate start/stop thresholds or a minimum duration, plus a **no-flap test**.
- **Compatibility:**
  - `Config.Setup == nil` keeps today's behaviour **bit for bit**. That path is used by tests, `-scenario` set pieces and captures.
  - `SpawnNamedPawn` creates a **neutral pawn**: all skills 7 (= exactly 1000‰ speed), no traits, no incapabilities, adult, average build, `look.Legacy(id)`.
  - `TestTemperateGenerationUnchanged`, `testdata/premotion_v1.sav` and every existing timing test stay green.
  - Golden state hashes are pinned before engine work. An intended behaviour change re-pins them, with the reason recorded in the handoff.
- **Checklists and handoffs** (AGENTS.md):
  - Every satisfied item in the UI/UX checklist, the vanilla checklist, the ART spec and the screens spec gets `[x]`/`[-]`/`[!]` in the same change.
  - Every scoped task leaves a handoff in `.cursor/handoffs/`.
  - Player-observable behaviour ships with its UI.
- **Git:** work on `main`; commit per stage area; never push.

### 0.5 Out of scope (recorded so nothing is half-implied)
Trade, caravans, world travel and quests are out of scope; the Factions dialog shows goodwill only. Also excluded:
- disease events (only the difficulty hook ships);
- Workshop / mod scenarios;
- multiplayer;
- a "Learning helper" overlay;
- per-tile world weather;
- bridges over water;
- fishing.

---

## 1. Shared visual language and UI framework

### 1.1 Font (D4)
- **Library:** Ebiten `text/v2`. It ships in `github.com/hajimehoshi/ebiten/v2 v2.9.11`; its dependency `github.com/go-text/typesetting` is added by the lead with `go get` + `go mod tidy`.
- **Font:**
  - Noto Sans (OFL 1.1), Regular and SemiBold, embedded from `ebiten/fontdata/` with `//go:embed`, together with `OFL.txt`.
  - Embedding means no packaging-script change.
  - Credits get a line "Noto Sans — SIL Open Font License 1.1", and `THIRD_PARTY_NOTICES.md` gets the license.
- **API** (`ebiten/font.go`):

  | Function | Purpose |
  |---|---|
  | `type textStyle struct{ Size float64; Bold bool; Color color.Color; Align align }` | Style value |
  | `drawText(dst, s, x, y, st)` | Draws with a top-left origin |
  | `measure(s, st) (w, h)` | Size of a string |
  | `wrapPx(s, st, maxW) []string` | Wraps on word boundaries; hard-breaks words longer than `maxW` |
  | `lineH(st)` | Line height = ascent + descent + 2 px leading |
  | `ellipsize(s, st, maxW)` | Truncates with an ellipsis |

  - Faces are cached per (size, bold) and guarded by the existing text mutex (`debugPrintAt` uses `ebitenutil`; the new mutex wraps `text.Draw`).
- **Type scale:**

  | Style | Size / weight | Used for |
  |---|---|---|
  | `tsSmall` | 11 | Tooltips' second line, table footnotes |
  | `tsBody` | 13 | Default |
  | `tsLabel` | 13 SemiBold | Buttons and headings inside panels |
  | `tsLarge` | 16 SemiBold | Panel titles |
  | `tsTitle` | 22 SemiBold | Screen titles |
  | `tsHuge` | 32 SemiBold | Main-menu title |

  UI scale (spec L1429) multiplies all sizes; the existing Accessibility settings gain `UIScale` 90/100/115/130%.
- **Migration:**
  1. `drawLabel(dst,x,y,text)` and `debugPrintAt` route to `drawText(..., tsBody)` with the same top-left semantics, so every screen keeps working at once.
  2. A sweep replaces every 6/7-px glyph and 14/16-px line assumption with `measure`/`wrapPx`/`lineH`. Sites verified at HEAD: `app.go:794,890`; `architect.go:90,132,422,472,474,683,684`; `biome_ui.go:56,58,60`; `climate_ui.go:121`; `devmode.go:228-285`; `draw_combat.go:88`; `gizmo.go:79`; `hud.go:161,166,171,311,314,449`; `environment.go:1371,1382`; `inspector.go:168,330,415`; `labels.go:30`; `move_mode.go:197`; `orders.go:46`; `saves.go:144`; `room_stats.go:138-145`; `screen_newgame.go:318,324,331`; `search_ui.go:252`; `threat_ui.go:99,124`; `toast.go:48`; `trough_ui.go:102,109`; `animal_inspector.go:294-324`; `item_inspector.go:215`; `plant_inspector.go:210`; `splash.go:22` (`debugCharW`); `log_ui.go:18` (`logCharW`); and the fixed -7/+11 label offsets in `drawButton` and the options rows.
  3. `wrapText(s, cols)` (`ui_text.go:7`) is kept only as a wrapper around `wrapPx`, for tests.
- **Performance budget:** text rendering ≤ 0.6 ms per frame on the colony HUD with all panels open. Measured by `BenchmarkHUDText`, which uses text/v2 glyph caching.

### 1.2 Palette and tokens
- `assets/palettes/ui.yaml` keeps its tokens; the hand copy in `widgets.go:13-26` must still match (`TestUIPaletteMatchesYAML`, already exists).
- **New tokens** (lead adds to `ui.yaml` and `widgets.go` in Phase 0):

  | Token | Hex | Use |
  |---|---|---|
  | `text` | #E8E2D6 | Body text |
  | `text_dim` | #A89F92 | Secondary text |
  | `text_disabled` | #6E665C | Disabled text |
  | `heading` | #F2D9A6 | Headings |
  | `warning` | #E0A040 | Warnings |
  | `danger` | #D2564A | Danger |
  | `good` | #7FB86A | Positive values |
  | `selection` | #8B5A2B | Selected row (same as `button_selected`) |
  | `field` | #1A130E | Text-field background |
  | `field_border` | #6B5340 | Text-field border |
  | `field_focus` | #E8C070 | Text-field border when focused |
  | `slider_track` | #3A2E28 | Slider track |
  | `slider_fill` | #8B5A2B | Slider fill |
  | `slider_knob` | #E8C070 | Slider knob |
  | `scroll_track` | #1A130E | Scrollbar track |
  | `scroll_thumb` | #6B5340 | Scrollbar thumb |
  | `tooltip` | #120D0A, alpha F2 | Tooltip background |
  | `tooltip_border` | #8A6A48 | Tooltip border |
  | `modal_dim` | #000000, alpha B0 | Background behind modals |

- **New `assets/palettes/world.yaml`** for globe colours:

  | Key | Colour |
  |---|---|
  | `ocean_deep` | #0E2A47 |
  | `ocean_shallow` | #1F4E79 |
  | `lake` | #2D6A9A |
  | `river` | #3F86C2 |
  | `road` | #8C7355 |
  | `ice_sheet` | #E6EEF2 |
  | `sea_ice` | #CFE0EA |
  | `tundra` | #9AA58A |
  | `boreal` | #3F5E46 |
  | `cold_bog` | #5B6B4E |
  | `temperate_forest` | #4E7A3A |
  | `temperate_swamp` | #4A6B47 |
  | `tropical_rainforest` | #2F6B2A |
  | `tropical_swamp` | #3C6A3A |
  | `arid_shrubland` | #A58E5A |
  | `desert` | #CDAE72 |
  | `extreme_desert` | #DDBE86 |
  | `mountain_tint` | #6E6A62 (mixed in by hilliness) |
  | `uncovered` | #121820 |
  | `atmosphere_rim` | #4A7FD4 |

  Each biome has one base colour, mixed toward `mountain_tint` by hilliness (0 / 12 / 25 / 40%).

### 1.3 Layout system (`ebiten/ui_layout.go`)
- **Types and helpers:**
  - `type rect struct{X, Y, W, H float32}` with `inset(p)`, `splitH(at)`, `splitV(at)`, `columns(n, gap)`, `rows(h, gap)`, `contains(x, y)`, `center()`.
  - `screenRect(sw, sh)`.
- **Setup screens** use a centred **dialog frame**:
  - width `min(sw-80, 1240)`, height `min(sh-80, 760)`, centred;
  - title bar 44 px (`tsTitle`);
  - footer bar 56 px holding Back (bottom-left, 140×36) and forward (bottom-right, 160×36);
  - content area in between with 16 px padding.
- **Minimum window:** 1280×720. Smaller windows get a dimmed "Window too small" note instead of overlapping widgets (`TestSetupMinWindow`).
- **Backdrop** (behind every setup screen except Landing):
  - the existing diorama at dim 0.55 (`drawMenuDiorama`), **plus** a baked starfield;
  - a distant planet disc rendered from the generated world when one exists, otherwise from a fixed showcase seed;
  - `drawSetupBackdrop(screen, sw, sh)` in `ebiten/setup_backdrop.go`;
  - Reduced Motion freezes the slow drift.

### 1.4 Widget kit (immediate mode)
**State.** `ebiten/ui_kit.go` holds `type uiState struct{ hot, active, focus string; dragFrom vec2; caret, selStart, selEnd int; scroll map[string]float32; tipID string; tipFrames int }`, stored on `App` as `a.ui`. Widget ids are strings with a per-screen prefix (`scen.`, `sced.`, `story.`, `diff.`, `world.`, `land.`, `pawns.`, `fac.`, `modal.`).

**Click path.**
- `applyClick` stays the dispatch point for button-like widgets: every clickable widget registers in a per-frame **hit list**.
- `layoutButtons` is extended to `layoutHits()`, which returns buttons plus widget hit rects, so existing tests using `click()` keep working.
- Drag widgets read `a.in.down(mbLeft)` and the cursor while `a.ui.active == id`.

**Widgets.**

| Widget | File | Behaviour |
|---|---|---|
| Button (existing `button`) | `widgets.go` | Adds the `tip` string, a `style` (primary gold, normal brown, danger) and an icon key. Disabled buttons show the reason in a tooltip, and the footer hint stays. |
| Toggle checkbox | `ui_check.go` | 16×16 box with a label. Click or Space when focused toggles. |
| Radio group | `ui_check.go` | Vertical list. Arrow keys move the selection when focused. |
| Slider | `ui_slider.go` | Track 6 px tall, 14×18 px knob. Press on the knob or track captures (`ui.active`) and drag follows the cursor. Steps: continuous or N discrete stops with labels (e.g. Low/Normal/High). Arrow keys ±1 step, Shift ×5. Tooltip shows the value. While captured, the release is consumed so it can't click elsewhere. |
| Stepper | existing options pattern | −/value/+ |
| Text field | `ui_textfield.go` | Single line and multi-line (scenario description, intro). Click focuses it and puts the caret at the click position. Supports `CtxText` input (printable runes via `raw.chars`), Backspace/Delete (with key repeat, below), Left/Right/Home/End, Shift-select, Ctrl/Cmd+A, Ctrl/Cmd+C/V (clipboard if Ebiten exposes it; otherwise copy/paste is skipped and recorded), Enter commits single-line fields, Esc cancels the edit (restores the value from focus time). Clicking elsewhere commits. Has a max length and validator (digits-only, name charset). Caret blinks on `menuTick` unless Reduced Motion is on. Focus is claimed through `textActive()` and keys go through `handleTextInput()`. While a field is focused, `CtxText` stops global shortcuts (verified: `input.Resolve` returns nothing when `stack[0]==CtxText`), so Esc/Enter are read with `a.in.key`. |
| Key repeat | `input_frame.go` | Add `keyRepeat(name) bool`: first repeat after 24 frames, then every 3 frames. Used by text fields and list navigation. |
| Scroll view | `ui_scroll.go` | Virtualised rows (only visible rows are laid out), mouse wheel over the view (needs **wheel routing**: menus consume `raw.wheelY` when hovered; the colony camera only zooms when no scroll view is hovered), scrollbar 8 px wide with a draggable thumb, PageUp/PageDown, keeps the focused/selected row visible. Offsets live in `ui.scroll[id]`. |
| Tooltip | `ui_tooltip.go` | Shown after the cursor rests 24 frames (0.4 s) on a widget with `tip` or a disabled reason. Up to 360 px wide, wrapped, kept inside the screen, 2-line style (title in `tsLabel`, body in `tsSmall`). |
| Modal | `ui_modal.go` | `a.modals []modal{id, title, w, h, draw, buttons, onClose}`. Drawn over a `modal_dim` scrim. Esc and the X close the top modal. Clicks outside are ignored. Pushes `CtxModal` on the input context stack. |
| Table | `ui_table.go` | Header row, sortable columns (click the header to cycle asc/desc), rows inside a scroll view, row hover/selection, per-cell tooltips. Used by the faction tables, Load list and characters' team skills. |
| Tabs | `ui_tabs.go` | Generalised from Options (`optionTabs`). |
| Progress bar | `ui_progress.go` | Label, fraction, phase text. |
| Sparkline | `ui_spark.go` | Small line chart from `[]int`, used by the storyteller cards. |

**Focus order.** Tab / Shift+Tab cycle through focusable widgets in registration order. The focus ring is a 1 px `field_focus` outline. `a.ui.focus` resets on every screen change.

**Tests** (`ui_kit_test.go`):
- `TestSliderDragAndKeys`
- `TestTextFieldEditing` (insert, backspace repeat, selection, home/end, esc restores, click-away commits)
- `TestScrollViewWheelAndThumb`
- `TestTooltipDelayAndClamp`
- `TestModalEscClosesTopOnly`
- `TestTableSort`
- `TestTabFocusOrder`
- `TestWheelRoutingMenuVsCamera`

### 1.5 Navigation model
- **Screen IDs** (`nav.go`): existing, plus `screenScenarioEditor`. These are dialogs/modals **inside** their parent screen rather than separate screens: `modalCustomDifficulty` (Storyteller), `modalWorldProgress` (World), `modalLandingAdvanced`, `modalFactions`, `modalAddPart`, `modalConfirm`.
- **Order:** `wizardNext` is Scenario → Storyteller → WorldGen → Landing → Pawns, unchanged.
- **Back** pops one screen. The **wizard draft is kept** across Back/Next until the player returns to the Main Menu or starts the game. Returning to the Main Menu from Scenario discards it.
- **Esc order:** text field edit → top modal → Back.
- **Click dispatch:** `applyClick` dispatches by prefix to `a.scenarioClick`, `a.editorClick`, `a.storyClick`, `a.worldClick`, `a.landClick` and `a.pawnsClick`. This replaces the hard-coded `scenario.0-2` / `storyteller.0-2` / `difficulty.0-2` cases (`app.go:592-609`).
- **Enter** (`pressAccept`, `input_frame.go:164`): fires the enabled forward button of the current screen (`wizard.next`, `world.generate`, `wizard.start`). Disabled when a modal or text field is focused.

### 1.6 Wizard draft state (`ebiten/newgame_state.go`)
`newGameState` replaces `wizardState` (`screen_newgame.go:132`) and the `App.scenario/storyteller/difficulty` ints:
```go
type newGameState struct {
    scenarios   []game.ScenarioView      // built-ins + custom files
    scenarioSel int
    scenario    scenario.Def             // working copy (editor edits a copy)
    storyteller string                   // profile key
    preset      difficulty.Preset        // Wanderer..Merciless or Custom
    custom      difficulty.Settings      // edited copy when preset==Custom
    saveMode    difficulty.SaveMode
    world       planet.Params            // seed text, coverage, rain, temp, pop
    factions    []faction.Slot           // configured types (world-gen list)
    builder     *planet.Builder          // in-flight generation
    planet      *planet.Planet           // generated world (kept across Back)
    planetKey   string                   // params hash the planet was built from
    tile        int                      // selected tile, -1 none
    season      clock.Season             // Auto or a quadrum
    mapSize     int                      // 250 default
    pool        *game.CandidatePool      // character candidates
    err         string
}
```
- **Forward stamp:** each screen's Next writes its part into the draft. The draft is turned into a `game.Setup` (§5.1) only when Start is pressed.
- **Invalidation:**
  - Changing the world params after a planet exists marks the planet **stale**. Next becomes Generate again and the landing tile is cleared.
  - Changing the scenario's people count resizes the candidate pool and keeps existing picks where possible.

### 1.7 Captures
New `-capture-ui` states in `capture.go` `menuCaptures` (`:202`), driven by scripted clicks from the main menu, plus help text in `cmd/game/main.go`:

| State | Shows |
|---|---|
| `scenario` | Choose Scenario |
| `scenario-editor` | Scenario Editor |
| `scenario-editor-addpart` | Add-part modal |
| `storyteller` | Choose Storyteller |
| `difficulty-custom` | Custom Difficulty |
| `worldgen` | Create World |
| `worldgen-progress` | Generation progress, frozen at 40% |
| `landing` | Landing site with a tile selected |
| `landing-advanced` | Advanced options |
| `landing-factions` | Factions dialog |
| `pawns` | Create Characters |
| `pawns-reroll` | Create Characters after a reroll |
| `arrival` | Colony 3 seconds after drop pods land |
| `pause-commitment` | Pause menu in commitment mode |
| `factions-colony` | In-colony Factions window |
| `bio` | Pawn Bio tab |
| `work-incapable` | Work tab with incapable cells |
| `minimap` | Minimap on a 500² map |

---

## 2. Screens (spec §1–§8 follow the skill `rimworld-go-ui-screens`; each block = acceptance criteria)

### 2.1 Main Menu (`ebiten/screen_menu.go`)
- **Purpose:** top-level hub. It doesn't simulate anything.
- **Layout:**
  - Diorama background, with the existing original splash at no more than 55% of the screen width.
  - Title in `tsHuge` on the left over the diorama.
  - Version number under the title.
  - Right rail at `x = sw-340`, `y = 140`, buttons 300×40 with a 10 px gap.
- **Buttons:**

  | id | Label | Behaviour | Disabled when (reason) |
  |---|---|---|---|
  | `menu.continue` | Continue | Loads the most recent save by modified time (commitment saves too) | No saves ("no saves yet") |
  | `menu.newgame` | New Colony | Creates a fresh `newGameState` (§1.6) seeded with the defaults (§3.10), then pushes Choose Scenario | — |
  | `menu.load` | Load Game | Pushes Load | — |
  | `menu.options` | Options | Pushes Options | — |
  | `menu.plugins` | Plugins | Unchanged | "not implemented" |
  | `menu.tutorial` | Tutorial / Help | Unchanged | "not implemented" |
  | `menu.credits` | Credits | Pushes Credits (adds the font licence line) | — |
  | `menu.quit` | Quit | Exits | — |

- **Changes from today:**
  - The label "New Game" becomes "New Colony", keeping the id `menu.newgame` for tests.
  - Continue moves to the top when it is enabled.
- **No-overwrite:** New Colony never touches existing saves. A new slot name is created at Start (§2.13).
- **Tests:** keep `TestClickSounds`. Add `TestNewColonyCreatesFreshDraft` and `TestContinueLoadsNewestIncludingCommitment`.

### 2.2 Choose Scenario (`ebiten/screen_scenario.go`)
- **Purpose:** decide who arrives, what they bring and the special rules. Difficulty is a separate choice.
- **Entry:** Main Menu → New Colony. Back from Storyteller.
- **Exit:** Back → Main Menu, discarding the draft (confirm modal "Discard this new colony setup?" if any later step was changed). Next → Storyteller.
- **Layout** (dialog frame §1.3; content split 38% / 62%):

  **Left: scenario list** (scroll view `scen.list`)
  - **Search field** `scen.search` at the top (text field; filters by name and summary).
  - **Group "Built-in"**: the four built-ins (§3.1) in fixed order.
  - **Group "Your scenarios"**: custom files from the scenario store (§5.7), sorted by name. Each row has a delete icon `scen.del.<key>` (danger, opens a confirm modal).
  - **Each row** (72 px tall): 40 px icon (`ui_icon/scenario_<key>`, or `scenario_custom`), name (`tsLabel`), summary (`tsSmall`, 2 lines, ellipsized), and difficulty pips (1–4) from the scenario's `Hardness` value.
  - **Selection:** the selected row uses `selection` fill. Click selects; Up/Down move the selection when the list has focus.
  - **Below the list:**

    | id | Label | Behaviour |
    |---|---|---|
    | `scen.new` | New scenario | Opens the Editor with a blank copy of Broken Hull, renamed "New scenario" |
    | `scen.edit` | Edit a copy | Opens the Editor on a copy of the selection; built-ins are never edited in place |
    | `scen.import` | Load file… | Modal listing `*.json` files in the scenarios dir (scroll table: name, modified) |

  **Right: details** (scroll view `scen.details`, read-only)
  - Title (`tsTitle`).
  - Summary line.
  - Narrative intro (wrapped body text).
  - Key–value block:
    - Player faction type (Settlers / Clan).
    - Starting people (N, plus "chosen from M candidates").
    - Tech level and its research grants (list of project names) plus the research cost percentage.
    - Arrival (Drop pods / Standing / Walk in).
    - Starting equipment and starting supplies (item lines "Rifle ×1"; each line has a tooltip with the item description).
    - Scattered on the map (item lines).
    - Special rules (conditions, scheduled or disabled incidents, disallowed buildings, forced traits or ages).
  - Nothing in the details panel is editable; clicking does nothing.
- **Footer:** Back (`wizard.back`), and Next (`wizard.next`), which is disabled with the reason when the selected scenario fails `Validate()` ("Fix N problems in the editor").
- **Keyboard:** Enter = Next, Esc = Back, Ctrl+F focuses search.
- **Interaction rules:**
  - Selecting a row updates the details immediately; nothing is generated.
  - Next copies the selected `scenario.Def` into the draft. A custom scenario is **embedded** in the setup, so the game never depends on the file later.
- **Empty and error states:**
  - No custom files: the group shows "No saved scenarios yet — use New scenario".
  - A file that fails to decode: shown greyed with "Can't read this file (reason)" in its tooltip, and it can't be selected.
- **Commands and read models:** `game.Scenarios()` returns `[]ScenarioView`: built-ins plus store listings, with `Key, Name, Summary, Builtin, Problems, Lines []DetailLine`.
- **Tests:**
  - `TestScenarioListShowsBuiltinsThenCustom`
  - `TestScenarioSearchFilters`
  - `TestScenarioDetailsFromParts`
  - `TestInvalidScenarioBlocksNext`
  - `TestDeleteCustomConfirms`
  - `TestBackDiscardConfirmOnlyWhenChanged`
- **Capture:** `scenario`.

### 2.3 Scenario Editor (`ebiten/screen_scenario_editor.go`, `screenScenarioEditor`)
- **Purpose:** build a custom scenario without mods. This is not a character editor.
- **Entry:** `scen.new` / `scen.edit` / `scen.import`.
- **Exit:**
  - Done: returns to Choose Scenario with the edited scenario selected. If it was never saved, it stays in the list as "(unsaved)" until the app quits.
  - Back/Esc: asks "Discard changes?" if the scenario is dirty.
- **Layout:**

  **Header row**
  - Name field `sced.name` (max 40 characters).
  - Buttons:

    | id | Label | Behaviour |
    |---|---|---|
    | `sced.save` | Save | Writes `<slug>.json`; asks before overwriting |
    | `sced.load` | Load | Loads from the scenarios directory |
    | `sced.reset` | Reset | Back to the state when the editor was opened |
    | `sced.seed` | Randomize seed | Rerolls the **scenario's own** seed, which only drives scenario-level random parts (e.g. forced-trait chance). This is not the planet seed. |

  **Left column (42%): fields and parts**
  - `sced.summary`: single line, max 90 characters.
  - `sced.desc`: multi-line description, max 1200 characters.
  - **Mandatory part block** (always present, can't be removed):
    - Player faction type: radio `Settlers | Clan`.
    - Starting people: stepper 1–10, plus "choose from" stepper N…N+8 (default N+5).
    - Arrival: radio `Drop pods | Standing | Walk in`.
    - Tech level: radio `Neolithic | Industrial | Spacer`.
  - **Optional parts list** (scroll):
    - Each part is a collapsible card: title, a summary when collapsed, ▲ `sced.up.<i>`, ▼ `sced.down.<i>`, ✕ `sced.del.<i>`, and its controls when expanded.
    - `sced.add` "Add part…" opens `modalAddPart`: a categorised list (Starting people, Items, Conditions, Incidents, Restrictions, Presentation) with a description of each kind. Choosing one appends it, expanded.

  **Right column (58%):** a live preview of the details panel (same renderer as §2.2) and a **Validation** panel underneath.
  - Each problem has an icon, the part's name and a message. Clicking a problem scrolls to and expands its part.
  - Done is disabled while there are problems ("Fix N problems").
- **Part kinds** (all in `domain/scenario`, §4.4):

  | Part | Controls | Limits / validation |
  |---|---|---|
  | Forced trait | Trait dropdown, "who": all / one random / N random, chance % slider (5–100) | Trait must exist; conflicting forced traits are an error |
  | Age range | Min / max steppers (14–80) | min ≤ max |
  | Forced condition | Condition dropdown (old fracture, nearsighted, tremor), chance % | — |
  | No clothes | Toggle | — |
  | Starting item | Item dropdown (any player item), quantity stepper (1–5000; weapons 1–10) | Item must exist |
  | Scattered item | Item, quantity, "near start / anywhere" | — |
  | Starting animal | Species dropdown (tameable domestic and pet roster only), count 1–5 | — |
  | Permanent condition | Dropdown: **Ashfall sky** (−20% light, crops −15%), **Endless chill** (−8 °C all year), **Long summer** (+6 °C, amplitude −30%), **Still air** (no rain or snow) | One of each at most |
  | Scheduled incident | Incident dropdown (raid, predator pack, manhunter pack, hive swarm, herd crossing, stray joins, farm animals), first day stepper 1–120, repeat every N days (0 = once) 1–60 | — |
  | Disabled incident | Incident dropdown | Can't disable and schedule the same one |
  | Disallowed building | Building dropdown | Can't disallow a building the kit needs to survive; warning, not error |
  | Research granted | Project dropdown | — |
  | Research cost | % slider 50–300 | — |
  | Intro text | Multi-line, max 1500 | Shown once after landing (§2.12) |

- **Validation** (`scenario.Def.Validate()` → `[]Problem{Part int, Severity (Error|Warning), Msg}`):
  - **Errors:** empty name; people outside 1–10; "choose from" less than people; unknown item, trait, species or incident key; min age > max age; the same incident both disabled and scheduled; two forced traits that conflict; text over its maximum length.
  - **Warnings:** no food in the kit while arrival isn't Walk in; disallowing walls, beds or campfire.
  - Difficult is allowed: warnings never block.
- **Save and load:**
  - Files are versioned JSON (`{"version":1, "def":{...}}`) in `<UserConfigDir>/rimworld-go/scenarios/`.
  - The file name is a slug of the name; a collision asks before overwriting.
  - Saving a scenario never touches colony saves.
- **Commands and ports:** `scenariostore` port (`List/Load/Save/Delete`), implemented by a new outbound adapter `internal/adapters/outbound/scenariofile`, wired in `cmd/game`.
- **Tests:**
  - `TestEditorAddMoveRemoveParts`
  - `TestEditorValidationLinksToPart`
  - `TestEditorSaveLoadRoundTrip` (with a fake store)
  - `TestEditorDoneBlockedByErrorsNotWarnings`
  - `TestEditorSeedIsScenarioOnly`
- **Captures:** `scenario-editor`, `scenario-editor-addpart`.

### 2.4 Choose Storyteller (`ebiten/screen_storyteller.go`)
- **Purpose:** three independent choices: who paces the story, how hard it is, and whether the player can reload.
- **Entry:** Scenario → Next.
- **Exit:** Back → Scenario. Next → Create World.
- **Layout** (dialog frame, three columns):

  **Left (300 px): storyteller list**
  - Three portrait cards `story.pick.<key>`, each 300×150: bust portrait 120×150 (§7.2), name (`tsLarge`), title (`tsSmall`, `text_dim`).
  - The selected card gets a gold border and a warm fill.

  **Centre: the chosen storyteller**
  - Large portrait (up to 360×450, keeping aspect).
  - Name, title, then a 2–3 paragraph bio (§3.2).
  - A **pacing sparkline**: 60 in-game days simulated by `storyteller.Plan` with a fixed showcase seed, showing threat bars and quiet spans. Caption: "Example pacing — every colony differs."

  **Right (360 px): difficulty and save mode**
  - "Difficulty" heading.
  - Radio list of the 6 presets plus Custom (`diff.<key>`). Each row shows the name and a one-line description (§3.3), and its tooltip shows the key numbers (threat scale, yields, mood, infection).
  - Selecting **Custom** opens `modalCustomDifficulty` (§2.5) the first time. Afterwards, `diff.customize` "Customize…" re-opens it.
  - "Save mode" heading, then a radio:
    - `save.reload` **Reload anytime**: "Save and load as often as you like. Good for learning and experimenting."
    - `save.commit` **Commitment**: "One save per colony that updates itself. You live with every decision. This can't be changed later."
  - A note under it: "Storyteller and difficulty can be changed later in Options. Save mode can't."
- **Footer:** Back, Next.
- **Defaults:**
  - **First run:** Orla Vane, Settled, Reload anytime.
  - **Later runs:** the last choices, remembered in settings as `Gameplay.LastStoryteller/LastDifficulty/LastSaveMode`.
- **Rules:**
  - The three choices are independent; changing difficulty never changes the storyteller.
  - Next is always enabled, because every choice has a default.
- **In-game changes:**
  - A new Options tab "Story" lets the player change the storyteller and difficulty (including Custom) during play with the same widgets. Changes are sent as typed commands `command.SetStoryteller` / `command.SetDifficulty`.
  - Save mode is shown there read-only.
- **Tests:**
  - `TestStorytellerPickIndependentOfDifficulty`
  - `TestCustomOpensEditorOnce`
  - `TestSaveModeDefaultsAndPersistsInDraft`
  - `TestPacingSparklineDeterministic`
  - `TestOptionsStoryTabSendsCommands`
- **Capture:** `storyteller`.

### 2.5 Custom Difficulty (modal `modalCustomDifficulty`, `ebiten/screen_difficulty.go`)
- **Purpose:** edit every parameter instead of a preset.
- **Layout:**
  - A 900×620 modal with a scroll view.
  - Top row: "Start from: [preset dropdown] [Copy]" (`diff.copy`) copies that preset's values into Custom.
  - Grouped sections (§3.3 lists every field and range):
    1. **Threats**: Threat scale slider (0–400%, step 5), Big threats toggle, Peaceful start days stepper (0–30), Predators hunt colonists toggle, Hostile wildlife revenge slider (0–300%).
    2. **Escalation**: Adaptation toggle + Adaptation strength slider (0–200%), Time-based escalation toggle + "Escalation per year" slider (0–200%, disabled and dimmed when the toggle is off, with tooltip "Only used with time-based escalation"), "Ignore wealth" toggle (disabled unless time-based is on).
    3. **Economy**: Crop yield, Mining yield, Butchering yield (each 20–300%), Research speed (25–300%).
    4. **Health and mind**: Mood offset slider (−20 to +20), Infection chance (0–300%), Disease frequency (0–300%, with a note "No diseases yet — takes effect when they arrive"), Pain tolerance offset (−20 to +20%).
    5. **Nature**: Extreme weather toggle (heat waves, cold snaps).
- **Footer:** Cancel (closes and restores the values from when it opened), Reset (back to the "Start from" preset), Accept (stores into `draft.custom`).
- **Validation:** none can fail, because sliders clamp.
- **Tests:**
  - `TestCustomDifficultyCancelRestores`
  - `TestCopyPresetFillsAllFields`
  - `TestDependentFieldsDisabled`
- **Capture:** `difficulty-custom`.

### 2.6 Create World (`ebiten/screen_worldgen.go`)
- **Purpose:** set the parameters for the planet, then generate it.
- **Entry:** Storyteller → Next.
- **Exit:**
  - Back → Storyteller. The generated planet and parameters are kept.
  - **Generate** → progress modal → Landing.
- **Layout** (two columns, 50/50):

  **Left: planet**

  | Field | id | Control | Behaviour |
  |---|---|---|---|
  | Seed | `world.seed` | Text field | Free text, 1–32 characters: letters, digits, space, `-`, `_`. The default is a random two-word phrase from an original word list (e.g. "amber-hollow"). |
  | Randomize seed | `world.dice` | Button | Replaces the text; does not generate. |
  | Globe coverage | `world.coverage` | Three-stop slider | 30% / 50% / 100%. A dev-mode-only fourth stop of 5% exists for tests. Tooltip: "How much of the planet is generated. Bigger worlds take longer. This is not your colony's map size." |
  | Rainfall | `world.rain` | Slider, 5 stops | Very dry / Dry / Normal / Wet / Very wet |
  | Temperature | `world.temp` | Slider, 5 stops | Very cold / Cold / Normal / Warm / Very warm |
  | Population | `world.pop` | Slider, 3 stops | Sparse / Normal / Crowded. Tooltip: "How many other settlements exist. Doesn't change your colonists or raid sizes." |

  **Right: factions** (table `world.factions`)
  - Columns: emblem, type name, count (− / + per row, 0–3), remove ✕.
  - Rows: Settlers, Clans, Drifters, Freebooters. The Swarm row is fixed and hidden with a lock tooltip "Always present".
  - `world.facAdd` "Add faction…" opens a modal listing types not yet at their maximum.
  - `world.facReset` "Reset factions" restores the defaults: Settlers 2, Clans 2, Drifters 1, Freebooters 1.
  - Warning line when no hostile-capable human faction is present: "No faction can raid you — raids will be replaced by other threats."

  **Footer:**
  - Back.
  - `world.resetAll` "Reset all": a confirm modal, then defaults with a new random seed.
  - `world.generate` **Generate**, which is primary. When a matching planet already exists (same params hash), the label becomes **Next** and it skips regeneration.
- **Generate flow:**
  1. Commit the text fields.
  2. Hash the params.
  3. Open `modalWorldProgress` (title "Generating world", phase name, progress bar, Cancel).
  4. Each frame calls `planet.Builder.Step(budget)` with a budget of 8 ms of work per frame measured in **work units** (tile operations), not wall-clock, so it stays deterministic. The UI may adapt the unit count from frame time without changing the result.
  5. Phases: **Shaping the globe**, **Raising land**, **Climate**, **Biomes**, **Rivers**, **Settling factions**, **Roads**.
  6. On done, store `draft.planet`, close the modal and push Landing.
  - Cancel discards the builder.
  - Generate can't be pressed twice while generating, because the modal blocks input.
- **Errors:** generation can't fail on valid params. If the land share comes out too small, the builder re-rolls the sea level up to 3 times deterministically. Otherwise the modal shows "Couldn't make a habitable world with these settings — try another seed" and keeps the screen.
- **Tests:**
  - `TestSeedFieldFreeTextAndDice`
  - `TestGenerateRunsBuilderToDone` (5% coverage)
  - `TestGenerateSameParamsSkips`
  - `TestParamsChangeMarksPlanetStale`
  - `TestFactionTableAddRemoveReset`
  - `TestNoHostileWarning`
  - `TestCancelDiscards`
- **Captures:** `worldgen`, `worldgen-progress`.

### 2.7 Select Landing Site (`ebiten/screen_landing.go` + `ebiten/globe.go`)
- **Purpose:** pick the one world tile that becomes the colony.
- **Entry:** world generated.
- **Exit:**
  - Back → Create World (planet kept).
  - Next → Create Characters. Disabled until a valid tile is selected.
- **Layout:**
  - **Globe:** full screen, over the starfield, no dialog frame.
  - **Top centre:** title "Choose your landing site" (`tsTitle`, with a shadow).
  - **Top right:** a compact **legend toggle** `land.legend` (biome colours) and a **view mode** cycle `land.mode`: Biome / Temperature / Rainfall / Elevation (§4.2 overlays).
  - **Right panel** (380 px, full height minus the footer): **Tile info** (scroll view) for the hovered tile while nothing is selected, otherwise the selected tile. See the table below.
  - **Bottom bar** (centred, 56 px), left to right:

    | id | Label | Behaviour |
    |---|---|---|
    | `wizard.back` | Back | Back to Create World |
    | `land.random` | Select random site | §4.2 rules |
    | `land.advanced` | Advanced… | Modal, below |
    | `land.factions` | Factions… | Modal, §2.8 |
    | `wizard.next` | Next | Disabled with a reason until a valid tile is chosen |

- **Tile info fields:**

  | Field | Source | Notes |
  |---|---|---|
  | Biome | `planet.Biome` | Icon + name + blurb (from `biome.Def`) + difficulty tag |
  | Terrain | Hilliness | Flat / Small hills / Large hills / Mountainous, with a tooltip on what it means locally |
  | Coordinates | Lat/Lon | "34.2°N 121.7°W" |
  | Elevation | m | — |
  | Avg temperature | — | Celsius, or Fahrenheit if the Options unit setting is on |
  | Summer high / winter low | `Climate.Range` | — |
  | Rainfall | mm/year | — |
  | Growing period | `Climate.GrowingDays(30)` of 60 days | e.g. "35 of 60 days", or "Year-round" |
  | Stone types | 2–3 names | — |
  | Water | River size (Creek / River / Large river), Coast, Lake | — |
  | Caves | Yes/No | — |
  | Roads | Yes/No | — |
  | Nearest settlement | Name, faction, distance in tiles | — |
  | Movement difficulty | ×1.0–×3.0 | From hills and biome; informational until world travel exists |
  | Forage | Low / Medium / High | From the biome's plants |
  | Wildlife | Sparse … Abundant | From the biome's Wildlife |

  **Warnings** (in `warning` colour):
  - "Extreme climate" for extreme biomes.
  - "Very short growing season" for less than 15 days.
  - "Large map — needs a capable computer" for sizes ≥ 400.
- **Validity** (Next's disabled reasons):
  - Ocean or lake tile: "Can't land on water."
  - A faction settlement: "Occupied by <settlement>."
  - Within 1 tile of a settlement: "Too close to <settlement>."
  - Outside the coverage: "Not part of this world."
  - Ice Sheet and Sea Ice are allowed, with the extreme warning.
- **Globe controls:**

  | Input | Effect |
  |---|---|
  | Left-click | Select a tile; a click is release within 4 px of the press |
  | Left-drag | Rotate, with inertia (decays 8% per frame) |
  | Middle-drag | Pan the view centre |
  | Mouse wheel / PageUp / PageDown / `+` `-` | Zoom; the distance clamps from 1.25 to 4.0 radii |
  | WASD / arrows | Rotate; Shift = 3× faster |
  | Right-click | Clear the selection |
  | Double-click | Select and centre the tile |
  | Home | Centre on the selected tile |

  Edge-scrolling is off on the globe. Reduced Motion turns inertia off.
- **Advanced modal** (`modalLandingAdvanced`):
  - **Starting season:** radio `Auto | Spring | Summer | Fall | Winter`.
    - Auto picks the first season whose middle day has a mean temperature of 8 °C or more and rising. If none, it uses the warmest season.
    - Seasons are **local**: the southern hemisphere is shifted by 30 days (§4.2).
    - Tooltip: "Season the colony starts in, at this site."
  - **Map size:** radio list, each with a tooltip of tile count and travel time:
    - **Standard:** 200×200 · 225×225 · **250×250 (default)** · 275×275 · 300×300.
    - **Large:** 350×350 · 400×400 · 450×450 · 500×500. Labelled "needs a capable computer". These appear only once the engine gate (§6.9) passes; before that they are disabled with the reason "Not available yet".
  - Footer: Reset, Close.
- **Select random site:**
  - Choose deterministically from (planet seed, a counter that increases per press) among valid tiles. Weight them toward non-extreme biomes with growing period ≥ 20 days and flat-to-large hills.
  - Rotate the globe to centre the tile, select it and show its info. The player can keep picking.
- **Read models:** `planet.Planet` is a pure domain value (no `game` dependency). The UI reads its exported accessors through a small interface `landingWorld` (`TileCount`, `Tile(i) TileView`, `Pick(ray)`, `Neighbours(i)`, `Settlements()`). The concrete `*planet.Planet` is created by the wizard through `game.NewPlanetBuilder(params)`, a thin application facade so the adapter does not import domain internals directly.
- **Tests:**
  - `TestLandingInvalidTilesBlockNext` (water, settlement, adjacent, uncovered)
  - `TestRandomSiteValidAndDeterministic`
  - `TestAdvancedSeasonAuto`
  - `TestMapSizeOptionsGated`
  - `TestTileInfoFields`
  - `TestGlobePickHitsTileUnderCursor` (`globemath`)
  - `TestGlobeDragDoesNotSelect`
- **Captures:** `landing`, `landing-advanced`, `landing-factions`.

### 2.8 Factions dialog (`modal.factions`, `ebiten/screen_factions.go`, reused in the colony)
- **Purpose:** show who lives on the planet. At setup it is read-only.
- **Table columns:**
  - Emblem (24 px, tinted with the faction accent).
  - Name.
  - Type.
  - Leader ("Warden Iska Vell").
  - Goodwill (−100…+100, shown with a bar).
  - Relation: Hostile / Neutral / Friendly, coloured `danger` / `text_dim` / `good`.
  - Settlements (count).
  - Tech (Neolithic / Industrial).
- The Swarm row shows "Hive swarms — always hostile; no settlements".
- **Row behaviour:** clicking a row at setup closes the modal, centres the globe on that faction's capital and highlights all its settlements for 3 seconds.
- **In the colony:** a "Factions" button on the HUD bottom bar opens the same table plus a **goodwill history** sparkline over the last 60 days and the last 5 events affecting goodwill (raid defeated, …).
- **Read model:** `Factions() []FactionView` (§5.4).
- **Tests:** `TestFactionsTableSortAndCentre`, `TestColonyFactionsWindowReadsReadModel`.

### 2.9 Create Characters (`ebiten/screen_pawns.go`)
- **Purpose:** choose and adjust the starting people.
- **Entry:** Landing → Next.
- **Exit:**
  - Back → Landing.
  - **Start** → build `game.Setup` → `SimFactory` → arrival.
- **Data:**
  - `game.CandidatePool` is built when entering (or when the scenario's people count changes).
  - The pool size is the scenario's "choose from" value M (default N+5).
  - The first N candidates are **Selected**; the rest are **Left behind**.
  - All generation is deterministic from (`setup seed`, candidate slot, roll counter) (§4.7).
- **Layout** (dialog frame):

  **Left (240 px): roster** (scroll view `pawns.roster`)
  - Two groups, "Selected (N)" and "Left behind".
  - Each row is 56 px: a 40×48 portrait, the nickname (`tsLabel`) and the adulthood title (`tsSmall`).
  - **Drag a row** between or within groups to reorder or swap. Without dragging, use `pawns.up` / `pawns.down` and `pawns.swap` "Swap with left-behind…".
  - Selected has exactly N members. Moving one out pulls the top left-behind candidate in.

  **Centre: details** of the highlighted candidate
  - **Name row:** first `pawns.first`, nick `pawns.nick`, last `pawns.last`. Each is a text field (max 16 characters; letters, `'`, `-`, space). An ⓘ tooltip reads "Nickname is what everyone calls them."
  - **Under the names:** "Female, age 34 (biological), arrived <n> years ago" (chronological age = biological; no cryosleep in our lore). Then the player faction badge (scenario faction type) and the build (Thin / Average / Heavy).
  - **Portrait:** 160×200 on the right of the name block. Rendered by a **standalone portrait renderer** (`ebiten/portrait.go`, `drawLookPortrait(dst, look, x, y, w, h)`), which uses the art catalog without a `Renderer`. Today's `drawPawnPortraitArt` needs a `*Renderer`, so the layer logic is extracted and shared by both.
  - **Backstories** (`tsLabel` headings): Childhood `<title>` and Adulthood `<title>`. Hovering a title shows a tooltip with the description and its skill bonuses and incapabilities.
  - **Traits:** chips. Hovering a chip shows a tooltip with the description and its effects.
  - **Incapable of:** a list (e.g. "Violence, Cooking") in `danger` colour, or "None" in `text_dim`.
  - **Skills table**, one row per skill (11, §4.6):
    - Skill name.
    - Passion glyph: none, minor (1 flame) or major (2 flames). Our own flame icon `ui_icon/passion_minor|major`.
    - Level 0–20 as a number and a bar.
    - Incapable skills show "—" and are greyed.
    - Tooltip: level description (Novice 0–3, Capable 4–7, Skilled 8–11, Expert 12–15, Master 16–20), work affected, speed %, and the passion effect.
  - **Health:** "No conditions", or a list like "Old leg fracture: moves 8% slower".

  **Right (280 px): Team**
  - "Team skills" grid: each skill with the best level among the Selected, and who has it.
  - **Warnings** (`warning`):

    | Warning | Condition |
    |---|---|
    | "Nobody can fight" | All Selected are incapable of violence |
    | "No doctor" | Best Medicine < 3, or all incapable of caring |
    | "No cook" | Best Cooking < 3, or all incapable of cooking |
    | "No grower" | Best Plants < 3 |
    | "No builder" | Best Construction < 3 |
    | "No researcher" | Best Intellectual < 3, or all incapable of intellectual |
    | "Everyone is very young or old" | — |

- **Buttons:**

  | id | Label | Behaviour |
  |---|---|---|
  | `pawns.reroll` | Randomize | Rerolls the highlighted candidate; keeps the name if it was edited, unless Shift is held |
  | `pawns.rerollAll` | Randomize all | Confirm modal |
  | `pawns.reset` | Undo changes | Restores the pool from entry |

- **Footer:** Back, **Start** (primary). Start is disabled only if a name is empty ("Every colonist needs a nickname").
- **Scenario rules enforced in generation:** forced traits and chances, the age range, forced conditions, no clothes, and the people count.
- **Tests:**
  - `TestRosterSelectedCountMatchesScenario`
  - `TestRerollDeterministicAndKeepsEditedName`
  - `TestReorderAndSwap`
  - `TestNameValidationBlocksStart`
  - `TestTeamWarnings`
  - `TestIncapableSkillsShownDash`
  - `TestForcedTraitAndAgeHonoured`
  - `TestPortraitWithoutRenderer`
- **Captures:** `pawns`, `pawns-reroll`.

### 2.10 Start and generating the colony (transition)
- Start builds `game.Setup` from the draft (§5.1) and calls `SimFactory(NewGame{Setup})`. `NewGame` gains `Setup *game.Setup`; `Seed` and `Biome` remain for legacy captures.
- **Loading screen:**
  - Reuses the boot style (`screen_boot.go`) with phases **Preparing the site**, **Shaping the land**, **Placing rock and ore**, **Seeding plants and animals**, **Arrival**.
  - Map generation runs in steps through `game.NewColonyBuilder(cfg).Step(budget)`, the same pattern as the planet builder, so 500² generation (≤ 2 s) shows progress instead of freezing.
- **On done:**
  - `startSession`, a new slot name `colony-<YYYY-MM-DD-HHMM>`, autosave reset.
  - The camera is centred on the landing spot at zoom 1.
  - The game starts **paused** if Options "Pause on start" (new, default on) is set.
- **Tests:** `TestStartBuildsSetupFromDraft`, `TestColonyBuilderStepsToDone`, `TestStartPausedOption`.

### 2.11 Arrival (in the colony)
- **Drop pods** (Broken Hull, Gilded Expedition, Bare Hands):
  - One pod per colonist plus one per 300 value of kit.
  - Pods land within 6 tiles of the centre on walkable, non-water tiles, staggered over 3 seconds of game time (180 ticks).
  - Each pod shows a fall streak and a dust burst (presentation only, from a `WorldView.Arrivals` stamp list), then a **pod shell** building `construction.PodShell` (1×1, walkable, deconstruct for 6 steel).
  - Colonists step out with the "dazed" mood thought (−4 for 1 day, `mood.Landed`).
  - Items are dropped beside the pod.
- **Standing** (Ember Clan): colonists stand in a ring around the centre, with the kit in a pile next to them.
- **Walk in** (custom only): they enter from the map edge nearest the start along a path.
- **Intro letter:** if the scenario has intro text, a letter window shows it once after arrival ("Continue"). A built-in has a short original intro.
- **Tests:** `TestPodArrivalPlacesEveryoneOnLand`, `TestStandingArrivalRing`, `TestWalkInFromEdge`, `TestIntroLetterOnce`.
- **Capture:** `arrival`.

### 2.12 In-colony additions
- **HUD bottom bar:** add **Factions**.
- **Options:** new tab **Story** (storyteller, difficulty, read-only save mode) (§2.4).
- **Pawn inspector:** new **Bio** tab (§2.9 details, read-only, plus XP progress per skill and "This week: +N" learning). The colonist bar tooltip shows age and adulthood title.
- **Work tab:**
  - Incapable cells are hatched and can't be clicked; the tooltip reads "Incapable: <reason (backstory/trait)>".
  - Each live cell shows the related skill level (small number) and a passion glyph.
  - Header tooltips explain the skill for each work type.
- **Minimap** (maps over 150²): bottom-right, 180×180, toggled with `M`.
  - Draws terrain colours, water, colonists (dots), hostiles (red dots) and the camera rect.
  - Click moves the camera.
  - Read model `MinimapLayer(ver)`.
- **Letters for threats** name the faction, e.g. "Raid! 6 raiders of the Ashen Compact are coming from the east."

### 2.13 Save modes in play
- **Reload anytime** (today's behaviour): manual Save and Load in pause, autosave to `<slot>-autosave`.
- **Commitment** (D8):
  - **Single file:** `colony-<date>.rwsave`, with `SaveInfo.Commitment=true`. No `-autosave` copy.
  - **Saves happen:** on the daily autosave hour (`autosaveHour=6`, using the Options interval with a minimum of daily), on Return to Main Menu, on Quit, and on window close. The close path uses `ebiten.SetWindowClosingHandled(true)` and, when `IsWindowBeingClosed()`, saves before quitting.
  - **Pause menu:** "Save" becomes "Save & Quit to Menu". "Load" is disabled with the reason "Commitment: one timeline".
  - **Dev mode:** disabled in Options with the reason "Commitment mode".
  - **Load screen:** commitment saves have a "Commitment" tag and are loadable. Loading one and later saving overwrites the same file.
  - **Atomic writes:** `filesave` writes `<file>.tmp`, then fsync, then renames, so a crash can't corrupt the only save.
- **Tests:**
  - `TestCommitmentSingleSlotNoAutosaveCopy`
  - `TestCommitmentPauseButtons`
  - `TestCommitmentSavesOnReturnToMenu`
  - `TestCommitmentSavesOnClose` (fake window hook)
  - `TestFilesaveAtomic`
- **Capture:** `pause-commitment`.

---

## 3. Original content (data tables)

All names and text below are ours. Final prose is written by the content agent within these constraints. A name-collision lint in `cmd/contentlint` rejects a blocklist of RimWorld proper nouns (storytellers, scenarios, factions, backstory titles we must not reuse).

### 3.1 Scenarios (built-ins)

| Key | Name | Summary | People (choose from) | Faction | Tech | Arrival | Hardness |
|---|---|---|---|---|---|---|---|
| `broken_hull` | Broken Hull | Three crew members survive a hull breach and ride the lifeboats down. | 3 (8) | Settlers | Industrial | Drop pods | 2 |
| `ember_clan` | Ember Clan | Five clanfolk carry their hearth-fire to new land. | 5 (10) | Clan | Neolithic | Standing | 3 |
| `gilded_expedition` | Gilded Expedition | One wealthy explorer, well armed and well supplied, alone. | 1 (6) | Settlers | Industrial (+Spacer grants) | Drop pods | 3 |
| `bare_hands` | Bare Hands | One survivor, no clothes, no tools, no food. | 1 (6) | Settlers | Industrial | Drop pods | 4 |

**Kits.** Items marked *new* come from §4.8; the rest exist.

- **Broken Hull**
  - Packaged survival rations (`Food`) ×40
  - Meals ×10
  - Industrial medicine ×12 *(new)*
  - Herbal medicine ×6
  - Components ×20 *(new)*
  - Steel ×300
  - Wood ×200
  - Silver ×600
  - Rifle ×2
  - Knife ×1
  - Parka ×1
  - Duster ×1
  - Tuque ×1
  - One random pet from the roster's pet set (cat, labrador, husky, guinea pig)
  - Scattered within 25 tiles: steel ×400 in 4 piles, rations ×8, components ×6
  - Research granted: Food Preservation, Traps
  - Intro: "The alarms are still ringing in your ears…" (original, ≤120 words)

- **Ember Clan**
  - Meat ×40, Berries ×60
  - Herbal medicine ×10
  - Wood ×120
  - Leather ×30
  - Spear ×3, Club ×2, Knife ×2
  - Tuque ×5
  - Two domestic animals (goats or chickens, by biome)
  - Research cost **×150%**; nothing granted
  - Disallowed at start: nothing (rifles locked by research as today)
  - Intro: original

- **Gilded Expedition**
  - Meals ×20, rations ×20
  - Industrial medicine ×20
  - Components ×30
  - Steel ×400
  - Silver ×3000
  - Gold ×30
  - Rifle ×1 and Knife ×1 (marked "fine")
  - Parka ×1, Duster ×1
  - Research granted: Food Preservation, Herbalism, Tailoring, Traps, Gunsmithing
  - Intro: original

- **Bare Hands**
  - No items
  - Part "No clothes"
  - Forced condition "Exhausted" (rest 20%)
  - Research: none
  - Intro: original

**Hardness pips:** 1 = gentle … 4 = brutal (display only).

**Tech levels:** Neolithic research cost 150%, Industrial 100%, Spacer 80% (custom only). Research grants are parts.

### 3.2 Storytellers

| Key | Name | Title | Pacing model | First threat | Gaps | Positive events | Special |
|---|---|---|---|---|---|---|---|
| `steady` | Orla Vane | the Steady Hand | **Cycle**: 8-day "pressure" phase, then a 5-day "rest" phase. In pressure: big threat every 3–5 days. In rest: no big threats, minor events only. | Day 6 | 3–5 (pressure) | 20% | Threat points ramp +4% per completed cycle (capped at +40%) |
| `hearth` | Bram Holloway | the Hearthkeeper | **Calm**: one big threat every 9–15 days; more good events | Day 10 | 9–15 | 45% | After a colonist death, the next big threat is delayed +6 days and runs at 70% points (LossEase) |
| `gambler` | Kestrel Quay | the Gambler | **Dice**: each day at 07:00 rolls a big threat with probability p = 1/8 (rising +1/40 per quiet day, reset on a threat); points ×U(0.6, 1.5) | Day 4 | any (clumps allowed, ≥ 1 day) | 30% | Can roll two threat categories in one day (5%) |
| `legacy` (hidden) | — | — | Today's constants | Day 5 | 5–9 | — | Used for `Setup == nil` and v1 saves |

- **Bios:** 2–3 short paragraphs each, original, describing temperament, not mechanics. Mechanics are in tooltips:
  - Orla is a retired convoy dispatcher who "believes in a rhythm".
  - Bram is an old innkeeper who "lets you finish your soup".
  - Kestrel is a card sharp who "doesn't remember the last hand".
- **Positive events** (`PositiveBias`): the share of nature incidents chosen from the helpful set (`stray_joins`, `farm_animals`, `taken_a_liking`, `herd_crossing`) versus neutral. This uses the existing incident weights scaled by the bias.

### 3.3 Difficulty presets (our numbers)

| Field | Wanderer | Gentle | Settled | Seasoned | Grim | Merciless | Custom range |
|---|---|---|---|---|---|---|---|
| Description | "Build in peace — no raids or packs." | "Rare, small threats; room to learn." | "Fair challenge with time to recover." | "The intended challenge." | "Harsh threats, thin margins." | "Built to break you. Mostly." | — |
| ThreatScale % | 5 | 35 | 65 | 100 | 145 | 200 | 0–400 |
| BigThreats | off | on | on | on | on | on | toggle |
| PeacefulDays (min first threat) | — | 12 | 8 | 6 | 4 | 3 | 0–30 |
| Adaptation | off | on 60% | on 80% | on 100% | on 100% | on 100% | toggle, 0–200% |
| TimeEscalation | off | off | off | off | off | off | toggle, 0–200%/yr |
| IgnoreWealth | off | off | off | off | off | off | toggle (requires time) |
| CropYield % | 130 | 115 | 105 | 100 | 90 | 80 | 20–300 |
| MineYield % | 130 | 115 | 105 | 100 | 90 | 80 | 20–300 |
| ButcherYield % | 120 | 110 | 100 | 100 | 90 | 80 | 20–300 |
| ResearchSpeed % | 130 | 115 | 105 | 100 | 95 | 90 | 25–300 |
| MoodOffset | +12 | +8 | +4 | 0 | −4 | −8 | −20…+20 |
| Infection % | 40 | 60 | 80 | 100 | 115 | 130 | 0–300 |
| DiseaseFreq % | 0 | 50 | 75 | 100 | 120 | 150 | 0–300 (hook) |
| AnimalRevenge % | 50 | 70 | 85 | 100 | 115 | 130 | 0–300 |
| PredatorsHuntColonists | off | off | on | on | on | on | toggle |
| ExtremeWeather | off | on | on | on | on | on | toggle |
| PainTolerance offset % | +10 | +5 | 0 | 0 | 0 | −5 | −20…+20 |

**Threat points (D14, original):**
```
wealthCurve(w) = w/50                      for w ≤ 20 000
               = 400 + (w−20 000)/80       for w ≤ 100 000
               = 1 400 + (w−100 000)/150   above
colonistPts    = 25 × standing colonists
timePts        = 80 × years elapsed (only when IgnoreWealth; replaces wealthCurve)
base           = (IgnoreWealth ? timePts : wealthCurve) + colonistPts
points         = base × ThreatScale/100 × (100 + adapt)/100
                      × (TimeEscalation ? (100 + EscPerYear × years)/100 : 1)
points         = max(points, 35)   // never empty
```
- Raider count and kit come from points using the existing weapon costs: club 60, knife 50, spear 70, rifle 90 (`raid_plans.go`).
- `maxScaledRaiders` rises from 8 to `8 + mapArea/40 000`, capped at 24, so 500² allows 14.

**Adaptation** (`difficulty.Adapter`, D14):
- **State:** `Days int16` and `Value int8`, where Value is a percent from −30 to +40.
- **Each day without a colonist death:** Days++ (max 120).
- **A colonist death:** Days −= 25 (min −40).
- **Target:** `T = clamp(Days/2 − 10, −30, 40)` × strength/100.
- **Value moves toward T** by at most 2 points per day, and only when |T − Value| ≥ 3. This dead band plus rate limit is the hysteresis.
- **Test:** `TestAdaptationNoFlap` alternates death and no-death at the boundary for 60 days and expects ≤ 1 direction change per 6 days.

### 3.4 Factions

| Type | Key | Tech | Relation range at start | Can be friendly | Settlements / 10k land tiles (normal pop) | Raid kit | Name grammar (original syllables) | Leader titles |
|---|---|---|---|---|---|---|---|---|
| Settlers | `settlers` | Industrial | −20…+60 | yes | 1.2 | Rifles + knives | "the {Adj} Compact" / "{Place} Commonwealth" | Warden, Steward |
| Clans | `clans` | Neolithic | −40…+40 | yes | 1.6 | Spears, clubs, knives | "{Noun} Clan" / "People of the {Place}" | Elder, Hearth-mother, Hearth-father |
| Drifters | `drifters` | Industrial | −60…+10 | yes (rarely) | 1.0 | Mixed | "the {Adj} Drift" / "{Place} Wanderers" | Boss, Caller |
| Freebooters | `freebooters` | Industrial | −100 (fixed) | no | 0.8 | Rifles heavy (+20% points) | "the {Noun} Reavers" / "{Adj} Knives" | Captain, Ringleader |
| Swarm | `swarm` | — | −100 (fixed) | no | 0 (hidden) | Hive insects | "The Swarm" | — |
| Player | `player` | per scenario | — | — | — | — | "{Name}'s Colony" (renameable later) | — |

- **Word lists:** ~60 adjectives, ~60 nouns and ~80 generated place names (two-syllable combinations from our own syllable table) in `domain/faction/names.go`. Example names: "the Ashen Compact", "Thornwick Commonwealth", "Kestrel-Eye Clan", "the Salt Drift", "the Rust Reavers".
- **Relations:**
  - Hostile at goodwill ≤ −50; it becomes non-hostile only at ≥ −10 (**hysteresis**).
  - Friendly at ≥ +50; it stops being friendly at ≤ +30.
- **Daily drift:** goodwill moves 1 point per day toward the type's baseline (Settlers +10, Clans 0, Drifters −30). Freebooters and Swarm are fixed.
- **Events:**
  - A raid repelled from faction F: F −5.
  - Killing a raider: no change.
  - Nothing else in this task (no trade or gifts).
- **Settlement placement:**
  - For each faction, count = round(land × density × popFactor / 10 000). popFactor: Sparse 0.5, Normal 1.0, Crowded 1.6.
  - Placement uses Poisson-disc on land tiles with minimum spacing 6 tiles (sparse 8, crowded 4), excluding Ice Sheet, Sea Ice, Extreme Desert and water.
  - Clans prefer forest/swamp/tundra; Settlers prefer temperate/arid; Freebooters prefer hills.
  - Each faction's first settlement is its **capital**.
  - Settlement names come from the place-name generator.
- **Raid attribution:** `PickRaider(factions, rng)` chooses a hostile human faction, weighted by settlement count. Kit by tech level. If no hostile human faction exists, the scheduler converts the raid to another threat category (pack / manhunters / hive).

### 3.5 Skills (11)

| Skill | Key | Work types affected (existing `work.Type`) | Other effects |
|---|---|---|---|
| Shooting | `shooting` | Hunt (ranged) | Ranged hit chance |
| Melee | `melee` | Hunt (melee weapon) | Melee hit chance |
| Construction | `construction` | Construct (build, deconstruct, repair, roof, smooth) | — |
| Mining | `mining` | Mine | Ore/chunk yield |
| Cooking | `cooking` | Cook (stove/campfire bills, butchering) | Butcher yield |
| Plants | `plants` | Grow (sow, harvest, cut, cut grass) | Harvest yield, failure chance |
| Animals | `animals` | Handle (tame, milk, shear, collect) | Tame chance, product yield |
| Crafting | `crafting` | Craft (workbench, tailoring, stonecutting) | — |
| Medicine | `medicine` | Doctor (tend, rescue) | Heal amount, infection chance |
| Social | `social` | — | Chat mood bonus, tame chance ×(1+0.02·(L−7)) |
| Intellectual | `intellectual` | Research | — |

Haul, Clean and Firefight use only the WorkSpeed stat.

### 3.6 Traits (18, original; conflict groups in brackets)

| Trait | Group | Effects |
|---|---|---|
| Tireless | Pace | WorkSpeed +20% |
| Unhurried | Pace | WorkSpeed −20%, recreation drain −15% |
| Sunny | Outlook | Mood +8 |
| Brooding | Outlook | Mood −8 |
| Steady | Nerve | Break threshold −6 |
| Brittle | Nerve | Break threshold +6 |
| Fleet-footed | Gait | MoveSpeed +15% |
| Plodding | Gait | MoveSpeed −12% |
| Hearty eater | Appetite | HungerRate +25%, AteMeal thought +2 |
| Light eater | Appetite | HungerRate −20% |
| Roughneck | Standards | SleptOutside / SleptOnGround / AteRaw thoughts ×0.25 |
| Fussy | Standards | Those thoughts ×2, SleptInNiceRoom +3 |
| Deliberate aim | Trigger | ShootAcc +12, AimCooldown +20% |
| Quick draw | Trigger | AimCooldown −25%, ShootAcc −10 |
| Green thumb | — | Plants work +25% |
| Bookish | — | Research +25%, Intellectual learning ×1.5 |
| Gentle | — | Incapable of Violence, Social +3 levels |
| Cold-hardy | — | ColdComfort −6 °C |

**Generation:**
- Trait count: 1 (30%), 2 (50%), 3 (20%).
- Never two traits from the same group, and never a trait that conflicts with the backstory's incapabilities (e.g. Gentle with a soldier backstory).
- Pain tolerance trait "Sturdy" (+15%) is the 19th, ungrouped.

### 3.7 Backstories
- **~24 childhood + ~30 adulthood**, original titles and descriptions, written by the content agent.
- Each entry: `{Title, Short, Desc template using {name} {they} {their}, Skills (+2…+6 or −2), Incapable bits, Weight, Tags (tech: neolithic/industrial/spacer; for scenario fitting)}`.
- **Fitting:** Ember Clan uses neolithic-tagged backstories. Others use industrial/spacer, with neolithic allowed at 10%.
- **Example titles:**
  - Childhood: *Lighthouse kid*, *Ration-line runner*, *Orchard foundling*, *Deck-scrubber*, *Hearth-tender's child*, *Quarry urchin*.
  - Adulthood: *Convoy mechanic*, *Field medic*, *Hydroponics keeper*, *Debt collector*, *Itinerant teacher*, *Salvage diver*, *Clan tracker*, *Retired duellist*, *Seed archivist*, *Signal operator*.
- **Lint** (`cmd/contentlint`): titles unique, descriptions non-empty, no blocklisted names, incapable bits valid, skill keys valid.

### 3.8 Names
- `domain/pawn/pawngen/names.go`:
  - ~90 female and ~90 male first names: invented and common real-world names, no RimWorld lists.
  - ~110 surnames.
  - ~50 nicknames, used 35% of the time; otherwise the nickname is the first name.
- **Uniqueness:** names are unique among the pool and the existing colony.
- **Raider names:** raiders keep `raiderNames` (`raids.go`) until they gain bios: hostile pawns from factions get a generated name via `pawngen` with a faction tag.

### 3.9 Starting conditions (12% chance on generated pawns; forced by the scenario part)

| Condition | Effect |
|---|---|
| Old leg fracture | MoveSpeed −8% |
| Nearsighted | ShootAcc −15% |
| Hand tremor | Crafting and Medicine speed −10% |
| Exhausted | Bare Hands only; rest starts at 20% |

### 3.10 Defaults for a fresh draft
- Scenario Broken Hull.
- Storyteller Orla Vane, Settled, Reload anytime, or the last choices from settings.
- World: random two-word seed, coverage 30%, Rain Normal, Temp Normal, Pop Normal, default factions.
- Map 250×250 (or the largest **enabled** size ≤ 250 before the engine gate: 120×120 is offered as a temporary "Medium (pre-release)" size until §6.9 passes).
- Season Auto.

---

## 4. Domain systems (pure, deterministic; `internal/domain/...`)

### 4.1 `world/planet`: globe generation
- **Mesh:**
  - Icosahedron subdivided with frequency f, then the dual is taken: **10f²+2 tiles**, of which 12 are pentagons and the rest hexagons.
  - At f=48 that is **23,042 tiles at 100% coverage**. f is fixed, so coverage only changes how many tiles exist.
- **Coverage:**
  - Keep the tiles whose centre lies inside a spherical cap of angular radius `acos(1−2c)` around a seed-chosen pole.
  - c=0.30 gives ≈6.9k tiles; 0.50 gives ≈11.5k; 1.0 gives all of them.
  - Tiles outside the cap don't exist. Neighbour lists only reference kept tiles.
- **Storage (struct of arrays, indexed by tile):**
  - Geometry: `Center [][3]float32`, `Corners []uint32` + `CornerOff []uint32` (CSR), `Neigh []int32` + `NeighOff` (CSR).
  - Climate and terrain: `Lat, Lon []int16` (tenths of a degree), `Elev []int16` (m), `Hill []uint8`, `Biome []biome.ID`, `Water []uint8` (0 land, 1 ocean, 2 lake), `Temp []int16` (tenths °C, yearly mean), `Rain []int16` (mm/yr), `River []uint8` (0 none, 1 creek, 2 river, 3 large; flows to `RiverTo []int32`), `Caves []bool`, `Stones [][3]geology.Stone`.
  - Settlements: `Settlement []int32` (−1 none).
  - Roads: `Roads [][2]int32` (tile pairs).
  - Totals at 100%: ≈23k × ~60 B ≈ 1.4 MB.
- **Builder** (`NewBuilder(p Params) *Builder`, `Step(units int) (phase Phase, frac float32, done bool)`, `Planet() *Planet`):
  - `Params{SeedText string; Coverage 30|50|100; Rain, Temp 0..4; Pop 0..2; Factions []faction.Slot; Version uint16}`.
  - `seed = fnv64a(SeedText) ^ Version`. Each phase gets its own `randx` with a phase salt, so it is fully deterministic.
  - Phases (work units ≈ tile visits):
    1. **Shape:** build the mesh and the coverage cap. Cached per (f, coverage, pole).
    2. **Elevation:**
       - 3D value noise on the unit sphere: `pkg/noise3` (new, pure), 6 octaves, base frequency 1.6, persistence 0.5.
       - Plus a "continent" mask of 3–5 large blobs (random centres, radius 25–55°).
       - Sea level = the elevation quantile that gives **38% ± 6% land** (seeded). If land is under 25%, nudge the quantile; up to 3 deterministic retries.
       - Elevation in metres: ocean down to −4000, land 0–6000.
    3. **Climate:**
       - `Temp = 28 − 52·|sin(lat)|^1.25 − 6.5·max(0,elev)/1000 + TempOffset[Temp]` °C, where `TempOffset` = −12/−6/0/+6/+12.
       - `Rain = band(lat) × moisture(distToCoast) × RainMult[Rain]`:
         - `band` gives ≈2400 mm at the equator, a minimum of ≈300 at ±25°, ≈900 at ±50° and ≈200 at the poles.
         - `moisture = 0.35 + 0.65·exp(−d/9 tiles)`.
         - Rain shadow: −35% when a tile with elevation over 2500 m lies upwind (westerlies at mid-latitudes, easterlies in the tropics).
         - `RainMult` = 0.4/0.7/1.0/1.4/1.8.
    4. **Biomes** (land): classify by mean temperature T (°C) and rain R (mm), checked in this order:

       | Condition | Biome |
       |---|---|
       | T < −17 | Ice Sheet |
       | T < −5 | Tundra |
       | T < 5 and R > 1200 and Hill ≤ Small | Cold Bog |
       | T < 5 | Boreal Forest |
       | T < 18 and R > 1500 and Hill ≤ Small and elev < 400 | Temperate Swamp |
       | T < 18 and R > 550 | Temperate Forest |
       | T ≥ 20 and R > 2500 and Hill ≤ Small and elev < 300 | Tropical Swamp |
       | T ≥ 20 and R > 1900 | Tropical Rainforest |
       | R ≥ 250 | Arid Shrubland |
       | R ≥ 100 | Desert (if T ≥ 15; otherwise Tundra) |
       | else | Extreme Desert (if T ≥ 24, else Desert) |

       Ocean tiles with T < −8 become **Sea Ice**, which counts as land.
    5. **Hills:**
       - `g` = maximum elevation difference to the neighbours (m), plus a ridge-noise term.
       - Flat if g < 150, Small hills if < 400, Large hills if < 900, otherwise Mountainous. Elevation over 3000 m forces at least Large hills.
       - **Caves:** a hash of the tile below 30% on Mountainous, 12% on Large hills, otherwise none.
       - **Stones:** each tile picks 2–3 of the 5 stones from 5 smooth noise fields (the top 2, plus the third if within 0.08), so neighbouring tiles share stones.
    6. **Rivers:**
       - Sources: land tiles with R > 1000 and elevation > 400 m, chosen with probability 6%.
       - Each source flows downhill along the steepest neighbour descent until it reaches ocean, a lake or a local minimum. A local minimum becomes a lake when its basin holds ≥ 2 tiles; otherwise flow spills over the lowest rim.
       - Flow is accumulated along the path. Size: creek if flow ≥ 1, river if ≥ 4, large river if ≥ 12.
       - Rivers never cross Ice Sheet; they stop there.
    7. **Factions:** place settlements (§3.4) using `faction.PlaceSettlements(planet, slots, popFactor, rng)`. The faction list with names and leaders is generated here.
    8. **Roads:**
       - Build a minimum spanning tree over settlements in each landmass, plus 15% extra shortest edges to make loops.
       - Each edge is an A* path over tiles. Cost = 1 + hill penalty (0 / 0.5 / 1.5 / 4) + biome penalty (swamp 1.5, ice 3) + river crossing 2.
       - Roads are stored as tile pairs.
- **Queries:**
  - `TileAt(dir [3]float32) int`: nearest tile by lat/lon bucket (2° grid), then a greedy neighbour walk.
  - `Site(tile, season, mapSize) site.Site` (§4.2).
  - `GrowingDays(tile)`.
  - `Neighbours(tile)`.
  - `DistanceTiles(a, b)`: BFS hop count, capped at 64.
  - `Nearest(tile, pred)`.
  - `RandomLandingTile(seed, n)`.
  - `ValidLanding(tile) (ok bool, reason string)`.
- **Performance:** full build at 100% ≤ **1.5 s** (`BenchmarkPlanetBuild100`); 30% ≤ 0.5 s.
- **Tests:**
  - `TestTileCountFormula`
  - `TestPentagonsAre12`
  - `TestNeighboursSymmetric`
  - `TestCoverageFraction`
  - `TestLandShareInRange` (100 seeds)
  - `TestBiomeClassificationTable` (table cases)
  - `TestRiversFlowDownhillToWater`
  - `TestSettlementSpacing`
  - `TestRoadsConnectSettlements`
  - `TestPlanetGoldenHash` (seed "amber-hollow", 30%)
  - `TestStepBudgetDeterministic` (step sizes 1 vs 10,000 give the same planet)

### 4.2 `world/site`: from world tile to local map parameters
```go
type Site struct {
    Biome    biome.ID
    Climate  temperature.Climate   // YearMean/Amplitude/PeakDay/DaySwing from the tile
    Hills    Hills                 // Default(=legacy) Flat Small Large Mountain
    River    River                 // {Size None/Creek/River/Large; In, Out Dir8}
    Coast    Dir8Set               // edges facing ocean (0..2 edges)
    Lakes    uint8                 // 0..3 (from lake tiles adjacent / within)
    Caves    bool
    Stones   []geology.Stone       // 2..3
    Road     Dir8Set               // road edges (presentation: a dirt track)
    Latitude int16                 // tenths of a degree, for season shift
}
```
- **Climate from the tile:**
  - `YearMean = Temp`.
  - `Amplitude` grows with |lat| (6 °C at the equator to 22 °C at 60°+), plus continentality (+4 °C inland).
  - `DaySwing` comes from rain (dry gives a bigger swing: 12 °C desert, 5 °C rainforest).
  - `PeakDay = 15` in the north and `45` in the south (`(15+30) mod 60`).
- **Seasons:** new `temperature.Climate.SeasonAt(t) Season` uses the climate's `PeakDay`. The global `Season(t)` is kept for legacy. Every caller of `temperature.Season` (raids winter check, UI date strip) switches to `g.climate.SeasonAt`.
- **Edges:**
  - River In/Out edges come from neighbour bearings: the world-tile direction maps onto 8 map edges.
  - Coast: every neighbour bearing that is ocean.
- The zero `Site` makes `GenerateSite` behave exactly like `GenerateBiome` (legacy path).

### 4.3 Local map generation with water (`world/generation/site.go`, `world/terrain`)
**Terrain** (append-only, saved):

| Type | Walkable | WalkCost | Fertility | Wetness | Notes |
|---|---|---|---|---|---|
| `ShallowWater` | yes | 300 | 0 | 1000 | No building, items or stockpiles. Wading washes 50% of boot dirt and gives `mood.Soaked`. |
| `DeepWater` | no | — | 0 | — | Blocks movement and line of sight for nothing (no LOS model) |
| `Mud` | yes | 140 | 70 | 800 | Dirt gain ×2. Buildable. |

- Existing `Marsh` (walk 200, fertility 60) is reused.
- Add `IsWater(t)` and `Wetness(t)`.

**Generation** (`GenerateSite(seed uint64, w, h int, b biome.Def, s site.Site) Result`):
- **Hills thresholds** (rock/stone/edge weight):

  | Hills | Rock | Stone | Edge weight |
  |---|---|---|---|
  | Default | 0.80 | 0.71 | 0.28 (today's map) |
  | Flat | 0.88 | 0.80 | 0.15 |
  | Small | 0.82 | 0.73 | 0.28 |
  | Large | 0.75 | 0.66 | 0.34 |
  | Mountain | 0.66 | 0.58 | 0.40, plus a central mass |

  The central mass is a large ridge band across the map at a seeded angle that avoids the start (StartRadius+8).
- **Passes:**
  1. **Caves:** 3–6 worm tunnels when `Caves` is set (length 40–120, radius 1–2, turning noise). They are carved only inside Rock, turning it to rough Stone. Natural roof is kept, so `roof` marks them as cave roofs. Never within StartRadius+4.
  2. **Coast**, for each coast edge:
     - `d` = distance from that edge + value noise (±6 tiles).
     - Deep water where `d < 0.18·size`, shallow for the next 5 tiles, then a 3–5 tile **Sand** beach.
     - Water replaces rock.
  3. **River:**
     - A polyline from the In edge to the Out edge, both at noise-chosen points; when In==Out there's a bend (enters and leaves on the same side).
     - Midpoint displacement 3 levels deep, amplitude 0.12·length.
     - Pushed away from the start disc: control points within StartRadius+10 are moved outward.
     - Width: Creek 3 (all shallow), River 9 (deep core 3), Large 15 (deep core 7).
     - Banks: 1–2 tiles of Mud, noisy.
     - **Fords:** a 3-tile-wide shallow crossing every 45±10 tiles along the path, so the far bank is always reachable.
  4. **Lakes** (count = `Lakes`): noise-perturbed blobs of radius 6–14 placed outside the start disc. Deep where `dist < r−4`, otherwise shallow. Ringed by Marsh (2–4 tiles) in wet biomes (Wetness ≥ 60) or Mud elsewhere.
  5. **Wet ground:**
     - Soft ground within 4 tiles of fresh water may become Marsh, with chance (5−d)·12% in biomes whose Ground mix has Marsh > 0.
     - Otherwise Soil within 3 tiles becomes RichSoil 40% of the time (river silt).
  6. **Safety:**
     - No water within StartRadius+2 of the centre.
     - Flood-fill from the centre over walkable tiles must reach at least one map edge and at least 70% of walkable soft ground. Otherwise carve extra fords (shallow 3-wide) at the narrowest water crossings until it does; at most 8 tries, then widen fords to 5.
  7. **Plants:** none on water; reeds on Marsh as today. The plant density mix is from the biome.
- **Geology:** `geology.GenerateWith(seed, terr, stones)` uses the site's 2–3 stones instead of `pickStones`. The new ore `Machinery` (§4.8) is added to the vein table.
- **Road:** when `Road` has edges, a 2-wide **packed dirt track** (new terrain `Track`, walk 90, not farmable, appended) from the edge toward the centre. It stops at StartRadius and is presentation-useful only.
- **Performance:** `BenchmarkGenerateSite500` ≤ **2 s** (3 s hard), including plants (needs §6.2 per-cell plant index).
- **Tests:**
  - `TestZeroSiteMatchesLegacy` (golden)
  - `TestRiverEdgeToEdgeAllSizes`
  - `TestNoWaterAtStart`
  - `TestReachableSoftGround70` (50 seeds × site combinations)
  - `TestCavesOnlyInRock`
  - `TestRockShareRisesWithHills`
  - `TestCoastDepthBands`
  - `TestLakeRings`
  - `TestFordsEveryInterval`

### 4.4 `scenario`
- **Types:**
  ```go
  type Def struct { Key, Name, Summary, Desc, Intro string; Builtin bool; Seed uint64
                    Faction faction.Type; People, ChooseFrom int; Arrival Arrival; Tech TechLevel
                    Hardness uint8; Parts []Part }
  type Part struct { Kind PartKind; Trait trait.ID; Chance uint8; Who Who; N uint8
                     MinAge, MaxAge uint8; Condition pawngen.Condition
                     Item item.Kind; Qty int32; Near bool; Species animal.Species
                     Cond Condition; Incident string; FirstDay, EveryDays uint16
                     Building construction.Kind; Project research.Project; Percent uint16; Text string }
  ```
  - One tagged struct encodes simply in gob and JSON. Unused fields are zero.
- `Builtins() []Def` (§3.1).
- `Validate() []Problem`.
- `Encode(Def) []byte` / `Decode([]byte) (Def, error)`: versioned JSON. Unknown part kinds are an error that names the kind.
- `Apply(def) Effects` gives a flattened view for the application:
  ```go
  type Effects struct { People, ChooseFrom int; Arrival; Tech;
                        StartItems, Scatter []Stack; Animals []Spawn;
                        Research []research.Project; ResearchCostPct int;
                        Conditions []Condition; Scheduled []Sched; Disabled set; Disallowed set;
                        Forced Forced; NoClothes bool; Intro string }
  ```
- **Tests:** `TestBuiltinsValid`, `TestValidateCases` (every error and warning), `TestCodecRoundTrip`, `TestDecodeUnknownPart`, `TestApplyFlattens`.

### 4.5 `storyteller` and `difficulty`
- **`storyteller.Profile`:**
  ```
  {Key, Name, Title, Bio, Portrait string;
   Mode (Cycle|Calm|Dice|Legacy);
   FirstThreatDay int;
   GapMin, GapMax int;
   CycleOn, CycleOff int;
   DiceP0, DiceStep int (per-mille);
   PointsMin, PointsMax int (percent);
   RampPerCycle, RampCap int;
   PositiveBias int;
   LossEaseDays, LossEasePct int}
  ```
- **Planning:**
  - `Plan(st State, now Day, rng *randx.Rand) Decision{AtTick uint64; Category Category; PointsPct int}`.
  - `State{LastThreatDay, CyclePhase, CycleDay, QuietDays, LastLossDay, Cycles}`.
  - Categories: `BigRaid`, `Pack`, `Manhunters`, `Hive`, `Mad`, plus `None` (rest phase).
  - The category is chosen by the existing `pickThreat` rules (winter / day gates) filtered by difficulty (BigThreats, PredatorsHuntColonists, disabled incidents) and by available hostile factions.
- **Difficulty:**
  - `Settings` (§3.3) and `Preset` enum.
  - `Presets() []Named`.
  - `ThreatPoints(...)` (§3.3).
  - `Scale(kind, base int) int` for each yield/chance field.
  - `Adapter{Days, Value}` with `Day(lostColonist bool)` (§3.3).
  - `SaveMode`.
- **Tests:**
  - `TestLegacyPlanMatchesConstants` (1,000 days vs today's `scheduleThreats`/`fireThreat`)
  - `TestCycleRestHasNoBigThreats`
  - `TestCalmGapsAndLossEase`
  - `TestDiceProbabilityRisesWhenQuiet`
  - `TestPresetValues` (table)
  - `TestThreatPointsMonotonicInWealth`
  - `TestAdaptationNoFlap`

### 4.6 Pawn domain (`pawn/skill`, `pawn/stat`, `pawn/trait`, `pawn/backstory`, `pawn/look`, `pawn/pawngen`)
**`skill`:**
- `ID` (11, append-only).
- `Skill{Level uint8; XP int32; Passion uint8}` and `Set [NumSkills]Skill`.
- `ForWork(work.Type) (ID, bool)`. Hunt maps to Shooting or Melee, decided by the equipped weapon at the call site.
- `SpeedPermil(L) = 1000 + 60·(L−7)`, so 580‰ at 0 and 1780‰ at 20.
- `Cost(L) = 20000 + 6000·L` XP to reach L+1.
- `Learn(s *Skill, xp int)` with passion multipliers: none 35%, minor 100%, major 150%.
- **Decay:**
  - `DecayDay(s *Skill)` above level 10 removes `500·(L−9)` XP per day.
  - **No-flap band:** XP may go negative, and the level drops only when XP < −Cost(L−1)/2. After a drop, XP = 75% of Cost(L−1).
- **Levels:** `Label(L)` gives Novice/Capable/Skilled/Expert/Master. Max level is 20.

**`stat`:**
- `ID`: WorkSpeed, MoveSpeed, HungerRate, RestRate, MoodOffset, BreakThreshold, ColdComfort, HeatComfort, ShootAcc, MeleeAcc, AimCooldown, PainTolerance, LearnRate.
- `Sheet [N]int32`, in permille for rates and units for offsets.
- `Build(base, mods...)` applies mods in a fixed order: base → body → age → condition → traits → difficulty (PainTolerance, Mood).

**`trait`:** `Def{ID, Key, Label, Desc, Group, Mods []stat.Mod, ThoughtScale map[mood.Kind]int16, AddThought map[mood.Kind]int8, Incapable backstory.Bits, SkillBonus map[skill.ID]int8}`; `Conflicts(a, b)`; `All()`.

**`backstory`:**
- `Slot` (Childhood, Adulthood).
- `Bits` (Violence, Caring, DumbLabor, Intellectual, Animals, Cooking, Plants, Mining, Crafting, Construction).
- `Bits.WorkMask() work.Mask`:

  | Bit | Work types disabled |
  |---|---|
  | Violence | Hunt (and drafted attacks) |
  | Caring | Doctor |
  | DumbLabor | Haul, Clean |
  | Intellectual | Research |
  | Animals | Handle |
  | Cooking | Cook |
  | Plants | Grow |
  | Mining | Mine |
  | Crafting | Craft |
  | Construction | Construct |

- `Bits.Skills() []skill.ID` lists the skills shown as "—".
- `Def{ID, Slot, Title, Short, Desc, Skills map[skill.ID]int8, Incapable Bits, Weight, Tags}`.

**`look`:** `Look{Sex (F/M); Build (Thin/Average/Heavy); Skin, Hair, HairStyle, Beard, Shirt, Pants uint8; Vest bool; Grey bool}` holds palette indices only. `Legacy(id)` reproduces today's `lookFor` exactly: the hashing moves from `ebiten/art.go:536` into the domain, as **male average with today's hair styles**.

**`pawngen`:**
- `Pawn{First, Nick, Last string; Sex; Build; AgeDays int32; Childhood, Adulthood backstory.ID; Traits []trait.ID; Skills skill.Set; Look look.Look; Conditions []Condition; Temperament idle.Temperament}`.
- `Generate(seed uint64, slot, roll int, rules Rules) Pawn`, with `Rules{MinAge, MaxAge; Forced []ForcedTrait; ForcedCond; Tech tags; NoClothes}`.
- **Distributions:**
  - **Age:** 92% are 18–68, triangular with mode 30. 8% are 14–17: childhood backstory only, no adulthood ("—"), skills ×0.6.
  - **Sex:** 50/50, which drives names and art only.
  - **Build:** Thin 25%, Average 50%, Heavy 25%.

    | Build | HungerRate | ColdComfort | MoveSpeed | MeleeAcc |
    |---|---|---|---|---|
    | Thin | 900 | +2 °C | +3% | −5 |
    | Heavy | 1150 | −3 °C | −5% | +5 |

  - **Age effects:**

    | Age | LearnRate | WorkSpeed | MoveSpeed |
    |---|---|---|---|
    | under 18 | +25% | −15% | — |
    | 50–64 | — | −5% | −5% |
    | 65+ | — | −15% | −10% |

    Grey hair: 55% chance at 50+, 90% at 65+.
  - **Skills:** for each skill, `rand(0..3)` + backstory deltas + age points (`min(8, (age−18)/6)`, spread at random over non-incapable skills), clamped to 0..20. Incapable skills are fixed at 0, never get a passion and never learn.
  - **Passions:** 1 + rand(0..3) picks. Each pick is weighted by level+2 over capable skills; 30% of picks become major.
  - **Traits:** §3.6.
  - **Conditions:** 12% chance, §3.9.
  - **Temperament:** chosen from traits (Sunny → Social, Brooding → Homebody, otherwise a hash).
- `Legacy(id, name) Pawn`: all skills 7, no passions, no traits, no incapabilities, age 30, male average, `look.Legacy(id)`, temperament from today's name hash. This keeps old saves and `SpawnNamedPawn` identical.
- **Tests:**
  - `TestGenerateDeterministic`
  - `TestDistributions` (2,000 pawns: sex 50±3%, builds ±5%, ages)
  - `TestIncapableSkillsZeroNoPassion`
  - `TestTraitConflictsNever`
  - `TestBackstoryFitsTech`
  - `TestLegacyLookEqualsRenderer` (golden vs today's `lookFor` output)
  - `TestSkillXPAndDecayNoFlap`
  - `TestStatSheetOrder`

### 4.7 Determinism of setup-time randomness
- **Setup seed:** `setupSeed = fnv64a(worldSeedText) ^ uint64(tile)·φ ^ scenario.Seed`.
- **Candidate i, roll r:** `pawngen.Generate(setupSeed, i, r, rules)`.
- **Map generation:** `GenerateSite(setupSeed ^ siteSalt, …)`.
- **Simulation RNG:** `randx.New(setupSeed)`.
- **Consequence:** the same choices give the same colony, which the replay and determinism tests can rely on.

### 4.8 Items, recipes, research, buildings
- **Items** (append before `numKinds`, after `Gold`):

  | Item | Value | Notes |
  |---|---|---|
  | `Components` | 28 | Stack limit as default |
  | `IndustrialMedicine` | 24 | — |

  Pod shells deconstruct into **steel**, so no salvage item is needed. Silver stays as is (value 1 per unit, wealth only; trade later).
- **Ore:** `geology.Machinery`, "Buried machinery":
  - 1 vein per 2,500 depth≥2 rock cells, 1–3 cells per vein, yield 2 components.
  - Mined like other ores (×1.5 work).
- **Recipes:**

  | Recipe | Bench | Input | Output | Work | Skill | MinSkill | Research |
  |---|---|---|---|---|---|---|---|
  | Fabricate component | Workbench | 12 steel | 1 component | 1,500 | Crafting | 6 | **Fabrication** |
  | Refine medicine | Workbench | 2 herbal medicine | 1 industrial medicine | 800 | Medicine (under the Doctor work type: `assignWorkType(Doctor)` also takes Doctor bills) | 4 | **Pharmacology** |

- **Research** (append): **Fabrication** (5,000, requires Gunsmithing) and **Pharmacology** (4,000, requires Herbalism).
- **Construction:**
  - `Def.Components int`; `Frame.Comp, CompNeed int`.
  - `Ready() = Wood ≥ Need && Comp ≥ CompNeed`.
  - Haulers deliver the material first, then the components (haul-route extension).
- **New buildings:**

  | Building | Size | Cost | Work | Effect |
  |---|---|---|---|---|
  | `ToolCabinet` | 1×1 | 30 steel + 1 component | 120 | Benches within 6 tiles: bill work +6%, max 2 cabinets |
  | `HospitalBed` | 1×2, rotatable | 45 wood or steel + 2 components | 200 | Tending a patient in it: heal +15%, infection chance ×0.5 |
  | `PodShell` | 1×1, walkable | — (placed by arrival) | — | Deconstructs into 6 steel |

- **Steel:** `MetalOK` added to Table, Chair, Bed and DoubleBed.
- **Medicine tiers:**
  - `infection.Care` gains an `IndustrialMedicine` tier: infection chance 1%, recovery step −12 per hour, `tendHeal ×2.5`.
  - `withMedicine` picks industrial when injury ≥ `severeInjury` or the wound is infected, otherwise herbal, with the other as fallback.
  - The choice is **fixed at job start** (no flap).
- **Stockpiles:** `allows()` accepts components and industrial medicine as goods.
- **Tests:**
  - `TestMachineryOreYieldsComponents`
  - `TestComponentFramesWait`
  - `TestFabricateNeedsSkillAndResearch`
  - `TestRefineMedicineDoctorBill`
  - `TestIndustrialBeatsHerbal`
  - `TestMedicineChoiceNoFlap`
  - `TestToolCabinetBonusCap`
  - `TestHospitalBedHeal`
  - `TestPodShellDeconstructsToSteel`

---

## 5. Application integration (`internal/application/game`)

### 5.1 `Setup` and colony creation
```go
type Setup struct {
    Scenario    scenario.Def            // embedded (custom scenarios survive without the file)
    Storyteller string                  // profile key
    Difficulty  difficulty.Settings     // resolved (preset copied or custom)
    Preset      difficulty.Preset
    SaveMode    difficulty.SaveMode
    World       planet.Params           // regenerated on load
    Tile        int
    Site        site.Site               // derived; stored for robustness
    Factions    []faction.Faction       // generated state at start
    MapSize     int                     // square side
    Season      clock.Season            // resolved (never Auto here)
    Colonists   []pawngen.Pawn          // the Selected candidates, in order
    Seed        uint64                  // setupSeed (§4.7)
}
type Config struct { Seed uint64; Width, Height int; Biome biome.ID; Setup *Setup }
```
- **`NewColony(cfg)` with `Setup == nil`** is unchanged.
- **With `Setup`**, `NewColonyBuilder(cfg)` runs in steps:
  1. `New(cfg)`: `Width = Height = MapSize`; climate = `Site.Climate`; the clock starts at the chosen season's first day, 08:00, with the hemisphere shift applied; `rng = randx.New(Setup.Seed)`.
  2. `generateSite`: `GenerateSite` + `geology.GenerateWith` + `seedGrass`.
  3. Factions: copy `Setup.Factions` into `g.factions`. The player faction is named from the scenario.
  4. Arrival (§2.11): colonists from `Setup.Colonists` via `spawnGenerated(pawn, x, y)`, items, scatter and animals.
  5. `startHome()`, centred on the landing spot.
  6. Research: start empty; then apply grants; `researchCostPct` from tech × parts.
  7. Scenario rules: conditions, scheduled and disabled incidents, disallowed buildings.
  8. `spawnWildlife` with the new cap (§6.5).
  9. Storyteller state init, `scheduleThreats()` via `Plan`.
  10. Intro letter queued.
- **`spawnGenerated(p pawngen.Pawn, x, y int) EntityID`** creates the same stores as `SpawnNamedPawn`, plus:
  - bios, skills and stats;
  - the labor mask (incapable types removed) and priorities (incapable set to 0);
  - apparel from the scenario (`NoClothes` → none);
  - the look.
- **Disallowed buildings:** `CanPlaceAt` / `canPlace` return `reasonScenario` ("Not allowed by this scenario"). `Buildables()` marks them `Locked` with that reason.
- **Permanent conditions** are applied as climate/plant modifiers in `climate.go` and `world.go` (growth):

  | Condition | Effect |
  |---|---|
  | Ashfall sky | Daylight factor ×0.8; crop growth ×0.85 |
  | Endless chill | `YearMean −80` (tenths °C) |
  | Long summer | `YearMean +60`; Amplitude ×0.7 |
  | Still air | Weather Markov forced to Clear/Overcast |

- **Scheduled incidents** reuse `fireThreat` / incident dispatch by key at `FirstDay` and every `EveryDays` after. Disabled incidents are filtered in `pickThreat` and `phaseIncidents`.

### 5.2 Threat scheduler (`raids.go`, `raid_plans.go`, `incidents.go`)
- **`scheduleThreats`** and **`fireThreat`**:
  - With a Setup, call `storyteller.Plan(g.story, day, g.rng)` to decide the time and category.
  - The legacy profile reproduces `nextThreat = 5d + rand(1d)` and gaps of 5 + rand(5) exactly.
- **`pickThreat(roll)`** is unchanged for legacy. With a Setup it is filtered:
  - `!BigThreats` → no Raid or Hive.
  - `!PredatorsHuntColonists` → no Pack.
  - Disabled incidents are removed.
  - No hostile human faction → Raid becomes Pack / Manhunters / Hive by the same die.
- **`raidPoints`**: legacy keeps `Wealth/40 + standing×30 + day×2`; with a Setup, `difficulty.ThreatPoints` (§3.3) is used. Then `× storyteller PointsPct`.
- **Raid groups:**
  - `raidGroup` gains `Faction int` and the kit comes from the faction's tech: Clans use spears, clubs and knives; Freebooters get +20% points.
  - Raiders get generated bios: `pawngen` with the faction's tech tags. Raider names now come from bios.
- **Messages:** "Raid! 6 raiders of the Ashen Compact are coming." Siege and sapper messages are the same with the faction name.
- **After a raid is defeated or leaves:** that faction's goodwill −5.
- **Adaptation:** `g.adapter.Day(lost)` runs at the day rollover (in the existing hourly phase, at hour 0). `lost` is true when a colonist died in the last 24 h.
- **Peaceful days:** the first threat is never before `max(storyteller.FirstThreatDay, difficulty.PeacefulDays)`.

### 5.3 Difficulty hooks (exact sites)

| Field | Site | Change |
|---|---|---|
| CropYield | `doHarvest` (`tick.go`) | `qty = Scale(CropYield, d.Yield)` and the Plants-skill yield (§5.4) |
| MineYield | `doMine` (`world.go`) | Chunk/ore qty ×% (chunks round at least 1) |
| ButcherYield | `doBill` butcher branch (`production.go`) | Meat and leather ×% |
| ResearchSpeed | `doResearch` | Units ×% |
| MoodOffset | `thoughts()` / `moodOf` (`rooms.go`/`mood.go`) | Add a constant "Difficulty" thought line (hidden when 0) |
| Infection | `woundOpened` (`infection.go`) | Chance ×% (clamped 0–100) |
| DiseaseFreq | (hook only) | `g.diff.DiseaseFreq` stored; noted in the handoff |
| AnimalRevenge | `animals.go`/`handling.go` revenge roll | ×% |
| PredatorsHuntColonists | `wildlife.go` predator target selection | When off, wild predators never target colonists (they still hunt animals) |
| ExtremeWeather | `weather.go` | New events **Heat wave** (+12 °C, 2–4 days, summer) and **Cold snap** (−14 °C, 2–4 days, winter). Probability 6% per in-season day; off when the toggle is off. Shown as a letter and in the climate panel. |
| PainTolerance | `stat.Build` difficulty step | Downed threshold offset |
| ThreatScale / BigThreats / Adaptation / TimeEscalation / IgnoreWealth / PeacefulDays | §5.2 | — |

**In-game changes:** `command.SetStoryteller{Key}` and `command.SetDifficulty{Preset, Settings}` take effect at the next scheduling decision. The current pending threat stays. SaveMode can't be changed.

### 5.4 Skills, stats and traits in play (`game/skills.go`, new)
- **Stores:**
  - `bios *component.Store[pawngen.Pawn]` (the generated identity, saved)
  - `skills map[EntityID]*skill.Set`
  - `stats map[EntityID]stat.Sheet` (rebuilt on level, condition, trait, birthday or difficulty change)
  - `workCarry map[EntityID]int32` (fractional progress, permille)
- **`workProgress(id, sk skill.ID, hasSkill bool) int`** replaces the boolean `workTick(id)` (`rooms.go:289`):
  - `rate = moodGate(id) × WorkSpeed × (hasSkill ? SpeedPermil(level) : 1000) / 1e6`. `moodGate` keeps today's rule: every 4th tick is skipped when mood is below 30.
  - `carry += rate`; `units = carry/1000`; `carry %= 1000`.
  - A neutral pawn returns exactly 1 per tick, so legacy timings are identical.
  - Also calls `learn(id, sk, units)`.
- **Call sites** (each `if !g.workTick(id) {...}; WorkLeft--` becomes `WorkLeft -= g.workProgress(id, skill, true)`):

  | Function (file) | Skill | Extra |
  |---|---|---|
  | `doConstruct`, `doDeconstruct` (×1.5 speed), `doRepair` (`defense.go`), `doRoof` (`roof.go`), `doSmooth` | Construction | — |
  | `doMine` (`world.go`) | Mining | Yield ×clamp(100 + 4·(L−7), 60, 140)% |
  | `doSow` (`food.go`), `doHarvest` (`tick.go`), `doCut`, `doCutGrass` (`grass.go`) | Plants | Harvest yield same formula; below L4, harvest fails (4−L)·6% of the time and drops nothing, with message "<name> fumbled the harvest" |
  | `doBill` (`production.go`) | Recipe `Learn` skill (new `recipe.Def.Learn skill.ID`, `MinSkill uint8`): cooking/butcher → Cooking, craft/stonecut/tailor → Crafting, refine medicine → Medicine | Butcher yield; a bill is not taken by pawns under `MinSkill` (`ReasonSkill` in orders) |
  | `doTend`, `doRescue` (`medical.go`) | Medicine | Heal ×(100 + 5·(L−7))%; infection chance ×max(10, 100 − 6·(L−7))% |
  | `doResearch` (`research.go`) | Intellectual | `research.Add(cur, units × ResearchSpeed%)` |
  | `doTame` (`handling.go`), `doCollect` (milk/shear/eggs, `animal_products.go`) | Animals | Tame chance ×(100 + 6·(L−7))% × Social factor; product yield ±3%/level |
  | `strike` (`combat.go:154`) | Shooting / Melee | Hit chance ×clamp(100 + 4·(L−7), 50, 160)% + ShootAcc/MeleeAcc offsets; aim cooldown × AimCooldown |
  | `doClean`, `doHaul`, `doRefuel`, `doBury`, `doDig`, `doWarm` | none | WorkSpeed only |
  | `doIdle` chat (`idle.go`) | Social | 40 XP per chat; chat thought +1 at Social ≥ 12 |

- **Learning:**
  - 1 XP per unit of progress, × passion, × LearnRate. Combat gives 120 XP per attack. Learning is capped at 4,000 XP per skill per day ("saturation", reset at day rollover) so 3× speed can't over-level.
  - **Daily decay** runs at the pawn's hour bucket (`id % 24`, the pattern from `mental.go`).
- **Passion joy:**
  - While doing work tied to a minor or major passion skill, the recreation drain is −20% / −40%.
  - State thought `mood.LovesWork` ("Doing work I love") +2 / +4, shown only while working.
- **Traits in play:**
  - Mood offset in `thoughts()`; trait thought scales.
  - `BreakThreshold` in `mental.go` (`Threshold + offset`).
  - `HungerRate`/`RestRate` in the needs decay.
  - `MoveSpeed` in `weatherSlow` (step time ÷ MoveSpeed).
  - `ColdComfort`/`HeatComfort` in `bandsOf`.
  - Temperament from the bio.
- **Incapable work:**
  - `prioOf`/`setPriority` force incapable types to 0. `command.SetWorkPriority` on an incapable type is ignored, with a toast "Incapable: <reason>".
  - Forced orders (`orders.go`) return `ReasonIncapable` with the reason text.
  - `assignWorkType` never assigns.
  - **Violence-incapable:** may be drafted (to move), but `doCombat` never attacks, hunting is skipped, and auto self-defence retreats instead.
- **Birthdays:** every 60 days from the arrival offset; `AgeDays++` daily. Age thresholds rebuild the stat sheet. A letter "<name> turned 35" is optional (off by default, Options toggle).
- **Read models:**
  - `PawnView` gains `Sex string`, `Build string`, `AgeYears int`, `Look look.Look`, `Incapable work.Mask`, `IncapableWhy map[work.Type]string` (only filled when the inspector asks), `SkillLv [11]uint8`, `Passion [11]uint8`, `Title string` (adulthood or childhood title).
  - `BioView(id) (BioView, bool)` is on demand: full name, backstories (title + description with pronouns filled), traits (label + desc + effects text), skills (level, label, XP%, passion, affected work, speed%), incapabilities with reasons, conditions, stats summary.
  - `CandidatePool` (pre-game, no `Game`):
    - `NewCandidatePool(setupSeed, scenario.Effects)`
    - `Views() []CandidateView` (same fields as BioView, plus `Selected`)
    - `Reroll(i, keepName bool)`, `RandomizeAll()`, `Rename(i, first, nick, last) error`, `Move(i, j)`, `Swap(i, j)`
    - `Team() TeamReport{Best [11]{Level, Who}; Warnings []string}`
    - `Selected() []pawngen.Pawn`
- **Tests:**
  - `TestNeutralPawnTimingsUnchanged` (runs a representative set of existing timing tests through `workProgress`)
  - `TestSkillSpeedRatio` (L20 vs L0 builder ≈ 1780/580)
  - `TestLearningSaturation`
  - `TestDecayNoFlapInPlay`
  - `TestIncapablePriorityIgnoredAndOrderRefused`
  - `TestViolenceIncapableNeverStrikes`
  - `TestAccuracyRisesWithSkill` (400 shots, fixed seed)
  - `TestTraitMoodAndBreakOffsets`
  - `TestBirthdayRebuildsStats`
  - `TestCandidatePoolOps`
  - `TestTeamWarnings`

### 5.5 Factions in play (`game/factions.go`)
- **State:** `g.factions []faction.Faction` and `g.playerFaction`.
- **Daily:** `faction.DailyDrift` at the day rollover.
- **Raid outcome:** goodwill hooks.
- **Read model:** `Factions() []FactionView{ID, Name, Type, Leader, LeaderTitle, Goodwill, Relation, Settlements int, Tech, Emblem, Accent, History []int16 (last 60 days), Events []string (last 5)}`.
- `Story() StoryView{Storyteller, Title, Preset, Difficulty difficulty.Settings, Adaptation int, SaveMode, NextThreatHint string (dev only)}`.
- Both are added to the `Simulation` interface (`ebiten/sim.go` or wherever `Simulation` lives), with a compile-time assertion in `cmd/game`.

### 5.6 Water and new items in play
- **Placement:** `canPlace` returns `reasonWater` ("Can't build on water") for any water cell in a footprint. Floors and carpets are also refused on water.
- **Items:** `dropStack` and `nearestFreeCell` skip water. Stockpile painting skips water cells, and the tool shows them as invalid.
- **Spawning:** wildlife, raider and hive spawns, and edge-entry tiles (`edgeTile`) skip water. Hive exits never use water. Animals avoid deep water (blocked) and wade shallow.
- **Paths:** `crowdCost.ExtraCost` adds `(WalkCost − 100)/10` per step on non-floored terrain, so A* avoids shallow water and marsh without banning them. Step time already scales by `WalkCost` in `weatherSlow`.
- **Dirt** (`filth.go`): gain uses `max(weatherWetPm, terrain.Wetness(t))`. Mud ×2. Wading in shallow water halves carried dirt. `mood.Soaked` memory when wading.
- **Temperature:** water tiles don't change air temperature (no water thermals).
- **Components:**
  - Frames with `CompNeed` show "Needs 2 components" in the frame inspector.
  - Hauling delivers components after the material (`haul_route.go` gains a second material slot).
  - Deconstruction refunds 50% of components (round down).
- **Medicine:** tending takes from the selected tier (§4.8). The Doctor work type also runs "refine medicine" bills.
- **ToolCabinet:** `doBill` speed bonus +6% per cabinet within 6 tiles (max 2), from a cached per-bench count invalidated on building changes.
- **HospitalBed:** tending a pawn lying in one applies the bonus. Rescue prefers hospital beds.

### 5.7 Ports and adapters
- **Scenario store port** (consumer-owned, defined by the adapter that uses it, per `HEXAGONAL_ARCHITECTURE.md` §6): `ebiten.ScenarioStore{List() ([]ScenarioFile, error); Load(name) ([]byte, error); Save(name string, data []byte) error; Delete(name) error}`.
  - Implemented by `internal/adapters/outbound/scenariofile` (atomic writes, dir `<UserConfigDir>/rimworld-go/scenarios`).
  - Wired in `cmd/game` as `AppConfig.Scenarios`.
- **Save hooks:** `SaveHooks` gains `SaveCommit(sim, slot)`, which is identical but tags `SaveInfo.Commitment`. `filesave` gets atomic writes (§2.13).
- **Settings:** `config.Settings.Gameplay` gains `LastStoryteller string`, `LastDifficulty string`, `LastSaveMode string`, `PauseOnStart bool` (default true), `BirthdayLetters bool`. `config.Settings.Accessibility` gains `UIScale int` (default 100). `Normalize` clamps them.

### 5.8 Saves (`save.go`, `SaveVersion = 2`)
- **New `saveState` fields** (gob, additive):
  - `Setup *Setup` (the world is **not** stored, only params + tile; regenerated on load, ~0.5–1.5 s, done by the load builder with progress)
  - `Factions`, `PlayerFaction`
  - `Climate temperature.Climate` (stop re-deriving it from the biome on load when a Setup exists)
  - `Story storyteller.State`, `Adapt difficulty.Adapter`
  - `Bios`, `Skills`, `WorkCarry`
  - `Conditions`, `Scheduled`
  - `CompFrames` (component progress)
  - `Arrival` (pod shells done)
- **`SaveInfo`** gains `Commitment bool`, `Storyteller string`, `Difficulty string`, `MapSize int`, `Biome string`.
- **v1 → v2 migration** (load accepts 1 and 2; new saves write 2):
  - `Setup = nil`, so the legacy storyteller and difficulty are used.
  - `Bios` from `pawngen.Legacy(id, name)`, so looks and temperament are unchanged.
  - Factions: none, so raids remain unattributed ("Raiders").
  - Climate re-derived as today.
- **Golden fixture:** `testdata/premotion_v1.sav` must still load. Add `testdata/ng1_v2.sav` as a v2 fixture after Stage S3.
- **Tests:**
  - `TestLoadV1MigratesBios`
  - `TestSaveV2RoundTripSetup`
  - `TestWorldRegeneratedOnLoadMatches` (hash of planet)
  - `TestClimateNotRederivedForSetup`
  - `TestSaveInfoFields`

---

## 6. Engine scale: 60 fps from 200² to 500² (D6)

### 6.1 Budgets
| Item | Budget |
|---|---|
| Frame | 16.67 ms at 60 fps |
| Sim at 3× (`speedFast`, 3 ticks/frame) | ≤ 6 ms/frame → **avg tick ≤ 1.5 ms at 500², ≤ 1.0 ms at 250²**, p99 ≤ 4 ms, any 3 consecutive ticks ≤ 7 ms |
| Read models (`World()`+`Snapshot()`) | ≤ 0.5 ms and ≤ 64 KB allocated per frame when nothing changed |
| Draw | ≤ 6 ms CPU, ≤ 6 000 draw calls at every zoom |
| Remainder | ~3.6 ms GC, audio, GPU submit |
| Allocation | ≤ 2 KB per tick in steady state |
| Legacy | `BenchmarkColonyHour` (48×40) within +10% of today's 17.5 ms/hour |

### 6.2 E0: Measure first (lands before any optimisation)
- **Fixture** `internal/application/scenario.GrownColony(seed, w, h, opts)`. It builds through public dev hooks only (`BuildNow`, `SpawnNamedPawn`, `SpawnStack`, zone and designation commands, `Raid`).
  - 20 colonists.
  - 50 tame animals in a fenced pen, plus wildlife at the cap.
  - About 400 buildings in walled rooms with doors.
  - 1,000 stacks.
  - 600 grow cells, 200 mine designations, 50 blueprints.
  - Stains and dirt.
  - 2-day warm-up. Built once with `sync.Once`, saved to bytes, and each benchmark `Load`s it.
- **Benchmarks:**

  | Benchmark | Location | Sizes / variants |
  |---|---|---|
  | `BenchmarkTickGrown` | `game/scale_bench_test.go` | {120, 250, 500}; `ReportMetric` `ms/tick`, `p99-ms/tick`, `max-ms/3ticks` |
  | `BenchmarkTickRaid` | `game/scale_bench_test.go` | 500, with `Raid(12)` |
  | `BenchmarkReadModel` | `game/scale_bench_test.go` | 500 |
  | `BenchmarkDrawArt500` | `ebiten/draw_bench_test.go` | zoom z1, z0.5, z0.25, zmin; reports draw-call count from a counting wrapper |
  | `BenchmarkGenerateSite500`, `BenchmarkSave500`, `BenchmarkLoad500`, `BenchmarkReachable500`, `BenchmarkPath500Long`, `BenchmarkPathUnreachable500`, `BenchmarkRoomsAfterWall500`, `BenchmarkPlanetBuild100`, `BenchmarkGlobeFrame`, `BenchmarkHUDText` | — | — |

- **Runner:** `cmd/benchmark/main.go` runs fixture × size × speed and prints p50/p95/p99 per tick and per phase. It uses an optional `game.PhaseObserver` hook set only from `cmd/*`. Wall-clock time is used for observation only and never feeds gameplay.
- **Dev flags** in `cmd/game`: `-cpuprofile`, `-memprofile`, `-trace`.
- **pprof workflow:** `go test ./internal/application/game -run '^$' -bench 'TickGrown/500' -benchtime 3000x -cpuprofile cpu.out`, then `go tool pprof -top -cum cpu.out`. Use `benchstat` for before/after.
- **Golden state hashes:**
  - `domain/sim/statehash`: `Game.StateHash()` = FNV-64 of `Save` bytes.
  - `TestScaleGolden`: seeds 1/7/42 at 48×40 and 120², 20k ticks. Pinned before E1.

### 6.3 E1: Behaviour-preserving fixes for the costs that grow with area
1. **Walkability storage:** `grid.Grid.blocked` (`grid.go`) changes from `map[int]bool` to a `[]uint64` bitset. `Blocked()`/`Restore` keep the save format.
2. **Per-cell indexes** replace the linear scans. Each is maintained at spawn, move and delete:

   | Function (file) | New index |
   |---|---|
   | `plantAt` (`orders.go`) | `plantGrid []EntityID` |
   | `itemAt` (`tick.go`) | Per-cell item head + next links |
   | `layerBlockerAt`, `layerBldgAt` (`placement.go`) | `bldgGrid [layers][]EntityID` |
   | `blockingFrameAt` | `frameGrid` |
   | `isBoundary` (`rooms.go`) | Reads `bldgGrid` |

   This also fixes the O(P²) plant spawning during generation, which blocks 500² maps.
3. **Removals:** `removePlant` (`world.go`) and `deleteItem` (`tick.go`) use tombstones plus an id→index map, compacting in order once tombstones pass 25%. Iteration order is preserved.
4. **Version-keyed caches:**
   - `weatherSeed` (`weather.go`): cached per terrain version.
   - `hostileMap` / `buildingCells` (`defense.go`): bitsets cached per building version.
   - `sapperBreaks` (`raid_plans.go`): cached.
5. **Aggregates:**
   - `hasEdible` (`tick.go`): a maintained count of edible free stacks.
   - `mineList` (`world.go`): a maintained sorted slice.
   - `frostCrops` / `rotItems` (`climate.go`): no full ID copy; they iterate the tombstoned lists.
6. **Read models:**
   - Per-tile layers become versioned copy-on-write: `TerrainVer`, `RoomVer` (`RoomGrid` → `[]int32`), `RoofVer`, `GrassVer`, `HomeVer` (no `slices.Clone` per frame), `DirtVer`, `StainVer`, all following the existing `stoneLayers()` pattern.
   - `World()` / `Snapshot()` are cached per (tick, command generation).
   - The renderer fetches once per frame in `advance` / `drawArt` and passes the views down.
   - `lightGrid.stale` (`light.go`) is keyed on versions, not slice pointers.

### 6.4 E2: Chunks, regions, rooms (`domain/world/chunk`, `domain/world/region`)
- **Chunks:** 16×16 tiles; 32×32 = 1,024 chunks at 500².
- **Regions:**
  - A region is a **4-connected walkable flood within one chunk**. Diagonal moves can't cut corners in `pathing.Find`, so 4-connectivity matches A* connectivity exactly.
  - Doors, gates, fence cells and trap cells are each a 1-cell region with a flag.
  - **Links** are runs of shared edge cells between neighbouring chunks' regions.
- **Movement layers**, each with connected-component labels computed by BFS over the region graph in ascending region-id order:

  | Layer | Rule |
  |---|---|
  | Colonist | Doors passable |
  | Hostile | Doors closed (`hostileMap`) |
  | Tame | Fences block, gates pass |
  | Wild | Fences and gates block |

- **Reachability:** `reachable` (`orders.go`) and `reachableSpot` (`world.go`) become label comparisons, keeping `pathTo`'s rule that an unwalkable target counts as reached from any adjacent walkable cell. Retire `fenceLabels` (`fences.go`).
- **Dirty tracking:** a cell walkability change marks its chunk dirty, plus neighbours when the cell is on a chunk edge, in a bitset. Dirty chunks are processed in index order: re-flood (≤ 256 cells), relink, then relabel components (≈3k nodes, < 50 µs).
- **Rooms:**
  - A second partition per chunk uses the room boundary predicate (rock, walls, doors) to make room-regions.
  - A room is a component of room-regions; it is outdoors if it touches the map edge.
  - Room IDs are ordered by each room's smallest cell index, so numbering is identical to `room.Detect`.
  - Per-region contents counters (cells, floored, beds, tables, chairs, lamps, benches, stone/total walls, filth sums) are maintained, replacing the full rescan in `ensureRooms` (`rooms.go`).
  - `rebuildFilth` (`filth.go`), `countRoofed`/`refreshRoofSupport` (`roof.go`) and `remapRoomTemps` run only for dirty chunks and changed rooms.
  - `roomAt` = `roomOf[regionOf[cell]]`.
- **Tests:**
  - `FuzzRegionReachability` / `TestRegionReachMatchesAStar`: random maps × all layers × random edits (walls, doors, fences, gates, mining, water).
  - `TestRoomsIncrementalMatchesDetect`: room grid, IDs and contents identical after every edit.

### 6.5 E3: Pathfinding
- **Unreachable targets:** check the region label before running A*. Today an unreachable target floods the whole component, and `pathTo` can repeat it up to 9 times.
- **A* scratch:** `[]int32` instead of `[]int`, with a bitset fast path that avoids the interface call per cell.
- **Long paths** (octile distance > 96 tiles):
  1. A* over the region graph (link midpoints).
  2. Tile A* restricted to the corridor of regions on that path plus one ring.
  - Paths shorter than that stay exact tile A*, so tests on small maps don't change.
- **Raid entry:** probes in `raids.go` are pre-filtered by region labels. `hiveExit` (`hive.go`) is cached per structure version.
- **Per-tick budget:** at most 6 long searches per tick. The rest are deferred to the next tick in entity-id order (deterministic).
- **Tests:** `TestCorridorPathValid` (walkable, no corner cutting, cost ≤ 1.10× optimal, exact under the threshold) and `TestLongSearchBudgetDeterministic`.

### 6.6 E4: Staggered periodic work and job-giver indexes
- **Plants:** grown per chunk every 250 ticks, staggered by chunk index, via `plant.GrowSpan(f, t0, t1)`.
  - This re-pins the golden hashes once (with a reason) because `climate.Outdoor` changes every tick.
  - `frostCrops` folds into the same pass.
  - `TestGrowSpanTelescopes`.
- **Climate-interval work** (`heatRooms` in `heat.go`, `stainDecay` in `stains.go`, `checkRoofSupport` in `roof.go`) is bucketed across the 250-tick window. `checkRoofSupport` uses a dirty list.
- **Job givers:**
  - Maintained sets: ready frames, deconstruct marks, harvest-marked plants, cut-marked plants, haulable stacks (not already in a matching stockpile), and refuel/feed targets.
  - The item `bucketGrid` (`animal_index.go`) becomes incremental instead of rebuilt every tick.
  - Searches go ring by ring over buckets with the same (distance, id) tie-break: `haulToStockpile` (`tick.go`), `constructTry`, `nearestFree` (`orders.go`), `nearestPlant` (`world.go`), `stockpileCell` (`zones.go`), `assignSleep`.
- **Wildlife cap (D15):** `wildlifeCap` (`population.go:69`) = `Wildlife × min(W·H, 14 400)/10 000` for maps ≤ 120², otherwise `Wildlife × 1.44 × sqrt(W·H/14 400)`, capped at **300**. The winter reduction is applied after the cap.

### 6.7 E5: Renderer
- **Per-chunk lists:** the read model provides visible lists per chunk: `ChunkLayer(cx, cy) ChunkView{Ver; Plants, Items, Buildings []id}` and `PlantsIn(rect, buf)`.
- **No full-map loops per frame.** These loops are removed:
  - `draw_art.go` plant/item/building loops
  - `shadow.go`
  - `environment.go`
  - `grass_draw.go`
  - `sound_signals.go` (per-frame maps)
- **Allocation:** cull before any closure; replace `artDraw` closures with plain records; reuse buffers for `occupied` and `creatures`.
- **Level of detail** by on-screen tile size:

  | Tile size | Rendering |
  |---|---|
  | ≥ 48 px | Live per-tile drawing (today) |
  | 16–48 px | **Baked chunk images** at 16 px/tile (256² each, LRU of 96 ≈ 24 MB): terrain, smoothed stone, rock autotiles, floors, carpets, water. Rebaked when the chunk's version changes. |
  | < 16 px | **Whole-map bake** at 4 px/tile (2000² at 500², split into ≤ 4096-px pages): plants as dots, no shadows, tufts, decals or item sprites. Creatures are still drawn as sprites with a minimum size; selection rings stay. |

- **Lighting:** the light grid updates only dirty chunks.
- **Minimap** (§2.12): a 4 px/tile bake reused at small scale, plus the camera rect.
- **Interface:** only through new narrow methods on the consumer-owned `Simulation` interface; the renderer never touches domain types it doesn't already use.

### 6.8 E6: Memory, saves, generation at 500²
- **Memory:** ≈30 MB of per-tile state, with A* scratch at 3.25 MB as `int32` and the occupancy grid at 2 MB.
- **Saves:**
  - File ≤ 4 MB (gob + gzip). Drop `Blocked` from new saves and rebuild it from terrain and buildings, keeping v1 loading.
  - Save ≤ 300 ms: encode from a snapshot on a worker goroutine. The snapshot is taken on the sim thread, and the goroutine only serialises immutable data.
  - Load ≤ 1.5 s, including the region/room builds and the planet regeneration (§5.8). The planet rebuild is cached per params hash within the session.
- **Generation:** ≤ 2 s, 3 s hard limit, with progress shown (§2.10).

### 6.9 Gate for exposing map sizes
- **Standard sizes (200–300):** enabled when every §6.1 budget passes on `TickGrown/250`, `TickRaid/250` and `DrawArt` at 250².
- **Large sizes (350–500):** enabled when every budget passes at 500², plus a **manual run**: 10 minutes at 3× on seed "amber-hollow", 500², a temperate river tile, with `ebiten.ActualFPS() ≥ 59` and the 1% low ≥ 60 fps (≤ 16.7 ms), recorded in the handoff.
- **Before the gate:**
  - The Advanced modal shows those sizes disabled with the reason "Not available yet".
  - The default map size falls back to the largest enabled size. 120×120 serves as a "Medium (pre-release)" size until the 250² gate passes, so the flow is usable throughout.
- **Implementation:** `game.MapSizes() []MapSizeView{Side int; Enabled bool; Reason string}` reads a lead-owned constant `game.MaxEnabledSide`, raised only by the lead after the gates pass.

---

## 7. Art and audio (skill `rimworld-go-art-assets`; ART spec sections added by the lead in Phase 0)

### 7.1 Pawn builds (D9)
- **Rig:** one parametrised rig in `assets/source/blender/artlib/families/pawn.py`.
  - New `BUILDS` table: `m_thin, m_avg, m_heavy, f_thin, f_avg, f_heavy` → torso width, chest and hip offsets, shoulder width, limb radius, neck, head scale (±3%).
  - The body and clothing primitives (`_body`, `_pants`, `_shirt`, `_vest`, `_parka`, `_duster`, `_tuque`) read the build dimensions.
  - Poses and attachments (`hand`, `head`) are computed from the rig, so held items and hair still align.
- **Keys:**
  - `pawn/<layer>_<build>_<facing>_<frame>`, where layer is body, shirt, pants, vest, parka or duster.
  - The existing `pawn/body_…` keys become **aliases for `m_avg`**, so old content and tests keep working.
- **Volume:** today there are 6,768 pawn files (51 MB). The 5 new builds add ≈12,600 sprites (with mask and normal ≈ 38k files, ≈255 MB unpacked).
- **Atlas packing** (D9):
  - New tool `cmd/atlasgen`: shelf packing into ≤ 4096² pages, with metadata `assets/metadata/atlas/<family>.yaml` (key → page, rect, origin).
  - Packs the pawn family including masks and normals into matching pages.
  - `make art` runs it after normals.
  - The renderer's art catalog loads atlas pages and sub-images via `SubImage`.
  - `TestAtlasCoversKeys` and `validate_assets.py` check atlas integrity.
  - Target: pawn family ≤ 120 MB on disk.
- **Hair and beards** (shared across builds):
  - 5 new hair styles: long, braid, shaved, curls, tail. The existing short/crop/bun stay.
  - 3 beards: short, full, moustache. Males only, 40% chance.
  - 4 facings each.
  - A grey-hair palette entry.
- **Render time:** ≈10 minutes per build (~50 min total), run with a private `TMPDIR` per worker as today.
- **Renderer:**
  - `lookFor` is replaced by `PawnView.Look` (domain `look.Look`) → tint names plus a build prefix.
  - The pawn layer composer picks `<layer>_<build>`.
  - Portraits (`drawLookPortrait`, §2.9) use the same composer.

### 7.2 Storyteller busts (D5)
- **Family:** new `families/portrait.py`, rendered at 512×640 with a transparent background through the same stage and lighting contract, plus a 3/4 view.
- **Characters** (original designs, briefs in the ART spec):
  - **Orla Vane:** late 50s, close-cropped silver hair, dispatcher's headset around the neck, quilted navy jacket with a reflective stripe, calm half-smile.
  - **Bram Holloway:** 60s, broad, ruddy, short white beard, knitted cardigan over an apron, holding a steaming mug.
  - **Kestrel Quay:** 30s, angular, asymmetric dark hair with one shaved side, long coat with brass buttons, a card held between two fingers, one raised eyebrow.
- **Outputs:** `ui_portrait/<key>` (generated/ui/portraits/<key>.png) (512×640) and a 120×150 thumbnail.
- **Never** reference RimWorld storyteller art. Silhouettes, palettes and props must differ clearly.

### 7.3 World and UI art
| Asset | Keys | Count |
|---|---|---|
| Faction emblems | `ui_icon/faction_<type>` (settlers, clans, drifters, freebooters, swarm, player) | 6 |
| Settlement icons | `ui_icon/settlement_<tech>` (neolithic, industrial) | 2 (tinted by accent) |
| Scenario icons | `ui_icon/scenario_<key>` (+ `scenario_custom`) | 5 |
| Passion icons | `ui_icon/passion_minor`, `passion_major` | 2 |
| Skill icons | `ui_icon/skill_<key>` | 11 |
| Trait chip icon | `ui_icon/trait` | 1 |
| Drop pod | `structure/pod_shell`, `effect/pod_fall` (streak), `effect/pod_dust` | 3 |
| Cave entrance overlay | `rock/cave_mouth_<facing>` | 4 |
| Water terrain | `terrain/shallow_v0..3`, `deep_v0..3`, `mud_v0..3`, edge masks (16 each for mud-over-soil, shallow-over-deep, shore foam) | ≈60 |
| Track terrain | `terrain/track_v0..3` | 4 |
| Items | `item/components_s/m/l`, `item/industrial_medicine_s/m/l` | 6 |
| Ore overlay | `rock/ore_machinery_v0..3` | 4 |
| Buildings | `furniture/tool_cabinet`, `furniture/hospital_bed_<facing>` | 5 |
| Icons for new items/buildings/tools | `ui_icon/components`, `industrial_medicine`, `tool_cabinet`, `hospital_bed`, `factions`, `minimap` | 6 |

- **Water shader:** new `ebiten/water.kage`, a UV ripple plus caustic glints on the presentation clock (`present`).
  - Falls back to 4 animated frames per water type.
  - Shore foam is drawn as an edge-mask overlay.
  - Reduced Motion freezes the ripple.
- **Globe:** no sprites, only vertex colours from `world.yaml`, plus the emblem icons for settlements.

### 7.4 Audio (procedural, `cmd/audiogen` recipes; no credits)
| Id | Plays when | Recipe |
|---|---|---|
| `ui.slider_tick` | A slider changes step | Short click |
| `ui.page` | Wizard Next / Back | Soft swish |
| `sfx.pod_fall` | Pod descent | Rising whistle + rumble |
| `sfx.pod_land` | Pod impact | Thud + dust |
| `sfx.pod_open` | Pod opens | Hiss + clank |
| `amb.water_river` | River on screen | Babbling loop, pinkish noise with slow band-pass sweeps |
| `amb.water_shore` | Shore on screen | Wave wash loop |
| `music.setup` | Wizard screens | Reuses the menu theme |

Water ambience uses the existing ambience director: on-screen water fraction → volume. `make audio-check` must pass with the budget.

---

## 8. Execution (`.cursor/PARALLEL_WORK.md`, skill `parallel-work`)

### 8.1 Phase 0: Lead contract (one commit series, before any agent)
1. **Dependencies:** `go get github.com/go-text/typesetting@latest` (pulled by text/v2) and `go mod tidy`. Add the font files plus `OFL.txt` under `ebiten/fontdata/`.
2. **Enums, append-only:**
   - terrain `ShallowWater`, `DeepWater`, `Mud`, `Track`
   - items `Components`, `IndustrialMedicine`
   - ore `Machinery`
   - research `Fabrication`, `Pharmacology`
   - construction `ToolCabinet`, `HospitalBed`, `PodShell`
   - job kinds (none new: arrival uses existing moves)
   - mood kinds `Landed`, `LovesWork`, `Difficulty`
   - commands `SetStoryteller`, `SetDifficulty`
   - skill/stat/trait/backstory IDs
   - scenario `PartKind`
   - `difficulty.Preset`, `SaveMode`, `storyteller.Mode`
   - faction `Type`
   - `site.Hills`
   - screen id `screenScenarioEditor`
3. **Struct stubs and signatures:**
   - `site.Site`, `planet.Params`, `planet.Builder` / `Planet` (methods returning zero values)
   - `scenario.Def` / `Part` / `Effects`
   - `storyteller.Profile` / `State` / `Decision`
   - `difficulty.Settings` / `Adapter`
   - `faction.Faction` / `Def` / `Slot`
   - `pawngen.Pawn`, `look.Look`, `skill.Set`, `stat.Sheet`
   - `game.Setup`, `Config.Setup`, `NewColonyBuilder`
   - read models `PawnView` additions, `BioView`, `CandidateView`, `CandidatePool`, `TeamReport`, `FactionView`, `StoryView`, `ScenarioView`, `MapSizeView`, `ChunkView`, versioned layer fields
   - `Simulation` interface additions
   - `SaveHooks.SaveCommit`
   - `AppConfig.Scenarios`
   - `config.Settings` new fields
4. **Specs:**
   - `MAIN_UI_SCREENS_SPECIFICATION.md` §2–§8 rewritten to match §2 of this plan (layouts, ids, states), plus §39–§41 for commitment.
   - ART spec: §10 (builds), §17 (portraits, setup chrome, fonts), §18 (factions, emblems), new §8.x water, new §11.x globe.
   - `MASTER_SPEC.md` notes for storyteller, difficulty and factions.
   - Palettes: `ui.yaml` additions and the new `world.yaml`.
   - `TestUIPaletteMatchesYAML` extended.
5. **Measurement:** `statehash` + `TestScaleGolden` pins; baseline benchmarks recorded (`BenchmarkColonyHour`, `ColonyRaid`, `DrawArt`, wildlife).
6. **Map-size constant:** `game.MaxEnabledSide = 120` until §6.9 gates pass.
7. **Commit:** `chore(contracts): new-game flow, planet, pawns, factions, water, scale`.

### 8.2 Stages and agents (≤ 5 at once; disjoint ownership)
| Stage | Agent | Model (reason) | Owns (may edit) | Must not touch |
|---|---|---|---|---|
| **S1** | **ENG-A** E0+E1 | opus (perf, determinism) | `grid`, per-cell indexes in `game/{orders,tick,placement,world,rooms}.go` (index code only), removals, caches in `weather.go`, `defense.go`, `raid_plans.go` (cache only), read-model versioning in `game.go` (view fill only), `application/scenario` fixture, `domain/sim/statehash`, `cmd/benchmark`, bench files | domain packages owned by others; UI |
| | **PLANET** | opus (geometry, determinism) | `domain/world/planet/**`, `domain/world/site/**`, `pkg/noise3/**`, `temperature.Climate.SeasonAt` | game, UI |
| | **RULES** | sonnet (tables + validation) | `domain/scenario/**`, `domain/storyteller/**`, `domain/difficulty/**`, `domain/faction/**` (incl. names) | game, UI |
| | **PAWN-D** | opus (math + determinism) + a sonnet sub-pass for text/name pools | `domain/pawn/{skill,stat,trait,backstory,look,pawngen}/**`, `cmd/contentlint` additions | game, UI |
| | **UIKIT** | sonnet | `ebiten/{font,ui_kit,ui_layout,ui_slider,ui_check,ui_textfield,ui_scroll,ui_tooltip,ui_modal,ui_table,ui_tabs,ui_progress,ui_spark}.go`, `widgets.go`, `ui_text.go`, `input_frame.go` (keyRepeat + wheel routing only) | screens, renderer |
| **S2** | **ENG-B** E2+E3 | opus | `domain/world/{chunk,region}/**`, `domain/ai/pathing/**`, reach/room call sites in `game/{orders,world,rooms,fences,raids,hive}.go` | §5 integration files |
| | **SITEGEN** | opus | `domain/world/generation/**`, `domain/world/terrain/**`, `domain/world/geology/**`, `domain/economy/{item,recipe}/**`, `domain/research/**`, `domain/building/construction/**`, `domain/pawn/infection/**` | game |
| | **GLOBE** | sonnet | `ebiten/{globe,setup_backdrop}.go`, `ebiten/globemath/**` | screens |
| | **SCREENS-A** | sonnet | `ebiten/{screen_menu,screen_scenario,screen_scenario_editor,screen_storyteller,screen_difficulty,newgame_state}.go`, `adapters/outbound/scenariofile/**` | renderer, game |
| | **ART** | sonnet | `assets/source/blender/artlib/families/{pawn,portrait,terrain,item,furniture,overlay,rock,structure}.py`, `cmd/atlasgen/**`, generated outputs | Go outside `cmd/atlasgen` |
| **S3** | **SETUP** | opus | `game/{setup,factions,raids,raid_plans,incidents,weather,climate}.go` (threat and difficulty parts), difficulty hook sites, `game/scenarios.go` (read model), `game/mapsizes.go` | pawn files |
| | **PAWN-G** | opus | `game/skills.go`, `do*` call sites (one-line changes in their files, coordinated through the lead), `game/{mental,idle,needs,heat,combat,medical,orders,priorities}.go` (trait/incapable parts), `game/candidates.go`, `game/bio_view.go` | threat files |
| | **ENG-C** E4 | opus | plant staggering in `world.go`, climate buckets, job-giver indexes in `tick.go`, `orders.go`, `zones.go`, `animal_index.go`, `population.go` | — |
| | **SCREENS-B** | sonnet | `ebiten/{screen_worldgen,screen_landing,screen_factions,screen_pawns,portrait,screen_loading}.go`, Bio tab (`bio_ui.go`), Work tab cells (`work.go`), commitment UI (`saves.go`, `screen_pause.go`), minimap UI (`minimap.go`) | draw files |
| | **WATER** | sonnet | `game/` water and item rules (`placement.go` reasons, `filth.go` wetness, `haul_route.go` components, `production.go` cabinet, `medical.go` tiers, spawn filters), `ebiten/{water_draw,water.kage}`, arrival effects draw (`arrival_draw.go`) | §5.4 files |
| **S4** | **ENG-D** E5 | opus | `ebiten/{draw_art,shadow,environment,grass_draw,light,chunk_cache}.go`, `sound_signals.go` loops, `ChunkLayer`/`PlantsIn` read models | screens |
| | **FONT-1 / FONT-2** | sonnet ×2 | Font sweep split alphabetically by file (a–i / j–z) over files not owned above | owned files |
| | **SAVE** | opus | `game/save.go` v2, migration, `adapters/outbound/filesave/**` (atomic, commit), E6 budgets | — |
| | **MEDIA** | haiku | captures for every `-capture-ui` state, README media | code |
| **S5** | Lead | — | Gates, checklists, handoffs, README, notices, raising `MaxEnabledSide` after §6.9 | — |

**Ownership details:**
- Lead-only files: `go.mod`, `go.sum`, `Makefile`, `cmd/game/**` (wiring, flags, capture help), palettes, `contracts.py`/`stage.py`, `.cursor/**`, `README.md`, `THIRD_PARTY_NOTICES.md`, `game/game.go` (Config/Setup/view struct fields), `ebiten/app.go` (dispatch prefixes, `NewGame`), `ebiten/nav.go`, `ebiten/capture.go` (capture states, added by the lead from agent requests).
- **Shared-file protocol:** where two agents need one file (for example `do*` call sites touched by both PAWN-G and SETUP yield hooks), the lead assigns line ranges in the brief. One agent lands first, then the lead relays "rebased; your lines are X–Y". Edits to another agent's file are never allowed. If ownership can't be made disjoint, that agent works in a git worktree (`isolation: "worktree"`) and the lead merges.

### 8.3 Gates (lead, after every stage)
1. `go build ./...`
2. `gofmt -l` (empty)
3. `go vet ./...`
4. `staticcheck ./...`: no new findings
5. `go test -race -count=1 ./...` (exit code checked; never piped into `tail`)
6. `go test -count=3 -run 'Determin|SameSeed|Golden|Scale' ./internal/...`
7. `make art-check` and `make audio-check`
8. Benchmarks vs §6.1 and baselines
9. Captures reviewed by eye
10. Commits on `main` per area, messages per GIT rules, never pushed

### 8.4 Dependency order
```
P0 contract
  → S1 (ENG-A, PLANET, RULES, PAWN-D, UIKIT)            [parallel]
  → S2 (ENG-B, SITEGEN, GLOBE, SCREENS-A, ART)          [GLOBE/SCREENS-A may start in S1 against stubs]
  → S3 (SETUP, PAWN-G, ENG-C, SCREENS-B, WATER)          [needs S1 domain + SITEGEN + UIKIT]
  → S4 (ENG-D, FONT-1, FONT-2, SAVE, MEDIA)              [font sweep last to avoid conflicts]
  → S5 lead gates → raise MaxEnabledSide 300 → (500 after manual run) → docs
```
The game stays playable after every stage (legacy path untouched until S3 wires Setup).

---

## 9. Verification

### 9.1 Test index (new; all must pass with `-race`)
- **Domain:**
  - Planet: `TestTileCountFormula`, `TestPentagonsAre12`, `TestNeighboursSymmetric`, `TestCoverageFraction`, `TestLandShareInRange`, `TestBiomeClassificationTable`, `TestRiversFlowDownhillToWater`, `TestSettlementSpacing`, `TestRoadsConnectSettlements`, `TestPlanetGoldenHash`, `TestStepBudgetDeterministic`
  - Site generation: `TestZeroSiteMatchesLegacy`, `TestRiverEdgeToEdgeAllSizes`, `TestNoWaterAtStart`, `TestReachableSoftGround70`, `TestCavesOnlyInRock`, `TestRockShareRisesWithHills`, `TestCoastDepthBands`, `TestLakeRings`, `TestFordsEveryInterval`
  - Scenario: `TestBuiltinsValid`, `TestValidateCases`, `TestCodecRoundTrip`, `TestDecodeUnknownPart`, `TestApplyFlattens`
  - Storyteller and difficulty: `TestLegacyPlanMatchesConstants`, `TestCycleRestHasNoBigThreats`, `TestCalmGapsAndLossEase`, `TestDiceProbabilityRisesWhenQuiet`, `TestPresetValues`, `TestThreatPointsMonotonicInWealth`, `TestAdaptationNoFlap`
  - Factions: `TestFactionHostilityHysteresis`, `TestFactionNamesUniqueAndOriginal`, `TestPlaceSettlementsRules`, `TestPickRaiderHostileOnly`
  - Pawns: `TestGenerateDeterministic`, `TestDistributions`, `TestIncapableSkillsZeroNoPassion`, `TestTraitConflictsNever`, `TestBackstoryFitsTech`, `TestLegacyLookEqualsRenderer`, `TestSkillXPAndDecayNoFlap`, `TestStatSheetOrder`
  - Items: `TestMachineryOreYieldsComponents`, `TestFabricateNeedsSkillAndResearch`
  - Seasons: `TestSeasonAtSouthernShift`
- **Application:**
  - Setup and arrival: `TestSetupNilIsLegacy` (golden hash equality), `TestSetupBuildsChosenMap` (size, biome, climate, season, stones), `TestPodArrivalPlacesEveryoneOnLand`, `TestStandingArrivalRing`, `TestWalkInFromEdge`, `TestIntroLetterOnce`, `TestScenarioKitSpawned`, `TestResearchGrantsAndCostPct`, `TestDisallowedBuildingRefused`, `TestScheduledAndDisabledIncidents`, `TestPermanentConditions`
  - Threats and difficulty: `TestStorytellerPacingInGame`, `TestRaidsAttributedToFaction`, `TestNoHostileFactionConvertsRaid`, `TestThreatPointsUsesDifficulty`, `TestPeacefulDaysRespected`, `TestDifficultyHooks` (table over each §5.3 row), `TestExtremeWeatherEvents`, `TestSetStorytellerCommand`
  - Pawns in play: `TestNeutralPawnTimingsUnchanged`, `TestSkillSpeedRatio`, `TestLearningSaturation`, `TestDecayNoFlapInPlay`, `TestIncapablePriorityIgnoredAndOrderRefused`, `TestViolenceIncapableNeverStrikes`, `TestAccuracyRisesWithSkill`, `TestTraitMoodAndBreakOffsets`, `TestBirthdayRebuildsStats`, `TestCandidatePoolOps`, `TestTeamWarnings`
  - Water and items: `TestWaterPlacementRefused`, `TestPathAvoidsShallow`, `TestMudDirtGain`, `TestWadingSoakedAndWashes`, `TestNothingSpawnsInWater`, `TestComponentFramesWait`, `TestRefineMedicineDoctorBill`, `TestIndustrialBeatsHerbal`, `TestMedicineChoiceNoFlap`, `TestToolCabinetBonusCap`, `TestHospitalBedHeal`, `TestPodShellDeconstructsToSteel`
  - Saves: `TestLoadV1MigratesBios`, `TestSaveV2RoundTripSetup`, `TestWorldRegeneratedOnLoadMatches`, `TestClimateNotRederivedForSetup`, `TestSaveInfoFields`, `TestFilesaveAtomic`, `TestCommitmentSingleSlotNoAutosaveCopy`
- **Engine:** `TestScaleGolden`, `FuzzRegionReachability`, `TestRegionReachMatchesAStar`, `TestRoomsIncrementalMatchesDetect`, `TestCorridorPathValid`, `TestLongSearchBudgetDeterministic`, `TestWorldViewCachedEqualsFresh`, `TestGrowSpanTelescopes`, `TestWildlifeCapCurve`
- **UI:**
  - Widgets (§1.4): `TestSliderDragAndKeys`, `TestTextFieldEditing`, `TestScrollViewWheelAndThumb`, `TestTooltipDelayAndClamp`, `TestModalEscClosesTopOnly`, `TestTableSort`, `TestTabFocusOrder`, `TestWheelRoutingMenuVsCamera`
  - Every screen test listed in §2.1–§2.13
  - `TestSetupMinWindow`, `TestUIPaletteMatchesYAML`, `TestAtlasCoversKeys`, `TestGeneratedArtCoversRenderer` (extended), `TestScriptedWizardBuildsSetup` (clicks through every screen, then asserts the resulting `Setup`)
- **Existing tests to update deliberately** (not weaken):
  - `TestNewGameWizardAndStart`, `TestBackAndEscapeUnwind`, `TestScenarioSelectionStaysOnScreen`, `TestBodyOriginClearsLeftRail` (rail widths change), `TestWorldGenerateStaysInFooter`, `TestPickerListsEveryBiomeAndStartPassesTheChoice` (the biome picker is replaced by the globe; its assertion moves to "Start passes the chosen tile's biome"), `TestWorldStepSeedFieldAndDice` (free-text seed).
  - The four tests clicking newgame + 4×next + start (`app_test.go:181/217/249`, `saves_test.go:7`) gain a test helper `quickStart(a)` that drives the new flow with defaults, including a tiny 5% world.

### 9.2 Benchmarks (pass/fail against §6.1)
| Benchmark | Budget |
|---|---|
| `TickGrown/250` | avg ≤ 1.0 ms |
| `TickGrown/500` | avg ≤ 1.5 ms, p99 ≤ 4 ms, max-3 ≤ 7 ms, ≤ 2 KB/tick |
| `TickRaid/500` | p99 ≤ 5 ms |
| `ReadModel/500` | ≤ 0.5 ms, ≤ 64 KB |
| `DrawArt500/*` | ≤ 6 ms CPU, ≤ 6k draws |
| `Reachable500` | ≤ 1 µs |
| `PathUnreachable500` | ≤ 5 µs |
| `Path500Long` | ≤ 1.5 ms |
| `RoomsAfterWall500` | ≤ 0.2 ms |
| `GenerateSite500` | ≤ 2 s |
| `Save500` / `Load500` | ≤ 300 ms / ≤ 1.5 s |
| `PlanetBuild100` | ≤ 1.5 s |
| `GlobeFrame` | ≤ 2 ms |
| `HUDText` | ≤ 0.6 ms |
| `ColonyHour` (48×40) | ≤ +10% vs 17.5 ms |
| Existing draw, shadow, stain and dirt benches | within +10% |

### 9.3 End-to-end manual script (lead; results in the handoff)
1. `go run ./cmd/game`, then New Colony. Check that every scenario shows correct details. Run the Editor: add each part kind, see validation, save, reload, delete.
2. Storyteller: pick each storyteller, check the sparkline. Each preset. Custom: edit and cancel, edit and accept. Commitment on.
3. Create World: typed seed "amber-hollow", 30%, generate; Back; change temperature, generate again (stale handling); 100% coverage timing.
4. Landing: rotate, zoom, hover; select an ocean tile (Next disabled with its reason), a settlement (reason), then a temperate river tile. Advanced: season Summer, 250×250. Open the Factions modal and click a row (globe centres). Select random site ×3.
5. Characters: reroll, rename, reorder, swap, team warnings; Start.
6. Loading progress, then pods land, then the intro letter. Play 2 in-game days at 3× with the FPS overlay. Wade a river (Soaked); build a carpet and a tool cabinet; check the work-tab incapable cells and the Bio tab.
7. Wait for the first raid: the letter names the faction, and the Factions window shows goodwill.
8. Commitment: Pause shows "Save & Quit" and a disabled Load; quit, Continue resumes.
9. Repeat at 500×500 (after the gate) for 10 minutes at 3× and record FPS. Repeat with Ember Clan (standing, tribal kit) on a mountain tile with caves, and Bare Hands on a coast.

### 9.4 Captures (MEDIA agent, reviewed by the lead)
- Every `-capture-ui` state from §1.7.
- `-scenario` captures still work.
- README gallery adds: setup flow strip, globe, characters, pods landing, river map, minimap at 500².

---

## 10. Documentation, checklists, handoffs

**Handoffs** (lead, from agent reports):
- `UI-020-new-game-flow.md`: screens, widgets, font, commitment
- `ENG-001-scale-500.md`: budgets, measured numbers, gates, golden re-pins with reasons
- `SIM-030-planet-factions.md`
- `SIM-031-storyteller-difficulty.md`
- `SIM-032-pawn-identity-skills.md`
- `SIM-033-water-sites.md`
- `SIM-034-components-medicine.md`
- `ART-014-pawn-builds-portraits-water.md`

**Checklists.** Re-grep these before editing, because the line numbers move:
- **UI/UX checklist:**
  - §26.1 world setup (≈L1362–1369): seed `[x]`, size/temp/rain `[x]`, progress `[x]`.
  - §26.2 landing (≈L1371–1386): select tile `[x]`, tile card `[x]`, search `[ ]` (not in scope, add note), settle `[x]`, back/regenerate `[x]`.
  - §27.1 main menu (≈L1392–1400).
  - §27.2 scenario (≈L1404–1416).
  - §27.3 scenario editor (≈L1420–1425).
  - §27.4 storyteller (≈L1429–1436).
  - §27.5 difficulty (≈L1438–1449).
  - §27.6 pawns (≈L1451–1471).
  - §33.2 save dialog incl. "Commitment/ironman restrictions" (≈L1614–1622).
  - Milestone F (≈L2122–2124).
  - Also fix the stale notes found: Options and Credits "stub" in §27.1.
- **Vanilla checklist:**
  - §3.1 pawn identity (≈L218), procedural pawn generation (≈L225), §3.2 backstories (≈L233), §3.3–3.5 skills, traits, passions.
  - Phase 20 factions.
  - Phase 22 planet (≈L1716–1803), incl. water terrain (§22.4).
  - Phase 23 scenario, storyteller, difficulty (≈L1806–1868).
  - Phase 33 saves (≈L2402–2421), incl. "Ironman/commitment-style save mode".
  - Construction skill items.
  - Medicine tiers.
- **Screens spec:**
  - §2–§8 boxes (≈L96–L300).
  - §56 UI-6 wave (≈L1893–1900), which still says "Generate disabled": fix.
  - §57 navigation tree.
- **ART spec:** Wave 8 world items (≈L1063–1068), §10, §17, §18, plus the water and globe sections added in Phase 0.
- **Rule for every item:** `[x]` only with full acceptance; `[-]` with what's missing; `[!]` blocked with why.

**Other docs:**
- README feature lines: new-game flow, globe, storytellers and difficulty, factions, colonist identity and skills, water, components and medicine, big maps; plus media.
- `THIRD_PARTY_NOTICES.md`: Noto Sans OFL.
- `ebiten/AGENTS.md`: remove the "new-game stubs" and "Ada/Bo/Cora kit" claims; document the widget kit and font API.
- `game/AGENTS.md` (or nearest): document Setup, the neutral-pawn rule and the region system.
- **Memory:** nothing new for the user profile.

---

## 11. Risks and mitigations
| Risk | Mitigation |
|---|---|
| 60 fps at 500² is the long pole; budgets may not be met by E1–E5 | Measure-first; per-system budgets; `MaxEnabledSide` stays lower until the gates pass; report honestly. Fallback options (user decision at that time): lower 3×-speed tick budget or drop the largest sizes |
| Golden re-pins (plant staggering, corridor paths, `workProgress`) hide real regressions | Each re-pin recorded with a reason; `TestNeutralPawnTimingsUnchanged`; legacy path stays bit-identical |
| Region code misses a walkability source (fences, gates, traps, water, frames, doors) | Fuzz test against A* over all layers, with every edit kind |
| Font sweep touches ~40 files and conflicts with other agents | Runs last (S4), split by file groups; `drawLabel` shim lands first in S1 so nothing breaks meanwhile |
| Pawn art volume (~12.6k sprites, ~50 min render, disk) | Parametrised rig, atlas packing, private TMPDIRs, render per build in parallel across art workers |
| Old saves and looks change | `look.Legacy` + `pawngen.Legacy` golden tests; v1 fixture |
| Originality: names, text and portraits drifting toward RimWorld | Contentlint blocklist; ART briefs; lead review of every text and portrait |
| Commitment save corruption on crash | Atomic temp + fsync + rename; save on close |
| Text input conflicts with the shortcut system | `CtxText` already suppresses shortcuts; tests for Esc/Enter in fields |
| Scope size | Stages land independently and keep the game playable; each stage commits after gates |

## 12. Critical files (most-touched)
- `internal/application/game/game.go`, `setup.go` (new), `raids.go`, `raid_plans.go`, `skills.go` (new), `save.go`, `rooms.go`, `world.go`, `tick.go`
- `internal/domain/world/planet/` (new), `site/` (new), `generation/`, `terrain/`, `region/`, `chunk/`
- `internal/domain/pawn/{skill,stat,trait,backstory,look,pawngen}/`, `internal/domain/{scenario,storyteller,difficulty,faction}/`
- `internal/adapters/inbound/ebiten/{app.go,nav.go,font.go,ui_*.go,screen_*.go,globe.go,draw_art.go,work.go,saves.go}`
- `assets/source/blender/artlib/families/{pawn,portrait,terrain}.py`, `cmd/atlasgen/`, `cmd/benchmark/`

