# rimworld-go --- Main UI Screens Specification

> **Purpose:** Canonical build specification for every major
> player-facing screen in `rimworld-go`.
>
> **Companion documents:**
> `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`,
> `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`,
> `HEXAGONAL_ARCHITECTURE.md`, and
> `ART_DIRECTION_AND_ASSET_PIPELINE.md`.
>
> **Architecture:** UI is an inbound adapter. Screens consume read-only
> presentation models and emit typed commands. No screen may directly
> mutate authoritative simulation state.
>
> **Design target:** Deliver the information density, discoverability,
> and management capabilities expected from a deep vanilla colony
> simulator while using original layouts, artwork, terminology where
> appropriate, and visual identity.
>
> **Agent loading:** This file is *not* default context. Route via
> `.cursor/skills/rimworld-go-router/SKILL.md` and follow
> `.cursor/skills/rimworld-go-ui-screens/SKILL.md`. Load §0, §58, §59, and
> only the named screen section for the task. Tick Wave UI / architecture
> / DoD checkboxes in this file using `[x]` / `[-]` / `[!]`.

------------------------------------------------------------------------

# 0. Screen Architecture

Every screen specification uses:

-   **Purpose** --- why the screen exists.
-   **Entry** --- how the player reaches it.
-   **Primary regions** --- major layout areas.
-   **Information** --- data the player needs.
-   **Actions** --- interactions available.
-   **Commands** --- authoritative requests sent to the application.
-   **States** --- empty/error/disabled/special states.
-   **Dependencies** --- systems required before implementation.
-   **Scale requirements** --- behavior for large colonies.

Global rules:

-   [-] Escape closes the topmost modal/panel before opening the game
    menu. (Architect, then Work, then inspector selection, then pause; no generic window manager)
-   [ ] Screens preserve useful scroll/filter/sort state.
-   [ ] Entity references can select/jump to the entity when meaningful.
-   [-] Disabled actions explain why. (footer/hover reason string; no rich tooltips)
-   [ ] Tables with potentially hundreds of rows are virtualized.
-   [ ] Search/filtering happens against presentation data, not world
    scans each frame.
-   [ ] Screen animations never affect simulation.
-   [ ] Plugin-provided content appears through registries rather than
    hardcoded switches.

------------------------------------------------------------------------

# 1. Boot / Loading Screen

## Purpose

Provide immediate feedback while configuration, plugins, definitions,
assets, saves, and renderer resources initialize.

## Primary regions

-   game logo/branding;
-   build/version;
-   loading phase;
-   progress indicator;
-   current operation;
-   recoverable warnings.

## Loading phases

-   configuration;
-   plugin discovery;
-   dependency resolution;
-   content definitions;
-   asset manifests/atlases;
-   localization;
-   renderer;
-   main-menu transition.

## Required states

-   [-] normal loading; (phase list + pack load; no plugin/fatal recovery)
-   [-] content warnings; (pack-load warning on boot/menu only)
-   [ ] plugin dependency failure;
-   [ ] fatal initialization error;
-   [ ] safe/recovery mode where supported.

------------------------------------------------------------------------

> **NG-1 (normative):** §2–§8 below are implemented per `NG1_NEW_GAME_PLAN.md` §1 (shared visual language, font, widget kit, navigation, wizard draft) and §2.1–§2.13 (every screen: layout, control ids, validation, states, tests, captures). Where this spec and the plan differ, the plan wins for these screens. Original names only: storytellers Orla Vane / Bram Holloway / Kestrel Quay; difficulties Wanderer…Merciless; scenarios Broken Hull, Ember Clan, Gilded Expedition, Bare Hands.

# 2. Main Menu

## Purpose

Primary entry point for a play session.

## Primary actions

-   [x] New Game (opens Choose Scenario; the wizard keeps its draft until the menu or Start)
-   [x] Load Game (save list: slot, game date, colonists, file time; commitment saves tagged)
-   [x] Continue (loads the most recent save, commitment ones included; disabled with "no saves yet" when there are none)
-   [-] Plugins (visible, disabled)
-   [x] Options (Audio/Graphics/Gameplay/Accessibility/Controls tabs, saved to settings.json; a Story tab appears during play)
-   [-] Tutorial/Help (visible, disabled)
-   [x] Credits (creator, originality notice, asset credits)
-   [x] Quit
-   [x] Low-memory notice (PLAY-019: a warning card under the rail when
    the computer has less RAM than the game needs to run smoothly; warns
    only, "Got it" hides it until the amount of RAM changes)

## Information

Show game version/build and, in development builds, commit identifier.

`Continue` should identify the most recent compatible save without
loading it blindly.

## Background

-   [-] original colonist bust splash on the left over the existing menu
    backdrop (bust flush to PNG bottom edge); title and actions sit on a
    right rail. No live simulation.
-   [x] generated colony diorama behind the bust (ART-011): a dusk colony
    rendered in Blender from the sprite builders (`make art-menu`), drawn
    with a slow pan/zoom, lamp and fire glow, the campfire flame, embers
    and dust, and a scrim behind the title, rail and bust. Sub-menus show
    it dimmed. The code-drawn gradient remains the fallback when the
    files are missing. PLAY-020: the house counts as roofed, so its room
    is lit by its lamp (a point light, furniture shaded toward it, soft
    shadows away from it), not by the sun.

Menu presentation must not require an active authoritative simulation.

------------------------------------------------------------------------

# 3. New Game --- Scenario Selection

## Purpose

Choose the fundamental starting situation.

## Layout

**Left:** scenario list/search.\
**Center:** selected scenario description.\
**Right:** starting conditions summary.\
**Bottom:** Back / Next.

## Information

For each scenario: - starting pawn count; - starting technology; -
starting resources/equipment; - special conditions; - difficulty
character; - original narrative description.

## Actions

-   [x] choose scenario; (Broken Hull, Ember Clan, Gilded Expedition, Bare Hands; searchable list with hardness pips)
-   [x] inspect details; (people, technology, kit, research, special rules, original narrative)
-   [x] create/edit custom scenario; (New and Edit a copy open the Scenario Editor)
-   [x] load custom scenario; (custom JSON files are listed beside the built-ins, with delete)
-   [x] continue. (Next stamps a copy of the scenario into the draft)

Plugins may register scenarios.

------------------------------------------------------------------------

# 4. Scenario Editor

## Purpose

Create data-driven custom starting rules without modifying code.

## Regions

-   scenario metadata;
-   ordered scenario-part list;
-   add-part browser;
-   selected-part editor;
-   validation panel.

## Actions

-   add/remove/reorder parts;
-   configure starting pawns/items/research;
-   configure recurring/permanent conditions where supported;
-   save/load;
-   reset;
-   validate.

Invalid combinations must identify the offending part rather than
failing when the game starts.

Status (NG-1):

-   [x] scenario metadata; (name, summary, description, mandatory setup)
-   [x] ordered scenario-part list; (14 part kinds, move up/down/remove)
-   [x] add-part browser; (modal picker)
-   [x] selected-part editor; (per-part fields and a live preview)
-   [x] validation panel; (names the offending part; errors block Done, warnings do not)
-   [x] save/load/reset. (versioned JSON files, atomic writes; Reset restores the opening state)
-   [ ] plugin-registered parts.

------------------------------------------------------------------------

# 5. Storyteller & Difficulty Screen

## Purpose

Choose incident pacing and challenge.

## Storyteller region

Original storyteller cards representing: - paced escalating; -
relaxed/forgiving; - highly random.

Each explains behavior qualitatively without exposing formulas
unnecessarily.

## Difficulty region

-   preset list;
-   custom difficulty.

Custom settings can expose threat scale, economy, mood, disease,
adaptation, resource yield and other supported parameters.

## Actions

-   [x] select storyteller; (portrait cards for Orla Vane, Bram Holloway, Kestrel Quay with bios and an example pacing sparkline)
-   [x] select preset; (Wanderer, Gentle, Settled, Seasoned, Grim, Merciless)
-   [-] customize; (Custom Difficulty dialog with 19 settings; disease frequency has no effect yet, a few sliders lack tooltips)
-   [x] reset; (copy from a preset, reset, cancel)
-   [x] continue. (Next; the save mode, Reload anytime or Commitment, is chosen on the same screen)

------------------------------------------------------------------------

# 6. World Generation Screen

## Purpose

Configure and generate the campaign world.

## Controls

-   seed;
-   world coverage/size;
-   temperature;
-   rainfall;
-   faction/population density where supported;
-   Generate.

## During generation

Show phase/progress while keeping UI responsive.

## Result

Transition into landing-site selection without discarding generated
world state.

Status (NG-1):

-   [x] seed; (free text, original two-word phrases; random button)
-   [x] world coverage/size; (30, 50 or 100 % of the globe)
-   [x] temperature; (five stops)
-   [x] rainfall; (five stops)
-   [x] faction/population density; (population stops and a faction table with counts)
-   [x] Generate. (progress dialog with phases; unchanged settings reuse the world; Next goes to the landing screen)

------------------------------------------------------------------------

# 7. Landing Site Selection / World Preview

## Purpose

Allow informed choice of colony location.

## Main region

Interactive planet/world map.

## Selected-tile panel

Show: - biome; - terrain/hilliness; - elevation; - temperature; -
rainfall; - growing period; - movement difficulty; - roads/rivers; -
forage; - useful known resources; - nearby settlements/factions.

## Actions

-   inspect tiles;
-   search/find features where supported;
-   choose tile;
-   regenerate/back;
-   continue.

The player should be able to understand why one tile is easier or harder
than another.

Status (NG-1):

-   [x] selected-tile panel; (terrain, elevation, temperatures, rainfall, growing period, stones, water, caves, roads, forage, wildlife, nearest settlement, movement difficulty, warnings)
-   [x] interactive planet/world map; (rotating hex globe with biome, temperature, rainfall and elevation views, rivers, roads, faction-tinted settlements)
-   [-] inspect tiles / search features; (inspect and hover yes; no feature search, only Random site and the Factions dialog)
-   [x] choose tile; (invalid tiles give the reason on the disabled Next button)
-   [x] regenerate/back; (Back keeps the draft, Create World regenerates, Random site)
-   [x] continue. (Next to Create Characters)
-   [x] map size and starting season. (Advanced dialog; 200x200 to 500x500, 250 default; PLAY-019: a size whose estimated memory is over 70% of the computer's RAM says "Needs about N GB; this computer has M GB." in the picker and the site panel and stays selectable; with unknown hardware 350+ notes "Large map: needs a capable computer")

------------------------------------------------------------------------

# 8. Starting Pawn Selection

## Purpose

Choose the initial colonists and understand team strengths/weaknesses.

## Layout

**Top/left:** candidate roster.\
**Center:** selected pawn.\
**Right:** detailed bio/skills/health.\
**Bottom:** team warnings and Start.

## Pawn information

-   name;
-   age;
-   traits;
-   childhood/adulthood;
-   skills;
-   passions;
-   work incapabilities;
-   health;
-   relationships;
-   starting gear if applicable.

## Actions

-   select roster;
-   reroll/randomize candidates according to scenario rules;
-   inspect pawn;
-   rename where permitted;
-   start game.

## Team analysis

Warn about important capability gaps such as no doctor, cook, grower,
constructor, researcher, or violence-capable pawn.

Status (NG-1):

-   [x] candidate roster; (selected and left-behind lists, drag, swap, reorder)
-   [x] selected pawn and details; (portrait by build, age, backstories, traits, skills with passions, incapabilities, health conditions)
-   [x] reroll/randomize, rename (validated, unique nicknames), start;
-   [x] team analysis. (nobody can fight, no doctor, cook, grower, builder or researcher, everyone very young or old)
-   [ ] relationships; (no relationship system)
-   [ ] per-pawn starting gear. (the kit belongs to the scenario)

------------------------------------------------------------------------

# 9. Main Colony Screen

## Purpose

The central gameplay screen. Nearly all colony play begins here.

## Major layout regions

User decision (PLAY-018, 2026-10-25): the colony screen uses the
familiar colony-sim ARRANGEMENT so a returning player knows where to
look, sized on this project's own grid with original fonts, icons and
palette. No measurements or colours are taken from another game's code.
The map fills the whole screen; the HUD floats over it.

``` text
┌──────────────────────────────────────────────────────────────────┐
│ resources        [ colonist bar, top-centre ]     learning helper│
│ (icon list,                                        alerts         │
│  stored goods)          messages (right-aligned) ─┤ (severity     │
│                                                    │  ordered)    │
│                     WORLD VIEWPORT (full screen)   │               │
│                                                    │ letters      │
│ inspector tab panel                                │ minimap      │
│ (opens upward)                                     │ toggles      │
│ [tabs] ────────────┐ [command buttons, wrap upward]│ speed ▮▶▶▶  │
│ inspect pane        │ [■][■][■][■][■][■][■][■][■]  │ date/weather │
│ (name, bars, lines) │                              │ temperature  │
├────────────────────────────────────────────────────────────────┤
│ Architect│Work│Schedule│Assign│Animals│Wildlife│Research│World│…  │
└──────────────────────────────────────────────────────────────────┘
```

- **Grid.** One pure function places every region
  (`hud_layout.go` `computeHUDLayout`, logical px, multiples of 4,
  margin 8), so regions never overlap at any window size or UI scale.
- **Inspect pane.** It sits at the lower-left. Its tabs open a larger
  panel upward. Every action on the selection is a square command
  button to the pane's right, wrapping upward. A group merges its
  members' commands.
- **Architect.** Two category columns at the lower-left, a description
  box above them, and the tools as command buttons.
- **Main-tab windows.** They open bottom-left above the tab strip and
  never cover the colonist bar, alerts or time controls.
- **Nothing selected.** The hovered cell's info shows as text at the
  lower-left.
- **Palette.** An original slate charcoal with a sea-glass accent
  (`ui.yaml`). The learning helper uses a warm instructional set.

## Core responsibilities

-   map rendering;
-   camera;
-   selection;
-   time controls;
-   colonist bar;
-   resources;
-   messages;
-   alerts;
-   event letters;
-   contextual inspector;
-   contextual actions;
-   global management tabs;
-   Architect access;
-   overlays.

## Camera

-   pan;
-   zoom;
-   center selected;
-   jump from alert/event;
-   far-zoom LOD.

## Selection

-   single;
-   drag-select;
-   additive/subtractive;
-   multi-selection;
-   cycle stacked entities;
-   double-click same category where useful.

## Time

-   pause;
-   normal;
-   fast;
-   very fast (four clickable speed buttons, lower-right, with key hints);
-   current game time/date/season, weather, and the temperature where
    the cursor points (indoors/outdoors).

## Letters, alerts and messages (PLAY-018)

-   **Letters** stack on the right above the time controls, coloured
    by kind (threat, caution, info, good):
    -   click opens one;
    -   right-click dismisses it (saved);
    -   only the opening letter pops up by itself;
    -   major threats are letters too.
-   **Alerts** list ongoing problems under the learning helper:
    -   danger first;
    -   a hover card explains each one;
    -   click and right-click cycle through what it is about.
-   **Messages** are short notices under the colonist bar that fade.

## Learning helper (PLAY-018)

A collapsible top-right panel of short original lessons (camera,
selecting, building, zones, work, schedule, drafting, forbid and haul
marks, research, time speed). A lesson completes when the player does
it, or with "Got it". It can be switched off (Options › Gameplay or the
lower-right toggle) and its progress is saved.

This screen is implemented incrementally throughout the project rather
than in one task.

------------------------------------------------------------------------

# 10. Generic Selection Inspector

## Purpose

Answer: **"What is this, what state is it in, and what can I do with
it?"**

## Header

-   name;
-   icon/portrait;
-   category;
-   faction/ownership;
-   concise condition.

## Body

Varies by entity: - description; - material; - quality; - hit points; -
room; - temperature; - power; - fuel; - growth; - contents; - current
work; - etc.

## Tabs

Entity types can register applicable inspector tabs: - General; -
Character; - Needs; - Health; - Social; - Gear; - Prisoner; - Bills; -
Contents; - Power; - Stats.

## Action bar

Only valid contextual commands appear.

Disabled commands show reason.

**Implemented (UI-011):**
- **Selecting:** a map click selects, in order, a pawn, animal, ground item, building, plant, then floor. A second click on the same tile cycles through what's there. PLAY-020: the zone comes last; a click on an empty zone tile or on its label selects the zone.
- **Pawn panel:** an action bar (Draft, Clear orders), and "Equip nearby" on the Gear tab.
- **Plant panel:** growth stage, readiness, frost and zone, with Harvest, Cut/Chop and per-colonist "now" orders.
- **Item panel:** freshness, the body's name and storage, with per-colonist Equip, Wear, Eat and Haul.
- **Building panel:**
  - **Move:** a ghost placement mode that reinstalls the same building, keeping its bills, fuel and owners.
  - **Copy:** the Architect, preset to that building.
- **Action availability:** actions come from the same options as the right-click menu, and disabled ones show the simulation's reason.
- **Paused play:** every action applies at once while paused.

**Implemented (PLAY-018):**
- **Inspect pane** (lower-left, see §9):
  - a header (name, subtitle);
  - compact status bars;
  - chips (a colonist's allowed area, which opens the picker; the schedule activity);
  - a scrolling body.
- **Tabs:** they sit in a strip above the pane and open a larger panel upward; clicking the open tab collapses it, and Escape closes it first.
  - colonist: Needs, Health, Bio, Social, Gear, Log (Prisoner for prisoners);
  - animal: Log, Training;
  - building: Bills, Pen.
- **Command buttons:** the action bar became square command buttons right of the pane (icon, label, hotkey, disabled reason in the tooltip), wrapping upward.
  - Hotkeys follow declaration order; dedicated keys stay (R draft, F forbid).
  - Per-colonist choices are options: right-click or the corner arrow. These cover Equip/Wear/Eat/Haul on items, "Now by…" on plants, the trough filter and an animal's master.
  - A group selection merges its members' commands: a toggle shows mixed, and a command is disabled only when it is for every member.
- **Nothing selected:** the hovered cell's details show as text at the lower-left.

------------------------------------------------------------------------

# 11. Detailed Stats Screen

## Purpose

Explain simulation values instead of forcing players to guess.

## Features

-   searchable stat list;
-   categories;
-   base value;
-   modifiers;
-   final value;
-   units;
-   explanatory text.

## Breakdown

A calculated stat should show its derivation when useful.

Example:

``` text
Movement Speed
Base                       4.60
Health capacity           ×0.82
Apparel                   ×0.96
Environment               ×0.90
--------------------------------
Final                      3.26
```

This screen becomes a universal debugging/explainability tool for
players.

------------------------------------------------------------------------

# 12. Pawn Character / Bio Screen

## Purpose

Explain who a pawn is and what they are capable of.

## Sections

-   identity;
-   age;
-   faction/origin;
-   backstories;
-   traits;
-   skills;
-   passions;
-   incapabilities;
-   relationships summary.

## Skills

Show level, XP progress, passion and learning rate.

Click/hover explains gameplay effects.

------------------------------------------------------------------------

# 13. Pawn Needs & Mood Screen

## Purpose

Explain current psychological state and likely mental-break risk.

## Need bars

-   food;
-   rest;
-   recreation;
-   comfort;
-   beauty;
-   outdoors;
-   chemical/addiction needs when present.

Show value, thresholds and trend.

## Mood

-   current mood;
-   break thresholds;
-   trend;
-   risk severity.

## Thoughts

Separate positive/negative effects and show: - description; - mood
contribution; - duration; - stacking; - cause where appropriate.

## Special states

Show active mental state or inspiration prominently.

------------------------------------------------------------------------

# 14. Pawn Health Screen

## Purpose

Make anatomy, injury, illness and treatment understandable.

## Overview

-   pain;
-   consciousness;
-   moving;
-   manipulation;
-   sight/hearing/etc.;
-   blood loss;
-   bleeding;
-   downed state.

## Body tree

Hierarchical anatomy with injuries/effects attached to relevant body
parts.

## Conditions

Show: - severity; - stage; - progression; - immunity; - tend quality; -
duration where knowable.

## Medical controls

-   maximum medicine;
-   self-tend;
-   operations.

## Operations

Add/cancel/reorder valid procedures and show required materials/medicine
and relevant risks.

## ENG-003 contract (inspector Health tab)

-   **Entry:** the pawn inspector's Health tab (live since ENG-003).
-   **Regions:** capacities with percent; pain; body-part rows (hurt,
    missing, scarred, artificial); queued operations; drug lines
    (highs, tolerance, addictions).
-   **Information:** `Simulation.Health(id)` → `game.HealthView`
    (`Capacities`, `Parts`, `Pain`, `Operations`, `Drugs`). Read-only.
-   **Actions / Commands:** add operation (`command.AddOperation`:
    `Pawn`, `Index`=body.OpKind, `Build`=part, `Stuff`=implant item),
    cancel operation (`command.RemoveOperation`), drug policy
    (`command.SetDrugPolicy`, `Mode`=drug.Preset). Not wired yet (M1, M2).
-   **States:** `ok == false` shows "No data yet." in dim text; a dead
    pawn shows the last state; operation buttons explain why disabled
    (no doctor, no medicine, no implant item).
-   [-] Live (SIM-039/040): capacities, pain, part rows, operations queue
    with add/remove, drug lines; missing: bleeding, conditions list,
    medical controls, operation reorder/risk preview.

------------------------------------------------------------------------

# 15. Pawn Social Screen

## Purpose

Explain relationships and social opinion.

## Information

-   family;
-   partner/ex-partner;
-   friends/rivals where modeled;
-   directional opinion;
-   important social memories;
-   off-map relationships.

Opinion breakdown distinguishes A→B from B→A.

## ENG-003 contract (inspector Social tab)

-   **Entry:** the pawn inspector's Social tab (live since ENG-003).
-   **Regions:** relation table (relation label, name, my opinion /
    their opinion, status: on map, prisoner, away, dead); hover card
    with both opinion breakdowns; romance note line; social log below
    with an interaction icon per line (`ui_icon/social_*`).
-   **Information:** `Simulation.Social(id)` → `game.SocialView`
    (`Relations []RelationRowView`, `Log []SocialLogLineView`).
    Opinions are points (the sim keeps centi-points). A→B and B→A are
    separate columns and separate breakdowns (`TermsTo`, `TermsFrom`).
-   **Actions / Commands:** read-only; clicking a row selects/jumps to
    that pawn when on the map. Weighty log lines also go to History.
-   **States:** `ok == false` shows "No data yet."; empty relations show
    "Knows nobody yet."; off-map people show their whereabouts.
-   [-] Live (UI-022): relations with A->B / (B->A) opinions, status tags,
    hover breakdowns, log with icons; missing: romance/ex rows and relationship events (M5).

------------------------------------------------------------------------

# 16. Pawn Gear / Inventory Screen

## Purpose

Manage and inspect equipment, apparel and carried items.

## Sections

-   equipped weapon;
-   worn apparel;
-   inventory;
-   carrying capacity/mass where applicable.

## Actions

-   equip;
-   wear;
-   drop;
-   consume/use;
-   inspect.

Weapon/apparel entries expose relevant combat, armor, insulation,
quality and condition stats.

------------------------------------------------------------------------

# 17. Architect Screen / Build Palette

## Purpose

Primary interface for changing the colony's physical environment.

## Categories

-   Orders;
-   Zones/Areas;
-   Structure;
-   Furniture;
-   Production;
-   Power;
-   Security;
-   Miscellaneous;
-   endgame categories when unlocked.

## Build cards

Show: - icon; - name; - resource cost; - research lock; - short
description.

## Placement mode

Show: - ghost; - footprint; - rotation; - interaction cells; -
material/stuff; - validity; - failure reason; - estimated resource
requirement.

Support point, line, rectangle and paint placement depending on tool.

------------------------------------------------------------------------

# 18. Zone / Area Manager

## Purpose

Create spatial rules for storage, growing, pawn movement and colony
maintenance.

## Zone modes

-   stockpile;
-   growing;
-   home;
-   allowed areas.

## Area manager

-   create;
-   rename;
-   delete;
-   clear;
-   invert;
-   choose overlay.

## Stockpile settings

-   priority;
-   hierarchical item filters;
-   quality;
-   condition;
-   freshness/taint/etc.;
-   copy/paste.

## Growing settings

-   [x] crop (PLAY-020: "Plant: <crop>" on the selected zone, the architect's list and research locks);
-   [x] sowing toggle (PLAY-020: Allow sowing; harvest goes on; plus Allow cutting);
-   [-] plant requirements (the pane says growing now, too cold or too hot with the zone's temperature; no fertility or light lines);
-   [-] growth/yield information (sown and ripe counts and average growth; no yield estimate).
-   [x] PLAY-020, as RimWorld does: a selected zone also offers Expand and Shrink (drag-rectangle tools for that zone only, active until right-click or Esc), Delete, Hide (the overlay shows again while selected) and Rename (the label shows the name).

## ENG-003 contract (allowed areas)

-   **Entry:** Architect → Zones → Areas, and a per-pawn "Area" picker
    on the Schedule/Assign screen.
-   **Regions:** list of up to 10 areas (name, cell count), paint/clear
    tools, rename field.
-   **Information:** `Simulation.Areas()` → `[]game.AreaView`
    (`Index`, `Name`, `Cells`).
-   **Commands:** `command.PaintArea` (`Index`, X..X2/Y..Y2, `Enabled`
    paints or clears), `command.RenameArea` (`Index`, `Name`),
    `command.AssignArea` (`Pawn`, `Index`, -1 = unrestricted).
-   **States:** no areas yet: "No areas: add one to restrict where
    colonists go."; the 10-area cap disables "New area" with the reason.
-   [x] Wired (SIM-046), rebuilt in PLAY-015 (below).

## PLAY-015 contract (allowed areas, pens, Home, fire)

-   **Area manager** (modal): rows = colour swatch, inline rename,
    Invert, red ✕ Delete (confirm when pawns use it); "New area" (off at
    10), "Close". Opened from Architect → Zone → Expand/Clear allowed
    area ▸ "Manage allowed areas…", the Schedule, Animals and Assign
    tabs. Commands carry the area **id** (`AreaView.ID`):
    `CreateArea{Name}`, `RenameArea`, `InvertArea`, `DeleteArea`,
    `PaintArea`, `AssignArea{Pawn, Index: id | AreaUnrestricted (-1) |
    AreaHome (-2)}`.
-   **Overlay:** the armed or hovered area tints its cells
    (`AreaView.Bits`, rebuilt on `Ver`).
-   **Selectors:** Schedule tab "Allowed area" column per colonist; pawn
    inspector line "Area: …"; Animals tab "Area / Pen" column (area
    animals: dropdown; penned: pen name, "Roaming", "Being roped").
-   **Pen marker inspector:** Animals (per species allow + ♂/♀, All/
    None), Food (grazing/day, demand/day, trough stock, verdict, "Not
    enclosed"), Auto-cut (plant checklist) → `SetPenSettings`.
    Architect → Misc "Pen marker".
-   **Home:** the "Auto" toggle in the top strip (`SetHomeAuto`);
    Architect → Zone "Expand/Clear home area", "Build/Remove roof area",
    "Snow clearing area"/"Clear snow clearing area".
-   **Fire:** right-click a burning cell with a drafted pawn → "Extinguish
    fire"; Work tab Firefight column; "Fire in the colony!" alert; dev
    "Start fire here".

------------------------------------------------------------------------

# 19. Work Screen

## Purpose

Control autonomous labor priorities.

## Layout

Rows = pawns.\
Columns = work types.

## Modes

-   simple enabled/disabled;
-   manual priorities equivalent to 1--4.

## Cells

Display: - priority; - relevant skill; - passion; - incapability.

## Interaction

-   click cycle;
-   drag paint;
-   bulk selection;
-   sort/filter;
-   copy profile.

## Scale

Must remain usable with 1,000 pawns through virtualization, grouping,
filtering and bulk operations.

------------------------------------------------------------------------

# 20. Schedule Screen

## Purpose

Control when pawns sleep, work, recreate or choose freely.

## Grid

Rows = pawns.\
Columns = 24 game hours.

## Modes

-   Anything;
-   Work;
-   Sleep;
-   Recreation.

## Actions

-   paint cells;
-   copy schedule;
-   apply to multiple pawns;
-   use group preset.

Current hour is visibly highlighted.

------------------------------------------------------------------------

# 21. Assign / Policies Screen

## Purpose

Centralize per-pawn behavioral policies.

## Columns

-   food;
-   apparel;
-   drugs;
-   medical care;
-   allowed area where appropriate.

## Actions

-   assign existing policy;
-   create;
-   duplicate;
-   rename;
-   edit;
-   delete;
-   bulk assign.

Individual overrides should remain visible when group policies are
introduced.

------------------------------------------------------------------------

# 22. Policy Editor

## Purpose

Edit reusable colony policies.

## Food

Hierarchical allowed-food filter.

## Apparel

Allowed items plus quality/condition/tainted constraints.

## Drugs

Per drug: - recreational permission; - schedule; - mood threshold; -
recreation threshold; - carry amount.

## Medical

Maximum medicine/care tier.

Policies are data-driven and plugin-extensible.

------------------------------------------------------------------------

# 23. Production / Bills Screen

## Purpose

Control what a workstation produces and under what conditions.

## Bill stack

-   add;
-   reorder;
-   suspend;
-   delete;
-   copy/paste.

## Bill modes

-   Do X;
-   Forever;
-   Until X.

## Configuration

-   ingredients;
-   material filters;
-   ingredient radius;
-   quality/condition;
-   worker;
-   skill range;
-   output destination;
-   target count.

## Feedback

Explain why a bill cannot run: - no ingredients; - no worker; - no
power; - research lock; - inaccessible materials.

------------------------------------------------------------------------

# 24. Research Screen

## Purpose

Understand and control technological progression.

## Main region

Zoomable/pannable technology graph.

## Node states

-   locked;
-   available;
-   selected;
-   researching;
-   completed.

## Detail panel

-   description;
-   prerequisites;
-   research cost;
-   required bench/facilities;
-   unlocks.

## Features

-   search;
-   jump to prerequisite;
-   plugin-added technologies;
-   dynamically manageable graph layout.

------------------------------------------------------------------------

# 25. Animals Screen

## Purpose

Manage owned animals efficiently.

## Table

-   name;
-   species;
-   sex;
-   age;
-   health;
-   master;
-   allowed area/pen;
-   training;
-   slaughter;
-   medical care.

## Training

Expose supported training categories and progress.

## Actions

-   rename;
-   assign master;
-   assign area;
-   designate training;
-   designate slaughter;
-   set medicine.

------------------------------------------------------------------------

# 26. Wildlife Screen

## Purpose

Survey wild animals and manage hunting/taming.

## Table

-   species;
-   count;
-   predator status;
-   hunt risk;
-   wildness;
-   tame chance;
-   body size/yield context.

## Actions

-   hunt;
-   tame where supported;
-   select/jump;
-   sort/filter.

------------------------------------------------------------------------

# 27. Prisoner Screen

## Purpose

Manage captured pawns.

## Information

-   faction;
-   health;
-   mood;
-   resistance;
-   recruitment difficulty;
-   current interaction.

## Interaction policies

-   none;
-   reduce resistance;
-   recruit;
-   release;
-   execute where supported.

Show recent warden progress and explain missing warden/food/bed
problems.

## ENG-003 contract (inspector Prisoner tab)

-   **Entry:** selecting a prisoner shows a Prisoner tab in the pawn
    inspector; the Capture / Arrest orders come from the right-click
    menu on a downed or targetable pawn.
-   **Regions:** mode selector (none, reduce resistance, recruit,
    release, execute); resistance bar; assigned warden; bed line.
-   **Information:** `Simulation.Prisoner(id)` → `game.PrisonerView`
    (`Mode` = prisoner.Mode, `Resistance`, `Warden`, `ForPrisonersBed`).
-   **Commands:** `command.SetPrisonerMode` (`Target`, `Mode`),
    `command.ReleasePrisoner` (`Target`), `command.Capture` /
    `command.Arrest` (`Pawn`, `Target`), `command.SetBedForPrisoners`
    (`Target` = bed, `Enabled`). Wardening is the Warden work column
    (off by default).
-   **States:** `ok == false`: no tab; no warden enabled, no prisoner
    bed or no food each show a warning line with the fix.
-   [ ] Not wired yet (M8).

------------------------------------------------------------------------

# 28. Trade Screen

## Purpose

Perform a transaction with a visiting/orbital/faction trader.

## Layout

Large two-sided inventory table.

Columns: - item; - colony quantity; - transaction amount; - trader
quantity; - buy/sell price; - quality; - condition.

## Controls

-   search;
-   category filters;
-   sort;
-   quantity;
-   reset;
-   gift mode;
-   accept;
-   cancel.

## Summary

-   colony currency;
-   transaction total;
-   resulting balance;
-   negotiator;
-   trade modifier;
-   projected goodwill in gift mode.

Validation handles affordability and stock changes.

------------------------------------------------------------------------

# 29. Factions Screen

## Purpose

Understand geopolitical relationships.

## Faction list

-   emblem;
-   name;
-   technology level;
-   goodwill;
-   relationship state;
-   permanent hostility.

## Detail

-   description;
-   known settlements;
-   relationship history;
-   recent goodwill changes.

## Communication actions

Where available: - call; - request trader; - request aid; - offer gift.

Costs and unavailable reasons must be explicit.

------------------------------------------------------------------------

# 30. History / Records Screen

## Purpose

Provide a long-term record of the colony.

## Tabs

### Timeline

Major events by game date.

### Graphs

-   wealth;
-   population;
-   animal population;
-   threats/events;
-   other useful colony metrics.

### Records

Pawn-specific statistics such as kills, damage, production, research and
notable accomplishments.

Long graphs require downsampling rather than rendering every sample.

## ENG-003 contract (History tab)

-   **Entry:** bottom-bar History tab.
-   **Regions:** chronological list, newest first, filter chips by kind
    (social, death, wedding, fight, recruit).
-   **Information:** `Simulation.History()` → `[]game.HistoryEntryView`
    (`Tick`, `Text`, `Kind`, `Pawns`).
-   **Actions:** click a line to select/jump to a pawn named in it
    (when on the map). Read-only; no commands.
-   **States:** empty: "Nothing worth remembering has happened yet."
-   [ ] Not wired yet (M4).

------------------------------------------------------------------------

# 31. Quest Screen

## Purpose

Track available, active and completed structured opportunities.

## Quest list

-   available;
-   active;
-   completed;
-   failed.

## Detail

-   original narrative;
-   source/faction;
-   objectives;
-   known threats;
-   world targets;
-   deadline;
-   reward choices;
-   goodwill consequences.

## Actions

-   accept;
-   decline;
-   choose reward;
-   jump to target.

Active quests show objective progress and countdowns.

------------------------------------------------------------------------

# 32. Alerts & Event Letters

## Purpose

Surface simulation events requiring awareness or action.

## Alerts

Persistent conditions such as: - low food; - medical emergency; - break
risk; - no bed; - dangerous temperature; - idle colonists; - animal
starvation; - no research; - hostile threat.

Repeated alerts aggregate.

## Letters

Persistent event notifications with: - title; - narrative; -
entities/faction; - severity; - location; - choices where required.

Actions can jump to relevant map/world target.

------------------------------------------------------------------------

# 33. World Map Screen

## Purpose

Manage the campaign beyond the local colony map.

## Main view

Interactive generated world.

## Objects

-   colony;
-   settlements;
-   caravans;
-   quest sites;
-   temporary sites;
-   encounter sites;
-   endgame targets.

## Selected tile

Show biome, terrain, weather/climate, growing period, roads/rivers,
movement cost, forage and ownership.

## Routing

Show path, travel time, terrain cost and invalid route reasons.

## ENG-003 contract (World tab)

-   **Entry:** bottom-bar World tab (the globe view of §7 in read-only
    colony mode).
-   **Regions:** globe with the colony tile and caravan markers; caravan
    list on the side.
-   **Information:** `Simulation.WorldMap()` → `game.WorldMapView`
    (`Tile`, `Caravans []CaravanView`).
-   **Actions / Commands:** "Form caravan" opens §34; selecting a
    caravan opens §35.
-   **States:** no caravans: the list says so; the colony tile is always
    marked.
-   [ ] Not wired yet (M9).

------------------------------------------------------------------------

# 34. Form Caravan Screen

## Purpose

Build an expedition and verify it can survive the journey.

## Sections

-   destination;
-   colonists;
-   prisoners;
-   animals;
-   items;
-   food;
-   medicine;
-   currency.

## Summary

-   mass/capacity;
-   movement speed;
-   estimated travel time;
-   food duration;
-   forage;
-   temperature concerns.

## Warnings

-   overload;
-   insufficient food;
-   immobile pawn;
-   dangerous climate;
-   missing capability.

## Actions

-   select contents;
-   send;
-   cancel.

## ENG-003 contract (minimal slice)

-   **Entry:** World tab → Form caravan.
-   **Regions:** colonist and animal pick lists (checkboxes), the
    destination tile picked on the globe, a summary (count, speed).
-   **Commands:** `command.FormCaravan` (`Name` = comma-separated member
    ids, `Count` = destination tile).
-   **States:** no members picked or no destination: Send disabled with
    the reason; downed or drafted colonists are listed disabled.
-   [ ] Not wired yet (M9); no goods or mass budget in this slice.

------------------------------------------------------------------------

# 35. Caravan Inspector

## Purpose

Manage an active world-traveling group.

## Information

-   members;
-   health/mood;
-   animals;
-   inventory;
-   food;
-   carrying capacity;
-   speed;
-   current route;
-   arrival estimate;
-   rest/forage.

## Actions

-   change destination;
-   enter settlement/site;
-   trade;
-   attack;
-   settle;
-   return/reform where appropriate.

## ENG-003 contract (minimal slice)

-   **Entry:** selecting a caravan on the World tab.
-   **Information:** `game.CaravanView` (`ID`, `Members`, `Tile`,
    `Dest`, `Progress` percent to the next tile, `State`).
-   **Commands:** `command.CaravanReturn` (`Target` = caravan id).
-   **States:** arrived caravans show "Waiting at destination";
    returning ones show the trip home.
-   [ ] Not wired yet (M9).

------------------------------------------------------------------------

# 36. Transport Pod Loading / Targeting Screen

## Purpose

Load and launch transport systems.

## Loading

-   launchers;
-   fuel;
-   pawns;
-   items;
-   mass/capacity;
-   loading progress.

## Targeting

World map displays: - range; - valid targets; - destination; - estimated
result.

## Actions

-   launch;
-   cancel;
-   select landing/attack/gift/reinforcement behavior where supported.

------------------------------------------------------------------------

# 37. Endgame Ship Screen

## Purpose

Explain endgame readiness and launch sequence.

## Information

-   required components;
-   connectivity;
-   reactor;
-   missing components;
-   caskets;
-   occupants;
-   countdown;
-   launch readiness.

## Actions

-   start reactor;
-   load/unload caskets;
-   launch.

Reactor activation requires explicit warning about the major defense
phase.

------------------------------------------------------------------------

# 38. Victory / Campaign Summary

## Purpose

Close the campaign and reflect the emergent story back to the player.

## Content

-   outcome;
-   surviving/escaped pawns;
-   colony age;
-   population history;
-   major events;
-   deaths;
-   relationships;
-   research;
-   wealth;
-   notable records;
-   final colony image where practical.

## Actions

-   continue viewing colony if supported;
-   main menu;
-   credits;
-   export/share summary later.

------------------------------------------------------------------------

# 39. In-Game Pause Menu

## Purpose

Session-level controls.

## Actions

-   Resume
-   Save
-   Load
-   Options
-   Help
-   Return to Main Menu
-   Quit

Destructive session exits require confirmation when unsaved progress
exists.

------------------------------------------------------------------------

# 40. Save Game Screen

## Purpose

Create and manage saves.

## List information

-   name;
-   real timestamp;
-   game date;
-   colony;
-   version;
-   plugin summary.

## Actions

-   create save;
-   overwrite;
-   delete;
-   cancel.

Show save progress and errors without freezing indefinitely.

Commitment/ironman-like modes may restrict controls.

------------------------------------------------------------------------

# 41. Load Game Screen

## Purpose

Choose a campaign safely.

## Information

-   save name;
-   colony;
-   game date;
-   version;
-   required plugins;
-   compatibility.

## States

-   compatible;
-   migration required;
-   missing plugin;
-   incompatible version;
-   corrupt.

Never silently load a materially incompatible save.

------------------------------------------------------------------------

# 42. Options Screen

Settings live in `internal/platform/config` and are saved to
`<user config dir>/rimworld-go/settings.json` (UI-012). They change
presentation only, never the simulation; captures ignore them.

## Tabs

### Graphics

-   [x] window: fullscreen toggle (the window resizes freely; no
    resolution presets);
-   [-] VSync/frame cap (VSync toggle; Ebitengine has no frame cap);
-   [x] UI scale (UI-024: Accessibility row Auto / 75–200 %; "Sharp on
    high-density screens" (HiDPI) on the Graphics tab);
-   [x] effects/detail (Full/Reduced/Minimal: particles, weather drops,
    menu embers and dust);
-   [x] quality preset (PLAY-019: High / Medium / Low, picked from the
    computer's RAM and cores at the first start and saved; "Custom" once
    a row is changed by hand, and those rows survive a new preset;
    "Detect again" re-applies the detected tier);
-   [x] detail rows (PLAY-019: Shadows, Lighting (normal-map light; off
    keeps the hour, weather and lamp tint), Water (animated / simple),
    Grass (full / reduced / sparse tufts), each with a cost tooltip and
    applied live);
-   [ ] far-zoom detail.

### Audio

-   [x] master;
-   [x] music;
-   [x] effects;
-   [x] ambience;
-   [x] UI;
-   [x] mute when unfocused.

### Gameplay

-   [x] pause on threat;
-   [x] autosave (daily, every 3 days, weekly, off);
-   [x] edge scrolling;
-   [x] camera pan speed;
-   [x] developer mode (UI-016: ultrafast, tick step, debug actions, log, inspector);
-   [ ] speed behavior;
-   [ ] confirmations;
-   [ ] learning helper.

### Controls

-   [x] bindings (UI-016: every action by category, paged);
-   [x] rebinding (click a slot, press a key; Esc cancels, Backspace clears);
-   [x] conflicts ("Swap?" before a used key is taken);
-   [x] reset.

### Accessibility

-   [x] text/UI scale (UI-024: one UI scale row, Auto or 75–200 %, text included);
-   [ ] color-vision support;
-   [x] reduced effects/flashing (Reduced flashing caps lightning; Reduced
    motion freezes the menu drift, drops embers and dust, and snaps the
    camera);
-   [ ] tooltip delay;
-   [ ] other supported accessibility options.

------------------------------------------------------------------------

# 43. Tutorial / Learning Helper

## Purpose

Teach mechanics contextually without requiring a separate scripted
campaign for everything.

## Features

-   lesson cards;
-   contextual triggers;
-   highlighted control/entity;
-   action completion detection;
-   lesson history;
-   dismiss;
-   mark learned;
-   reopen.

The helper should stop interrupting players who demonstrate familiarity.

------------------------------------------------------------------------

# 44. Help / Encyclopedia Screen

## Purpose

Provide searchable in-game reference.

## Content

-   mechanics;
-   controls;
-   stats;
-   work types;
-   health;
-   combat;
-   construction;
-   research;
-   factions;
-   caravans;
-   plugins.

Plugin content may register help entries.

Avoid duplicating every tooltip verbatim; provide conceptual
explanations.

------------------------------------------------------------------------

# 45. Plugin Manager

## Purpose

Make mod/plugin management a first-class product capability.

## List

-   name;
-   author;
-   version;
-   ID;
-   enabled;
-   compatibility;
-   error state.

## Detail

-   description;
-   dependencies;
-   incompatibilities;
-   capabilities;
-   source/homepage metadata;
-   settings availability.

## Actions

-   enable/disable;
-   search/filter;
-   inspect dependency;
-   configure;
-   resolve/review load order.

------------------------------------------------------------------------

# 46. Plugin Load Order / Dependency Screen

## Purpose

Explain exactly how enabled plugins resolve.

## Information

-   resolved order;
-   dependencies;
-   load-before/after constraints;
-   cycles;
-   conflicts;
-   missing dependencies.

## Actions

Manual ordering should exist only where the plugin model permits it.

The UI should explain **why** a plugin occupies a position rather than
presenting an unexplained list.

------------------------------------------------------------------------

# 47. Plugin Settings Screen

## Purpose

Expose plugin configuration without bespoke UI code for every plugin.

## Schema-driven controls

-   boolean;
-   number;
-   enum;
-   string;
-   safe key binding.

Clearly distinguish: - global; - per-save; - requires restart; -
requires new game.

Actions: - apply; - reset; - cancel.

------------------------------------------------------------------------

# 48. Content / Plugin Error Screen

## Purpose

Turn content failures into actionable diagnostics.

## Error information

-   plugin;
-   definition;
-   source file;
-   field/path;
-   validation message;
-   dependency/provenance.

## Actions

-   copy diagnostics;
-   inspect plugin;
-   disable plugin when safe;
-   continue in recovery mode when safe;
-   quit.

Recoverable definition errors should not produce an opaque crash.

------------------------------------------------------------------------

# 49. Developer Mode Main Panel

## Purpose

Provide first-class development tools for an open-source simulation.

## Sections

-   debug actions;
-   overlays;
-   entity inspector;
-   incident tools;
-   content inspector;
-   profiler;
-   replay/determinism tools.

Developer controls are visually unmistakable from normal gameplay UI.

------------------------------------------------------------------------

# 50. Developer Entity Inspector

## Purpose

Explain authoritative state deeply enough to debug AI/simulation
problems.

## Information

-   EntityID;
-   components;
-   position;
-   current intent;
-   job/state;
-   reservations;
-   path;
-   needs;
-   mood;
-   health;
-   faction;
-   relevant revisions;
-   RNG identity/streams where useful;
-   state hash/debug information.

## Actions

Development-only mutation tools may spawn, destroy, teleport, damage,
heal, set needs, force jobs, etc., through explicit debug commands.

------------------------------------------------------------------------

# 51. Debug Overlay Screen

## Purpose

Visualize invisible simulation structures.

Toggleable overlays: - coordinates; - chunks/activity; - regions; -
rooms; - walkability; - path cost; - paths; - reservations; - work
candidates; - storage indexes; - temperature; - power networks; -
light; - beauty; - cover; - roof; - fire active set.

Overlay computation must not accidentally become an expensive
every-frame world scan.

------------------------------------------------------------------------

# 52. Definition / Content Inspector

## Purpose

Debug data-driven content and plugins.

## Features

-   search all definitions;
-   type filter;
-   resolved fields;
-   DefID/DefIndex;
-   source plugin;
-   inheritance;
-   patch history;
-   references.

This screen should make it possible to answer: **"Why does this
definition have this value?"**

------------------------------------------------------------------------

# 53. Performance Profiler Screen

## Purpose

Measure whether the simulation meets project scaling goals.

## Top metrics

-   TPS;
-   average tick;
-   p50/p95/p99;
-   heap;
-   allocations;
-   goroutines;
-   active/sleeping entities.

## System breakdown

-   scheduler;
-   pawn AI;
-   job discovery;
-   pathfinding;
-   combat;
-   health;
-   rooms;
-   temperature;
-   power;
-   incidents;
-   rendering;
-   UI;
-   plugins.

## Drill-down

Show requests, nodes expanded, cache hit rates, dirty rebuilds, active
sets and other subsystem metrics.

------------------------------------------------------------------------

# 54. Large-Colony Management Screen

## Purpose

Provide a management layer specifically for hundreds or thousands of
colonists.

This is a `rimworld-go` differentiator rather than simply reproducing
vanilla-scale UI.

## Regions

### Groups

-   create/rename/delete;
-   membership;
-   group icon/color;
-   hierarchy if later justified.

### Strategic overview

-   pawn count;
-   working/idle;
-   injured;
-   break risk;
-   drafted;
-   food;
-   medicine;
-   beds;
-   production bottlenecks;
-   critical alerts.

### Bulk policies

-   work preset;
-   schedule;
-   area;
-   food;
-   apparel;
-   drugs;
-   medicine.

### Search

Find a pawn/group by name, skill, state, job or policy.

## Requirements

The player must be able to move from: **"42 colonists are medically
compromised"** to the relevant subgroup without manually inspecting 42
portraits.

------------------------------------------------------------------------

# 55. Global Search / Command Palette

## Purpose

Make a very large game navigable quickly.

Potential searchable targets: - pawns; - animals; - buildings; -
items; - stockpiles; - research; - architect commands; - world sites; -
quests; - factions; - plugins; - help; - developer entities.

Results should support actions such as select, jump, open screen, or
activate command.

This becomes increasingly valuable at large colony scale.

------------------------------------------------------------------------

# 56. Screen Implementation Order

Agents should implement screens approximately in this dependency order.

## Wave UI-0 --- Framework

-   [-] UI primitives (button/label/panel; no sliders, text fields, virtual lists)
-   [-] input/focus (UI-016: input context stack, shortcut registry with rebinding, text fields swallow keys; no focus ring)
-   [-] window/modal system (full-screen stack + pause overlay)
-   [-] tooltips (disabled-reason line only)
-   [ ] virtual lists/tables
-   [-] presentation snapshot boundary (colony PawnView; menus are headless)
-   [-] typed command gateway (session intents plus `Enqueue` gameplay: MovePawn, designations, SetWorkEnabled)

## Wave UI-1 --- First interactive simulation

-   [-] Boot (phase list + pack load; no plugin/fatal recovery)
-   [-] Main Menu (navigation shell; Continue/Plugins/Tutorial disabled; original colonist splash left over the animated colony diorama, actions right rail)
-   [-] Main Colony (viewport plus Food/Wood/time strip, colonist names, Architect/Work/Menu; no alerts)
-   [x] camera (WASD/arrows pan with Shift fast pan, edge pan, middle-drag, keyboard/wheel zoom, centre on colonist)
-   [-] time controls (pause / normal 1x / fast 3x / superfast 6x, date/period/clock, current schedule block in top chrome; no temperature/weather)
-   [-] selection (click pawn or colonist bar; inspector Character/Needs)
-   [-] Generic Inspector (name, job, status, reason, food/rest bars; Health/Social/Gear disabled)
-   [-] colonist bar (snapshot slots, click selects, idle portrait, display name; no status icons, no double-click center)
-   [-] contextual commands (Architect + Work + Menu live; others disabled `not implemented`; no selection command bar)

## Wave UI-2 --- Build a colony

-   [-] Architect (Wall/Bed/Stockpile/Harvest/Cancel + drag ghosts; no categories/cost cards)
-   [-] placement (drag rect; walls/beds per cell; no rotation/validity colors)
-   [-] zones (stockpile paint food+wood; no grow-zone/home/allowed)
-   [ ] stockpile settings
-   [x] growing settings (PLAY-020)
-   [-] Work (overlay from snapshot; Grow/Bld/Haul toggle Y/-; not virtualized)
-   [ ] Schedule
-   [-] Area Manager (SIM-046: create, rename, paint, assign; no delete or overlay)

## Wave UI-3 --- Understand pawns

-   [-] Character (Bio tab: identity, backstories, traits, skills with passions, incapabilities, conditions; no Social/Gear tabs)
-   [-] Needs/Mood (food/rest bars; no recreation/mood/thoughts)
-   [-] Health (capacities, pain, part rows, operations queue; no bleeding or conditions)
-   [x] Social (relations incl. lovers/spouses/exes, breakdowns with hover tooltip, log, romance, engagement, wedding, breakup, fight and party letters; captured: `-scenario social -capture-ui social`, also health, prisoner, history, world, assign)
-   [ ] Gear
-   [ ] Detailed Stats
-   [-] Alerts/Letters (alert list and letter windows with Continue; no letter stack or history)

## Wave UI-4 --- Production and defense

-   [ ] Bills
-   [ ] Research
-   [ ] Assign/Policies
-   [ ] combat controls/targeting on Main Colony
-   [ ] environmental overlays

## Wave UI-5 --- Colony ecosystem

-   [-] Animals (SIM-047: master and training columns; no health or area)
-   [ ] Wildlife
-   [-] Prisoner (SIM-045: Prisoner tab with modes and resistance; no prison break or chat history)
-   [ ] Trade
-   [-] Factions (window with the faction table, 60-day goodwill sparkline and last five events; goodwill only, no trade or communication)
-   [ ] History

## Wave UI-6 --- Complete new-game flow

-   [x] Scenario (built-ins, custom files, search, details; no plugin-registered scenarios)
-   [x] Scenario Editor
-   [-] Storyteller/Difficulty (cards, six presets, save mode, Custom Difficulty dialog; disease frequency has no effect and a few sliders lack tooltips)
-   [x] World Generation (seed, coverage, rainfall, temperature, population, factions, progress)
-   [-] Landing Site (globe and full tile panel, Advanced, Factions dialog; no feature search)
-   [x] Starting Pawns (generated candidates, reroll, rename, team warnings, Start with a loading screen; no relationships)

These can begin earlier once their underlying definitions exist, but are
required before a polished external build.

## Wave UI-7 --- World gameplay

-   [-] World Map (SIM-046: World tab F8 with globe and caravan list; no object selection)
-   [-] Caravan Formation (SIM-046: colonists and destination only)
-   [-] Caravan Inspector (SIM-046: caravan list in the World tab)
-   [ ] Quest
-   [ ] Transport Pod

## Wave UI-8 --- Campaign completion

-   [ ] Endgame Ship
-   [ ] Victory
-   [-] Save (pause > Save writes the colony's slot; no name prompt; commitment colonies save themselves and offer Save & Quit)
-   [x] Load (save list from the main menu and pause; corrupt saves listed disabled with the reason; disabled with a reason in commitment mode)
-   [-] Pause Menu (Resume/Save/Load/Options/Main Menu/Quit; Help disabled; Save & Quit in commitment mode)
-   [x] Options (Audio/Graphics/Gameplay/Accessibility/Controls tabs, saved to settings.json; Story tab during play)
-   [ ] Tutorial
-   [ ] Help

## Wave UI-9 --- Modding platform

-   [ ] Plugin Manager
-   [ ] Load Order
-   [ ] Plugin Settings
-   [ ] Content Error
-   [ ] Definition Inspector

## Wave UI-10 --- Developer/scaling

-   [ ] Developer Panel
-   [ ] Entity Inspector
-   [ ] Debug Overlays
-   [ ] Profiler
-   [ ] Large-Colony Management
-   [ ] Global Search/Command Palette

------------------------------------------------------------------------

# 57. Shared Screen Navigation Model

Recommended conceptual navigation:

``` text
BOOT
  ↓
MAIN MENU
  ├── Continue ───────────────→ COLONY
  ├── Load ─→ LOAD ──────────→ COLONY
  ├── New Game
  │     ↓
  │   Scenario
  │     ↓
  │   Storyteller/Difficulty
  │     ↓
  │   World Generation
  │     ↓
  │   Landing Site
  │     ↓
  │   Starting Pawns
  │     ↓
  │   COLONY
  │
  ├── Plugins
  └── Options

COLONY
  ├── Inspector
  │    ├── Character
  │    ├── Needs
  │    ├── Health
  │    ├── Social
  │    ├── Gear
  │    └── Stats
  │
  ├── Architect
  ├── Work
  ├── Schedule
  ├── Assign
  ├── Animals
  ├── Wildlife
  ├── Research
  ├── Factions
  ├── History
  ├── Quests
  ├── World Map
  └── Menu
```

NG-1 routing: Scenario opens the Scenario Editor (and the add-part picker as a modal); Storyteller/Difficulty opens the Custom Difficulty modal; World Generation opens a progress modal; Landing Site opens the Advanced and Factions modals; Starting Pawns leads to a loading screen and then the colony. The colony's Factions entry opens the Factions window. Back keeps the wizard draft; Back from Scenario asks before discarding.

Not every conceptual screen must become a full-screen route. Pawn
screens, bills and similar surfaces may be tabs/windows inside the main
colony UI.

------------------------------------------------------------------------

# 58. Screen vs Window vs Overlay Rule

Use **full screens** when the player's primary context changes: - main
menu; - new-game steps; - world map; - major management tables where
screen space is necessary; - plugin manager; - options.

Use **windows/panels** for focused inspection/configuration: - entity
inspector; - bill editor; - policy editor; - trade; - quest details; -
caravan formation; - health operations.

Use **overlays** for spatial information: - power; - temperature; -
roof; - fertility; - beauty; - room; - path/debug data.

**Roof overlay (UI-010, implemented):**
- **Toggle:** R or the HUD "Roof" button (icon `overlay_roof`). It is forced on while the Architect's Build roof or Remove roof tool is held.
- **What it shows:**
  - built roofs in blue
  - natural (mined-rock) roofs in stippled slate
  - planned roofs as amber dashes
  - planned removals as a red hatch
  - under a roof tool's rectangle, unsupported tiles in red with "×"
- **Legend:** "Roof: blue built · slate natural · dashed planned · hatched removal".
- **Hover panel:** a roof line: "Roofed", "Natural roof (rock)", "Roof planned", "Roof removal planned", "Unroofed", or "Unroofed (room too large to auto-roof)". The room line adds ", roofed N%".
- **Architect (Structure):**
  - "Build roof": "Colonists roof these tiles. A roof needs a wall within 6 tiles; closed rooms are roofed automatically."
  - "Remove roof": "Take roofs down and keep them off, even in closed rooms."
  - Both are area tools emitting `DesignateRoof` and `DesignateRemoveRoof`.

Use **modal confirmation** sparingly for destructive/irreversible
actions.

------------------------------------------------------------------------

# 59. Screen Definition of Done

A main screen is complete only when:

-   [ ] its underlying gameplay feature exists;
-   [ ] it consumes read models/snapshots;
-   [ ] all authoritative mutations are typed commands;
-   [ ] navigation works;
-   [ ] Escape/back behavior works;
-   [ ] keyboard focus works;
-   [ ] disabled actions explain why;
-   [ ] empty states exist;
-   [ ] stale EntityIDs are handled safely;
-   [ ] loading/error states exist where applicable;
-   [ ] search/sort/filter work where specified;
-   [ ] large lists virtualize;
-   [x] supported resolutions/UI scales work (UI-024);
-   [ ] important information is not color-only;
-   [ ] tooltips explain non-obvious mechanics;
-   [ ] no full-world scan occurs every render frame;
-   [ ] controller/view-model behavior has tests where practical;
-   [ ] a visual acceptance screenshot/scenario exists;
-   [ ] plugin-provided content works where extension is intended;
-   [ ] relevant documentation is updated.

------------------------------------------------------------------------

# 60. Final Screen Inventory

The major interfaces defined by this specification are:

1.  Boot / Loading
2.  Main Menu
3.  Scenario Selection
4.  Scenario Editor
5.  Storyteller & Difficulty
6.  World Generation
7.  Landing Site
8.  Starting Pawns
9.  Main Colony
10. Generic Inspector
11. Detailed Stats
12. Character/Bio
13. Needs/Mood
14. Health
15. Social
16. Gear/Inventory
17. Architect
18. Zones/Areas
19. Work
20. Schedule
21. Assign/Policies
22. Policy Editor
23. Production/Bills
24. Research
25. Animals
26. Wildlife
27. Prisoners
28. Trade
29. Factions
30. History/Records
31. Quests
32. Alerts/Event Letters
33. World Map
34. Form Caravan
35. Caravan Inspector
36. Transport Pod
37. Endgame Ship
38. Victory/Campaign Summary
39. Pause Menu
40. Save
41. Load
42. Options
43. Tutorial/Learning Helper
44. Help/Encyclopedia
45. Plugin Manager
46. Plugin Load Order
47. Plugin Settings
48. Content/Plugin Errors
49. Developer Mode
50. Developer Entity Inspector
51. Debug Overlays
52. Definition/Content Inspector
53. Performance Profiler
54. Large-Colony Management
55. Global Search / Command Palette

The objective is not to maximize the number of windows. Several of these
should share navigation and reusable components. The objective is to
ensure **every major player workflow has an explicitly designed home**
before implementation becomes fragmented across agents.
