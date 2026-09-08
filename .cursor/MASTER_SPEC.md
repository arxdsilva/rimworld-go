# rimworld-go — Master Product, Architecture & Multi-Agent Engineering Specification

**Status:** Greenfield master specification  
**Primary implementation language:** Go  
**Working repository name:** `rimworld-go`  
**Purpose:** This file is the authoritative engineering/product contract for humans and multiple AI coding agents collaborating on a large-scale open-source colony simulation.

`rimworld-go` is intended to reproduce the *experience class* of a deep systemic colony simulator: autonomous settlers, construction, survival, farming, crafting, combat, health, mood, relationships, research, factions, trade, environmental simulation, disasters, and emergent storytelling.

The project must be an original implementation. Do not copy commercial source code, decompiled logic, proprietary datasets, dialogue, lore, artwork, UI artwork, sounds, names, exact balancing tables, or extracted assets. Colony-simulation mechanics and broad genre ideas may be independently implemented, but final branding, visual identity, content, terminology, code, and balance must be original.

The primary differentiator is scale: the architecture should ultimately support colonies with **1,000+ active settlers**, very large maps, tens or hundreds of thousands of world objects, headless simulation, deterministic replay, extensive modding, and lower CPU/RAM use than conventional simulation-heavy games.

For the ordered, checkbox-tracked feature and UI/UX backlog derived from this spec, see `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` and `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`. For per-screen layout, navigation, commands, and states, see `MAIN_UI_SCREENS_SPECIFICATION.md` (agent playbook: `skills/rimworld-go-ui-screens/SKILL.md`). For player-visible art, sprites, icons, UI chrome, and the asset pipeline, see `ART_DIRECTION_AND_ASSET_PIPELINE.md` (agent playbook: `skills/rimworld-go-art-assets/SKILL.md`).


# 1. Product Pillars

1. **Emergent stories from interacting systems.**
2. **Performance is a player-facing feature.**
3. **Simulation and rendering are strictly decoupled.**
4. **Determinism is a design target, not a test-only feature.**
5. **Content is data-driven.**
6. **Modding uses first-class APIs and schemas.**
7. **Large colonies must be manageable through groups, roles, policies, and automation.**
8. **AI agents must work through explicit package ownership and documented contracts.**
9. **Go code remains idiomatic, explicit, benchmarkable, and easy to inspect.**
10. **Optimization follows profiling: complexity → frequency → invalidation → sleeping → batching → allocations → parallelism.**


# 2. Player Fantasy and Core Loop

The player establishes and manages a remote settlement populated by individually simulated people.

Primary actions are indirect: designations, work priorities, schedules, allowed areas, storage rules, production bills, construction, research priorities, equipment rules, medical policy, farming plans, trading, diplomacy, and tactical combat orders.

Core loop:

1. Begin with a small group and limited resources.
2. Secure food, shelter, beds, temperature stability, medicine, and storage.
3. Create sustainable food/resource production.
4. Build workshops and specialized infrastructure.
5. Improve comfort, safety, morale, and productivity.
6. Research new capabilities.
7. Defend against hostile factions, wildlife, fire, weather, and disease.
8. Grow through recruitment, rescue, migration, or later population systems.
9. Build a resilient economy and social structure.
10. Experience unscripted stories produced by losses, relationships, accidents, victories, and external events.
11. Pursue optional long-term goals or survive indefinitely.


# 3. Explicit Non-Goals for the First Releases

Not required for the first stable playable release:

- photorealistic/3D graphics;
- physically accurate fluid dynamics;
- multi-floor maps;
- multiplayer;
- distributed simulation;
- vehicles;
- world-scale individual NPC simulation;
- arbitrary native Go plugins;
- LLM-dependent gameplay;
- exact duplication of another game's content or balance;
- fully destructible geology;
- complex orbital/space simulation.

Architecture may leave room for these, but they must not block the core colony simulation.


# 4. Top-Level Architecture

Use this dependency direction:

```text
Platform / Filesystem / Logging / Profiling
                 ↓
       Core primitives and content IDs
                 ↓
Grid / Chunks / Entity Storage / Spatial Index
                 ↓
          Simulation Systems
                 ↓
 Commands / Saves / Replay / Session
                 ↓
       Presentation / UI / Audio
```

The authoritative simulation must compile and run without renderer/audio packages.

Required headless capability:

```bash
rimworld-go simulate --seed 42 --ticks 1000000 --scenario frontier
rimworld-go benchmark --scenario colony-1000
rimworld-go replay verify bug-123.rwgr
```

The renderer consumes read-only snapshots. It never owns authoritative state.


# 5. Repository Layout

Recommended structure:

```text
cmd/
  game/
  simulate/
  benchmark/
  contentlint/
  replay/
internal/
  app/
  sim/
    clock/
    entity/
    components/
    scheduler/
    command/
    event/
    snapshot/
  world/
    grid/
    chunk/
    region/
    room/
    generation/
    terrain/
    temperature/
    weather/
    light/
    fire/
  pawn/
    needs/
    skills/
    traits/
    mood/
    memory/
    social/
    health/
    inventory/
  ai/
    intent/
    jobs/
    work/
    reservation/
    pathing/
    movement/
    combat/
  economy/
    item/
    storage/
    recipe/
    production/
    trade/
  building/
    construction/
    doors/
    zones/
    power/
  combat/
    weapon/
    projectile/
    damage/
    armor/
    cover/
  faction/
  incident/
  research/
  save/
  replay/
  content/
  mod/
  metrics/
  debug/
pkg/
  geom/
  fixed/
  collections/
  randx/
content/
assets/
mods/
scenarios/
benchmarks/
docs/
  architecture/
  systems/
  adr/
  ai-agents/
    handoffs/
tools/
scripts/
```

Avoid catch-all `utils` packages and circular dependencies.


# 6. Authoritative Simulation Clock

Use discrete ticks. The precise in-game time conversion is content/balance, but authoritative ordering always uses integer tick numbers.

Different systems run at different frequencies. Example:

| System | Typical frequency |
|---|---:|
| movement / projectile | every tick |
| combat state | every tick |
| job state machine | every 3–10 ticks |
| urgent needs | every 30–60 ticks |
| normal needs | every 60–300 ticks |
| mood | every 300+ ticks or invalidation |
| plant growth | 600–3600 ticks / analytical |
| relationship decay | slow buckets |
| world-faction simulation | in-game hours/days |

Never update every entity for every subsystem every tick.

Deterministic bucket scheduling:

```text
bucket(entityID, frequency) = entityID % frequency
```

If 60,000 objects update once per 60 ticks, each tick touches roughly 1,000.


# 7. Determinism Contract

Authoritative gameplay must avoid hidden nondeterminism.

Rules:

- no global `math/rand`;
- no wall-clock time inside simulation outcomes;
- no machine-specific decisions based on CPU count or scheduler timing;
- no order-sensitive Go map iteration;
- stable entity IDs;
- stable command ordering;
- stable event ordering;
- integer/fixed-point values for cumulative authoritative values where practical;
- worker goroutines operate on immutable snapshots;
- worker results are committed in deterministic sequence order;
- state hashing must be stable.

Recommended random identity:

```text
hash(worldSeed, rngStream, tick, entityID, operationKind, localSequence)
```

Separate streams/categories for world generation, combat, incidents, health, social behavior, etc. Adding one unrelated random roll should not shift every later random result.


# 8. Simulation Phases

Use explicit phase ordering:

```text
1. ApplyPlayerCommands
2. Spawn/Despawn Commit
3. Emergencies & Critical Needs
4. High-Level Intent
5. Job Selection
6. Job Execution
7. Movement
8. Combat
9. Damage / Health
10. Environment
11. Production / Construction
12. Social / Mood
13. Incidents / World Systems
14. Cleanup / Reservation Release
15. Derived-State Invalidation
16. Publish Read Snapshot
17. Tick++
```

Avoid a magical event bus that hides ownership. Typed event queues are acceptable; mutation order must remain visible and documented.


# 9. Entity and Component Model

Use a data-oriented ECS-like hybrid, but do not begin with a reflection-heavy general-purpose ECS framework.

```go
type EntityID uint64
```

IDs are monotonically allocated and never reused within a running world.

Components can live in sparse-set/dense-array stores:

```go
type ComponentStore[T any] struct {
    denseEntities []EntityID
    denseValues   []T
    sparse        []uint32
}
```

Likely components:

```text
Position
Pawn
Species
Needs
Skills
Traits
Mood
Health
Inventory
Equipment
Faction
CurrentJob
Path
Building
Plant
Animal
ItemStack
Container
PowerNode
TemperatureSource
Reservation
Decay
Quality
Ownership
Forbidden
```

Hot loops should iterate dense arrays rather than maps of pointers.


# 10. Entity Lifecycle and Invariants

Lifecycle:

```text
allocate ID
→ queue spawn
→ commit spawn during spawn phase
→ active
→ queue destroy
→ commit destruction
→ emit destruction events
→ release reservations/references
```

Never mutate store structure while iterating it.

Global invariants:

- every live entity has a unique ID;
- destroyed entities cannot execute jobs;
- no negative stack quantity;
- one active job maximum per pawn;
- exclusive reservations cannot overlap;
- item location is unambiguous;
- paths contain only valid walkable tiles for their navigation revision;
- simulation tick never moves backward;
- content registry is immutable after startup;
- save/load preserves authoritative state.


# 11. Grid and Chunking

The local map is a 2D tile grid. Start around 256–512 tiles per side, but design for 2048×2048 or larger.

Recommended chunk size: 32×32 tiles.

Each chunk owns/cache-localizes:

- terrain;
- roofs;
- compact tile flags;
- occupancy indexes;
- item/pawn/plant references;
- dirty flags;
- room/region references;
- temperature summary;
- light information;
- fire active set;
- navigation metadata.

Chunk activity levels:

```go
Sleeping
Background
Active
Critical
```

Sleeping chunks skip unnecessary work. Background chunks use analytical/staggered updates. Critical chunks handle combat/fire/rapid changes.


# 12. Dirty Flags and Invalidation

Expensive derived state recalculates only after relevant changes.

Examples:

```text
NavigationDirty
RegionDirty
RoomDirty
PowerNetworkDirty
TemperatureTopologyDirty
LightingDirty
BeautyDirty
StorageDirty
CoverDirty
RoofDirty
```

Examples:

```text
wall built → local region + room + navigation dirty
door destroyed → local navigation + room dirty
power cable changed → affected power graph dirty
item moved → storage/resource aggregate dirty
lamp toggled → lighting region dirty
```

Never recompute entire-map rooms, power, pathing, lighting, or resource totals every tick.


# 13. Spatial Indexes

Required spatial queries must be indexed:

```text
EntitiesAt(tile)
PawnsInChunk(chunk)
ItemsInRegion(region)
EntitiesWithinRadius(center, radius)
NearestCandidate(filter)
HostilesNear(entity)
HaulablesNear(region)
```

No gameplay system may repeatedly scan all entities to answer local questions.

Tile membership for zones can use chunk-local bitsets.


# 14. World and Local Map Generation

World generation has two scales:

**World layer**
- elevation;
- rainfall;
- average temperature;
- biomes;
- rivers;
- roads;
- settlement nodes;
- faction territories;
- travel difficulty;
- broad resource profiles.

**Local colony map**
- terrain;
- soil;
- mountains;
- water;
- vegetation;
- deposits;
- ruins;
- wildlife;
- safe spawn area.

All generation is seed-reproducible. Concurrency may calculate independent chunks, but results must merge in stable order.

Do not simulate every individual person in off-map settlements. Off-map locations use aggregate population, wealth, military, production, and relations.


# 15. Regions and Hierarchical Pathfinding

Long-distance movement must not use naive full-map tile A* for every pawn.

Maintain walkable connected **regions** and region adjacency.

Path pipeline:

```text
origin tile
→ origin region
→ high-level region route
→ local detailed segment
→ follow segment
→ refine next segment when needed
```

Detailed algorithm can initially be A*. Later evaluate JPS, bidirectional A*, flow fields, or specialized movement profiles.

Cheap job distance estimation should use Manhattan/octile/region distance. Only a small finalist set gets exact path evaluation.

Dynamic pawn collisions should use local avoidance, not global repathing.


# 16. Path Requests and Parallelism

Pathfinding is a good worker-pool candidate.

```go
type PathRequest struct {
    Seq      uint64
    Pawn     EntityID
    Start    Tile
    Goal     Goal
    Profile  MovementProfile
    NavRev   uint64
}
```

Workers read immutable navigation snapshots. Results contain the same sequence. Simulation sorts/commits by sequence.

Never allow goroutine completion order to change game outcomes.

Cache high-level region routes by:

```text
originRegion + targetRegion + movementProfile + navigationRevision
```

Paths should invalidate locally when geometry revisions change.


# 17. Pawn Model

A pawn is a collection of components rather than a giant class.

Core aspects:

- species/anatomy;
- age;
- position;
- needs;
- health;
- skills;
- traits;
- mood;
- memories;
- relationships;
- work profile;
- schedule;
- inventory/equipment;
- faction;
- current intent/job/path.

Do not create one goroutine per pawn. Pawns are data; systems process them in batches.


# 18. Needs

Initial needs:

```text
food
rest
comfort
recreation
social
safety
environment
```

MVP may implement only food/rest/comfort.

Use compact fixed-point values, e.g. `uint16 0..65535`.

Needs update through staggered/analytical decay:

```text
newValue = oldValue + elapsedTicks * rate
```

Critical thresholds emit urgency signals so AI can react immediately without running full need logic every tick.


# 19. Mood and Memories

Mood is derived from:

```text
baseline
+ current needs
+ active memories
+ traits
+ health
+ environment
+ relationships
+ notable events
```

Memory:

```go
type Memory struct {
    DefID      DefIndex
    CreatedAt  Tick
    ExpiresAt  Tick
    Subject    EntityID
    Magnitude  int16
}
```

Mood should be cached and invalidated, not constantly recomputed.

For very large colonies, notifications are aggregated: “27 settlers are critically hungry” rather than 27 alerts.


# 20. Skills and Traits

Skills initially:

```text
construction
cooking
medicine
plants
crafting
research
mining
shooting
melee
social
animals
```

Skill:

```go
type Skill struct {
    Level   uint8
    XP      uint32
    Passion uint8
}
```

Traits are definition-driven stat/behavior modifiers. Avoid embedded arbitrary scripts in v1.

Computed stats use deterministic modifier order:

```text
base → additive modifiers → multiplicative modifiers → clamp
```

Cache stat groups and invalidate after equipment, health, trait, or environmental changes.


# 21. Relationships and Social Simulation

Never allocate an N×N relationship matrix.

Store only meaningful sparse relationships.

```go
type Relationship struct {
    Other   EntityID
    Opinion int16
    Kind    RelationshipKind
}
```

Social interactions are event-driven. Relationship decay is staggered across existing sparse edges.

Possible relationships: friend, rival, partner, spouse, parent, child, sibling, mentor, enemy.

The story system records major social events but should avoid notifying the player about every trivial interaction in a 1,000-person settlement.


# 22. Health and Anatomy

Species define anatomy trees instead of hardcoded humanoid body parts.

Health conditions are data-driven effects attached globally or to a body part.

Categories:

```text
wounds
burns
infection
disease
scars
missing parts
prosthetics
drug effects
hypothermia
heat injury
```

Damage pipeline:

```text
attack
→ hit determination
→ body part
→ armor
→ damage type
→ wound/condition
→ bleeding/pain
→ incapacitation/death
```

Use aggregate bleeding rates, not particle simulation.


# 23. Jobs and Work Architecture

High-level AI selects intent. Jobs execute concrete work.

Intent priority concept:

```text
forced command
→ immediate danger/medical emergency
→ survival need
→ schedule need
→ assigned work
→ recreation/social
→ idle
```

Work types:

```text
firefighting
patient
doctoring
rescue
construction
repair
haul
clean
cook
hunt
grow
harvest
mine
craft
research
social/wardening
animal handling
```

A job is an explicit deterministic state machine.


# 24. Job Structure and State Machines

Example:

```go
type Job struct {
    ID        JobID
    Def       DefIndex
    Actor     EntityID
    TargetA   Target
    TargetB   Target
    Count     int32
    State     JobState
    CreatedAt Tick
}
```

Construction example:

```text
AcquireMaterial
→ MoveToMaterial
→ PickUp
→ MoveToBlueprint
→ Deliver
→ Work
→ Complete
```

Failure reasons must be explicit and debug-visible:

```text
TargetDestroyed
PathBlocked
ReservationLost
Forbidden
Unsafe
MissingMaterial
ActorIncapacitated
Interrupted
```


# 25. Work Discovery at Scale

Never let every pawn scan every possible task.

Maintain central indexes per work type:

```text
available targets
priority
region
skill requirement
reservation state
urgency
```

Selection pipeline:

```text
10,000 possible targets
→ work-type and region filter
→ ~100 cheap candidates
→ integer score
→ ~5 finalists
→ exact path check
→ reserve best
```

This is a foundational scalability rule.


# 26. Reservation System

Reservations prevent conflicting actors.

Reservable:

- entities;
- tiles;
- item quantities;
- beds;
- patients;
- interaction slots;
- workstation slots.

```go
TryReserve(actor, target, amount) bool
Release(actor, target)
ReleaseAll(actor)
```

Automatic release on:

```text
job completion/failure
pawn downing/death
target destruction
forced cancellation
```

Developer builds should run reservation-leak validation.


# 27. Movement and Congestion

Movement follows paths tile-to-tile with deterministic conflict arbitration.

Potential tile arbitration:

```text
forced/emergency priority
then movement priority
then entity ID
```

Do not treat other pawns as static permanent obstacles. Use local waiting/avoidance and short replans.

Track optional decaying congestion cost so heavily used corridors become slightly more expensive for future path searches.

Flow fields can later optimize raids, evacuations, and mass same-destination movement.


# 28. Items, Inventory, and Containers

Bulk resources should use stack entities.

```go
type ItemStack struct {
    Def       DefIndex
    Count     uint32
    Quality   uint8
    Condition uint16
}
```

An item exists in exactly one place:

```text
ground
pawn inventory
container
```

Transfers must be atomic.

Compatible stacks merge based on definition, quality/condition rules, and stack limit.

Avoid one entity per grain/unit/material piece.


# 29. Storage and Hauling

Storage zones have:

```text
filter
priority
free capacity/slots
region index
```

Maintain indexed free destinations.

Hauling candidate selection uses cheap distance before exact path.

Multiple haulers can reserve quantities from one stack without over-reserving.

Long-term optimizations:
- opportunistic hauling;
- batch hauling;
- route chaining;
- role-specific haulers;
- logistics automation.


# 30. Construction

Lifecycle:

```text
designation
→ blueprint
→ resource reservations/delivery
→ work
→ completed building
```

Blueprints are lightweight entities.

Buildings are data definitions plus runtime components.

Geometry changes invalidate local:

```text
navigation
regions
rooms
cover
lighting
temperature topology
```

depending on building properties.


# 31. Rooms and Temperature

Rooms derive from enclosure/region topology.

Cached room stats:

```text
area
wealth
beauty
cleanliness
temperature
bed count
workstation count
```

Temperature should be room/thermal-region based, not expensive per-tile fluid simulation.

Thermal region stores:

```text
temperature (milli-Celsius)
heat capacity
heat gains/losses
adjacent transfer boundaries
```

Heat exchange updates slowly unless critical.


# 32. Weather, Light, and Fire

Weather modifies temperature, visibility, fire, solar production, plant growth, work, combat, and mood.

Fire uses an **active burning set**, not a whole-map scan. Fire entities/tiles update in buckets. Spread probability uses deterministic keyed RNG based on flammability, neighbors, humidity/weather, roof state, and intensity.

Lighting can be chunk/region approximate. Rebuild only dirty areas.

No expensive global illumination is required.


# 33. Plants and Farming

Plant state:

```go
type Plant struct {
    Def            DefIndex
    Growth         uint16
    Health         uint16
    LastGrowthTick Tick
}
```

Growth is analytical:

```text
growth += elapsed × effectiveRate
```

Effective rate derives from fertility, temperature, light, season, water rules if present.

Growing zones generate sow/harvest/cut work via indexes.

100,000 wild plants must not require 100,000 updates every tick.


# 34. Animals

Animals can share pawn positioning, health, movement, and combat components while using simpler intent logic.

Common intents:

```text
wander
graze
sleep
flee
hunt
attack
follow
mate
```

Domestication can later add training, master, allowed zone, hauling, guarding, rescue.

Inactive herds should update at low frequency.

Off-map ecology is aggregate population statistics, not individual entities.


# 35. Combat

Combat supports melee, ranged, and later explosives.

Ranged pipeline:

```text
select target
→ aim/warmup
→ fire
→ deterministic hit resolution
→ logical projectile flight
→ armor/cover
→ body damage
→ cooldown
```

Prefer logical projectiles in v1: authoritative hit/flight data with renderer interpolation, rather than expensive physics.

Accuracy may consider skill, weapon, range, cover, light, weather, target size, and movement.

Melee uses approach → attack selection → hit/dodge → armor → damage → cooldown.


# 36. Enemy and Group AI

Hostiles should have group-level objectives plus individual execution.

Group objectives later:

```text
assault
siege
sabotage
steal
kidnap
flee
```

MVP: approach colony and attack reachable targets.

Group planning prevents hundreds of attackers from independently performing expensive strategic searches.

All enemy target lookup must use spatial and region indexes.


# 37. Factions, Trade, and World Strategy

Off-map factions are aggregates:

```text
relations
wealth
population
military strength
technology profile
settlements
trade profile
```

World updates at coarse intervals (hours/days).

Relations can map to hostile/unfriendly/neutral/friendly/allied.

Trade price:

```text
base value × quality × condition × trader profile × supply/demand modifiers
```

Do not build a fully agent-based global economy initially.


# 38. Research and Production

Research projects form a DAG:

```go
type ResearchDef struct {
    ID            string
    Cost          int64
    Prerequisites []string
    Unlocks       []string
}
```

Production recipes are data:

```text
ingredients
products
work amount
required skill
minimum level
required research
workstation tags
```

Bills support:

```text
make X
make until X
do forever
one-shot
```

and configurable ingredient filters/radius/worker restrictions later.


# 39. Power and Networks

Electrical networks are graphs of:

```text
producers
consumers
storage
conduits
switches
```

Rebuild connected topology only when a power component changes.

Per-network runtime tracks:

```text
generation
load
stored energy
capacity
```

Never perform global network discovery every tick.


# 40. Incident / Story Director

Build an original event-director system.

Inputs:

```text
colony wealth
population
recent threat history
difficulty
adaptation/recovery
season
biome
faction relations
elapsed time
```

Candidate events receive integer scores and cooldown checks. Deterministic RNG selects from valid candidates.

Event categories:

```text
major threat
minor threat
positive
trade
social
weather
world
```

Create original profiles, e.g. steady escalation, chaotic variance, environmental pressure, faction-war pressure. Do not copy proprietary storyteller personalities/content.


# 41. Story History and Notifications

Maintain structured story records:

```go
type StoryEvent struct {
    Tick     Tick
    Type     DefIndex
    Subjects []EntityID
    Location Tile
}
```

Uses:
- history UI;
- replay summaries;
- debugging;
- optional generated narrative;
- significance scoring.

Alerts have severity and aggregation. At high population, surface systemic issues rather than spamming individual warnings.


# 42. Rendering

Evaluate Ebitengine first because of Go-native ergonomics and portability.

Before committing, spike:

- large tilemap;
- 10k+ sprites;
- camera pan/zoom;
- text;
- UI;
- dirty chunk redraw;
- Windows/Linux/macOS;
- integrated graphics.

Renderer accepts immutable `RenderSnapshot` data. Simulation does not import rendering.

Rendering may freely use floats; authoritative simulation should prefer integers/fixed-point.


# 43. UI and Large-Colony UX

Required major screens:

```text
map
inspector
construction/designation
work priorities
schedules
groups
zones
storage
production
research
health
social
animals
factions
history
alerts
debug tools
```

For 1,000 settlers, individual configuration alone is unusable. Support:

- groups/roles;
- presets/templates;
- bulk work priorities;
- group schedules;
- group allowed areas;
- equipment/food/medical policies;
- searchable/virtualized settler lists;
- alert aggregation;
- favorites/pinned important settlers.

Do not reproduce a commercial title's exact UI artwork/layout.


# 44. Commands and Replay-Ready Input

Every authoritative player action is a command.

Examples:

```text
PlaceBlueprint
CancelDesignation
SetWorkPriority
SetSchedule
DraftPawn
MovePawn
AttackTarget
CreateZone
SetStorageFilter
CreateBill
SetResearch
```

Command envelope:

```text
target tick
source/player ID
sequence
payload
```

Commands are serialized and sorted deterministically. This supports replay and future multiplayer.


# 45. Save Architecture

Do not serialize runtime Go structs using `gob` as the public save contract.

Use explicit versioned schemas, preferably Protobuf initially.

Suggested container:

```text
metadata
world state
map chunks
entities/components
factions
story history
mod list
content dictionary/hash
```

Save metadata includes:

```text
format version
game version
seed
tick
content hash
mods + versions
```

Write atomically through temporary file + rename. Support migrations.


# 46. Snapshot and Autosave

Avoid long pause during large saves.

Pipeline:

```text
authoritative state
→ consistent save snapshot
→ resume simulation
→ async serialization/compression
→ atomic write
```

Snapshot generation must not double enormous amounts of memory unnecessarily.

Future chunk-by-chunk persistence supports partial loading and large worlds.


# 47. Replay and State Hashing

Replay file:

```text
initial scenario/seed
ordered commands
optional periodic checkpoints
expected state hashes
```

Periodic authoritative hashes enable:

- deterministic tests;
- exact bug reproduction;
- future multiplayer desync detection.

On divergence, diagnostic hashing should narrow:

```text
global
→ subsystem
→ component store/chunk
→ first mismatching entity
```


# 48. Modding

Mod structure:

```text
manifest
definitions
patches
textures
audio
localization
optional future scripts
```

Content definition IDs use namespaces:

```text
core.wall_wood
examplemod.weapon_arc
```

Startup:

```text
discover
→ dependency resolution
→ stable load order
→ load definitions
→ apply patches
→ validate
→ resolve references
→ create compact runtime indexes
→ compute content hash
→ freeze registry
```

Core content should use the same definition system available to mods.


# 49. Mod Scripting

Do not load arbitrary native Go plugins by default.

Future choices: Starlark or WASM.

Required properties:
- sandbox;
- deterministic host APIs;
- explicit seeded RNG;
- instruction/time budget;
- memory budget;
- no uncontrolled filesystem/network access;
- cross-platform behavior.

Script performance should be attributable per mod in developer diagnostics.


# 50. Content Definitions and Validation

Definition categories:

```text
terrain
item
building
species
body
need
skill
trait
recipe
plant
weapon
projectile
damage
job
work_type
research
incident
faction
biome
weather
scenario
```

Author in YAML or JSON; validate with schemas. At load, resolve human-readable IDs to compact runtime indexes.

`contentlint` must catch duplicate IDs, invalid references, dependency cycles, invalid dimensions, unknown localization keys, bad recipe outputs, invalid research graphs, and mod patch errors.


# 51. Localization and Assets

All player-facing text uses localization keys. Initial locales can include `en-US`, with community support for `pt-BR`, Spanish, French, etc.

Bundled assets must be original, commissioned, public-domain, or compatibly licensed.

Do not ship extracted assets from commercial games.

Keep code and asset licenses separable if appropriate.


# 52. Security

Treat mods and save files as untrusted input.

Requirements:
- reject path traversal;
- cap decompressed sizes;
- validate lengths/counts;
- schema validation;
- script sandbox;
- no native arbitrary code execution by default;
- clear mod provenance for errors;
- `SECURITY.md`.

Content registry should track which mod/file produced or patched every definition.


# 53. Performance Budgets

Reference target: modern 6-core laptop, 16 GB RAM, integrated graphics supported.

Aspirational simulation targets:

| Scenario | Goal |
|---|---|
| 10 settlers | trivial |
| 100 settlers | <2 ms avg simulation tick |
| 500 settlers | <8 ms |
| 1,000 settlers | <16 ms |
| 100k plants | practical |
| 100k stackable/item-like records | practical |
| 1024²–2048² map | practical memory |
| 100 vs 100 battle | deterministic and smooth |

Treat these as targets to refine with real benchmarks, not promises.


# 54. Memory Policy

Hot-path rules:

- no per-tick temporary maps if avoidable;
- no `fmt.Sprintf` in hot loops;
- avoid pointer-heavy object graphs;
- intern definition IDs;
- dense arrays for hot components;
- bitsets for flags;
- reuse A*/query scratch buffers;
- stack identical resources;
- monitor allocations/tick and heap growth;
- use pools only when benchmarks prove value;
- avoid `unsafe` until benchmark + ADR justify it.

No goroutine per pawn and no channel-driven gameplay architecture.


# 55. Concurrency Model

One authoritative goroutine/thread owns simulation mutation.

Parallel work is snapshot → compute → ordered commit.

Safe candidates:
- pathfinding;
- world generation;
- save serialization;
- render preparation;
- pure batch scoring;
- asset loading.

Unsafe default:
- many goroutines mutating component stores.

Channels are boundary tools, not the core simulation model.


# 56. Profiling and Metrics

Expose metrics:

```text
ticks/sec
ms/tick
phase p50/p95/p99
active entities
active pawns
sleeping chunks
path requests
path cache hit rate
job candidate counts
reservation count
room rebuilds
temperature cost
power rebuilds
save snapshot duration
heap bytes
alloc bytes/tick
```

Integrate `pprof`, runtime trace, and a developer tick profiler.

Optimization tasks should cite before/after benchmarks.


# 57. Testing Pyramid

Required:
- unit tests;
- subsystem tests;
- simulation integration tests;
- determinism golden tests;
- property/fuzz tests;
- benchmarks;
- replay regressions;
- save compatibility fixtures;
- content validation;
- race tests for concurrent boundaries.

Useful invariants for fuzz/property tests:
- quantities never negative;
- IDs never duplicate;
- dead pawns cannot work;
- exclusive reservation uniqueness;
- save/load state hash equality;
- path never crosses invalid impassable tile;
- item cannot be in ground + inventory simultaneously.


# 58. Golden and Long-Run Simulation Tests

Maintain fixed seeds:

```text
seed 1: 100k ticks
seed 42: 1M ticks
seed 12345: long-run
```

Store expected hashes.

Nightly simulation should run several colonies for millions/tens of millions of ticks and detect:
- memory leaks;
- entity leaks;
- orphan reservations;
- invalid references;
- deadlocks;
- deterministic divergence;
- impossible economic states.


# 59. Simulation Test DSL

Provide a concise fixture builder:

```go
scenario := simtest.New().
    Map(100, 100).
    Pawn("alice", 10, 10).
    Item("core.wood", 15, 10, 100).
    DesignateBuild("core.wall", 30, 30).
    Run(10_000)
```

Assertions can check:
- wall completed;
- correct material consumed;
- actor survived;
- no reservation leaks;
- expected final hash.

This enables AI agents to express bugs and acceptance tests quickly.


# 60. Debugging Tools

Developer inspector must expose:

```text
entity components
current intent
candidate intent scores
current job + state
job history/failure
path + region route
reservations
needs
mood modifiers
health
relationships
room ID/stats
region ID
chunk activity
power network
temperature
incident candidate scores
RNG identity
```

Overlays:
- path graph;
- region graph;
- rooms;
- chunk boundaries/activity;
- temperature;
- lighting;
- cover;
- power networks;
- congestion.

Good debugging tooling is mandatory for multi-agent development.


# 61. CI

CI should eventually run:

```text
gofmt check
go vet
staticcheck
go test ./...
go test -race ./...
content validation
determinism smoke
architecture lint
platform builds
benchmark subset
save compatibility
```

Automated architecture lint should reject simulation imports of presentation packages and direct wall-clock/global random calls in authoritative packages.


# 62. Coding Rules

- idiomatic Go;
- explicit data ownership;
- no global mutable simulation state;
- no unnecessary interfaces;
- interfaces generally live at consumer boundaries;
- no generic abstraction without real consumers;
- no package named `utils`;
- avoid magic `init()` registration;
- typed enums for state machines;
- comments explain invariants/reasons;
- exported APIs have Go doc;
- hot loops target zero avoidable allocations;
- TODO format: `TODO(TASK-123): explanation`;
- never leave repository intentionally uncompilable for another agent.


# 63. Fixed-Point Types

Useful types:

```go
type Tick uint64
type EntityID uint64
type DefIndex uint32
type MilliCelsius int32
type Permille int32
type BasisPoints int32
```

Rendering may convert to float.

Authoritative cumulative simulation should use integer/fixed-point where reasonable to reduce cross-platform divergence and rounding drift.


# 64. AI Agent Organization

Recommended standing agents:

**Architecture Agent**
- system boundaries;
- ADRs;
- primitives;
- scheduler;
- cross-cutting design.

**World Agent**
- grid/chunks;
- generation;
- terrain;
- regions/rooms;
- weather/temperature/fire/plants.

**Pawn Agent**
- needs;
- skills;
- traits;
- mood;
- memory;
- social;
- health.

**AI/Jobs Agent**
- intents;
- jobs;
- work indexing;
- schedules;
- reservations.

**Navigation Agent**
- pathfinding;
- movement;
- congestion;
- region routes.

**Economy Agent**
- items;
- inventory;
- storage;
- hauling;
- recipes/trade.

**Construction Agent**
- blueprints;
- buildings;
- doors;
- zones;
- power.

**Combat Agent**
- weapons;
- targeting;
- projectiles;
- damage;
- armor;
- cover;
- enemy tactical AI.

**World Strategy Agent**
- factions;
- diplomacy;
- incidents;
- research;
- world map/travel later.

**Presentation Agent**
- renderer;
- UI;
- camera;
- input;
- overlays.

**Persistence/Mod Agent**
- save/load;
- replay;
- content;
- mods;
- localization.

**Performance/QA Agent**
- profiling;
- benchmarks;
- determinism;
- fuzz;
- CI;
- regression analysis.

**Integration Agent**
- merge parallel branches;
- resolve contracts;
- run full validation;
- publish integration handoffs.


# 65. Multi-Agent Ownership Rules

Each package should contain `AGENTS.md` describing:

```text
purpose
allowed dependencies
forbidden responsibilities
public contracts
invariants
hot paths
tests
benchmarks
```

Agents may edit another agent's owned package only when a required contract is missing, and the change should be minimal.

Shared primitive ownership should be frozen early:

```text
EntityID
Tick
Tile
ChunkCoord
DefID/DefIndex
command envelope
RNG API
component-store semantics
event/phase ordering
```

Search the repository before inventing a shared type.


# 66. AI Task Contract

Every coding assignment contains:

```text
Task ID
Objective
Allowed packages
Forbidden packages
Required input interfaces
Required output interfaces
Acceptance tests
Performance constraints
Determinism constraints
Save/mod compatibility considerations
Documentation/handoff requirement
```

Example:

```text
TASK: PAWN-014 Hunger decay

Allowed:
internal/pawn/needs
internal/sim/systems

Requirements:
- integer deterministic math
- bucketed update every 60 ticks
- no per-pawn allocation
- critical hunger event

Acceptance:
- unit tests
- determinism test
- benchmark 100k pawns
- handoff file
```


# 67. AI Handoff Contract

Every completed task creates:

```text
.cursor/handoffs/<TASK-ID>.md
```

Template:

```markdown
# Task ID
# Summary
# Files Changed
# Interfaces Added/Changed
# Assumptions
# Invariants
# Tests Run
# Benchmarks
# Determinism Impact
# Save/Mod Impact
# Known Limitations
# Follow-Up Tasks
```

Integration agent may reject a task lacking a useful handoff.


# 68. ADR Process

Cross-cutting decisions require an Architecture Decision Record.

Examples:
- discrete tick model;
- deterministic RNG;
- sparse component stores;
- Ebitengine renderer;
- hierarchical pathfinding;
- Protobuf saves;
- Starlark vs WASM;
- multiplayer model.

ADR template:

```text
Title
Status
Context
Decision
Alternatives
Consequences
Benchmarks if performance-related
Migration impact
```


# 69. Branching and Integration

Use short-lived task branches:

```text
feature/PATH-021-region-route
feature/PAWN-014-hunger
```

For multiple agents, separate Git worktrees are strongly recommended.

Commit convention:

```text
PATH-021: add cached region routing
PAWN-014: implement deterministic hunger decay
```

Agents rebase/merge frequently. Avoid long divergent branches.

Integration agent runs full tests, deterministic hashes, architecture lint, and relevant benchmark set after merging a wave.


# 70. PR Definition of Done

Every feature PR must include:
1. behavior implementation;
2. tests;
3. deterministic implications;
4. performance measurement if hot;
5. save compatibility analysis;
6. mod/content impact;
7. documentation;
8. debug visibility;
9. no untracked TODO;
10. AI handoff.

A full subsystem additionally needs:
- architecture/system document;
- property/fuzz coverage where useful;
- large-scale benchmark scenario;
- failure diagnostics;
- explicit complexity notes.


# 71. System Design Document Template

Every major subsystem file under `docs/systems/` should contain:

```text
Purpose
Owned Data
Update Frequency
Inputs
Outputs
Invariants
Algorithms
Complexity
Indexes
Caching
Dirty/Invalidation Rules
Sleeping/LOD Behavior
Determinism
Persistence
Modding
Debugging
Tests
Benchmarks
Failure Modes
```

Every new system must answer:

- What is its N at 1,000 settlers?
- How often does it update?
- Does it perform a global scan?
- What invalidates it?
- Can inactive instances sleep?
- What allocates?
- Can it batch?


# 72. MVP

First playable MVP deliberately contains:

- one generated local map;
- three settlers;
- selection/camera;
- walkable terrain;
- movement;
- A*;
- hunger/rest;
- raw food + cooked food;
- one crop;
- hauling/storage;
- work priorities;
- walls/doors/beds;
- one cooking/work station;
- construction;
- basic temperature;
- simple hostile attacker;
- one firearm + one melee weapon;
- basic wounds/downing/death;
- pause/speed;
- save/load;
- basic alerts.

Excluded from MVP: complex factions, world travel, advanced mood/social, multiplayer, animals, deep research, scripting, vehicles, multi-floor maps.


# 73. Milestones

**M0 Foundations**
- repository, CI, clock, RNG, entities, grid, chunks, headless runner, state hash, renderer spike.

**M1 Movement**
- pawn position, selection, A*, path follower, 1,000-pawn movement benchmark.

**M2 Jobs**
- job state machine, reservation system, work indexing, hauling/storage.

**M3 Survival**
- hunger, rest, beds, farming, cooking, consumption.

**M4 Construction/Environment**
- blueprints, materials, walls, doors, rooms, temperature, weather/fire basics.

**M5 Combat/Health**
- draft, ranged/melee, armor/cover basics, body damage, downing/death, 100v100 deterministic test.

**M6 Story/Economy**
- incidents, wealth, raids, trade, research, initial mood/memories/history.

**M7 Persistence/Mods**
- versioned saves, autosave snapshots, content definitions, manifests, patches, localization.

**M8 Scale Alpha**
- hierarchical navigation, chunk sleeping, staggered systems, performance dashboard, 1,000-pawn benchmark.


# 74. First Engineering Backlog

Suggested dependency-ordered tasks:

```text
ARCH-001 repository boundaries
ARCH-002 dependency rules + AGENTS templates
SIM-001 Tick
SIM-002 simulation clock
SIM-003 deterministic RNG
SIM-004 command queue
SIM-005 phase scheduler
SIM-006 state hash
SIM-007 scheduled events
SIM-020 EntityID allocator
SIM-021 lifecycle queues
SIM-022 component store
WORLD-001 Tile/Grid
WORLD-002 ChunkCoord/chunks
WORLD-003 tile flags
WORLD-004 terrain registry
WORLD-005 seeded map generation
WORLD-006 spatial index
UI-001 Ebitengine spike
UI-002 terrain renderer
UI-003 camera
UI-004 selection
PAWN-001 pawn definition/spawn
PATH-001 movement costs
PATH-002 A* heap/scratch
PATH-003 deterministic A*
PATH-004 path follower
PATH-005 benchmark
AI-001 JobID/lifecycle
AI-002 reservation manager
AI-003 MoveTo job
AI-004 intent selector
AI-005 work indexes
ECON-001 item defs
ECON-002 stack entities
ECON-003 inventory
ECON-004 storage index
ECON-005 transfer
ECON-006 hauling
PAWN-010 needs store
PAWN-011 hunger
PAWN-012 rest
BUILD-001 building defs
BUILD-002 blueprints
BUILD-003 resource delivery
BUILD-004 construction
BUILD-005 wall/door
SAVE-001 Protobuf schema
SAVE-002 minimal save/load
TEST-001 sim test DSL
PERF-001 benchmark harness
PERF-002 pprof/tick profiler
```


# 75. Recommended Parallel Work Waves

**Wave 1 — Freeze foundations**
- Architecture: clock/RNG/entities/scheduler;
- World: grid/chunks/terrain;
- Presentation: Ebitengine proof-of-concept;
- QA: CI/state hash/benchmark harness.

**Wave 2**
- Navigation: A* + movement;
- Pawn: pawn/needs data;
- Persistence: save schema;
- Content: definition loader.

**Wave 3**
- AI: jobs/reservations/work indexes;
- Economy: items/inventory/storage;
- Building: blueprint/construction;
- Presentation: inspector/commands.

Do not start aggressive parallelization until shared primitives and ownership contracts compile and have tests.


# 76. Reference AI Coding Prompt

```text
You are implementing rimworld-go.

Read:
- .cursor/MASTER_SPEC.md
- package AGENTS.md
- relevant docs/systems/*
- accepted ADRs
- recent handoffs for dependencies

Task ID: PATH-021
Objective: implement local deterministic A*.

Allowed packages:
- internal/ai/pathing
- pkg/geom

Forbidden:
- pawn internals
- renderer
- combat

Constraints:
- no global RNG
- no wall clock
- integer movement costs
- reusable scratch buffers
- deterministic tie breaking
- explicit no-path result
- no unrelated refactors

Acceptance:
- unit tests
- random-map property/fuzz coverage
- 512×512 benchmark
- deterministic path result
- handoff PATH-021.md

Before coding:
1. search for existing primitives;
2. summarize relevant interfaces;
3. propose only necessary contract changes.

After coding:
1. run tests;
2. run benchmark;
3. list changed public contracts;
4. report limitations;
5. write handoff.
```


# 77. AI Guardrails

All AI agent prompts should include:

- Do not delete tests to make CI green.
- Do not weaken invariants silently.
- Do not replace deterministic logic with wall-clock randomness.
- Do not add goroutine-per-entity designs.
- Do not introduce global mutable simulation state.
- Do not broadly refactor outside task scope.
- Do not invent duplicate primitives before repository search.
- Do not change public contracts without documentation.
- Do not copy proprietary game content/code/assets.
- Do not commit generated binaries.
- For bugs: write failing regression first.
- For performance work: benchmark before and after.


# 78. Architectural Anti-Patterns

Explicitly reject by default:

```text
one goroutine per pawn
channel per gameplay relationship
full-map scan per tick
every pawn scans every job
every pawn checks every pawn socially
global tile A* for every long path
per-tile thermal fluid simulation
global room rebuild every tick
global power rebuild every tick
reflection-heavy generic ECS before real need
service-locator GameContext containing everything
runtime Go plugins from untrusted mods
renderer-owned authoritative state
gob as permanent public save schema
```


# 79. Scalability Algorithms by System

Target shapes:

```text
work: pawns × local indexed candidates, not pawns × all jobs
social: active interaction edges, not all pawn pairs
combat: spatial nearby enemies, not all combatants × all combatants
fire: active burning frontier, not map scan
plants: bucket/analytical, not per-tick per plant
temperature: thermal regions/boundaries
rooms: invalidation near geometry changes
power: topology change rebuild only
items: indexes by def/region/storage/haulable state
incidents: periodic evaluation
factions: coarse world ticks
UI: incremental aggregates + virtualized tables
```


# 80. Large-Colony Product Design

A technically scalable simulation is insufficient if management does not scale.

At high population, player operates through:
- groups;
- professions/roles;
- bulk priorities;
- policy inheritance;
- templates;
- automated stock thresholds;
- squad commands;
- aggregated alerts;
- searchable settlers;
- colony dashboards.

Individual characters remain important through history, relationships, significant events, health crises, exceptional skills, leadership, and player favorites.


# 81. Automated Balance Harness

Headless simulation should support batches:

```bash
rimworld-go simulate-batch   --scenario frontier   --seeds 1000   --ticks 500000
```

Collect:
- survival time;
- starvation frequency;
- food production/consumption;
- construction throughput;
- deaths/cause;
- combat outcomes;
- wealth curves;
- mood;
- population;
- path/job costs.

Provide scripted colony-policy bots for regression/balance—not as player-facing AI. This allows thousands of unattended simulations and makes tuning evidence-based.


# 82. Benchmark Scenarios

Permanent fixtures:

```text
bench-empty-2048
bench-pawns-100
bench-pawns-1000
bench-hauling-10000
bench-items-100000
bench-plants-100000
bench-path-open
bench-path-maze
bench-rooms-10000
bench-fire-large
bench-combat-100v100
bench-raid-1000
```

Machine-readable output includes commit, scenario, ticks, ns/tick, allocations/tick, max heap, and subsystem timing.


# 83. Save and Replay Regression Corpus

Keep historical save fixtures:

```text
testdata/saves/v1/
testdata/saves/v2/
```

Every significant deterministic/system bug can add a replay:

```text
testdata/replays/PATH-198-door-deadlock.rwgr
```

CI verifies old supported saves load and fixed bug replays no longer regress.


# 84. Crash Reports

Crash diagnostics should capture:
- game/build version;
- commit;
- Go/runtime version;
- OS;
- seed;
- tick;
- mods/content hash;
- recent commands;
- state hash;
- panic + stack;
- performance counters;
- optional recovery snapshot.

No user data should be sent externally without explicit consent.


# 85. Accessibility

Plan early for:
- UI scaling;
- keyboard navigation;
- configurable controls;
- high contrast;
- colorblind-safe overlays;
- screen-shake toggle;
- pause-on-alert options;
- subtitles;
- readable font choices.

Accessibility belongs to presentation and must not contaminate authoritative simulation semantics.


# 86. Licensing and Clean-Room Policy

Choose project and asset licenses deliberately. Plausible code choices: MPL-2.0, GPL-3.0, AGPL-3.0, Apache-2.0, or MIT depending on desired reciprocity. Assets can use separate licenses.

Before commercial distribution, get legal advice.

Contributors must not:
- decompile/reference proprietary implementation;
- copy code;
- extract game data/assets;
- copy dialogue/descriptions;
- duplicate exact balancing databases.

Public final branding should be original. `rimworld-go` is best treated as a working engineering codename; a public title should avoid implying affiliation with another trademark.


# 87. Product Differentiators Beyond Inspiration

The project should become its own product through:

1. 1,000+ settler scalability;
2. much larger maps;
3. open-source engine;
4. transparent benchmarks;
5. deterministic replay;
6. headless simulation;
7. first-class mod validation/tooling;
8. large-colony automation;
9. groups/roles/policies built for scale;
10. future server/multiplayer readiness;
11. content authoring tools;
12. inspectable simulation/debug overlays.


# 88. Future Multiplayer Readiness

Multiplayer is not MVP, but these foundations make it possible:

- deterministic ticks;
- command serialization;
- stable IDs;
- content hashes;
- state hashes;
- headless simulation;
- replay;
- snapshotting.

Likely future model:
- server authoritative simulation;
- clients submit commands;
- server publishes deltas/snapshots.

Do not add networking complexity before single-player simulation is stable.


# 89. Future World Expansion

Potential stretch features:
- multiple settlements/maps;
- caravans/world travel;
- larger faction strategy;
- underground;
- multi-floor maps;
- vehicles;
- logistics conveyors/pipes;
- worker robots;
- genetic systems;
- space travel;
- planetary endgames.

Off-map simulation should remain aggregate wherever individual fidelity is not currently observable/relevant.


# 90. Immediate Execution Order

Do not begin by building mood, diplomacy, raids, advanced crafting, and mods simultaneously.

First establish:

```text
repository + CI
→ Tick
→ deterministic RNG
→ EntityID
→ Tile/Grid/Chunk
→ component store
→ headless Simulation.Step()
→ command ordering
→ state hashing
→ renderer spike
→ pawn Position
→ deterministic A*
→ movement
→ movement benchmark
→ Job engine
→ Reservation manager
→ indexed work
```

When these contracts stabilize, parallel development becomes dramatically safer.


# 91. Final Engineering Directive

When two implementations appear reasonable, prefer the one that:

1. keeps authoritative state deterministic;
2. avoids global scans;
3. has explicit ownership;
4. has measurable complexity;
5. supports sleeping/batching/invalidation;
6. is serializable;
7. is mod-safe;
8. is testable headlessly;
9. allocates minimally in hot loops;
10. is understandable to the next human or AI agent.

The goal is not merely to write a colony game in Go. The goal is to build a colony-simulation platform whose architecture makes **large-scale systemic gameplay** practical, testable, reproducible, moddable, and maintainable by a distributed human/AI contributor ecosystem.
