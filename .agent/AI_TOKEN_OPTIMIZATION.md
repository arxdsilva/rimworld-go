# rimworld-go — AI Token & Context Optimization Guide

> **Status:** Mandatory operating guide for AI coding agents  
> **Goal:** Minimize token usage, repeated context loading, hallucinated architecture, and duplicated work while preserving correctness.

---

# 1. Why This Document Exists

`rimworld-go` is intentionally designed for parallel development by multiple AI agents.

The main cost and failure modes in AI-heavy development are often not compilation or CPU—they are:

- repeatedly feeding the same large documents;
- agents reading unrelated files;
- agents inventing duplicate abstractions;
- huge prompts with low signal;
- long handoffs containing irrelevant prose;
- agents re-deriving architecture already decided;
- broad search instead of targeted context;
- re-sending full source files for tiny changes;
- generating explanations that do not affect implementation.

This document defines a **context economy**.

The goal is:

> Give each agent the smallest context that is sufficient to produce a correct, architecture-compliant change.

---

# 2. Context Hierarchy

Agents should load context in this order:

```text
1. Task statement
2. Root AGENTS.md
3. Package AGENTS.md
4. Relevant interfaces/types
5. Relevant tests
6. Relevant system doc
7. Relevant ADRs
8. Recent dependency handoffs
9. ART_DIRECTION sections named by rimworld-go-art-assets, only if visual/asset work
10. MAIN_UI_SCREENS sections named by rimworld-go-ui-screens, only if a player-facing screen is in scope
11. MASTER_SPEC only if needed
```

Do **not** feed the full master specification by default once package-level guidance exists.

---

# 3. Progressive Disclosure

Use layered documents.

## Level A — Always

Small:

```text
AGENTS.md
task
package AGENTS.md
```

## Level B — Usually

```text
relevant interfaces
tests
one system document
```

When the task produces sprites, icons, UI chrome, or asset-pipeline tooling, also load the `rimworld-go-art-assets` skill and only the named sections of `ART_DIRECTION_AND_ASSET_PIPELINE.md`.

When the task implements or changes a player-facing screen, window, overlay, or HUD layout, also load the `rimworld-go-ui-screens` skill and only the named sections of `MAIN_UI_SCREENS_SPECIFICATION.md`.

## Level C — Only if architectural question arises

```text
specific ADR
hexagonal architecture guide
```

## Level D — Rare

```text
MASTER_SPEC.md
```

This keeps context focused.

---

# 4. Context Budget Targets

Recommended task context:

**Small bug**
```text
2k–8k tokens
```

**Normal feature**
```text
5k–15k tokens
```

**Subsystem change**
```text
10k–30k tokens
```

**Architecture/RFC**
```text
20k–50k tokens
```

Avoid 100k-token prompts merely because the context window allows it.

Large context often reduces attention quality.

---

# 5. Agent Task Packet

Every task packet should include only:

```text
Task ID
Objective
Scope
Allowed packages
Forbidden packages
Acceptance criteria
Relevant interface excerpts
Relevant tests
Relevant ADR links/names
Relevant handoff summaries
```

Avoid repeating global rules word-for-word if they are already in root `AGENTS.md`.

---

# 6. Root AGENTS.md Should Be Short

Target:

```text
< 1,500 words
```

It should link to canonical docs rather than duplicate them.

Example:

```text
Architecture: .agent/HEXAGONAL_ARCHITECTURE.md
Go rules: .agent/GO_BEST_PRACTICES.md
Token rules: .agent/AI_TOKEN_OPTIMIZATION.md
Art/assets: .agent/ART_DIRECTION_AND_ASSET_PIPELINE.md
Screens: .agent/MAIN_UI_SCREENS_SPECIFICATION.md
```

---

# 7. Package AGENTS.md Should Be Tiny

Target:

```text
300–800 words
```

It should answer:

- what this package owns;
- what it must not own;
- allowed dependencies;
- core invariants;
- hot paths;
- test command;
- benchmark command.

Do not paste the master spec.

---

# 8. Handoff Compression

Handoffs must be useful but concise.

Target:

```text
200–700 words
```

A good handoff says:

```text
what changed
interfaces changed
assumptions
tests
benchmarks
known limitations
follow-ups
```

A bad handoff is a narrative transcript of how the agent thought.

Never store chain-of-thought.

---

# 9. Interface Summaries

When another agent depends on a package, prefer a generated/summarized interface file instead of the entire package.

Example:

```text
.agent/interfaces/pathing.md
```

Contains:

```go
type Request struct { ... }
type Result struct { ... }
type Service interface { ... }
```

plus invariants.

This can save thousands of tokens.

---

# 10. Context Index

Create:

```text
.agent/CONTEXT_INDEX.md
```

It maps tasks/subsystems to canonical files.

Example:

```text
Pathfinding:
- internal/domain/ai/pathing/AGENTS.md
- docs/systems/pathfinding.md
- docs/adr/0007-hierarchical-pathfinding.md
- .agent/interfaces/pathing.md
```

Agents should consult the index before broad repository search.

---

# 11. Search Before Read

Preferred workflow:

```text
search symbol/task ID
→ locate exact file
→ read relevant range
```

Avoid:

```text
read every file in package
```

when only one interface is needed.

---

# 12. Read Tests Early

Tests often encode behavior more efficiently than design prose.

For implementation work, load:

```text
public contract
current tests
failing test
```

before reading many implementation files.

---

# 13. Diff-Oriented Context

For code review or follow-up changes, send:

```text
git diff
relevant surrounding functions
tests
```

not entire files unless structure is necessary.

---

# 14. Avoid Re-Sending Stable Docs

Architecture documents should be referenced by path and summarized only when needed.

Task prompts should say:

```text
Follow ADR-0004 deterministic RNG.
```

not paste ADR-0004 every time.

---

# 15. Canonical Facts

Facts with high duplication risk should live in one canonical place.

Examples:

```text
Tick owner
EntityID owner
phase order
RNG rules
save schema policy
dependency direction
tile scale / pivots / palettes (art spec)
```

All other docs link there.

This avoids inconsistent copies.

---

# 16. Token-Safe Documentation Style

Prefer:

```text
tables
short invariants
code signatures
decision bullets
```

over long narrative paragraphs.

Example:

Bad:

> There are many reasons why domain packages generally should not reach outward into infrastructure...

Better:

```text
Rule: domain must not import adapters/platform.
Reason: deterministic headless testability.
```

---

# 17. Don't Explain Obvious Code

Handoffs and docs should avoid descriptions like:

```text
This function loops through the slice.
```

Document:

- invariant;
- complexity;
- ordering;
- reason;
- edge cases.

---

# 18. Stable Naming Reduces Tokens

Canonical naming prevents agents from asking/reasoning about synonyms.

Use exactly:

```text
EntityID
Tick
Tile
ChunkCoord
DefIndex
JobID
CommandEnvelope
RenderSnapshot
SaveSnapshot
```

Do not create alternatives such as `ActorID`, `SimTick`, `GridPos`, etc. unless domain semantics truly differ.

---

# 19. Generated Repository Maps

Maintain a lightweight generated file:

```text
.agent/REPO_MAP.md
```

Example:

```text
domain/sim/entity — IDs and lifecycle
domain/ai/job — job state machines
domain/ai/pathing — pure path solver
adapters/outbound/pathworker — async worker execution
```

This is cheaper than exploratory reads.

---

# 20. Symbol Index

Optional tooling can generate:

```text
package
exported type
exported function
interface
file
```

for fast agent lookup.

A symbol index often saves more tokens than long prompt engineering.

---

# 21. Task Size Optimization

Ideal AI task:

```text
one behavior
one owner
one acceptance test set
5–20 files max
```

Large epics should split into tasks with stable interfaces.

Avoid:

```text
Implement combat
```

Prefer:

```text
COMBAT-021 implement deterministic ranged hit resolution
```

---

# 22. Parallelization and Tokens

Parallelism only saves time if agents have isolated scopes.

Before launching many agents:

1. freeze shared contracts;
2. assign package ownership;
3. provide minimal task packets;
4. provide dependency handoffs;
5. avoid overlapping file sets.

Otherwise context/merge cost exceeds benefit.

---

# 23. Task Dependency Graph

Maintain:

```text
.agent/TASK_GRAPH.md
```

Example:

```text
SIM-020 EntityID
   ↓
WORLD-006 SpatialIndex
   ↓
PATH-003 A*
   ↓
PATH-004 Movement
   ↓
AI-003 MoveToJob
```

Agents only receive upstream handoffs they actually depend on.

---

# 24. Context Caching Strategy

Long-running agents should retain a short local working summary:

```text
Current task
Relevant interfaces
Invariants
Files touched
Open questions
```

Do not repeatedly re-read canonical docs unless they change.

---

# 25. Session Summary Template

For continuing work:

```markdown
Task: PATH-021
Current branch: feature/PATH-021
Goal: deterministic A*
Relevant contracts:
- Tile: world/grid
- MovementCostReader
- deterministic tie-break by node ID
Files touched:
- ...
Tests:
- ...
Open:
- ...
```

Target under 300 words.

---

# 26. Avoid Reasoning Transcripts

Never store or pass chain-of-thought.

Pass decisions:

```text
Decision: use dense sparse-set store.
Reason: stable iteration + cache locality.
```

not the internal reasoning process that produced it.

This is both safer and cheaper.

---

# 27. Review Context

Reviewer agent needs:

```text
task
acceptance criteria
diff
relevant architecture rule
tests/benchmark results
```

It usually does **not** need the entire repository.

---

# 28. Integration Context

Integration agent receives:

```text
wave task list
handoffs
diff summaries
public interface changes
test results
benchmark changes
```

Only open source files when conflicts or regressions require them.

---

# 29. Prompt Template — Small Task

```text
Task: PAWN-014
Implement bucketed hunger decay.

Read:
- root AGENTS.md
- internal/domain/pawn/needs/AGENTS.md
- hunger.go
- hunger_test.go
- ADR-0004 summary

Constraints:
- deterministic integer math
- no allocation/tick
- no other package refactors

Acceptance:
- tests
- benchmark
- handoff
```

This should be the default style.

---

# 30. Prompt Template — Cross-Package Task

```text
Task: AI-032
Add hauling reservation quantity support.

Read:
- root AGENTS.md
- reservation AGENTS.md
- economy/item AGENTS.md
- interface summaries for reservations and stacks
- existing integration tests

Allowed:
- reservation
- item
- hauling tests

Do not:
- modify pathfinding
- change EntityID
- change command format

Acceptance:
...
```

---

# 31. Prompt Template — Architecture Task

Architecture agents may receive broader context:

```text
problem statement
MASTER_SPEC relevant sections
HEXAGONAL_ARCHITECTURE
current ADRs
package graph
benchmark evidence
```

But even architecture tasks should not blindly ingest all source files.

---

# 32. Token Budget as an Engineering Metric

Track per-task approximate:

```text
input tokens
output tokens
number of files read
number of files modified
rework count
```

Purpose is not to optimize cost at the expense of quality.

Use metrics to identify:

- oversized prompts;
- poor docs;
- repeated context;
- architecture ambiguity.

---

# 33. High Token Usage Usually Signals a Problem

Repeated very large prompts can indicate:

```text
poor package boundaries
unclear interfaces
missing AGENTS.md
missing ADR
bad task decomposition
duplicate abstractions
insufficient test coverage
```

Fix the architecture/documentation rather than only compressing prompts.

---

# 34. Context Freshness

Agents should verify the current branch's:

```text
interfaces
tests
handoffs
ADRs
```

before coding.

Do not trust an old copied prompt if repository contracts changed.

---

# 35. Never Duplicate Full Specs Into Agent Files

`AGENTS.md` should link to master documents.

Avoid copying 5,000-word sections into 20 packages.

Duplication causes both token waste and stale guidance.

---

# 36. Documentation DRY Rule

Canonical ownership:

```text
product scope → MASTER_SPEC
architecture → HEXAGONAL_ARCHITECTURE
Go rules → GO_BEST_PRACTICES
token/context → AI_TOKEN_OPTIMIZATION
visual production / asset pipeline → ART_DIRECTION_AND_ASSET_PIPELINE
per-screen layout / navigation → MAIN_UI_SCREENS_SPECIFICATION
ordered UI/UX backlog → RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST
subsystem behavior → docs/systems/*
design decisions → docs/adr/*
package-specific constraints → package AGENTS.md
task outcome → handoff
```

---

# 37. Pre-Task Checklist

Agent should answer:

```text
What exact behavior am I changing?
Who owns it?
What interfaces exist?
Which invariants apply?
Which tests prove completion?
Which docs are actually required?
```

If these are clear, no additional broad context should be loaded.

---

# 38. Post-Task Compression

At completion:

- do not paste full changed files into handoff;
- list files;
- list signatures changed;
- include benchmark numbers;
- include follow-ups;
- update compact interface summary if public contract changed.

---

# 39. Recommended Automation Tooling

Create helper tools:

```text
tools/contextpack
tools/repomap
tools/symbolindex
tools/handofflint
```

Potential commands:

```bash
go run ./tools/contextpack --task PATH-021
go run ./tools/repomap
go run ./tools/handofflint .agent/handoffs/PATH-021.md
```

`contextpack` can output exactly the files/summaries an agent should receive.

---

# 40. Context Pack Manifest

Each subsystem can have:

```yaml
name: pathfinding
always:
  - internal/domain/ai/pathing/AGENTS.md
  - docs/systems/pathfinding.md
interfaces:
  - .agent/interfaces/pathing.md
adrs:
  - docs/adr/0007-hierarchical-pathfinding.md
```

This allows automated prompt assembly.

---

# 41. Code Generation and Tokens

Generated code should not be fed to agents unless debugging generation.

Prefer schema/proto source.

For protobuf tasks:

```text
read .proto
read adapter
do not read 10k lines generated .pb.go
```

---

# 42. Large Data Files

Do not feed large content packs/assets.

Use:
- definition schemas;
- small fixture;
- specific content ID;
- named ART_DIRECTION sections plus neighboring reference assets, never the whole art bible plus an atlas dump;
- named MAIN_UI_SCREENS sections for the screen under work, never the whole 50-screen spec.

For visual tasks, follow `rimworld-go-art-assets` section routing instead of pasting `.agent/ART_DIRECTION_AND_ASSET_PIPELINE.md` in full.

For screen tasks, follow `rimworld-go-ui-screens` section routing instead of pasting `.agent/MAIN_UI_SCREENS_SPECIFICATION.md` in full.

---

# 43. Logs and Traces

When debugging:

```text
filter by tick/entity/task
```

Do not paste megabytes of logs.

Developer tooling should support:

```text
--entity
--tick-range
--system
```

---

# 44. Benchmark Output

Pass summarized before/after:

```text
BenchmarkPath512
before: 1.8 ms/op, 64 KB/op
after: 1.2 ms/op, 8 KB/op
```

Only include raw full benchmark logs when needed.

---

# 45. Error Reports

Compact bug context:

```text
seed
tick
entity
replay ID
failure
state hash
relevant log window
```

Deterministic replay dramatically reduces required prompt context.

---

# 46. Replay Is a Token Optimization Feature

A replay file can replace thousands of words of bug description.

Target workflow:

```text
bug → replay fixture → failing test → task
```

Agent receives:

```text
replay path
expected behavior
relevant subsystem
```

---

# 47. State Hashing Is a Token Optimization Feature

Instead of describing subtle divergence:

```text
expected hash A
actual hash B
first divergent subsystem: movement
first divergent entity: 912
```

This narrows context immediately.

---

# 48. Debug Inspector Is a Token Optimization Feature

Structured diagnostics are cheaper than asking agents to infer state from many files.

Expose machine-readable debug data when possible.

---

# 49. Documentation Should Favor Machine Retrieval

Use:

- strong headings;
- predictable section names;
- code identifiers;
- task IDs;
- explicit paths.

This improves targeted retrieval.

---

# 50. Final Rule

The best token optimization is **good architecture**.

If an agent can understand a task from:

```text
one small AGENTS.md
one interface
one test
one ADR
```

the repository is healthy.

If every task requires the entire 2,000-line master specification plus dozens of source files, the architecture has failed to communicate its boundaries.
