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

Load the least that is sufficient, in this order: task statement, root `AGENTS.md`, package `AGENTS.md`, relevant interfaces and tests, one system doc, then specs by section only (ART via `rimworld-go-art-assets`, MAIN_UI via `rimworld-go-ui-screens`). `MASTER_SPEC` only if needed; do not feed it whole once package guidance exists. The doc list and loading order live in `.agents/README.md`.

---

# 3. Progressive Disclosure

Layer the documents: always the task, root and package `AGENTS.md`; usually the relevant interfaces, tests and one system doc; architecture questions add the specific ADR and HEX; `MASTER_SPEC` is rare. Visual and screen work load their skill and only the sections it names. The index is `.agents/README.md`.

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

Task packets follow `.agents/templates/TASK_PACKET.md`. Do not repeat rules already in root `AGENTS.md`.

---

# 6. Root AGENTS.md Should Be Short

Root `AGENTS.md` links to canonical docs instead of duplicating them. Its size is capped by the byte budget in `internal/arch/agents_budget.txt`, enforced by the guard test in `internal/arch`.

---

# 7. Package AGENTS.md Should Be Tiny

Package `AGENTS.md` is tiny (300-800 words) and never pastes the master spec. Shape: `.agents/templates/PACKAGE_AGENTS.md`.

---

# 8. Handoff Compression

Handoffs are 200-700 words and never store chain-of-thought. Shape: `.agents/templates/HANDOFF.md`. They are sections of the family ledgers in `.agents/handoffs/`, indexed by `INDEX.md`.

---

# 9. Interface Summaries

Not built: use the LSP tool or a ranged Read of the package's interface.

---

# 10. Context Index

Not built: use `.agents/README.md` and skill `rimworld-go-router`.

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

Not built: use package `AGENTS.md` files and `grep`.

---

# 20. Symbol Index

Not built, and not planned: use the LSP tool (`gopls`) or `grep -n`.

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

Not built: use the checklists in `.agents/backlog/` and the plan's Parallel workers section.

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
- internal/domain/building/aqua/AGENTS.md (an example package guide)
- hunger_rate.go
- hunger_rate_test.go

Constraints:
- deterministic integer math
- no allocation/tick
- no other package refactors

Acceptance:
- tests
- benchmark
- handoff
```

This should be the default style. The full packet shape is `.agents/templates/TASK_PACKET.md`.

---

# 30. Prompt Template — Cross-Package Task

```text
Task: AI-032
Add hauling reservation quantity support.

Read:
- root AGENTS.md
- reservation AGENTS.md
- economy/item AGENTS.md
- reservation and stack interfaces (LSP or ranged Read)
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

# 32a. Enforcement and Measurement

Advisory rules in this guide did not hold: most subagent work ran on the lead's model and most code reading went through the shell. These parts are enforced instead:

| Rule | Where | Limit |
| --- | --- | --- |
| Long shell views go through ranged Read | `.agents/hooks/read_guard.py` | Over 150 lines per command (`READ_GUARD_MAX_VIEW_LINES`) |
| Big files are read in ranges | `.agents/hooks/read_guard.py` | Whole-file Read over 500 lines (`READ_GUARD_MIN_LINES`) |
| `general-purpose` agents name a model | `.agents/hooks/agent_model_guard.py` | Any spawn with no `model` |
| Context stays small | `CLAUDE_CODE_AUTO_COMPACT_WINDOW` in the user's Claude Code settings | Auto-compact at 300k |

Small shell views stay allowed on purpose. Through Read they print the same text, so denying them only adds a call, and every call re-reads the whole context. `READ_GUARD=off` disables the read guard.

Measure with `python3 scripts/token_report.py --since <date>`. Before changing a guard rule, run `--last 400 --replay` to see what it would deny, and run `python3 .agents/hooks/test_hooks.py`. The baseline and expected results are in the TOKEN-001 entry in `.agents/handoffs/` (see `INDEX.md`).

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

Canonical ownership of each kind of fact is listed in `.agents/README.md`; state a rule once in its home and point to it elsewhere.

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
- note any public contract change in the handoff.

---

# 39. Recommended Automation Tooling

Not built, and not planned: no `tools/` helpers exist. The tooling in use is in §32a. A context-pack or handoff linter could come later.

---

# 40. Context Pack Manifest

Not built, and not planned: use `.agents/README.md` and grep.

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

For visual tasks, follow `rimworld-go-art-assets` section routing instead of pasting `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` in full.

For screen tasks, follow `rimworld-go-ui-screens` section routing instead of pasting `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` in full.

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
