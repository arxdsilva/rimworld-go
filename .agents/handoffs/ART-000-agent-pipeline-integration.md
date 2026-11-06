# Task Handoff

Task ID: ART-000
Status: complete

## Summary

Wired `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` into the agent pipeline so every agent can recognize it from `AGENTS.md` and the Cursor always-on rule, then load named sections via skill `rimworld-go-art-assets` when implementing player-visible art. The full spec is not default context for headless work.

## Files Changed

- `AGENTS.md`
- `.agents/README.md`
- `.agents/specs/MASTER_SPEC.md`
- `.agents/specs/HEXAGONAL_ARCHITECTURE.md`
- `.agents/specs/AI_TOKEN_OPTIMIZATION.md`
- `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`
- `.agents/backlog/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.agents/backlog/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.agents/skills/README.md`
- `.agents/skills/rimworld-go-router/SKILL.md`
- `.agents/skills/rimworld-go-art-assets/SKILL.md` (new)
- `.agents/skills/rimworld-go-feature/SKILL.md`
- `.agents/skills/rimworld-go-content-modding/SKILL.md`
- `.agents/skills/rimworld-go-review-integration/SKILL.md`
- `.agents/skills/rimworld-go-documentation/SKILL.md`
- `.agents/skills/rimworld-go-testing/SKILL.md`
- `.agents/skills/rimworld-go-bootstrap/SKILL.md`
- `.agents/skills/rimworld-go-hexagonal/SKILL.md`
- `.agents/skills/rimworld-go-determinism/SKILL.md`
- `.agents/templates/TASK_PACKET.md`
- `.agents/templates/HANDOFF.md`
- `.cursor/rules/art-direction.mdc` (new)
- `.agents/skills/rimworld-go-art-assets/SKILL.md` (new; thin pointer)

## Public Contracts Changed

None. Agent-facing documentation and skills only.

## Invariants / Decisions

- Canonical spec stays `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`; the skill operationalizes section routing and DoD.
- Recognition is always-on (`AGENTS.md` + Cursor rule); full spec load remains task-gated.
- ART wave checkboxes in the spec use the same `[x]` / `[-]` / `[!]` rule as the gameplay/UI checklists.
- Pixels, texture handles, camera, and animation stay in presentation adapters.

## Verification

Tests: not applicable (docs/skills only).
Benchmarks: none.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

Pipeline integration only. No Wave 0 contracts, palettes, templates, or assets were produced. ART wave checkboxes remain `[ ]`.

## Known Limitations

`assets/AGENTS.md` and `assettool` do not exist yet; the skill refers to them for when that layout is created.

## Follow-ups

- Freeze ART Wave 0 (scale, camera, pivots, palettes, metadata) before bulk production.
- Add `assets/` layout + `assets/AGENTS.md` when renderer art work begins.
