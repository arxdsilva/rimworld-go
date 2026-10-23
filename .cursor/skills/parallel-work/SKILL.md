---
name: parallel-work
description: Split a multi-area rimworld-go task across parallel subagents per .cursor/PARALLEL_WORK.md — contract first, disjoint file ownership, lead-only shared config, phased integration. Use for any task touching two or more of domain, simulation, ports, renderer/UI, outbound adapters, cmd, art pipeline, content or docs, or whenever the user asks to work in parallel.
---

# Parallel work (lead checklist)

1. **Map areas → agents.** List the areas the task touches (rule intro table); one agent per area/layer (hexagon). ≤ 15 running at once.
   1b. **Pick each agent's model** (rule §6): `haiku` for mechanical work, `sonnet` for implementation against a written contract, `opus` for determinism/pathing/jobs/saves/timing math. Escalate one tier after two failed gates.
2. **Contract first (you, alone):**
   - `.cursor` spec sections, read models (`internal/application/game/*View`), consumer-owned interfaces, ports, typed commands, asset names/metadata, save format — additive only.
   - Stubs for every new hook so all agents compile.
   - `go get` / `go mod tidy` for every dependency the agents need.
   - All shared-config edits (`go.mod`, `Makefile`, `cmd/**`, palettes, Blender `contracts.py`/`stage.py`, CI, `.gitignore`).
   - Build `bin/game` / `bin/benchmark`; record baselines (benchmarks, captures).
3. **Launch producers in parallel** (domain/sim, pathing, Blender art, pure logic), then consumers (renderer, UI, outbound adapters; they may start against the contract + stubs). Background agents; one message with all launches.
4. **Relay** every cross-agent fact via `SendMessage`; broadcast contract changes to every affected agent.
5. **Integrate and verify** yourself after each phase with the rule §5 gates (build, fmt/vet/lint, race tests, determinism, art-check, benchmarks, visual review). Route failures to the owner.
6. **Write handoffs and update checklists** from the agent reports (agents never edit them).
7. **Don't enter plan mode while agents run** (it pauses them); if you did, resume each with `SendMessage`.

## Agent prompt template

```
Model: <haiku|sonnet|opus> — <one-line reason>   (set the Agent `model` parameter to match)

You are agent <ID> on the rimworld-go repo (/Users/asilva/youtube/rimworld-go). Read first (authoritative):
- <plan file §…>, AGENTS.md, <nearest package AGENTS.md>, .cursor/PARALLEL_WORK.md
- <named .cursor spec sections>, skill(s) .cursor/skills/<rimworld-go-*>/SKILL.md

## Contract (pasted in full)
<C-sections this agent follows>

## Your job
<scope, numbered, with acceptance criteria and the failing tests to write first>

## Rules
- Owns ONLY: <files/globs>. Must NOT touch: <files/globs> (owned by <agents>), go.mod/go.sum, Makefile, cmd/**, .cursor checklists/handoffs.
- No go get / go mod tidy (report missing deps). No git add/commit/push.
- Strict TDD; never weaken a test; run only: <targeted go test / go vet / make art FAMILIES=… commands>.
- Don't rebuild bin/*; build to the scratchpad if you need your own binary.
- If a producer isn't done, develop against the contract + stubs and report.

## Final report (under N words)
Changed files; public contracts; invariants; tests + coverage %; benchmark delta; determinism impact; save impact; visual impact; deviations; failures seen in others' files.
```
