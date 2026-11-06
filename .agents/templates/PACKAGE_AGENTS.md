# Package AGENTS.md template

A package `AGENTS.md` says only what is local to its package. The shared rules (inward dependencies, a headless and deterministic domain, no RNG or wall clock in the simulation, presentation-only adapters, tests with `-race`) are stated once in the root `AGENTS.md` and `.agents/specs/`; never repeat them here. Leave out a section with nothing package-specific to say; never write "None".

```markdown
# <path relative to the repo root>

## Purpose
What the package owns and why it exists, with the IDs and dated decisions that shaped it.

## Owns
Its types and their roles; method or field counts only when near a SOLID limit.

## Depends on
Imports beyond the layer default (leaves, read models, third-party packages).

## Must not depend on
Package-specific bans: the root package, siblings above it, packages that import it.

## Contracts
What callers rely on, each with the test that pins it.

## Invariants
What must stay true, each with its test.

## Hot paths
Per-frame or per-tick code, with its benchmark and numbers.

## Tests
Where they live, guards in other packages, and anything special about running them.

## Notes
Known design constraints, and recipes for extending the package.
```
