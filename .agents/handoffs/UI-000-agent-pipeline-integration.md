# Task Handoff

Task ID: UI-000
Status: complete

## Summary

Wired `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` into the agent pipeline the same way as art direction: always-on recognition from `AGENTS.md` and a Cursor rule, task-gated section loading via skill `rimworld-go-ui-screens`. The UI/UX checklist remains the ordered backlog; the screens spec is the per-screen layout/commands/states contract.

## Files Changed

- `AGENTS.md`
- `.agents/README.md`
- `.agents/specs/MASTER_SPEC.md`
- `.agents/specs/HEXAGONAL_ARCHITECTURE.md`
- `.agents/specs/AI_TOKEN_OPTIMIZATION.md`
- `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md`
- `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md`
- `.agents/backlog/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`
- `.agents/backlog/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`
- `.agents/skills/README.md`
- `.agents/skills/rimworld-go-router/SKILL.md`
- `.agents/skills/rimworld-go-ui-screens/SKILL.md` (new)
- `.agents/skills/rimworld-go-feature/SKILL.md`
- `.agents/skills/rimworld-go-content-modding/SKILL.md`
- `.agents/skills/rimworld-go-review-integration/SKILL.md`
- `.agents/skills/rimworld-go-documentation/SKILL.md`
- `.agents/skills/rimworld-go-testing/SKILL.md`
- `.agents/skills/rimworld-go-bootstrap/SKILL.md`
- `.agents/skills/rimworld-go-hexagonal/SKILL.md`
- `.agents/skills/rimworld-go-art-assets/SKILL.md`
- `.agents/templates/TASK_PACKET.md`
- `.agents/templates/HANDOFF.md`
- `.cursor/rules/ui-screens.mdc` (new)
- `.agents/skills/rimworld-go-ui-screens/SKILL.md` (new; thin pointer)

## Public Contracts Changed

None. Agent-facing documentation and skills only.

## Invariants / Decisions

- Canonical screen spec stays `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md`; the skill operationalizes section routing and §59 DoD.
- `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` remains the ordered UI backlog; agents load both when implementing a named screen.
- Recognition is always-on; full spec load remains task-gated (named section plus §0/§58/§59).
- Wave UI-0 framework must exist before later-wave screens.
- Screens consume read models and emit typed commands.

## Verification

Tests: not applicable (docs/skills only).
Benchmarks: none.

## Determinism Impact

None.

## Save / Mod Impact

None.

## Art / Visual Impact

None.

## Screen / UI Impact

Pipeline integration only. No Wave UI-0 primitives or named screens were implemented. Screen-wave checkboxes remain `[ ]`.

## Known Limitations

Presentation package `AGENTS.md` and reusable window/virtual-list primitives do not exist yet; the skill refers to them for when that layout is created.

## Follow-ups

- Implement Wave UI-0 (primitives, focus, window/modal, snapshot boundary, command gateway) before later screens.
- Add presentation-package `AGENTS.md` when the inbound UI adapter is more than the current Ebitengine shell.
