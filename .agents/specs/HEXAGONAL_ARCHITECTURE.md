# rimworld-go — Hexagonal Architecture & Repository Operating Model

> **Status:** Authoritative architecture guide  
> **Audience:** Human contributors, coding agents, reviewers  
> **Index of all documents:** `.agents/README.md`
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

Domain may depend on pure utility packages such as geometry, fixed-point types, deterministic collections, and RNG primitives. These live in pkg/ and use only the standard library.

Forbidden:

```text
domain → renderer
domain → filesystem
domain → Ebitengine
domain → OS APIs
domain → network libraries
domain → CLI
```

---

# 4. Repository Structure

Real top-level layout (package detail lives in each package's `AGENTS.md`):

```text
rimworld-go/
├── cmd/                  game, simulate, benchmark, replay, contentlint, mapgen, atlasgen, audiogen, sfxgen, normalgen, rambudget
├── internal/
│   ├── domain/           ai, animal, building, combat, difficulty, economy, faction, gameplay, incident,
│   │                     pawn, raid, research, scenario, sim, storyteller, world (headless, deterministic)
│   │   └── sim/          clock, command, component, entity, event, scheduler, snapshot, statehash
│   ├── application/      game, session, commands, scenario, savegame, replay, benchmark
│   │   └── game/         the simulation owner (Game); features live in sub-packages (care, combat, craft,
│   │                     mind, prison, rest, ...) behind consumer-owned hosts (SOLID_GO §10)
│   ├── ports/
│   │   ├── inbound/      game.go, commands.go, queries.go
│   │   └── outbound/     content, persistence, pathing, metrics, asset, event_sink
│   ├── adapters/
│   │   ├── inbound/      ebiten (the client's composition root; features in sub-packages behind
│   │   │                 `ports` roles, ARCH-001…006), cli, debugui, future_net
│   │   └── outbound/     contentfs, modloader, filesave, settingsfile, scenariofile, cardpresetfile,
│   │                     slogmetrics, ebitenaudio, updatecheck, usagehttp
│   ├── platform/         assetroot, atomicfile, config, input, openfolder, sysinfo
│   ├── testkit/          simtest, fixtures, fakecontent, replaytest, deepeq
│   └── arch/             architecture and SOLID tests (solid_budget.txt)
├── pkg/                  low-level primitives with no game concepts (mathx, randx, noise*, flatyaml, msgfmt, synths, ...)
├── api/proto/  assets/  content/  mods/  scenarios/  benchmarks/  docs/  migrations/  scripts/  tools/  third_party/
└── AGENTS.md  README.md  Makefile
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
- save files → persistence port;
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

The target shape (ADR 0001, ADR 0002 in `docs/adr/`):
- **Systems.** A systems scheduler (`game/sched`) runs each system with its declared `Reads` and `Writes` component stores, its `Every`/`Stagger` period and its `After` order. The order is deterministic and starts out equal to today's phase order. A system's own state lives in its own saved store.
- **Pure stages.** A stage runs on the one `Pool` port (built in `internal/compose`, implemented in `internal/platform/pool`) over a frozen view, and returns results that the scheduler applies serially in entity-ID order. `go` statements appear only in the pool implementation.

Determinism and the no-goroutine-per-entity rule: `.agents/specs/MASTER_SPEC.md` §6–7, §55.

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
```

Other identifiers are typed per package (`skill.ID`, `trait.ID`, `recipe.ID`, `zone.ID`, …). Content definitions are consumed through the `internal/ports/outbound/content.go` port and loaded by `internal/adapters/outbound/contentfs`; there is no shared domain content package.

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

Add a primitive here only when the standard library doesn't have it (GO_BEST_PRACTICES §86). container/heap and math/rand/v2 already cover the general cases; pkg/randx wraps the latter for seeding and save state.

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

The target (ADR 0003, `docs/adr/0003-event-queue.md`):
- **Events.** Every cross-system effect is a typed event in `domain/sim/event`. Events are appended to the tick's queue and drained at fixed boundaries, after each system, in emit order. The queue is empty at every tick's end. Work that must cross ticks goes on the saved timer heap.
- **Queries.** A system reads another system's state only through a **query port**, whose methods return values and never mutate.
- **Commands.** External commands stay the entry point; the commands system turns each one into events.

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

Saves are gob + gzip, versioned by SaveVersion with one migration per step
(internal/application/game/save.go). Gob matches fields by name: renaming or
removing a field drops its data without an error. The save schema test lists
every saved field, so such a change must bump SaveVersion and add a migration.
Reconsider Protobuf only if saves must be read outside Go.

The application layer encodes and migrates the state; the persistence adapter
owns the files.

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

Inside the client the same rule repeats one level down (SOLID_GO.md §7, ARCH-004/005): the root `Renderer` fills one `render.View` per frame (camera, art, this frame's world and pawns, clocks, options) and the draw layers under `render/` (`motion`, `light`, `sprite`, `env`, `creature`, `overlay`, `cue`, `minimap`, `world`) read that view and each other's exported types, never the `Renderer`; each layer's needs of the simulation are a small consumer-owned interface (`env.Chunks`, `world.Source`) taking values, never a pointer to a frame local through an interface call. The App's features (`hud`, `inspector`, `architect`, `windows`, `orders`, …) see the shell only through the `ports` roles plus a small `Host` their `deps.go` declares, send cross-feature requests as `ports.Intent`s, and never import the root or one another; the root's `<pkg>_compat.go` holds the host adapters. The simulation reaches the root as the union `Simulation` of eight consumer groups (`sim_ports.go`), stored only at the composition root.

The target (ADR 0004, `docs/adr/0004-render-command-list.md`) is a render-command list:
- **Emit, never draw.** Layers emit commands into one per-frame arena (`render/cmdlist`) through a small emitter port, and never draw to an `*ebiten.Image` themselves. The 64-bit key is `pass | layer | sub | seq`, so sorting keeps today's draw order exactly.
- **Merge.** Adjacent commands with the same target, shader, sources, blend and uniforms merge into one draw.
- **Light in the vertices.** Per-sprite light data rides in vertex attributes.

Renderer-specific data such as sprite handles, camera transforms, screen pixels, and animation interpolation stay outside domain packages. Visual production (scale, pivots, palettes, sprites, icons, UI chrome, atlases) follows `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`; named screens, windows, overlays, and HUD layout follow `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` (the screen flow is §0). Those specs do not move presentation ownership into the domain.

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
adapters/outbound (a new package when pathing goes async)   worker pool orchestration
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

Determinism of the clock and RNG: `.agents/specs/MASTER_SPEC.md` §6–7.

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

Every `cmd/<tool>` is **one wiring-only `main.go`** (SOLID_GO.md §8a, guarded by `internal/arch/cmd_test.go`). `main` parses its arguments through a package, builds the concrete adapters and application types, hands them over as interfaces, calls one `Run` and exits. It holds no loops, switches, closures, helper functions, tests or long strings; everything else lives in a package that follows the SOLID limits.

| logic | home |
| --- | --- |
| Flags, usage text and a tool's `Run` | `internal/adapters/inbound/cli/<tool>` |
| Composition that needs many adapters (the game: window, audio, saves, content, mods, probes) | `internal/compose/<tool>`; only `cmd/` imports it (`TestOnlyCmdImportsCompose`) |
| Dev and capture scenes | `internal/application/devscene` |
| Profiling and the log file | `internal/platform/profiling`, `internal/platform/logfile` |
| Network clients a tool calls | `internal/adapters/outbound/<client>` |

The executables:

- `cmd/game`: the client. It wires the Ebitengine adapter, the application session, the simulation, content, saves and metrics through `internal/compose/game`.
- `cmd/simulate`: headless, with no renderer imports.
- `cmd/benchmark`: deterministic benchmark fixtures.
- `cmd/replay`: replays command streams and verifies state hashes.
- `cmd/contentlint`: loads and validates definitions and mods only.
- The asset generators (`audiogen`, `sfxgen`, `atlasgen`, `normalgen`, `mapgen`) and `rambudget`: their logic is in `cli/<tool>`.

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

The lint exists as a test: `internal/arch/solid_test.go` (`make arch`, run by CI with everything else) checks the import edges below plus the size limits of `SOLID_GO.md` §8. `tools/archlint` is not needed.

Rules should reject:

```text
internal/domain imports internal/adapters
internal/domain imports internal/platform
internal/domain imports Ebitengine
internal/domain imports os
internal/domain, internal/application or pkg imports a third-party module
authoritative package calls time.Now
authoritative package uses global math/rand
```

CI must run archlint.

---

# 32. AGENTS.md Root Contract

See the root `AGENTS.md` for the contract and `.agents/README.md` for the document index.

---

# 33. Package AGENTS.md Template

Use `.agents/templates/PACKAGE_AGENTS.md`.

---

# 34. Architecture Decision Records to Create First

`docs/adr/` is empty. Write an ADR only for a decision that changes a contract (a port, the save format, the tick model, the command set); everything else is a handoff (`.agents/templates/HANDOFF.md`).

---

# 35. First Implementation Skeleton

Superseded by the code. Entry points: `cmd/game/main.go` (composition root, builds `game.NewColony` / `game.NewColonyBuilder` from `internal/application/game`), `cmd/simulate` (headless), `cmd/replay`, `cmd/benchmark`, `cmd/contentlint`. Create a package when its first real behavior needs it.

---

# 36. Example Simulation Root

Superseded by the code. The simulation root is `Game` in `internal/application/game` (`Step`, `Tick`); explicit phase calls stay preferred over a plugin-like registry.

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
