# rimworld-go Agent Skills

This package contains reusable `SKILL.md` playbooks for AI coding agents working on rimworld-go.

## Recommended installation

Copy the individual skill directories into the skill directory supported by your agent environment, or keep this folder inside the repository and instruct agents to read the relevant `SKILL.md`.

Do not load all skills into every task. Start with `rimworld-go-router`.

## Skills

| Skill | Purpose |
|---|---|
| rimworld-go-router | Select minimal task context and skills |
| rimworld-go-bootstrap | Repository foundations and CI |
| rimworld-go-feature | Normal feature implementation |
| rimworld-go-hexagonal | Ports/adapters/package boundaries |
| rimworld-go-determinism | Ticks, RNG, ordering, workers, hashes |
| rimworld-go-performance | CPU/memory/scale optimization |
| rimworld-go-pathing | Navigation, regions, movement |
| rimworld-go-ai-jobs | Pawn work, jobs, reservations |
| rimworld-go-persistence-replay | Saves, migrations, replay |
| rimworld-go-content-modding | Definitions and mods |
| rimworld-go-testing | Tests, fuzz, benchmarks, long runs |
| rimworld-go-debugging | Reproduce and fix simulation bugs |
| rimworld-go-review-integration | PR review and multi-agent merge |
| rimworld-go-documentation | ADRs, system docs, AGENTS, handoffs |

## Suggested repo layout

```text
.agent/
├── skills/
│   ├── rimworld-go-router/
│   │   └── SKILL.md
│   └── ...
├── MASTER_SPEC.md
├── HEXAGONAL_ARCHITECTURE.md
├── GO_BEST_PRACTICES.md
├── AI_TOKEN_OPTIMIZATION.md
├── RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md
├── RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md
├── DOCUMENT_INDEX.md
└── handoffs/

docs/
├── adr/
└── systems/
```

You can alternatively use `.codex/skills`, `.claude/skills`, or another tool-specific location if your agent supports it.

## Global context rule

A normal implementation task should load only:

```text
task
root AGENTS.md
nearest package AGENTS.md
relevant skill(s)
relevant interface/types
tests
one system doc/ADR when necessary
```

The full master spec should not be injected into every coding task.

## Source of truth

These skills operationalize the project's existing architecture, token optimization, Go best practices, and master specification; they do not replace those documents.
