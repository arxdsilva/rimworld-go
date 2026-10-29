# rimworld-go --- UI/UX Implementation Specification & Checklist

> **Companion to:**
> `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`,
> `MAIN_UI_SCREENS_SPECIFICATION.md` (per-screen layout/commands), and
> `ART_DIRECTION_AND_ASSET_PIPELINE.md` (icons, chrome, overlays).
>
> **Purpose:** Define every major player-facing interface required to
> deliver the vanilla colony-simulation experience, explain what each
> interface is for, what information it must expose, what commands it
> may issue, and place the work in an implementation order suitable for
> multiple AI agents.
>
> **Scope:** Base/vanilla RimWorld-style functionality plus the
> `rimworld-go` plugin/mod system. DLC-specific interfaces are
> intentionally excluded.
>
> **Architecture rule:** The UI is an inbound adapter. It displays
> immutable/read-only simulation snapshots and submits typed commands
> through application ports. It must never own authoritative gameplay
> state or mutate domain stores directly.
>
> **Design rule:** Match the *information architecture and usability
> class* of modern RimWorld where that behavior is important to the
> experience, but use original visual design, icons, layout details,
> wording, assets, and branding.

------------------------------------------------------------------------

# 0. How Agents Must Use This Document

This file is ordered by **UI implementation dependency**, not
alphabetically.

An agent may mark `[x]` only when the interface is usable, integrated
with its underlying system, has automated tests where practical, and
satisfies the acceptance criteria in this file.

For every UI task:

1.  Read root `AGENTS.md`.
2.  Read the presentation/UI package `AGENTS.md`.
3.  Read the corresponding feature section in
    `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`.
4.  Read only the relevant read-model and command interfaces.
5.  Keep domain logic out of widgets/controllers.
6.  Make UI state clearly distinct from authoritative simulation state.
7.  Add keyboard/mouse behavior and failure feedback.
8.  Verify large-colony behavior; lists must not become unusable at
    500--1,000 pawns.
9.  If the task is a named screen, window, overlay, or HUD layout, follow
    `MAIN_UI_SCREENS_SPECIFICATION.md` via `rimworld-go-ui-screens`
    (named section plus §0/§58/§59).
10. If the task includes icons, chrome, overlays, portraits, or other
    visual presentation, follow `ART_DIRECTION_AND_ASSET_PIPELINE.md`
    via `rimworld-go-art-assets` (original visual design; do not copy
    RimWorld panel art or iconography).
11. Tick the checkbox only after the UI is functionally complete.

## Status

-   `[ ]` not started
-   `[-]` in progress
-   `[x]` complete
-   `[!]` blocked

------------------------------------------------------------------------

# 1. Global UI Architecture and Design System

Build this before feature-specific screens. Every later interface should
use these primitives.

## 1.1 Presentation architecture

-   [-] **Read-only presentation snapshot** (colony PawnView snapshots; menus are headless)
    -   Create a compact presentation model published by the
        application/simulation boundary.
    -   UI reads entities, map state, alerts, jobs, needs, selections,
        overlays, research, factions, etc. from snapshots.
    -   The renderer must not hold pointers to mutable domain component
        stores.
    -   Snapshot IDs must remain stable enough for selection and
        inspector continuity.
-   [-] **Typed UI command gateway** (session intents plus `Enqueue` MovePawn/designations/SetWorkEnabled; no draft/bills/trade/caravan)
    -   Every gameplay mutation becomes a typed command: move, draft,
        designate, build, assign work, change policy, set bill, trade,
        form caravan, etc.
    -   Commands carry enough information for domain-side validation.
    -   UI may optimistically preview actions but authoritative success
        comes from simulation state.
-   [-] **Presentation-only state store** (screen stack, pause, camera, selection and groups, hover/hint, Architect/Work overlays, bindings, search (UI-016))
    -   Camera.
    -   current selection.
    -   hovered tile/entity.
    -   open dialog.
    -   active global tab.
    -   active architect category.
    -   overlay toggles.
    -   scroll positions.
    -   collapsed groups.
    -   search/filter text.
    -   tooltip state.
    -   UI scale.
    -   key bindings.
    -   None of these should affect deterministic simulation unless
        converted into an explicit command.

## 1.2 Visual design tokens

-   [-] **UI scale and density system** (PLAY-012 UI-024: the UI lays out in logical pixels and draws through a scaling canvas, text at the real font size; Auto plus 75–200 % presets; map coordinates unaffected; PLAY-017: portraits and world labels fixed at every scale, guarded by a per-function draw check and frame-invariance tests, `-capture-ui-scale` for scaled captures; spacing/row heights are still per-file constants, not a central token set; PLAY-018: one layout function places every colony HUD region on a shared grid, with no-overlap tests at three window sizes and three UI scales)
    -   Support multiple UI scales.
    -   Define spacing, row heights, icon sizes, text sizes, border
        widths, modal sizes, and minimum click targets centrally.
    -   Scale must not change simulation or map coordinates.
-   [ ] **Typography hierarchy**
    -   Game title.
    -   window title.
    -   section heading.
    -   body.
    -   small metadata.
    -   alert.
    -   disabled/help text.
    -   Numeric columns should align predictably.
-   [-] **Semantic colors** (menu/pause chrome: space, dust, leather button, selected gold, disabled, dim; PLAY-018 mood tokens; PLAY-021 power grid tokens `ui.power_net1..6`, `ui.power_none`; health/hostile not yet)
    -   Positive.
    -   warning.
    -   danger.
    -   disabled.
    -   selected.
    -   hostile.
    -   friendly.
    -   neutral.
    -   health severity.
    -   mood.
    -   temperature.
    -   power.
    -   forbidden/designated.
    -   Do not encode important state by color alone.
-   [ ] **Icon system**
    -   Original icons for commands, work types, needs, health,
        factions, research, speed, overlays, policies, etc.
    -   Every icon has tooltip/accessibility label.
    -   Icons must be addressable by namespaced content ID so plugins
        can add icons.

## 1.3 Core widgets

-   [-] **Button**
    -   Normal, hover, pressed, disabled, selected/toggled states. (hover/disabled/selected; no pressed-frame or shortcut label)
    -   Tooltip and shortcut label. (disabled-reason line only)
-   [ ] **Icon button**
-   [ ] **Checkbox**
-   [ ] **Radio/select option**
-   [ ] **Slider**
-   [ ] **Dropdown**
-   [ ] **Text input**
-   [-] **Search input** (UI-016: the Z search box; no shared widget yet)
-   [ ] **Numeric input/stepper**
-   [ ] **Scrollable panel**
-   [ ] **Virtualized list**
    -   Required for pawn/item/plugin/definition lists at scale.
-   [ ] **Virtualized table/grid**
    -   Required for Work, Schedule, Assign, Animals, History and debug
        data.
-   [ ] **Tree/filter control**
    -   Used by storage, food, apparel, ingredient and plugin definition
        filters.
-   [ ] **Tabs**
-   [-] **Progress bar** (ART-027: in-world work progress bar under the worked tile for bills, construction, mining, sowing, harvest, tending and cleaning; no shared UI progress widget)
-   [ ] **Stat bar**
-   [ ] **Context menu**
-   [ ] **Confirmation dialog**
-   [ ] **Modal window**
-   [-] **Toast/message** (PLAY-018: messages fade under the colonist bar; toasts stack just above the command buttons or the pane; no shared toast widget)
-   [x] **Persistent alert card**
-   [ ] **Tooltip**
-   [ ] **Rich tooltip/stat breakdown**
-   [ ] **Drag painting tool**
    -   For zones, schedules and priorities.
-   [ ] **Resizable/repositionable developer window primitive**

## 1.4 Input routing

-   [-] **Mouse hit testing**
    -   UI consumes pointer before map interaction. (menus and pause overlay)
    -   Map receives pointer only when not blocked by a panel/modal.
-   [x] **Keyboard focus** (UI-016: input context stack text > modal > tool > panel > map > global > dev; the seed field, search box and key capture swallow keys)
    -   Text fields capture typing without triggering game hotkeys.
    -   Escape backs out one interaction level at a time. (overlay, then stack, then pause)
-   [x] **Shortcut manager** (UI-016: `internal/platform/input` action registry with contexts, defaults, conflicts and the resolver; tooltips and labels read the live keys, e.g. "Draft [R]")
    -   Named actions rather than hardcoded keys.
    -   Display shortcut in tooltips.
    -   Rebinding later uses the same registry.
-   [x] **Drag interaction state** (UI-016: selection box, zone/area drags, build line/rectangle/oval, schedule painting, formation right-drag; no list reorder yet)
    -   Selection rectangle.
    -   zone painting.
    -   build line/area.
    -   schedule painting.
    -   list drag/reorder where appropriate.

------------------------------------------------------------------------

# 2. First Playable HUD: Map, Camera, Time, Selection

This is the minimum UI required to interact with a running simulation.

## 2.1 Main game viewport

-   [-] **Map viewport** (grid, pawns, stockpiles, plants, items, frames, buildings from read models; curved, eased, sub-frame pawn/animal motion) (SIM-018, ART-010: roof overlay and sprite lighting done; ART-012: creatures cast their own animated silhouette shadows; MAPGEN-2: fog of war over unexplored tiles, black on the minimap; PLAY-022: animals drawn at their body's area and picked by any tile of it, walking with planted feet)
    -   Occupies the central play area.
    -   Draw terrain, roofs/visibility treatment, structures, items,
        plants, pawns, effects and selection markers.
    -   Culls off-screen content.
    -   UI panels overlay or reserve space without changing
        authoritative map geometry.
-   [x] **Camera pan** (keyboard WASD/arrow pan with acceleration and smooth coast-to-stop, Shift fast pan, edge scrolling (Options) and middle-mouse drag (UI-016); presentation-only)
    -   Mouse edge/drag and keyboard movement.
    -   Smooth presentation movement.
    -   Camera position is presentation-only.
-   [-] **Camera zoom** (keyboard/wheel zoom clamped, eased toward the target and anchored at the cursor; far-zoom detail simplification not implemented)
    -   Multiple useful zoom levels from tactical detail to colony
        overview.
    -   At far zoom, simplify sprites/details rather than drawing
        expensive fine detail.
    -   Zoom toward cursor where practical.
-   [x] **Jump/center camera** (alerts and messages jump; `,`/`.` select the previous/next colonist and centre on them; search results centre on the pick (UI-016))
    -   Double-click/select alert/entity.
    -   Hotkey to center selected pawn.
    -   Event letters can provide jump target.

## 2.2 Selection interaction

-   [x] **Single-click selection** (UI-011: pawn → animal → item → building → plant → floor under the cursor; empty ground clears)
    -   Prioritize sensible entity under cursor.
    -   Clicking empty terrain selects tile or clears selection
        depending interaction state.
-   [x] **Selection cycling** (UI-011: clicking the same tile again cycles its stack)
    -   When multiple selectable things share a cell, provide
        predictable way to inspect/cycle them.
-   [x] **Drag-select colonists** (UI-016: a left drag boxes colonists, else animals, else items; Shift+click and Shift+drag add/remove)
    -   Rectangle selects player-controllable pawns.
    -   Modifiers add/remove from selection.
-   [x] **Double-click same-type selection** (UI-016: visible colonists, animals of the species, stacks of the item)
    -   Select visible entities of the same relevant type, useful for
        pawns/animals/items.
-   [-] **Selection markers** (yellow outline on the selected pawn; every group member outlined and its colonist-bar slot lit (UI-016); no hostile/friendly differentiation yet)
    -   Clear original visual indicator.
    -   Differentiate single/multiple selection and hostile/friendly if
        needed.

## 2.3 Time controls

-   [x] **Pause button** (`Space` toggles pause; PLAY-018: four clickable speed buttons in the lower-right stack, active one highlighted)
-   [x] **Normal speed** (key `1`; 1x = 60 ticks / real second)
-   [x] **Fast speed** (key `2`; 3x)
-   [x] **Very-fast speed** (key `3`; superfast 6x)
-   [x] **Keyboard speed shortcuts**
-   [x] **Current date/time display** (HUD shows 1-based year/quadrum/day, period, HH:MM:SS and schedule block, plus season, outdoor temperature, weather and wind)
    -   Local game hour.
    -   day.
    -   quadrum/season.
    -   year.
    -   Optional world temperature/weather nearby.
-   [x] **Forced-pause feedback** ("paused (threat)", "Paused (window in background)"; PLAY-017: an animal chasing a colonist pauses with a red message, and an off-screen chase is centred)
    -   If major threats/settings pause the game, visually explain why.

## 2.4 Minimal top-level HUD

-   [-] **Current game state indicator** (PLAY-018: resource list top-left (stored goods, icons, categorised and collapsible); lower-right stack: hour, date and season, weather and wind, temperature where the cursor points, speed with the paused reason, active conditions; no dev-mode indicator)
    -   Paused/speed.
    -   map/world context.
    -   developer mode indicator if enabled.
-   [x] **Message feed anchor** (PLAY-018: right-aligned under the colonist bar, beside the right column)
    -   Short transient messages appear without obscuring play.
-   [x] **Alert anchor** (PLAY-018: right column under the learning helper, above the letter stack)
    -   Reserve a consistent edge of the HUD for persistent colony
        alerts.

------------------------------------------------------------------------

# 3. Selection Inspector and Contextual Command Surface

The inspector is the primary way the player understands one pawn,
building, item, plant, animal, corpse or tile.

## 3.1 Generic selection panel

-   [-] **Selected entity header** (pawn, animal, building, plant and item panels with name, icon, category and condition (UI-011); UI-014 animal inspector adds state, food/rest bars, sex/stage/age, pregnancy %, product bar, diet and tame chance; PLAY-017: plant panel shows why a plant is or isn't growing and when it ripens; no faction/ownership line; PLAY-018: one lower-left inspect pane for every kind (header, compact status bars, chips, scrolling lines) with its tabs opening a panel upward)
    -   Display label/name.
    -   entity category.
    -   faction/ownership where relevant.
    -   health/condition summary.
    -   concise current state.
-   [-] **Entity portrait/icon** (vendor idle frame when pack loaded; otherwise empty)
    -   Original presentation.
    -   For pawns, portrait or representative pawn rendering.
-   [ ] **Description**
    -   Data-driven description.
    -   Plugins can provide localization.
-   [-] **Location and environment** (tile coordinates from `PawnView`; no room/temperature/terrain/roof) (SIM-018: roof line in hover panel; PLAY-018: with nothing selected the hovered cell's terrain, fertility, roof, temperature and room show as text at the lower-left)
    -   Cell.
    -   room/outdoors.
    -   temperature.
    -   terrain.
    -   roof state where useful.

## 3.2 Context command bar

-   [-] **Context-sensitive actions** (UI-011 panels: draft, clear orders, equip nearby; plant harvest/cut and per-colonist now-orders; item equip/wear/eat/haul per colonist; animal hunt/tame/release and (UI-014) milk/shear products; building move (reinstall)/copy/deconstruct; no forbid/arrest/set owner; PLAY-018: every action is a square command button right of the pane (icon, label, hotkey, disabled reason, per-colonist options on right-click); capture/arrest, trough filter and bed owner are commands too; SIM-062: rescue, capture, arrest and prioritize tending on the clicked pawn from the right-click menu)
    -   Draft.
    -   forbid.
    -   hunt.
    -   tame.
    -   harvest.
    -   cut.
    -   deconstruct.
    -   uninstall/reinstall if supported.
    -   prioritize work.
    -   equip.
    -   consume.
    -   arrest/capture/rescue.
    -   set owner.
    -   etc.
    -   Only show actions relevant to selection.
-   [x] **Disabled action explanation** (order menu and placement preview show reasons from the simulation)
    -   Disabled command must explain *why*: incapable, unreachable,
        reserved, missing material, wrong faction, research missing,
        unsafe, etc.
-   [-] **Multi-selection actions** (UI-016: group panel with Draft all/Undraft all, Forbid all/Allow all and the member list; R, F and right-click act on every member; formation moves; no assign area yet; PLAY-018: a group's commands merge (mixed toggles, disabled only when disabled for all))
    -   Draft/undraft.
    -   assign area.
    -   group commands.
    -   shared valid actions only.

## 3.3 Generic stats view

-   [ ] **Stats window**
    -   Searchable list of entity statistics.
    -   Current value.
    -   base value.
    -   modifiers.
    -   final value.
    -   explanation of calculation.
    -   This becomes essential for understanding deep simulation.

------------------------------------------------------------------------

# 4. Pawn Bar and Basic Pawn Inspector

## 4.1 Colonist bar

-   [x] **Always-visible colonist portraits** (snapshot slots, click selects, double-click centres the camera on the colonist (PLAY-017), layered original portrait drawn in its slot at every UI scale, display name)
    -   Compact representation of each colonist.
    -   Click selects; double-click centers.
    -   Preserve identity at a glance.
-   [-] **Status indicators** (PLAY-018: drafted, sleeping, mental break, downed, idle, fighting, fleeing and burning marks on the bar card; no inspired or caravan marks)
    -   Downed.
    -   drafted.
    -   mental state.
    -   severe illness/injury.
    -   sleeping.
    -   caravan/off-map.
    -   break risk.
    -   Icons must remain readable without opening inspector.
-   [-] **Ordering** (snapshot insertion order; no groups or custom sort)
    -   Stable default ordering.
    -   Later allow groups/custom ordering.
-   [x] **Large-colony scaling** (PLAY-018: the bar scales to 0.75 and 0.5 and wraps to 2-3 rows, then a "+N" slot; it never reaches the resource list or the right column)
    -   Collapse, scroll, wrap, group or virtualize rather than
        rendering 1,000 full portraits.
    -   Zoom/density modes.

## 4.2 Pawn summary

-   [-] **Current activity/job** (inspector job line + schedule activity + tile; no draft)
-   [x] **Draft state** (inspector status + Draft/Undraft button)
-   [x] **Walking path** (every colonist's remaining route and destination on the map; drafted brighter; a haul's drop point marked (UI-013))
-   [x] **Event log** (inspector Log tab: what happened to the colonist, newest first, 30 kept; animal inspector: Recent + Log (LOG-001))
-   [ ] **Faction**
-   [-] **Immediate health state** (alive/downed/dead + reason; no injury tree)
-   [x] **Immediate mood state**
-   [ ] **Current need warnings**

------------------------------------------------------------------------

# 5. Architect / Orders UI

This should be built as soon as construction/designations exist. It is
the player's primary world-editing command surface.

## 5.1 Architect root

-   [x] **Architect button/tab** (opens on last category)
    -   Opens build/designation categories.
    -   Remembers last category optionally.
-   [x] **Category navigation** (Orders, Zones, Structure, Furniture — only categories with content; PLAY-018: two category columns at the lower-left, a description box above them)
    -   Orders.
    -   Zone/Area.
    -   Structure.
    -   Furniture.
    -   Production.
    -   Power.
    -   Security.
    -   Miscellaneous/endgame.
    -   Categories appear only when content exists/unlocked.
-   [-] **Build option cards/buttons** (icon, name, wood cost, hover tooltip with work and shortage; no research lock/material; MAPGEN-3: Bridge tool in Structure, locked until the Bridges research; PLAY-018: tools are command buttons right of the categories; the description box shows name, cost, stock, material and crop choice)
    -   Icon.
    -   name.
    -   material requirement.
    -   research lock.
    -   cost.
    -   short tooltip.

## 5.2 Placement preview

-   [x] **Ghost blueprint** (real sprite per cell, walls autotile among ghosts, hover preview before dragging; a door or gate over a wall or fence shows "replaces wall" (UI-021); doors and gates preview their automatic facing per cell, a material change reads "wood wall → steel" and the same material is a neutral grey cell (UI-022))
    -   Show exact occupied cells before command submission.
-   [-] **Validity visualization** (green/red ghosts, first failure reason, count and wood total; reasons: outside, blocked, occupied, planned, items) (SIM-018: unsupported roof tiles marked red ×)
    -   Valid/invalid placement.
    -   Reasons: occupied, impassable, unsupported, missing research,
        wrong terrain, blocked interaction cell, roof/support
        constraint.
-   [ ] **Material/stuff selector**
    -   For buildings that can be made from different materials.
    -   Show available amount and relevant stat differences.
-   [x] **Rotation** (Q/E for bed and chair; facing reaches the blueprint and building)
    -   Rotate directional buildings before placement.
-   [x] **Single-cell placement** (door, furniture)
-   [x] **Line placement** (walls along the dominant drag axis)
    -   Walls/conduits.
-   [x] **Rectangle/area placement** (floor fill; stockpile/harvest/deconstruct/cancel areas)
    -   Floors/zones/roof designations where appropriate.
-   [ ] **Drag continuous placement**

## 5.3 Orders

-   [ ] **Mine**
-   [x] **Cut plants** (trees, bushes and grass (UI-011))
-   [x] **Harvest** (wild plants via Architect; ripe zone crops get the mark automatically (PLAY-017))
-   [x] **Hunt**
-   [x] **Deconstruct**
-   [x] **Cancel** (blueprints, harvest and deconstruct marks, zones)
-   [x] **Forbid/unforbid** (Orders › Forbid/Allow area tools, the item panel button and F on the selection, UI-016)
-   [x] **Haul things** (PLAY-017: Orders › Haul things marks items found on the map for hauling; item panel and group buttons, "Found here" line, a haul mark on the map; Cancel unmarks)
-   [ ] **Remove floor**
-   [x] **Remove roof** (SIM-018)
-   [ ] **Claim structure where base-game semantics require it**
-   [x] **Smooth stone/floor equivalents if supported** (Orders › Smooth: rock walls without ore and rough stone floors, UI-017/SIM-029)

## 5.4 Designation feedback

-   [x] **Designation overlay** (blueprints are translucent blue ghosts, scaffold once wood arrives; deconstruct marker on buildings)
    -   Clearly distinguish planned work from completed structures.
-   [x] **Resource requirement feedback** (card cost + colony wood, placement total, `Haul wood to X (n/m)` in the order menu; UI-015: frame inspector shows delivered/needed, on the way, stock and craft hints; carpet card shows stock; "Blueprints waiting for materials" alert)
-   [x] **Cancel designation interaction** (Orders → Cancel area)
-   [x] **Selection inspector for blueprint/frame** (UI-015: status, materials, work estimate, haulers, builder, footprint, Cancel blueprint with refund)

------------------------------------------------------------------------

# 6. Zones and Areas UI

## 6.1 Zone tool

-   [x] **Create stockpile** (Architect paint, tile by tile: skips tiles another zone owns; the drag ghost marks each tile green or red (ZONE-001); PLAY-023: the cursor names a refused cell's reason, a refused drag hints the most common one; Dumping stockpile tool)
-   [x] **Create growing zone** (crop picker incl. tree zones (ZONE-001); PLAY-023: soil-only placement with per-cell reasons)
-   [x] **Expand zone** (painting next to a zone of the same kind, and for grow zones the same crop, extends it; a drag bridging two merges them; PLAY-020: a selected zone's Expand tool adds a dragged rectangle to that zone)
-   [x] **Shrink/delete zone** (the Cancel tool clears just its tiles; a zone cut in two becomes two zones; PLAY-020: a selected zone's Shrink tool removes only its own tiles, and Delete removes it)
-   [x] **Select existing zone** (PLAY-020: a click on an empty zone tile or its label selects it, repeat clicks on a crop cycle to it; the pane shows size, crop, growing now or too cold/hot, sown and ripe counts, average growth; commands Plant, Allow sowing, Allow cutting, Expand, Shrink, Delete, Hide, Rename)
-   [x] **Zone color/outline** (borderless fill per tile plus one outline along the zone's outer edge: stockpile dashed gold, grow zone solid green, pen fence)
    -   Distinct but unobtrusive.
-   [x] **Zone labels** (PLAY-017: each zone named once on a plate, pinned to the zone at every UI scale, sized with the zone, readable zoomed out, capped smaller zoomed in and drawn over stacks and crops)

## 6.2 Stockpile configuration panel

-   [ ] **Stockpile name**
-   [x] **Storage priority selector** (PLAY-023: Priority picker and pane line on stockpiles and dumps)
-   [ ] **Allowed-items filter tree**
    -   Hierarchical categories.
    -   Allow all/disallow all.
    -   Search.
    -   Individual definitions.
-   [ ] **Quality range**
-   [ ] **Hit-point/condition range**
-   [ ] **Special filters**
    -   Rotten/fresh, tainted/clean, etc. where relevant.
-   [ ] **Copy/paste settings**
    -   Important quality-of-life feature.

## 6.3 Growing-zone configuration

-   [-] **Crop selector** (the architect's crop picker and, PLAY-020, a selected zone's "Plant: <crop>" list with research locks; no search, skill, growth time, sensitivity or yield columns)
    -   Searchable plant list.
    -   required skill.
    -   growth time.
    -   fertility sensitivity.
    -   expected yield.
    -   current research lock.
-   [x] **Allow sowing toggle** (PLAY-020: per zone; harvest goes on; plus Allow cutting)
-   [x] **Plant-cut/harvest behavior feedback**

## 6.4 Home area

-   [x] **Home area overlay** (SIM-021: blue tint, HUD toggle, held on while a Home tool is in hand)
-   [x] **Expand/shrink home area** (Architect > Zones > Home / Clear home)
-   [-] **Automatic home-area expansion setting** (Home always grows around new buildings; no setting to turn it off)
    -   If implemented.
-   [ ] **Explain effects**
    -   Firefighting, cleaning and repair relevance.

## 6.5 Allowed areas

-   [x] **Area manager** (SIM-046; PLAY-015 UI-025: create, rename, invert, delete with confirm, colour swatches; opened from Architect, Schedule, Animals, Assign)
    -   Create.
    -   rename.
    -   delete.
    -   invert.
    -   clear.
-   [x] **Paint allowed area** (PLAY-015: Expand/Clear allowed area submenus list the areas; tinted overlay while painting)
-   [x] **Area overlay selector** (PLAY-015: the armed or hovered area shows in its colour)
-   [x] **Assign pawn/animal to area** (PLAY-015: Schedule tab dropdown per colonist, Animals tab Area/Pen column, pawn inspector line; Unrestricted, Home or an area)
    -   Integrated later with Schedule/Assign/Animals.

------------------------------------------------------------------------

# 7. Work UI

The Work screen is one of the most important colony-management
interfaces.

## 7.1 Work table

-   [-] **Pawn rows** (snapshot rows with display names; no portraits/status/sort in the table)
    -   Name/portrait.
    -   relevant status.
    -   sortable.
-   [-] **Work-type columns** (Grow/Bld/Haul clickable Y/-; other columns `-`; ENG-003 Warden column, off by default; no patient/bed rest/basic/art/plant-cut columns)
    -   Firefight, patient, doctor, bed rest, basic, warden, handle,
        cook, hunt, construct, grow, mine, plant cut, craft categories,
        art, haul, clean, research.
    -   Column header icon/name.
    -   Tooltip explains work category.
-   [-] **Simple priorities mode** (superseded by manual priorities; `SetWorkEnabled` still maps on/off to 3/off)
    -   Toggle work type on/off.
-   [x] **Manual priorities mode** (Work tab cells cycle off/1/2/3/4, shaded by urgency)
    -   Priority values equivalent to 1--4.
    -   Click cycles values.
    -   Drag paints a priority across cells.
    -   Disabled/incapable cells are visually distinct.
-   [ ] **Skill/passions in cells**
    -   Show relevant skill and passion in compact form without
        overwhelming the grid.
-   [ ] **Incapacity explanation**
    -   Hover reveals backstory/trait/health reason.

## 7.2 Scaling

-   [ ] **Sort by name/skill/current priority**
-   [ ] **Filter/search pawns**
-   [ ] **Pin/freeze work headers**
-   [ ] **Virtualized rows/columns**
-   [ ] **Group assignment**
    -   Later allow applying priorities to selected group.
-   [ ] **Copy/paste work profile**

------------------------------------------------------------------------

# 8. Schedule UI

## 8.1 Schedule grid

-   [ ] **24-hour columns**
-   [ ] **Pawn rows**
-   [x] **Schedule modes** (anything, work, recreation, sleep brushes)
    -   Anything.
    -   Work.
    -   Sleep.
    -   Recreation.
-   [x] **Paint schedule** (click or drag across hour cells, per colonist or Everyone; current hour outlined)
    -   Select mode then drag cells.
-   [ ] **Copy schedule**
-   [ ] **Multi-pawn painting**
-   [ ] **Current hour indicator**

## 8.2 Schedule explanations

-   [ ] **Tooltip for each mode**
    -   Explain that schedule influences autonomous intent rather than
        being a hard script in emergencies.
-   [-] **Current pawn schedule in inspector** (selected pawn shows the current hour's activity; no full schedule grid yet)
-   [ ] **Group schedule presets**
    -   Later large-colony feature.

------------------------------------------------------------------------

# 9. Needs, Mood, Thoughts and Mental-State UI

## 9.1 Needs tab

-   [-] **Need bars** (food and rest 0–100; no recreation/comfort/beauty/outdoors)
    -   Food.
    -   rest.
    -   recreation.
    -   comfort.
    -   beauty.
    -   outdoors.
    -   chemical/addiction needs when applicable.
-   [ ] **Threshold markers**
-   [ ] **Current value and trend**
-   [ ] **Tooltip**
    -   Explain what raises/lowers the need and current modifiers.

## 9.2 Mood

-   [x] **Mood bar**
    -   Current mood.
    -   target mood if smoothed.
    -   break-risk thresholds.
-   [ ] **Mood trend**
-   [ ] **Break-risk severity label**

## 9.3 Thought list

-   [x] **Positive thoughts**
-   [x] **Negative thoughts**
-   [x] **Mood contribution**
-   [ ] **Remaining duration for memories where appropriate**
-   [ ] **Stacking/count**
-   [ ] **Situational explanation**
    -   Example: room, environment, hunger, pain, social event.

## 9.4 Mental state

-   [ ] **Active mental state banner**
    -   What the pawn is doing.
    -   whether player control is unavailable.
    -   likely end condition/duration if knowable.
-   [ ] **Inspiration indicator**
    -   Type.
    -   effect.
    -   expiration/use state.

------------------------------------------------------------------------

# 10. Health UI

The Health screen must make a complex anatomy/condition model
understandable.

## 10.1 Health overview

-   [-] **Overall state** (MED-01: Health-tab state line, e.g. "Bleeding 140%/day · death in 9h"; no summary of diseases or conditions yet)
    -   Healthy/injured/sick/downed/dead.
-   [x] **Pain** (UI-022: Health tab)
-   [x] **Blood loss** (MED-01: blood-loss row with a bar in the Health tab)
-   [x] **Consciousness** (Health tab capacity row)
-   [x] **Moving** (Health tab capacity row)
-   [x] **Manipulation** (Health tab capacity row)
-   [x] **Sight/hearing/talking and other capacities** (Health tab capacity rows)
-   [x] **Bleeding rate / time-to-danger where useful** (MED-01: bleed per day and death-in hours in the state line, per-part bleed in tooltips, "Bleeding (death in Nh)" pawn status, danger alert at ≤ 12 h)

## 10.2 Body-part tree

-   [-] **Hierarchical anatomy** (parts listed per body part; not a collapsible tree)
    -   Torso/head/limbs/organs/subparts.
-   [x] **Injury attached to part** (Health tab part rows)
-   [x] **Missing/destroyed part** (Health tab part rows)
-   [x] **Artificial/replaced part** (prosthetics shown on the part rows)
-   [x] **Part health/severity** (Health tab part rows)

## 10.3 Conditions

-   [x] **Disease/health-effect list** (MED-02/MED-04: Health-tab conditions section lists infections and diseases with stage, severity, immunity, tend window, and tending progress for parasites; disease alert and outbreak letter)
    -   Name.
    -   severity.
    -   stage.
    -   immunity.
    -   treatment quality.
    -   progression.
-   [x] **Immunity vs disease visualization** (MED-02: red severity bar beside a green immunity bar, "needs tend in Nh" marker, losing/winning in the pawn status and an infection alert)
    -   Easy to compare which is winning.

## 10.4 Medical settings

-   [x] **Maximum medical care selector** (MED-03: five care buttons in the Health-tab header; Assign screen Medical column)
-   [x] **Self-tend toggle** (MED-03: Health-tab checkbox for colonists)
-   [x] **Bed-rest/medical-bed state** (SIM-063: the inspector shows "Resting in a medical bed" / "On bed rest", a guest's "Guest of <faction>" or "Stranger"; SIM-066: the mental break names its target)
-   [ ] **Doctor/treatment status**

## 10.5 Operations

-   [x] **Operations list** (Health tab operations queue)
-   [x] **Add operation dialog** (SIM-049: every valid operation listed incl. replacing healthy parts, unavailable ones greyed with the reason; MED-06: harvest, install organ, euthanize, anesthetize; queued rows show the success chance with a factor breakdown)
    -   Valid surgeries only.
    -   Required medicine/body part/item.
    -   Risk information.
-   [x] **Queued operations** (Health tab)
-   [x] **Cancel/reorder operation** (remove works; MED-03: up/down arrows on each queued operation)

## 10.6 Rich explanation

-   [x] **Capacity breakdown tooltip** (MED-10: each capacity row explains its value: affected parts and implants, consciousness cap, blood pumping/filtration, pain, blood loss, condition modifiers)
    -   Show which injuries/body parts/effects cause final capacity.
-   [x] **Treatment tooltip** (MED-02: each wound's tend quality and the tend window in part-row tooltips; condition tooltips show the last tend quality)
    -   Tend quality, remaining duration, medicine used.
-   [x] **Prognosis information** (MED-01/02: death clock for bleeding and losing infections; MED-06: operation success chance with factors)
    -   Avoid omniscient information the simulation does not intend to
        reveal.

------------------------------------------------------------------------

# 11. Character/Bio and Skills UI

## 11.1 Character tab

-   [ ] **Name**
-   [ ] **Age**
-   [ ] **Childhood**
-   [ ] **Adulthood**
-   [ ] **Traits**
-   [ ] **Work incapabilities**
-   [ ] **Faction/origin**
-   [x] **Relationship summary** (UI-022: Bio tab relationship summary)

## 11.2 Skills

-   [ ] **Skill list**
    -   All core skills.
-   [ ] **Level**
-   [ ] **XP progress**
-   [ ] **Passion indicator**
-   [ ] **Learning rate**
-   [ ] **Relevant disabled work**
-   [ ] **Skill tooltip**
    -   Explain gameplay effects at current level.

## 11.3 Rename/customization

-   [ ] **Rename colonist**
    -   Nickname/allowed identity fields.
    -   Presentation command goes through application if persisted.

------------------------------------------------------------------------

# 12. Gear, Equipment, Apparel and Inventory UI

## 12.1 Gear tab

-   [x] **Equipped weapon**
    -   Name.
    -   quality.
    -   condition.
    -   key combat stats.
-   [ ] **Worn apparel**
    -   Layer/body coverage.
    -   armor.
    -   insulation.
    -   quality.
    -   condition.
-   [ ] **Inventory**
    -   Carried items and quantities.
-   [ ] **Carrying mass/capacity if relevant**

## 12.2 Gear actions

-   [ ] **Drop**
-   [x] **Equip**
-   [ ] **Wear**
-   [ ] **Consume/use where valid**
-   [ ] **Inspect item**

## 12.3 Combat-stat tooltips

-   [ ] **Weapon accuracy/range/warmup/cooldown**
-   [ ] **Melee damage/cooldown**
-   [ ] **Armor by damage category**
-   [ ] **Temperature insulation**
-   [ ] **Movement/work penalties from gear**

------------------------------------------------------------------------

# 13. Drafted Combat UI

## 13.1 Draft controls

-   [x] **Draft toggle** (inspector button + R)
    -   Prominent when colonist selected.
-   [x] **Draft state in colonist bar**
-   [x] **Multi-draft** (UI-016: R or Draft all on a group; drafted groups spread around a right-click or line up along a right-drag)

## 13.2 Tactical orders

-   [x] **Move order** (drafted: right-click moves and holds)
    -   Right-click valid destination.
-   [x] **Attack order** (drafted: right-click a raider or animal opens Shoot / Melee attack (UI-013))
    -   Hostile target.
-   [x] **Melee attack**
-   [x] **Ranged attack**
-   [ ] **Fire-at-will toggle**
-   [ ] **Hold position behavior if implemented**
-   [x] **Emergency tend** (MED-03: drafted right-click "Tend <name>" on a wounded colonist or prisoner)
-   [x] **Rescue/capture contextual commands** (SIM-062: select a colonist, right-click a pawn: "Rescue <name>", "Capture <name>", "Arrest <name>", "Prioritize tending <name>" (undrafted) or "Tend <name>" (drafted); disabled rows give the reason (no free bed, no free prisoner bed, carried by, cannot reach); Shift queues; journeys `TestJourneyOrders{RescueDowned,CaptureFromMenu,ArrestFromMenu,PrioritizeTend,RescueQueued}`)

## 13.3 Targeting feedback

-   [ ] **Weapon range overlay**
-   [ ] **Line-of-sight line**
-   [ ] **Target highlight**
-   [ ] **Cover visualization**
-   [ ] **Hit-chance tooltip**
    -   Break down shooter, weapon, range, cover, weather/light and
        target modifiers.
-   [ ] **Invalid shot explanation**

## 13.4 Combat feedback

-   [x] **Damage indicators** (floating -N over hit pawns, animals and buildings; "miss" for missed shots)
-   [x] **Projectile/impact feedback**
-   [x] **Downed/death feedback**
-   [x] **Threat markers** (red chevron over raiders, orange while fleeing; Raid alert and arrival message; PLAY-017: every hostile animal gets the red marker and a red chase line to its target; the hostile alert names who is chased)
-   [-] **Hostile faction coloring/icons** (raiders wear a reserved maroon cloth tint)
-   [x] **Pause-on-threat option** (on by default, Options screen toggle; HUD reads "paused (threat)")

------------------------------------------------------------------------

# 14. Production Workbench and Bill UI

## 14.1 Workbench inspector

-   [x] **Power/operational state** (PLAY-021: the building pane says Uses/Produces N W, the grid balance and stored energy, a switch Closed/Open, a thermostat target and whether it works now, flick pending; the Power toggle (V))
-   [-] **Interaction status** (work spot outlined on selection)
-   [ ] **Current worker/progress**
-   [x] **Bills list** (select a bench: bills with count/mode/status)

## 14.2 Bill stack

-   [-] **Add bill** (one button per recipe for that bench; no search)
    -   Search/filter recipes.
    -   show research/material constraints.
-   [ ] **Reorder bills**
-   [x] **Suspend bill**
-   [x] **Delete bill**
-   [ ] **Copy/paste bills**

## 14.3 Bill configuration

-   [x] **Do X times**
-   [x] **Do forever**
-   [x] **Do until X**
-   [x] **Target count** (-/+ buttons)
-   [ ] **Pause/resume thresholds if supported**
-   [ ] **Ingredient filter tree**
-   [ ] **Ingredient search radius**
-   [ ] **Allowed material/stuff**
-   [ ] **Quality/condition restrictions**
-   [ ] **Worker skill range**
-   [ ] **Specific worker**
-   [ ] **Product destination**
    -   Drop.
    -   best stockpile.
    -   selected stockpile if supported.

## 14.4 Bill explanation

-   [ ] **Missing ingredient reason**
-   [ ] **No valid worker reason**
-   [x] **No power reason** (PLAY-021: not connected, brownout, switched off, solar flare, no fuel; in the warning colour, and a no-power badge on the map)
-   [x] **Research locked reason** (architect tools, crops and bills greyed with "Needs research: X" on hover)
-   [ ] **Output-count explanation**

------------------------------------------------------------------------

# 15. Research UI

## 15.1 Research screen

-   [ ] **Technology graph/tree**
    -   Nodes.
    -   prerequisites.
    -   dependency lines.
    -   zoom/pan.
    -   search.
-   [x] **Research project states** (done / researching / available / needs X, progress bar)
    -   locked.
    -   available.
    -   selected.
    -   in progress.
    -   completed.
-   [ ] **Current project**
    -   progress.
    -   cost.
    -   estimated work context where useful.

## 15.2 Project detail panel

-   [ ] **Name/description**
-   [ ] **Prerequisites**
-   [ ] **Required bench/facilities**
-   [ ] **Unlock list**
    -   Buildings.
    -   recipes.
    -   items.
    -   systems.
-   [x] **Select research command** (click a project in the Research tab)

## 15.3 Plugin support

-   [ ] **Dynamic graph layout**
    -   Plugin projects must integrate without hardcoded coordinates
        where possible.
-   [ ] **Namespaced provenance in developer mode**

------------------------------------------------------------------------

# 16. Power, Temperature, Roof and Environmental Overlays

## 16.1 Overlay framework

-   [-] **Overlay toggle registry** (UI-016: Roof (Shift+R), Home and Room stats (G) toggles; PLAY-018: eight icon toggles in the lower-right stack (roof, home, auto home, plans, rooms, learning helper, colonist bar, categorised resources); no shared registry or plugin API yet)
    -   Only one or compatible multiple overlays active.
    -   Plugins can register overlays through safe API.

## 16.2 Power overlay

-   [x] **Power conduits/networks** (PLAY-021: Shift+P "Power grid" overlay, auto-shown while placing power buildings; each net in its own colour, hidden conduits included)
-   [x] **Powered/unpowered buildings** (PLAY-021: the no-power sign on unpowered consumers; glow layers only while running)
-   [x] **Producer/consumer state** (PLAY-021: a disc on producers, a ring on consumers, link lines to their conduit)
-   [x] **Battery charge** (PLAY-021: a charge bar on the battery sprite and in the overlay)
-   [-] **Network production vs consumption** (PLAY-021: in each device's pane, "Grid: +X W (G made, L used), stored S of C Wh"; not drawn in the overlay)
-   [x] **Disconnected network visualization** (PLAY-021: grey for conduits on no grid, a red frame on devices with no link, a "Not connected" alert)

## 16.3 Temperature overlay

-   [ ] **Room temperatures**
-   [ ] **Outdoor temperature**
-   [ ] **Heat/cold gradient or labels**
-   [ ] **Selected room thermal details**
-   [ ] **Dangerous temperature highlighting**

## 16.4 Roof overlay

-   [x] **Roofed** (SIM-018)
-   [x] **Unroofed** (SIM-018: left unmarked by the overlay; hover line "Unroofed", with "room too large to auto-roof")
-   [x] **Mountain/thick roof distinction where gameplay requires it** (MAPGEN-2: hover "Thick rock overhead (permanent)"; the dev Map fields overlay shows none/thin/thick)
-   [x] **Build/remove roof designation** (SIM-018)

## 16.5 Fertility overlay

-   [ ] **Terrain fertility**
-   [ ] **Growing-zone usefulness**

## 16.6 Beauty overlay

-   [ ] **Beauty values**
-   [ ] **Selected pawn perceived beauty**
-   [x] **Room beauty/impressiveness**

## 16.7 Room overlay

-   [-] **Room boundary** (UI-016: the G overlay tints each room by impressiveness; no outline)
-   [x] **Room role**
-   [ ] **Space**
-   [ ] **beauty**
-   [x] **cleanliness** (SIM-021 room labels and hover panel; UI-016 room stats card)
-   [ ] **wealth**
-   [x] **impressiveness**

------------------------------------------------------------------------

# 17. Assign / Policies UI

This screen centralizes per-pawn behavioral policies.

## 17.1 Assign table

-   [ ] **Pawn rows**
-   [ ] **Food restriction**
-   [ ] **Apparel policy**
-   [-] **Drug policy** (SIM-040: preset selector on the Assign tab (F3); no policy manager)
-   [ ] **Medical care**
-   [x] **Allowed area where chosen UX places it** (SIM-046: Assign tab)
-   [x] **Bed ownership shortcut where useful** (SIM-042: bed owner assignment UI)

## 17.2 Policy manager

-   [ ] **Create policy**
-   [ ] **Rename**
-   [ ] **Duplicate**
-   [ ] **Delete**
-   [ ] **Assign to multiple pawns**

## 17.3 Food policy editor

-   [ ] **Hierarchical food filter**
-   [ ] **Allow/disallow**
-   [ ] **Search**
-   [ ] **Food category explanation**

## 17.4 Apparel policy editor

-   [ ] **Allowed apparel**
-   [ ] **Quality range**
-   [ ] **Condition range**
-   [ ] **Tainted allowance**
-   [ ] **Automatic replacement behavior explanation**

## 17.5 Drug policy editor

-   [ ] **Per-drug row**
-   [ ] **Recreational permission**
-   [ ] **Scheduled frequency**
-   [ ] **Mood threshold**
-   [ ] **Recreation threshold**
-   [ ] **Carry quantity**

## 17.6 Medical policy

-   [x] **Care tier selector** (MED-03: per pawn in the Health tab and Assign screen; "Default medical care" row per group on the Assign screen)
-   [-] **Clear icon/tooltip for medicine level** (MED-03: text buttons with tooltips; icons still to come)

------------------------------------------------------------------------

# 18. Social UI

## 18.1 Social tab

-   [x] **Known relationships** (SIM-042: lover, spouse, ex-lover, ex-spouse; UI-022: relations table with perspective labels; no ex/lover until romance (M5))
    -   Parent.
    -   child.
    -   sibling.
    -   lover/spouse.
    -   ex.
    -   friend/rival where modeled.
-   [x] **Directional opinion** (UI-022: A->B big, (B->A) beside it, 5-band colour)
    -   Selected pawn's opinion of each known pawn.
-   [x] **Relationship status** (UI-022: tags off-map, dead, prisoner, kidnapped, in caravan)
-   [x] **Recent social memories** (UI-022: social log with icons and opinion breakdown; PLAY-020: the bubble over chatting colonists is just the round interaction icon, no box)
-   [x] **Off-map relation marker** (UI-022)

## 18.2 Opinion explanation

-   [x] **Opinion breakdown tooltip** (UI-022: both directions)
    -   Relationship.
    -   traits.
    -   social memories.
    -   injuries/disfigurement modifiers only if game models them.
-   [x] **Reciprocal opinion** (UI-022)
    -   Clearly distinguish A→B from B→A.

## 18.3 Relationship events

-   [x] **Romance notification** (SIM-042/048: history entries, mood memories and a letter for new lovers; letters also for a rejected proposal and an affair)
-   [x] **Engagement/marriage** (SIM-042/048: gathering banner, history entries; engagement, wedding and party letters)
-   [x] **Breakup** (SIM-042/048: history entries; breakup and divorce letters)
-   [x] **Social fight** (SIM-043/048: fight marker, history entries and a social fight letter)
-   [x] **Relation death** (UI-022: History tab (F9) entries click to select)
    -   Link messages to relevant pawns.

------------------------------------------------------------------------

# 19. Animals and Wildlife UI

## 19.1 Animals tab

-   [x] **Owned animal table** (UI-014, SIM-047; PLAY-015: Master dropdown, Follow drafted toggle, Area/Pen column (area dropdown or pen name/Roaming/Being roped), health and training columns)
    -   Name.
    -   species.
    -   sex.
    -   age.
    -   health.
    -   master.
    -   area/pen.
    -   training.
    -   pregnancy/product state where applicable.
-   [x] **Slaughter toggle**
-   [x] **Medical care** (MED-09: animal inspector health block with bleeding, blood loss, infection vs immunity and "needs tending"; Care picker per tame animal; Animals table health cell flags bleeding or infection)
-   [ ] **Rename**
-   [x] **Master assignment** (SIM-047)

## 19.2 Training matrix

-   [x] **Obedience** (SIM-047)
-   [x] **Release**
-   [ ] **Rescue**
-   [ ] **Haul**
-   [x] **Current training progress** (SIM-047: steps, cooldown, decay)
-   [ ] **Trainability/ineligible explanation**

## 19.3 Pen UI

-   [x] **Pen marker inspector** (PLAY-015: Animals (species, male/female), Food (grazing vs demand, trough stock, verdict), Auto-cut)
-   [x] **Pen boundary visualization** (brown wash, fence rails and posts; ART-014: buildable wood/stone fences and gates with autotiled art)
-   [x] **Enclosure validity** (PLAY-015: pen outline when selected; open pens dashed red "Not enclosed")
-   [x] **Assigned species** (PLAY-015: per-species and per-sex filters)
-   [x] **Nutrition/grazing capacity** (PLAY-015: grazing per day vs herd demand in the pen inspector)
-   [-] **Animal count vs food capacity** (UI-014: pen full alert only; no capacity ratio)
-   [x] **Missing gate/breach warning** (PLAY-015: "Pen N is open" alert)

## 19.4 Wildlife tab

-   [x] **Species rows** (UI-014: Animals tab Colony/Wild toggle, Wild species list)
-   [x] **Count**
-   [x] **Predator/herbivore** (diet and predator flag)
-   [-] **Hunt risk** (predator flag only; no risk estimate)
-   [-] **Tame chance/wildness** (tame % shown; no wildness stat)
-   [-] **Body size/meat yield context** (meat yield shown; no body size)
-   [x] **Hunt designation toggle** (Hunt all / Tame one / Cancel per species)
-   [ ] **Sort/filter**

------------------------------------------------------------------------

# 20. Prisoner UI

## 20.1 Prisoner tab

-   [x] **Prisoner status** (SIM-045: Prisoner tab)
-   [ ] **Faction**
-   [x] **Resistance** (SIM-045)
-   [-] **Recruitment difficulty** (SIM-045: resistance only; no difficulty estimate)
-   [x] **Mood** (SIM-045)
-   [x] **Health**
-   [x] **Interaction mode** (SIM-045)

## 20.2 Interaction modes

-   [x] **No interaction** (SIM-045)
-   [x] **Reduce resistance** (SIM-045)
-   [x] **Recruit** (SIM-045)
-   [x] **Release** (SIM-045)
-   [x] **Execute where supported** (SIM-045)
-   [x] **Medical/operation controls through Health UI** (MED-06: prisoner and guest Health tabs queue operations; MED-03 care levels)

## 20.3 Warden feedback

-   [ ] **Last interaction**
-   [x] **Resistance/recruitment progress** (SIM-045)
-   [ ] **Social skill effect**
-   [ ] **No warden/food/bed warning**

------------------------------------------------------------------------

# 21. Trade UI

## 21.1 Trade initiation

-   [x] **Context action on trader/communications console** (PLAY-024: "Trade with X" and "Use radio console" orders, greyed with reasons)
-   [ ] **Select negotiator**
    -   Show relevant social/trade stat.

## 21.2 Trade dialog

-   [x] **Two-sided inventory table** (PLAY-024: colony and trader quantities per row, grouped by category)
    -   Colony goods.
    -   trader goods.
    -   quantity.
    -   buy price.
    -   sell price.
    -   quality/condition.
-   [x] **Trade quantity control** (PLAY-024: ±1 / ±10 / all steppers clamped to stock)
    -   Keyboard/mouse efficient.
    -   buy/sell direction visually obvious.
-   [ ] **Category filters**
-   [ ] **Search**
-   [ ] **Sort**
    -   name/value/category/quantity/quality.
-   [x] **Currency balance** (PLAY-024: colony and trader silver after the deal, red when short)
    -   Current silver.
    -   net transaction.
    -   post-trade balance.
-   [x] **Negotiator modifier** (PLAY-024: negotiator, Social and price modifier in the header)
-   [x] **Accept/reset/cancel** (PLAY-024)
-   [x] **Validation** (PLAY-024: Accept disabled with a reason; a refused deal shows why)
    -   Trader cannot afford.
    -   colony cannot afford.
    -   unavailable/changed stock.

## 21.3 Gift mode

-   [x] **Switch trade/gift** (PLAY-024: gift toggle)
-   [x] **Projected goodwill** (PLAY-024: "Goodwill x -> y" in gift mode)
-   [ ] **Confirm gift**

------------------------------------------------------------------------

# 22. Factions UI

## 22.1 Factions screen

-   [ ] **Faction list**
    -   Name.
    -   icon.
    -   tech level.
    -   relationship.
    -   goodwill.
    -   permanent hostility.
-   [ ] **Sort/filter**
-   [ ] **Faction detail**
    -   description.
    -   settlements.
    -   recent relationship changes.
    -   available communication.

## 22.2 Goodwill explanation

-   [ ] **Current goodwill**
-   [ ] **Hostile/neutral/ally thresholds**
-   [ ] **Recent modifiers/history**
-   [ ] **Permanent hostility explanation**

## 22.3 Communication

-   [x] **Call faction** (PLAY-024 comms dialog)
-   [x] **Request trader** (PLAY-024)
-   [x] **Request military aid** (PLAY-024 phase 4: allies arrive, fight, leave; "Allies on the map" alert; inspector says "Ally")
-   [x] **Offer gift** (PLAY-024)
-   [x] **Unavailable action reason/cost** (PLAY-024: every disabled comms action shows its cost and reason)

------------------------------------------------------------------------

# 23. Event Letters, Messages and Alerts

These are critical to RimWorld's storytelling loop: the game must tell
the player what just happened and what requires attention.

## 23.1 Messages

-   [x] **Transient message stack** (PLAY-018: up to five, right-aligned under the colonist bar, fading out; danger in red; click focuses the camera)
    -   Brief.
    -   non-blocking.
    -   severity.
    -   optional world target.
-   [ ] **Rate limiting/coalescing**
    -   Repeated identical events do not spam the player.

## 23.2 Letters

-   [x] **Letter stack** (PLAY-018: right column above the time controls; kind colour and icon (threat, caution, info, good); kept and saved until right-click dismissal; "+N older"; major threats are letters too)
    -   Persistent until opened/dismissed.
    -   Categories such as positive, neutral, negative and major threat.
-   [-] **Letter window** (NG-1: title, text and Continue; PLAY-018: opened from the stack (only the intro opens by itself), with Jump to for a place or creature; no faction links or choices)
    -   Title.
    -   generated original narrative.
    -   relevant entities/faction.
    -   jump-to-location.
    -   action choices if event requires decision.
-   [x] **Auto-pause option** (Options › Gameplay "Pause on threat": threat letters and an animal chasing a colonist pause the game)
-   [ ] **Letter history**

## 23.3 Alerts

-   [x] **Persistent alert list**
    -   Critical issues remain until resolved.
-   [x] **Severity ordering** (PLAY-018: danger alerts first, with a pulsing backing)
-   [-] **Expandable details** (PLAY-018: a hover card left of the alert explains what is wrong and what to do; affected targets come by click cycling, not listed)
    -   Which pawns/items/rooms are affected.
-   [x] **Click cycles affected targets** (click forward, right-click back)
-   [-] **Examples** (UI-014: animals hungry, wild animals eating crops, timbergnaws eating trees (N felled), pen full alerts)
    -   low food.
    -   colonist needs rescue.
    -   medical emergency.
    -   extreme break risk.
    -   dangerous temperature.
    -   missing bed.
    -   animal starvation.
    -   no research selected.
    -   idle colonists.
    -   unburied colonist.
    -   major hostile threat.

## 23.4 Alert scalability

-   [x] **Aggregate repeated alerts**
    -   "27 colonists need treatment" rather than 27 cards.
-   [ ] **Search/group in large colonies**
-   [ ] **Do not recompute expensive alert queries in UI every frame**

------------------------------------------------------------------------

# 24. History, Statistics and Records UI

## 24.1 History screen

-   [-] **Colony timeline** (UI-022: History tab (F9) lists major events with click-to-select; no category filter or date ordering controls)
    -   Major events ordered by game date.
    -   raids.
    -   deaths.
    -   joins.
    -   marriages.
    -   research.
    -   quests.
    -   disasters.
    -   ship events.
-   [ ] **Filter event categories**
-   [ ] **Jump to surviving entity/location where valid**

## 24.2 Graphs

-   [ ] **Wealth graph**
-   [ ] **Population graph**
-   [ ] **Colonist/animal counts**
-   [ ] **Threat/event history**
-   [ ] **Time-range controls**
-   [ ] **Efficient downsampling**
    -   Long games should not render millions of points.

## 24.3 Pawn records

-   [ ] **Combat records**
-   [ ] **Kills**
-   [ ] **Damage dealt/taken**
-   [ ] **Work/production records**
-   [ ] **Medical/social/research records where useful**
-   [ ] **Time as colonist**
-   [ ] **Historical achievements**

------------------------------------------------------------------------

# 25. World Map UI

## 25.1 World/local transition

-   [-] **World button/tab** (SIM-046: World tab (F8))
    -   Switch from local map to planet.
    -   Preserve local camera.
-   [-] **Planet navigation** (SIM-046: World tab globe plus setup globe; no object selection; NG-1: setup only. The landing globe rotates with drag, inertia and keys, zooms and picks tiles; there is no world view inside a running colony)
    -   Rotate/pan/zoom or equivalent world navigation.
    -   Select world tile/object.

## 25.2 World tile inspector

-   [-] **Biome** (NG-1: shown on the landing screen's tile panel; the colony top bar names it; no world tab in the colony)
-   [-] **Terrain/hilliness** (landing screen only)
-   [-] **Elevation** (landing screen only)
-   [-] **Temperature** (landing screen only: mean, summer high, winter low)
-   [-] **Rainfall** (landing screen only)
-   [-] **Growing period** (landing screen only)
-   [-] **Movement difficulty** (landing screen only; informational until world travel exists)
-   [-] **Road/river** (landing screen only)
-   [-] **Forage** (landing screen only)
-   [-] **Settlement/faction if occupied** (landing screen only: nearest settlement and the Factions dialog; an occupied tile cannot be chosen)
-   [ ] **Distance/travel estimate when route context exists**

## 25.3 World objects

-   [ ] **Player colony**
-   [ ] **Faction settlement**
-   [-] **Caravan** (SIM-046: caravan list and globe marker)
-   [ ] **Quest site**
-   [ ] **Temporary encounter**
-   [ ] **Distant endgame objective**
-   [ ] **Transport pod destination marker**

## 25.4 World route visualization

-   [ ] **Path line**
-   [ ] **Estimated travel time**
-   [ ] **Terrain/road cost**
-   [ ] **Invalid destination reason**

------------------------------------------------------------------------

# 26. World Generation and Landing UI

## 26.1 New-world setup

-   [x] **World seed input** (UI-014: typed digit field with Dice; same seed and biome give the same site)
-   [x] **Planet coverage/size option** (NG-1: 30 / 50 / 100 % of the globe; 5 % in dev mode)
-   [x] **Temperature/rainfall/population options where supported** (NG-1: five temperature and rainfall stops, three population stops, plus the faction table)
-   [x] **Generate button** (NG-1: builds the globe behind a progress dialog; unchanged settings reuse the world, changed ones mark it stale)
-   [x] **Generation progress** (NG-1: phase and percentage; the builder runs a fixed budget per frame so the UI stays responsive)
    -   UI remains responsive.

## 26.2 Landing-site selection

-   [x] **Select world tile** (NG-1: click on the rotating hex globe; ocean, lakes and occupied tiles show why they cannot be chosen)
-   [x] **Detailed tile card** (NG-1: terrain, coordinates, elevation, mean/summer/winter temperature, rainfall, growing period, stones, water, caves, roads, nearest settlement, movement difficulty, forage, wildlife and warnings; biome hover card)
    -   biome.
    -   temperature range.
    -   growing period.
    -   terrain.
    -   rainfall.
    -   movement difficulty.
    -   stone/resource context if known.
    -   nearby factions.
-   [ ] **Search/find world features** (not in NG-1 scope: only a Random site button and the Factions dialog's find-capital click)
    -   Optional high-value usability feature.
-   [x] **Settle/start command** (NG-1: Next goes to Create Characters; Start there builds the colony)
-   [x] **Back/regenerate** (NG-1: Back keeps the draft; Create World regenerates; Random site picks another tile)

------------------------------------------------------------------------

# 27. New Game: Scenario, Storyteller, Difficulty and Pawn Selection

## 27.1 Main menu

-   [x] **New game** (NG-1: opens the Choose Scenario wizard)
-   [x] **Load game** (save list; Continue loads the newest, commitment saves included)
-   [x] **Options** (Audio, Graphics, Gameplay, Accessibility, Controls, and Story during play)
-   [-] **Plugins** (disabled)
-   [x] **Credits** (creator, originality notice, asset credits)
-   [x] **Quit**
-   [-] **Version/build identifier** (shows `dev` / AppConfig.Version; no commit id)

## 27.2 Scenario selection

-   [x] **Scenario list** (NG-1: Broken Hull, Ember Clan, Gilded Expedition, Bare Hands and custom files; searchable)
    -   Standard crash start.
    -   tribal/low-tech.
    -   difficult solo.
    -   rich solo/high-tech.
    -   custom scenarios.
-   [x] **Scenario detail** (NG-1: people, equipment, research, special rules and the original narrative)
    -   Starting people.
    -   equipment.
    -   research.
    -   special rules.
    -   original narrative description.
-   [x] **Scenario editor entry** (New, Edit a copy)

## 27.3 Scenario editor

-   [x] **Scenario parts list** (NG-1: ordered list, 14 part kinds)
-   [x] **Add/remove part** (add-part picker, remove, reorder)
-   [x] **Configure values** (per-part fields plus a live preview)
-   [x] **Validation** (a panel names the offending part; errors block Done, warnings do not)
-   [x] **Save/load custom scenario** (versioned JSON files, atomic writes)
-   [ ] **Plugin scenario parts** (no scenario-part plugin API yet)

## 27.4 Storyteller selection

-   [x] **Storyteller cards** (NG-1: Orla Vane the Steady Hand, Bram Holloway the Hearthkeeper, Kestrel Quay the Gambler, each with an original bust)
    -   Paced.
    -   relaxed.
    -   random.
    -   original characters/names/art.
-   [x] **Behavior description** (NG-1: a bio and an example pacing sparkline per storyteller, no formulas)
    -   Explain pacing differences without revealing unnecessary
        implementation formulas.

## 27.5 Difficulty

-   [x] **Preset difficulties** (NG-1: Wanderer, Gentle, Settled, Seasoned, Grim, Merciless)
-   [-] **Custom difficulty** (NG-1: 19 sliders and toggles in groups with dependent fields; the Disease frequency slider has no effect because disease events do not exist)
    -   Threat scale.
    -   economy.
    -   mood.
    -   disease.
    -   adaptation.
    -   other exposed factors.
-   [x] **Reset preset** (Copy from a preset, Reset, Cancel)
-   [-] **Tooltip each setting** (most have one; the economy yields, infection chance and pain tolerance do not yet)

## 27.6 Starting pawn screen

-   [x] **Candidate pawn cards** (NG-1: generated candidates with portraits by build, backstories and traits)
-   [x] **Select starting roster** (selected and left-behind lists; drag, swap, reorder)
-   [x] **Randomize/reroll** (per candidate and Randomize all; rename; undo; deterministic from the seed)
-   [-] **Character details** (NG-1: all but relationships; SIM-041 seeds starting families but the starting-pawn screen does not show them yet)
    -   age.
    -   skills/passions.
    -   traits.
    -   backstories.
    -   health.
    -   incapabilities.
    -   relationships.
-   [x] **Team warnings** (NG-1: nobody can fight, no doctor, cook, grower, builder or researcher, everyone very young or old)
    -   No capable doctor.
    -   no grower.
    -   no constructor.
    -   no cook.
    -   no violence-capable pawn, etc.
-   [x] **Start colony** (NG-1: builds the Setup with the chosen roster behind a loading screen)

------------------------------------------------------------------------

# 28. Caravan Formation UI

## 28.1 Form caravan dialog

-   [x] **Destination selection** (SIM-046)
-   [-] **People list** (SIM-046: colonists only)
    -   Colonists.
    -   prisoners.
-   [ ] **Animals**
-   [ ] **Items**
-   [-] **Food** (SIM-046: food days)
-   [ ] **Medicine**
-   [ ] **Currency**
-   [ ] **Mass summary**
    -   carried / capacity.
-   [ ] **Travel time**
-   [ ] **Food duration**
-   [ ] **Forage estimate**
-   [ ] **Movement speed**
-   [ ] **Warnings**
    -   immobile pawn.
    -   insufficient food.
    -   no capable handler.
    -   overloaded.
    -   dangerous temperature.
-   [x] **Send command** (SIM-046)

## 28.2 Caravan loading status

-   [ ] **Gathering indicator**
-   [ ] **Items remaining**
-   [ ] **Pawns/animals not ready**
-   [ ] **Cancel formation**
-   [ ] **Failure explanation**

------------------------------------------------------------------------

# 29. Caravan World Inspector

-   [-] **Caravan member list** (SIM-046: World tab caravan list)
-   [ ] **Inventory**
-   [ ] **Food remaining**
-   [ ] **Travel speed**
-   [ ] **Current route**
-   [ ] **Estimated arrival**
-   [ ] **Health/mood warnings**
-   [ ] **Rest state**
-   [ ] **Foraging**
-   [ ] **Reform/enter settlement actions**
-   [ ] **Trade**
-   [ ] **Attack**
-   [ ] **Settle**
-   [ ] **Return/change destination**

------------------------------------------------------------------------

# 30. Quest UI

## 30.1 Quest list

-   [ ] **Available**
-   [ ] **Active**
-   [ ] **Completed/failed history**
-   [ ] **Expiry/deadline**
-   [ ] **Faction/source**
-   [ ] **Search/filter**

## 30.2 Quest detail

-   [ ] **Original narrative description**
-   [ ] **Objectives**
-   [ ] **World/map targets**
-   [ ] **Time limits**
-   [ ] **Threat information known to player**
-   [ ] **Reward choices**
-   [ ] **Goodwill consequences**
-   [ ] **Accept/decline**
-   [ ] **Jump to world target**

## 30.3 Active quest tracker

-   [ ] **Objective checklist**
-   [ ] **Countdown**
-   [ ] **State changes**
-   [ ] **Failure condition**
-   [ ] **Completion notification**

------------------------------------------------------------------------

# 31. Transport Pod UI

-   [ ] **Launcher inspector**
-   [ ] **Fuel level**
-   [ ] **Load group**
    -   pawns/items.
-   [ ] **Loading progress**
-   [ ] **Launch button**
-   [ ] **World targeting mode**
-   [ ] **Range visualization**
-   [ ] **Destination action**
    -   land.
    -   reinforce.
    -   gift.
    -   attack.
    -   trade where supported.
-   [ ] **Invalid target explanation**

------------------------------------------------------------------------

# 32. Endgame Ship UI

-   [ ] **Ship component inspector**
-   [ ] **Ship connectivity/validity overlay**
-   [ ] **Missing required component explanation**
-   [ ] **Reactor status**
-   [ ] **Start reactor confirmation**
    -   Explicitly warn about endgame threat escalation.
-   [ ] **Reactor countdown**
-   [ ] **Casket occupancy**
-   [ ] **Load/unload pawn**
-   [ ] **Launch readiness**
-   [ ] **Launch confirmation**
-   [ ] **Victory/escape screen**
-   [ ] **Colony story summary**
-   [ ] **Credits**

------------------------------------------------------------------------

# 33. Save, Load and Pause/Menu UI

## 33.1 In-game menu

-   [x] **Resume**
-   [x] **Save** (writes the colony's slot, "saved <slot>" hint)
-   [x] **Load** (pause and main menu open the save list; main menu Continue loads the newest)
-   [x] **Options** (opens the Options screen, Back returns to pause)
-   [x] **Return to main menu**
-   [x] **Quit**

## 33.2 Save dialog

-   [ ] **Save name**
-   [ ] **Existing saves**
-   [ ] **Timestamp/game date**
-   [ ] **Colony name**
-   [ ] **Enabled plugin summary**
-   [ ] **Overwrite confirmation**
-   [ ] **Saving progress/error**
-   [x] **Commitment/ironman restrictions** (NG-1: one save slot that the game updates itself at each day, on return to menu, on quit and on window close; Pause offers Save & Quit; Load is disabled with its reason; developer mode is locked; saves are tagged Commitment)

## 33.3 Load dialog

-   [x] **Save list** (newest first: slot, game date, colonists, file time)
-   [ ] **Version**
-   [x] **Game date**
-   [ ] **Colony**
-   [ ] **Plugin set**
-   [ ] **Compatibility warning**
-   [ ] **Missing plugin warning**
-   [ ] **Migration warning/error**
-   [x] **Corrupt save handling** (listed disabled with the reason)
-   [ ] **Delete save confirmation**

## 33.4 Autosave feedback

-   [ ] **Small non-blocking autosave indicator**
-   [ ] **Autosave failure alert**
-   [ ] **Rotating autosave configuration**

------------------------------------------------------------------------

# 34. Options and Accessibility UI

## 34.1 Graphics

-   [ ] **Resolution/window mode**
-   [x] **VSync/frame cap** (VSync toggle, off by default; PERF-002: Frame cap 60 / 120 / 144 / Off in Options › Graphics, 120 on every display by default, paced in the client)
-   [x] **UI scale** (UI-024: Options › Accessibility, Auto (follows the window, 1280×720 = 100 %) or 75–200 %, applies live)
-   [x] **Rendering quality toggles** (PLAY-019: Quality High/Medium/Low/Custom picked from the hardware at the first start, Detect again; Shadows, Lighting, Water, Grass rows applied live; the player's own changes always kept)
-   [x] **Animation/effects density** (Effects Full/Reduced/Minimal scales particles and weather; PLAY-019: Grass full/reduced/sparse thins tufts per tile)
-   [x] **World render scale** (PERF-002: Full / 3/4 / Half of the physical pixels; HUD and text native; preset High Full, Medium 3/4 on 2× displays)
-   [x] **Performance overlay** (PERF-002: Options › Graphics toggle; fps, 1 % low, frame, update and draw time, sim rate and held ticks, heap and GC rate, a frame-time sparkline, by the resource list; also with the UI hidden)
-   [x] **Hardware check** (PLAY-019: RAM and cores read once at startup; a low-memory notice on the main menu and per-size memory warnings in the new-game wizard; warns only)
-   [ ] **Far-zoom detail**

## 34.2 Audio

-   [ ] **Master**
-   [ ] **Music**
-   [ ] **Effects**
-   [ ] **Ambient**
-   [ ] **UI**

## 34.3 Gameplay

-   [x] **Pause on threat** (Options › Gameplay, on by default; PLAY-017: animal chases trigger it)
-   [x] **Run in background** (PLAY-017: Options › Gameplay; off pauses the colony while the window is away, on keeps it running at a capped frame rate)
-   [ ] **Autosave interval**
-   [ ] **Default speed behavior**
-   [ ] **Edge scrolling**
-   [ ] **Confirmation preferences**
-   [x] **Tutorial/learning helper** (PLAY-018: Options › Gameplay "Learning helper" with Reset lessons)

## 34.4 Controls

-   [x] **Key binding list** (UI-016: Options › Controls, paged, by category)
-   [x] **Rebind** (click a slot, press a key; Esc cancels, Backspace clears; saved in settings.json)
-   [x] **Conflict detection** (a used key asks "Swap?")
-   [x] **Reset defaults**

## 34.5 Accessibility

-   [ ] **UI scaling**
-   [ ] **Color-independent state indicators**
-   [ ] **Color-vision-friendly palette option**
-   [ ] **Reduced flashing/effects**
-   [-] **Text size** (text scales with the UI scale (UI-024); no separate text-size setting)
-   [ ] **Tooltip delay**
-   [ ] **Pause-friendly interaction**
-   [ ] **Keyboard navigation where practical**

------------------------------------------------------------------------

# 35. Tutorial and Learning Helper UI

## 35.1 Learning helper

-   [-] **Contextual lesson cards** (PLAY-018: ten original lessons in a top-right learning helper; the next three show, each with a detail card; not triggered by first contact with a mechanic)
    -   Trigger when player first encounters a mechanic.
-   [-] **Lesson queue/history** (PLAY-018: lessons queue in order and finished ones drop out; no history view)
-   [ ] **Dismiss**
-   [x] **Mark learned** (PLAY-018: done by doing it, or "Got it"; saved in the settings)
-   [ ] **Reopen help**
-   [x] **Do not repeatedly interrupt experienced players** (PLAY-018: the helper never pops up; it collapses, hides when everything is learned, and can be switched off)

## 35.2 Tutorial highlights

-   [ ] **Highlight UI control**
-   [ ] **Highlight map cell/entity**
-   [ ] **Wait for required action**
-   [ ] **Explain failure**
-   [ ] **Skip tutorial step**

## 35.3 Help/reference

-   [ ] **Searchable concept index**
-   [ ] **Controls reference**
-   [ ] **Stat explanations**
-   [ ] **Mod/plugin help entry**

------------------------------------------------------------------------

# 36. Plugin Manager UI

The plugin system is a first-class project feature and needs a
first-class interface rather than relying on manual file editing.

## 36.1 Plugin manager

-   [ ] **Installed plugin list**
    -   Name.
    -   author.
    -   version.
    -   plugin ID.
    -   compatibility.
    -   enabled state.
-   [ ] **Enable/disable**
-   [ ] **Search**
-   [ ] **Sort**
-   [ ] **Filter**
    -   enabled.
    -   disabled.
    -   incompatible.
    -   update/compatibility issue.
-   [ ] **Plugin detail**
    -   description.
    -   dependencies.
    -   incompatibilities.
    -   source/homepage metadata.
    -   provided content/capabilities.

## 36.2 Load order

-   [ ] **Resolved load-order list**
-   [ ] **Dependency visualization**
-   [ ] **Manual ordering only where permitted**
-   [ ] **Load-before/load-after hints**
-   [ ] **Cycle/conflict error**
-   [ ] **Explain why plugin appears at position**
-   [ ] **Reset automatic order**

## 36.3 Compatibility

-   [ ] **Missing dependency dialog**
-   [ ] **Version mismatch**
-   [ ] **Duplicate plugin ID**
-   [ ] **Declared incompatibility**
-   [ ] **Unsafe capability warning**
-   [ ] **Save requires missing plugin warning**

## 36.4 Plugin settings

-   [ ] **Settings list**
-   [ ] **Schema-generated controls**
    -   bool.
    -   number.
    -   enum.
    -   string.
    -   key binding where safe.
-   [ ] **Reset defaults**
-   [ ] **Per-save vs global setting distinction**
-   [ ] **Restart/new-session requirement**

------------------------------------------------------------------------

# 37. Mod Author / Content Diagnostics UI

## 37.1 Content errors

-   [ ] **Startup configuration-error window**
    -   Plugin ID.
    -   file.
    -   definition.
    -   path/field.
    -   validation message.
-   [ ] **Copy diagnostics**
-   [ ] **Open provenance information where environment permits**
-   [ ] **Do not crash on recoverable content errors**

## 37.2 Definition inspector

-   [ ] **Search all loaded definitions**
-   [ ] **Definition type**
-   [ ] **Resolved values**
-   [ ] **Source plugin**
-   [ ] **Patch history**
    -   Which plugin added/replaced/merged each value.
-   [ ] **Runtime DefIndex**
-   [ ] **Referenced definitions**

## 37.3 Plugin performance

-   [ ] **Per-plugin hook timings**
-   [ ] **Script budget violations**
-   [ ] **Event hook counts**
-   [ ] **Plugin memory/state size where measurable**
-   [ ] **Disable offending plugin recommendation without silently doing
    it**

------------------------------------------------------------------------

# 38. Developer Mode UI

Developer tooling should be excellent because `rimworld-go` is open
source and intended for AI-assisted development.

## 38.1 Developer toolbar

-   [x] **Developer mode toggle** (UI-016: Options › Gameplay; ultrafast 15x and Numpad + single-tick step)
-   [-] **Debug actions** (UI-016: `/` menu: spawn deer/wolf, add wood, fire nature events, raid, kill animal, fill needs)
-   [-] **Debug overlays** (MAPGEN-2: "Map fields" cycles 12 layers over the map with a legend: elevation, fertility, wetness, rock types, caves, river distance, terrain classes, vegetation density, start score, roofs, fog, regions; no path/region-cost or temperature overlays)
-   [-] **Inspector** (UI-016: `'` raw id, position, job and flags of the selection)
-   [ ] **Profiler**
-   [ ] **Incident/storyteller tools**
-   [ ] **Content/plugin tools**

## 38.2 Entity debug inspector

-   [x] **EntityID** (UI-016 debug inspector)
-   [ ] **Components**
-   [x] **Current job** (UI-016 debug inspector)
-   [ ] **Intent scores**
-   [ ] **Reservations**
-   [ ] **Path**
-   [ ] **Needs**
-   [ ] **Mood**
-   [ ] **Health**
-   [ ] **Faction**
-   [ ] **RNG/debug identity**
-   [ ] **Last important events**
-   [ ] **State hash**

## 38.3 Map debug overlays

-   [-] **Cell coordinates** (UI-016: Alt-held hover lines; no grid overlay)
-   [ ] **Chunk IDs/activity**
-   [ ] **Region IDs**
-   [-] **Room IDs** (UI-016: Alt-held hover shows room id and score)
-   [-] **Walkability/path cost** (UI-016: Alt-held hover shows the walk cost)
-   [ ] **Current paths**
-   [ ] **Reservations**
-   [ ] **Work candidates**
-   [ ] **Storage index**
-   [ ] **Temperature regions**
-   [ ] **Power networks**
-   [ ] **Light**
-   [ ] **Beauty**
-   [ ] **Cover**
-   [ ] **Roof**
-   [x] **Fire active set** (PLAY-015: per-chunk fire lists; flames, smoke, light and sound only for burning cells)

## 38.4 Debug actions

-   [-] **Spawn item** (UI-016: 100 wood at the cursor)
-   [-] **Spawn pawn/animal** (UI-016: deer, timber wolf)
-   [ ] **Destroy entity**
-   [ ] **Damage/heal**
-   [ ] **Add/remove health effect**
-   [-] **Set need/mood** (UI-016: fill the selected colonist's food and rest)
-   [ ] **Teleport**
-   [ ] **Trigger mental state**
-   [-] **Trigger incident** (UI-016: herd crossing, farm animals, mad animal)
-   [ ] **Set weather**
-   [ ] **Change time**
-   [ ] **Finish research**
-   [ ] **Change goodwill**
-   [x] **Generate raid** (UI-016)
-   [ ] **Force job**
-   [ ] **Rebuild selected cache/index**

------------------------------------------------------------------------

# 39. Profiling and Performance UI

## 39.1 Simulation profiler

-   [x] **Ticks per second** (PERF-002: the performance overlay shows the sim rate and the ticks actually stepped)
-   [x] **Average tick time** (PERF-002: the overlay shows update ms per tick; `cmd/benchmark` the per-tick percentiles)
-   [ ] **p50/p95/p99 tick time**
-   [ ] **Per-phase time**
-   [ ] **Active entities**
-   [ ] **Sleeping entities/chunks**
-   [ ] **Jobs evaluated**
-   [ ] **Path requests**
-   [ ] **Nodes expanded**
-   [ ] **Cache hit rates**
-   [-] **Allocations** (PERF-002: the overlay shows the GC rate only; `BenchmarkWorldView` and `TestWorldViewQuietTickAllocs` pin the read model's allocations)
-   [x] **Heap** (PERF-002: the overlay shows the live heap from `runtime/metrics`)
-   [ ] **Goroutine count**

## 39.2 Hot-system drill-down

-   [ ] **Pathing**
-   [ ] **Job discovery**
-   [ ] **Rooms**
-   [ ] **Temperature**
-   [ ] **Power**
-   [-] **Rendering** (PERF-002: the overlay shows draw time; no per-pass breakdown)
-   [x] **Alerts**
-   [ ] **Plugins**

## 39.3 Timeline capture

-   [ ] **Start/stop capture**
-   [ ] **Mark tick/event**
-   [ ] **Export profile metadata**
-   [ ] **Correlate spike with incident/entity count**

------------------------------------------------------------------------

# 40. Large-Colony UI

This is a deliberate `rimworld-go` differentiator. The UI must remain
manageable beyond vanilla-scale colonies.

## 40.1 Pawn groups

-   [ ] **Create group**
-   [ ] **Rename**
-   [ ] **Assign/remove pawns**
-   [ ] **Group color/icon**
-   [ ] **Collapse colonist bar by group**
-   [ ] **Select whole group**

## 40.2 Group policies

-   [ ] **Work preset**
-   [ ] **Schedule**
-   [ ] **Allowed area**
-   [ ] **Food**
-   [ ] **Apparel**
-   [ ] **Drug**
-   [ ] **Medical**
-   [ ] **Individual override visualization**

## 40.3 Search everywhere

-   [x] **Pawn search** (UI-016: Z)
-   [x] **Animal search** (name or species)
-   [-] **Item/storage search** (items by kind on the map; buildings, plants and blueprints too; no storage contents yet)
-   [ ] **Research search**
-   [ ] **Build menu search**
-   [ ] **Quest search**
-   [ ] **Plugin search**
-   [ ] **Debug entity search**

## 40.4 Bulk operations

-   [ ] **Multi-row selection in management screens**
-   [ ] **Apply policy to selection**
-   [ ] **Set work priority in bulk**
-   [ ] **Set schedule in bulk**
-   [ ] **Set area in bulk**
-   [ ] **Draft/undraft group**

## 40.5 Aggregation

-   [x] **Aggregated alerts**
-   [ ] **Aggregated health warnings**
-   [ ] **Idle pawn groups**
-   [ ] **Food/medicine strategic summaries**
-   [ ] **Production bottleneck summaries where useful**

## 40.6 Virtualization performance

-   [ ] **1,000-pawn Work table remains interactive**
-   [ ] **1,000-pawn Schedule table remains interactive**
-   [ ] **1,000-pawn Assign table remains interactive**
-   [ ] **Large item lists do not allocate/render every row**
-   [ ] **UI update frequency is decoupled from every simulation tick
    where safe**

------------------------------------------------------------------------

# 41. Final UI Integration and Polish

## 41.1 Consistency audit

-   [ ] Same interaction has same control pattern everywhere.
-   [ ] Destructive actions use consistent confirmation rules.
-   [ ] Search fields behave consistently.
-   [ ] Tables share sorting semantics.
-   [ ] Disabled controls always explain why.
-   [ ] Selection and jump-to-target behavior is consistent.
-   [ ] Tooltips use consistent stat formatting.
-   [ ] Policies use consistent create/duplicate/delete flows.

## 41.2 Error-state audit

-   [ ] Missing assets show safe placeholder.
-   [ ] Missing plugin definition does not crash UI.
-   [ ] Stale selected EntityID clears gracefully.
-   [ ] Command rejection produces useful feedback.
-   [ ] Save/load errors are readable.
-   [ ] Invalid caravan/trade/build commands explain corrective action.
-   [ ] UI survives map transition while panels are open.

## 41.3 Performance audit

-   [ ] No full-world query every rendered frame.
-   [ ] No rebuilding large tables every frame.
-   [ ] Text/layout caches are bounded.
-   [ ] Tooltips do not trigger expensive domain queries.
-   [ ] Far zoom reduces rendering cost.
-   [ ] Overlays use cached/dirty data.
-   [ ] Large management screens virtualize.

## 41.4 Determinism audit

-   [ ] UI animation timing never affects gameplay.
-   [ ] Pointer hover never changes simulation.
-   [ ] Selection never changes AI.
-   [ ] Sorting a UI list never changes authoritative order.
-   [-] Renderer frame rate never changes command semantics. (QA-001: `TestJourneyTimeSpaceAndSpeeds` pins the tick count per Update at each speed and the camera journeys step Draws on a pinned clock; the Update budget's tick debt (`update_budget.go`) is covered in its own tests, not by a journey at a slow frame rate)
-   [ ] Commands have explicit target/sequence assigned by application
    boundary.

------------------------------------------------------------------------

# 42. Recommended UI Implementation Milestones

## UI Milestone A --- First interactive build

Complete:

-   [ ] Core UI framework.
-   [ ] Map viewport.
-   [ ] Camera.
-   [ ] Time controls.
-   [ ] Selection.
-   [ ] Generic inspector.
-   [ ] Colonist bar.
-   [ ] Context commands.

**Player outcome:** Can see the simulation, inspect entities, select
colonists and control time.

## UI Milestone B --- Colony construction

Complete:

-   [-] Architect.
-   [-] Placement preview.
-   [-] Orders.
-   [-] Zones.
-   [ ] Stockpile settings.
-   [x] Growing-zone settings. (PLAY-020: crop, allow sowing, allow cutting, hide, rename)
-   [-] Work.
-   [ ] Schedule.
-   [x] Allowed areas. (PLAY-015: see 6.5)

**Player outcome:** Can organize labor and physically build a
functioning settlement.

## UI Milestone C --- Character simulation

Complete:

-   [ ] Needs/mood.
-   [ ] Health.
-   [ ] Character/skills.
-   [ ] Gear.
-   [ ] Social.
-   [x] Prisoner basics. (SIM-045)
-   [ ] Alerts/messages.

**Player outcome:** Can understand why individual colonists behave,
fail, suffer, or break.

## UI Milestone D --- Production and combat

Complete:

-   [ ] Bills.
-   [ ] Research.
-   [x] Power overlay. (PLAY-021)
-   [-] Temperature/roof overlays. (SIM-018: roof overlay done; temperature overlay missing)
-   [ ] Drafted combat.
-   [ ] Hit/cover feedback.
-   [ ] Assign/policies.

**Player outcome:** Can run an industrial colony and tactically defend
it.

## UI Milestone E --- Colony ecosystem

Complete:

-   [ ] Animals.
-   [ ] Wildlife.
-   [ ] Trade.
-   [ ] Factions.
-   [ ] History.
-   [ ] richer letters/alerts.

**Player outcome:** Can manage the full local-map vanilla colony loop.

## UI Milestone F --- World game

Complete:

-   [-] World map. (SIM-046: World tab (F8) with globe and caravan list; no object selection)
-   [x] World generation.
-   [x] Landing selection.
-   [x] New-game flow. (map sizes 200 to 500 enabled after the ENG-001 frame-rate gate)
-   [-] Caravan formation. (SIM-046: colonists and destination only)
-   [-] Caravan inspector. (SIM-046: caravan list in the World tab)
-   [ ] Quests.
-   [ ] Transport pods.

**Player outcome:** Colony participates in the generated world rather
than existing only on one map.

## UI Milestone G --- Complete campaign

Complete:

-   [ ] Endgame ship.
-   [ ] Save/load polish.
-   [ ] Options.
-   [ ] Tutorial.
-   [ ] Victory/credits.

**Player outcome:** A new player can start, learn, save, play a full
campaign, and win.

## UI Milestone H --- Open-source platform

Complete:

-   [ ] Plugin manager.
-   [ ] Plugin settings.
-   [ ] Content diagnostics.
-   [ ] Developer mode.
-   [ ] Profiling UI.
-   [ ] Large-colony management.

**Player/developer outcome:** `rimworld-go` becomes a scalable moddable
simulation platform rather than only a feature-complete game.

------------------------------------------------------------------------

# Appendix A --- UI-to-Domain Boundary Contract

Every UI feature should be classified as one of these:

## Read model

Examples:

``` text
PawnSummary
PawnNeedsView
PawnHealthView
SelectionDetails
WorkMatrixView
ScheduleView
StockpileSettingsView
ResearchGraphView
FactionView
TradeView
CaravanView
QuestView
AlertView
```

Read models are presentation-friendly snapshots. They should not expose
writable domain storage.

## Command

Examples:

``` text
SetGameSpeed
DraftPawn
MoveDraftedPawn
DesignateBuild
CancelDesignation
SetWorkPriority
SetSchedule
SetAllowedArea
SetStockpileFilter
SetBill
SetResearchProject
SetMedicalPolicy
EquipItem
FormCaravan
ExecuteTrade
AcceptQuest
EnablePlugin
```

The UI requests. The application/domain validates.

## Presentation state

Examples:

``` text
camera
selection
hover
open tab
scroll
sort
filter
window geometry
tooltip
overlay selection
```

These do not belong in authoritative simulation state.

------------------------------------------------------------------------

# Appendix B --- Required UI Readability Principles

-   [ ] The player can determine **what a pawn is doing**.
-   [ ] The player can determine **why a pawn is not doing expected
    work**.
-   [ ] The player can determine **why a building does not work**.
-   [ ] The player can determine **why a room is bad**.
-   [ ] The player can determine **why a pawn is unhappy**.
-   [ ] The player can determine **why a pawn is sick/downed**.
-   [ ] The player can determine **why a job/path/order is invalid**.
-   [ ] The player can determine **where colony resources are going**.
-   [ ] The player can determine **what an alert expects them to fix**.
-   [ ] The player can inspect the modifiers behind important stats.
-   [ ] Important information is available without developer mode.
-   [ ] Developer mode exposes substantially deeper diagnostics.

------------------------------------------------------------------------

# Appendix C --- UI Definition of Done

A UI checkbox is complete only when all applicable items below pass.

-   [ ] Reads through presentation/application boundary rather than
    mutable domain internals.
-   [ ] Mutations are typed commands.
-   [ ] Loading/stale/missing entity states are handled.
-   [ ] Disabled actions explain why.
-   [-] Mouse behavior works. (QA-001: 89 player journeys click, drag, box-select, wheel and middle-drag through `App.Update`, and every `click(t, a, id)` test presses its button with the real mouse; per-screen sign-off still manual)
-   [-] Keyboard behavior works where appropriate. (QA-001: `TestEveryInputActionHasAJourney` fails on a bound action no journey presses; per-screen sign-off still manual)
-   [ ] Tooltip/help text exists for non-obvious mechanics.
-   [ ] Scrolling works at realistic data size.
-   [ ] Lists/tables are virtualized when scale requires it.
-   [x] Layout works at supported UI scales/resolutions. (UI-024: setup screens, HUD, panels, tools and Options tested at 100/150/200 %; HiDPI renders at the screen's true resolution)
-   [ ] Important states are not communicated by color alone.
-   [ ] No expensive world scan occurs every frame.
-   [-] Automated tests cover controller/view-model logic where
    practical. (QA-001: journeys assert the command that left the client or the read model after it for every command kind the client emits, bar two owed debts; CI runs the suite on every push)
-   [ ] Screenshot/manual acceptance scenario is documented for visual
    components.
-   [ ] Plugin-provided content does not require hardcoded UI edits
    where the system is intended to be extensible.
-   [ ] Relevant package/system documentation is updated.

------------------------------------------------------------------------

# Appendix D --- Explicitly Excluded DLC UI

Do not implement these as part of vanilla UI unless they later ship as
plugins/expansions:

-   Royal title/honor/permit/psycast interfaces.
-   Ideology creation, memes, precepts, rituals, roles, relics.
-   Gene/xenotype editor and mechanitor/mech control interfaces.
-   Child growth/learning interfaces.
-   Pollution/wastepack management.
-   Anomaly monolith, containment, dark research and ritual interfaces.
-   Odyssey-specific expansion travel/vehicle/space interfaces.

The base UI architecture should be extensible enough that optional
plugins can add new global tabs, inspector tabs, commands, overlays,
definition categories and settings without modifying core domain
packages.

------------------------------------------------------------------------

# Final UI Directive

The UI should preserve the central colony-simulator interaction loop:

> **observe → understand → prioritize → designate/assign → let
> autonomous pawns act → receive feedback → intervene when necessary**

A successful interface does not merely expose every variable. It helps
the player understand a complicated autonomous simulation without
micromanaging every tick.

For `rimworld-go`, the UI must additionally remain effective at scales
much larger than traditional vanilla colonies. Every management
interface should therefore be designed from the beginning around:

-   filtering;
-   sorting;
-   search;
-   grouping;
-   bulk actions;
-   virtualization;
-   aggregation;
-   explainability;
-   keyboard efficiency;
-   plugin extensibility.

The player should be able to manage **10 colonists comfortably and 1,000
colonists deliberately** without changing the underlying simulation
architecture.
