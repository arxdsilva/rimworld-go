---
name: rimworld-go-documentation
description: Create or update rimworld-go ADRs, system design documents, package AGENTS.md files, interface summaries, and concise task handoffs without duplicating canonical documentation.
---

# Architecture & Agent Documentation

## Canonical ownership

```text
.cursor/MASTER_SPEC.md              product/game scope
.cursor/HEXAGONAL_ARCHITECTURE.md   architecture/dependency model
.cursor/GO_BEST_PRACTICES.md        Go engineering standard
.cursor/AI_TOKEN_OPTIMIZATION.md    context/token policy
.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md   visual production / asset pipeline
.cursor/MAIN_UI_SCREENS_SPECIFICATION.md      per-screen layout / navigation / commands
.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md   ordered gameplay feature backlog
.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md             ordered UI/UX backlog
.cursor/rules/*                     Cursor always-on recognition
docs/systems/*              subsystem behavior
docs/adr/*                  cross-cutting decisions
package AGENTS.md           package-local constraints
.cursor/handoffs/*                  completed-task deltas
```

Do not copy long canonical sections into multiple files.

## ADR template

```markdown
# ADR-NNNN: Title

Status:
Date:
Owners:

## Context

## Decision

## Consequences

### Positive

### Negative / tradeoffs

## Alternatives considered

## Determinism impact

## Performance impact

## Persistence/mod impact

## Migration / rollout

## Verification
```

Write ADRs for decisions, not tutorials.

## System document template

```markdown
# System

## Purpose
## Owned data
## Inputs
## Outputs/events
## Simulation phase
## Update frequency
## Invariants
## Algorithms
## Complexity
## Indexes
## Caching/invalidation
## Sleeping/analytical progression
## Determinism
## Persistence
## Modding
## Debugging
## Tests
## Benchmarks
## Failure modes
```

## Package AGENTS.md

Target 300–800 words.

Include only:

```text
purpose
owned concepts
allowed dependencies
forbidden dependencies
contracts
invariants
hot paths
determinism
tests
benchmarks
```

## Handoff

Target 200–700 words.

Never include reasoning transcript.

State decisions/results only.

## Interface summary

For cross-agent dependencies, create a compact signature/invariant summary rather than copying entire packages.
