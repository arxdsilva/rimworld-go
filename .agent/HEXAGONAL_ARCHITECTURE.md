# rimworld-go — Hexagonal Architecture & Repository Operating Model

> **Status:** Authoritative architecture guide  
> **Audience:** Human contributors, coding agents, reviewers, integration agents  
> **Companion documents:** `AI_TOKEN_OPTIMIZATION.md`, `GO_BEST_PRACTICES.md`, `MASTER_SPEC.md`, `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

---

# 1. Purpose

This document translates the master product specification into a concrete repository architecture based on **Hexagonal Architecture (Ports and Adapters)**.

The goal is to make `rimworld-go`:

- easy to reason about;
- testable without rendering;
- deterministic;
- modular enough for multiple AI agents;
- resistant to architecture drift;
- suitable for headless simulation;
- suitable for future multiplayer/server use;
- friendly to modding and tooling;
- optimized for large-scale simulation.

The simulation domain must not depend on presentation, persistence, transport, or platform-specific concerns.

The architecture is intentionally explicit rather than framework-heavy.

---

# 2. Hexagonal Architecture Model

The game is divided into:

```text
                         +----------------------+
                         |      Adapters        |
                         |                      |
                         | UI / Renderer / CLI  |
                         | Save / Filesystem    |
                         | Mods / Assets        |
                         | Future Networking    |
                         +----------+-----------+
                                    |
                                    | inbound/outbound ports
                                    v
                    +-------------------------------+
                    |       Application Layer       |
                    |                               |
                    | Commands / Use Cases          |
                    | Session Coordination          |
                    | Game Speed / Save Requests    |
                    | Scenario Lifecycle            |
                    +---------------+---------------+
                                    |
                                    v
                    +-------------------------------+
                    |          Domain Core          |
                    |                               |
                    | Simulation                    |
                    | Entities / Components         |
                    | World / Pawns / Jobs          |
                    | Combat / Economy              |
                    | Environment / Incidents       |
                    +---------------+---------------+
                                    |
                                    v
                    +-------------------------------+
                    |        Domain Ports           |
                    |                               |
                    | Content Lookup                |
                    | Snapshot Persistence          |
                    | Path Computation              |
                    | Metrics Sink                  |
                    | Event Output                  |
                    +-------------------------------+
```

The domain owns the abstractions it requires.

Adapters implement those abstractions.

---

# 3. Dependency Rule

The core rule:

> **Dependencies point inward.**

Allowed direction:

```text
adapters
   ↓
application
   ↓
domain
```

Domain may depend on pure utility packages such as geometry, fixed-point types, deterministic collections, and RNG primitives.

Forbidden:

```text
domain → renderer
domain → filesystem
domain → Ebitengine
domain → protobuf generated package
domain → OS APIs
domain → network libraries
domain → CLI
```

---

# 4. Repository Structure

Recommended layout:

```text
rimworld-go/
├── cmd/
│   ├── game/
│   │   └── main.go
│   ├── simulate/
│   │   └── main.go
│   ├── benchmark/
│   │   └── main.go
│   ├── replay/
│   │   └── main.go
│   └── contentlint/
│       └── main.go
│
├── internal/
│   ├── domain/
│   │   ├── sim/
│   │   │   ├── clock/
│   │   │   ├── command/
│   │   │   ├── event/
│   │   │   ├── entity/
│   │   │   ├── component/
│   │   │   ├── scheduler/
│   │   │   ├── snapshot/
│   │   │   └── statehash/
│   │   │
│   │   ├── world/
│   │   │   ├── grid/
│   │   │   ├── chunk/
│   │   │   ├── terrain/
│   │   │   ├── region/
│   │   │   ├── room/
│   │   │   ├── generation/
│   │   │   ├── temperature/
│   │   │   ├── weather/
│   │   │   ├── fire/
│   │   │   └── lighting/
│   │   │
│   │   ├── pawn/
│   │   │   ├── needs/
│   │   │   ├── health/
│   │   │   ├── skill/
│   │   │   ├── trait/
│   │   │   ├── mood/
│   │   │   ├── memory/
│   │   │   ├── social/
│   │   │   └── inventory/
│   │   │
│   │   ├── ai/
│   │   │   ├── intent/
│   │   │   ├── job/
│   │   │   ├── reservation/
│   │   │   ├── work/
│   │   │   ├── movement/
│   │   │   └── pathing/
│   │   │
│   │   ├── economy/
│   │   │   ├── item/
│   │   │   ├── storage/
│   │   │   ├── recipe/
│   │   │   ├── production/
│   │   │   └── trade/
│   │   │
│   │   ├── building/
│   │   │   ├── construction/
│   │   │   ├── door/
│   │   │   ├── zone/
│   │   │   └── power/
│   │   │
│   │   ├── combat/
│   │   │   ├── weapon/
│   │   │   ├── attack/
│   │   │   ├── projectile/
│   │   │   ├── armor/
│   │   │   ├── cover/
│   │   │   └── damage/
│   │   │
│   │   ├── faction/
│   │   ├── incident/
│   │   └── research/
│   │
│   ├── application/
│   │   ├── game/
│   │   ├── session/
│   │   ├── commands/
│   │   ├── scenario/
│   │   ├── savegame/
│   │   ├── replay/
│   │   └── benchmark/
│   │
│   ├── ports/
│   │   ├── inbound/
│   │   │   ├── game.go
│   │   │   ├── commands.go
│   │   │   └── queries.go
│   │   └── outbound/
│   │       ├── content.go
│   │       ├── persistence.go
│   │       ├── pathing.go
│   │       ├── metrics.go
│   │       ├── asset.go
│   │       └── event_sink.go
│   │
│   ├── adapters/
│   │   ├── inbound/
│   │   │   ├── ebiten/
│   │   │   ├── cli/
│   │   │   ├── debugui/
│   │   │   └── future_net/
│   │   └── outbound/
│   │       ├── contentfs/
│   │       ├── protobufsave/
│   │       ├── pathworker/
│   │       ├── slogmetrics/
│   │       ├── filesystem/
│   │       └── modloader/
│   │
│   ├── platform/
│   │   ├── config/
│   │   ├── logging/
│   │   ├── profiling/
│   │   └── buildinfo/
│   │
│   └── testkit/
│       ├── simtest/
│       ├── fixtures/
│       ├── fakecontent/
│       └── replaytest/
│
├── pkg/
│   ├── geom/
│   ├── fixed/
│   ├── randx/
│   ├── bitset/
│   └── collections/
│
├── api/
│   └── proto/
│
├── content/
├── assets/
├── mods/
├── scenarios/
├── benchmarks/
├── docs/
│   ├── architecture/
│   ├── systems/
│   ├── adr/
│   └── ai-agents/
├── tools/
├── scripts/
├── go.mod
├── go.sum
├── AGENTS.md
├── CONTRIBUTING.md
└── README.md
```

---

# 5. Why `domain`, `application`, `ports`, and `adapters`

## Domain

Owns:

- game rules;
- simulation state;
- entities;
- jobs;
- needs;
- movement;
- combat;
- economy;
- environment;
- faction/event logic.

It must remain runnable headlessly.

## Application

Owns orchestration:

- start/load a game;
- queue player commands;
- advance simulation;
- control pause/speed;
- create save snapshots;
- replay commands;
- expose read models.

Application coordinates domain objects but should not contain core game rules.

## Ports

Ports are narrow interfaces at architectural boundaries.

Examples:

- content registry;
- persistence;
- path computation service;
- metrics;
- presentation query interface.

## Adapters

Adapters translate external technology into ports.

Examples:

- Ebitengine UI → inbound application commands;
- Protobuf files → persistence port;
- filesystem content → content port;
- worker pool → pathing port.

---

# 6. Ports Are Consumer-Owned

A port should usually be defined by the code that **needs** it.

**Mandatory rule:** any code that depends on an adapter (Ebitengine, filesystem, protobuf, CLI, worker pool, future network, etc.) must depend on a narrow interface describing the *behavior* it needs, never on the adapter's concrete type. The concrete adapter type is constructed once, at the composition root (`cmd/*`, or the top of an adapter package that wires a sibling application type), and is passed around only as the interface it satisfies. This is the Dependency Inversion Principle (DIP) applied at the architecture boundary: the interface is the abstraction both sides depend on, so any adapter implementation can be swapped (real vs. fake vs. future replacement) without touching its consumers.

This also applies in the inward direction: an adapter (e.g. `internal/adapters/inbound/ebiten`) must not hold a concrete `*application.Game`-style pointer merely because that is what exists today. Define a small interface, owned by the adapter package, listing only the methods the adapter actually calls (e.g. `Step`, `Snapshot`, `Width`, `Height`). The application's concrete type satisfies it implicitly; nothing else needs to change when the interface is introduced.

Keep every such interface small and single-purpose (Interface Segregation Principle — ISP): do not force a consumer to depend on methods it never calls. Prefer several narrow interfaces over one wide one, and never add a method to an existing port "just in case."

Bad:

```go
package persistence

type GiantRepository interface {
    SaveEverything(...)
    LoadEverything(...)
    DeleteEverything(...)
}
```

Better:

```go
package savegame

type SnapshotWriter interface {
    Write(ctx context.Context, snapshot Snapshot) error
}
```

Keep interfaces small.

Do not create an interface merely because a concrete type exists — but do create one wherever a package boundary is crossed (adapter ↔ application, application ↔ domain-facing port), even if today there is only one implementation. The point is substitutability and testability, not ceremony.

---

# 7. Inbound Ports

Inbound ports describe operations external actors can request.

Example:

```go
type Game interface {
    Step(ticks int)
    Submit(CommandEnvelope) error
    Snapshot() ReadSnapshot
}
```

Potential inbound clients:

- renderer/UI;
- CLI simulation runner;
- benchmark runner;
- replay verifier;
- future network server.

All should use the same application surface where practical.

---

# 8. Outbound Ports

Examples:

```go
type ContentReader interface {
    Terrain(id DefID) (TerrainDef, bool)
}

type SnapshotWriter interface {
    Write(ctx context.Context, snapshot SaveSnapshot) error
}

type PathService interface {
    Request(PathRequest)
    Poll() []PathResult
}

type MetricsSink interface {
    ObserveTick(TickMetrics)
}
```

Adapters implement these.

---

# 9. Domain Services vs Application Services

Use domain services when logic is a game rule spanning multiple domain objects.

Use application services for orchestration.

Example:

**Domain**
```text
ResolveAttack
AssignJob
TransferItem
ApplyDamage
```

**Application**
```text
LoadGame
SaveGame
StartScenario
AdvanceFrame
VerifyReplay
```

---

# 10. Commands

Authoritative player mutations happen through domain/application commands.

Example:

```go
type CommandEnvelope struct {
    Tick     clock.Tick
    SourceID uint32
    Sequence uint64
    Payload  Command
}
```

Never allow UI code to directly mutate component stores.

Commands should be serializable and deterministic.

---

# 11. Queries and Read Models

Presentation should consume explicit read models.

Example:

```go
type PawnView struct {
    ID         EntityID
    Name       string
    Position   Tile
    HealthText string
    JobText    string
}
```

Do not hand raw mutable component stores to UI.

UI query adapters can use optimized snapshots.

---

# 12. Simulation Ownership

The authoritative simulation is mutation-owned by one goroutine.

This remains true even though Go supports cheap goroutines.

Workers may compute:

- paths;
- worldgen;
- save encoding;
- read-snapshot preparation;
- pure batch scoring.

They must not directly mutate authoritative stores.

---

# 13. Deterministic Worker Pattern

```text
simulation snapshot
→ enqueue work with sequence ID
→ workers compute
→ results collected
→ sort by sequence
→ validate revision
→ commit on simulation owner
```

This pattern is mandatory for pathfinding and similar systems.

---

# 14. Package-Level Boundaries

A package should own one coherent concept.

Prefer:

```text
internal/domain/ai/reservation
```

over:

```text
internal/domain/ai
```

containing all AI logic.

However do not fragment into one package per struct.

The architecture agent decides package granularity based on dependency direction.

---

# 15. Domain Types

Canonical ownership:

```text
Tick            internal/domain/sim/clock
EntityID        internal/domain/sim/entity
Tile            internal/domain/world/grid
ChunkCoord      internal/domain/world/chunk
JobID           internal/domain/ai/job
DefID/DefIndex  internal/domain/content or shared content primitive
FactionID       internal/domain/faction
```

Never recreate these types elsewhere.

---

# 16. Pure Utility Packages

`pkg/` is reserved for reusable low-level primitives that do not depend on game-domain packages.

Examples:

- geometry;
- fixed-point math;
- deterministic heaps;
- bitsets;
- sparse sets if generic enough;
- deterministic RNG primitives.

Do not put game concepts in `pkg/`.

---

# 17. Domain Event Rules

Typed events are permitted for decoupling.

Example:

```go
type PawnDied struct {
    Pawn EntityID
    Tick Tick
}
```

Events should:

- be explicit;
- be processed in known phases;
- avoid hidden synchronous side effects;
- have deterministic ordering.

Avoid generic `map[string]any` events.

---

# 18. State Machines

Jobs, doors, combat actions, health progression, etc. should use explicit enums.

Example:

```go
type DoorState uint8

const (
    DoorClosed DoorState = iota
    DoorOpening
    DoorOpen
    DoorClosing
)
```

Prefer visible transitions over implicit callback chains.

---

# 19. Persistence Boundary

Runtime domain types are not the save schema.

The persistence adapter translates:

```text
domain snapshot
↔ versioned persistence DTO
↔ protobuf representation
```

Generated protobuf code must stay outside the domain.

Migration logic belongs to persistence/application boundaries.

---

# 20. Content Boundary

Human-authored content:

```text
YAML/JSON
```

is loaded by an adapter.

The domain receives validated immutable definitions.

Runtime content references should use compact indexes where hot.

The domain must not parse YAML files.

---

# 21. Renderer Boundary

The renderer receives:

```text
RenderSnapshot
UIReadModel
```

It sends:

```text
CommandEnvelope
UI-only intent
```

Renderer never calls entity stores to mutate game state.

Renderer-specific data such as sprite handles, camera transforms, screen pixels, and animation interpolation stay outside domain packages.

---

# 22. Pathfinding Boundary

Pathfinding has two possible forms:

1. domain-owned deterministic algorithm package;
2. outbound port + worker adapter for asynchronous execution.

The algorithm itself may live inside domain because pathfinding is a game mechanic.

Parallel execution is an adapter/infrastructure concern.

Recommended split:

```text
domain/ai/pathing      pure deterministic solver
adapters/outbound/pathworker   worker pool orchestration
```

---

# 23. Time Boundary

Simulation time uses `Tick`.

Wall-clock time belongs to:

- UI frame pacing;
- logging timestamps;
- autosave request cadence;
- profiling.

Wall-clock must not influence authoritative game outcomes.

---

# 24. Error Taxonomy

Separate:

**Expected domain result**
```text
JobFailedPathBlocked
ReservationUnavailable
TargetForbidden
```

**Validation errors**
```text
invalid definition
invalid command
```

**Infrastructure errors**
```text
disk full
corrupt save
asset read failure
```

Do not represent ordinary gameplay outcomes as Go errors unless they truly signal exceptional failure.

---

# 25. Context Usage

Use `context.Context` at I/O and long-running operation boundaries:

- save;
- load;
- content load;
- world generation;
- benchmark task;
- future network calls.

Do not thread `context.Context` through every per-tick domain function.

---

# 26. Dependency Injection

Prefer explicit constructors.

Example:

```go
func NewSession(
    sim *sim.Simulation,
    content ContentReader,
    saves SnapshotWriter,
    metrics MetricsSink,
) *Session
```

Avoid dependency injection frameworks and service locators.

---

# 27. Initialization Flow

```text
platform config
→ logging/profiling
→ discover mods
→ load/validate content
→ create frozen content registry
→ create adapters
→ create simulation
→ create application/session
→ create inbound adapter (game UI/CLI)
→ run
```

---

# 28. Main Executables

## `cmd/game`

Wires:

```text
Ebitengine adapter
application session
simulation
content filesystem
save adapter
metrics
```

## `cmd/simulate`

No renderer imports.

Wires:

```text
CLI
application simulation runner
domain
content
optional save output
```

## `cmd/benchmark`

Runs deterministic benchmark fixtures.

## `cmd/replay`

Replays command streams and verifies state hashes.

## `cmd/contentlint`

Loads and validates definitions/mods only.

---

# 29. Testing Architecture

Domain tests should require no rendering/filesystem.

Prefer:

```text
domain unit test
→ in-memory fake ports
```

Application tests use fake outbound adapters.

Adapter tests validate technology translation separately.

Integration tests wire real adapters only where necessary.

---

# 30. Test Doubles

Good fakes:

- in-memory content;
- in-memory persistence;
- deterministic path result adapter;
- fake metrics sink.

Avoid mocking every method call.

Use real domain objects whenever cheap.

---

# 31. Architecture Lint

Create `tools/archlint`.

Rules should reject:

```text
internal/domain imports internal/adapters
internal/domain imports internal/platform
internal/domain imports Ebitengine
internal/domain imports os
internal/domain imports protobuf generated code
authoritative package calls time.Now
authoritative package uses global math/rand
```

CI must run archlint.

---

# 32. AGENTS.md Root Contract

Root `AGENTS.md` should state:

```text
Read MASTER_SPEC.md.
Read HEXAGONAL_ARCHITECTURE.md.
Read GO_BEST_PRACTICES.md.
Read AI_TOKEN_OPTIMIZATION.md.

Do not violate dependency direction.
Search before creating shared primitives.
Do not mutate simulation from adapters.
Do not introduce global mutable state.
Do not add goroutine-per-entity designs.
Run tests and relevant benchmarks.
Write task handoff.
```

---

# 33. Package AGENTS.md Template

```markdown
# Package Purpose

# Owned Concepts

# Allowed Dependencies

# Forbidden Dependencies

# Public Contracts

# Invariants

# Hot Paths

# Determinism Rules

# Save/Mod Impact

# Tests

# Benchmarks

# Known Design Constraints
```

---

# 34. Architecture Decision Records to Create First

Recommended initial ADRs:

```text
ADR-0001 Hexagonal architecture
ADR-0002 Single-owner deterministic simulation
ADR-0003 Discrete tick model
ADR-0004 Deterministic RNG
ADR-0005 Entity/component storage
ADR-0006 Chunked world
ADR-0007 Hierarchical pathfinding
ADR-0008 Command-based mutations
ADR-0009 Snapshot-based rendering
ADR-0010 Protobuf persistence schema
ADR-0011 Content definitions and runtime indexing
ADR-0012 Async worker ordered-commit model
```

---

# 35. First Implementation Skeleton

Start with these packages:

```text
internal/domain/sim/clock
internal/domain/sim/entity
internal/domain/sim/command
internal/domain/sim/statehash
internal/domain/world/grid
internal/domain/world/chunk
internal/application/game
internal/ports/inbound
internal/ports/outbound
internal/adapters/inbound/cli
internal/testkit/simtest
```

Do not create empty package trees all at once.

Create packages when the first real behavior requires them.

---

# 36. Example Simulation Root

```go
type Simulation struct {
    tick clock.Tick

    entities *entity.World
    commands command.Queue

    // explicit subsystem state
}

func (s *Simulation) Step() {
    s.applyCommands()
    s.spawnPhase()
    s.needsPhase()
    s.intentPhase()
    s.jobsPhase()
    s.movementPhase()
    s.combatPhase()
    s.healthPhase()
    s.environmentPhase()
    s.cleanupPhase()
    s.tick++
}
```

Explicit phase calls are preferred over a generic plugin-like system registry at first.

---

# 37. Architecture Smells

Flag for review:

- domain interfaces with 20+ methods;
- giant `GameContext`;
- adapters returning raw domain internals;
- UI package imported by game logic;
- generated serialization code in domain;
- `any`-heavy APIs;
- pervasive reflection;
- global registries;
- package cycles;
- `utils`;
- hidden goroutine mutation;
- interface per struct;
- manager/service naming without clear ownership.

---

# 38. Migration Policy

If architecture must change:

1. write/update ADR;
2. update ports;
3. update all call sites in same PR where practical;
4. update package `AGENTS.md`;
5. update tests;
6. update handoff.

Do not let architecture drift via many undocumented “temporary” exceptions.

---

# 39. Definition of Architecture Compliance

A feature is architecture-compliant when:

- domain contains game rules;
- application contains orchestration;
- adapters contain technology;
- dependencies point inward;
- all authoritative mutation is command/system owned;
- external concerns enter through ports;
- test can run headlessly;
- deterministic behavior is preserved;
- package ownership remains explicit;
- public contracts are documented.

---

# 40. Final Directive

Hexagonal architecture is not an excuse to create needless abstractions.

Use it to preserve **direction of dependencies and testability**, not to make every function pass through five interfaces.

Prefer concrete domain code with small ports at real boundaries.

The architecture should make the game easier to understand, benchmark, simulate, and extend—not more ceremonial.
