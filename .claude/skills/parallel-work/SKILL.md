---
name: parallel-work
description: Split a multi-area rimworld-go task across parallel subagents per .cursor/PARALLEL_WORK.md — contract first, disjoint file ownership, lead-only shared config, phased integration. Use for any task touching two or more of domain, simulation, ports, renderer/UI, outbound adapters, cmd, art pipeline, content or docs, or whenever the user asks to work in parallel.
---

# Parallel work

The canonical playbook lives with the other project skills. Read and follow both:

- `.cursor/PARALLEL_WORK.md` — the rule (ownership, phases, gates, model tiers)
- `.cursor/skills/parallel-work/SKILL.md` — the lead checklist, the plan section template and the agent prompt template

Every plan for a multi-area task includes its Parallel workers section before it is presented (PARALLEL_WORK §0).

Reuse agents before spawning new ones, and `TaskStop` each agent once its work is committed and you won't resume it (PARALLEL_WORK §5a).
