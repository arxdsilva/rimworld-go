# rimworld-go --- Main UI Screens Specification

> **What:** Canonical build spec for every major player-facing screen in `rimworld-go`: layout, information, actions, commands and states.
> **Load by section through skill `rimworld-go-ui-screens`, never whole.**
> Update the matching checklist rows in the same change (root `AGENTS.md`).
> Design target: the information density, discoverability and management depth of a deep vanilla colony simulator, with original layouts, artwork and visual identity.
> Companion docs: `.agents/backlog/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`, `.agents/backlog/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.agents/specs/HEXAGONAL_ARCHITECTURE.md`, `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`.

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

> **NG-1 (normative):** §2–§8 below are implemented per `.agents/handoffs/NG.md#ng1_new_game_plan` §1 (shared visual language, font, widget kit, navigation, wizard draft) and §2.1–§2.13 (every screen: layout, control ids, validation, states, tests, captures). Where this spec and the plan differ, the plan wins for these screens. Original names only: storytellers Orla Vane / Bram Holloway / Kestrel Quay; difficulties Wanderer…Merciless; scenarios Broken Hull, Ember Clan, Gilded Expedition, Bare Hands.

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
-   [x] Update notice (a card under the version line when GitHub has a
    newer release: Download opens the page in the browser, Later hides it
    until the next launch; a failed check shows nothing)

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

-   [x] seed; (free text, original two-word phrases; random button; WG-2: the seeded planet name heads the screen)
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

-   [x] selected-tile panel; (terrain, elevation, temperatures, rainfall, growing period, stones, water, caves, roads, forage %, wildlife, nearest settlement, movement difficulty, time zone, disease, travel by season, quadrum temperatures, nearby bases and the 4-tile base warning, warnings; Celsius or Fahrenheit)
-   [x] interactive planet/world map; (rotating globe with relief, faceted light, snow caps, coast and shore, biome, temperature, rainfall, elevation and hilliness views, a legend, optional grid, rivers by size, roads by class, feature labels, faction-tinted settlements)
-   [x] inspect tiles / search features; (inspect and hover; a search box over named features and settlements; a filter panel for Random site; the Factions dialog)
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
- **Palette.** Neutral translucent charcoal panels with a warm gold accent
  (`ui.yaml`, ART-024), close to RimWorld's readable chrome but with
  original values. The learning helper uses a warm instructional set.

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
-   double-click same category where useful. Furniture, blueprints and zones group too; walls, the hopper, floors, fences, conduits and plants do not (`MOUSE_CONTROLS_GAP_ANALYSIS.md`).

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

## Colonist card

With one pawn or one animal selected, the card replaces the pane's body (RHUD, `.agents/gaps/RIMHUD_GAP_ANALYSIS.md`). It is a tree of value widgets the player can edit; groups, buildings, plants, items, zones and cell info keep the plain pane.

-   [x] on by default; Options > Accessibility has the switch, and off draws the plain pane;
-   [x] docked in the pane, or floating over the map at a chosen anchor; the docked card never scrolls: pane and tabs take at most 30 % of the screen's height (DOS-10: the sizes are set for a 720-tall screen and `sheet.DockedFor` scales them with it, 168 + 24 px at 720, 252 + 24 at 1080; the built-in docked tree is two lines, a rule and a four-column strip of bars, selectors and skills);
-   [x] setup modal from the card's gear: Look, Sections, Presets, Colours, with Back to defaults;
-   [x] presets saved as files, with a button that opens their folder;
-   [x] selectors on the card: food policy, allowed area (hovering a row previews it on the map), and the current hour (with Manage, which opens Schedule);
-   [x] the letter stack moves clear of a floating card;
-   [x] five lessons in the learning helper, only while the card is on;
-   [x] the developer inspector shows each widget's timing.

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
  - **Uninstall (AX-1):** a toggle that marks it to be packed into a crated building; a filled grave also offers **Open**, a downed person **Strip**.
  - **Auto refuel (REF-1):** a burner's toggle (`bldg.autorefuel`, the icon of the fuel it burns) sending `SetAutoRefuel`. Off, haulers leave it alone and only an ordered refuel feeds it; the fuel line adds "auto refuel off". Not on a fish tank or a hostile building.
- **Crated building (AX-1):** the item panel names what it holds; **Reinstall** runs the Move ghost for the packed kind and sends `Reinstall`.
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
  - Right-clicking a burner offers "Prioritize refueling <building>" (REF-1, `OrderRefuel`): the colonist fills it to full now; disabled rows give the simulation's reason (already full, no free wood or chemfuel, cannot reach, in use by, hauling disabled).
  - A group selection merges its members' commands: a toggle shows mixed, and a command is disabled only when it is for every member. A menu merges to the options every member has, and the pick runs on each; a command for one thing (Move, Copy, Settings) leaves the grid.
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

GEAR-* (`.agents/gaps/GEAR_GAP_ANALYSIS.md`): summary lines first (mass
carried against carrying capacity, comfortable range, overall armour
Sharp / Blunt / Heat, move speed), then Equipment, Apparel in layer
order, and the pack. Each row: icon, label with quality, HP %, mass,
tainted mark; Info, Drop (Shift queues), Lock on apparel, Eat on pack
food and drugs, Unload while that habit is on. The list scrolls. The tab
shows on colonists, on prisoners beside the Prisoner tab (drop allowed),
on pack animals, and read-only on guests.

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

STK-AG: all twelve vanilla categories are always shown, in the vanilla
order, as six rows of two buttons (`.agents/gaps/ARCHITECT_GAP_ANALYSIS.md`
lists every core item and its card):

-   Orders;
-   Zones/Areas;
-   Structure;
-   Production;
-   Furniture;
-   Power (PLAY-021, implemented: conduit and hidden conduit as line tools, switch, wood-fired, solar, wind and geothermal generators, battery; each card names its draw or output and its research lock; while a powered building is in hand the ghost links to the nearest conductor (conduit, switch that is on, generator or battery) within 6 tiles, joins by touching with no wire, or says "Not connected"; PWR-5: placed power blueprints show a dimmer wire);
-   Security;
-   Miscellaneous (grave, sarcophagus, tool cabinet, multi-analyzer, vitals monitor, spots, comms);
-   Floors (wood floor, carpet, Smooth);
-   Recreation (AX-6: horseshoe pin, ring toss, ur board, chess, billiards and poker tables, tube and flatscreen televisions; the big screen television and telescope cards are locked "Found in ruins, never built"; a recreation piece's pane shows its joy kind and users, a colonist's Needs tab what it enjoys and its tolerances);
-   Ship (placeholders only);
-   Temperature (campfire, heater, cooler, passive cooler, vent).

A vanilla item this game does not have yet is a greyed placeholder card
in its category (icon `ui_icon/planned`): its tooltip gives one original
sentence and "Not in this game yet", and it can never be armed or
placed. Working cards come before placeholders, so a small screen's grid
never hides a working tool.

## Build cards

Show: - icon; - name; - resource cost; - research lock; - short
description; - for a placeholder, "Not in this game yet".

## Placement mode

Show: - ghost; - footprint; - rotation; - interaction cells; -
material/stuff; - validity; - failure reason; - estimated resource
requirement.

Support point, line, rectangle and paint placement depending on tool.

Zone tools (PLAY-023): the drag ghost tints each cell green where the zone can claim it and red where it can't. A tip by the cursor names the hovered cell's reason ("Can't zone: Not soil"; also Unexplored, Too close to the map edge, Another zone, Rock, Water, Floor, Bridge, Building, Planned building). A drag with no free cell hints its most common reason.

------------------------------------------------------------------------

# 18. Zone / Area Manager

## Purpose

Create spatial rules for storage, growing, pawn movement and colony
maintenance.

## Zone modes

-   stockpile;
-   dumping stockpile (PLAY-023, STK-ST: corpses and stone chunks by default, rotting ones too, Normal priority, own `zone_dump` icon);
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

-   priority (PLAY-023: "Priority: <level>" on a selected stockpile or dumping stockpile picks Critical, Important, Preferred, Normal or Low; the pane shows it);
-   hierarchical item filters;
-   quality;
-   condition;
-   freshness/taint/etc.;
-   copy/paste.

## Growing settings

-   [x] crop (PLAY-020: "Plant: <crop>" on the selected zone, the architect's list and research locks);
-   [x] sowing toggle (PLAY-020: Allow sowing; harvest goes on; plus Allow cutting);
-   [-] plant requirements (the pane says growing now, growing slowly, too cold or too hot with the zone's temperature and the 6-42 C full-speed range; no fertility or light lines);
-   [-] growth/yield information (sown and ripe counts and average growth; no yield estimate).
-   [x] PLAY-020, as RimWorld does: a selected zone also offers Expand and Shrink (drag-rectangle tools for that zone only, active until right-click or Esc), Delete, Hide (the overlay shows again while selected) and Rename (the label shows the name).

## ENG-003 contract (allowed areas)

-   **Entry:** Architect → Zones → Areas, and a per-pawn "Area" picker
    on the Schedule/Assign screen.
-   **Regions:** list of up to 8 areas (STK-AL, wiki; Home is not one) (name, cell count), paint/clear
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

**RSCH-2 (implemented):** the Research tab (F6) is RimWorld's research screen: a left info panel (name, tech level, description, "X / Y" with ETA, prerequisite links, hidden prerequisites as text, bench and facility in red while missing, unlocks, Start research, "Debug: finish now" in developer mode) and the tech tree on the right (tech-level columns, prerequisite lines, done green, available dark, unavailable grey, the current project outlined and filling with progress; drag, middle-drag, wheel and Shift+wheel pan; search dims non-matches and Enter jumps to the first). Selecting a box lights its prerequisite chain. Code: `windows/research*.go`; widget ids `research.pick.<id>`, `research.tree`, `research.search`, `research.info`, `research.start`, `research.devfinish`, `research.prereq.<id>`.

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

## PLAY-024 contract (trade dialog)

-   **Entry:** a centred window (`trade_ui.go`) that opens while
    `Simulation.TradeView()` reports a session: a colonist ran the Trade
    job to a caravan trader ("Trade with <name>" on right-click), or
    called the orbital ship from the comms dialog. It closes when the
    session ends (Cancel, the trader leaves, the negotiator goes down).
    The display auto-pauses while it is open; the simulation itself keeps
    its own pause state.
-   **Regions:** header (trader name and kind, faction with goodwill, hours
    left, "Orbital" tag); negotiator line (name, Social level, "Price
    modifier" from `PriceModPct`); two-column table, colony on the left,
    trader on the right; footer with the running total, colony silver
    after the deal, trader silver after the deal, the gift toggle with
    projected goodwill, and Reset / Accept / Cancel.
-   **Rows:** one per `game.TradeRowView`, grouped by `Category`, sorted by
    `Label`: colony quantity, a stepper (−all, −10, −1, amount, +1, +10,
    +all), trader quantity, unit price (`SellC` on the colony side, `BuyC`
    on the trader side, shown in silver with two decimals). A positive
    amount buys from the trader (clamped to `TraderQty`, only when
    `WillBuy`); a negative amount sells (clamped to `ColonyQty`, only when
    `WillSell`). A disabled direction greys its half of the stepper with a
    tooltip ("They don't buy that").
-   **Information:** `Simulation.TradeView()` → `game.TradeView`
    (`Trader`, `TraderKind`, `Faction`, `FactionID`, `Orbital`,
    `HoursLeft`, `Negotiator`, `NegotiatorName`, `Social`, `PriceModPct`,
    `Goodwill`, `ColonySilver`, `TraderSilver`, `Rows`, `Reason`).
    Read through the UI-owned `tradeSource` interface. The dialog's own
    amounts are presentation state; totals are computed from the row
    prices for display only — the game settles authoritatively.
-   **Actions / Commands:** Accept sends `command.ProposeTrade` with
    `Name = game.EncodeTradeOffer([]trade.Delta)` (positive Qty: buy;
    negative: sell; zero lines left out) and `Enabled` = gift mode; Cancel
    sends `command.CloseTrade`; Reset clears the amounts locally. In gift
    mode only selling is allowed and the footer shows the projected
    goodwill (`faction.GiftGoodwill` of the offered value) instead of
    silver.
-   **States:** a refused deal shows `TradeView.Reason` in red under the
    footer and keeps the amounts; an accepted one clears them (the rows
    update from the next view). Accept is disabled with a reason when the
    offer is empty, the colony can't afford it, or the trader can't pay.
    `ok == false` closes the window.
-   [ ] Not built yet (PLAY-024 Phase 2, agent U1; the read model and
    commands exist since Phase 0).

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

## PLAY-024 contract (radio console comms dialog)

-   **Entry:** a centred window (`comms_ui.go`) that opens while
    `Simulation.CommsView()` reports a session: a colonist ran the
    UseComms job at a powered radio console ("Use radio console" on
    right-click; refused with a reason when unpowered or the colonist
    can't talk). Close sends `command.CloseComms`.
-   **Regions:** header (operator name, console); one row per faction
    (emblem, name, relation word, goodwill bar −100..100 with the value);
    per row the actions Gift (silver stepper, default 40, with the
    goodwill it earns), Request caravan, Request aid; an orbital ship row
    at the bottom (ship name, trader kind, hours left, Call); the silver
    within beacon reach (`BeaconSilver`) beside the gift stepper.
-   **Information:** `Simulation.CommsView()` → `game.CommsView`
    (`Operator`, `OperatorName`, `Console`, `Factions []CommsFactionView`
    with `ID`, `Name`, `Type`, `Goodwill`, `Relation`, and `Gift`,
    `Caravan`, `Aid` as `CommsActionView{Allowed, Cost, Reason}`;
    `Orbital OrbitalView{Present, Ship, Kind, HoursLeft, CallOK,
    CallReason}`; `BeaconSilver`). Read through the UI-owned
    `commsSource` interface.
-   **Actions / Commands:** Gift → `command.CommsGift` (`Index` faction
    id, `Count` silver); Request caravan → `command.CommsRequestCaravan`
    (`Index`); Request aid → `command.CommsRequestAid` (`Index`); Call →
    `command.CommsCallTrader` (opens the trade dialog); Close →
    `command.CloseComms`.
-   **States:** every action shows its cost ("12 goodwill"); a disabled one
    is greyed with its `Reason` as the label tail and tooltip ("cooling
    down: 2 days", "needs goodwill 0", "friendly factions only", "needs a
    trade beacon", "not enough silver in beacon reach"). Hostile factions
    show only their row and goodwill. With no ship in range the orbital
    row reads "No ship in range". `ok == false` closes the window.
-   [ ] Not built yet (PLAY-024 Phase 2, agent U1; the read model and
    commands exist since Phase 0).

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
-   [x] Wired: F8 opens the full-screen `worldview/` over the running colony (markers for colonies, sites, encounters, destroyed sites, caravans, pods and settlements; picking; inspector with disabled reasons; route preview and ETA; search, legend, grid). `windows/world*.go` is gone.

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
-   [-] Wired: `CaravanView`, `Game.TravelEstimate` and the inspector gizmos Travel, Return, Enter (only when Arrived), Attack (any human base; a non-hostile one asks first, modal `world.attack`, naming the faction, the 200 goodwill and that it turns hostile), Settle and Reform. Missing: Trade (no caravan trading) and Split, which stays disabled ("Not yet").

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

-   [x] Loading and launch are wired (AX-9: Load transport pods dialog, launcher fuel, reasons). Targeting: the engine supports Drop, Drop at centre, Pick spot, Attack (edge or centre), Found, gift and visit on any open map (MAP-2); the world view draws the range ring and the anchored landing menu (edge, centre, attack edge or centre, found, gift, visit, pick spot; Visit is refused without a settlement; Attack on a non-hostile base asks first with the same modal).

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

Close the campaign and reflect the emergent story back to the player. Two endings open it: the ship leaving, and everyone being gone (MAP-14: no colonist left on any map or on the road; prisoners, beasts and the kidnapped don't count).

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

-   continue viewing colony if supported (Keep watching);
-   load a save (the everyone-gone ending);
-   main menu;
-   credits;
-   export/share summary later.

## Everyone-gone modal

-   [x] `ending.gone` (MAP-14): opens by itself when the read model's `AX.EveryoneGone` turns true, and again if colonists came back and left. Buttons `modal.ending.gone.<btn>`: Keep watching (primary), Load game (disabled in a commitment colony, with the reason), Main menu.
-   [x] New wanderers (MAP-19, wiki: Endings): a button on `ending.gone`, disabled with a reason until 8 in-game hours have passed and after 7 groups. While everyone is gone an alert with a countdown stays up and reopens the modal, so Keep watching doesn't lose the option. The button opens a pawn picker with a reroll per slot and Start, which sends `command.NewWanderers`. The chosen person's card shows their portrait, traits and skills with passion glyphs (MAP-22, wiki: Ending).
-   [x] A launch with everyone aboard (MAP-22): `ending.ship` opens first and offers the same New wanderers button; `ending.gone` doesn't come on top, and re-arms if someone arrives.
-   [x] Abandoning the last colony is allowed; its confirm (`world.abandon`) adds that nobody would be left and the game ends, when no caravan, pod or other map holds a colonist.

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

OPT-* (`.agents/gaps/OPTIONS_GAP_ANALYSIS.md`) added the General, Interface
and Developer tabs below.

## Tabs

### General

-   [x] language (OPT-001: lists the catalogs in `assets/lang`; English only for now, every UI string goes through the catalog);
-   [x] restore all defaults, after a confirmation (OPT-002);
-   [x] open the saves folder and the log folder (OPT-003);
-   [x] pause on load (OPT-004);
-   [x] automatic pause: never / major threat / any threat / any letter (OPT-005, replaces pause on threat);
-   [x] autosaves kept, 1–25, rotating (OPT-006).

### Interface

-   [x] temperature unit adds Kelvin (OPT-007);
-   [x] realtime clock; 12-hour clock (OPT-008);
-   [x] animal names on the map: none / named tame / all tame (OPT-010);
-   [x] colonist bar: weapons under portraits (never / drafted / always), mood fill, the bar itself, categorised resources (OPT-011…013);
-   [x] headgear only on the map (OPT-014: the colonist bar portrait leaves the hat off);
-   [x] menu background: diorama / splash / random (OPT-015);
-   [x] far-zoom pawn highlight: none / hostiles / all, as dots / silhouettes / both (OPT-016);
-   [x] zoom to mouse, smooth camera jumps, drag sensitivity, zoom out to the world, remember draw styles, plant sway, screen shake 0–200 % (OPT-021…027);
-   [x] custom cursor (OPT-028);
-   [x] window size presets and borderless fullscreen (OPT-029);
-   [x] speed after unpausing: normal / last speed (OPT-020);
-   [x] preferred names for new colonists (OPT-032).

### Developer

-   [x] verbose log, open the log on warnings, close the log with Escape, pause on error (OPT-030);
-   [x] test map sizes in the wizard (OPT-031).

### Graphics

-   [x] window: fullscreen toggle, size presets and borderless
    fullscreen (OPT-029; the window still resizes freely);
-   [x] VSync (on by default since PERF-003; off from PERF-002 to then) and Frame cap (60 / 120 / 144 / Off; 120 on every display by default, paced by the client while VSync is off — PERF-002; off the colony — menu, wizard, loading — draws never exceed 60 fps, PERF-003);
-   [x] UI scale (UI-024: Accessibility row Auto / 75–200 %; "Sharp on
    high-density screens" (HiDPI) on the Graphics tab; its value reads
    "On, limited to WxH" while the render cap holds the frame below the
    screen's full size — one frame never holds more pixels than 5120x2880
    nor a side above 8192, on any screen, PERF-003);
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
-   [x] World detail (PERF-002: Full / 3/4 / Half of the physical pixels
    the map draws at on a high-density screen, composited with linear
    filtering; the HUD, labels and text stay native; never below one
    pixel per point; preset High Full, Medium 3/4 on 2× displays, Low
    Full; cost tooltip, applied live);
-   [ ] far-zoom detail (OPT-016).

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
-   [x] share detailed usage data (on by default; basic data is always sent, see the README);
-   [ ] speed behavior (OPT-020);
-   [ ] confirmations (OPT-019: deconstruct, release an animal);
-   [x] learning helper (switch and Reset lessons).

### Controls

-   [x] bindings (UI-016: every action by category, paged);
-   [x] rebinding (click a slot, press a key; Esc cancels, Backspace clears);
-   [x] conflicts ("Swap?" before a used key is taken);
-   [x] reset.

### Habits

-   [x] Common Sense habits: one switch per habit and the limits (they belong to the colony and go through typed commands).

### Accessibility

-   [x] colonist card: on or off, where it sits, the pane's height, and where a floating card starts;

-   [x] text/UI scale (UI-024: one UI scale row, Auto or 75–200 %, text included);
-   [ ] color-vision support (OPT-018);
-   [x] reduced effects/flashing (Reduced flashing caps lightning; Reduced
    motion freezes the menu drift, drops embers and dust, and snaps the
    camera);
-   [x] tooltip delay (OPT-017);
-   [x] larger small text (OPT-009);
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
-   [-] time controls (pause / normal 1x / fast 3x / superfast 6x, date/period/clock, current schedule block in top chrome; cursor temperature, biome chip, season, weather and climate event chips present; Shift+T temperature overlay, TEMP-5)
-   [-] selection (click pawn or colonist bar; inspector Character/Needs)
-   [-] Generic Inspector (name, job, status, reason, food/rest bars; Health/Social/Gear disabled)
-   [-] colonist bar (snapshot slots, click selects, idle portrait, display name, status marks, double-click centres; MAP-1: a frame per open map with a threat dot, then caravan groups with a caravan status icon; no custom sort)
-   [-] contextual commands (Architect + Work + Menu live; others disabled `not implemented`; no selection command bar)

## Wave UI-2 --- Build a colony

-   [-] Architect (Wall/Bed/Stockpile/Harvest/Cancel + drag ghosts; no categories/cost cards)
-   [-] placement (drag rect; walls/beds per cell; no rotation/validity colors)
-   [-] zones (stockpile paint food+wood; no grow-zone/home/allowed)
-   [ ] stockpile settings
-   [x] growing settings (PLAY-020)
-   [-] Work (overlay from snapshot; every live work type cycles 1-4 or off, columns left to right in the order a colonist breaks ties: `.agents/gaps/WORK_PRIORITIES_GAP_ANALYSIS.md`; not virtualized)
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
-   [x] Research (RSCH-2)
-   [ ] Assign/Policies
-   [ ] combat controls/targeting on Main Colony
-   [-] environmental overlays (roof, power, fertility, cover and Shift+T temperature done; AX-4 deep ore shows while a deep drill or scanner is selected or its tool is in hand; beauty and room overlays missing)

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
-   [-] Transport Pod (AX-9: §36's loading (dialog, pod and launcher panes) and targeting (World tab picker, pods in flight); no range ring, attack or founding)

## Wave UI-8 --- Campaign completion

-   [-] Endgame Ship (AX-8: the ship parts' inspector pane carries §37's information and actions, with the reactor warning, the launch confirmation and the selected ship's highlight; no dedicated screen)
-   [-] Victory (AX-8: the ending screen with outcome, escaped and remaining colonists, days and research; Keep watching and Main menu; no credits or full story summary)
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

**Power grid overlay (PLAY-021, implemented):**
- **Toggle:** Shift+P or the HUD "Power grid" button (icon `overlay_power`, the second row of the global toggles). It is forced on while a power tool or a powered building is in hand.
- **What it shows:** each net's conduits and switches in its own colour (`ui.power_net1..6`, grey `ui.power_none` for none), hidden conduits included; a link line from each device to the conductor it wires to, in its net's colour, over the wire the map shows all the time in `ui.power_cord` (PWR-1; 6 tiles; hidden conduits get a wire too); a disc on producers, a ring on consumers, a square and charge bar on batteries; the no-power sign on unpowered consumers; a red frame on devices with no grid; a legend in the global stack.
- **Reconnect (PWR-4):** a button `cmd.power.reconnect` (icon `power_reconnect`) in the pane of a producer, consumer or battery. It wires the device to the next grid in range at once (`command.ReconnectPower`, no colonist). Disabled reasons: "No grid in range.", "Touching the grid: no wire to move.", "No other grid in range."

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
