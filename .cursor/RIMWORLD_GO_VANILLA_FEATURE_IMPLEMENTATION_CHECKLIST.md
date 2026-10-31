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
-   [-] **Deterministic benchmark runner** (NG-1 ENG-001: `cmd/benchmark` and the grown-colony fixture time ticks, phases and allocations at 120, 250 and 500 tiles a side with 20 colonists; PERF-002: a `views` row times the read model and `frames > 5ms` counts what the Update budget would hold, and `make fps` is the repeatable uncapped frame probe; the colonist-count scales of 100 to 1,000 are not covered)
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

-   [-] **Generic inspect panel** (name, job, status/reason, tile, food/rest bars; Gear tab live; Health/Social tabs live with a "No data yet." placeholder until their read models fill, ENG-003)
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

-   [x] **Entity reservation** (exclusive pawn→entity; RES-1: `ReleaseTarget` frees every claim on a target)
-   [x] **Tile reservation** (RES-1: `ReleaseTile`; haul dest / move target; a delivery route reserves its spare stockpile tile)
-   [x] **Item quantity reservation** (RES-1: `TryCount`/`Free` count claims share a stack for eating, hauling pickups, bill ingredients and medicine, and a pickup releases the rest; in-flight material per frame/fuel user: haulers' shares are booked so a frame is never over-delivered (HAUL-001); HAUL-001 shares remain a separate book)
-   [-] **Bed/patient reservation** (bed `Claimant` for Sleep; PLAY-020: patients take a medical bed for the length of their rest, and a sowing cell is held by one grower; RES-1: `SleepIn` skips a bed another pawn holds; no per-patient policy)
-   [x] **Workstation reservation** (RES-1: a bill or research job claims the bench's work spot)
-   [x] **Automatic reservation cleanup** (RES-1: `ReleaseTarget` hooks in `deleteItem`, `removeBuilding`, `removePlant`, `removeAnimal` and `dieBy`; complete/cancel/replace/death)
    -   Completion.
    -   cancellation.
    -   death/downing.
    -   target deletion.
    -   job replacement.

## 5.3 Work settings

-   [-] **Work types** (Haul/Construct/Grow wired; other columns are UI stubs; PLAY-020: bed rest is a per-colonist "Rest when hurt" setting, on by default, not a work column)
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

-   [x] **Right-click/context forced work** (`OrdersAt` menu: go here, build, supply wood, deconstruct, harvest, sleep in bed, eat, haul to stockpile; disabled entries give the reason; `Prioritize` command; SIM-062: orders on a pawn — Rescue, Capture, Arrest, Prioritize tending, drafted Tend — with the target in `Command.Target`, from `care.PawnOrders`)
    -   Player can tell a selected pawn to perform a valid specific
        action immediately.
-   [x] **Queued forced jobs** (Shift queues up to 3 orders behind the current one; queue panel with Clear and numbered route on the map; invalid orders skipped with a message; SIM-062: pawn-targeted orders and drafted Tend queue too, labelled by target)
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

-   [x] **Named allowed areas** (SIM-046; PLAY-015 SIM-055: up to 10 areas with stable ids and colours; paint, clear, rename, invert, delete (users fall back to Unrestricted); Home as an area choice; colonists and tame non-penned animals; coloured overlay)
    -   Paint cells.
    -   invert/clear.
    -   assign pawn.
-   [x] **Area-aware AI** (SIM-046; PLAY-015: destinations (work, haul destination, food, beds, recreation, wander, hunting) must be inside; paths may cross outside at a cost; drafted, mental, fleeing, attack, rescue/tend and following a drafted master override it)
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
-   [-] **Stackable items** (stack limit 1000, carry 100 per trip, merge/take, no negatives; PRD-4: `item.Kind.StackLimit` carries the wiki limit for the wave E kinds, hauling, merging and spilling stop at it; missing: the older kinds keep 1000 so saves and pins stay valid)
    -   Quantity.
    -   stack limit.
    -   merge/split.
    -   no negative or duplicated quantity.
-   [-] **Item quality** (PRD-2: quality rolled from skill and inspiration on apparel, weapons, beds, bedrolls, animal bed, end table, dresser, table, chair and constructed buildings; grenades unrolled; art waits for PRD-3)
    -   Quality tiers for relevant crafted
        items/buildings/apparel/weapons/art.
    -   Quality affected by creator skill and other factors.
    -   Exact RimWorld probabilities need not be copied.
-   [x] **Item hit points/deterioration** (PRD-2: item HP, 2 HP/day outdoors (original tuning), rain x5, water x3, roofed 0, metal and stone immune, quality factor; shelves hold items and stop deterioration since PRD-4)
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

-   [x] **Stockpile creation** (paint tiles; one zone per tile; adjacent stockpiles merge; PLAY-023: RimWorld placement rules (no walls, impassable buildings or their blueprints, deep water, rock, fog or the outermost map ring; shallow water allowed; refused cells name their reason), cells removed and zones split when something is built over them, dumping stockpiles (animal corpses and chunks), priority Critical..Low with re-hauling to higher priority (1 h minimum stay))
    -   Paint storage cells.
-   [x] **Storage filters** (PRD-4 then STO-2: `domain/building/zone/filter.go` tree to depth 3 with human corpses, bones, body parts and unfinished categories; `FilterRoot` allow all/clear all; 10 special toggles in `special.go` (fresh, non-deadman's apparel, smeltable, burnable, grave colonist/stranger, rotten, tainted and others) plus HP and quality ranges; vanilla defaults for new stockpiles (no corpses, bones, chunks or rotten; dumps take corpses and chunks), old zones keep their switches; copy and paste, names; stockpiles and shelves)
    -   Category tree.
    -   allowed/disallowed definitions.
    -   quality/condition filters where appropriate.
-   [x] **Storage priorities** (PLAY-023: Critical…Low per stockpile; STO-3: shelves, hoppers, sarcophagi and graves take a priority too; re-hauling to a higher priority)
    -   Multiple priority levels.
    -   Haulers move goods from lower-priority to higher-priority valid
        storage.
-   [x] **Storage capacity/index** (STO-1: `haul.Ground` per-cell index and `zone.Stores` by priority; state hashes unchanged)
    -   Efficiently find valid free cells.
    -   Avoid map-wide storage scans.
-   [x] **Linked storage groups** (STO-2: `group.go` Link/Unlink, an edit reaches every member, `zone.Map.SameStore`, a group can be a bill source)
-   [-] **Hopper and nutrient paste dispenser** (STO-2: `game/store`; dispenser 0.3 nutrition per 0.9 meal, up to 13 hoppers, 200 W, research, paste thought -4; hoppers hold raw food at Important and refill below half (original); FOOD-4: haulers and cooks fill hoppers through the `FillHopper` job, wardens and doctors fetch paste for patients and prisoners; missing: none known, kept partial for the dispenser's original hopper numbers)
-   [x] **Graves and sarcophagi as storage** (STO-2: corpse filters and priority; sarcophagus holds colonists only)
-   [x] **Wiki stack limits** (STO-2: every item kind; corpses and gear one per stack)

## 6.4 Hauling

-   [-] **Haul loose item to storage** (up to 100 per trip to the nearest accepting stockpile tile, topping up stacks first; PLAY-017: only the colony's own goods (made, brought, harvested, chopped, mined, hunted) are hauled by themselves, items found on the map (ruin and vault loot, map chunks, predator kills, wild eggs) wait for a Haul mark; HAUL-002: one trip gathers the nearby compatible piles)
-   [-] **Haul construction resources** (a full load of wood or stone feeds a route of up to 8 nearby frames; leftovers go to the nearest stockpile, or only the needed amount is carried when storage has no room (HAUL-001))
-   [x] **Haul production ingredients** (PRD-1: a crafter gathers a bill's ingredients from several stacks within the bill's radius onto the bench, `job.HaulIngredients`)
-   [x] **Haul finished products** (PRD-1: store mode best stockpile hauls the product; drop on floor leaves it; a specific stockpile)
-   [x] **Haul corpses**
-   [x] **Haul urgently** (STO-2: designator plus urgent mark in `game/store`)
-   [x] **Opportunistic/batched hauling where compatible** (batched for construction materials and refuelling; HAUL-002: a stockpile haul or Haul order gathers nearby compatible piles (`item.SameLoad`, an 8-tile chain) up to the carry limit and the destination cell's room, then delivers once (`game/haul`))
    -   Performance-conscious without changing core player expectations.

------------------------------------------------------------------------

# PHASE 7 --- Designations, Construction, Buildings, Rooms, and Zones

## 7.1 Designation system

-   [-] **Build designation** (`DesignateBuild` for wall/door/floor/bed/table/chair/lamp with facing; 1×1 footprints only)
-   [x] **Deconstruct designation** (Architect Orders rectangle; cancel clears it)
-   [x] **Mine designation** (Architect Orders rectangle over rock; Mine work type; cancel clears)
-   [x] **Cut plant designation** (trees drop wood scaled by growth; bushes cleared)
-   [x] **Harvest designation** (wild plants by order; PLAY-017: zone crops get the harvest mark when they ripen, and Cancel holds until the next ripening; Grow work type)
-   [x] **Hunt designation**
-   [x] **Plan designation** (PLAY-012 UI-023: Plan / Remove plan paint visual layout marks with no gameplay effect; a mark clears when any building is finished on its tile; Plans button hides the overlay)
-   [-] **Cancel designation** (frames refund delivered materials (UI-015 `CancelFrame` and the Cancel area), harvest marks, deconstruct marks; zones lose just the cancelled tiles (ZONE-001))
-   [x] **Forbid/unforbid designation/action** (SIM-028 Forbid/Allow area tools and item toggle; UI-016 F key)

## 7.2 Construction lifecycle

-   [-] **Blueprint** (`CanPlace` shared by preview and sim: bounds, layer occupancy, blocked, items under walls; floor + object layers share a tile; doors and gates may replace walls and fences (deconstruct first, UI-021); doors and gates face along their walls and fences by themselves, and placing the same piece in another material swaps it in place (UI-022); no research/support checks)
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
-   [-] **Construction work** (NG-1: Construction skill scales the work speed; no quality or failed construction; PRD-2 now rolls building quality; still no failed construction)
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
-   [-] **Doors** (buildable, walkable for colonists, shut to raiders and wild animals, count as wall neighbours, drawn open when occupied; DEF-3: opening delay by stuff, powered autodoor, hold open, open door lets shots through, `domain/building/door`; missing: door forbid UI)
    -   Open/closed states.
    -   Opening delay.
    -   faction/permission interaction.
    -   affect pathing and room boundaries.
-   [x] **Bridges** (MAPGEN-3: research-gated wooden foundation layer over water, marsh or mud, never the open sea; walkable at full speed, holds light things (wooden walls, doors, fences, furniture) but not stone/steel walls or floors; a destroyed bridge breaks what it held and sinks items on deep water; generated road bridges)
-   [-] **Floors** (buildable/deconstructable separate layer; SIM-028: may go under any furniture, not on rock, walls, fences/gates or dug graves; no movement/beauty/cleanliness stats)
    -   Construct/remove.
    -   movement, beauty, cleanliness, flammability attributes.
-   [-] **Roofs** (SIM-018; FOOD-6: a roof shelters pawns, animals and plants from toxic fallout; no deterioration or collapse injuries)
    -   Roof cells.
    -   roof construction/removal jobs.
    -   support rules.
    -   roof affects weather, temperature, light, deterioration, toxic
        fallout.
-   [-] **Furniture** (PRD-4: shelf, a storage zone over its footprint, 3 stacks per cell, Preferred, no deterioration, beauty 0.5; bed (rotatable, claimed for Sleep), table, chair (rotatable), lamp buildable; BED-1: bedroll, double bedroll, animal sleeping spot, animal bed, end table, dresser; bed comfort, rest effectiveness and beauty; BED-3: stool, dining chair and armchair carry comfort and beauty and are sat on (`rest/seats.go`); PRD-3: sculptures (small 1x1, large 1x1, grand 2x2) are furniture with quality-scaled beauty; missing: couch, joy, light effects)
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
-   [x] **Room statistics** (cleanliness and an ad-hoc impressiveness; missing: space, beauty, wealth and the vanilla impressiveness formula — BED-2; BED-2: space, beauty, wealth, cleanliness and the wiki impressiveness formula, tallied at room rebuild with live filth and item value; RoomView carries them)
    -   Space.
    -   beauty.
    -   cleanliness.
    -   wealth/value.
    -   impressiveness derived from room properties.
-   [x] **Room roles** (general, bedroom, barracks, dining, workshop; missing: hospital, prison cell/barracks, rec room, kitchen — BED-2; BED-2: adds hospital, prison cell, prison barracks, kitchen and recreation room)
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

-   [x] **Growing zones** (painted tiles sowing the chosen crop; same-crop neighbours merge; cancel clears tiles; resow after harvest (ZONE-001); PLAY-023: only on soil, rich soil, gravel or marsh, never on floors, bridges, water, sand, mud or ice, fog or the map edge; a floor or building over a cell removes it; sowing only while the growing temperature is between 0 and 58 °C; FOOD-6: a roofed cell is sown only within a powered sun lamp's 51 % reach, never with a tree)
-   [x] **Home area** (SIM-021; PLAY-015: auto-expansion toggle, grows around buildings, new zones and pen markers; bounds cleaning, repairs and firefighting together with the pawn's area)
    -   Influences firefighting/cleaning/repair behavior.
-   [x] **Roof build/remove areas** (SIM-018)
-   [x] **Allowed areas** (SIM-046; PLAY-015: manager with invert/delete, Architect submenus, overlay, Schedule/Animals/inspector selectors)
    -   Already introduced in Phase 5 but integrated with zone UI.

------------------------------------------------------------------------

# PHASE 8 --- Food, Hunger, Farming, Cooking, and Survival

## 8.1 Nutrition

-   [x] **Nutrition values** (FOOD-1, wiki scale: a full bar is 1.0 = 10000; raw food, milk, pemmican and kibble 0.05 a unit, simple/fine/survival/paste meals 0.9, lavish 1.0, eggs by species 0.25-0.6, the legacy ration a survival meal; one meal or up to 20 raw units a sitting, overflow wasted)
    -   Foods provide nutrition.
    -   Pawns consume an amount based on hunger/body needs.
-   [x] **Food need** (FOOD-1: 1.6 nutrition a day for an adult, Fed/Hungry/Ravenous falls 100/50/25 %, thoughts Hungry -6, Ravenously hungry -12, Malnourished -20...-44 with a band hysteresis; eats at 30 % and keeps looking until 80 %, sleepers wake only ravenous; malnutrition at 0 % rises 1.51-2.27 %/h (rolled per pawn), recedes when fed, five stages with consciousness and hunger offsets, stops wound healing, kills at 100 %; `PawnView.Malnutrition`; FOOD-6: immunity gain x0.9 ravenous, x0.7 malnourished; wiki hunger formula, offsets (gut worms, malnutrition, stoned on the ashleaf stand-in +30 %) add before the gourmand multiplier; the wiki's -10/-40/-95 % is insect-only hypothermic slowdown, not modelled)
    -   Hunger decays over time.
    -   Thresholds affect behavior/mood.
    -   Severe hunger causes malnutrition/starvation and can kill.
-   [-] **Food search** (nearest edible stack; entity reserved; SIM-028: forbidden stacks skipped, animals never pick food behind fences; FOOD-1: a malnourished pawn eats animal carcasses too; FOOD-4: food policies filter the search, choice scores 300 - distance + mood worth with kibble and corpse penalties and a near-rot bonus (original numbers); missing: a vanilla optimality formula (not published))
    -   Prefer valid edible food according to restrictions and
        availability.
    -   Reserve food before fetching it.
    -   Respect forbidden/allowed area rules.
-   [x] **Eating job** (fetch the portions needed, carry them to a table seat - chair preferred - within 30 tiles, ingest; FOOD-4 thoughts: fine +5, lavish +12, raw -7, kibble -12, paste -4, no table -3; human and insect meat and butchered-human thoughts, the cannibal trait flips them; patients and prisoners get the thoughts too)
    -   Fetch.
    -   choose table/seat when reasonable.
    -   ingest over time.
    -   gain nutrition.
    -   generate context-dependent thought later.

## 8.2 Raw food

-   [x] **Plant food** (harvest yield stacks; vegetarian and carnivore variants and policy categories by FOOD-4)
-   [x] **Meat**
-   [x] **Animal products** (SIM-026, FOOD-3: milk, wool and eggs per species with saturation scaling, fertile eggs edible, omnivores eat eggs and milk; Handle work type)
    -   Milk/eggs where species definitions support them.
-   [ ] **Food categories and diet compatibility**

## 8.3 Cooking

-   [x] **Cooking workstation** (stove)
-   [x] **Meal recipes** (FOOD-4: recipes take nutrition per item, vegetarian and carnivore fine and lavish variants, batch x4 bills, ingredients recorded on every meal, fueled stoves burn wood, benches outside 10-35 C work at 70 %; simple meal at the stove or slower over a campfire; PRD-4: fine meal, lavish meal, pemmican, packaged survival meal (multi-ingredient, any-veg and any-meat inputs, wiki skills and yields) and kibble at the butcher table and spot)
    -   Simple/standard meal classes sufficient to recreate vanilla
        progression.
    -   Ingredient filters.
    -   nutrition conversion.
    -   work amount.
-   [x] **Food poisoning** (FOOD-4: wiki room-cleanliness and cook-skill rolls, a poisoned batch poisons all of it, fed patients roll too; FOOD-6: wiki stage effects incl. blood filtration, an eating speed factor (`body.FoodPoisonEatSpeed`, read by the eat step), a synth stomach halves the chance; MED-11: cooked meals carry a poison chance from the cook's Cooking skill and kitchen cleanliness (campfire meals at least 1.5 %); eating rolls it into a 24 h food-poisoning condition with vomiting and bed rest at its major stage; never lethal)
    -   Chance influenced by cook skill and cleanliness/conditions.
    -   Health effect and mood consequences.
-   [x] **Butchering** (FOOD-3: body-size formula by age, missing parts and damage, species-named meat, insect and human meat, rotten corpses give leather; FOOD-6: 450 ticks of work, the butcher spot yields 70 %, default bills butcher animals only)
    -   Corpse -\> meat + leather/other products.
    -   Yield affected by species and skill/stat.

## 8.4 Farming

-   [-] **Fertility** (hover text only in `hud/cellinfo.go`; no painted fertility overlay: FOOD-5)
    -   Terrain fertility affects plant growth.
-   [x] **Crop definitions** (FOOD-2: 15 crops incl. rice, strawberry, haygrass, hops at wiki grow days, yields and sensitivity; sow 170 / harvest 200 work; minimum sowing skill for healroot, strawberry, hops and trees; devilstrand, smokeleaf and psychoid are out of food scope; picker numbers FOOD-5)
    -   Growth time.
    -   fertility sensitivity.
    -   yield.
    -   minimum sowing skill.
    -   temperature/light requirements.
-   [x] **Sowing** (Grow work sows empty fertile zone cells; prioritizable from the order menu; tree zones are planted, chopped when grown and replanted (UI-011); PLAY-017: growers first cut a wrong plant (wild, moss, bush, tree or another crop) off a zone cell, a ripe one is harvested)
    -   Growers plant designated crops in growing zones.
-   [x] **Plant growth** (FOOD-2: rest 19:00-06:00 in any light, 51 % light minimum, wiki temperature curve and cold death, 65 % partial harvest, lifespan 5 x grow days, hydroponics 280 %, sun lamp, blight; integer per-tick growth scaled by fertility x soil sensitivity x temperature; PLAY-016: cold and heat damage plants over hours, shown in the plant view and cutting the harvest; fire burns them via PLAY-015)
    -   Analytical/bucketed growth.
    -   Depends on temperature, light, fertility, plant health.
-   [x] **Harvest maturity** (plants ripen at GrowTicks; berry bushes regrow to 35% after harvest)
    -   Mature plants generate harvest work.
    -   Skill can affect yield/failure.
-   [x] **Wild plants** (seeded generation scatters trees and berry bushes on fertile ground; PLAY-010 SIM-038: per-biome bramble thicket (slows walking), ground moss, desert rosette, ball cactus, elephant ear, fire lily, dandelions)
    -   Map generation creates harvestable/non-harvestable vegetation.

## 8.5 Food spoilage

-   [x] **Rot progression** (FOOD-4: wiki days to rot, food deteriorates unroofed with food HP, shelves protect, meals 50 HP; per-stack rot ticks; full speed from 10C, slower when cool, none at 0C or below; merges average by quantity)
    -   Perishable food/corpses rot based on temperature and elapsed
        time.
-   [-] **Frozen/refrigerated preservation** (winter and cold rooms freeze food; a powered cooler holds a room at its thermostat target; no freezer building)
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

-   [x] **Rest need** (awake decay, sleep restore 20%->100% in 8 h at rest effectiveness 100 %, scaled by the bed's rest effectiveness (BED-1: ground and spots 80 %); sleepers wake at 80% outside Sleep hours and 100% in them; at 0 the colonist collapses and sleeps where it drops until 80%)
    -   Decays while awake.
    -   Replenishes while sleeping.
    -   Severe exhaustion can force sleep.
    -   Tiredness affects mood and performance.
-   [-] **Sleep intent** (owned bed else nearest free bed else current tile, also when the bed is out of reach; schedule-aware: tired sleep only outside Work hours)
    -   Schedule-aware.
    -   Select owned/assigned bed if available.
    -   Fall back to available sleeping location.
-   [x] **Beds** (SIM-042/045, PLAY-020: partner double beds, owner assignment UI, prisoner beds, medical beds; claim on Sleep; BED-1: comfort, rest effectiveness and beauty by kind × stuff × quality, end table and dresser links, bedrolls, animal beds, forbidding)
    -   Assign ownership.
    -   sleeping slots.
    -   rest effectiveness.
    -   comfort.
    -   medical/prisoner designation later.

## 9.2 Comfort

-   [x] **Comfort need** (BED-3: `rest/needs.go`; rises sitting on stool, dining chair or armchair and asleep in a bed (end table and dresser count), falls otherwise; wiki ladder -3..+10; SitDown job in `rest/seats.go`; idle Relax and meals prefer seats)
    -   Current furniture/bed comfort drives a slowly changing need.
    -   Comfortable beds/chairs create positive psychological effect.

## 9.3 Beauty

-   [x] **Tile beauty** (BED-3: floor, filth (outdoor 30 % of wiki), building once, plant, sculpture beauty; `rest/beauty.go`)
    -   Buildings, items, filth, natural objects, art, and terrain
        contribute.
-   [x] **Perceived beauty** (BED-3: average over 241 cells of the pawn's room or open ground every 250 ticks; local sampling, no per-cell cache)
    -   Pawn samples local surroundings.
    -   Cached/low-frequency evaluation.
-   [x] **Beauty need** (BED-3: trends toward perceived beauty, wiki ladder; target and drift rates original)
    -   Gradually trends toward environmental perception.
    -   Generates positive/negative thoughts at thresholds.

## 9.4 Outdoors

-   [-] **Outdoors need** (BED-3: six places (room or not; open, roofed, thick rock), wiki rates and ladder -9..0, losses x0.2 in bed; missing: Indoorsy and Undergrounder trait modifiers)
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
-   [x] **Situational thoughts** (BED-3: slept cold and slept hot, -4 for a day against the pawn's comfort band, `rest/waking.go`)
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
-   [x] **Smoothed current mood**
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

-   [x] **Mental break eligibility** (SIM-066: minor/major/extreme tiers at mood 35/20/5 shifted by break-threshold traits, mean time between breaks 4/0.8/0.5 days, rolled hourly; wiki numbers, ENG-003)
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
-   [-] **Destructive/aggressive breaks** (SIM-066: insulting spree, targeted insulting spree, berserk, murderous rage; colonists may arrest or, drafted, attack a berserker. CBT-4: tantrum, Major tier, 8000-12000 ticks, smashes nearby objects; escape is 19.4)
    -   Insulting spree.
    -   tantrum/object destruction.
    -   berserk/aggression.
    -   prisoner-like escape behavior where relevant.
-   [x] **Catatonic/extreme shutdown equivalent** (SIM-066: catatonic breakdown downs the pawn for its duration; it is rescued and fed like any downed patient)
    -   Pawn becomes temporarily unavailable and requires care.
-   [x] **Recovery** (break ends after its duration with a day of catharsis +30)
    -   End state.
    -   mood catharsis/recovery thought if desired by independent
        balance.

## 10.2 Inspirations

-   [x] **High-mood inspiration chance** (PRD-2: mood >= 50 and Crafting or Construction >= 3, checked hourly, 1-day cooldown; mean-time curve is original tuning)
    -   Positive temporary state after sustained good mood.
-   [-] **Inspiration types** (PRD-2: creativity only; the other types are missing)
    -   Inspired creativity/quality.
    -   inspired recruitment.
    -   inspired trade.
    -   inspired taming.
    -   inspired surgery/work equivalents as supported.

## 10.3 Social interactions

-   [x] **Periodic social interaction** (SIM-041: every 60 ticks staggered among nearby same-room pawns; chitchat, deep talk, slight, insult, kind words; SIM-065: an awake bedridden patient is a partner, and friends visit the sick for shared recreation)
    -   Nearby awake pawns may chat, insult, compliment, etc.
    -   Sparse event-driven relationships, never all-pairs every tick.
-   [x] **Opinion** (SIM-041: directional, relation bonus + traits + beauty + memories, clamp +-100)
    -   Pawn A's opinion of Pawn B is directional.
    -   Influenced by relationship, traits, memories, social
        interactions.
-   [x] **Social fight** (SIM-043: from insults/slights, fists-only brawl, cathartic/angering opinions; insulting-spree break not done)
    -   Negative interaction can escalate to non-lethal melee conflict.

## 10.4 Relationships

-   [x] **Family relationships** (SIM-041: kin derived from parent links; starting families seeded)
    -   Parent/child.
    -   sibling.
    -   spouse/partner.
    -   other close-family links needed for social thoughts.
-   [x] **Romance** (SIM-042: attraction, attempts, lovers, rebuff memories; no manual Romance button by design)
    -   Attraction/compatibility.
    -   romance attempt.
    -   lover relationship.
    -   rejection memories.
-   [x] **Marriage** (SIM-042: proposal, engagement, rejection cooldown, wedding gathering/ceremony/reception, save-safe)
    -   Proposal/engagement.
    -   marriage ceremony/event.
    -   relationship/mood consequences.
-   [x] **Breakups** (SIM-042: breakup, divorce, affairs and ex memories)
    -   Relationship can end.
    -   social/mood memories.
-   [x] **Relationship death reactions** (SIM-041: grief via the relationship graph incl. off-map relatives, killer opinion; Cold-hearted exempt)
    -   Death of friend/lover/spouse/family creates stronger memories.

------------------------------------------------------------------------

# PHASE 11 --- Health, Anatomy, Damage, Disease, Medicine, and Death

RimWorld models injuries and conditions on individual body parts; damage
to specific parts changes capacities, and prosthetics/transplants can
replace missing functionality. Diseases, infection, pain, consciousness,
treatment, and surgery are major colony-management systems.
citeturn2search3turn2search11

## 11.1 Anatomy

-   [x] **Species anatomy tree** (SIM-039 humanlike tree, MED-05: 24 parts; MED-13: per-archetype animal plans (quadruped, bird, snake, lizard, insect) with their own parts, coverage and capacities, scaled by species)
    -   Body -\> torso/head -\> limbs -\> hands/feet -\> digits, organs,
        etc.
    -   Species can define different anatomy.
-   [x] **Body-part health** (SIM-039: per-part health; non-vital parts destroyed; vital damage spills to the downed/death thresholds)
    -   Each part has maximum health.
    -   Damage can destroy/remove a part.

## 11.2 Capacities

-   [x] **Pawn capacities** (SIM-039: consciousness, moving, manipulation, sight, hearing, talking, breathing; MED-05: blood pumping, blood filtration, digestion)
    -   Consciousness.
    -   Moving.
    -   Manipulation.
    -   Sight.
    -   Hearing.
    -   Talking.
    -   Breathing.
    -   blood filtration/circulation/metabolism equivalents as needed.
-   [x] **Capacity derivation** (SIM-039: feeds MoveSpeed/WorkSpeed/ShootAcc; MED-05: pumping weighs 0.2 on consciousness and moving and scales bleeding, filtration weighs on consciousness and immunity gain, spine multiplies moving; pumping, filtration, breathing or digestion at 0 is organ failure)
    -   Relevant body parts and health effects combine into functional
        capacity.
    -   Capacity influences work/combat/movement/social/medical
        performance.

## 11.3 Injuries

-   [x] **Damage types** (CBT-1: `damage.Hit` carries weapon, form and stopping power from the shot, never the attacker's current weapon; forms Blunt, Cut, Stab, Bullet, Arrow, Scratch, Bite, Crush, Burn, Flame, Bomb, EMP, Stun; Stab and Gunshot wounds bleed; wiki overkill windows destroy parts, brain and heart kill; SIM-039: cut/bruise/bite/scratch/burn/surgical wound kinds; no armor or explosion damage; DEF-1/DEF-2: armor resolution and bomb, flame and EMP explosion damage)
    -   Blunt.
    -   sharp.
    -   burn.
    -   explosion-derived damage.
    -   bite/scratch may map to sharp/blunt variants.
-   [x] **Wounds** (SIM-039: per-part wounds with severity, pain and tended flag; MED-01: per-wound bleed rate, fresh stumps bleed until tended)
    -   Injury assigned to body part.
    -   severity.
    -   bleeding rate.
    -   pain.
    -   tendability.
-   [x] **Bleeding** (MED-01: wiki bleed rates per wound kind, heart ×5, blood loss stages 15/30/45/60 % with consciousness penalties and exit hysteresis, regen 33 %/day while not bleeding, death at 100 %; raiders bleed for real, the old bleed-out timer is gone)
    -   Blood loss accumulation.
    -   severe blood loss -\> downing/death.
    -   tending reduces/stops bleeding.
-   [x] **Pain** (SIM-039: derived from wounds, feeds consciousness; MED-01: 1.25 % per HP open or tended, pain shock at 80 % scaled by pain tolerance)
    -   Derived from injuries/conditions.
    -   affects consciousness/mood/performance.
    -   extreme pain can down pawn.
-   [x] **Downed state** (MED-01: downed by consciousness < 30 %, moving < 15 % or pain shock, back up at 35 %/20 %/shock − 5 % (hysteresis); rescue and capture work (PLAY-010, SIM-045); rest 0 forces sleep instead of downing; SIM-063: every downed cause but collapse-asleep is rescued; downed pawns get hungry and doctors feed them (SIM-064); MED-12: animals go down at 80 % injury or severe blood loss and stand at 50 % / moderate (hysteresis), lie where they fell, are tended in place, can be finished by predators and hunters, and tame at double chance)
    -   Pawn cannot normally act.
    -   can be rescued/captured.
    -   not equivalent to death.
-   [x] **Death** (starvation, injury, infection, exposure; leaves a corpse and drops the weapon; SIM-041: grief and opinion memories through the relationship graph)
    -   Vital part destruction or lethal capacity/condition.
    -   create corpse.
    -   release jobs/reservations.
    -   trigger social/story events.

## 11.4 Healing

-   [x] **Natural wound healing** (MED-02: per wound at wiki rates: 8 HP/day, tended +4 + 0.8 % of tend quality, lying +4 ground / +8 bed / +14 hospital bed)
    -   Healing over time based on pawn state and care.
-   [x] **Scars/permanent injury** (SIM-039: disfigurement for face scars; MED-02: scar odds 2 % × clamp((damage − 4)/10) on cut/bite/scratch/burn)
    -   Some wounds heal into lasting impairment/pain.

## 11.5 Medical care

-   [x] **Medical care policy** (MED-03: per-patient level No care / Care, no medicine / Herbal or less / Industrial or less / Best available, caps the medicine used for tending and surgery; group defaults colonists Best, prisoners, guests and animals Herbal or less; self-tend toggle)
    -   No care.
    -   no medicine.
    -   herbal-equivalent.
    -   industrial-equivalent.
    -   best medicine-equivalent.
-   [x] **Tending** (doctor fetches herbal medicine when there is some; patients down, in bed or asleep; MED-02: tend quality from Medicine skill, manipulation, sight, medicine potency and cap, hospital bed +10 %, self-tend ×0.7, ±25 % roll; one tend per 12 h window)
    -   Doctor reserves patient.
    -   fetches medicine if policy allows.
    -   performs treatment.
    -   quality depends on skill, medicine, environment, capacities.
-   [x] **Self-tend** (only when no other doctor; weaker care)
    -   Optional with reduced effectiveness.
-   [x] **Medical beds** (NG-1: the hospital bed gives 115 % healing and half the infection chance, and rescuers prefer it; PLAY-020: any bed can be marked Medical, that bed only, with no owner; healthy colonists never sleep in it; patients and rescuers prefer it; hospital beds start medical)
    -   Mark beds for medical use.
    -   injured/sick pawns seek treatment/rest.
-   [x] **Bed rest** (PLAY-020: colonists in pain (healable injury 40+), with an untended wound, sick or waiting for surgery lie in bed until healed (15 or less, nothing to tend, at least 2 h), by default; a hungry patient eats and returns; SIM-064: a patient who cannot feed itself is fed by a doctor ("Feed <name>" too); no free bed retries after 6 h; self-tend in bed without another doctor; per-colonist "Rest when hurt"; no-flap tested)
-   [x] **Rescue** (PLAY-010 autonomous Doctor rescue; SIM-062 "Rescue <name>" order, drafted or not, Shift-queued; SIM-063: every downed cause but collapse-asleep, guests, allies and strangers too, a safe bed (no fire, survivable temperature; medical for a patient who needs care, else its own, else any free bed), the patient's own bed kept, a carrier downed or the patient dying drops it)
    -   Carry downed friendly pawn to valid bed.

## 11.6 Infection

-   [x] **Wound infection chance** (MED-02: rolled 6–18 h after the wound; bite/burn 30 %, other bleeding wounds 15 %, bruises 0; × tend factor 85 % → 5 % with quality; × 50 % in a clean roofed, floored room)
    -   Eligible untreated/poorly treated wounds can become infected.
-   [x] **Infection progression vs immunity** (MED-02: per-wound infection condition racing severity +84 %/day (tend cuts up to 53 %/day) against immunity +64 %/day, −70 %/day once immune; stages 33/78/87 %, death at 100 %; shared race engine for diseases; raiders and prisoners included)
    -   Disease severity progresses.
    -   immunity grows based on treatment/rest/health.
    -   reaching immunity first allows recovery; severity first can
        kill.

## 11.7 Diseases

-   [x] **Disease framework** (MED-04: disease conditions on the shared severity-vs-immunity race engine (MED-02), stage capacity/pain/need modifiers, tend windows, cure-by-tending for parasites, bed rest at the major stage until immune)
    -   Severity over time.
    -   stages.
    -   symptoms/stat modifiers.
    -   tendable conditions.
    -   immunity where appropriate.
-   [-] **Representative vanilla disease classes** (MED-04: flu, plague, malaria, sleeping sickness, gut worms, muscle parasites at wiki rates; food poisoning (FOOD-4) and toxic buildup (FOOD-6, `body/cond_toxic.go`) are conditions of their own)
    -   flu-like disease.
    -   plague-like severe disease.
    -   malaria-like tropical disease.
    -   sleeping-sickness-like disease.
    -   food poisoning.
    -   heatstroke.
    -   hypothermia.
    -   toxic buildup.

## 11.8 Age/chronic conditions

-   [-] **Biological aging** (MED-08: age drives chronic onset after wiki onset ages, rolled daily per colonist; no separate biological vs chronological age or age-based body decline beyond the conditions)
    -   Age increases.
    -   age can influence health/stat factors.
-   [x] **Age-related chronic conditions** (MED-08: bad back, cataract, frailty, dementia (confused wandering), hearing loss, artery blockage with heart attacks, carcinoma with excision; cures by spinal brace, eye/ear replacement, artificial or new heart)
    -   Bad back/frailty/cataract-like conditions.
    -   Data-driven health effects.

## 11.9 Surgery

-   [x] **Operation bills** (SIM-039: install/remove implant, amputate; queue add/remove in the Health tab; MED-03 reorder; MED-06: harvest organ, install organ, euthanize, anesthetize; colonists, prisoners and guests)
    -   Add/remove scheduled medical operations.
-   [x] **Anesthesia** (MED-06: anesthetic condition, sedated 0–6 h, woozy to 12 h, wearing off to 20 h)
    -   Required for appropriate surgery.
-   [x] **Surgery success/failure** (MED-06: wiki base by Medicine level × manipulation × sight × bed × room cleanliness × light × vitals monitor × medicine × operation factor, capped at 98 %; minor, catastrophic and ridiculous failures; death on failed heart installs; botched opinion memory (SIM-039); every operation consumes medicine within the care level)
    -   Doctor skill.
    -   medicine.
    -   manipulation/sight.
    -   room cleanliness/bed factors.
    -   failure can injure nearby/body parts.
-   [x] **Organ extraction/transplant** (MED-06: harvest drops the organ item (a life organ kills), install organ into a missing or damaged inner part, donor and colony mood, faction goodwill; MED-08: carcinoma excision)
    -   Removable organs.
    -   harvested organ becomes item.
    -   missing vital organs can kill.
    -   transplant restores/replaces functionality.
-   [x] **Prosthetics/bionics** (SIM-039/SIM-049: peg leg, prosthetic leg/arm/foot, wooden hand, denture, hearing trumpet, glass eye, wooden nose; MED-05: servo arm/leg, optic and cochlear implants, pump heart and synth stomach at 125 %, spinal brace, valve heart 80 %, wiki efficiencies for the prosthetic arm (50 %) and wooden hand (60 %), Servo replacements research; archotech tier deferred)
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
-   [x] **Heat sources/sinks** (wood campfire heats its room, banks at 20C and relights below 16C; haulers refuel it; PLAY-021: powered heater and in-wall cooler with a thermostat (21 C by default, hysteresis))
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
-   [x] **Growing period** (the landing screen shows the growing period, `wizard/screen_landing.go`)
    -   World tile communicates expected crop-growing season.

## 12.3 Weather

-   [-] **Weather state machine** (deterministic 6-hour blocks: clear, cloudy, rain, storm, snow, blizzard by season and temperature; wind; snow depth and wetness; no fog kind)
    -   Clear.
    -   rain.
    -   fog.
    -   snow where climate supports it.
    -   thunderstorm/flashstorm-like weather.
-   [-] **Weather effects** (outdoor chill in snow, outdoor campfires doused, slower walking in rain and snow, soaked/snow-day thoughts; PLAY-015 SIM-057: snow lies per tile (none under roofs) and slows walkers and grass per cell; a snow clearing area is shovelled under Cleaning; light only darkens visually)
    -   light.
    -   temperature.
    -   fire suppression.
    -   movement/accuracy where appropriate.
    -   solar generation.

## 12.4 Lighting

-   [-] **Day/night light** (outdoor level is a pure function of tick; indoor/roofed and artificial lights not yet) (ART-010, SIM-018: sun/moon, indoor and lamp light; PLAY-020: lamps are point lights with falloff and direction, inside their room or, outdoors, in their line of sight; no light gameplay effects)
-   [-] **Roof/indoor darkness** (ART-010, SIM-018: indoor ambient under roofs; MAPGEN-3: roofed cells are dark to plants unless lit; no work/mood effects)
-   [x] **Artificial lights**
-   [x] **Light affects plants** (MAPGEN-3: simulation cell light from daylight, lamps, campfires and glowing plants; moss needs light, duskmoss grows in the dark and withers in light; PLAY-017: every other plant, grass included, grows only with sun on unroofed cells or a strong lamp; campfires light only while burning)
-   [-] **Light can affect work/mood/accuracy where appropriate** (PRD-3: work part done, bench work speed 0.8 at 0 % light, wiki, ramp up to 30 % original; mood and accuracy parts still open)

## 12.5 Fire

-   [x] **Flammability** (PLAY-015 SIM-058: grass, plants (trees, duskmoss), crops, wooden buildings and bridges, flammable items; stone, steel and terrain never burn)
    -   Terrain/buildings/items/plants define flammability.
-   [-] **Ignition** (PLAY-015: lightning strikes in storms, bursting animals, developer action; no weapon or overheat sources yet; DEF-2: flame explosions now ignite; still no overheat or weapon-fire source)
    -   Weapon/explosion/lightning/random event/overheat sources.
-   [x] **Fire spread** (PLAY-015: active fires in per-chunk lists; spreads to flammable neighbours, downwind more, barely into roofed cells; rain and snow put outdoor fires out)
    -   Active-set simulation only.
    -   nearby flammable objects.
    -   weather/rain influences.
-   [x] **Fire damage** (PLAY-015: buildings (bridge collapse cascade), plants, grass, items destroyed, pawns take burn wounds, ash left behind; fires heat rooms and warm the air)
    -   Buildings/items/plants/pawns take burn damage.
-   [x] **Firefighting work** (PLAY-015: Firefight work type on by default; fires inside Home and the pawn's area; drafted Extinguish order anywhere; pawns and animals avoid and flee flames)
    -   Pawns extinguish fire in home area/urgent contexts.
-   [ ] **Automatic rain after major fire**
    -   Optional vanilla-equivalent storyteller/environment behavior.

## 12.6 Environmental events

-   [-] **Heat wave** (NG-1: colonies from the wizard with the Extreme weather setting on; +12 C for 2 to 4 days in summer, 6 % a day, 3-day cooldown; letter and top-strip chip)
-   [-] **Cold snap** (NG-1: same, -14 C in winter)
-   [x] **Eclipse** (FOOD-6: incident `eclipse`, wiki weight and 15-day cooldown, 0.75-1.25 days; sky light 0: crops outdoors stop, solar panels give 0; caution letter; WorldView.SkyEvent)
    -   Reduces sunlight and solar generation; affects crops.
-   [x] **Solar flare** (PLAY-021: every net goes down for 3-8 h, generators idle; letters at start and end, a status chip)
    -   Temporarily disables relevant electrical devices.
-   [x] **Toxic fallout** (FOOD-6: incident `toxic_fallout`, from day 60, 90-day cooldown, 2.5-10.5 days; open-sky plants die (57.5 damage an hour), unroofed pawns gain toxic buildup 40 %/day and animals 20 %, with wiki stages, vomiting, dementia and carcinoma; roofs shelter; -5 thought outdoors; a body dying with buildup rots at once with a chance equal to its severity)
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
-   [x] **Outdoor deterioration** (PRD-2: roof-aware, raw resources exempt)
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

-   [x] **Workstation** (workbench and stove: 2x1 rotatable footprints with interaction cell, block movement; PLAY-021: electric stove, tailoring and research benches work 40% faster and stop without power; PRD-3: 24 bench and building kinds with wiki footprints, wiki watts on the powered benches, recipes shared across benches (`recipe.Def.Also`, `RecipeView.RunsAt`), facilities (tool cabinet +6 %, `craft/facility.go`), work speed from bench factor, light, roof and cleanliness (`craft/station.go`, `BuildingView.Linked`/`WorkSpeed`), wood-fired forge fuel, 16 new research projects)
    -   Supported recipes.
    -   interaction cell.
    -   power requirement where applicable.
    -   facility/environment modifiers.
-   [-] **Recipe definition** (inputs incl. any-raw-food, any-veg, any-meat, smelt-wear and wreck inputs, work, work type, product, research gates; 31 PRD-4 recipes; missing: bill quality and hit-point ranges)
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
-   [x] **Ingredient radius** (PRD-1: per bill, tiles from the bench; 0 unlimited)
    -   Limit ingredient search area.
-   [x] **Ingredient filters** (PRD-1: per-bill allow/forbid of each ingredient kind the recipe accepts)
    -   Restrict materials/categories.
-   [x] **Worker restrictions** (PRD-1: skill range and a specific colonist)
    -   Skill range/specific pawn if desired for parity.
-   [x] **Product destination** (PRD-1: best stockpile, drop on floor, a specific stockpile)
    -   Drop on floor.
    -   take to best stockpile.
    -   specific storage destination where supported.

## 14.3 Crafting

-   [x] **Stone cutting** (SIM-029: stonecutter table, one chunk of any of five stones cuts into 10 blocks of it)
-   [x] **Butchering**
-   [x] **Cooking** (stove: 2 raw food -> 1 simple meal)
-   [x] **Tailoring/apparel production** (tailoring bench sews leather parkas, tuques and dusters; butchering yields leather)
-   [x] **Smithing/weapon production** (workbench: 4 wood -> wooden club, 3 stone -> knife, 5 wood -> spear; PRD-3: smithy blades and pila, crafting-spot bows, 13 machining-table guns, smelting metal weapons back to steel; `domain/economy/recipe/benches.go`)
-   [x] **Machining/advanced fabrication equivalents** (NG-1 components; PRD-3 machining table with 13 guns; PRD-4: fabrication bench makes advanced components and marine armor and helmet, machining table makes flak armor, grenades and four mortar shells, smithy plate armor, smelter smelts apparel, shred mechanoid; 8 research projects)
-   [x] **Drug production** (SIM-040 beer, tea, tablets; PRD-4: wort at the brewing bench, fermenting barrel (25 wort, 6 days, 7-32 C, wiki), ash flake, silverdust, battle tonic and reagent at the drug table, chemfuel at the refinery; missing: the barrel is filled by cooks, wiki haulers)
-   [x] **Component production in late tech** (NG-1: Fabricate component, 12 steel into 1 at the workbench with Crafting 6; components also come from buried machinery ore)

## 14.4 Quality

-   [x] **Craft quality generation** (PRD-2: skill roll via domain/economy/quality; inspiration shifts +2; value and stat multipliers read the stored quality)
    -   Skill/inspiration influence.
    -   Quality affects market value and relevant item stats.

## 14.5 Art

-   [x] **Artistic work** (PRD-3: art bench carves small, large and grand sculptures; quality rolled from the worker's skill and inspiration (PRD-2); beauty 50/100/400 wiki times the quality factor, read by the beauty need and room stats; maker named in the generated text; `craft/art.go`)
    -   Sculptures/art objects.
    -   quality.
    -   beauty.
    -   creator attribution.
-   [-] **Art descriptions** (PRD-3: original generated title (`BuildingView.Epitaph`) and description from a seeded grammar (`craft/artgen.go`, `ArtDescription`), shown in the completion message; missing: description not in the inspector, no minified item)
    -   Generate original procedural descriptions from structured
        events/grammar.
    -   Do not copy RimWorld text.

------------------------------------------------------------------------

# PHASE 15 --- Power and Electrical Infrastructure

## 15.1 Power network

-   [x] **Power producers** (PLAY-021: wood-fired generator 800 W while fuelled and on; solar up to 900 W by daylight and its unroofed share; wind up to 1100 W by wind speed and a clear band; geothermal 1800 W on a steam vent; our numbers)
    -   Fuel-based generator.
    -   solar.
    -   wind.
    -   geothermal-like late generator.
-   [x] **Power consumers** (PLAY-021: lamps 50 W, heater 180 W / cooler 220 W with a thermostat, electric stove, tailoring and research benches; PLAY-024: sentry turret 120 W firing / 40 W idle (defense class, shed last), radio console 150 W)
    -   Workstations.
    -   lights.
    -   coolers/heaters.
    -   communications.
    -   defensive/utility equipment where applicable.
-   [x] **Power conduits/network** (PLAY-021: conduits and hidden conduits on their own layer link 4-neighbour nets, devices join a transmitter within 3 tiles, an open switch cuts; nets rebuilt only on structure or switch change)
    -   Connected graph.
    -   Topology recalculated only when changed.
-   [x] **Power availability** (PLAY-021: every 50 ticks each net balances; batteries cover a deficit, then consumers are shed by class with a minimum off time and headroom (no-flap tested))
    -   Producers feed network.
    -   consumers activate only with sufficient supply.

## 15.2 Batteries

-   [x] **Energy storage** (PLAY-021: battery 1200 Wh, 75% charge efficiency, slow self-discharge)
    -   Charge/discharge.
    -   capacity.
-   [x] **Battery discharge event** (PLAY-021: a short circuit drains a charged net, blows one visible conduit and starts a fire sized by the energy)
    -   Short-circuit/Zzztt-like event can release stored energy and
        start fire.
    -   Implement independently without copying exact balance.

## 15.3 Variable generation

-   [-] **Solar day/weather output** (PLAY-021: follows daylight and roofs; PLAY-022: weather dims it (shared sun factor with the renderer: cloudy 70 %, rain 45 %, storm 35 %, snow 40 %, blizzard 25 %); FOOD-6: an eclipse stops it)
-   [x] **Wind turbine output** (PLAY-021: wind speed times the clear share of a band beside it; walls, rock, roofs and trees block)
    -   Wind state and obstruction rules.
-   [x] **Fuel consumption** (PLAY-021: one fuel table for campfires, torch lamps and wood-fired generators, refuelled by haulers; a generator burns only while running)
    -   Refuel jobs for generators/devices.

------------------------------------------------------------------------

# PHASE 16 --- Apparel, Equipment, Weapons, and Loadout Policies

## 16.1 Equipment

-   [x] **Single equipped primary weapon** (CBT-2: `DropWeapon` drops or stows into a spare slot, `domain/pawn/inventory`; CBT-3: `combat.Gear` keeps quality, stuff and wear, saved in the Turrets block, drops and spares keep the make, old saves load as Normal steel)
    -   Equip/drop.
    -   weapon stats.
    -   melee fallback for ranged weapons.
-   [x] **Equipment jobs**
    -   Player order to equip.
    -   dropped weapon can be hauled/stored.

## 16.2 Apparel

-   [x] **Wearable apparel** (body and head slots over everyday clothes; insulation shifts the comfort band; Wear order; Gear tab; PRD-2/DEF-2: stuff, quality, hit points and armor per piece; four slots incl. legs; gear tab shows them)
    -   Body-part/layer coverage.
    -   material.
    -   armor.
    -   insulation.
    -   quality.
    -   hit points.
-   [ ] **Apparel compatibility**
    -   Conflicting body layers/slots cannot be worn together.
-   [x] **Apparel deterioration** (PRD-2: worn pieces lose 1 HP on 40 % of days; ratty at 50 % HP, tattered under 20 %)
    -   Worn apparel can lose condition.
-   [-] **Apparel policy** (PLAY-017: colonists change straight into the best body and head outfit for their comfortable range, before work and by dropping interruptible work, with a 1 C dead band and a per-slot cooldown; a Wear order keeps its slot; changes logged on the colonist; no player policy screen)
    -   Allowed categories.
    -   minimum condition.
    -   quality filters.
    -   pawn automatically changes apparel when safe/appropriate.
-   [x] **Tainted apparel** (PRD-2: dead pawn outfit is tainted and dropped; -5/-8/-11/-14 by count; auto-dress skips it)
    -   Apparel from a corpse can carry negative social/mood semantics
        for ordinary colonists.

## 16.3 Weapon categories

-   [x] **Melee weapons**
    -   Multiple attack verbs/damage profiles.
-   [x] **Ranged weapons** (DEF-1: per-weapon warmup, cooldown, burst, accuracy bands and armor penetration; projectiles in flight; the rifle is the bolt-action; missing: the rest of the vanilla catalogue incl. burst weapons (DEF-2); DEF-2: bows, revolver, autopistol, assault and sniper rifles, shotguns, LMG, minigun; 30 weapon and armor item kinds; rocket launchers skipped)
    -   Range.
    -   warmup.
    -   cooldown.
    -   accuracy by distance.
    -   burst count where applicable.
    -   projectile definition.
-   [-] **Thrown/explosive weapons** (DEF-2: frag, molotov and EMP grenades with blast radius, raiders only; colonist throw order is DEF-4)
    -   Grenade-like projectile.
    -   blast radius.
-   [x] **Single-use launchers** (CBT-3: rocket launcher and triple rocket launcher are consumed by one trigger pull; wiki numbers)
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
-   [x] **Auto-undraft** (CBT-2: released after 10,000 quiet ticks, re-checked before release; wiki: Drafting)
-   [x] **Hold/fire-at-will behavior** (DEF-4: fire at will off stops a drafted colonist starting attacks of its own; hold position shoots from its tile and never chases; `combat/draft.go`)
-   [x] **Drafted field tending** (MED-03: drafted "Tend <name>" order; the doctor brings the best allowed medicine within 15 tiles, or none)
    -   Allow emergency tending under appropriate conditions.

## 17.2 Line of sight

-   [x] **Projectile LOS**
    -   Walls/solid obstacles block.
    -   Adjacent wall leaning/corner firing equivalent where needed for
        familiar tactical behavior.

## 17.3 Cover

-   [x] **Cover objects** (sandbags 55%, wall/door 75%, barricade 55%, turrets 40/50; DEF-3: trees, rock, cacti, bushes and stone chunks too, `domain/combat/cover`)
    -   Walls, barricade/sandbag equivalents, chunks, trees, etc.
    -   Cover affects incoming ranged hit chance.
-   [x] **Cover calculation** (CBT-2: angle bands, 33/67 % at close range, combined covers; best cover on the cells beside the target facing the shooter; ranged shots only; DEF-3: permille cover from buildings and natural objects in `Sight`)
    -   Relative shooter-target-cover geometry.
    -   Stable deterministic resolution.

## 17.4 Ranged attack

-   [ ] **Target acquisition**
-   [x] **Warmup** (DEF-1: per weapon, `weapon.Def.Warmup`)
-   [x] **Accuracy** (CBT-2: Euclidean ranges, weather factors unless both ends are roofed)
    -   Shooting skill.
    -   weapon.
    -   distance.
    -   target size.
    -   weather/light if modeled.
    -   cover.
-   [x] **Projectile flight** (DEF-1: shots fly; walls stop them; creatures on the path may intercept; a missed shot can hit others, friendly fire beyond 5 tiles)
    -   Logical projectile entity or deterministic trajectory.
    -   Can miss intended target and impact another valid cell/entity
        where design supports it.
-   [x] **Burst fire** (DEF-1: the mechanism, shots and ticks between from the weapon; no burst weapon until DEF-2; DEF-2: burst weapons use it)
-   [x] **Cooldown** (per weapon, `weapon.Def.Cooldown`)

## 17.5 Melee

-   [x] **Adjacent melee combat** (CBT-2: humans adjacent strike with a gun's stock, grip or limb)
    -   Pawns next to hostile target use melee even if holding ranged
        weapon, matching vanilla's core behavior. citeturn2search1
-   [x] **Melee hit chance** (CBT-2: wiki hit curve, wiki dodge, damage x0.8-1.2)
-   [x] **Weapon/body attacks** (CBT-2: wiki tools per melee weapon; humans fist, head, teeth; animals a second attack at original tuning)
-   [x] **Melee cooldown**

## 17.6 Armor

-   [x] **Armor coverage** (CBT-1: the body part is picked first, then armor from every garment covering its group, outermost first, with garment HP loss; replaces the DEF-1 region roll)
    -   Apparel covers body parts.
-   [x] **Armor categories** (DEF-1: sharp, blunt, heat; damage types map to them)
    -   Sharp/blunt/heat-style protection.
-   [x] **Armor resolution** (DEF-1: rating minus penetration; deflect below half, halve below the full value, halved sharp becomes blunt; wiki numbers)
    -   Attack penetration vs armor.
    -   Can reduce/deflect damage.
    -   Damage ultimately applies to selected anatomy.

## 17.7 Explosions

-   [x] **Blast cells** (DEF-2: domain/combat/explosion, line of sight from centre)
-   [x] **Radial damage** (DEF-2: full at centre to half at rim (original tuning))
-   [x] **Fire chance** (CBT-5: flame ignites every cell in the radius by flammability, a creature's cell 70 % (original), the centre always)
-   [x] **Building/item/pawn damage** (CBT-5: bomb x2 passable buildings, x4 impassable and plants, x8 walls (wiki); bombs wear weapons and apparel and destroy them at 0 HP; burnable raw stacks destroyed by a hit of 50 or more, stone and metal immune (original thresholds); explosions do not yet count toward kill and damage records)
-   [x] **Chain reaction support for explosive content** (DEF-4: a blast lights the IEDs in its radius, a bullet landing on one sets it off; CBT-5: shells and grenades in a bomb or flame blast or lying in fire go off 30 ticks later via the IED fuse queue, `Fuse.Stack` saved, at most 8 a tick, each stack once (original tuning))

## 17.8 Combat AI

-   [x] **Hostile target selection** (CBT-4: threat-weighted choice; DEF-4: raiders take a colonist or ally, a powered turret in sight (also when no colonist is reachable), a rival faction's raider closer, then the walls and doors in the way; no threat weighting yet)
-   [x] **Approach/ranged engagement**
-   [x] **Melee closing**
-   [x] **Basic cover seeking** (CBT-4: raiders and threat-responding colonists take cover and hold their spot, with hysteresis)
-   [x] **Undrafted threat response** (CBT-4: Flee, Attack or Ignore per colonist, `SetThreatResponse`, `PawnView.ThreatResponse`)
-   [x] **Grenade avoidance** (CBT-4: raiders step out of a thrown grenade's blast)
-   [x] **Mechanoid doors and pods** (CBT-4: mechs bash walls, shut doors, arrive by drop pod and never retreat (wiki); EMP adaptation shared with turrets; mech voices are blocked on the creature skill's four-sound rule)
-   [x] **Shield belt** (CBT-3: `game/hit` absorbs before armor; wiki energy, recharge and 3200-tick break; EMP breaks it; the wearer cannot shoot or throw)
-   [x] **Smokepop pack** (CBT-3: 3 uses, `PopSmokepop`; raiders pop it when shot)
-   [-] **Weapon catalogue extras** (CBT-3: smoke launcher, charge rifle, charge lance, ikwa with wiki numbers; missing: uranium for slug turrets and reinforced barrels for mortars have no source, so steel stands in)
-   [x] **Flee behavior for appropriate human enemies** (raiders flee at half HP, once the group has lost its rolled 40-70 % (wiki), an idle hour or a day on the map; a staged or besieging raid's day starts at its storm)

------------------------------------------------------------------------

# PHASE 18 --- Animals, Wildlife, Hunting, Taming, Training, and Pens

Base-game animal simulation should include wild populations, hunting,
taming, training, animal products, predators, manhunter behavior, and
domestic management. Where modern base-game RimWorld uses pens for farm
animals, support the equivalent without pulling in DLC-only genetics or
children systems.

## 18.1 Species definitions

-   [x] **Animal species** (SIM-019: 61 species from size/behaviour classes; ten originals replace RimWorld-invented creatures; DEF-4 R2: four original mechanoids appended, 65 in all, with full art; their four voices each are blocked on the user, the insect group stands in [!])
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
-   [x] **Animal hunger/rest** (SIM-024: every animal has Food and Rest; states Wander/Feed/Rest/Hunt/Flee/Leave with hysteresis; herds bed down together, night and day sleepers, starvation; FOOD-1: daily need on the nutrition scale by size (cow 0.86, alpaca 0.44 a day), babies x0.4 and juveniles x0.75, products fill 100/50/25/0 % by saturation)
    -   Simpler needs than humans.
-   [x] **Grazing/foraging** (SIM-024: grazing grass, browsing, crop raiding, carcass eating, timbergnaw tree felling; feeding troughs in pens; SIM-038: grazers prefer fuller grass, meals scale with growth; PLAY-017: tame animals eat the nearest food their diet allows, stockpiles included, and tame predators hunt wild prey and eat it; FEED-001: the wiki diet table (kibble and pemmican for all but strict carnivores, meals for all but strict carnivores and tree eaters), animal feed before meals, only reachable food with a memory of unreachable targets, a whole-area search while starving (15/25 % band), kibble in troughs; FOOD-4: per-animal food policy, Animal feed default)
-   [x] **Predation** (SIM-019: predators hunt prey no bigger than themselves; small-prey predators XS/S only; packs share targets)
    -   Predators select prey and hunt.
-   [x] **Wild animal flee/fight response** (flee within 5 tiles and run until clear of 9; enraged animals don't flee; PLAY-010 SIM-035: every animal hit by an attacker fights back or flees, one that can't fight back flees on every hit (penned ones inside the pen), predators may charge colonists within 3 tiles unless asleep (off on Wanderer/Gentle); SIM-036: unarmed undrafted colonists flee from an animal coming for them within 12 tiles, even mid-order or asleep; armed ones keep their target; PLAY-017: an animal starting to chase a colonist posts a danger message (once per chase) that auto-pauses the game)

## 18.3 Hunting

-   [x] **Hunt designation**
-   [x] **Hunter requires suitable ranged weapon**
-   [ ] **Stalk/approach/fire job**
-   [-] **Animal retaliation/manhunter chance** (SIM-019: revenge chance by behaviour class, legendary 100% and rallies its kind; SIM-024: badly hurt fighters break off, predators give up chases; SIM-035: tame fighters defend themselves against anything but their own colonists)
-   [x] **Corpse hauling/butchering integration**

## 18.4 Taming

-   [x] **Tame designation** (Orders > Tame or the animal inspector; hunters tame)
-   [-] **Animal-handling interaction** (SIM-026: taming moved from Hunt to the Handle work type, also used for Milk/Shear; no separate trust/nuzzle interaction)
-   [x] **Food requirement** (FOOD-3: taming costs food)
-   [x] **Success chance based on skill/wildness** (FOOD-3: wildness, skill and a minimum skill; wiki Taming numbers)
-   [x] **Tame animal joins colony** (named; chance per try by species (deer 60%, timber wolf 25%); a botched try may enrage it; tame young are named at birth)

## 18.5 Training

-   [ ] **Trainability**
-   [x] **Obedience** (SIM-047: training steps, cooldown, master follow when drafted)
-   [x] **Release/attack training** (SIM-047/048: obedience/guard/release training; a guard defends its master against hostiles within 5 tiles or whoever attacked the master in the last 120 ticks; a release-trained animal attacks the drafted master's target; tame wolves guard against raiders and hostile animals untrained)
-   [x] **Rescue**
-   [ ] **Hauling**
    -   Only species capable of relevant training.
-   [x] **Training decay/maintenance** (SIM-047: skills decay)
    -   Skills can require reinforcement if matching target behavior.

## 18.6 Animal zones/pens

-   [x] **Animal allowed areas for zonable animals** (PLAY-015: tame non-penned animals use allowed areas; Animals tab dropdown)
-   [x] **Pen system for appropriate livestock** (PLAY-015 SIM-056: pen markers whose flood-filled enclosure (fences, gates, walls, doors, rock) makes the pen; 29 penned species; gates and doors hold livestock unless a handler ropes and leads it; species/sex filters, grazing vs demand, auto-cut; open-pen and unpenned alerts; unpenned livestock roam and may wander off; painted pens convert on load; SIM-024 troughs; SIM-028 fences and gates)
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

-   [x] **Mating/pregnancy** (SIM-026: daily 35% mating check, pregnancy and litters, ageing, juveniles follow the mother, old-age death; FOOD-6: wiki gestation, litter range, juvenile and maturity ages and life expectancy per species, `animal/breeding.go`)
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

-   [x] **Capture downed hostile** (SIM-045)
    -   Carry to prisoner-designated bed/room.
-   [-] **Prisoner state** (SIM-045: room-bound; SIM-063: a warden carries a downed prisoner back to a prison bed, "Take <name> to bed" too; no escape behaviour)
    -   Faction remains distinct until recruited.
    -   Restricted movement/escape behavior.

## 19.2 Prisoner needs

-   [x] **Food/rest/health/mood** (SIM-045: Imprisoned mood)
    -   Prisoners remain simulated pawns with reduced player control.
-   [x] **Prisoner feeding** (SIM-045: warden delivery with hysteresis)
-   [x] **Prisoner medical care** (SIM-045: doctors tend in place)

## 19.3 Warden interactions

-   [x] **Prisoner interaction mode** (SIM-045: reduce resistance, recruit, release, execute)
    -   No interaction.
    -   reduce resistance.
    -   recruit.
    -   release.
    -   execute/medical operations if allowed by game rules.
-   [x] **Resistance/recruitment difficulty** (SIM-045: opinion and mood factors x social impact)
    -   Warden social skill and prisoner mood influence progress.
-   [x] **Recruitment** (SIM-045: keeps relations, recruited-me opinion)
    -   Successful recruit joins player's faction.

## 19.4 Prison breaks

-   [x] **Escape attempt** (CBT-4: rooms join per the wiki rule; timing original: 60-day mean time between breaks, 3-day grace, 10-day calm; escaped prisoners fight their way out)
    -   Prisoner(s) can attempt to break out based on conditions.

## 19.5 Other recruitment

-   [-] **Rescue/recovery join chance** (SIM-063: a rescued faction-less stranger asks to join on recovery, 50 % when leaving is safe, 100 % when not; Accept/Decline letter. Missing: an event that brings such strangers (transport pod crash))
-   [x] **Wanderer/migrant joins event** (SIM-046)
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

-   [-] **Faction types** (NG-1 SIM-030: settlers, clans, drifters, freebooters, swarm and the player; mechanoids act as a hostile raid source (DEF-4 R2) but are not a faction object; no ancient factions)
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
-   [-] **Goodwill changes** (NG-1: a repelled raid costs 5 and goodwill drifts toward a baseline daily; PLAY-024: gifts by radio (1 per 40 silver, at most 15), caravan requests cost 12, killing or capturing a trader costs 40, military aid costs 20 (friendly only, 3-day cooldown); no quests)
    -   Gifts.
    -   attacks.
    -   harming visitors.
    -   quest outcomes.
    -   assistance.
    -   settlement proximity if included.
-   [x] **Permanent hostility** (NG-1: freebooters and the swarm are fixed at -100)
    -   Some factions cannot become friendly.

## 20.3 Visitors

-   [x] **Friendly visitor group** (SIM-046: friendly/neutral factions wander, socialise, leave)
    -   Enters map.
    -   wanders/stays.
    -   leaves.
    -   reacts to danger.

## 20.4 Trade caravans

-   [x] **Trader arrival** (PLAY-024: trader caravan incident from friendly or neutral human factions, a trader with 1–3 guards staying 0.75 day; requestable by radio)
    -   Group with trader pawn and pack animals/inventory.
-   [x] **Trade interaction** (PLAY-024: "Trade with X" order or the radio console opens a session held by the negotiator's job)
    -   Select social-capable colonist.
    -   Open trade screen.
-   [x] **Buy/sell** (PLAY-024: settled against live counts; caravan goods dropped beside the negotiator, orbital goods by drop pod near the beacon)
    -   Colony stored eligible items available.
    -   Trader inventory and silver/currency.
    -   Purchased goods delivered/dropped.
-   [x] **Trade price** (PLAY-024: market value with a buy markup and sell cut falling with Social skill and goodwill; sell never above 90 % of buy; traders pay more for their specialty)
    -   Social skill and faction/trader factors influence price.
-   [x] **Trader types** (PLAY-024: bulk hauler, quartermaster (caravans), curio merchant (orbital); neolithic traders carry no rifles or components)
    -   Different inventories such as bulk goods, combat/exotic, slave
        trader if included for historical vanilla parity, etc.
    -   Use original names/content.

## 20.5 Orbital trade

-   [x] **Communications console** (PLAY-024: powered radio console behind Radio relay research: faction list, gifts, caravan requests, call trade ship, military aid)
-   [x] **Orbital trade beacon** (PLAY-024: goods within 6 tiles are tradable with ships; purchases land by drop pod)
    -   Defines tradeable nearby storage.
-   [x] **Orbital trader event** (PLAY-024: a ship in radio range for 1.5 days when a powered console exists; letter and alert)
-   [x] **Remote faction communication** (PLAY-024: gifts, caravan requests and military aid by radio (ENG-007))
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

-   [x] **Electricity** (PLAY-021: unlocks conduits, switch, wood-fired generator, lamp, heater, cooler and the electric benches)
-   [x] **Battery/storage** (PLAY-021: the Batteries project; plus Solar, Wind and Geothermal power)
-   [ ] **Cooling/temperature control**
-   [ ] **Stoneworking**
-   [ ] **Complex furniture**
-   [x] **Drug production** (SIM-040 research; PRD-4: TonicSynthesis, LeafRefining and the hard-drug chain)
-   [ ] **Microelectronics/communications**
-   [x] **Advanced fabrication** (PRD-4: PrecisionFabrication, FlakArmor, Armoursmithing, PoweredArmor, TrailRations, SealedRations; gates on Upholstery, Metalworking, RifledBarrels, Microcircuits, Artillery)
-   [ ] **Weapon technologies**
-   [ ] **Armor/apparel technologies**
-   [x] **Geothermal/advanced power** (MAPGEN-2: steam vents warm the air; PLAY-021: a geothermal generator stands exactly on a vent, 1800 W)
-   [x] **Hospital/medical technology** (hospital bed (SIM-034); MED-06: powered vitals monitor beside a hospital bed: tend +7 %, surgery ×1.05, immunity +2 %; MED-05: servo implants, nanomed kit)
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
-   [x] **Mountainous** (MAPGEN-2: a mountain mass leaning toward the uphill world edge, tapered branching caves)

## 22.4 Local map generation

-   [x] **Terrain from biome/topology** (MAPGEN-2 SIM-050: staged field generator; warped elevation/fertility/wetness fields, terrain by precedence, data-driven biome patches, patch cleanup; legacy generator frozen for old maps)
-   [x] **Rock/mountain mass** (MAPGEN-2 SIM-050: rock share by hilliness quantile, coherent mountains with irregular edges and component cleanup, broad stone regions, thin and thick natural roofs (thick: permanent, no drop pods), caves only in large rock bodies)
-   [x] **Ore/resource deposits** (SIM-029; MAPGEN-2: compact irregular lumps deep in rock with spaced origins; stone debris in small formations near cliffs)
-   [x] **Water** (NG-1 SIM-033; MAPGEN-2: ragged coasts with depth bands and beaches, meandering rivers with varying width, banks, fertile floodplains, canyons and fords, lakes, wetness field; roads A*-routed and smoothed; MAPGEN-3: roads cross rivers on straight wooden bridges and are re-routed if a repair breaks them; no fishing)
-   [x] **Wild vegetation** (MAPGEN-3: duskmoss clusters on dark cave floor, moss at lit cave mouths; MAPGEN-2: groves and clearings, local saturation, species clusters and balance, spacing, varied growth; trees, berry bushes and seven new ground plants (SIM-038); grass is simulated ground cover removed by building, sowing and Cut plants (UI-011) and regrows from shoots over three days after grazing or cutting)
-   [x] **Wildlife** (SIM-023/025: per-biome density, species weights, sex and age at spawn, carrying capacity; MAPGEN-2: herds start in reachable habitat away from the colony, arrivals walk in from a reachable edge)
-   [x] **Ancient ruins/debris** (MAPGEN-2: small surface ruins, stone debris, steam vents; MAPGEN-3: sealed vaults cut into rock with intact walls, a tiled floor, loot, dormant hive defenders scaled by threat (none when peaceful), a one-time "Ominous walls" warning, and grouped sleeper caskets whose occupants may join, attack, or be empty)
-   [x] **Starting clear/landing area** (MAPGEN-2: the landing spot is scored (open land, fertility, stone, roads, centre) and repaired locally, ≥ 70% of land reachable; drop pods land on dry ground; fog of war hides what is not reachable from it)

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
-   [-] **Display skills/traits/incapabilities/health/relationships** (all but relationships on this screen; starting families are seeded but not shown here yet)

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
-   [x] **Generate equipment based on faction/tech** (NG-1: neolithic factions field clubs only, industrial ones rifles; CBT-3: armor kits in `domain/combat/kit` by tech and weapon cost, wiki pawn kinds)
-   [x] **Map-edge arrival** (raids walk in from a map edge)
-   [x] **Immediate attack** (the assault plan)
-   [x] **Prepare-before-attack behavior** (DEF-4 staged raids: gather 10 tiles in from the edge, storm on a colonist in sight, when gathered after 3 h, or at a 6-14 h limit; `raid/stage.go`)

## 24.3 Raid tactics

-   [x] **Raid leftovers** (CBT-4: crop burning, siege resupply, a staged assault timer of 10-15 h in `domain/raid`)
-   [x] **Basic assault** (assault plan: target the nearest colonist, ally or powered turret; breach when shut out)
-   [x] **Sappers/breachers equivalent** (against an enclosed colony from day 10: tunnel through walls, doors and rock, step around traps)
    -   Attack walls/alternate route instead of blindly using normal
        entrance.
-   [x] **Drop-pod assault** (DEF-4: industrial factions and up, 30 of 90 map-entry rolls (wiki); pods fall 240 ticks, break the roof they come through (it hurts whoever is beneath), leave a pod shell; `raid/pods.go`)
    -   Industrial factions can arrive from above.
-   [x] **Center/scatter drop variants** (DEF-4: on the colonist nearest home, together near an edge, or scattered over the map)
-   [x] **Siege** (from day 12, industrial and 300+ points: camp 32 tiles out, sandbags, two mortars the raiders crew (15 high-explosive, 5 incendiary shells), storm after 1.5-3 days or at once on 8 % of hits (wiki))
    -   Raiders establish temporary siege position, receive/build
        artillery/supplies, bombard colony, then assault.

## 24.4 Raider morale

-   [x] **Human raider flee threshold** (badly hurt raiders, or a group that has lost its rolled 40-70 % (wiki), flee to the edge; logged once)
    -   Group may flee after sufficient casualties/damage.
-   [x] **Kidnap behavior** (raiders carry off downed colonists unless guarded; `kidnap.go`)
    -   Raiders can carry downed colonists away under suitable
        circumstances.
-   [x] **Steal behavior** (DEF-4 Steal raids: take the nearest stack worth 50+ silver and run, dropping it where they fall; CBT-4: other raiders grab loot on retreat)
    -   Raiders may steal valuable reachable items when
        retreating/meeting goals.

## 24.5 Mechanoid threats

-   [-] **Mechanoid pawn types** (DEF-4 R2: Harrow, Spindle, Bulwark, Quill, original designs with wiki role stats; own brain (`game/raid/mech.go`), no needs, never tamed, never down or flee, EMP stun with adaptation, wrecks shredded for salvage; missing: closed doors block them, voices blocked on the user (insect group stands in) [!])
    -   Machine enemies with different weapon/range roles.
    -   No human mood/needs.
-   [-] **Mechanoid raid** (`Mechs.Raid` from 300 points at an arrival spot, storyteller face with 1/(hostile human factions + 1); missing: mech groups do not use the pod hook)
    -   Persistent hostile behavior; does not use ordinary human morale.
-   [x] **Crashed ship-part threat** (DEF-4 R2: `construction.ShipPart` 6x3, 1200 HP, plant blight 5 to 100 tiles over 20 days, sleeping mech guards that wake once, salvage on destruction, incident weight 2 with a 400-point gate)
    -   Psychic/defoliator-like hostile world object.
    -   Dormant/guarding mechanoids.
    -   Persistent map effect until destroyed.
    -   Use original content/names.

## 24.6 Insects

-   [x] **Insect species** (SIM-019: hive beetle, hive brute, hive crawler; original designs)
-   [x] **Hive** (CBT-4: construction `HiveMound`, wiki 130 HP, cap 30, spreads within 10 tiles, -17 C; breed and spread rates original)
    -   Spawns/maintains insect population.
-   [-] **Infestation incident** (SIM-019: hive swarm bursts from the rock nearest the colony from day 12; hive mounds exist since CBT-4; the dark/warm placement rule is not verified)
    -   Can emerge in valid under-mountain/dark/warm areas according to
        independently implemented rules.
-   [x] **Hive reproduction** (CBT-4: hive mounds breed and spread at original rates)
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
-   [x] **Friendly visitors** (SIM-046)
-   [x] **Wanderer joins** (SIM-046)
-   [ ] **Transport pod crash / downed stranger**
-   [ ] **Resource meteorite**
-   [ ] **Cargo pod drop**
-   [x] **Animal joins** (SIM-025: stray joins)
-   [x] **Farm animals wander in** (SIM-025)
-   [x] **Rare self-tame animal** (SIM-025: taken a liking)
-   [x] **Wild animal migration**
-   [x] **Party** (SIM-042)
-   [x] **Marriage** (SIM-042: wedding gathering)
-   [ ] **Psychic soothe-like temporary positive mood event**
    -   Original presentation/content.

## 25.3 Negative incidents

-   [x] **Raid** (edge arrival, assault nearest colonist, flee, bleed out, dropped weapons; saved)
-   [ ] **Infestation**
-   [x] **Manhunter pack**
-   [x] **Animal insanity** (SIM-025: mad animal, a single manhunter)
-   [x] **Disease outbreak** (MED-04: six Nature incidents from day 15, biome-gated (malaria warm, sleeping sickness tropical), weights scaled by Disease frequency (0 = none), one or several victims, caution letter)
-   [x] **Blight** (FOOD-2: incident `crop_blight`, domesticated crops under 15 grow days, 4-tile spread, cut to clear; a Cut all blight command is FOOD-5)
    -   Crop disease spreads/destroys plants.
-   [x] **Toxic fallout** (FOOD-6: see 12.6)
-   [-] **Heat wave** (NG-1: see 12.6; fired by the day rollover, not the storyteller schedule)
-   [-] **Cold snap** (NG-1: see 12.6)
-   [x] **Eclipse** (FOOD-6: see 12.6)
-   [x] **Solar flare** (PLAY-021: every net goes down for 3-8 h, generators idle; letters at start and end, a status chip)
-   [ ] **Flashstorm**
-   [ ] **Dry thunderstorm**
-   [ ] **Psychic drone-like mood event**
-   [ ] **Volcanic-winter-like long climate event if part of target base
    version**
-   [x] **Crashed hostile ship part** (DEF-4 R2 incident `CrashedShipPart`, see 24.5; storyteller weight 40 vs people raids 74)
-   [x] **Short circuit/battery discharge** (PLAY-021: see 15.2)
-   [x] **Animal predator hunts colonist**
    -   May be AI-driven rather than storyteller incident.

## 25.4 Letters/messages

-   [-] **Event letter system** (PLAY-018: letters kept and saved in a right-side stack with threat/caution/info/good kinds, camera jump to their place or creature, dismissed by the player; no choices yet)
    -   Positive/neutral/negative/threat categories.
    -   Camera jump target.
    -   relevant entities.
    -   optional choices.
-   [-] **Message system** (short notices under the colonist bar that fade; no rate limiting yet)
    -   Short transient notices.
    -   Rate-limit repeated low-value messages.
-   [-] **Alert system** (persistent alerts for low food, rescue, starving, collapsed, break risk, hypothermia/heatstroke, fuel, spoiling, waiting blueprints, unhappy, raids, fire, animals; PLAY-018: severity order, hover explanation, click/right-click cycling; no research/idle/unburied/beds alerts yet)
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

-   [x] **Consumable drug** (SIM-040: beer, ashleaf tea, spark tablet with highs and mood; MED-07: overdose condition (wiki stages 50/75/100 %, decay 100 %/day), numbroot tincture painkiller, prophyl tablet)
    -   immediate effects.
    -   health effects.
    -   mood effects.
    -   consciousness/movement/pain modifiers.
    -   tolerance.
    -   addiction chance.
    -   overdose/toxicity where appropriate.
-   [-] **Representative functional categories** (SIM-040: alcohol, mild stimulant, spark tablet as the hard drug; MED-07: painkiller and disease prevention; no smokeleaf/go-juice)
    -   beer/alcohol.
    -   smokeleaf-like relaxant.
    -   psychite tea-like mild stimulant.
    -   wake-up-like stimulant.
    -   go-juice-like combat stimulant.
    -   hard euphoric drug.
    -   Use original names/assets.

## 26.2 Tolerance

-   [x] **Drug tolerance** (SIM-040)
    -   Repeated use builds tolerance.
    -   Tolerance reduces effect and/or increases health risk depending
        on definition.
    -   Decays with abstinence.

## 26.3 Addiction

-   [x] **Addiction chance** (SIM-040)
-   [x] **Addiction need** (SIM-040)
    -   Becomes a pawn need.
-   [x] **Withdrawal** (SIM-040: recovery after 10 days)
    -   Health/mood/stat effects.
    -   Resolves after sufficient abstinence for recoverable addictions.

## 26.4 Drug policy

-   [-] **Policy definitions** (SIM-040: three presets (No drugs, Social drinking, Unrestricted); no scheduling, thresholds or carry quantity)
    -   Allowed drugs.
    -   recreational use.
    -   scheduled use.
    -   mood threshold.
    -   recreation threshold.
    -   carry quantity.
-   [x] **Autonomous drug use** (SIM-040: craving with hysteresis; policy and traits respected)
    -   Pawn follows policy unless mental state/trait overrides.

## 26.5 Chemical traits

-   [x] **Chemical-interest/fascination equivalent** (SIM-040: Abstainer/Indulgent/Hedonist)
    -   Alters drug desire/autonomous behavior and chemical need.

------------------------------------------------------------------------

# PHASE 27 --- Clothing Assignment, Food Restrictions, Medical Policies, and Colony Management UI

## 27.1 Assign tab

-   [x] **Food restriction policy** (FOOD-4/FOOD-5: policies Lavish, Fine, Simple, Paste, Raw, Nothing and an Animal feed default for tame animals; Assign tab Food column and policy editor; `SetFoodPolicy` / `EditFoodPolicy`; food search respects them)
    -   Allowed food categories/items.
    -   Pawn food search respects policy.
-   [ ] **Apparel policy**
    -   Already implemented functionally; expose colony-wide policy
        editor.
-   [-] **Drug policy** (SIM-040: preset selector on the Assign tab (F3); no policy editor)
    -   Already implemented functionally; expose editor.
-   [x] **Medical care policy** (MED-03, see Health above)
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

-   [-] **Form caravan dialog** (SIM-046: colonists and destination; no animals, prisoners or items)
    -   Select colonists.
    -   animals.
    -   prisoners.
    -   items.
    -   food.
    -   destination.
-   [ ] **Mass/carry capacity**
    -   Pawns and pack animals carry limited mass.
-   [-] **Travel supplies** (SIM-046: food days only)
    -   Estimate food and travel time.
-   [ ] **Gather/loading phase**
    -   Pawns collect assigned cargo before departure.
-   [x] **Exit map** (SIM-046: walks to the edge, becomes a world object)
    -   Caravan becomes world object.

## 28.2 World movement

-   [-] **World pathfinding** (SIM-046: route over the rebuilt planet)
    -   Route across world tiles.
    -   Terrain/roads affect movement.
-   [-] **Travel speed** (SIM-046: slowest Moving speed; no mass/terrain/weather)
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

-   [-] **Food consumption** (SIM-046: food days)
-   [x] **Health/disease progression** (MED-04: hourly in the medical phase; MED-11: caravan members keep progressing, outbreaks can strike a caravan, the caravan's best medic tends each hour, and deaths away from home go through dieAway)
-   [ ] **Mood effects**
-   [x] **Animal feeding** (SIM-024: animals graze, eat carcasses and crops, and are fed from troughs)
-   [ ] **Prisoner handling**

Use aggregated/analytical updates rather than simulating local-map jobs.

## 28.4 Caravan interactions

-   [-] **Visit friendly settlement** (SIM-046: one day; no trade or gifts)
    -   Trade.
    -   gifts.
-   [ ] **Attack hostile settlement**
-   [ ] **Enter event/quest map**
-   [ ] **Ambush encounter**
-   [x] **Return to colony** (SIM-046)
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

-   [-] **Siege construction** (sandbags in front of each besieger; player mortars exist (DEF-3), raiders do not use mortars (DEF-4))

-   [ ] **Siege bombardment**

-   [ ] **Siege assault transition**

-   [x] **Retreat** (raiders leave at the flee thresholds)

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
-   [x] **Power** (PLAY-021: conduit, hidden conduit, switch, four generators, battery)
-   [ ] **Security**
-   [ ] **Misc/ship**
    -   Categories may use original naming.

## 34.2 Overlays

-   [-] **Roof overlay** (SIM-018; no thin or mountain roof distinction)
-   [x] **Power grid** (PLAY-021: the power grid overlay)
-   [ ] **Beauty**
-   [ ] **Room stats**
-   [x] **Fertility**
-   [ ] **Terrain/path cost debug**
-   [-] **Home area** (SIM-021: painted, auto-grown around buildings, drives cleaning; no firefighting or repair yet)
-   [-] **Allowed areas** (SIM-046: see 8.x)
-   [ ] **Storage zones**
-   [ ] **Growing zones**
-   [ ] **Temperature**

## 34.3 Pawn tabs

-   [ ] **Needs**
-   [-] **Health** (SIM-039/UI-022: Health tab live)
-   [ ] **Gear**
-   [x] **Social** (UI-022: Social tab live)
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

-   [x] **Top-priority alerts** (PLAY-018: danger alerts first, pulsing)
-   [x] **Event letters** (PLAY-018: right-side letter stack)
-   [x] **Messages** (PLAY-018)
-   [x] **Jump-to-location** (messages, alerts and letters jump the camera)
-   [x] **Pause-on-major-threat option** (Options › Gameplay)

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
-   [x] **Power network overlay** (PLAY-021: Shift+P; auto-shown while placing power buildings)
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
-   [x] **Disease/health effects where safe** (MED-07: painkiller halves pain for 12 h, prophyl blocks plague, malaria and sleeping sickness for 5 days, overdose from hard drugs and prophyl)
-   [ ] **World settlements/caravans**
    -   Aggregate simulation rather than local entity simulation.

## 40.3 Invalidation

-   [x] **Navigation dirty regions** (NG-1: a walkability change marks its 16x16 chunk dirty; walk regions and labels rebuild for dirty chunks only)
-   [x] **Room dirty regions** (NG-1: incremental rooms from room regions per chunk, checked against full detection in tests)
-   [x] **Power topology dirty** (PLAY-021: the net cache keys on structVer, a switch topology version and the store)
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
-   [-] Create stockpile. (filter tree, names, copy/paste, shelves; deeper tree, specials, linked groups and hoppers: STO-2, STO-3)
-   [x] Grow crops.
-   [ ] Hunt animals.
-   [ ] Cook food.
-   [-] Survive hunger/rest/temperature. (hunger: malnutrition (FOOD-1); rest: exhaustion; no temperature)
-   [ ] Treat injuries/disease.
-   [-] Manage mood.
-   [ ] Survive mental breaks.
-   [ ] Produce equipment.
-   [ ] Research technology.
-   [ ] Generate power.
-   [ ] Refrigerate food.
-   [ ] Trade.
-   [x] Recruit prisoners.
-   [x] Tame/train animals.
-   [ ] Defend raids.
-   [ ] Survive environmental events.
-   [-] Form caravans. (SIM-046: no trade or cargo)
-   [ ] Complete world quests.
-   [ ] Build/activate endgame ship.
-   [ ] Survive final assault period.
-   [ ] Escape.

## 41.2 Emergent-story audit

-   [ ] Pawns have meaningfully different skills/traits.
-   [x] Pawns form opinions and relationships.
-   [x] Relationships create gameplay consequences. (grief, opinion, romance, fights, prisoners, caravans, animal bonds)
-   [-] Injuries have anatomically meaningful consequences. (SIM-039: parts, capacities, scars, surgery; no bleeding or disease framework)
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
-   [-] Renderer FPS does not change result. (PERF-002: the client steps the sim one tick at a time in fixed order and the Update time budget only moves ticks between Updates; the goldens pin the state. No test yet runs the client at two frame rates and compares hashes.)
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
-   [-] Large-colony performance targets have reproducible benchmarks. (PERF-002: `make fps`, `cmd/benchmark` with a `views` row, `BenchmarkWorldView`, `BenchmarkGroundCover`, `BenchmarkShadowsHiDPI` and recorded baselines in the handoff; the colonist-count scales are still uncovered.)
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
