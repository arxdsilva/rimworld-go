# ENG-009: parallel work reuses and cleans up agents

User request (2026-10-30): Claude Code's agent list had piled up about 200 finished agents from the beds, production and defence programme.

## What changed
- `.cursor/PARALLEL_WORK.md` §5a "Agent lifecycle: reuse, then clean up":
  - follow-ups, fixes and the next wave go to the owning agent through `SendMessage`;
  - one docs and one journey agent per programme;
  - small lookups and edits are done by the lead;
  - `TaskStop` every agent once its work is committed and it won't be resumed, and stop them all at the end of a programme;
  - tell the user a new session starts clean once the list passes about 50.
- Lead checklist step 8 in `.cursor/skills/parallel-work/SKILL.md`; one line each in `.claude/skills/parallel-work/SKILL.md`, `.cursor/rules/parallel-work.mdc` and `AGENTS.md`.

## Facts the rule rests on (Claude Code docs, checked 2026-10-30)
- No model-callable or user-facing tool removes a finished run from the list. `TaskStop` only marks it stopped, and messaging a stopped agent resumes it.
- The user can clear failed or stopped rows sooner with `x` in `/tasks`.
- The default limit is 20 running subagents (`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`). There is no limit on the total spawned in a session.
- How the VS Code panel retains rows, and whether a finished agent holds memory, isn't documented.

## Not done
The rule can't remove the rows already in this session's list. A new session starts with a clean list.
