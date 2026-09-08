# rimworld-go — Root AI/Contributor Instructions

Read only what the current task needs.

Canonical documents:
- `MASTER_SPEC.md` — product/system scope.
- `HEXAGONAL_ARCHITECTURE.md` — dependency direction, ports/adapters, repository ownership.
- `GO_BEST_PRACTICES.md` — mandatory Go engineering rules.
- `AI_TOKEN_OPTIMIZATION.md` — context/token discipline.

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
- Every scoped task must leave a concise handoff.

## Repository-specific override: commits

In this repository only, the global rule forbidding agents from running `git add`/`git commit` does not apply. Agents may stage and commit their own changes here, following the message style and safety rules in the global git workflow guidance (no `--force`, no `--no-verify`, no rewriting published history, never push without explicit request).
