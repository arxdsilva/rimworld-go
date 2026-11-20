---
name: rimworld-go-content-modding
description: Use when implementing or reviewing rimworld-go data-driven content definitions, validation, namespaced IDs, mod dependencies or deterministic loading.
---

# Content & Modding

## Load

- `MASTER_SPEC` §48 Modding, §49 Mod Scripting, §50 Content Definitions and Validation, §52 Security
- `HEXAGONAL_ARCHITECTURE` §20 Content Boundary

## Principle

Core game content should use the same definition system exposed to mods whenever practical.

Human-authored files may be YAML/JSON; gameplay receives validated typed immutable definitions.

## Load pipeline

```text
discover
-> resolve dependencies
-> deterministic load order
-> parse
-> validate schema
-> apply explicit patches
-> validate references
-> freeze registry
-> assign compact runtime indexes
```

## IDs

Use namespaced stable IDs:

```text
core.wall_wood
examplemod.weapon_laser
```

Hot runtime code may use compact `DefIndex`.

Do not use free-form strings in hot loops.

## Determinism

Filesystem enumeration order must not affect content ordering.

Sort deterministically.

Mod dependency/override rules must be explicit.

## Validation

Reject:

- duplicate IDs;
- missing references;
- dependency cycles;
- invalid ranges;
- unsafe paths;
- unsupported versions;
- ambiguous patches.

`contentlint` should perform validation without launching the renderer.

Plugin/core visual assets (sprites, icons, UI chrome, palettes, metadata) use the same namespaced IDs and validators as definitions, and must follow `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` via `rimworld-go-art-assets`. Plugins never mutate generated core art.

Plugin Manager, Load Order, Plugin Settings, and Content Error screens follow `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` §45–§48 via `rimworld-go-ui-screens`.

## Scripting

Do not allow arbitrary native Go plugins from untrusted mods.

Scripting is Starlark (PLG-060; `MASTER_SPEC` §49 holds the decision and the criteria for any second runtime). The port is `internal/application/scripting` (its package doc is the script author's reference); the runtime is `internal/adapters/outbound/starlarkhost`.

- Hooks: `on_hour`, `on_day`, `on_incident`, `on_colonist_died`; cadence hooks `on_pawns(ctx, pawns)` and `on_pawn(ctx, pawn)` see each home colonist once every 60 ticks in its bucket (`idindex.Buckets`, `cadence.OffPawnScripts`), never every tick.
- Reads: `ctx.pawns()` (all home colonists in one call, read only) and `ctx.pawn(id)`, fields id, name, x, y, food, rest, mood, downed, drafted; prefer the hook's argument over `ctx.pawns()` inside a cadence hook.
- Budgets: `MaxSteps`/`MaxCommands` per call; `on_pawn` shares one budget per file per tick, all or nothing. A failing script is turned off, never the game.
- `pawnIDs` grows only through `addPawnID` (the cadence index notes arrivals; `TestPawnIDsGrowOnlyThroughAddPawnID`).
- Runtime cost baseline: `internal/adapters/outbound/scriptbench` (Starlark vs gopher-lua, batched vs per call; test code only).

## Tests

Cover:

- deterministic registry assignment;
- dependency ordering;
- duplicate IDs;
- patch precedence;
- missing dependency;
- invalid path;
- roundtrip/fixture validation;
- mod set reflected in save metadata;
- plugin asset ID/path/namespace validation when assets exist.
