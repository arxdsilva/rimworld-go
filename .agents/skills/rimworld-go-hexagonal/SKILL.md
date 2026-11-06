---
name: rimworld-go-hexagonal
description: Use when designing or reviewing rimworld-go package boundaries, ports, adapters, dependency direction, cross-layer interfaces, or SOLID size limits.
---

# Hexagonal Architecture Skill

## Load

- `HEXAGONAL_ARCHITECTURE` §3 Dependency Rule, §6 Ports Are Consumer-Owned, §14 Package-Level Boundaries, §37 Architecture Smells, §39 Definition of Architecture Compliance
- `SOLID_GO` §6 Interface Segregation, §7 Dependency Inversion, §8 Measurable Limits, §9 Mutable Package-Level State, §10 How to Split a God Type

## Boundary test

For every dependency ask:

> Does the inner gameplay model need to know this technology exists?

If no, keep it outside.

Forbidden domain dependencies include:

- Ebitengine;
- filesystem/OS APIs;
- protobuf-generated types;
- YAML/JSON parsers;
- CLI frameworks;
- network frameworks;
- platform configuration;
- logging infrastructure that controls behavior.

## Ports

Create a port only at a real boundary.

Ports are consumer-owned and narrow.

Consumers depend on a small interface for the behaviour they call, never the concrete type (HEX §6, SOLID_GO §6–7); construct concrete types only at the composition root.

Size limits, the no-`init()` and no-package-state rules and the split recipe are `SOLID_GO` §8–10, enforced by `internal/arch/solid_test.go` (`make arch`).

Prefer:

```go
type SnapshotWriter interface {
    Write(ctx context.Context, snapshot SaveSnapshot) error
}
```

over provider-owned giant interfaces.

Do not create an interface simply because a concrete type exists — but always create one at a genuine adapter/application or application/domain boundary, even with a single implementation today.

## Cross-package design

Before adding a shared interface/type:

1. search for an existing canonical type;
2. determine the consumer;
3. define the minimum behavior required;
4. keep technology-specific types out;
5. document determinism and ownership assumptions.

## Domain vs application

Domain:

```text
ResolveAttack
AssignJob
TransferItem
ApplyDamage
AdvanceNeeds
```

Application:

```text
StartScenario
LoadGame
SaveGame
AdvanceFrame
VerifyReplay
```

## Presentation rule

UI receives immutable/read snapshots and emits typed commands.

UI never receives writable entity stores.

Player-visible sprites, icons, and UI chrome follow `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`; presentation adapters consume generated assets, they do not become domain types.

Named screens, windows, overlays, and HUD layout follow `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md`; they consume read models and emit typed commands.

## Persistence rule

Runtime domain types are not the external save schema.

Translate:

```text
domain snapshot <-> persistence DTO <-> protobuf
```

## Adding to a type, interface or package

When a change adds a method or field to an existing type, declares an interface, adds a package-level var or `init()`, or creates or splits a package:

1. Read `SOLID_GO` §8 before adding to `App`, `Renderer`, `Game` or any type or package with a line in `internal/arch/solid_budget.txt`.
2. Put new state and behaviour on its own type with a consumer-owned interface (≤8 methods) and a constructor called from the composition root.
3. Register tables at the composition root, never from `init()`; a test hook is a field, not a global.
4. Run `go test ./internal/arch/` before committing. A budget line is only lowered or deleted, in the same commit as the split that earns it; new code never gets a budget line.

Skip this for a task that touches no type, interface, package-level var or package layout.

## Architecture review checklist

Reject or request redesign when:

- domain imports adapter/platform code;
- a "service" owns unrelated domains;
- an interface has broad CRUD merely for abstraction;
- mutable stores are exposed across the UI boundary;
- a package becomes a dumping ground;
- a change raises a line in `internal/arch/solid_budget.txt`, adds an `init()`, or adds a method to a type or interface already over its `.agents/specs/SOLID_GO.md` §8 limit instead of splitting it;
- shared primitives are duplicated;
- technology dictates domain modeling;
- a new abstraction adds indirection without boundary value.

## ADR trigger

Write/update an ADR for changes to:

- architectural layers;
- simulation ownership;
- canonical primitives;
- command protocol;
- persistence schema strategy;
- concurrency model;
- content/mod model;
- state hashing;
- pathing architecture.
