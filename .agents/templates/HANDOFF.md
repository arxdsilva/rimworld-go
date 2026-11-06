# Handoff section

Every scoped task appends one section to its family's ledger in `.agents/handoffs/` and regenerates `.agents/handoffs/INDEX.md` with `python3 scripts/handoff_index.py`. The ledger for each ID family is listed at the top of `INDEX.md`; start a new ledger only when no existing one fits, and never name one `AGENTS.md`.

```markdown
## <ID>-<slug>
<Title> · <YYYY-MM-DD> · <short commit, once known>
- Changed: what a player sees or a caller relies on now (≤ 6 bullets)
- Decisions: invariants and chosen numbers, each with its reason
- Limits: SOLID budgets paid or hit, types at a field or method limit
- Open: follow-ups, each with its ID
```

Rules:
- The heading is the task's stem (`SIM-036-colonist-flee`). It is unique across all ledgers; its anchor is the lowercased stem.
- At most 25 lines. Leave out a bullet with nothing to say.
- Also say, when there is one: the HUD or UI representation (or why there is none, AGENTS.md), and any determinism, save or mod impact.
- Leave out what git or the checklists already hold: file lists, test-name lists (unless a test is the only statement of a guarantee), commands run, full benchmark tables (keep the headline numbers), and the checklist row text.
- A plan rolled into a ledger may use up to 60 lines; its parts still owning an open checklist row stay verbatim under `### Open (verbatim)`.
