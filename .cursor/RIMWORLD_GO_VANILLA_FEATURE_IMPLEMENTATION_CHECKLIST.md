# rimworld-go --- Vanilla Feature Implementation Checklist

> **Purpose:** Authoritative, implementation-ordered feature backlog for
> AI agents building `rimworld-go`.
>
> **Scope:** The target experience is the **base/vanilla RimWorld colony
> simulator**, excluding expansion-only systems from Royalty, Ideology,
> Biotech, Anomaly, and Odyssey. The exception is the **plugin/mod
> system**, which is explicitly part of this project.
>
> **Important:** This is a behavioral reference and implementation plan,
> not a directive to copy RimWorld source code, copyrighted text,
> artwork, sounds, names, exact balance values, or proprietary data.
> Implement equivalent *classes of gameplay behavior* independently,
> using original content and naming where public distribution requires
> it.
>
> For player-visible sprites, icons, UI chrome, and the asset pipeline,
> also follow `ART_DIRECTION_AND_ASSET_PIPELINE.md`. For named
> player-facing screens, also follow `MAIN_UI_SCREENS_SPECIFICATION.md`.
>
> **Checklist rule:** Agents may tick `[x]` only when the feature's
> acceptance criteria, tests, deterministic behavior, persistence
> impact, and relevant documentation are complete.

------------------------------------------------------------------------

# 0. How Agents Must Use This File

Implementation order is intentional. Later phases assume earlier
contracts exist.

For every item:

1.  Read root `AGENTS.md`.
2.  Read the nearest package `AGENTS.md`.
3.  Load only the relevant project skill(s).
4.  Search for existing canonical types before creating new ones.
5.  Implement tests with the feature.
6.  Preserve deterministic authoritative simulation.
7.  Update save/replay support if authoritative state changes.
8.  Update mod/content definitions if the feature is data-driven.
9.  If the item includes sprites, icons, UI chrome, renderer
    presentation, or asset files, follow
    `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` via
    `rimworld-go-art-assets` and update matching ART wave checkboxes
    there.
10. If the item is player-observable as a named screen, window, overlay,
    or HUD layout, follow `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` via
    `rimworld-go-ui-screens` and update matching screen-wave checkboxes
    there.
11. Tick the item only after its acceptance criteria pass.

**Status notation**

-   `[ ]` not started
-   `[-]` in progress
-   `[x]` complete
-   `[!]` blocked; add a short reason immediately after the item

------------------------------------------------------------------------

# PHASE 1 --- Executable Game and Deterministic Simulation Foundation

These are not player-facing RimWorld features by themselves, but every
vanilla system depends on them.

## 1.1 Simulation clock

-   [x] **Discrete simulation ticks**
    -   Maintain authoritative game time as integer ticks.
    -   Support deterministic advancement independent of renderer frame
        rate.
    -   Provide conversion helpers for game hours, days,
        quadrums/seasons, and years.
    -   Game speed changes the number/rate of processed ticks, never
        simulation rules.
    -   **Done when:** identical seed + commands produces identical
        state after N ticks at every game speed.
-   [x] **Pause and game-speed controls**
    -   Paused, normal, fast, and very-fast simulation modes.
    -   Threats or important events may optionally force/prompt speed
        reduction at presentation level.
    -   Headless simulation must be able to ignore real-time pacing.
-   [-] **Simulation phase pipeline** (commands → needs/health → AI/jobs → movement → clock; no combat/environment/incidents/snapshot-hash phases)
    -   Explicit ordered phases for commands, AI, jobs, movement,
        combat, health, environment, production, social/mood, incidents,
        cleanup, derived-state invalidation, and snapshot publication.
    -   Phase order must be documented and deterministic.

## 1.2 Entity foundation

-   [-] **Stable EntityID lifecycle** (allocation implemented and tested; queued destruction not yet implemented)
    -   Unique IDs for pawns, animals, items, buildings, projectiles,
        plants, corpses, etc.
    -   IDs are never pointer addresses.
    -   Destruction is queued and committed safely outside active
        iteration.
-   [x] **Component/entity storage**
    -   Efficient dense iteration for hot components.
    -   Stable lookup by EntityID.
    -   No reflection-heavy runtime ECS requirement.
    -   Removal must not accidentally make gameplay depend on
        swap-remove order.
-   [-] **Canonical coordinates** (only a local tile/cell coordinate exists; map ID, chunk coordinate, world tile coordinate, and distance/neighbor helpers are missing)
    -   Tile/cell coordinate.
    -   Map identifier.
    -   Chunk coordinate.
    -   World tile coordinate.
    -   Distance helpers and neighbor iteration.

## 1.3 Determinism

-   [-] **Deterministic RNG streams** (seeded, non-global RNG implemented; not yet keyed by tick/entity/operation/sequence into separate streams)
    -   RNG keyed by stable inputs such as seed, tick, entity,
        operation, and local sequence.
    -   No global `math/rand` for gameplay.
    -   Unrelated random operations should not unnecessarily perturb
        other systems.
-   [ ] **Stable command ordering**
    -   All player/AI external authoritative mutations have stable
        target tick and sequence.
    -   Equal-tick commands resolve predictably.
-   [ ] **Canonical state hashing**
    -   Hash authoritative state in stable order.
    -   Support whole-simulation and subsystem/entity hashes for
        debugging.
-   [ ] **Replay command stream**
    -   Re-run seed/scenario + ordered commands.
    -   Verify state hashes at checkpoints.

## 1.4 Headless execution

-   [-] **Headless simulation executable** (`cmd/simulate` creates a colony and advances N ticks; does not yet emit a state hash)
    -   Create/start a colony without renderer/audio.
    -   Advance a requested number of ticks.
    -   Emit metrics and final state hash.
-   [-] **Deterministic benchmark runner** (NG-1 ENG-001: `cmd/benchmark` and the grown-colony fixture time ticks, phases and allocations at 120, 250 and 500 tiles a side with 20 colonists; the colonist-count scales of 100 to 1,000 are not covered)
    -   Fixed scenarios for 3, 20, 100, 500, and 1,000 colonist-scale
        testing.
    -   Record tick cost and allocations.

------------------------------------------------------------------------

# PHASE 2 --- Map, Terrain, Camera, Selection, and Basic Presentation

The first playable surface.

## 2.1 Local map grid

-   [x] **Finite tile map** (rectangular grid with walkability as a bitset; per-cell indexes for plants, buildings, frames and items; maps tested up to 500x500, sizes above 120 are not yet offered in the wizard, ENG-001)
    -   Rectangular local map.
    -   Every cell can hold terrain and spatial occupants.
    -   Efficient cell queries.
-   [x] **Chunk partitioning** (ENG-001: 16x16 chunks with dirty tracking, walk regions, incremental rooms, per-chunk plant growth, baked render chunks)
    -   Partition maps into chunks for spatial indexing, rendering,
        sleeping, dirty flags, and localized rebuilds.
-   [-] **Terrain definitions** (soil, rich soil, gravel, rough stone, natural rock, sand, marsh, ice, shallow and deep water, mud, dirt track with walkability, walk cost, wetness + fertility; no beauty/flammability per terrain)
    -   Soil/fertility-bearing ground.
    -   Rough natural terrain.
    -   Stone/rock terrain.
    -   Water or otherwise impassable terrain.
    -   Constructed floors later plug into the same terrain model.
    -   Terrain exposes walkability, movement cost, fertility,
        beauty/cleanability, build support, flammability where
        applicable.
-   [-] **Map edges and valid placement** (out-of-bounds handling implemented and tested; arrival/departure edge semantics not applicable yet)
    -   Out-of-bounds handling.
    -   Edge cells usable for arrivals/departures where allowed.

## 2.2 Rendering

-   [-] **Tile rendering** (domain terrain drawn from original art; rock and terrain drawn for visible tiles only, linked rock/wall connectivity cached until terrain or buildings change (UI-017); no baked chunk images yet)
    -   Draw visible terrain only.
    -   Camera culling.
    -   Chunk-aware redraw/cache strategy.
-   [-] **Entity rendering** (pawns and animals from snapshots with curved, eased, sub-frame motion and stride-driven walk/trot frames; original art for items/plants/frames/walls/beds; no projectiles)
    -   Draw pawns, items, plants, buildings, corpses, projectiles.
    -   Presentation uses read-only simulation snapshots.
-   [-] **Camera controls** (pan, zoom, and screen-to-world conversion implemented; center-on-selection not implemented)
    -   Pan.
    -   Zoom.
    -   Center on selected entity/event.
    -   Screen-to-world tile conversion.
-   [-] **Selection** (single click-to-select pawn with vendor crosshair overlay; drag-select multiple not implemented)
    -   Click entity/tile.
    -   Drag-select multiple player pawns.
    -   Selection overlays.
    -   Selection state remains presentation-only.

## 2.3 Inspect UI

-   [-] **Generic inspect panel** (name, job, status/reason, tile, food/rest bars; Health/Social/Gear tabs disabled)
    -   Label/type.
    -   Hit points/condition where relevant.
    -   Position.
    -   Basic description.
    -   Context-sensitive tabs supplied by entity capabilities.
-   [-] **Pawn inspect panel** (name, job/activity, food/rest, drafted state + Draft button; no mood/gear/skills/social)
    -   Name.
    -   Current job/activity.
    -   Draft status.
    -   Health summary.
    -   Needs/mood summary.
    -   Equipment.
    -   Skills.
    -   Social information as systems become available.

------------------------------------------------------------------------

# PHASE 3 --- Pawn Generation and Character Model

RimWorld's colonists are individually simulated characters rather than
interchangeable workers.

## 3.1 Pawn identity

-   [-] **Human pawn definition** (NG-1 SIM-032: names, age, sex, build, look, backstories, traits, skills; neutral legacy bios for old saves; no per-pawn faction field)
    -   Name.
    -   Biological/chronological age representation.
    -   Sex/gender field as required by game mechanics.
    -   Faction.
    -   Player-control state.
    -   Story/character metadata.
-   [-] **Procedural pawn generation** (NG-1: deterministic from (seed, slot, roll) with scenario constraints; no relationships)
    -   Generate coherent pawn data from seed.
    -   Skills, passions, traits, backstories, age, health, and
        relationships use deterministic generation.
    -   Scenario generation can constrain generated pawns.

## 3.2 Backstories

-   [-] **Childhood and adulthood background system** (NG-1: 59 original backstories with skill changes and incapabilities, adults have both; defined in Go, not yet data files or moddable)
    -   Backgrounds modify starting skills/stats.
    -   Backgrounds may prohibit categories of work.
    -   Adults can have both childhood and adulthood histories; younger
        characters may have only childhood.
    -   Definitions must be data-driven and moddable.
    -   Use original writing/content rather than copying RimWorld
        backstory text.

## 3.3 Traits

-   [x] **Permanent trait system** (NG-1: 19 original traits, conflict groups, declared stat/thought/work modifiers)
    -   Traits modify statistics, needs, mood behavior, work behavior,
        social behavior, combat, movement, or mental-break tendencies.
    -   Trait definitions declare modifiers rather than hardcoding all
        trait effects.
    -   Support mutually exclusive/conflicting traits.
-   [-] **Trait-aware stat calculation** (NG-1: fixed layers base, body, age, conditions, traits, difficulty; no equipment or environment layers)
    -   Fixed deterministic modifier pipeline:
        `base -> body/health -> skills -> traits -> equipment -> environment -> temporary effects`.

## 3.4 Skills

-   [-] **Core skill set** (NG-1: 11 skills, no Artistic)
    -   Shooting.
    -   Melee.
    -   Construction.
    -   Mining.
    -   Cooking.
    -   Plants.
    -   Animals.
    -   Crafting.
    -   Artistic.
    -   Medical.
    -   Social.
    -   Intellectual/research.
-   [x] **Skill levels** (NG-1: 0 to 20; work speed 580 to 1780 permille, yields, accuracy, fumbles below Plants 4)
    -   Numeric proficiency.
    -   Skill influences speed, quality, success chance, accuracy,
        yield, or other system-specific outcomes.
-   [x] **Passions** (NG-1: none/minor/major learn at 35/100/150 %, joy while working, a thought for a major passion)
    -   No passion / minor passion / major passion equivalents.
    -   Passion changes learning speed and can affect enjoyment/mood
        while performing relevant work.
-   [x] **Skill XP** (NG-1: work, combat and chats grant experience, 4000 per skill per day saturation, decay above level 10 with a no-flap band)
    -   Work and relevant activities grant XP.
    -   Skill progression is deterministic.
    -   Add skill decay only if the chosen vanilla target version
        requires it.

## 3.5 Work incapabilities

-   [x] **Disabled work categories** (NG-1: backstories and traits set incapabilities; the Work tab hatches those cells with the reason, priorities are forced to 0 and orders are refused)
    -   Pawn history/traits/condition may prohibit categories such as
        violence, skilled labor, caring, dumb labor, etc.
    -   Work UI clearly shows unavailable work.
    -   Job search never assigns prohibited work.

------------------------------------------------------------------------

# PHASE 4 --- Navigation and Pawn Movement

## 4.1 Walkability

-   [-] **Walkable/blocked cells** (grid bounds; completed walls block, doors/floors/furniture walkable via construction defs; builder ejected from a finished wall; no terrain/hazards)
    -   Terrain, walls, doors, buildings, hazards, and map boundaries
        contribute to traversal rules.
-   [ ] **Movement cost**
    -   Terrain and conditions influence movement time.
    -   Pawn movement speed stat affects traversal.

## 4.2 Pathfinding

-   [-] **Deterministic A**\* (8-connected octile costs 10/14, no cutting wall corners, optional per-cell extra cost; f/g/y/x tie-break; no region graph)
    -   Exact tile pathfinding.
    -   Stable tie-breaking.
    -   No dependence on map iteration.
-   [ ] **Region graph**
    -   Build connected walkable regions.
    -   Rebuild only dirty local topology.
    -   Use region reachability to reject impossible jobs cheaply.
-   [ ] **Hierarchical long-distance pathing**
    -   Region-level route followed by local tile paths.
    -   Designed to scale beyond original RimWorld colony sizes.
-   [ ] **Path worker pool**
    -   Pure snapshot-based path computation.
    -   Sequence/revision tagged results.
    -   Ordered authoritative commit.

## 4.3 Movement

-   [x] **Follow path** (A* path; per-creature step timing: straight 15 ticks, diagonal 21)
    -   Pawns traverse cell paths over time rather than teleporting.
    -   Movement updates position at deterministic tick boundaries.
-   [x] **Pawn collision and occupancy** (PLAY-010 SIM-037: colonists and tame animals walk through each other; wild animals and raiders block; a drafted, attacking or fleeing colonist never ends a move on another colonist's tile; downed/sleeping/carried pawns can be stepped over)
    -   Prevent invalid stacking where gameplay requires exclusivity.
    -   Handle passing and temporary congestion.
-   [x] **Deterministic movement conflict arbitration** (ID-ordered moves; one fixed yielder per ring of walkers sidesteps, repaths or backs off)
    -   Multiple pawns attempting same cell resolve with stable
        priority/tie-break.
-   [x] **Local congestion avoidance** (wait first; repath with extra cost around creatures at most every 45 blocked ticks; job fails as blocked after 600; SIM-027: a pawn behind a non-yielding blocker gives up after 2 s and working pawns on adjacent cells shuffle aside)
    -   Avoid constant global repathing.
    -   Optional short-lived congestion cost.

------------------------------------------------------------------------

# PHASE 5 --- Jobs, Work Priorities, Scheduling, and Player Orders

This is the heart of autonomous colony behavior.

## 5.1 Job engine

-   [-] **Single current job per pawn** (`job.Record` kind/state/target/path/reservations; Move/Eat/Sleep/Haul/Construct/Harvest)
    -   Job type.
    -   Targets.
    -   State.
    -   progress.
    -   failure reason.
    -   reservations.
    -   interruptibility.
-   [-] **Explicit job state machines** (Goto/Do/Complete/Fail incl. deconstruct; drafted pawns hold; no medical/mental states)
    -   Go to target.
    -   Wait/work.
    -   Carry/use/operate.
    -   Complete.
    -   Fail/cancel.
    -   Every transition is deterministic and testable.
-   [-] **Job interruption** (player `MovePawn`/`Prioritize`/`DraftPawn`, critical hunger/rest, target gone, reservation fail; SIM-027: blocked (15 s) or unreachable (1 h, reset on walkability change) work is postponed per pawn+job+target, givers skip postponed candidates, blocked player orders retry 3 times then drop with a message; prioritized jobs ignore tiredness; no danger/mental)
    -   Player order.
    -   Drafting.
    -   danger.
    -   critical need.
    -   target destroyed.
    -   reservation lost.
    -   medical emergency.
    -   mental state.

## 5.2 Reservations

-   [-] **Entity reservation** (exclusive pawn→entity; no quantity splits)
-   [-] **Tile reservation** (haul dest / move target; a delivery route reserves its spare stockpile tile)
-   [-] **Item quantity reservation** (in-flight material per frame/fuel user: haulers' shares are booked so a frame is never over-delivered (HAUL-001); no split reservations on a source stack)
-   [-] **Bed/patient reservation** (bed `Claimant` for Sleep; no patient/medical)
-   [ ] **Workstation reservation**
-   [-] **Automatic reservation cleanup** (complete/cancel/replace/death; no target-deletion index)
    -   Completion.
    -   cancellation.
    -   death/downing.
    -   target deletion.
    -   job replacement.

## 5.3 Work settings

-   [-] **Work types** (Haul/Construct/Grow wired; other columns are UI stubs)
    -   Firefight.
    -   Patient.
    -   Doctor.
    -   Bed rest.
    -   Basic/general maintenance.
    -   Warden/prisoner interaction.
    -   Handle animals.
    -   Cook.
    -   Hunt.
    -   Construct.
    -   Grow.
    -   Mine.
    -   Plant cut.
    -   Smith/craft/tailor categories or equivalent production
        categories.
    -   Art.
    -   Haul.
    -   Clean.
    -   Research.
-   [-] **Simple work enable/disable** (`SetWorkEnabled` for Haul/Construct/Grow)
    -   Pawn either performs a work type or does not.
-   [x] **Manual priorities** (per-colonist 1-4 or off per work type; AI tries priority 1 work first; ties follow work.Order)
    -   Multiple priority levels.
    -   Lower numeric priority means more urgent, equivalent to
        RimWorld's 1--4 behavior.
    -   Work-type ordering resolves equal priorities.
-   [ ] **Work capability display**
    -   Skill level.
    -   passion.
    -   incapable work.
    -   current assignment.

## 5.4 Indexed work discovery

-   [ ] **Work opportunity indexes**
    -   Do not scan every target for every pawn.
    -   Index construction, hauling, growing, mining, cleaning,
        doctoring, etc. by local region/chunk/type.
-   [ ] **Candidate scoring**
    -   Work priority.
    -   urgency.
    -   distance approximation.
    -   skill suitability where appropriate.
    -   reservation state.
    -   allowed area.
    -   exact path only for finalists.

## 5.5 Forced orders

-   [x] **Right-click/context forced work** (`OrdersAt` menu: go here, build, supply wood, deconstruct, harvest, sleep in bed, eat, haul to stockpile; disabled entries give the reason; `Prioritize` command)
    -   Player can tell a selected pawn to perform a valid specific
        action immediately.
-   [x] **Queued forced jobs** (Shift queues up to 3 orders behind the current one; queue panel with Clear and numbered route on the map; invalid orders skipped with a message)
    -   Support shift-style job queuing.
    -   Queue shown in pawn UI.
    -   Invalid queued tasks fail gracefully.

## 5.6 Daily schedule

-   [x] **24-hour schedule grid** (per-colonist grid, copied from the colony default; Anything/Work/Recreation/Sleep; painted in the Schedule tab)
    -   Anything.
    -   Work.
    -   Sleep.
    -   Recreation.
-   [-] **Schedule-aware intent** (bedtime overrides any job or prioritized order except drafting, eating first if hungry; recreation and idle time run free-time behaviours; tired colonists keep working through Work hours until they collapse; waking fully rested in a Sleep block turns the rest of it into free time until tired again)
    -   Schedule biases/limits autonomous intent but emergencies and
        critical needs can override appropriately.

## 5.7 Allowed areas

-   [ ] **Named allowed areas**
    -   Paint cells.
    -   invert/clear.
    -   assign pawn.
-   [ ] **Area-aware AI**
    -   Pawn chooses work/destinations inside assigned area.
    -   Path traversal semantics should emulate vanilla's distinction
        between allowed destination and traversal where appropriate.

------------------------------------------------------------------------

# PHASE 6 --- Items, Carrying, Inventory, Stockpiles, and Hauling

## 6.1 Item system

-   [-] **Item definitions** (food and wood stacks only)
    -   Resource/material.
    -   food.
    -   medicine.
    -   weapon.
    -   apparel.
    -   drug.
    -   manufactured component.
    -   corpse/product categories.
-   [-] **Stackable items** (stack limit 1000, carry 100 per trip, merge/take, no negatives)
    -   Quantity.
    -   stack limit.
    -   merge/split.
    -   no negative or duplicated quantity.
-   [ ] **Item quality**
    -   Quality tiers for relevant crafted
        items/buildings/apparel/weapons/art.
    -   Quality affected by creator skill and other factors.
    -   Exact RimWorld probabilities need not be copied.
-   [ ] **Item hit points/deterioration**
    -   Items can take damage.
    -   Outdoor/unroofed or unsuitable storage can cause deterioration
        where applicable.
-   [x] **Item forbidden state** (SIM-028: `Stack.Forbidden`; haul, eat, equip, bills, refuel, troughs and burial skip it; raid weapons and bodies drop forbidden; Forbid/Allow area tools, item toggle, F key; follow-ups: forced Prioritize on a forbidden item is allowed, stacks forbidden mid-haul are not interrupted)
    -   Player can forbid/unforbid items.
    -   Autonomous hauling/use ignores forbidden targets unless
        explicitly ordered where appropriate.

## 6.2 Carrying/inventory

-   [-] **Pawn carry tracker** (haul `Carrier`, shown as a load in both arms; no personal inventory)
    -   Pawn can carry an item stack while performing jobs.
-   [ ] **Personal inventory**
    -   Support carried consumables/medicine/other inventory where
        vanilla behavior needs it.
    -   Inventory distinct from equipped weapon and worn apparel.

## 6.3 Stockpile zones

-   [-] **Stockpile creation** (paint tiles; one zone per tile; adjacent stockpiles merge; food/wood/stone filter set per zone, no filter UI yet (ZONE-001))
    -   Paint storage cells.
-   [-] **Storage filters** (food and wood allowed; no category tree/quality)
    -   Category tree.
    -   allowed/disallowed definitions.
    -   quality/condition filters where appropriate.
-   [ ] **Storage priorities**
    -   Multiple priority levels.
    -   Haulers move goods from lower-priority to higher-priority valid
        storage.
-   [ ] **Storage capacity/index**
    -   Efficiently find valid free cells.
    -   Avoid map-wide storage scans.

## 6.4 Hauling

-   [-] **Haul loose item to storage** (up to 100 per trip to the nearest accepting stockpile tile, topping up stacks first)
-   [-] **Haul construction resources** (a full load of wood or stone feeds a route of up to 8 nearby frames; leftovers go to the nearest stockpile, or only the needed amount is carried when storage has no room (HAUL-001))
-   [ ] **Haul production ingredients**
-   [ ] **Haul finished products**
-   [x] **Haul corpses**
-   [-] **Opportunistic/batched hauling where compatible** (batched for construction materials and refuelling; not yet for general goods)
    -   Performance-conscious without changing core player expectations.

------------------------------------------------------------------------

# PHASE 7 --- Designations, Construction, Buildings, Rooms, and Zones

## 7.1 Designation system

-   [-] **Build designation** (`DesignateBuild` for wall/door/floor/bed/table/chair/lamp with facing; 1×1 footprints only)
-   [x] **Deconstruct designation** (Architect Orders rectangle; cancel clears it)
-   [x] **Mine designation** (Architect Orders rectangle over rock; Mine work type; cancel clears)
-   [x] **Cut plant designation** (trees drop wood scaled by growth; bushes cleared)
-   [x] **Harvest designation** (wild plants by order; zone crops harvest automatically; Grow work type)
-   [x] **Hunt designation**
-   [-] **Cancel designation** (frames refund delivered materials (UI-015 `CancelFrame` and the Cancel area), harvest marks, deconstruct marks; zones lose just the cancelled tiles (ZONE-001))
-   [x] **Forbid/unforbid designation/action** (SIM-028 Forbid/Allow area tools and item toggle; UI-016 F key)

## 7.2 Construction lifecycle

-   [-] **Blueprint** (`CanPlace` shared by preview and sim: bounds, layer occupancy, blocked, items under walls; floor + object layers share a tile; doors and gates may replace walls and fences (deconstruct first, UI-021); no research/support checks)
    -   Placement preview.
    -   Validate terrain, occupancy, research, material, and support
        requirements.
-   [-] **Material selection** (wood, five stone blocks or steel; per-material HP and work (SIM-029); no beauty/flammability yet)
    -   Stuff/material-based buildings where relevant.
    -   Different materials influence HP, beauty, flammability,
        work/material cost, etc.
-   [-] **Resource delivery** (material hauled to frames until `Need`, several frames per trip, in-flight amounts prevent over-delivery (HAUL-001); NG-1: frames of advanced buildings then wait for components)
    -   Haulers/builders deliver required materials.
-   [-] **Construction frame** (delivered materials tracked per frame with in-flight amounts; UI-015 frame inspector shows them)
    -   Delivered materials are committed to a partially constructed
        entity.
-   [-] **Construction work** (NG-1: Construction skill scales the work speed; no quality or failed construction)
    -   Construction skill affects speed and result.
    -   Failed construction may waste resources if independently chosen
        balance supports it.
-   [-] **Completed building** (blocking defs set unwalkable and eject pawns; beds claimable; facing kept; no room/power invalidate)
    -   Replace frame with functional building.
    -   Invalidate navigation/rooms/power/etc. only as needed.
-   [x] **Repair** (builders mend damaged buildings once no raiders are standing)
    -   Damaged player structures become repair work.
-   [x] **Deconstruction** (construct work type; half the wood refunded; tile and bed claim freed)
-   [x] **Reinstall (move)** (UI-011: a reinstall frame, no materials, half the work; the same building moves with bills, fuel, owners and HP; walls, doors, floors and graves stay)
    -   Work removes building and yields appropriate salvage.

## 7.3 Core structures

-   [-] **Walls** (block pathing, bound rooms, per-material HP/work, link visually to rock and each other (UI-017); no vision blocking)
    -   Block movement/vision/projectiles.
    -   Define rooms.
    -   Material-dependent stats.
-   [-] **Doors** (buildable, walkable for colonists, shut to raiders and wild animals, count as wall neighbours, drawn open when occupied; no opening delay)
    -   Open/closed states.
    -   Opening delay.
    -   faction/permission interaction.
    -   affect pathing and room boundaries.
-   [-] **Floors** (buildable/deconstructable separate layer; SIM-028: may go under any furniture, not on rock, walls, fences/gates or dug graves; no movement/beauty/cleanliness stats)
    -   Construct/remove.
    -   movement, beauty, cleanliness, flammability attributes.
-   [-] **Roofs** (SIM-018; no deterioration, toxic fallout or collapse injuries)
    -   Roof cells.
    -   roof construction/removal jobs.
    -   support rules.
    -   roof affects weather, temperature, light, deterioration, toxic
        fallout.
-   [-] **Furniture** (bed (rotatable, claimed for Sleep), table, chair (rotatable), lamp buildable; no comfort/joy/light effects)
    -   Beds.
    -   tables.
    -   chairs/stools.
    -   recreation furniture.
    -   shelves/storage furniture if target version includes vanilla
        shelf behavior.

## 7.4 Rooms

-   [x] **Room detection**
    -   Enclosed connected spaces bounded by walls/doors.
    -   Outdoor/unroofed semantics.
-   [x] **Room statistics**
    -   Space.
    -   beauty.
    -   cleanliness.
    -   wealth/value.
    -   impressiveness derived from room properties.
-   [x] **Room roles**
    -   Bedroom.
    -   barracks.
    -   dining room.
    -   recreation room.
    -   hospital.
    -   workshop.
    -   prison cell/barracks.
    -   storage/freezer is emergent rather than necessarily a formal
        role.
-   [x] **Room stat caching/invalidation**
    -   Recalculate only affected rooms.

## 7.5 Specialized zones

-   [x] **Growing zones** (painted tiles sowing the chosen crop; never on taken tiles or infertile rough stone/rock; same-crop neighbours merge; cancel clears tiles; resow after harvest (ZONE-001))
-   [-] **Home area** (SIM-021: painted, auto-grown around buildings, drives cleaning; no firefighting or repair yet)
    -   Influences firefighting/cleaning/repair behavior.
-   [x] **Roof build/remove areas** (SIM-018)
-   [ ] **Allowed areas**
    -   Already introduced in Phase 5 but integrated with zone UI.

------------------------------------------------------------------------

# PHASE 8 --- Food, Hunger, Farming, Cooking, and Survival

## 8.1 Nutrition

-   [x] **Nutrition values** (per-kind table: ration 4000, berries 1500, amberroot 2000, meat 2200, meal 6000)
    -   Foods provide nutrition.
    -   Pawns consume an amount based on hunger/body needs.
-   [-] **Food need** (permille decay; 0 downs then starvation death; no mood)
    -   Hunger decays over time.
    -   Thresholds affect behavior/mood.
    -   Severe hunger causes malnutrition/starvation and can kill.
-   [-] **Food search** (nearest edible stack; entity reserved; SIM-028: forbidden stacks skipped, animals never pick food behind fences; no diet)
    -   Prefer valid edible food according to restrictions and
        availability.
    -   Reserve food before fetching it.
    -   Respect forbidden/allowed area rules.
-   [x] **Eating job** (fetch the portions needed, carry them to a table seat - chair preferred - within 30 tiles, ingest, table/raw/meal thoughts)
    -   Fetch.
    -   choose table/seat when reasonable.
    -   ingest over time.
    -   gain nutrition.
    -   generate context-dependent thought later.

## 8.2 Raw food

-   [-] **Plant food** (harvest yield stacks; no meat/diet categories)
-   [x] **Meat**
-   [-] **Animal products** (SIM-026: milk, wool, plain and fertile eggs, ember oil; Handle work type, Milk/Shear jobs; young give no reduced meat, no per-species yield tuning UI)
    -   Milk/eggs where species definitions support them.
-   [ ] **Food categories and diet compatibility**

## 8.3 Cooking

-   [x] **Cooking workstation** (stove)
-   [-] **Meal recipes** (simple meal at the stove, or slower over a fuelled campfire; no fine meal / multi-ingredient recipes yet)
    -   Simple/standard meal classes sufficient to recreate vanilla
        progression.
    -   Ingredient filters.
    -   nutrition conversion.
    -   work amount.
-   [ ] **Food poisoning**
    -   Chance influenced by cook skill and cleanliness/conditions.
    -   Health effect and mood consequences.
-   [x] **Butchering**
    -   Corpse -\> meat + leather/other products.
    -   Yield affected by species and skill/stat.

## 8.4 Farming

-   [x] **Fertility**
    -   Terrain fertility affects plant growth.
-   [-] **Crop definitions** (7 crops: grow days, yield, soil sensitivity, frost-kill temperature, shelf life; no minimum skill)
    -   Growth time.
    -   fertility sensitivity.
    -   yield.
    -   minimum sowing skill.
    -   temperature/light requirements.
-   [x] **Sowing** (Grow work sows empty fertile zone cells; prioritizable from the order menu; tree zones are planted, chopped when grown and replanted (UI-011))
    -   Growers plant designated crops in growing zones.
-   [-] **Plant growth** (integer per-tick growth scaled by fertility x soil sensitivity x temperature; daylight only; frost kills crops; no plant health)
    -   Analytical/bucketed growth.
    -   Depends on temperature, light, fertility, plant health.
-   [x] **Harvest maturity** (plants ripen at GrowTicks; berry bushes regrow to 35% after harvest)
    -   Mature plants generate harvest work.
    -   Skill can affect yield/failure.
-   [x] **Wild plants** (seeded generation scatters trees and berry bushes on fertile ground; PLAY-010 SIM-038: per-biome bramble thicket (slows walking), ground moss, desert rosette, ball cactus, elephant ear, fire lily, dandelions)
    -   Map generation creates harvestable/non-harvestable vegetation.

## 8.5 Food spoilage

-   [x] **Rot progression** (per-stack rot ticks; full speed from 10C, slower when cool, none at 0C or below; merges average by quantity)
    -   Perishable food/corpses rot based on temperature and elapsed
        time.
-   [-] **Frozen/refrigerated preservation** (winter and cold rooms freeze food; no cooler/freezer building yet)
-   [x] **Drying** (drying rack dries up to 25 of any perishable food per batch over ~1-2.5 days; dried food never rots)
    -   Cold slows or stops rot according to independently selected
        balance.
-   [x] **Rotten state** (spoiled food is destroyed with a message; rotten corpses can't be butchered and decay away)
    -   No longer normal edible food.
    -   Corpse rot affects handling/visual state.

------------------------------------------------------------------------

# PHASE 9 --- Rest, Beds, Comfort, Recreation, Beauty, and Mood

Vanilla RimWorld's characters react psychologically to their
surroundings and experiences. Basic needs include food and rest; human
colonists additionally care about recreation, beauty, comfort, and being
outdoors, while some traits create chemical-related needs. Mood is
derived from thoughts and traits and low mood creates mental-break risk.
citeturn1search0turn1search5

## 9.1 Rest

-   [-] **Rest need** (awake decay, sleep restore 20%->100% in 8 h, sleepers wake at 80% outside Sleep hours and 100% in them; at 0 the colonist collapses and sleeps where it drops until 80%; hunger 0 downs, not rest; no rest effectiveness by bed quality yet)
    -   Decays while awake.
    -   Replenishes while sleeping.
    -   Severe exhaustion can force sleep.
    -   Tiredness affects mood and performance.
-   [-] **Sleep intent** (owned bed else nearest free bed else current tile, also when the bed is out of reach; schedule-aware: tired sleep only outside Work hours)
    -   Schedule-aware.
    -   Select owned/assigned bed if available.
    -   Fall back to available sleeping location.
-   [-] **Beds** (claim on Sleep; no comfort/medical/prisoner)
    -   Assign ownership.
    -   sleeping slots.
    -   rest effectiveness.
    -   comfort.
    -   medical/prisoner designation later.

## 9.2 Comfort

-   [ ] **Comfort need**
    -   Current furniture/bed comfort drives a slowly changing need.
    -   Comfortable beds/chairs create positive psychological effect.

## 9.3 Beauty

-   [ ] **Tile beauty**
    -   Buildings, items, filth, natural objects, art, and terrain
        contribute.
-   [ ] **Perceived beauty**
    -   Pawn samples local surroundings.
    -   Cached/low-frequency evaluation.
-   [ ] **Beauty need**
    -   Gradually trends toward environmental perception.
    -   Generates positive/negative thoughts at thresholds.

## 9.4 Outdoors

-   [ ] **Outdoors need**
    -   Tracks time spent roofed/indoors versus outdoors.
    -   Trait modifiers can alter preference.

## 9.5 Recreation

-   [x] **Recreation need** (drains ~4300/day awake, refilled by free-time activities; bored/starved/entertained thoughts)
    -   Decays.
    -   Pawns seek recreation during suitable schedule/idle time.
-   [-] **Recreation types** (wander, chat, relax on a chair, fireside, horseshoes; no cerebral/tech types)
    -   Solitary relaxation.
    -   social interaction.
    -   dexterity-style play.
    -   cerebral/game-style play.
    -   telescope/television/other technology-gated categories as
        vanilla-equivalent content is added.
    -   chemical/food enjoyment where appropriate.
-   [ ] **Recreation tolerance/variety**
    -   Repeating one recreation category reduces its effectiveness.
    -   Encourage multiple recreation sources.
-   [-] **Recreation jobs/buildings** (no-building activities plus the horseshoe pin)
    -   Basic no-building recreation.
    -   horseshoe-like dexterity object.
    -   chess/game table equivalent.
    -   television-like electronic recreation.
    -   telescope-like late recreation.
    -   social relaxation.

## 9.6 Thoughts/memories

-   [x] **Thought definition system**
    -   Mood offset.
    -   duration.
    -   stacking rules.
    -   situational versus memory-based.
-   [x] **Situational thoughts**
    -   Hungry/tired.
    -   environment beauty.
    -   room quality.
    -   ate without table.
    -   slept on ground.
    -   darkness/environment.
    -   pain.
    -   observed corpse/ugliness.
    -   comfort/recreation satisfaction.
    -   expectations.
-   [x] **Memory thoughts**
    -   Death of relation.
    -   social interaction.
    -   wedding/relationship events.
    -   combat/trauma where applicable.
    -   impressive meal/room/event equivalents.
-   [x] **Thought expiry**
    -   Memories expire deterministically after defined duration.

## 9.7 Mood

-   [x] **Mood target**
    -   Sum baseline, expectations, thoughts, traits, health, social
        effects, environment.
-   [ ] **Smoothed current mood**
    -   Current mood trends toward target rather than instantaneously
        matching it.
-   [ ] **Break-risk thresholds**
    -   Minor/major/extreme risk zones exposed to UI.
-   [x] **Mood UI**
    -   Show current mood, target, active positive/negative thoughts,
        and break risk.

------------------------------------------------------------------------

# PHASE 10 --- Mental States, Breaks, Inspirations, and Autonomous Social Behavior

## 10.1 Mental breaks

-   [-] **Mental break eligibility** (hourly roll below mood 20, 2% per point; no severity tiers/traits)
    -   Low mood sustained below thresholds.
    -   Break severity influenced by current threshold/traits.
-   [x] **Mental state framework** (break replaces work, schedule, orders and drafting; explicit start/end; needs still met when critical)
    -   Temporarily replaces normal work/player control.
    -   Explicit start/end.
    -   May select targets.
    -   Save/replay safe.
-   [x] **Minor break behaviors** (sad wander, food binge, hide in room)
    -   Sad wandering.
    -   food binge.
    -   drug binge where available.
    -   hide-in-room style isolation.
-   [ ] **Destructive/aggressive breaks**
    -   Insulting spree.
    -   tantrum/object destruction.
    -   berserk/aggression.
    -   prisoner-like escape behavior where relevant.
-   [ ] **Catatonic/extreme shutdown equivalent**
    -   Pawn becomes temporarily unavailable and requires care.
-   [x] **Recovery** (break ends after its duration with a day of catharsis +30)
    -   End state.
    -   mood catharsis/recovery thought if desired by independent
        balance.

## 10.2 Inspirations

-   [ ] **High-mood inspiration chance**
    -   Positive temporary state after sustained good mood.
-   [ ] **Inspiration types**
    -   Inspired creativity/quality.
    -   inspired recruitment.
    -   inspired trade.
    -   inspired taming.
    -   inspired surgery/work equivalents as supported.

## 10.3 Social interactions

-   [ ] **Periodic social interaction**
    -   Nearby awake pawns may chat, insult, compliment, etc.
    -   Sparse event-driven relationships, never all-pairs every tick.
-   [ ] **Opinion**
    -   Pawn A's opinion of Pawn B is directional.
    -   Influenced by relationship, traits, memories, social
        interactions.
-   [ ] **Social fight**
    -   Negative interaction can escalate to non-lethal melee conflict.

## 10.4 Relationships

-   [ ] **Family relationships**
    -   Parent/child.
    -   sibling.
    -   spouse/partner.
    -   other close-family links needed for social thoughts.
-   [ ] **Romance**
    -   Attraction/compatibility.
    -   romance attempt.
    -   lover relationship.
    -   rejection memories.
-   [ ] **Marriage**
    -   Proposal/engagement.
    -   marriage ceremony/event.
    -   relationship/mood consequences.
-   [ ] **Breakups**
    -   Relationship can end.
    -   social/mood memories.
-   [ ] **Relationship death reactions**
    -   Death of friend/lover/spouse/family creates stronger memories.

------------------------------------------------------------------------

# PHASE 11 --- Health, Anatomy, Damage, Disease, Medicine, and Death

RimWorld models injuries and conditions on individual body parts; damage
to specific parts changes capacities, and prosthetics/transplants can
replace missing functionality. Diseases, infection, pain, consciousness,
treatment, and surgery are major colony-management systems.
citeturn2search3turn2search11

## 11.1 Anatomy

-   [ ] **Species anatomy tree**
    -   Body -\> torso/head -\> limbs -\> hands/feet -\> digits, organs,
        etc.
    -   Species can define different anatomy.
-   [ ] **Body-part health**
    -   Each part has maximum health.
    -   Damage can destroy/remove a part.

## 11.2 Capacities

-   [ ] **Pawn capacities**
    -   Consciousness.
    -   Moving.
    -   Manipulation.
    -   Sight.
    -   Hearing.
    -   Talking.
    -   Breathing.
    -   blood filtration/circulation/metabolism equivalents as needed.
-   [ ] **Capacity derivation**
    -   Relevant body parts and health effects combine into functional
        capacity.
    -   Capacity influences work/combat/movement/social/medical
        performance.

## 11.3 Injuries

-   [ ] **Damage types**
    -   Blunt.
    -   sharp.
    -   burn.
    -   explosion-derived damage.
    -   bite/scratch may map to sharp/blunt variants.
-   [ ] **Wounds**
    -   Injury assigned to body part.
    -   severity.
    -   bleeding rate.
    -   pain.
    -   tendability.
-   [ ] **Bleeding**
    -   Blood loss accumulation.
    -   severe blood loss -\> downing/death.
    -   tending reduces/stops bleeding.
-   [ ] **Pain**
    -   Derived from injuries/conditions.
    -   affects consciousness/mood/performance.
    -   extreme pain can down pawn.
-   [-] **Downed state** (hunger at 0; no rescue/capture; rest 0 forces sleep instead of downing)
    -   Pawn cannot normally act.
    -   can be rescued/captured.
    -   not equivalent to death.
-   [-] **Death** (starvation, injury, infection, exposure; leaves a corpse and drops the weapon; colonists mourn; no social relations)
    -   Vital part destruction or lethal capacity/condition.
    -   create corpse.
    -   release jobs/reservations.
    -   trigger social/story events.

## 11.4 Healing

-   [x] **Natural wound healing**
    -   Healing over time based on pawn state and care.
-   [ ] **Scars/permanent injury**
    -   Some wounds heal into lasting impairment/pain.

## 11.5 Medical care

-   [-] **Medical care policy** (NG-1 SIM-034: care tiers untreated, tended, herbal and industrial medicine; the doctor picks by severity, no per-patient policy setting)
    -   No care.
    -   no medicine.
    -   herbal-equivalent.
    -   industrial-equivalent.
    -   best medicine-equivalent.
-   [x] **Tending** (doctor fetches herbal medicine when there is some; patients down, in bed or asleep)
    -   Doctor reserves patient.
    -   fetches medicine if policy allows.
    -   performs treatment.
    -   quality depends on skill, medicine, environment, capacities.
-   [x] **Self-tend** (only when no other doctor; weaker care)
    -   Optional with reduced effectiveness.
-   [-] **Medical beds** (NG-1: the hospital bed gives 115 % healing and half the infection chance, and rescuers prefer it; patients do not yet seek it out or have beds marked for medical use)
    -   Mark beds for medical use.
    -   injured/sick pawns seek treatment/rest.
-   [x] **Rescue**
    -   Carry downed friendly pawn to valid bed.

## 11.6 Infection

-   [x] **Wound infection chance** (untended 8h: 40%; self-tended 20%, tended 12%, herbal medicine 3%, industrial medicine 1%; halved in a hospital bed)
    -   Eligible untreated/poorly treated wounds can become infected.
-   [-] **Infection progression vs immunity** (hourly severity: +4 untreated, -5 tended, -9 with medicine; fatal at 100; no immunity race)
    -   Disease severity progresses.
    -   immunity grows based on treatment/rest/health.
    -   reaching immunity first allows recovery; severity first can
        kill.

## 11.7 Diseases

-   [ ] **Disease framework**
    -   Severity over time.
    -   stages.
    -   symptoms/stat modifiers.
    -   tendable conditions.
    -   immunity where appropriate.
-   [ ] **Representative vanilla disease classes**
    -   flu-like disease.
    -   plague-like severe disease.
    -   malaria-like tropical disease.
    -   sleeping-sickness-like disease.
    -   food poisoning.
    -   heatstroke.
    -   hypothermia.
    -   toxic buildup.

## 11.8 Age/chronic conditions

-   [ ] **Biological aging**
    -   Age increases.
    -   age can influence health/stat factors.
-   [ ] **Age-related chronic conditions**
    -   Bad back/frailty/cataract-like conditions.
    -   Data-driven health effects.

## 11.9 Surgery

-   [ ] **Operation bills**
    -   Add/remove scheduled medical operations.
-   [ ] **Anesthesia**
    -   Required for appropriate surgery.
-   [ ] **Surgery success/failure**
    -   Doctor skill.
    -   medicine.
    -   manipulation/sight.
    -   room cleanliness/bed factors.
    -   failure can injure nearby/body parts.
-   [ ] **Organ extraction/transplant**
    -   Removable organs.
    -   harvested organ becomes item.
    -   missing vital organs can kill.
    -   transplant restores/replaces functionality.
-   [ ] **Prosthetics/bionics**
    -   Replace missing/damaged body parts with artificial parts of
        varying efficiency.

## 11.10 Corpses

-   [x] **Corpse entity** (human corpse item remembers who it was and when they died; rots to bones; hauled to graves and pyres; animal corpses butchered)
    -   Stores deceased pawn identity/body.
    -   can rot.
    -   can be butchered where allowed.
    -   can be hauled.
-   [x] **Graves** (dug, no materials; haulers carry and bury the nearest body; the grave keeps name, cause and days under a headstone)
    -   Bury corpse.
    -   grave stores occupant.
-   [x] **Cremation** (funeral pyre bench: Cremate corpse bill burns a body and 2 wood)
    -   Production job destroys corpses.

------------------------------------------------------------------------

# PHASE 12 --- Temperature, Weather, Seasons, Light, Fire, and Environment

## 12.1 Temperature

-   [-] **Outdoor temperature** (temperate calendar: yearly triangle wave 9C +-15C peaking day 15, +-6C day/night swing; no weather/latitude)
    -   Driven by world tile climate, latitude/elevation/season/time,
        and weather.
-   [-] **Indoor room temperature** (per-room temperature drifting toward outside air by room size, kept across room rebuilds; no room-to-room or open-door exchange)
    -   Rooms have thermal state.
    -   Exchange heat with outdoors and neighboring rooms through
        openings/structures.
    -   Use efficient region/room approximation rather than per-cell
        fluid simulation.
-   [-] **Heat sources/sinks** (wood campfire heats its room, banks at 20C and relights below 16C; haulers refuel it; no powered heater/cooler)
    -   Campfire/heater-like source.
    -   cooler/air-conditioner-like device.
    -   passive cooler equivalent.
-   [x] **Pawn temperature comfort** (per-colonist comfort band from worn apparel; cold/freezing/hot/sweltering thoughts, hypothermia/heatstroke down and kill, colonists seek a bearable room)
    -   Apparel and species stats define safe/comfortable range.
    -   extreme heat -\> heatstroke.
    -   extreme cold -\> hypothermia.

## 12.2 Seasons/calendar

-   [-] **Year and quadrum/season cycle** (seasons centred on the temperature curve; crops grow 0-10C ramp, full 10-35C, frost kills tender crops)
    -   Seasonal temperature/growing changes.
    -   Hemisphere/world position can affect seasons.
-   [ ] **Growing period**
    -   World tile communicates expected crop-growing season.

## 12.3 Weather

-   [-] **Weather state machine** (deterministic 6-hour blocks: clear, cloudy, rain, storm, snow, blizzard by season and temperature; wind; snow depth and wetness; no fog kind)
    -   Clear.
    -   rain.
    -   fog.
    -   snow where climate supports it.
    -   thunderstorm/flashstorm-like weather.
-   [-] **Weather effects** (outdoor chill in snow, outdoor campfires doused, slower walking in rain and snow, soaked/snow-day thoughts; light only darkens visually)
    -   light.
    -   temperature.
    -   fire suppression.
    -   movement/accuracy where appropriate.
    -   solar generation.

## 12.4 Lighting

-   [-] **Day/night light** (outdoor level is a pure function of tick; indoor/roofed and artificial lights not yet) (ART-010, SIM-018: sun/moon, indoor and lamp light; no light gameplay effects)
-   [-] **Roof/indoor darkness** (ART-010, SIM-018: indoor ambient under roofs; no light-level gameplay effects)
-   [x] **Artificial lights**
-   [ ] **Light affects plants**
-   [-] **Light can affect work/mood/accuracy where appropriate**

## 12.5 Fire

-   [ ] **Flammability**
    -   Terrain/buildings/items/plants define flammability.
-   [ ] **Ignition**
    -   Weapon/explosion/lightning/random event/overheat sources.
-   [ ] **Fire spread**
    -   Active-set simulation only.
    -   nearby flammable objects.
    -   weather/rain influences.
-   [ ] **Fire damage**
    -   Buildings/items/plants/pawns take burn damage.
-   [ ] **Firefighting work**
    -   Pawns extinguish fire in home area/urgent contexts.
-   [ ] **Automatic rain after major fire**
    -   Optional vanilla-equivalent storyteller/environment behavior.

## 12.6 Environmental events

-   [-] **Heat wave** (NG-1: colonies from the wizard with the Extreme weather setting on; +12 C for 2 to 4 days in summer, 6 % a day, 3-day cooldown; letter and top-strip chip)
-   [-] **Cold snap** (NG-1: same, -14 C in winter)
-   [ ] **Eclipse**
    -   Reduces sunlight and solar generation; affects crops.
-   [ ] **Solar flare**
    -   Temporarily disables relevant electrical devices.
-   [ ] **Toxic fallout**
    -   Outdoor exposure builds toxic condition; damages/kills plants
        over time.
-   [ ] **Flashstorm**
    -   Lightning/fire threat.

------------------------------------------------------------------------

# PHASE 13 --- Cleaning, Filth, Deterioration, and Colony Maintenance

-   [-] **Filth entities/layers** (SIM-020: blood and ichor stains that fade over 7 days and wash off in rain and snow; SIM-021: dirt tracked in on boots from outdoor ground and field work, kept indoors until cleaned; no vomit or trash, animals track no dirt)
    -   Dirt, blood, vomit, trash-like filth.
    -   Created by movement/events/injuries/animals as appropriate.
-   [-] **Cleanliness** (SIM-021: room filth from dirt and stains costs up to 10 impressiveness, shown in room labels; floor properties and medical/cooking/research effects missing)
    -   Room cleanliness derived from filth/floor properties.
    -   Influences medical/cooking/research or other vanilla-equivalent
        systems.
-   [x] **Cleaning work** (SIM-021: Home tiles over 120 filth are queued, swept to 0, nearest first, reserved per tile; Clean work column)
    -   Home-area filth becomes indexed cleaning work.
-   [x] **Blood trails** (SIM-020: bleeding colonists, raiders and animals drip every third tile; untended downed pawns pool)
    -   Bleeding moving pawns can create blood filth.
-   [ ] **Outdoor deterioration**
    -   Eligible items deteriorate when exposed.
    -   Roof/storage protection matters.
-   [ ] **Building breakdown**
    -   Selected electrical/mechanical buildings can break down.
    -   Requires maintenance components/work if matching target vanilla
        behavior.
-   [x] **Repair work** (Construction work type)
    -   Damaged colony structures automatically become repair
        candidates.

------------------------------------------------------------------------

# PHASE 14 --- Production, Recipes, Bills, Crafting, Art, and Quality

## 14.1 Generic workbench system

-   [-] **Workstation** (workbench and stove: 2x1 rotatable footprints with interaction cell, block movement; no power/facilities)
    -   Supported recipes.
    -   interaction cell.
    -   power requirement where applicable.
    -   facility/environment modifiers.
-   [-] **Recipe definition** (inputs incl. any-raw-food, work, work type, product; no research/quality)
    -   ingredients.
    -   filters.
    -   work amount.
    -   skill.
    -   products.
    -   research requirement.
    -   quality eligibility.

## 14.2 Bills

-   [x] **Bill queue** (ordered bills per bench; first runnable bill wins)
    -   Multiple ordered bills per workstation.
-   [x] **Bill modes** (do X, forever, until you have X (waits at X, resumes at X - X/5, counts meals already cooking); suspend)
    -   Do X times.
    -   Do forever.
    -   Do until X inventory count.
-   [ ] **Ingredient radius**
    -   Limit ingredient search area.
-   [ ] **Ingredient filters**
    -   Restrict materials/categories.
-   [ ] **Worker restrictions**
    -   Skill range/specific pawn if desired for parity.
-   [ ] **Product destination**
    -   Drop on floor.
    -   take to best stockpile.
    -   specific storage destination where supported.

## 14.3 Crafting

-   [x] **Stone cutting** (SIM-029: stonecutter table, one chunk of any of five stones cuts into 10 blocks of it)
-   [x] **Butchering**
-   [x] **Cooking** (stove: 2 raw food -> 1 simple meal)
-   [x] **Tailoring/apparel production** (tailoring bench sews leather parkas, tuques and dusters; butchering yields leather)
-   [-] **Smithing/weapon production** (workbench: 4 wood -> wooden club, 3 stone -> knife, 5 wood -> spear)
-   [-] **Machining/advanced fabrication equivalents** (NG-1: the Fabrication research unlocks one recipe, components, plus tool cabinet and hospital bed)
-   [ ] **Drug production**
-   [x] **Component production in late tech** (NG-1: Fabricate component, 12 steel into 1 at the workbench with Crafting 6; components also come from buried machinery ore)

## 14.4 Quality

-   [ ] **Craft quality generation**
    -   Skill/inspiration influence.
    -   Quality affects market value and relevant item stats.

## 14.5 Art

-   [ ] **Artistic work**
    -   Sculptures/art objects.
    -   quality.
    -   beauty.
    -   creator attribution.
-   [ ] **Art descriptions**
    -   Generate original procedural descriptions from structured
        events/grammar.
    -   Do not copy RimWorld text.

------------------------------------------------------------------------

# PHASE 15 --- Power and Electrical Infrastructure

## 15.1 Power network

-   [ ] **Power producers**
    -   Fuel-based generator.
    -   solar.
    -   wind.
    -   geothermal-like late generator.
-   [ ] **Power consumers**
    -   Workstations.
    -   lights.
    -   coolers/heaters.
    -   communications.
    -   defensive/utility equipment where applicable.
-   [ ] **Power conduits/network**
    -   Connected graph.
    -   Topology recalculated only when changed.
-   [ ] **Power availability**
    -   Producers feed network.
    -   consumers activate only with sufficient supply.

## 15.2 Batteries

-   [ ] **Energy storage**
    -   Charge/discharge.
    -   capacity.
-   [ ] **Battery discharge event**
    -   Short-circuit/Zzztt-like event can release stored energy and
        start fire.
    -   Implement independently without copying exact balance.

## 15.3 Variable generation

-   [ ] **Solar day/weather output**
-   [ ] **Wind turbine output**
    -   Wind state and obstruction rules.
-   [ ] **Fuel consumption**
    -   Refuel jobs for generators/devices.

------------------------------------------------------------------------

# PHASE 16 --- Apparel, Equipment, Weapons, and Loadout Policies

## 16.1 Equipment

-   [x] **Single equipped primary weapon**
    -   Equip/drop.
    -   weapon stats.
    -   melee fallback for ranged weapons.
-   [x] **Equipment jobs**
    -   Player order to equip.
    -   dropped weapon can be hauled/stored.

## 16.2 Apparel

-   [-] **Wearable apparel** (body and head slots over everyday clothes; insulation shifts the comfort band; Wear order; Gear tab)
    -   Body-part/layer coverage.
    -   material.
    -   armor.
    -   insulation.
    -   quality.
    -   hit points.
-   [ ] **Apparel compatibility**
    -   Conflicting body layers/slots cannot be worn together.
-   [ ] **Apparel deterioration**
    -   Worn apparel can lose condition.
-   [-] **Apparel policy** (colonists put on free garments that suit the season and take off ones that don't; no player policy screen)
    -   Allowed categories.
    -   minimum condition.
    -   quality filters.
    -   pawn automatically changes apparel when safe/appropriate.
-   [ ] **Tainted apparel**
    -   Apparel from a corpse can carry negative social/mood semantics
        for ordinary colonists.

## 16.3 Weapon categories

-   [x] **Melee weapons**
    -   Multiple attack verbs/damage profiles.
-   [x] **Ranged weapons**
    -   Range.
    -   warmup.
    -   cooldown.
    -   accuracy by distance.
    -   burst count where applicable.
    -   projectile definition.
-   [ ] **Thrown/explosive weapons**
    -   Grenade-like projectile.
    -   blast radius.
-   [ ] **Single-use launchers**
    -   Consumable weapon/equipment class if desired for vanilla parity.
-   [ ] **No ammunition for ordinary guns**
    -   Base RimWorld's standard ranged weapons do not require ammo;
        preserve this experience unless a plugin changes it.
        citeturn2search1

------------------------------------------------------------------------

# PHASE 17 --- Drafting, Combat, Cover, Armor, Projectiles, and Explosions

## 17.1 Drafting

-   [x] **Draft/undraft**
    -   Drafted colonists stop autonomous work and accept tactical
        orders.
-   [x] **Drafted movement order**
-   [x] **Attack target order**
-   [ ] **Hold/fire-at-will behavior**
-   [ ] **Drafted field tending**
    -   Allow emergency tending under appropriate conditions.

## 17.2 Line of sight

-   [x] **Projectile LOS**
    -   Walls/solid obstacles block.
    -   Adjacent wall leaning/corner firing equivalent where needed for
        familiar tactical behavior.

## 17.3 Cover

-   [x] **Cover objects** (sandbags 55%, wall/door corners 40%)
    -   Walls, barricade/sandbag equivalents, chunks, trees, etc.
    -   Cover affects incoming ranged hit chance.
-   [-] **Cover calculation** (best cover on the cells beside the target facing the shooter; ranged shots only)
    -   Relative shooter-target-cover geometry.
    -   Stable deterministic resolution.

## 17.4 Ranged attack

-   [ ] **Target acquisition**
-   [ ] **Warmup**
-   [x] **Accuracy**
    -   Shooting skill.
    -   weapon.
    -   distance.
    -   target size.
    -   weather/light if modeled.
    -   cover.
-   [-] **Projectile flight**
    -   Logical projectile entity or deterministic trajectory.
    -   Can miss intended target and impact another valid cell/entity
        where design supports it.
-   [ ] **Burst fire**
-   [ ] **Cooldown**

## 17.5 Melee

-   [x] **Adjacent melee combat**
    -   Pawns next to hostile target use melee even if holding ranged
        weapon, matching vanilla's core behavior. citeturn2search1
-   [x] **Melee hit chance**
-   [-] **Weapon/body attacks**
-   [x] **Melee cooldown**

## 17.6 Armor

-   [ ] **Armor coverage**
    -   Apparel covers body parts.
-   [ ] **Armor categories**
    -   Sharp/blunt/heat-style protection.
-   [ ] **Armor resolution**
    -   Attack penetration vs armor.
    -   Can reduce/deflect damage.
    -   Damage ultimately applies to selected anatomy.

## 17.7 Explosions

-   [ ] **Blast cells**
-   [ ] **Radial damage**
-   [ ] **Fire chance**
-   [ ] **Building/item/pawn damage**
-   [ ] **Chain reaction support for explosive content**

## 17.8 Combat AI

-   [ ] **Hostile target selection**
-   [x] **Approach/ranged engagement**
-   [x] **Melee closing**
-   [ ] **Basic cover seeking**
-   [ ] **Flee behavior for appropriate human enemies**

------------------------------------------------------------------------

# PHASE 18 --- Animals, Wildlife, Hunting, Taming, Training, and Pens

Base-game animal simulation should include wild populations, hunting,
taming, training, animal products, predators, manhunter behavior, and
domestic management. Where modern base-game RimWorld uses pens for farm
animals, support the equivalent without pulling in DLC-only genetics or
children systems.

## 18.1 Species definitions

-   [x] **Animal species** (SIM-019: 61 species from size/behaviour classes; ten originals replace RimWorld-invented creatures)
    -   Anatomy.
    -   body size.
    -   movement.
    -   diet.
    -   wildness.
    -   trainability.
    -   combat stats.
    -   leather/meat yield.
    -   temperature range.
    -   reproduction/product properties if enabled.

## 18.2 Wildlife

-   [x] **Biome wildlife population** (SIM-023/025: per-biome wildlife density and spawn weights; start population sized to the biome's carrying capacity; arrivals below 70%, herds leave above 130%, predator cap)
    -   Spawn species appropriate to biome/climate.
-   [x] **Animal hunger/rest** (SIM-024: every animal has Food and Rest; states Wander/Feed/Rest/Hunt/Flee/Leave with hysteresis; herds bed down together, night and day sleepers, starvation)
    -   Simpler needs than humans.
-   [x] **Grazing/foraging** (SIM-024: grazing grass, browsing, crop raiding, carcass eating, timbergnaw tree felling; feeding troughs in pens; SIM-038: grazers prefer fuller grass, meals scale with growth)
-   [x] **Predation** (SIM-019: predators hunt prey no bigger than themselves; small-prey predators XS/S only; packs share targets)
    -   Predators select prey and hunt.
-   [x] **Wild animal flee/fight response** (flee within 5 tiles and run until clear of 9; enraged animals don't flee; PLAY-010 SIM-035: every animal hit by an attacker fights back or flees, one that can't fight back flees on every hit (penned ones inside the pen), predators may charge colonists within 3 tiles unless asleep (off on Wanderer/Gentle); SIM-036: unarmed undrafted colonists flee from an animal coming for them within 12 tiles, even mid-order or asleep; armed ones keep their target)

## 18.3 Hunting

-   [x] **Hunt designation**
-   [x] **Hunter requires suitable ranged weapon**
-   [ ] **Stalk/approach/fire job**
-   [-] **Animal retaliation/manhunter chance** (SIM-019: revenge chance by behaviour class, legendary 100% and rallies its kind; SIM-024: badly hurt fighters break off, predators give up chases; SIM-035: tame fighters defend themselves against anything but their own colonists)
-   [x] **Corpse hauling/butchering integration**

## 18.4 Taming

-   [x] **Tame designation** (Orders > Tame or the animal inspector; hunters tame)
-   [-] **Animal-handling interaction** (SIM-026: taming moved from Hunt to the Handle work type, also used for Milk/Shear; no separate trust/nuzzle interaction)
-   [ ] **Food requirement**
-   [ ] **Success chance based on skill/wildness**
-   [x] **Tame animal joins colony** (named; chance per try by species (deer 60%, timber wolf 25%); a botched try may enrage it; tame young are named at birth)

## 18.5 Training

-   [ ] **Trainability**
-   [ ] **Obedience**
-   [-] **Release/attack training** (release; tame wolves guard against raiders and hostile animals untrained)
-   [x] **Rescue**
-   [ ] **Hauling**
    -   Only species capable of relevant training.
-   [ ] **Training decay/maintenance**
    -   Skills can require reinforcement if matching target behavior.

## 18.6 Animal zones/pens

-   [ ] **Animal allowed areas for zonable animals**
-   [-] **Pen system for appropriate livestock** (pen zones of any shape, merged when adjacent, drawn with one fence; tame animals keep inside or stay near the colony; SIM-024: feeding troughs with plant/meat/any filter filled by haulers, animals graze in the pen; UI-014 pen-full alert; SIM-028: buildable wood/stone fences and gates (animals can't cross fences, tame animals pass gates, colonists climb fences at x3 cost; pathing doesn't prefer gates, no stone gate art), no rope/lead, no enclosure validity or capacity readout)
    -   Pen markers/boundaries.
    -   rope/lead jobs.
    -   enclosure validity.
    -   nutrition/grazing information.

## 18.7 Animal products

-   [x] **Milk production** (SIM-026: milk from dairy species with a product bar; Handle-work Milk job)
-   [x] **Wool production** (SIM-026: Shear job; wool parka and wool carpet recipes)
-   [x] **Egg laying** (SIM-026: chickens, ducks, geese lay plain eggs; fertile eggs hatch at 0 to 45 C, frozen ones die; ember oil campfire fuel)
    -   Data-driven species components.

## 18.8 Reproduction

-   [x] **Mating/pregnancy** (SIM-026: daily 35% mating check, pregnancy and litters, ageing, juveniles follow the mother, old-age death)
-   [x] **Birth/hatching** (SIM-026: litters, tame young named, fertile eggs hatch)
-   [-] **Population control policies if included in target vanilla
    version** (SIM-025: per-biome carrying capacity with arrivals/departures and predator cap; no player policy; slaughter/hunt only)

## 18.9 Manhunter/scaria-like state

-   [x] **Manhunter** (wild animals hurt by colonists may enrage for a day; chance by behaviour class (deer 0%, timber wolf 45%))
    -   Hostile animal mental state.
    -   Can result from damage/event.
-   [x] **Manhunter pack incident** (storyteller: manhunter herbivore packs outside winter, wolf packs in winter; SIM-025: single mad animal also in the threat pool)
    -   Storyteller-generated group attack.

------------------------------------------------------------------------

# PHASE 19 --- Prisoners, Capture, Recruitment, and Colony Population Growth

## 19.1 Capture

-   [ ] **Capture downed hostile**
    -   Carry to prisoner-designated bed/room.
-   [ ] **Prisoner state**
    -   Faction remains distinct until recruited.
    -   Restricted movement/escape behavior.

## 19.2 Prisoner needs

-   [ ] **Food/rest/health/mood**
    -   Prisoners remain simulated pawns with reduced player control.
-   [ ] **Prisoner feeding**
-   [ ] **Prisoner medical care**

## 19.3 Warden interactions

-   [ ] **Prisoner interaction mode**
    -   No interaction.
    -   reduce resistance.
    -   recruit.
    -   release.
    -   execute/medical operations if allowed by game rules.
-   [ ] **Resistance/recruitment difficulty**
    -   Warden social skill and prisoner mood influence progress.
-   [ ] **Recruitment**
    -   Successful recruit joins player's faction.

## 19.4 Prison breaks

-   [ ] **Escape attempt**
    -   Prisoner(s) can attempt to break out based on conditions.

## 19.5 Other recruitment

-   [ ] **Rescue/recovery join chance**
-   [ ] **Wanderer/migrant joins event**
-   [ ] **Refugee/visitor join opportunity where part of base event
    set**
-   [ ] **Quest-related joins when quest system is implemented**

------------------------------------------------------------------------

# PHASE 20 --- Factions, Goodwill, Visitors, Traders, and Trade

Factions drive NPC interaction. Depending on faction type and
relationship, members can visit, trade, assist, or raid; goodwill can
change relationships, and settlements exist on the world map.
citeturn2search0turn2search9

## 20.1 Faction definitions

-   [-] **Faction types** (NG-1 SIM-030: settlers, clans, drifters, freebooters, swarm and the player; no mechanoid or ancient factions)
    -   Player colony.
    -   tribal-level human factions.
    -   industrial/outlander-like factions.
    -   pirate/permanently hostile faction.
    -   mechanoid faction.
    -   insect faction.
    -   ancients/neutral special faction as required.
-   [-] **Faction tech level** (NG-1: neolithic or industrial; decides whether raiders carry rifles and the faction's name style; no effect on settlements or traders)
    -   Influences equipment, settlements, trader inventory, raid
        methods.
-   [x] **Faction naming** (NG-1: original procedural faction, leader and settlement names, unique per world)
    -   Original procedural faction/settlement names.

## 20.2 Goodwill

-   [x] **Relationship score** (NG-1: goodwill -100..100 with hysteresis: hostile at -50 or less, calm again at -10; friendly at +50, neutral again below +30)
    -   Hostile / neutral / ally states derived from goodwill/rules.
-   [-] **Goodwill changes** (NG-1: a repelled raid costs the raiding faction 5 goodwill and goodwill drifts toward a baseline daily; no gifts, quests, visitors or aid)
    -   Gifts.
    -   attacks.
    -   harming visitors.
    -   quest outcomes.
    -   assistance.
    -   settlement proximity if included.
-   [x] **Permanent hostility** (NG-1: freebooters and the swarm are fixed at -100)
    -   Some factions cannot become friendly.

## 20.3 Visitors

-   [ ] **Friendly visitor group**
    -   Enters map.
    -   wanders/stays.
    -   leaves.
    -   reacts to danger.

## 20.4 Trade caravans

-   [ ] **Trader arrival**
    -   Group with trader pawn and pack animals/inventory.
-   [ ] **Trade interaction**
    -   Select social-capable colonist.
    -   Open trade screen.
-   [ ] **Buy/sell**
    -   Colony stored eligible items available.
    -   Trader inventory and silver/currency.
    -   Purchased goods delivered/dropped.
-   [ ] **Trade price**
    -   Social skill and faction/trader factors influence price.
-   [ ] **Trader types**
    -   Different inventories such as bulk goods, combat/exotic, slave
        trader if included for historical vanilla parity, etc.
    -   Use original names/content.

## 20.5 Orbital trade

-   [ ] **Communications console**
-   [ ] **Orbital trade beacon**
    -   Defines tradeable nearby storage.
-   [ ] **Orbital trader event**
-   [ ] **Remote faction communication**
    -   Request caravans/help where relationship allows.

------------------------------------------------------------------------

# PHASE 21 --- Research and Technology Progression

Research is the main technology progression mechanism: colonists work at
research benches to accumulate research toward projects, most projects
have prerequisites, and technologies unlock buildings/gear.
citeturn0search12

## 21.1 Research definitions

-   [x] **Research project** (5 projects: food preservation, herbalism, tailoring, traps, gunsmithing)
    -   Cost.
    -   prerequisites.
    -   tech level.
    -   required bench/facilities.
    -   unlocked definitions.
-   [x] **Research DAG** (prerequisites; gunsmithing after traps)
    -   Validate no invalid cycles.
    -   UI layout can derive from prerequisite graph.

## 21.2 Research work

-   [x] **Simple research bench**
-   [ ] **Advanced research bench**
-   [x] **Research job** (Research work type; two-hour sittings at the nearest free bench)
    -   Intellectual skill.
    -   bench speed.
    -   room/environment modifiers as appropriate.
-   [x] **Research progress** (one point per working tick; finish message; saved)
    -   Player selects active project.
    -   Researchers contribute points.
    -   completion unlocks content.

## 21.3 Tech progression categories

Implement original content equivalents covering the vanilla functional
progression:

-   [ ] **Electricity**
-   [ ] **Battery/storage**
-   [ ] **Cooling/temperature control**
-   [ ] **Stoneworking**
-   [ ] **Complex furniture**
-   [ ] **Drug production**
-   [ ] **Microelectronics/communications**
-   [ ] **Advanced fabrication**
-   [ ] **Weapon technologies**
-   [ ] **Armor/apparel technologies**
-   [ ] **Geothermal/advanced power**
-   [ ] **Hospital/medical technology**
-   [ ] **Transport pods**
-   [ ] **Ship/endgame technologies**

------------------------------------------------------------------------

# PHASE 22 --- World Generation, Biomes, Local Map Generation, and Settlements

RimWorld generates a seeded planet with world tiles, biomes, rainfall,
temperature, elevation, latitude, terrain type, growing period, and
faction settlements; players choose a landing tile before generating the
local colony map. citeturn1search4

## 22.1 Planet

-   [x] **Seeded world generation** (NG-1 SIM-030: free-text seed; the same seed and settings give the same planet on every machine)
-   [x] **World tile graph**
    -   Hex-like adjacency or an independently implemented equivalent
        supporting travel.
-   [x] **Latitude**
-   [x] **Elevation**
-   [x] **Rainfall** (latitude bands, distance from the sea, rain shadow; arid shrubland is about 40 % of land, a tuning follow-up)
-   [x] **Temperature**
-   [x] **Seasonality** (hemisphere-aware peak day, amplitude by latitude and continentality)
-   [-] **Movement difficulty** (shown on the tile card; informational until world travel exists)
-   [x] **Roads** (settlements joined by roads; they become dirt tracks on the local map)
-   [x] **Rivers** (flow downhill to the sea, lakes in basins; become rivers on the local map)
    -   If part of chosen base-game target.

## 22.2 Biomes

Implement functional equivalents for vanilla biome classes, with
original assets/content:

-   [x] **Temperate forest**
-   [x] **Boreal forest**
-   [x] **Tropical rainforest**
-   [x] **Arid shrubland**
-   [x] **Desert**
-   [x] **Extreme desert**
-   [x] **Tundra**
-   [x] **Ice sheet**
-   [x] **Sea ice**
-   [x] **Temperate swamp**
-   [x] **Tropical swamp**
-   [x] **Cold bog**

Each biome defines: - terrain mix; - plant density/species; -
wildlife; - rainfall; - temperature profile; - disease frequency; -
forage; - local map appearance.

SIM-023: all twelve exist as `world/biome` data. NG-1 now classifies them on the generated globe from temperature, rain, hills and elevation, and the player picks a world tile. Each defines climate, wetness, ground mix, wild plants, grass density/regrowth and wildlife density; disease frequency is not modelled.

## 22.3 Terrain topology

-   [x] **Flat** (NG-1: per-tile hilliness drives the local map's rock thresholds)
-   [x] **Small hills**
-   [x] **Large hills**
-   [x] **Mountainous** (with a ridge band and caves)

## 22.4 Local map generation

-   [x] **Terrain from biome/topology** (SIM-023 ground mix per biome; NG-1 hills level changes rock, stone and ridge thresholds)
-   [x] **Rock/mountain mass** (noise outcrops biased to map edges, stone ground ring, open start area; 2-3 stone types per map in regions, SIM-029; overhead mountain roof tracked under roofs)
-   [x] **Ore/resource deposits** (SIM-029: steel, silver and gold veins grown inside rock; mined for metal)
-   [x] **Water** (NG-1 SIM-033: coast, rivers with deep cores, mud banks and fords, lakes, wet ground, tracks, a reachability pass; no bridges or fishing)
-   [x] **Wild vegetation** (trees, berry bushes and seven new ground plants (SIM-038); grass is simulated ground cover removed by building, sowing and Cut plants (UI-011) and regrows from shoots over three days after grazing or cutting)
-   [x] **Wildlife** (SIM-023/025: per-biome density, species weights, sex and age at spawn, carrying capacity)
-   [ ] **Ancient ruins/debris**
-   [x] **Starting clear/landing area** (NG-1: open start radius, no water within 7 tiles of the centre, drop pods land on dry ground)

## 22.5 Faction settlements

-   [x] **Settlement placement** (NG-1: Poisson-disc over land, capitals first, density by type and population setting)
-   [-] **Faction territory distribution** (preferred biomes and clustering near the capital; no territory ownership)
-   [ ] **Settlement trade inventory**
-   [ ] **Hostile settlement attack map**
-   [ ] **Settlement destruction**
    -   Destroying all settlements can eliminate a faction's world
        presence.

## 22.6 Landing-site UI

-   [x] **Inspect world tile** (NG-1: the landing screen's tile panel)
    -   Biome.
    -   terrain.
    -   temperature.
    -   rainfall.
    -   growing period.
    -   movement difficulty.
    -   nearby factions.
-   [x] **Choose starting tile**
-   [x] **Generate colony map from world tile**

------------------------------------------------------------------------

# PHASE 23 --- Scenarios, Starting Pawns, Difficulty, and Storytellers

Base RimWorld uses starting scenarios and three major storyteller
personalities: Cassandra Classic, Phoebe Chillax, and Randy Random.
Storytellers control event pacing rather than directly controlling
colonists. citeturn1search8turn1search9

## 23.1 Scenario system

-   [x] **Scenario definition** (NG-1: people, age and trait constraints, research, items, animals, conditions, scheduled and disabled incidents, forbidden buildings, intro)
    -   Starting pawn count.
    -   pawn constraints.
    -   starting research.
    -   starting items.
    -   starting animals.
    -   initial conditions.
    -   recurring/permanent scenario effects.
-   [x] **Crashlanded-like standard start** (NG-1: Broken Hull, three colonists in drop pods)
    -   Three colonists.
    -   practical starting supplies/weapons.
    -   basic technology.
-   [x] **Single-survivor difficult start** (Bare Hands)
-   [x] **Tribal/low-tech group start** (Ember Clan, five colonists standing, neolithic)
-   [x] **Rich solo high-tech start** (Gilded Expedition)
    -   Use original scenario names and prose.
-   [x] **Scenario editor** (NG-1: 14 part kinds, validation, save and load of JSON files)
    -   Optional but important vanilla feature.
    -   Add/remove scenario parts and conditions.
    -   Save custom scenarios.

## 23.2 Pawn selection

-   [x] **Starting pawn roster generation**
-   [x] **Reroll/randomize candidates**
-   [-] **Display skills/traits/incapabilities/health/relationships** (all but relationships)

## 23.3 Difficulty

-   [-] **Difficulty profile** (NG-1 SIM-031: six presets covering threat scale, yields, mood, adaptation, infection, revenge, weather; the disease frequency has no effect because there are no disease events; no death/downing options)
    -   Threat scaling.
    -   economy/yields.
    -   disease.
    -   mood.
    -   adaptation.
    -   death/downing behavior as appropriate.
-   [x] **Custom difficulty** (NG-1: every setting editable, copy from a preset, compact codec)
    -   Expose major multipliers/toggles.

## 23.4 Storyteller framework

-   [x] **Storyteller component** (NG-1: per-storyteller `Plan` from the colony state, loss easing, peaceful days, adaptation; the legacy profile keeps first threat day 5 then every 5 to 9 days)
    -   Reads colony state.
    -   schedules incidents.
    -   tracks cooldowns/threat pacing.
    -   deterministic from storyteller RNG stream.
-   [x] **Paced storyteller** (Orla Vane: 8 days of pressure, 5 of rest, points ramp per cycle)
    -   Regular pressure and recovery cadence.
-   [x] **Relaxed storyteller** (Bram Holloway: long gaps, eases after a loss)
    -   Longer building/recovery windows.
-   [x] **Random storyteller** (Kestrel Quay: daily dice with rising odds, occasional double threats; positive-event bias is defined but not applied yet)
    -   Less predictable event timing/type.

Use original names rather than copying RimWorld storyteller characters.

------------------------------------------------------------------------

# PHASE 24 --- Wealth, Threat Scaling, Raids, Sieges, Infestations, and Major Threats

## 24.1 Colony wealth

-   [ ] **Wealth calculation**
    -   Items.
    -   buildings.
    -   pawns/animals where appropriate.
    -   Expose wealth history.
-   [x] **Threat points** (NG-1: colonists, a wealth curve, threat scale, adaptation and optional time escalation, for colonies from the wizard)
    -   Storyteller converts colony state, population, wealth, time,
        adaptation/difficulty into threat budget.
    -   Exact RimWorld formula is not required and should not be copied.

## 24.2 Raid generation

-   [x] **Select hostile faction** (NG-1: a hostile human faction weighted by settlement count; none means no raid)
-   [x] **Generate raider group from point budget** (wealth/40 + 30 per standing colonist + 2 per day; clubs 60, rifles 90; up to 8)
-   [-] **Generate equipment based on faction/tech** (NG-1: neolithic factions field clubs only, industrial ones rifles)
-   [ ] **Map-edge arrival**
-   [ ] **Immediate attack**
-   [ ] **Prepare-before-attack behavior**

## 24.3 Raid tactics

-   [ ] **Basic assault**
-   [x] **Sappers/breachers equivalent** (against an enclosed colony from day 10: tunnel through walls, doors and rock, step around traps)
    -   Attack walls/alternate route instead of blindly using normal
        entrance.
-   [ ] **Drop-pod assault**
    -   Industrial factions can arrive from above.
-   [ ] **Center/scatter drop variants**
-   [x] **Siege** (from day 12: camp 18 tiles out, sandbags, shoot from cover, storm after 5h)
    -   Raiders establish temporary siege position, receive/build
        artillery/supplies, bombard colony, then assault.

## 24.4 Raider morale

-   [x] **Human raider flee threshold** (badly hurt raiders, or a group that has lost half its members, flee to the edge)
    -   Group may flee after sufficient casualties/damage.
-   [ ] **Kidnap behavior**
    -   Raiders can carry downed colonists away under suitable
        circumstances.
-   [ ] **Steal behavior**
    -   Raiders may steal valuable reachable items when
        retreating/meeting goals.

## 24.5 Mechanoid threats

-   [ ] **Mechanoid pawn types**
    -   Machine enemies with different weapon/range roles.
    -   No human mood/needs.
-   [ ] **Mechanoid raid**
    -   Persistent hostile behavior; does not use ordinary human morale.
-   [ ] **Crashed ship-part threat**
    -   Psychic/defoliator-like hostile world object.
    -   Dormant/guarding mechanoids.
    -   Persistent map effect until destroyed.
    -   Use original content/names.

## 24.6 Insects

-   [x] **Insect species** (SIM-019: hive beetle, hive brute, hive crawler; original designs)
-   [ ] **Hive**
    -   Spawns/maintains insect population.
-   [-] **Infestation incident** (SIM-019: hive swarm bursts from the rock nearest the colony from day 12; no hives or dark/warm rules)
    -   Can emerge in valid under-mountain/dark/warm areas according to
        independently implemented rules.
-   [ ] **Hive reproduction**
-   [x] **Insect hostile AI** (SIM-019: hostile from spawn, never flee, never tameable)

## 24.7 Manhunter threat

-   [x] **Manhunter pack incident**
    -   Storyteller selects suitable animal species and group size.
    -   Animals enter map hostile to humans.

------------------------------------------------------------------------

# PHASE 25 --- Incident and Story Event Catalog

The storyteller creates both threats and opportunities. Base-game event
classes include raids, infestations, manhunter packs, crashed ship
parts, weather/environmental events, traders, visitors, resource events,
and other colony incidents. citeturn2search2turn2search7

## 25.1 Incident framework

-   [-] **Incident definition** (SIM-025: `internal/domain/incident` defs with weight, cooldown, biome limits and nature-event cadence of 2 to 5 days; no category/point budget, raids and threats still use the storyteller schedule)
    -   Category.
    -   eligibility.
    -   weight.
    -   cooldown.
    -   target map/world object.
    -   point budget.
    -   execute function.
-   [ ] **Incident queue**
    -   Scheduled delayed incidents.
    -   Save/load safe.

## 25.2 Positive/neutral incidents

-   [ ] **Trader caravan**
-   [ ] **Orbital trader**
-   [ ] **Friendly visitors**
-   [ ] **Wanderer joins**
-   [ ] **Transport pod crash / downed stranger**
-   [ ] **Resource meteorite**
-   [ ] **Cargo pod drop**
-   [x] **Animal joins** (SIM-025: stray joins)
-   [x] **Farm animals wander in** (SIM-025)
-   [x] **Rare self-tame animal** (SIM-025: taken a liking)
-   [x] **Wild animal migration**
-   [ ] **Party**
-   [ ] **Marriage**
-   [ ] **Psychic soothe-like temporary positive mood event**
    -   Original presentation/content.

## 25.3 Negative incidents

-   [x] **Raid** (edge arrival, assault nearest colonist, flee, bleed out, dropped weapons; saved)
-   [ ] **Infestation**
-   [x] **Manhunter pack**
-   [x] **Animal insanity** (SIM-025: mad animal, a single manhunter)
-   [ ] **Disease outbreak**
-   [ ] **Blight**
    -   Crop disease spreads/destroys plants.
-   [ ] **Toxic fallout**
-   [-] **Heat wave** (NG-1: see 12.6; fired by the day rollover, not the storyteller schedule)
-   [-] **Cold snap** (NG-1: see 12.6)
-   [ ] **Eclipse**
-   [ ] **Solar flare**
-   [ ] **Flashstorm**
-   [ ] **Dry thunderstorm**
-   [ ] **Psychic drone-like mood event**
-   [ ] **Volcanic-winter-like long climate event if part of target base
    version**
-   [ ] **Crashed hostile ship part**
-   [ ] **Short circuit/battery discharge**
-   [x] **Animal predator hunts colonist**
    -   May be AI-driven rather than storyteller incident.

## 25.4 Letters/messages

-   [ ] **Event letter system**
    -   Positive/neutral/negative/threat categories.
    -   Camera jump target.
    -   relevant entities.
    -   optional choices.
-   [ ] **Message system**
    -   Short transient notices.
    -   Rate-limit repeated low-value messages.
-   [ ] **Alert system**
    -   Persistent colony problems:
        -   low food;
        -   colonist needs rescue;
        -   medical emergency;
        -   break risk;
        -   no research;
        -   idle colonists;
        -   unburied colonist;
        -   dangerous temperature;
        -   missing beds;
        -   animal starvation;
        -   major threat.

------------------------------------------------------------------------

# PHASE 26 --- Drugs, Alcohol, Addiction, Tolerance, and Policies

## 26.1 Drug definitions

-   [ ] **Consumable drug**
    -   immediate effects.
    -   health effects.
    -   mood effects.
    -   consciousness/movement/pain modifiers.
    -   tolerance.
    -   addiction chance.
    -   overdose/toxicity where appropriate.
-   [ ] **Representative functional categories**
    -   beer/alcohol.
    -   smokeleaf-like relaxant.
    -   psychite tea-like mild stimulant.
    -   wake-up-like stimulant.
    -   go-juice-like combat stimulant.
    -   hard euphoric drug.
    -   Use original names/assets.

## 26.2 Tolerance

-   [ ] **Drug tolerance**
    -   Repeated use builds tolerance.
    -   Tolerance reduces effect and/or increases health risk depending
        on definition.
    -   Decays with abstinence.

## 26.3 Addiction

-   [ ] **Addiction chance**
-   [ ] **Addiction need**
    -   Becomes a pawn need.
-   [ ] **Withdrawal**
    -   Health/mood/stat effects.
    -   Resolves after sufficient abstinence for recoverable addictions.

## 26.4 Drug policy

-   [ ] **Policy definitions**
    -   Allowed drugs.
    -   recreational use.
    -   scheduled use.
    -   mood threshold.
    -   recreation threshold.
    -   carry quantity.
-   [ ] **Autonomous drug use**
    -   Pawn follows policy unless mental state/trait overrides.

## 26.5 Chemical traits

-   [ ] **Chemical-interest/fascination equivalent**
    -   Alters drug desire/autonomous behavior and chemical need.

------------------------------------------------------------------------

# PHASE 27 --- Clothing Assignment, Food Restrictions, Medical Policies, and Colony Management UI

## 27.1 Assign tab

-   [ ] **Food restriction policy**
    -   Allowed food categories/items.
    -   Pawn food search respects policy.
-   [ ] **Apparel policy**
    -   Already implemented functionally; expose colony-wide policy
        editor.
-   [ ] **Drug policy**
    -   Already implemented functionally; expose editor.
-   [ ] **Medical care policy**
    -   Per pawn maximum medicine/care level.

## 27.2 Animal management

-   [x] **Animals tab** (tame animals, slaughter, release)
    -   Master assignment if supported.
    -   training.
    -   allowed area/pen.
    -   slaughter.
    -   medical settings.

## 27.3 Wildlife tab

-   [-] **Wildlife list** (UI-014: Animals tab Colony/Wild toggle with species, count, diet, predator, tame %, meat and Hunt all/Tame one/Cancel; no sort/filter)
    -   species.
    -   count.
    -   predator status.
    -   hunt designation.
    -   tameness-related information.

## 27.4 Work tab

-   [ ] **Scalable work-priority matrix**
    -   Sorting/filtering.
    -   manual priority toggle.
    -   skill/passions visible.

## 27.5 Schedule tab

-   [ ] **Schedule matrix**
    -   Paint schedule blocks across multiple pawns.

## 27.6 History/statistics

-   [ ] **Colony wealth graph**
-   [ ] **Population history**
-   [ ] **Major event history**
-   [ ] **Pawn records**
    -   kills, work, damage, time, etc. where meaningful.

------------------------------------------------------------------------

# PHASE 28 --- Caravans and World Travel

Caravans are groups of colonists/animals/prisoners that leave a local
map and travel across the world for trade, quests, attacks, or
relocation. citeturn2search12

## 28.1 Caravan formation

-   [ ] **Form caravan dialog**
    -   Select colonists.
    -   animals.
    -   prisoners.
    -   items.
    -   food.
    -   destination.
-   [ ] **Mass/carry capacity**
    -   Pawns and pack animals carry limited mass.
-   [ ] **Travel supplies**
    -   Estimate food and travel time.
-   [ ] **Gather/loading phase**
    -   Pawns collect assigned cargo before departure.
-   [ ] **Exit map**
    -   Caravan becomes world object.

## 28.2 World movement

-   [ ] **World pathfinding**
    -   Route across world tiles.
    -   Terrain/roads affect movement.
-   [ ] **Travel speed**
    -   Pawn/animal capacities.
    -   mass.
    -   terrain.
    -   road.
    -   weather/season where appropriate.
-   [ ] **Foraging**
    -   Caravan can gain food from suitable tiles based on skills/biome.
-   [ ] **Rest**
    -   Caravan pauses/rests according to travel cycle.

## 28.3 Caravan needs

-   [ ] **Food consumption**
-   [ ] **Health/disease progression**
-   [ ] **Mood effects**
-   [x] **Animal feeding** (SIM-024: animals graze, eat carcasses and crops, and are fed from troughs)
-   [ ] **Prisoner handling**

Use aggregated/analytical updates rather than simulating local-map jobs.

## 28.4 Caravan interactions

-   [ ] **Visit friendly settlement**
    -   Trade.
    -   gifts.
-   [ ] **Attack hostile settlement**
-   [ ] **Enter event/quest map**
-   [ ] **Ambush encounter**
-   [ ] **Return to colony**
-   [ ] **Settle new colony**
    -   If multiple colonies are supported by target scope.

## 28.5 Transport pods

-   [ ] **Pod launcher**
-   [ ] **Fuel**
-   [ ] **Load pawns/items**
-   [ ] **World-range targeting**
-   [ ] **Launch**
-   [ ] **Arrival at map/settlement**
    -   Trade/attack/reinforce/gift depending target.

------------------------------------------------------------------------

# PHASE 29 --- Quests and Temporary World Sites

Base-game factions can generate quests, often asking the player to
travel, rescue, fight, deliver, or interact with a temporary site in
exchange for rewards. citeturn2search0

## 29.1 Quest framework

-   [ ] **Quest state**
    -   offered.
    -   accepted.
    -   active.
    -   succeeded.
    -   failed.
    -   expired.
-   [ ] **Quest choice/reward**
    -   One or more reward options where appropriate.
-   [ ] **Quest timers**
    -   Offer expiry.
    -   completion deadline.
    -   delayed follow-up.
-   [ ] **Quest-generated world object**
    -   Temporary site.
    -   map generated on arrival.
    -   cleanup after resolution.

## 29.2 Base quest archetypes

-   [ ] **Rescue downed/captured person**
-   [ ] **Prisoner rescue**
-   [ ] **Item stash**
-   [ ] **Bandit/outpost attack**
-   [ ] **Trade request**
-   [ ] **Deliver requested items**
-   [ ] **Hospitality/refugee-style temporary guests if base-version
    appropriate**
-   [ ] **Peace talks/diplomatic site**
-   [ ] **AI persona/core-style rare objective**
-   [ ] **Distress signal/ambush variant**

Use original narrative text and generated names.

## 29.3 Rewards

-   [ ] **Items/silver**
-   [ ] **Goodwill**
-   [ ] **Joiner/recruit**
-   [ ] **Rare technology/item reward**

------------------------------------------------------------------------

# PHASE 30 --- Advanced Enemy AI and Tactical Variety

-   [x] **Raid group lord/controller** (group plan assault / siege / sappers; siege camp, dig-in, storm; flee)

    -   Group objective controls broad behavior.
    -   Individual pawns use local jobs.

-   [ ] **Staging**

    -   Raiders wait at rally area before attack.

-   [ ] **Assault trigger**

    -   Time, damage, player proximity, or storyteller condition.

-   [ ] **Target priority**

    -   Reachable colonists.
    -   defensive structures.
    -   valuable colony objects.
    -   power/doors depending tactic.

-   [x] **Breach logic** (raiders shut out plan through walls and doors and bash the first one; buildings have hit points)

    -   Select wall/door targets when normal route unsuitable.

-   [-] **Siege construction** (sandbags in front of each besieger; no mortars)

-   [ ] **Siege bombardment**

-   [ ] **Siege assault transition**

-   [ ] **Retreat**

    -   Human raid morale.
    -   carry stolen goods/kidnapped pawn if feasible.

-   [ ] **Faction-on-faction hostility**

    -   NPC groups from hostile factions can fight each other when
        simultaneously present.

------------------------------------------------------------------------

# PHASE 31 --- Advanced Economy, Market Value, Traders, and Colony Wealth

-   [ ] **Market value**
    -   Every tradeable item/pawn/animal has value derived from
        definition, quality, condition, material, etc.
-   [ ] **Buy/sell price modifiers**
    -   Trader markup.
    -   social skill.
    -   faction relationship where desired.
-   [ ] **Silver/currency**
    -   Physical stackable currency.
    -   Used in trade.
-   [ ] **Trader inventory generation**
    -   Category.
    -   tech level.
    -   wealth.
    -   random stock.
    -   deterministic generation.
-   [ ] **Restocking settlements**
    -   Inventory refreshes after time.
-   [ ] **Gifts**
    -   Give goods to faction for goodwill.
-   [ ] **Wealth-driven expectations**
    -   Pawn expectation thought scales with colony development.
-   [ ] **Wealth-driven threats**
    -   Storyteller threat budget uses colony wealth plus other pacing
        variables.

------------------------------------------------------------------------

# PHASE 32 --- Endgame: Ship Construction, Reactor Defense, and Escape

Vanilla RimWorld's classic endgame is to reach/build a ship, activate
its reactor, survive a sustained period of intensified attacks, place
colonists into cryptosleep caskets, and launch/escape. Implement the
experience with original names/assets/text.

## 32.1 Ship technology

-   [ ] **Endgame research chain**
-   [ ] **Ship structural components**
-   [ ] **Ship computer/core**
-   [ ] **Reactor**
-   [ ] **Cryptosleep caskets**
-   [ ] **Engine components**

## 32.2 Ship validation

-   [ ] **Connected valid ship**
    -   Required components connected.
    -   enough caskets for intended passengers.

## 32.3 Reactor startup

-   [ ] **Start reactor**
    -   Explicit player action.
    -   Starts long activation countdown.
-   [ ] **Endgame threat escalation**
    -   Storyteller generates frequent/high-pressure raids during
        startup.
    -   Deterministic incident schedule from campaign state.

## 32.4 Launch

-   [ ] **Load colonists into caskets**
-   [ ] **Launch command**
-   [ ] **Victory state**
    -   Escaping pawns recorded.
    -   colony story may continue if pawns remain, depending chosen
        behavior.
-   [ ] **Credits/story summary**
    -   Original presentation and music/assets.

## 32.5 Alternative ship objective

-   [ ] **Distant pre-existing ship world quest**
    -   Optional vanilla-equivalent path: travel across world to a known
        ship and activate it.

------------------------------------------------------------------------

# PHASE 33 --- Save/Load, Autosave, Multiple Saves, and Campaign Continuity

Although foundational persistence exists earlier, this phase completes
the player-facing system.

-   [x] **Manual save** (pause > Save writes the colony's slot)
-   [x] **Load game** (pause > Load and main menu Load/Continue)
-   [-] **Save metadata** (slot name, in-game date, day, colonists, format version 2, plus commitment, storyteller, difficulty, map size and biome; no plugins/content hash)
    -   Colony.
    -   world seed.
    -   tick/date.
    -   version.
    -   enabled plugins.
    -   content hash.
-   [-] **Autosave** (every in-game morning at 06:00 to <slot>-autosave; cadence not configurable)
    -   Configurable cadence.
    -   Consistent snapshot without long simulation stall.
-   [ ] **Rotating autosaves**
-   [-] **Save naming/overwrite** (one slot per colony, named at start, overwritten on save; no name prompt)
-   [x] **Ironman/commitment-style save mode** (NG-1: chosen at setup and fixed; one save slot written by the game itself at each day, on return to menu, on quit and on window close; Load and developer mode disabled)
    -   Restricts ordinary save-scumming behavior if selected.
-   [-] **Schema migration** (NG-1: version 1 saves still load, with neutral legacy bios for old colonists; no migration tool or warning dialog)
-   [-] **Corruption validation** (unreadable or wrong-version files listed but not loadable)
-   [x] **Safe atomic writes** (temp file + fsync + rename)
-   [x] **Save/replay deterministic equality tests** (save mid-game, load, run both in lock step: identical)

------------------------------------------------------------------------

# PHASE 34 --- Full UI, Overlays, Alerts, Architect Menus, and Quality of Life

## 34.1 Architect/build menu

-   [ ] **Orders**
-   [ ] **Zone/area**
-   [ ] **Structure**
-   [ ] **Production**
-   [ ] **Furniture**
-   [ ] **Power**
-   [ ] **Security**
-   [ ] **Misc/ship**
    -   Categories may use original naming.

## 34.2 Overlays

-   [-] **Roof overlay** (SIM-018; no thin or mountain roof distinction)
-   [ ] **Power grid**
-   [ ] **Beauty**
-   [ ] **Room stats**
-   [x] **Fertility**
-   [ ] **Terrain/path cost debug**
-   [-] **Home area** (SIM-021: painted, auto-grown around buildings, drives cleaning; no firefighting or repair yet)
-   [ ] **Allowed areas**
-   [ ] **Storage zones**
-   [ ] **Growing zones**
-   [ ] **Temperature**

## 34.3 Pawn tabs

-   [ ] **Needs**
-   [ ] **Health**
-   [ ] **Gear**
-   [ ] **Social**
-   [ ] **Character/bio**
-   [ ] **Records**

## 34.4 Global tabs

-   [ ] **Architect**
-   [ ] **Work**
-   [ ] **Schedule**
-   [ ] **Assign**
-   [ ] **Animals**
-   [ ] **Wildlife**
-   [ ] **Research**
-   [ ] **World**
-   [ ] **History**
-   [ ] **Factions**
-   [ ] **Quests**

## 34.5 Notifications

-   [ ] **Top-priority alerts**
-   [ ] **Event letters**
-   [ ] **Messages**
-   [ ] **Jump-to-location**
-   [ ] **Pause-on-major-threat option**

## 34.6 Input

-   [x] **Keyboard shortcuts** (UI-016: action registry in `internal/platform/input`; camera, time, tabs, selection, orders, overlays, Architect, gizmo keys, screenshots, developer keys)
-   [ ] **Control groups**
-   [x] **Shift command queue** (order menu and drafted moves, formations included)
-   [x] **Double-click select same type** (UI-016)
-   [x] **Drag designation** (areas, build lines/rectangles/ovals with Ctrl to square, UI-016)
-   [x] **Cancel tool** (right-click or Esc leaves a tool; C picks the Cancel tool)
-   [x] **Rebindable controls where practical** (Options › Controls, saved; modifiers are fixed)

------------------------------------------------------------------------

# PHASE 35 --- Audio, Visual Feedback, Animation, and Game Feel

-   [ ] **Sound event system**
    -   Domain emits semantic events; audio adapter selects assets.
-   [ ] **Ambient biome audio**
-   [ ] **Weather audio**
-   [ ] **Construction/work sounds**
-   [ ] **Weapon sounds**
-   [ ] **Impact/explosion sounds**
-   [ ] **UI feedback sounds**
-   [ ] **Music system**
    -   Contextual but not authoritative.
-   [x] **Pawn facing/aiming** (smoothed facing with hysteresis; turns to the work tile, attack target and blockers; rifles held at ready, raised to aim and turned towards the target)
-   [x] **Simple movement interpolation**
-   [x] **Muzzle/projectile effects** (muzzle flash and smoke at the rifle muzzle, ejected casing, tracer from the muzzle, impact puff)
-   [-] **Damage/fire effects** (hit flash, flinch and stagger away from the attacker, directional blood sprays scaled by damage, bloodied bodies, floor stains, falls; burning-building fire not implemented)
-   [ ] **Selection/targeting feedback**
-   [ ] **Weather visual effects**
-   [x] **Day/night presentation** (ART-010: every sprite lit by a moving sun, a cool moon, indoor ambient under roofs and lamp light; moving shadows; the old fullscreen veil is retired)

All assets must be original or appropriately licensed.

------------------------------------------------------------------------

# PHASE 36 --- Tutorial, Learning Helper, Tooltips, and Discoverability

RimWorld includes a tutorial/learning-helper approach that introduces UI
and survival concepts progressively. citeturn0search7turn1search8

-   [ ] **Contextual learning helper**
    -   Detect first encounter with major mechanics.
    -   Show short instructional card.
    -   Track dismissed/learned concepts.
-   [ ] **Interactive tutorial scenario**
    -   Camera/selection.
    -   stockpile.
    -   building.
    -   growing.
    -   work priorities.
    -   combat/drafting.
    -   medical care.
-   [ ] **Rich tooltips**
    -   Explain stats and modifiers.
    -   Show calculation breakdown for important values.
-   [ ] **Inspect explanations**
    -   Why pawn cannot do job.
    -   why room is invalid.
    -   why path fails.
    -   why building has no power.
    -   why plant is not growing.
-   [ ] **Difficulty descriptions**
-   [ ] **Scenario descriptions**
-   [ ] **World-tile explanations**

------------------------------------------------------------------------

# PHASE 37 --- Plugin/Mod System Foundation

RimWorld's mod ecosystem is strongly data-driven: definitions describe
content such as weapons, apparel, plants, recipes, research and terrain;
mods can also include localization, textures, sounds, patches and custom
code. RimWorld itself does not expose a formal narrow mod API, and
community mods commonly use runtime patching such as Harmony. For
`rimworld-go`, we should preserve the *power and extensibility* while
providing a safer, intentional plugin API instead of reproducing runtime
method patching.
citeturn0search4turn0search5turn0search8turn0search10

## 37.1 Plugin manifest

-   [ ] **Plugin metadata**
    -   Unique namespaced ID.
    -   display name.
    -   author(s).
    -   version.
    -   game compatibility.
    -   description.
    -   optional homepage/source.
    -   dependencies.
    -   load-before/load-after hints.
    -   incompatibilities.

RimWorld's current mod metadata similarly identifies package ID, name,
authors, description, supported versions, dependencies, and load-order
hints. citeturn0search2

## 37.2 Plugin discovery

-   [ ] **Plugin directories**
    -   Discover installed plugins from configured directories.
-   [ ] **Enable/disable list**
    -   Player chooses active plugins.
-   [ ] **Deterministic load order**
    -   Resolve dependencies.
    -   topological ordering.
    -   stable tie-break.
    -   explicit errors for cycles.
-   [ ] **Compatibility validation**
    -   Missing dependency.
    -   incompatible version.
    -   duplicate plugin ID.
    -   incompatible plugin pair.

## 37.3 Data definitions

-   [ ] **Definition registry**
    -   Namespaced stable IDs.
    -   Immutable after game start.
-   [ ] **Definition classes**
    -   Item.
    -   material.
    -   terrain.
    -   building.
    -   pawn/species.
    -   plant.
    -   weapon.
    -   apparel.
    -   recipe.
    -   research.
    -   trait.
    -   backstory.
    -   thought.
    -   health effect.
    -   incident.
    -   faction.
    -   biome.
    -   trader.
    -   scenario.
    -   recreation.
    -   drug.
-   [ ] **Human-readable definition format**
    -   YAML or JSON.
    -   Strong schema validation.
    -   Convert strings to compact runtime indexes.
-   [ ] **Definition inheritance/templates**
    -   Allow reusable parent/template definitions without runtime
        reflection-heavy behavior.

## 37.4 Patching definitions

-   [ ] **Declarative patch operations**
    -   Add.
    -   remove.
    -   replace.
    -   merge.
    -   conditional patch based on another plugin.
-   [ ] **Patch ordering**
    -   Deterministic.
    -   diagnostics show which plugin changed a definition.
-   [ ] **Conflict diagnostics**
    -   Detect/describe conflicting patches where possible.

RimWorld mods use patch operations to alter definitions without
replacing entire definition files; `rimworld-go` should offer the same
class of capability with an intentionally designed schema.
citeturn0search5

## 37.5 Content validation

-   [ ] **`contentlint`**
    -   Parse all active plugin definitions.
    -   validate schema.
    -   validate references.
    -   validate dependency graph.
    -   validate ranges.
    -   validate assets.
    -   report provenance of every error.
-   [ ] **Config errors**
    -   Human-readable startup diagnostics.

## 37.6 Plugin assets

-   [ ] **Textures/sprites**
-   [ ] **Audio**
-   [ ] **Localization**
-   [ ] **Optional shader/render resources**
    -   Adapter-level only.
-   [ ] **Namespaced asset paths**
    -   Prevent collisions.
-   [ ] **Safe path handling**
    -   No traversal outside plugin root.

## 37.7 Localization

-   [ ] **Localization keys**
-   [ ] **Language packs**
-   [ ] **Definition label/description translation**
-   [ ] **Generated grammar/word lists**
-   [ ] **Fallback language**

------------------------------------------------------------------------

# PHASE 38 --- Plugin Behavior API and Sandboxed Scripting

RimWorld mods can use custom C# and Harmony to alter almost any
behavior. citeturn0search0turn0search3 `rimworld-go` should achieve
comparable extensibility through stable extension points rather than
allowing arbitrary mutation of internal Go functions.

## 38.1 Stable extension API

-   [ ] **Plugin API version**
-   [ ] **Capability declaration**
-   [ ] **Read-only world query API**
-   [ ] **Command submission API**
-   [ ] **Definition registration hooks**
-   [ ] **Event subscription**
-   [ ] **Custom validation hooks**
-   [ ] **Custom UI panels/actions where safe**
-   [ ] **Custom incident behavior**
-   [ ] **Custom job/intent providers**
-   [ ] **Custom stat modifiers**
-   [ ] **Custom health-effect behavior**
-   [ ] **Custom recipe/building components**
-   [ ] **Custom world-generation hooks**

## 38.2 Event hooks

Expose typed, deterministic semantic hooks such as:

-   [ ] Pawn spawned.
-   [ ] Pawn died.
-   [ ] Damage resolved.
-   [ ] Job started/completed/failed.
-   [ ] Building constructed/destroyed.
-   [ ] Item produced/consumed.
-   [ ] Research completed.
-   [ ] Incident started/resolved.
-   [ ] Relationship changed.
-   [ ] New day/quadrum/year.
-   [ ] Caravan entered/left.
-   [ ] Faction goodwill changed.

Hooks must have documented ordering.

## 38.3 Sandboxed scripting

-   [ ] **Choose scripting runtime**
    -   Starlark or WASM preferred candidates.
    -   No arbitrary native Go plugin loading for untrusted community
        mods.
-   [ ] **Deterministic host API**
    -   Simulation tick, not wall clock.
    -   deterministic RNG service.
    -   stable entity handles.
    -   typed commands.
-   [ ] **Resource limits**
    -   CPU/instruction budget.
    -   memory limits where runtime permits.
    -   recursion/stack protection.
-   [ ] **No unrestricted OS access**
    -   No arbitrary filesystem.
    -   no network.
    -   no process execution.
-   [ ] **Script error isolation**
    -   Plugin failure reports plugin ID and hook.
    -   Avoid corrupting authoritative state.

## 38.4 Save support for plugins

-   [ ] **Plugin-owned persistent state**
    -   Namespaced versioned blob/schema.
    -   Deterministic serialization.
-   [ ] **Plugin migration API**
-   [ ] **Missing-plugin save diagnostics**
-   [ ] **Plugin set stored in save metadata**
-   [ ] **Content hash/replay compatibility**

## 38.5 Plugin UI

-   [ ] **Plugin manager**
    -   Installed.
    -   enabled.
    -   version.
    -   dependencies.
    -   conflicts.
    -   load order.
-   [ ] **Plugin settings**
    -   Typed settings schema.
    -   UI generated from schema where possible.
-   [ ] **Restart/reload requirements**
    -   Clearly communicate when content registry changes require new
        session/restart.

------------------------------------------------------------------------

# PHASE 39 --- Developer Mode, Debugging, Profiling, and Mod Author Tools

RimWorld exposes development/debug facilities and modding documentation
commonly relies on debug actions and configuration errors. `rimworld-go`
should make these first-class because the project is open source and
agent-developed. citeturn0search4

-   [ ] **Developer mode**
-   [ ] **Spawn entity/item/pawn**
-   [ ] **Trigger incident**
-   [ ] **Set weather**
-   [ ] **Set time**
-   [ ] **Damage/heal pawn**
-   [ ] **Add/remove health condition**
-   [ ] **Finish research**
-   [ ] **Change goodwill**
-   [ ] **Teleport pawn**
-   [ ] **Reveal path**
-   [ ] **Inspect reservations**
-   [ ] **Inspect current intent/job**
-   [ ] **Inspect component state**
-   [ ] **Inspect room/region/chunk IDs**
-   [ ] **Navigation overlay**
-   [ ] **Temperature overlay**
-   [ ] **Power network overlay**
-   [ ] **Incident/storyteller debug panel**
-   [ ] **RNG stream diagnostics**
-   [ ] **State hash inspector**
-   [ ] **Tick profiler**
-   [ ] **System timing profiler**
-   [ ] **Entity/chunk activity profiler**
-   [ ] **Pathfinding metrics**
-   [ ] **Job candidate metrics**
-   [ ] **Heap/allocation metrics**
-   [ ] **Plugin provenance inspector**
    -   Show which plugin supplied/patched a definition.

------------------------------------------------------------------------

# PHASE 40 --- Performance Scaling Beyond Vanilla

The project goal is not merely feature parity; it is to support
substantially larger colonies and maps with lower resource usage.

## 40.1 Sleeping and activity levels

-   [ ] **Chunk activity states**
    -   Sleeping.
    -   background.
    -   active.
    -   critical.
-   [ ] **Entity sleeping**
    -   Inactive plants/items/buildings do not tick every frame.

## 40.2 Analytical progression

-   [-] **Plants** (NG-1: grown per chunk once per 250 ticks from the elapsed span with `plant.GrowSpan`; other systems still tick)
    -   Advance growth from elapsed ticks.
-   [ ] **Rot/deterioration**
-   [ ] **Needs where safe**
-   [ ] **Disease/health effects where safe**
-   [ ] **World settlements/caravans**
    -   Aggregate simulation rather than local entity simulation.

## 40.3 Invalidation

-   [x] **Navigation dirty regions** (NG-1: a walkability change marks its 16x16 chunk dirty; walk regions and labels rebuild for dirty chunks only)
-   [x] **Room dirty regions** (NG-1: incremental rooms from room regions per chunk, checked against full detection in tests)
-   [ ] **Power topology dirty**
-   [ ] **Temperature topology dirty**
-   [ ] **Beauty dirty**
-   [ ] **Storage dirty**
-   [ ] **Cover dirty**
-   [ ] **Lighting dirty**
-   [ ] **Roof dirty**

## 40.4 Spatial indexes

-   [x] **Entities at cell** (NG-1: per-cell indexes for plants, buildings, frames and items)
-   [ ] **Pawns in chunk/region**
-   [ ] **Items by category/region**
-   [x] **Work opportunities** (NG-1: maintained candidate sets for frames, deconstruct and harvest/cut marks, refuel and feed targets)
-   [ ] **Storage destinations**
-   [-] **Hostiles** (threat scans are indexed)
-   [x] **Haulables** (stacks not already in a matching stockpile)
-   [ ] **Medical patients**
-   [ ] **Fires**
-   [x] **Plants ready for work** (harvest- and cut-marked plants)

## 40.5 Large-colony UX

-   [ ] **Pawn groups**
-   [ ] **Group work presets**
-   [ ] **Group schedules**
-   [ ] **Group allowed areas**
-   [ ] **Bulk policy assignment**
-   [ ] **Virtualized pawn lists**
-   [ ] **Aggregated alerts**
-   [ ] **Search/filter all management screens**

## 40.6 Performance acceptance

-   [ ] **100-colonist benchmark**
-   [ ] **500-colonist benchmark**
-   [ ] **1,000-colonist benchmark**
-   [ ] **100k item/plant benchmark**
-   [ ] **1024×1024 map benchmark**
-   [ ] **Long-run heap stability**
-   [ ] **No goroutine-per-pawn/entity**
-   [ ] **No hidden full-world scans in hot phases**

------------------------------------------------------------------------

# PHASE 41 --- Final Vanilla-Parity Audit

Do not call the vanilla milestone complete until this audit passes.

## 41.1 Colony survival loop

-   [-] Start with generated colonists and supplies. (Ada/Bo/Cora + food/wood/plants; wizard does not change kit)
-   [-] Build shelter. (wood walls; no rooms/roofs) (SIM-018: roofed rooms now built)
-   [-] Assign beds. (claim on Sleep)
-   [-] Create stockpile. (food+wood paint)
-   [x] Grow crops.
-   [ ] Hunt animals.
-   [ ] Cook food.
-   [-] Survive hunger/rest/temperature. (hunger/rest downed+starve/exhaustion; no temperature)
-   [ ] Treat injuries/disease.
-   [-] Manage mood.
-   [ ] Survive mental breaks.
-   [ ] Produce equipment.
-   [ ] Research technology.
-   [ ] Generate power.
-   [ ] Refrigerate food.
-   [ ] Trade.
-   [ ] Recruit prisoners.
-   [ ] Tame/train animals.
-   [ ] Defend raids.
-   [ ] Survive environmental events.
-   [ ] Form caravans.
-   [ ] Complete world quests.
-   [ ] Build/activate endgame ship.
-   [ ] Survive final assault period.
-   [ ] Escape.

## 41.2 Emergent-story audit

-   [ ] Pawns have meaningfully different skills/traits.
-   [ ] Pawns form opinions and relationships.
-   [ ] Relationships create gameplay consequences.
-   [ ] Injuries have anatomically meaningful consequences.
-   [x] Mood responds to environment and memories.
-   [ ] Low mood creates autonomous disruptive behavior.
-   [ ] High mood can create positive opportunities.
-   [ ] Storyteller alternates pressure/opportunity/recovery.
-   [ ] Enemy factions behave differently by tech/tactic.
-   [ ] Colony wealth increases stakes.
-   [ ] Deaths and disasters remain visible in colony history.
-   [ ] The game can produce interesting stories without scripted
    campaign missions.

## 41.3 Simulation integrity audit

-   [ ] Same seed + same commands = same authoritative result.
-   [ ] Renderer FPS does not change result.
-   [ ] Worker count does not change result.
-   [ ] Save/load preserves canonical hash.
-   [ ] Replay preserves canonical hash.
-   [ ] Plugin load order is deterministic.
-   [ ] No stale reservations after long-run simulation.
-   [ ] No negative/duplicated item quantities.
-   [ ] No references to destroyed entities.
-   [ ] No invalid room/power/navigation cache after topology changes.

## 41.4 Architecture audit

-   [ ] Domain imports no renderer.
-   [ ] Domain imports no filesystem persistence implementation.
-   [ ] Domain imports no generated serialization DTOs.
-   [ ] UI mutates game only through commands/application ports.
-   [ ] Plugin scripts cannot directly mutate internal stores.
-   [ ] External input is validated at boundaries.
-   [ ] Canonical types are not duplicated.

## 41.5 Go quality audit

-   [ ] `gofmt` clean.
-   [ ] `go vet ./...`.
-   [ ] `staticcheck ./...`.
-   [ ] `go test ./...`.
-   [ ] Race tests for concurrent infrastructure.
-   [ ] Fuzz targets for unsafe/untrusted parsers and critical
    transitions.
-   [ ] Hot benchmarks report allocations.
-   [ ] No unexplained `unsafe`.
-   [ ] No global mutable simulation state.

------------------------------------------------------------------------

# PHASE 42 --- Release Definition

## Vanilla Alpha

Can be called **Vanilla Alpha** when:

-   [ ] Phases 1--17 are substantially complete.
-   [ ] Basic storyteller raids/events exist.
-   [ ] Save/load works.
-   [ ] A colony can survive indefinitely.
-   [ ] Combat, health, mood, farming, construction, research, and power
    form a coherent loop.

## Vanilla Beta

Can be called **Vanilla Beta** when:

-   [ ] Animals/prisoners/factions/trade are complete.
-   [ ] World generation/caravans/quests work.
-   [ ] Major raid types and incident catalog work.
-   [ ] Endgame is completable.
-   [ ] Full management UI is usable.
-   [ ] Plugin data definitions are usable.

## Vanilla 1.0

Can be called **Vanilla 1.0** when:

-   [ ] Phase 41 parity audit passes.
-   [ ] Plugin behavior API is stable/versioned.
-   [ ] Save migrations are tested.
-   [ ] Large-colony performance targets have reproducible benchmarks.
-   [ ] Tutorial/onboarding exists.
-   [ ] Crash/replay diagnostics are production-ready.
-   [ ] Original or properly licensed art/audio/content is sufficient
    for distribution.

------------------------------------------------------------------------

# Appendix A --- Explicitly Out of Scope: DLC Systems

Do **not** accidentally pull these into vanilla implementation unless
they later become separate optional plugins/expansions.

## Royalty-style expansion systems

-   Psychic powers/psylinks.
-   Imperial honor/title hierarchy.
-   Nobility throne-room requirements.
-   Permits.
-   Royal court mechanics.
-   Mech clusters that specifically depend on expansion content.
-   Techprint-specific expansion progression.

## Ideology-style expansion systems

-   Ideoligions.
-   Memes/precepts.
-   Rituals.
-   Roles.
-   Relics.
-   Slavery as the expansion's full management system.
-   Biosculpters.
-   Dryads/Gauranlen trees.
-   Ideology-specific apparel/food/mood rules.

## Biotech-style expansion systems

-   Children/babies/growth moments.
-   Pregnancy system as implemented by Biotech for humans.
-   Genes/xenotypes.
-   Mechanitor/player-controlled mech system.
-   Pollution/wastepacks.
-   Sanguophages.
-   Gene extraction/implantation.

## Anomaly-style expansion systems

-   Monolith progression.
-   Entities/anomaly containment.
-   Dark research.
-   Ritual anomaly system.
-   Anomaly-specific storyteller threats.

## Odyssey-style expansion systems

-   Expansion-specific travel/vehicle/space systems.
-   Expansion biomes.
-   Expansion-specific exploration systems.

If a modern base-game mechanic has changed because of a free update,
agents should document the target behavior before implementing it rather
than silently importing DLC-only dependencies.

------------------------------------------------------------------------

# Appendix B --- Cross-Cutting Definition Types Required by the Plugin System

The plugin system should ultimately be able to define or extend at
least:

-   [ ] `ThingDef`-equivalent generic entity/item/building definition.
-   [ ] Item definitions.
-   [ ] Material/stuff definitions.
-   [ ] Terrain definitions.
-   [ ] Building definitions.
-   [ ] Plant definitions.
-   [ ] Pawn/species definitions.
-   [ ] Body/anatomy definitions.
-   [ ] Health-effect definitions.
-   [ ] Weapon/projectile definitions.
-   [ ] Apparel definitions.
-   [ ] Recipe definitions.
-   [ ] Workstation definitions.
-   [ ] Research definitions.
-   [ ] Trait definitions.
-   [ ] Backstory definitions.
-   [ ] Thought definitions.
-   [ ] Need definitions where safely extensible.
-   [ ] Recreation definitions.
-   [ ] Drug definitions.
-   [ ] Faction definitions.
-   [ ] Trader definitions.
-   [-] Biome definitions. (SIM-023: 12 biomes as Go data in `world/biome`, not data-pack files)
-   [ ] World-generation definitions.
-   [-] Incident definitions. (SIM-025: nature events as Go data in `domain/incident`, not data-pack files)
-   [ ] Storyteller definitions.
-   [ ] Scenario definitions.
-   [ ] Quest archetype/content definitions.
-   [ ] Sound-event mappings.
-   [ ] Localization.
-   [ ] UI icons/textures.
-   [ ] Stat definitions/modifiers where stable API permits.

------------------------------------------------------------------------

# Appendix C --- Definition of Done for Every Checkbox

An agent must **not** tick a feature merely because code compiles.

A checkbox is complete only when all applicable conditions are true:

-   [ ] Player-visible behavior exists.
-   [ ] Domain behavior has automated tests.
-   [ ] Deterministic ordering is defined.
-   [ ] RNG uses deterministic project RNG.
-   [ ] Hot-path complexity is acceptable.
-   [ ] No avoidable global scan was introduced.
-   [ ] Save/load includes new authoritative state.
-   [ ] Replay/state hash includes relevant state.
-   [ ] Plugin/content surface is data-driven where appropriate.
-   [ ] Debug tooling can inspect the feature where practical.
-   [ ] UI explains important failures/states.
-   [ ] Relevant package `AGENTS.md` remains correct.
-   [ ] Relevant system doc/ADR is updated.
-   [ ] `gofmt`, `go vet`, `staticcheck`, and tests pass.
-   [ ] Benchmarks are added/updated for performance-sensitive systems.
-   [ ] Task handoff is written.

------------------------------------------------------------------------

# Appendix D --- Research Basis and Version Caveat

This checklist was prepared against the current RimWorld Wiki material
available in September 2026, with DLC-marked mechanics excluded from the
vanilla target. The wiki documents the core game as a generated-world
colony story simulator with individually modeled colonists, needs/mood,
work priorities, body-part health, factions/trade, research, world
travel, storyteller incidents, and a broad data/code mod ecosystem.

Important implementation references used when constructing the checklist
include the RimWorld Wiki pages for:

-   Basics and Quickstart Guides.
-   About RimWorld / PC edition.
-   Characters.
-   Needs, Mood, Beauty, and Recreation.
-   Work.
-   Combat.
-   Health.
-   Research.
-   World generation.
-   Factions, Trade, and Caravans.
-   Events.
-   Modding Tutorials.
-   XML Defs.
-   About.xml / mod metadata.
-   Mod folder structure.
-   Writing custom code and Harmony.

Because RimWorld continues to receive free base-game updates alongside
expansions, a mechanic that appears on a current wiki page may be marked
as requiring a DLC. Those DLC-marked portions are **not** part of this
checklist unless the underlying mechanic is independently present in the
base game.

When uncertainty exists, agents must record it as a backlog
clarification rather than silently implementing expansion behavior.

------------------------------------------------------------------------

# Final Agent Directive

The objective is not to reproduce RimWorld's implementation.

The objective is to reproduce the **depth of the vanilla
colony-simulation experience**:

> autonomous individual colonists + survival + work + construction +
> production + psychology + relationships + health + combat +
> environment + factions + world travel + storyteller-driven adversity +
> emergent stories,

while making the implementation:

-   deterministic;
-   open;
-   moddable;
-   much more scalable;
-   resource efficient;
-   idiomatic Go;
-   headless-testable;
-   safe for multiple AI agents to develop concurrently.

When parity conflicts with scalability, preserve the **player-visible
behavior** while improving the underlying algorithm.
