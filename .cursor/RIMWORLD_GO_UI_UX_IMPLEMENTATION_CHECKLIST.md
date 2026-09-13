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
-   [-] **Typed UI command gateway** (session intents StartGame/Quit/ReturnToMenu only; no gameplay command catalog)
    -   Every gameplay mutation becomes a typed command: move, draft,
        designate, build, assign work, change policy, set bill, trade,
        form caravan, etc.
    -   Commands carry enough information for domain-side validation.
    -   UI may optimistically preview actions but authoritative success
        comes from simulation state.
-   [-] **Presentation-only state store** (screen stack, pause overlay, camera, selection, hover/hint; no architect/bindings store)
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

-   [ ] **UI scale and density system**
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
-   [ ] **Semantic colors**
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
-   [ ] **Search input**
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
-   [ ] **Progress bar**
-   [ ] **Stat bar**
-   [ ] **Context menu**
-   [ ] **Confirmation dialog**
-   [ ] **Modal window**
-   [ ] **Toast/message**
-   [ ] **Persistent alert card**
-   [ ] **Tooltip**
-   [ ] **Rich tooltip/stat breakdown**
-   [ ] **Drag painting tool**
    -   For zones, schedules and priorities.
-   [ ] **Resizable/repositionable developer window primitive**

## 1.4 Input routing

-   [-] **Mouse hit testing**
    -   UI consumes pointer before map interaction. (menus and pause overlay)
    -   Map receives pointer only when not blocked by a panel/modal.
-   [-] **Keyboard focus**
    -   Text fields capture typing without triggering game hotkeys. (no text fields yet)
    -   Escape backs out one interaction level at a time. (overlay, then stack, then pause)
-   [ ] **Shortcut manager**
    -   Named actions rather than hardcoded keys.
    -   Display shortcut in tooltips.
    -   Rebinding later uses the same registry.
-   [ ] **Drag interaction state**
    -   Selection rectangle.
    -   zone painting.
    -   build line/area.
    -   schedule painting.
    -   list drag/reorder where appropriate.

------------------------------------------------------------------------

# 2. First Playable HUD: Map, Camera, Time, Selection

This is the minimum UI required to interact with a running simulation.

## 2.1 Main game viewport

-   [-] **Map viewport** (draws grid and pawns from read-only snapshots each frame; pawn sprites lerp between ticks; no roofs/visibility treatment or effects yet)
    -   Occupies the central play area.
    -   Draw terrain, roofs/visibility treatment, structures, items,
        plants, pawns, effects and selection markers.
    -   Culls off-screen content.
    -   UI panels overlay or reserve space without changing
        authoritative map geometry.
-   [-] **Camera pan** (keyboard WASD/arrow pan implemented, presentation-only; mouse edge/drag pan and smoothing not implemented)
    -   Mouse edge/drag and keyboard movement.
    -   Smooth presentation movement.
    -   Camera position is presentation-only.
-   [-] **Camera zoom** (keyboard/wheel zoom clamped 0.5x-3x implemented; zoom-toward-cursor and far-zoom detail simplification not implemented)
    -   Multiple useful zoom levels from tactical detail to colony
        overview.
    -   At far zoom, simplify sprites/details rather than drawing
        expensive fine detail.
    -   Zoom toward cursor where practical.
-   [ ] **Jump/center camera**
    -   Double-click/select alert/entity.
    -   Hotkey to center selected pawn.
    -   Event letters can provide jump target.

## 2.2 Selection interaction

-   [-] **Single-click selection** (click selects the pawn on that tile, click on empty ground clears selection; entity-priority tie-breaking not needed yet with only pawns present)
    -   Prioritize sensible entity under cursor.
    -   Clicking empty terrain selects tile or clears selection
        depending interaction state.
-   [ ] **Selection cycling**
    -   When multiple selectable things share a cell, provide
        predictable way to inspect/cycle them.
-   [ ] **Drag-select colonists**
    -   Rectangle selects player-controllable pawns.
    -   Modifiers add/remove from selection.
-   [ ] **Double-click same-type selection**
    -   Select visible entities of the same relevant type, useful for
        pawns/animals/items.
-   [-] **Selection markers** (yellow outline on the selected pawn; no multi-selection/hostile/friendly differentiation yet)
    -   Clear original visual indicator.
    -   Differentiate single/multiple selection and hostile/friendly if
        needed.

## 2.3 Time controls

-   [x] **Pause button** (`Space` toggles pause)
-   [x] **Normal speed** (key `1`; 1x = 60 ticks / real second)
-   [x] **Fast speed** (key `2`; 3x)
-   [x] **Very-fast speed** (key `3`; superfast 6x)
-   [x] **Keyboard speed shortcuts**
-   [-] **Current date/time display** (HUD shows 1-based year/quadrum/day, period, HH:MM:SS, and schedule block; no temperature/weather yet)
    -   Local game hour.
    -   day.
    -   quadrum/season.
    -   year.
    -   Optional world temperature/weather nearby.
-   [ ] **Forced-pause feedback**
    -   If major threats/settings pause the game, visually explain why.

## 2.4 Minimal top-level HUD

-   [ ] **Current game state indicator**
    -   Paused/speed.
    -   map/world context.
    -   developer mode indicator if enabled.
-   [ ] **Message feed anchor**
    -   Short transient messages appear without obscuring play.
-   [ ] **Alert anchor**
    -   Reserve a consistent edge of the HUD for persistent colony
        alerts.

------------------------------------------------------------------------

# 3. Selection Inspector and Contextual Command Surface

The inspector is the primary way the player understands one pawn,
building, item, plant, animal, corpse or tile.

## 3.1 Generic selection panel

-   [ ] **Selected entity header**
    -   Display label/name.
    -   entity category.
    -   faction/ownership where relevant.
    -   health/condition summary.
    -   concise current state.
-   [ ] **Entity portrait/icon**
    -   Original presentation.
    -   For pawns, portrait or representative pawn rendering.
-   [ ] **Description**
    -   Data-driven description.
    -   Plugins can provide localization.
-   [ ] **Location and environment**
    -   Cell.
    -   room/outdoors.
    -   temperature.
    -   terrain.
    -   roof state where useful.

## 3.2 Context command bar

-   [ ] **Context-sensitive actions**
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
-   [ ] **Disabled action explanation**
    -   Disabled command must explain *why*: incapable, unreachable,
        reserved, missing material, wrong faction, research missing,
        unsafe, etc.
-   [ ] **Multi-selection actions**
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

-   [ ] **Always-visible colonist portraits**
    -   Compact representation of each colonist.
    -   Click selects; double-click centers.
    -   Preserve identity at a glance.
-   [ ] **Status indicators**
    -   Downed.
    -   drafted.
    -   mental state.
    -   severe illness/injury.
    -   sleeping.
    -   caravan/off-map.
    -   break risk.
    -   Icons must remain readable without opening inspector.
-   [ ] **Ordering**
    -   Stable default ordering.
    -   Later allow groups/custom ordering.
-   [ ] **Large-colony scaling**
    -   Collapse, scroll, wrap, group or virtualize rather than
        rendering 1,000 full portraits.
    -   Zoom/density modes.

## 4.2 Pawn summary

-   [ ] **Current activity/job**
-   [ ] **Draft state**
-   [ ] **Faction**
-   [ ] **Immediate health state**
-   [ ] **Immediate mood state**
-   [ ] **Current need warnings**

------------------------------------------------------------------------

# 5. Architect / Orders UI

This should be built as soon as construction/designations exist. It is
the player's primary world-editing command surface.

## 5.1 Architect root

-   [ ] **Architect button/tab**
    -   Opens build/designation categories.
    -   Remembers last category optionally.
-   [ ] **Category navigation**
    -   Orders.
    -   Zone/Area.
    -   Structure.
    -   Furniture.
    -   Production.
    -   Power.
    -   Security.
    -   Miscellaneous/endgame.
    -   Categories appear only when content exists/unlocked.
-   [ ] **Build option cards/buttons**
    -   Icon.
    -   name.
    -   material requirement.
    -   research lock.
    -   cost.
    -   short tooltip.

## 5.2 Placement preview

-   [ ] **Ghost blueprint**
    -   Show exact occupied cells before command submission.
-   [ ] **Validity visualization**
    -   Valid/invalid placement.
    -   Reasons: occupied, impassable, unsupported, missing research,
        wrong terrain, blocked interaction cell, roof/support
        constraint.
-   [ ] **Material/stuff selector**
    -   For buildings that can be made from different materials.
    -   Show available amount and relevant stat differences.
-   [ ] **Rotation**
    -   Rotate directional buildings before placement.
-   [ ] **Single-cell placement**
-   [ ] **Line placement**
    -   Walls/conduits.
-   [ ] **Rectangle/area placement**
    -   Floors/zones/roof designations where appropriate.
-   [ ] **Drag continuous placement**

## 5.3 Orders

-   [ ] **Mine**
-   [ ] **Cut plants**
-   [ ] **Harvest**
-   [ ] **Hunt**
-   [ ] **Deconstruct**
-   [ ] **Cancel**
-   [ ] **Forbid/unforbid**
-   [ ] **Remove floor**
-   [ ] **Remove roof**
-   [ ] **Claim structure where base-game semantics require it**
-   [ ] **Smooth stone/floor equivalents if supported**

## 5.4 Designation feedback

-   [ ] **Designation overlay**
    -   Clearly distinguish planned work from completed structures.
-   [ ] **Resource requirement feedback**
-   [ ] **Cancel designation interaction**
-   [ ] **Selection inspector for blueprint/frame**

------------------------------------------------------------------------

# 6. Zones and Areas UI

## 6.1 Zone tool

-   [ ] **Create stockpile**
-   [ ] **Create growing zone**
-   [ ] **Expand zone**
-   [ ] **Shrink/delete zone**
-   [ ] **Select existing zone**
-   [ ] **Zone color/outline**
    -   Distinct but unobtrusive.

## 6.2 Stockpile configuration panel

-   [ ] **Stockpile name**
-   [ ] **Storage priority selector**
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

-   [ ] **Crop selector**
    -   Searchable plant list.
    -   required skill.
    -   growth time.
    -   fertility sensitivity.
    -   expected yield.
    -   current research lock.
-   [ ] **Allow sowing toggle**
-   [ ] **Plant-cut/harvest behavior feedback**

## 6.4 Home area

-   [ ] **Home area overlay**
-   [ ] **Expand/shrink home area**
-   [ ] **Automatic home-area expansion setting**
    -   If implemented.
-   [ ] **Explain effects**
    -   Firefighting, cleaning and repair relevance.

## 6.5 Allowed areas

-   [ ] **Area manager**
    -   Create.
    -   rename.
    -   delete.
    -   invert.
    -   clear.
-   [ ] **Paint allowed area**
-   [ ] **Area overlay selector**
-   [ ] **Assign pawn/animal to area**
    -   Integrated later with Schedule/Assign/Animals.

------------------------------------------------------------------------

# 7. Work UI

The Work screen is one of the most important colony-management
interfaces.

## 7.1 Work table

-   [ ] **Pawn rows**
    -   Name/portrait.
    -   relevant status.
    -   sortable.
-   [ ] **Work-type columns**
    -   Firefight, patient, doctor, bed rest, basic, warden, handle,
        cook, hunt, construct, grow, mine, plant cut, craft categories,
        art, haul, clean, research.
    -   Column header icon/name.
    -   Tooltip explains work category.
-   [ ] **Simple priorities mode**
    -   Toggle work type on/off.
-   [ ] **Manual priorities mode**
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
-   [ ] **Schedule modes**
    -   Anything.
    -   Work.
    -   Sleep.
    -   Recreation.
-   [ ] **Paint schedule**
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

-   [ ] **Need bars**
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

-   [ ] **Mood bar**
    -   Current mood.
    -   target mood if smoothed.
    -   break-risk thresholds.
-   [ ] **Mood trend**
-   [ ] **Break-risk severity label**

## 9.3 Thought list

-   [ ] **Positive thoughts**
-   [ ] **Negative thoughts**
-   [ ] **Mood contribution**
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

-   [ ] **Overall state**
    -   Healthy/injured/sick/downed/dead.
-   [ ] **Pain**
-   [ ] **Blood loss**
-   [ ] **Consciousness**
-   [ ] **Moving**
-   [ ] **Manipulation**
-   [ ] **Sight/hearing/talking and other capacities**
-   [ ] **Bleeding rate / time-to-danger where useful**

## 10.2 Body-part tree

-   [ ] **Hierarchical anatomy**
    -   Torso/head/limbs/organs/subparts.
-   [ ] **Injury attached to part**
-   [ ] **Missing/destroyed part**
-   [ ] **Artificial/replaced part**
-   [ ] **Part health/severity**

## 10.3 Conditions

-   [ ] **Disease/health-effect list**
    -   Name.
    -   severity.
    -   stage.
    -   immunity.
    -   treatment quality.
    -   progression.
-   [ ] **Immunity vs disease visualization**
    -   Easy to compare which is winning.

## 10.4 Medical settings

-   [ ] **Maximum medical care selector**
-   [ ] **Self-tend toggle**
-   [ ] **Bed-rest/medical-bed state**
-   [ ] **Doctor/treatment status**

## 10.5 Operations

-   [ ] **Operations list**
-   [ ] **Add operation dialog**
    -   Valid surgeries only.
    -   Required medicine/body part/item.
    -   Risk information.
-   [ ] **Queued operations**
-   [ ] **Cancel/reorder operation**

## 10.6 Rich explanation

-   [ ] **Capacity breakdown tooltip**
    -   Show which injuries/body parts/effects cause final capacity.
-   [ ] **Treatment tooltip**
    -   Tend quality, remaining duration, medicine used.
-   [ ] **Prognosis information**
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
-   [ ] **Relationship summary**

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

-   [ ] **Equipped weapon**
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
-   [ ] **Equip**
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

-   [ ] **Draft toggle**
    -   Prominent when colonist selected.
-   [ ] **Draft state in colonist bar**
-   [ ] **Multi-draft**

## 13.2 Tactical orders

-   [ ] **Move order**
    -   Right-click valid destination.
-   [ ] **Attack order**
    -   Hostile target.
-   [ ] **Melee attack**
-   [ ] **Ranged attack**
-   [ ] **Fire-at-will toggle**
-   [ ] **Hold position behavior if implemented**
-   [ ] **Emergency tend**
-   [ ] **Rescue/capture contextual commands**

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

-   [ ] **Damage indicators**
-   [ ] **Projectile/impact feedback**
-   [ ] **Downed/death feedback**
-   [ ] **Threat markers**
-   [ ] **Hostile faction coloring/icons**
-   [ ] **Pause-on-threat option**

------------------------------------------------------------------------

# 14. Production Workbench and Bill UI

## 14.1 Workbench inspector

-   [ ] **Power/operational state**
-   [ ] **Interaction status**
-   [ ] **Current worker/progress**
-   [ ] **Bills list**

## 14.2 Bill stack

-   [ ] **Add bill**
    -   Search/filter recipes.
    -   show research/material constraints.
-   [ ] **Reorder bills**
-   [ ] **Suspend bill**
-   [ ] **Delete bill**
-   [ ] **Copy/paste bills**

## 14.3 Bill configuration

-   [ ] **Do X times**
-   [ ] **Do forever**
-   [ ] **Do until X**
-   [ ] **Target count**
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
-   [ ] **No power reason**
-   [ ] **Research locked reason**
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
-   [ ] **Research project states**
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
-   [ ] **Select research command**

## 15.3 Plugin support

-   [ ] **Dynamic graph layout**
    -   Plugin projects must integrate without hardcoded coordinates
        where possible.
-   [ ] **Namespaced provenance in developer mode**

------------------------------------------------------------------------

# 16. Power, Temperature, Roof and Environmental Overlays

## 16.1 Overlay framework

-   [ ] **Overlay toggle registry**
    -   Only one or compatible multiple overlays active.
    -   Plugins can register overlays through safe API.

## 16.2 Power overlay

-   [ ] **Power conduits/networks**
-   [ ] **Powered/unpowered buildings**
-   [ ] **Producer/consumer state**
-   [ ] **Battery charge**
-   [ ] **Network production vs consumption**
-   [ ] **Disconnected network visualization**

## 16.3 Temperature overlay

-   [ ] **Room temperatures**
-   [ ] **Outdoor temperature**
-   [ ] **Heat/cold gradient or labels**
-   [ ] **Selected room thermal details**
-   [ ] **Dangerous temperature highlighting**

## 16.4 Roof overlay

-   [ ] **Roofed**
-   [ ] **Unroofed**
-   [ ] **Mountain/thick roof distinction where gameplay requires it**
-   [ ] **Build/remove roof designation**

## 16.5 Fertility overlay

-   [ ] **Terrain fertility**
-   [ ] **Growing-zone usefulness**

## 16.6 Beauty overlay

-   [ ] **Beauty values**
-   [ ] **Selected pawn perceived beauty**
-   [ ] **Room beauty/impressiveness**

## 16.7 Room overlay

-   [ ] **Room boundary**
-   [ ] **Room role**
-   [ ] **Space**
-   [ ] **beauty**
-   [ ] **cleanliness**
-   [ ] **wealth**
-   [ ] **impressiveness**

------------------------------------------------------------------------

# 17. Assign / Policies UI

This screen centralizes per-pawn behavioral policies.

## 17.1 Assign table

-   [ ] **Pawn rows**
-   [ ] **Food restriction**
-   [ ] **Apparel policy**
-   [ ] **Drug policy**
-   [ ] **Medical care**
-   [ ] **Allowed area where chosen UX places it**
-   [ ] **Bed ownership shortcut where useful**

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

-   [ ] **Care tier selector**
-   [ ] **Clear icon/tooltip for medicine level**

------------------------------------------------------------------------

# 18. Social UI

## 18.1 Social tab

-   [ ] **Known relationships**
    -   Parent.
    -   child.
    -   sibling.
    -   lover/spouse.
    -   ex.
    -   friend/rival where modeled.
-   [ ] **Directional opinion**
    -   Selected pawn's opinion of each known pawn.
-   [ ] **Relationship status**
-   [ ] **Recent social memories**
-   [ ] **Off-map relation marker**

## 18.2 Opinion explanation

-   [ ] **Opinion breakdown tooltip**
    -   Relationship.
    -   traits.
    -   social memories.
    -   injuries/disfigurement modifiers only if game models them.
-   [ ] **Reciprocal opinion**
    -   Clearly distinguish A→B from B→A.

## 18.3 Relationship events

-   [ ] **Romance notification**
-   [ ] **Engagement/marriage**
-   [ ] **Breakup**
-   [ ] **Social fight**
-   [ ] **Relation death**
    -   Link messages to relevant pawns.

------------------------------------------------------------------------

# 19. Animals and Wildlife UI

## 19.1 Animals tab

-   [ ] **Owned animal table**
    -   Name.
    -   species.
    -   sex.
    -   age.
    -   health.
    -   master.
    -   area/pen.
    -   training.
    -   pregnancy/product state where applicable.
-   [ ] **Slaughter toggle**
-   [ ] **Medical care**
-   [ ] **Rename**
-   [ ] **Master assignment**

## 19.2 Training matrix

-   [ ] **Obedience**
-   [ ] **Release**
-   [ ] **Rescue**
-   [ ] **Haul**
-   [ ] **Current training progress**
-   [ ] **Trainability/ineligible explanation**

## 19.3 Pen UI

-   [ ] **Pen marker inspector**
-   [ ] **Pen boundary visualization**
-   [ ] **Enclosure validity**
-   [ ] **Assigned species**
-   [ ] **Nutrition/grazing capacity**
-   [ ] **Animal count vs food capacity**
-   [ ] **Missing gate/breach warning**

## 19.4 Wildlife tab

-   [ ] **Species rows**
-   [ ] **Count**
-   [ ] **Predator/herbivore**
-   [ ] **Hunt risk**
-   [ ] **Tame chance/wildness**
-   [ ] **Body size/meat yield context**
-   [ ] **Hunt designation toggle**
-   [ ] **Sort/filter**

------------------------------------------------------------------------

# 20. Prisoner UI

## 20.1 Prisoner tab

-   [ ] **Prisoner status**
-   [ ] **Faction**
-   [ ] **Resistance**
-   [ ] **Recruitment difficulty**
-   [ ] **Mood**
-   [ ] **Health**
-   [ ] **Interaction mode**

## 20.2 Interaction modes

-   [ ] **No interaction**
-   [ ] **Reduce resistance**
-   [ ] **Recruit**
-   [ ] **Release**
-   [ ] **Execute where supported**
-   [ ] **Medical/operation controls through Health UI**

## 20.3 Warden feedback

-   [ ] **Last interaction**
-   [ ] **Resistance/recruitment progress**
-   [ ] **Social skill effect**
-   [ ] **No warden/food/bed warning**

------------------------------------------------------------------------

# 21. Trade UI

## 21.1 Trade initiation

-   [ ] **Context action on trader/communications console**
-   [ ] **Select negotiator**
    -   Show relevant social/trade stat.

## 21.2 Trade dialog

-   [ ] **Two-sided inventory table**
    -   Colony goods.
    -   trader goods.
    -   quantity.
    -   buy price.
    -   sell price.
    -   quality/condition.
-   [ ] **Trade quantity control**
    -   Keyboard/mouse efficient.
    -   buy/sell direction visually obvious.
-   [ ] **Category filters**
-   [ ] **Search**
-   [ ] **Sort**
    -   name/value/category/quantity/quality.
-   [ ] **Currency balance**
    -   Current silver.
    -   net transaction.
    -   post-trade balance.
-   [ ] **Negotiator modifier**
-   [ ] **Accept/reset/cancel**
-   [ ] **Validation**
    -   Trader cannot afford.
    -   colony cannot afford.
    -   unavailable/changed stock.

## 21.3 Gift mode

-   [ ] **Switch trade/gift**
-   [ ] **Projected goodwill**
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

-   [ ] **Call faction**
-   [ ] **Request trader**
-   [ ] **Request military aid**
-   [ ] **Offer gift**
-   [ ] **Unavailable action reason/cost**

------------------------------------------------------------------------

# 23. Event Letters, Messages and Alerts

These are critical to RimWorld's storytelling loop: the game must tell
the player what just happened and what requires attention.

## 23.1 Messages

-   [ ] **Transient message stack**
    -   Brief.
    -   non-blocking.
    -   severity.
    -   optional world target.
-   [ ] **Rate limiting/coalescing**
    -   Repeated identical events do not spam the player.

## 23.2 Letters

-   [ ] **Letter stack**
    -   Persistent until opened/dismissed.
    -   Categories such as positive, neutral, negative and major threat.
-   [ ] **Letter window**
    -   Title.
    -   generated original narrative.
    -   relevant entities/faction.
    -   jump-to-location.
    -   action choices if event requires decision.
-   [ ] **Auto-pause option**
-   [ ] **Letter history**

## 23.3 Alerts

-   [ ] **Persistent alert list**
    -   Critical issues remain until resolved.
-   [ ] **Severity ordering**
-   [ ] **Expandable details**
    -   Which pawns/items/rooms are affected.
-   [ ] **Click cycles affected targets**
-   [ ] **Examples**
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

-   [ ] **Aggregate repeated alerts**
    -   "27 colonists need treatment" rather than 27 cards.
-   [ ] **Search/group in large colonies**
-   [ ] **Do not recompute expensive alert queries in UI every frame**

------------------------------------------------------------------------

# 24. History, Statistics and Records UI

## 24.1 History screen

-   [ ] **Colony timeline**
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

-   [ ] **World button/tab**
    -   Switch from local map to planet.
    -   Preserve local camera.
-   [ ] **Planet navigation**
    -   Rotate/pan/zoom or equivalent world navigation.
    -   Select world tile/object.

## 25.2 World tile inspector

-   [ ] **Biome**
-   [ ] **Terrain/hilliness**
-   [ ] **Elevation**
-   [ ] **Temperature**
-   [ ] **Rainfall**
-   [ ] **Growing period**
-   [ ] **Movement difficulty**
-   [ ] **Road/river**
-   [ ] **Forage**
-   [ ] **Settlement/faction if occupied**
-   [ ] **Distance/travel estimate when route context exists**

## 25.3 World objects

-   [ ] **Player colony**
-   [ ] **Faction settlement**
-   [ ] **Caravan**
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

-   [-] **World seed input** (placeholder label)
-   [-] **Planet coverage/size option** (placeholder label)
-   [-] **Temperature/rainfall/population options where supported** (placeholder labels)
-   [-] **Generate button** (visible, disabled)
-   [ ] **Generation progress**
    -   UI remains responsive.

## 26.2 Landing-site selection

-   [ ] **Select world tile**
-   [-] **Detailed tile card** (static placeholder; no map)
    -   biome.
    -   temperature range.
    -   growing period.
    -   terrain.
    -   rainfall.
    -   movement difficulty.
    -   stone/resource context if known.
    -   nearby factions.
-   [ ] **Search/find world features**
    -   Optional high-value usability feature.
-   [-] **Settle/start command** (Next to pawns; Start is on the pawn screen)
-   [-] **Back/regenerate** (Back only; Generate disabled)

------------------------------------------------------------------------

# 27. New Game: Scenario, Storyteller, Difficulty and Pawn Selection

## 27.1 Main menu

-   [-] **New game** (opens scenario stub)
-   [-] **Load game** (empty stub)
-   [-] **Options** (stub)
-   [-] **Plugins** (disabled)
-   [-] **Credits** (stub)
-   [x] **Quit**
-   [-] **Version/build identifier** (shows `dev` / AppConfig.Version; no commit id)

## 27.2 Scenario selection

-   [-] **Scenario list** (3 hardcoded names; not data-driven)
    -   Standard crash start.
    -   tribal/low-tech.
    -   difficult solo.
    -   rich solo/high-tech.
    -   custom scenarios.
-   [-] **Scenario detail** (placeholder description/summary)
    -   Starting people.
    -   equipment.
    -   research.
    -   special rules.
    -   original narrative description.
-   [-] **Scenario editor entry** (disabled button)

## 27.3 Scenario editor

-   [ ] **Scenario parts list**
-   [ ] **Add/remove part**
-   [ ] **Configure values**
-   [ ] **Validation**
-   [ ] **Save/load custom scenario**
-   [ ] **Plugin scenario parts**

## 27.4 Storyteller selection

-   [-] **Storyteller cards** (3 placeholder names; no art)
    -   Paced.
    -   relaxed.
    -   random.
    -   original characters/names/art.
-   [-] **Behavior description** (one-line placeholder text)
    -   Explain pacing differences without revealing unnecessary
        implementation formulas.

## 27.5 Difficulty

-   [-] **Preset difficulties** (Forgiving/Standard/Harsh labels)
-   [ ] **Custom difficulty**
    -   Threat scale.
    -   economy.
    -   mood.
    -   disease.
    -   adaptation.
    -   other exposed factors.
-   [ ] **Reset preset**
-   [ ] **Tooltip each setting**

## 27.6 Starting pawn screen

-   [-] **Candidate pawn cards** (3 fake roster lines)
-   [ ] **Select starting roster**
-   [-] **Randomize/reroll** (disabled)
-   [ ] **Character details**
    -   age.
    -   skills/passions.
    -   traits.
    -   backstories.
    -   health.
    -   incapabilities.
    -   relationships.
-   [ ] **Team warnings**
    -   No capable doctor.
    -   no grower.
    -   no constructor.
    -   no cook.
    -   no violence-capable pawn, etc.
-   [x] **Start colony** (creates sim via SimFactory; roster is still a placeholder)

------------------------------------------------------------------------

# 28. Caravan Formation UI

## 28.1 Form caravan dialog

-   [ ] **Destination selection**
-   [ ] **People list**
    -   Colonists.
    -   prisoners.
-   [ ] **Animals**
-   [ ] **Items**
-   [ ] **Food**
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
-   [ ] **Send command**

## 28.2 Caravan loading status

-   [ ] **Gathering indicator**
-   [ ] **Items remaining**
-   [ ] **Pawns/animals not ready**
-   [ ] **Cancel formation**
-   [ ] **Failure explanation**

------------------------------------------------------------------------

# 29. Caravan World Inspector

-   [ ] **Caravan member list**
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
-   [-] **Save** (disabled)
-   [-] **Load** (disabled from pause; empty stub from main menu)
-   [-] **Options** (opens stub, Back returns to pause)
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
-   [ ] **Commitment/ironman restrictions**

## 33.3 Load dialog

-   [ ] **Save list**
-   [ ] **Version**
-   [ ] **Game date**
-   [ ] **Colony**
-   [ ] **Plugin set**
-   [ ] **Compatibility warning**
-   [ ] **Missing plugin warning**
-   [ ] **Migration warning/error**
-   [ ] **Corrupt save handling**
-   [ ] **Delete save confirmation**

## 33.4 Autosave feedback

-   [ ] **Small non-blocking autosave indicator**
-   [ ] **Autosave failure alert**
-   [ ] **Rotating autosave configuration**

------------------------------------------------------------------------

# 34. Options and Accessibility UI

## 34.1 Graphics

-   [ ] **Resolution/window mode**
-   [ ] **VSync/frame cap**
-   [ ] **UI scale**
-   [ ] **Rendering quality toggles**
-   [ ] **Animation/effects density**
-   [ ] **Far-zoom detail**

## 34.2 Audio

-   [ ] **Master**
-   [ ] **Music**
-   [ ] **Effects**
-   [ ] **Ambient**
-   [ ] **UI**

## 34.3 Gameplay

-   [ ] **Pause on threat**
-   [ ] **Autosave interval**
-   [ ] **Default speed behavior**
-   [ ] **Edge scrolling**
-   [ ] **Confirmation preferences**
-   [ ] **Tutorial/learning helper**

## 34.4 Controls

-   [ ] **Key binding list**
-   [ ] **Rebind**
-   [ ] **Conflict detection**
-   [ ] **Reset defaults**

## 34.5 Accessibility

-   [ ] **UI scaling**
-   [ ] **Color-independent state indicators**
-   [ ] **Color-vision-friendly palette option**
-   [ ] **Reduced flashing/effects**
-   [ ] **Text size**
-   [ ] **Tooltip delay**
-   [ ] **Pause-friendly interaction**
-   [ ] **Keyboard navigation where practical**

------------------------------------------------------------------------

# 35. Tutorial and Learning Helper UI

## 35.1 Learning helper

-   [ ] **Contextual lesson cards**
    -   Trigger when player first encounters a mechanic.
-   [ ] **Lesson queue/history**
-   [ ] **Dismiss**
-   [ ] **Mark learned**
-   [ ] **Reopen help**
-   [ ] **Do not repeatedly interrupt experienced players**

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

-   [ ] **Developer mode toggle**
-   [ ] **Debug actions**
-   [ ] **Debug overlays**
-   [ ] **Inspector**
-   [ ] **Profiler**
-   [ ] **Incident/storyteller tools**
-   [ ] **Content/plugin tools**

## 38.2 Entity debug inspector

-   [ ] **EntityID**
-   [ ] **Components**
-   [ ] **Current job**
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

-   [ ] **Cell coordinates**
-   [ ] **Chunk IDs/activity**
-   [ ] **Region IDs**
-   [ ] **Room IDs**
-   [ ] **Walkability/path cost**
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
-   [ ] **Fire active set**

## 38.4 Debug actions

-   [ ] **Spawn item**
-   [ ] **Spawn pawn/animal**
-   [ ] **Destroy entity**
-   [ ] **Damage/heal**
-   [ ] **Add/remove health effect**
-   [ ] **Set need/mood**
-   [ ] **Teleport**
-   [ ] **Trigger mental state**
-   [ ] **Trigger incident**
-   [ ] **Set weather**
-   [ ] **Change time**
-   [ ] **Finish research**
-   [ ] **Change goodwill**
-   [ ] **Generate raid**
-   [ ] **Force job**
-   [ ] **Rebuild selected cache/index**

------------------------------------------------------------------------

# 39. Profiling and Performance UI

## 39.1 Simulation profiler

-   [ ] **Ticks per second**
-   [ ] **Average tick time**
-   [ ] **p50/p95/p99 tick time**
-   [ ] **Per-phase time**
-   [ ] **Active entities**
-   [ ] **Sleeping entities/chunks**
-   [ ] **Jobs evaluated**
-   [ ] **Path requests**
-   [ ] **Nodes expanded**
-   [ ] **Cache hit rates**
-   [ ] **Allocations**
-   [ ] **Heap**
-   [ ] **Goroutine count**

## 39.2 Hot-system drill-down

-   [ ] **Pathing**
-   [ ] **Job discovery**
-   [ ] **Rooms**
-   [ ] **Temperature**
-   [ ] **Power**
-   [ ] **Rendering**
-   [ ] **Alerts**
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

-   [ ] **Pawn search**
-   [ ] **Animal search**
-   [ ] **Item/storage search**
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

-   [ ] **Aggregated alerts**
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
-   [ ] Renderer frame rate never changes command semantics.
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

-   [ ] Architect.
-   [ ] Placement preview.
-   [ ] Orders.
-   [ ] Zones.
-   [ ] Stockpile settings.
-   [ ] Growing-zone settings.
-   [ ] Work.
-   [ ] Schedule.
-   [ ] Allowed areas.

**Player outcome:** Can organize labor and physically build a
functioning settlement.

## UI Milestone C --- Character simulation

Complete:

-   [ ] Needs/mood.
-   [ ] Health.
-   [ ] Character/skills.
-   [ ] Gear.
-   [ ] Social.
-   [ ] Prisoner basics.
-   [ ] Alerts/messages.

**Player outcome:** Can understand why individual colonists behave,
fail, suffer, or break.

## UI Milestone D --- Production and combat

Complete:

-   [ ] Bills.
-   [ ] Research.
-   [ ] Power overlay.
-   [ ] Temperature/roof overlays.
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

-   [ ] World map.
-   [ ] World generation.
-   [ ] Landing selection.
-   [ ] New-game flow.
-   [ ] Caravan formation.
-   [ ] Caravan inspector.
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
-   [ ] Mouse behavior works.
-   [ ] Keyboard behavior works where appropriate.
-   [ ] Tooltip/help text exists for non-obvious mechanics.
-   [ ] Scrolling works at realistic data size.
-   [ ] Lists/tables are virtualized when scale requires it.
-   [ ] Layout works at supported UI scales/resolutions.
-   [ ] Important states are not communicated by color alone.
-   [ ] No expensive world scan occurs every frame.
-   [ ] Automated tests cover controller/view-model logic where
    practical.
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
