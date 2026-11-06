# ENG-008: every plan includes its parallel work

Date: 2026-10-29. User request: "change rules so all plans also plans parallel work like the skill and rules ask to".

## Why
Twice in one session, a plan for multi-area work was rejected because it lacked worker detail. The rules covered how to run parallel agents (`PARALLEL_WORK.md` §1–§6, the skill's lead checklist), but nothing required the plan itself to contain them.

## What changed
- `.cursor/PARALLEL_WORK.md` has a new §0. A plan for a multi-area task is incomplete until it carries a **Parallel workers** section:
  - the areas touched and the contract the lead writes first;
  - waves in dependency order, with an agent table per wave (id, model and reason, job, owned files, forbidden files, test commands);
  - the RAM check and wave size;
  - the gates and commit points;
  - art, sound and creature agents when assets are involved.

  A single-area or small task says so in one line.
- `.cursor/skills/parallel-work/SKILL.md`: lead-checklist step 0 ("Writing a plan? Put the workers in it") and a plan section template.
- `.claude/skills/parallel-work/SKILL.md`: a pointer to §0.
- `AGENTS.md`: the multi-area bullet now requires the section.
- `.cursor/rules/parallel-work.mdc`: a new always-apply recognition rule.
- Router skill: a new row for planning a multi-area task.
- `.cursor/DOCUMENT_INDEX.md`: the row mentions §0.

## First use
The beds/production/defence plan (BED/PRD/DEF): five waves, each with a full agent table.
