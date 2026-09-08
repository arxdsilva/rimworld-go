# rimworld-go — Go Best Practices & Engineering Standard

> **Status:** Mandatory coding standard  
> **Audience:** All human and AI contributors  
> **Goal:** Ensure idiomatic, maintainable, performant, deterministic Go throughout the project.

---

# 1. Guiding Principle

Write Go that is:

- simple;
- explicit;
- idiomatic;
- benchmarkable;
- deterministic where authoritative;
- allocation-conscious in hot loops;
- easy to test;
- easy for the next engineer or AI agent to understand.

Avoid “enterprise Go” ceremony.

---

# 2. Formatting and Static Analysis

Every change must pass:

```bash
gofmt -w .
go vet ./...
staticcheck ./...
go test ./...
```

CI should enforce formatting rather than rely on reviewers.

Use `goimports` if the project standardizes on it.

---

# 3. Package Naming

Use short, descriptive, lowercase package names.

Good:

```text
entity
pathing
reservation
damage
storage
clock
```

Bad:

```text
utils
helpers
common
manager
misc
core2
```

Avoid stutter.

Bad:

```go
package entity
type EntityManager struct{}
```

Better:

```go
package entity
type World struct{}
```

---

# 4. Export Only What Is Needed

Default to unexported identifiers.

Export when another package has a legitimate architectural reason to use the symbol.

Smaller APIs:
- reduce coupling;
- reduce documentation burden;
- reduce AI context;
- make refactoring safer.

---

# 5. Interfaces

Use interfaces at real behavioral boundaries.

Prefer small consumer-defined interfaces.

This is SOLID's Dependency Inversion Principle (depend on an abstraction owned by the consumer, not on a concrete implementation) combined with the Interface Segregation Principle (no consumer should depend on methods it does not call). At every adapter/application/domain boundary, the caller defines the minimal interface it needs and the concrete type is only ever constructed at the composition root. See `.agent/HEXAGONAL_ARCHITECTURE.md` §6 for the enforced project-wide rule.

Good:

```go
type WalkabilityReader interface {
    IsWalkable(grid.Tile) bool
}
```

Avoid:

```go
type WorldService interface {
    GetPawn(...)
    SetPawn(...)
    FindPath(...)
    Save(...)
    Load(...)
    ...
}
```

Do not create `Fooer` interfaces for every concrete type preemptively.

---

# 6. Accept Interfaces, Return Concrete Types

As a general default:

```go
func NewService(dep Dependency) *Service
```

not:

```go
func NewService(dep Dependency) ServiceInterface
```

Return interfaces only when the API genuinely requires abstraction.

---

# 7. Constructors

Constructors should establish invariants.

```go
func NewQueue(capacity int) *Queue
```

If zero value is valid, consider whether a constructor is needed.

Do not create constructors that only do:

```go
return &T{}
```

without adding clarity/invariants.

---

# 8. Zero Values

Prefer useful zero values when natural.

Examples:
- empty queues;
- counters;
- optional flags.

Do not force a zero value to be valid if that weakens invariants.

---

# 9. Error Handling

Use errors for exceptional/operation failures.

Expected gameplay results should use domain result types.

Bad:

```go
return errors.New("pawn is hungry")
```

Good:

```go
return JobResult{Status: Failed, Reason: MissingFood}
```

Infrastructure errors should wrap context:

```go
return fmt.Errorf("load save %q: %w", path, err)
```

Use `%w`.

---

# 10. Sentinel and Typed Errors

Use sentinel/typed errors only when callers need programmatic branching.

Do not create large error hierarchies.

Prefer:

```go
errors.Is
errors.As
```

where appropriate.

---

# 11. Context

Use `context.Context` for:
- I/O;
- cancellation;
- long-running application/infrastructure operations.

Context should be the first parameter:

```go
func Load(ctx context.Context, ...)
```

Do not store contexts in structs.

Do not pass context through every pure simulation function.

---

# 12. Concurrency

Go's goroutines are not free architecture.

Rules:

- no goroutine per pawn;
- no goroutine per job;
- one owner mutates authoritative simulation state;
- workers compute from immutable snapshots;
- results are committed deterministically.

Use channels where ownership boundaries benefit from them.

Do not use channels as a replacement for ordinary function calls.

---

# 13. Mutexes

Prefer ownership over shared mutation.

When mutexes are needed:
- keep protected state small;
- document lock ordering if multiple locks;
- do not hold locks across I/O;
- do not expose internal slices/maps while locked state can mutate.

Run race tests on concurrent adapters/workers.

---

# 14. Deterministic Go

Authoritative code must avoid:

```go
time.Now()
math/rand global functions
Go map iteration for decisions
goroutine completion order
runtime.NumCPU() affecting game behavior
filesystem order
```

Sort keys when order matters.

Use explicit RNG/context.

---

# 15. Maps

Maps are good for sparse/random lookup.

Do not use maps by habit in hot component loops.

If deterministic iteration is required:

```go
keys := make(...)
sort.Slice(keys, ...)
```

Better yet, use ordered/dense structures if they match the domain.

---

# 16. Slices

Prefer slices for dense ordered data.

Preallocate when capacity is known:

```go
items := make([]Item, 0, expected)
```

Reuse scratch slices in hot loops.

Avoid retaining huge backing arrays accidentally.

---

# 17. Struct Layout

Hot structs should be compact.

Be aware of alignment/padding.

Do not micro-pack everything prematurely, but inspect hot component sizes with tools/tests.

Prefer IDs/indices over pointer graphs in hot data.

---

# 18. Pointer vs Value

Use values for:
- small immutable/value-like types;
- IDs;
- coordinates;
- fixed-point quantities.

Use pointers when:
- mutation identity matters;
- struct is large;
- copying is undesirable.

Avoid pointers to tiny scalar-like structs without reason.

---

# 19. Entity References

Use stable IDs instead of pointers between simulation entities.

Good:

```go
Target EntityID
```

Avoid:

```go
Target *Pawn
```

Stable IDs simplify:
- deletion;
- save/load;
- replay;
- deterministic hashing.

---

# 20. Generics

Use generics when they clearly reduce duplication.

Good candidates:
- sparse component store;
- heap;
- bitset helpers;
- small collection primitives.

Avoid generic “architecture frameworks”.

Concrete code is easier to debug and optimize.

---

# 21. Reflection

Avoid reflection in hot paths.

Do not build the simulation around reflection-driven ECS registration.

Reflection may be acceptable in tooling/serialization adapters when justified.

---

# 22. `any`

Avoid `any` in domain APIs unless truly dynamic data is part of the model.

Prefer typed events, typed commands, typed definitions.

Bad:

```go
map[string]any
```

for core gameplay messages.

---

# 23. Enums

Use typed integer enums.

```go
type JobState uint8
```

Keep zero value meaningful where possible.

Use `String()` for diagnostics only if useful.

Do not use strings in hot state machines.

---

# 24. IDs

Canonical ID types should be distinct where semantic mistakes are possible.

Examples:

```go
type EntityID uint64
type JobID uint64
type FactionID uint32
```

Do not over-wrap every integer if it creates excessive conversion noise.

---

# 25. Fixed-Point Arithmetic

For deterministic cumulative simulation prefer integer/fixed-point.

Examples:

```go
type MilliCelsius int32
type Permille int32
type BasisPoints int32
```

Document range and units.

Avoid hidden unit mixing.

---

# 26. Constants

Use typed constants for meaningful domain values.

Avoid magic numbers in hot logic.

Balance values should come from validated content where practical.

---

# 27. Time

Simulation duration = ticks.

Wall-clock duration = `time.Duration`.

Never mix them.

Name parameters clearly:

```go
elapsedTicks
realTimeout
```

---

# 28. Randomness

No global RNG.

Preferred API should make randomness explicit:

```go
roll := rng.Roll(stream, tick, entityID, operation)
```

The exact API is an ADR-level decision.

Randomness identity should survive unrelated code changes when possible.

---

# 29. Allocation Policy

Hot per-tick code should strive for zero avoidable allocations.

Watch for:
- `fmt.Sprintf`;
- closures;
- temporary maps;
- repeated `append` growth;
- interface boxing;
- `[]byte(string)` conversions;
- reflection;
- `defer` in extremely hot tiny functions if benchmark shows cost.

Do not sacrifice readability without evidence.

---

# 30. Pools

Use `sync.Pool` only for temporary reusable objects with demonstrated allocation pressure.

Do not use pools for long-lived ownership.

Benchmark first.

---

# 31. `defer`

Use `defer` normally for:
- unlock/close;
- cleanup;
- tracing scope.

In ultra-hot loops, benchmark before avoiding it.

Never manually duplicate cleanup solely from folklore.

---

# 32. String Handling

Do not use human-readable string IDs in hot loops if compact indexes exist.

Content load:

```text
"core.wall_wood" → DefIndex(42)
```

UI/debug may map back to string.

---

# 33. Logging

Use structured logging (`log/slog` recommended).

Good:

```go
logger.Warn("job failed",
    "pawn", pawnID,
    "job", jobID,
    "reason", reason,
    "tick", tick,
)
```

Do not log every tick in production.

Trace/debug logging must be selectively enabled.

---

# 34. Panics

Panics are acceptable for:
- impossible programmer invariants;
- development-only corruption detection.

Do not panic for ordinary bad input or gameplay outcomes.

Infrastructure should return errors.

Production crash path may attempt recovery snapshot.

---

# 35. Assertions

Go has no built-in assertion.

Implement targeted invariant checks in dev/test builds where needed.

Avoid scattering generic `assert()` helpers everywhere.

Make invariant failures informative.

---

# 36. Comments

Comments explain:
- why;
- invariant;
- complexity;
- determinism;
- subtle edge cases.

Bad:

```go
// increment i
i++
```

Good:

```go
// Tie-break by EntityID so worker scheduling cannot affect movement order.
```

---

# 37. Doc Comments

Exported API comments should begin with identifier name where standard Go linting expects it.

Keep comments concise.

Architecture belongs in docs, not 100-line function comments.

---

# 38. Function Size

Prefer focused functions.

Large orchestration functions can be explicit if they show phase order.

Do not mechanically enforce tiny functions.

A 40-line clear state transition may be better than 8 one-line helpers.

---

# 39. Parameters

If a function takes many related parameters, consider a request struct.

Good:

```go
type AttackRequest struct {
    Attacker EntityID
    Target   EntityID
    Weapon   DefIndex
    Tick     Tick
}
```

Do not create request structs for trivial two-argument functions.

---

# 40. Options Pattern

Use functional options sparingly.

They can obscure required invariants.

Prefer explicit config structs for complex constructors.

---

# 41. Configuration

Application/platform config structs should be explicit and validated.

Do not read environment variables deep inside domain packages.

Load config at the edge.

---

# 42. File I/O

Domain does not call filesystem APIs.

Adapters own:
- files;
- directories;
- compression;
- protobuf encoding;
- mod discovery.

---

# 43. Serialization

Runtime structs are not persistence contracts.

Use explicit DTO translation.

Generated protobuf code should not leak into domain signatures.

---

# 44. Testing

Tests should:
- be deterministic;
- name behavior;
- avoid flaky sleeps;
- avoid real time where simulation ticks suffice;
- use table tests when cases naturally share structure;
- use integration fixtures for multi-system behavior.

---

# 45. Test Naming

Examples:

```go
func TestReservation_RejectsExclusiveConflict(t *testing.T)
func TestHunger_DecaysAtBucketTick(t *testing.T)
func TestPath_AvoidsImpassableDoor(t *testing.T)
```

Names should describe behavior.

---

# 46. Table Tests

Good when:
- same behavior;
- many input/output cases.

Bad when every row needs unrelated setup and assertions.

Do not use table tests ritualistically.

---

# 47. Subtests

Use for clear grouping and failure diagnostics.

Parallel subtests must not share mutable state.

---

# 48. Fuzzing

Good targets:
- serialization round trips;
- pathfinding invariants;
- item transfer quantities;
- content parser;
- command validation;
- geometry;
- save migration.

Use Go fuzzing where it yields real value.

---

# 49. Benchmarks

Hot systems need benchmarks.

Examples:

```go
func BenchmarkPathfinding512(b *testing.B)
func BenchmarkNeeds100k(b *testing.B)
func BenchmarkJobSearch1000(b *testing.B)
```

Use `b.ReportAllocs()`.

Do not optimize without a baseline.

---

# 50. Benchmark Correctness

Benchmarks must not benchmark dead-code elimination or irrelevant setup.

Separate setup from timed work:

```go
b.ResetTimer()
```

where appropriate.

---

# 51. Profiling

Use:
- `pprof`;
- execution trace;
- benchmark comparison tools;
- heap profiles.

Optimize algorithmic behavior before micro-level instruction tweaks.

---

# 52. Complexity Documentation

Hot systems should document expected complexity.

Examples:

```text
Job candidate query: O(local candidates)
Region route: O(regions log regions)
Component iteration: O(component count)
```

Any expected O(N²) behavior needs justification.

---

# 53. Data-Oriented Design

Prefer:

```text
dense component arrays
stable IDs
compact records
spatial indexes
bit flags
```

over:

```text
deep pointer object graph
inheritance-like interfaces
per-object goroutines
```

Go composition works well with explicit stores.

---

# 54. Sparse Set

If using a generic sparse-set component store:
- stable entity lookup;
- dense iteration;
- no hidden map allocation;
- safe removal via swap/delete or ordered semantics;
- deterministic behavior documented.

If swap-remove changes iteration ordering, ensure game outcomes do not depend on that ordering or sort/tie-break explicitly.

---

# 55. Iteration Ordering

Never assume component storage iteration order is meaningful unless contract says so.

Order-sensitive resolution should use explicit deterministic keys.

Example:
- movement conflicts sorted by priority + EntityID;
- worker results sorted by sequence.

---

# 56. Avoid Hidden Work

Functions named like getters should not:
- perform global scans;
- allocate heavily;
- trigger mutation;
- do I/O.

Use names reflecting expensive operations:

```go
FindNearest
RebuildRegions
ComputePath
```

---

# 57. Cache Invalidation

Cached values need explicit invalidation ownership.

Document:
- cache source;
- dirty trigger;
- rebuild phase;
- revision.

Avoid “cache until it looks stale” logic.

---

# 58. Revisions

Use revision counters for:
- navigation;
- room topology;
- content snapshot;
- render chunk state.

Revision numbers simplify cache validation and deterministic worker results.

---

# 59. Atomic Operations

`sync/atomic` should not be sprinkled into domain code.

Use only at clear concurrency boundaries, e.g. snapshot pointer swap or metrics.

Simulation ownership should eliminate most atomics.

---

# 60. Build Tags

Use build tags sparingly for:
- dev diagnostics;
- platform-specific adapters.

Do not fork core gameplay logic by build tags.

---

# 61. Unsafe

`unsafe` is forbidden by default.

Any introduction requires:
- ADR;
- benchmark proof;
- correctness tests;
- clear portability analysis.

---

# 62. cgo

Avoid cgo in domain/application.

Presentation dependencies may use it if unavoidable, but renderer selection should consider cross-platform build complexity.

---

# 63. Protobuf

Keep `.proto` files under a dedicated API/schema location.

Never hand-edit generated `.pb.go`.

Do not feed generated code to AI agents unless debugging generator output.

---

# 64. YAML/JSON Content

Validate at load.

Convert to strongly typed domain definitions.

Do not keep untyped maps flowing through gameplay.

---

# 65. File Paths

Use `filepath` for OS paths.

Normalize and validate mod paths.

Reject traversal.

---

# 66. Security

Treat saves/mods as untrusted.

Check:
- lengths;
- counts;
- nesting;
- path safety;
- decompression limits;
- schema version;
- dependency cycles.

Avoid arbitrary code execution.

---

# 67. Performance-Friendly API Design

Good:

```go
func (s *Store) Get(id EntityID) (*Component, bool)
```

or value/reference patterns appropriate to ownership.

Avoid APIs that construct large temporary slices on every query.

Prefer caller-provided scratch buffers where benchmarks justify it.

---

# 68. Iterator APIs

Be cautious with closure-based iterator APIs in hot loops because of allocation/escape risk.

Benchmark.

Sometimes explicit indexed loops are best.

---

# 69. Escape Analysis

For hot code, inspect with:

```bash
go build -gcflags='-m=2'
```

when allocation cause is unclear.

Do not optimize based solely on escape-analysis output without runtime benchmark.

---

# 70. Memory Retention

Beware of:
- slicing huge backing arrays;
- caches with no eviction;
- maps retaining deleted keys;
- pooled objects retaining references.

Long-running simulation requires heap stability tests.

---

# 71. Map Capacity

Where map size is known approximately:

```go
make(map[K]V, n)
```

can reduce growth allocations.

Only relevant in non-hot or initialization-heavy areas where maps are appropriate.

---

# 72. Copying Slices

When exposing read-only snapshots, do not leak mutable backing arrays.

Options:
- clone;
- immutable snapshot ownership;
- copy-on-publish;
- double buffer.

Choose based on benchmark and ownership.

---

# 73. API Ownership

Do not put interfaces in `pkg/common`.

The consumer package defines what it needs.

This keeps hexagonal ports narrow.

---

# 74. Naming Services

Avoid meaningless suffixes:

```text
Manager
Helper
Processor
Handler
Service
```

unless the concept truly matches.

Prefer domain language:

```text
Scheduler
ReservationTable
Pathfinder
Director
Store
Registry
```

---

# 75. Domain Language

Use consistent game vocabulary in code.

Do not introduce three words for the same concept.

Canonical names belong in architecture docs.

---

# 76. State Transitions

Centralize important state transitions.

Example:

```text
Alive → Downed → Dead
```

should not be mutated ad hoc from five packages.

Use explicit transition functions/events.

---

# 77. Validation at Boundaries

Validate untrusted/external input once at boundary.

Inside domain, rely on established invariants where reasonable.

Avoid repeated defensive validation in every hot function.

---

# 78. Command Validation

Player command handling should:
1. parse adapter input;
2. construct typed command;
3. validate application/domain constraints;
4. enqueue;
5. apply at deterministic phase.

Never let UI-specific types into domain.

---

# 79. Save Migration

Migration code must be version explicit.

Never infer migration solely from missing fields if schema version exists.

Test historical fixtures.

---

# 80. Backward Compatibility

Before 1.0, internal APIs may evolve quickly.

Still:
- update call sites atomically;
- update docs;
- update handoff;
- keep save migration policy deliberate.

---

# 81. Logging and Determinism

Logging can include wall-clock timestamps externally.

But logging calls must not influence RNG, ordering, or state.

Do not parse logs back into authoritative behavior.

---

# 82. Metrics and Determinism

Metrics collection must be observational.

Do not change simulation behavior based on real-time CPU load.

No frame-time-dependent skipping of authoritative work.

---

# 83. Game Speed

Fast-forward means “process more ticks”, not “change simulation rules”.

Slow machines produce same outcomes as fast machines.

---

# 84. Renderer Interpolation

Renderer may interpolate positions with floats between fixed simulation states.

Interpolation never feeds back into authoritative position.

---

# 85. Stable Hashing

State hash traversal must:
- use stable entity ordering;
- stable component ordering;
- stable map key ordering;
- fixed numeric encoding.

Never hash raw pointer addresses or Go map iteration.

---

# 86. Standard Library First

Prefer Go standard library when adequate.

Every external dependency should justify:
- capability;
- maintenance;
- license;
- platform impact;
- determinism/security implications.

Avoid dependency sprawl.

---

# 87. Dependency Updates

Update deliberately.

Run:
- tests;
- race tests where relevant;
- deterministic hashes;
- benchmarks for hot dependencies.

Do not auto-merge major dependency upgrades blindly.

---

# 88. Linting Custom Rules

Project-specific static checks should catch:
- `time.Now` in domain;
- global `math/rand` in authoritative packages;
- forbidden imports;
- `utils` packages;
- generated code edits if detectable.

---

# 89. Review Checklist

Reviewer checks:

```text
Architecture boundary
Determinism
State ownership
Allocation impact
Complexity
Error semantics
Concurrency/race risk
Save compatibility
Mod/content impact
Test quality
Benchmark evidence
Naming/API clarity
```

---

# 90. AI Contributor Checklist

Before coding:
- search for existing type;
- read package `AGENTS.md`;
- read tests;
- identify hot path status;
- identify determinism constraints.

Before finishing:
- `gofmt`;
- tests;
- benchmark if relevant;
- race test if concurrent;
- docs/handoff;
- ensure no architecture leak.

---

# 91. Performance Escalation Path

Optimize in this order:

```text
1. eliminate O(N²)/global scans
2. reduce update frequency
3. add indexes
4. add dirty invalidation
5. sleeping/analytical progression
6. batch operations
7. reduce allocations
8. improve data layout
9. cache
10. parallelize pure work
11. micro-optimize only with evidence
```

---

# 92. Final Go Directive

Prefer boring, explicit, measurable Go.

The code should make it obvious:
- who owns state;
- when state changes;
- why ordering is deterministic;
- what a subsystem costs;
- how it can be tested;
- where an external technology plugs in.

If a clever abstraction makes those answers harder, remove the clever abstraction.
