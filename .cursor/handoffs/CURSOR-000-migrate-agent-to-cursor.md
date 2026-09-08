# Task Handoff

Task ID: CURSOR-000
Status: complete

## Summary

Moved the entire `.agent/` tree into `.cursor/` so specs, skills, handoffs, and Cursor rules share one project location. Thin Cursor skill stubs were replaced by the full playbooks. All `.agent/` path references were retargeted to `.cursor/`. The empty `.agent/` directory is gone.

## Files Changed

- `AGENTS.md` (paths only)
- `.agent/**` → `.cursor/**` (git mv)
- `.cursor/skills/rimworld-go-art-assets/` and `rimworld-go-ui-screens/` now hold the full playbooks (stubs removed)
- `.cursor/skills/README.md`, `manifest.txt`, `rimworld-go-documentation/SKILL.md`
- `.cursor/DOCUMENT_INDEX.md`
- `.cursor/rules/art-direction.mdc`, `ui-screens.mdc`

## Public Contracts Changed

None. Agent-facing documentation layout only. Go packages and `docs/` are unchanged.

## Invariants / Decisions

- Canonical agent data lives under `.cursor/`: specs at the `.cursor/` root, playbooks in `.cursor/skills/`, handoffs in `.cursor/handoffs/`, always-on recognition in `.cursor/rules/`.
- Root `AGENTS.md` remains the always-applied workspace rule and now points at `.cursor/DOCUMENT_INDEX.md`.
- Skills are native Cursor project skills (auto-discovered); no duplicate stub layer.
- Historical handoffs were rewritten to the new paths so later agents do not look in `.agent/`.

## Verification

Tests: not applicable (docs/skills move).
Benchmarks: none.
Repo search: no remaining `.agent` path references in tracked source/docs.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

None.

## Screen / UI Impact

None.

## Known Limitations

Generated/future files mentioned in token-optimization examples (`interfaces/`, `CONTEXT_INDEX.md`, `REPO_MAP.md`, `TASK_GRAPH.md`) now hypothetically live under `.cursor/` as well; they still do not exist.

## Follow-ups

- None required for the move. Continue using `.cursor/handoffs/` for new tasks.
