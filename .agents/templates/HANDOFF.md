# Handoff section

Every scoped task appends one section to its family's ledger in `.agents/handoffs/` and regenerates `.agents/handoffs/INDEX.md` with `python3 scripts/handoff_index.py`. The ledger for each ID family is listed at the top of `INDEX.md`; start a new ledger only when no existing one fits, and never name one `AGENTS.md`.

```markdown
## <ID>-<slug>
<Title> · <YYYY-MM-DD> · <short commit, once known>
- Changed: what a player sees or a caller relies on now (≤ 6 bullets)
- Decisions: invariants and chosen numbers, each with its reason
- Limits: SOLID budgets paid or hit, types at a field or method limit
- Boxes: the checklist, spec and gap items this task ticked or changed, by bold name or row ID with the new state, and the `spec-backlog` issues ticked or closed; or `none: <why>`
- Open: follow-ups, each with its ID
```

Boxes (required in every section dated 2026-11-20 or later, `TestLedgersAndIndex`): before writing the section, search `.agents/backlog/`, `.agents/specs/` and `.agents/gaps/` for every item the change completes or advances (grep the feature's names and IDs), not only the one the task started from. Set `[x]` when all its acceptance criteria and sub-bullets are met, `[-]` with what is missing, `[!]` with why; a gap row's state column follows its file's legend. Then the GitHub issue labelled `spec-backlog` for that section: tick the item's task line, and close the issue with `Closes #N` in the commit body when its last line is ticked (the commit closes it once `main` is pushed). The `Boxes:` line declares what was ticked; it does not prove it, so review it like code.

Rules:
- The heading is the task's stem (`SIM-036-colonist-flee`). It is unique across all ledgers; its anchor is the lowercased stem.
- At most 25 lines. Leave out a bullet with nothing to say.
- Also say, when there is one: the HUD or UI representation (or why there is none, AGENTS.md), and any determinism, save or mod impact.
- Leave out what git or the checklists already hold: file lists, test-name lists (unless a test is the only statement of a guarantee), commands run, full benchmark tables (keep the headline numbers), and the checklist row text.
- A plan rolled into a ledger may use up to 60 lines; its parts still owning an open checklist row stay verbatim under `### Open (verbatim)`.
