# Task Handoff

Task ID: ART-000
Status: complete

## Summary

Wired `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` into the agent pipeline so every agent can recognize it from `AGENTS.md` and the Cursor always-on rule, then load named sections via skill `rimworld-go-art-assets` when implementing player-visible art. The full spec is not default context for headless work.

## Files Changed

- `AGENTS.md`
- `.cursor/DOCUMENT_INDEX.md`
- `.cursor/MASTER_SPEC.md`
- `.cursor/HEXAGONAL_ARCHITECTURE.md`
- `.cursor/AI_TOKEN_OPTIMIZATION.md`
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.cursor/skills/README.md`
- `.cursor/skills/rimworld-go-router/SKILL.md`
- `.cursor/skills/rimworld-go-art-assets/SKILL.md` (new)
- `.cursor/skills/rimworld-go-feature/SKILL.md`
- `.cursor/skills/rimworld-go-content-modding/SKILL.md`
- `.cursor/skills/rimworld-go-review-integration/SKILL.md`
- `.cursor/skills/rimworld-go-documentation/SKILL.md`
- `.cursor/skills/rimworld-go-testing/SKILL.md`
- `.cursor/skills/rimworld-go-bootstrap/SKILL.md`
- `.cursor/skills/rimworld-go-hexagonal/SKILL.md`
- `.cursor/skills/rimworld-go-determinism/SKILL.md`
- `.cursor/skills/_templates/TASK_PACKET.md`
- `.cursor/skills/_templates/HANDOFF.md`
- `.cursor/rules/art-direction.mdc` (new)
- `.cursor/skills/rimworld-go-art-assets/SKILL.md` (new; thin pointer)

## Public Contracts Changed

None. Agent-facing documentation and skills only.

## Invariants / Decisions

- Canonical spec stays `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`; the skill operationalizes section routing and DoD.
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
