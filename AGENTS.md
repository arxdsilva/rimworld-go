# rimworld-go — Root AI/Contributor Instructions

Read only what the current task needs.

All canonical specs, rules, and agent-facing data live in [`.agent/`](.agent/DOCUMENT_INDEX.md):
- `.agent/MASTER_SPEC.md` — product/system scope.
- `.agent/HEXAGONAL_ARCHITECTURE.md` — dependency direction, ports/adapters, repository ownership.
- `.agent/GO_BEST_PRACTICES.md` — mandatory Go engineering rules.
- `.agent/AI_TOKEN_OPTIMIZATION.md` — context/token discipline.
- `.agent/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` — ordered vanilla gameplay feature backlog.
- `.agent/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` — ordered player-facing UI/UX backlog.
- `.agent/handoffs/` — per-task AI handoff records.
- `.agent/skills/` — task playbooks; start with `.agent/skills/rimworld-go-router/SKILL.md`.

Non-negotiable rules:
- Dependencies point inward: adapters → application → domain.
- Domain must remain headless and deterministic.
- One authoritative simulation owner; no goroutine-per-pawn/job.
- Search before creating shared primitives.
- All external mutations enter as typed commands.
- UI/rendering never mutates domain state directly.
- No global RNG or wall-clock time in authoritative gameplay.
- Avoid global scans; use indexes, batching, sleeping, and invalidation.
- Run relevant tests and benchmarks.
- Do not weaken tests to pass CI.
- Do not copy proprietary code, assets, text, or balancing data.
- Every scoped task must leave a concise handoff in `.agent/handoffs/`.

## Repository-specific override: commits

In this repository only, the global rule forbidding agents from running `git add`/`git commit` does not apply. Agents may stage and commit their own changes here, following the message style and safety rules in the global git workflow guidance (no `--force`, no `--no-verify`, no rewriting published history, never push without explicit request).
