---
name: rimworld-go-content-modding
description: Implement or review rimworld-go data-driven content definitions, validation, namespaced IDs, mod dependencies, deterministic loading, and future safe scripting.
---

# Content & Modding

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

## Scripting

Do not allow arbitrary native Go plugins from untrusted mods.

If scripting is introduced later, prefer deterministic sandboxable choices such as Starlark/WASM with:

- restricted host API;
- instruction/resource budgets;
- no arbitrary filesystem/network;
- deterministic time/RNG.

## Tests

Cover:

- deterministic registry assignment;
- dependency ordering;
- duplicate IDs;
- patch precedence;
- missing dependency;
- invalid path;
- roundtrip/fixture validation;
- mod set reflected in save metadata.
