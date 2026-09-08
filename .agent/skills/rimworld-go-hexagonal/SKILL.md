---
name: rimworld-go-hexagonal
description: Design or review rimworld-go package boundaries, ports, adapters, dependency direction, and cross-layer interfaces using pragmatic hexagonal architecture.
---

# Hexagonal Architecture Skill

## Core rule

Dependencies point inward:

```text
inbound adapters -> application -> domain
outbound adapters -> ports required by application/domain
```

The domain must remain headless.

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

Prefer:

```go
type SnapshotWriter interface {
    Write(ctx context.Context, snapshot SaveSnapshot) error
}
```

over provider-owned giant interfaces.

Do not create an interface simply because a concrete type exists.

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

## Persistence rule

Runtime domain types are not the external save schema.

Translate:

```text
domain snapshot <-> persistence DTO <-> protobuf
```

## Architecture review checklist

Reject or request redesign when:

- domain imports adapter/platform code;
- a "service" owns unrelated domains;
- an interface has broad CRUD merely for abstraction;
- mutable stores are exposed across the UI boundary;
- a package becomes a dumping ground;
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
