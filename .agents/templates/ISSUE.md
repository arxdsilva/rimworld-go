# New GitHub issue

Every new issue in this repository, by a person or an agent, uses one shape: a defect, an unmet spec row, debt, a flaky test or a tooling problem alike. The body is `/.github/ISSUE_TEMPLATE/issue.md` without its front matter; the web form offers only that template (`config.yml` turns blank issues off), and agents fill the same file.

## Title

`<area>: <symptom>`, at most 80 characters, no trailing period. The area is lower case and names the package or system: `incident`, `travel`, `render`, `modding`, `contentlint`, `savefmt`, `arch`, `tests`, `ci`, and `loops` for the loops harness itself (not this repo's code). The symptom says what is wrong in plain words, not the fix.

## Body

All eight sections, in order: Summary, Where, Evidence, Expected vs actual, Impact, Suggested fix, Acceptance criteria, Refs. Write "unknown", "n/a" or "Not recorded" rather than dropping one; never invent evidence.
- **Where:** real `path:line` on the base branch (check it), plus the issue or PR it came from.
- **Evidence:** the command and the tail of its output, or the test name; numbers with the machine load when timing matters.
- **Impact:** the line `Priority: P<n> (<rubric reason>) · Size: <XS..XL>` (below), then what it breaks or blocks.
- **Acceptance criteria:** checkboxes a reviewer can tick.

## Labels

Three labels, always: one type, one `priority: P0`..`P3`, one `size: XS`..`XL`. Keep labels a tool adds (`loops-finding`, `spec-backlog`).

Type: `bug` (game behaviour is wrong), `debt` (works, but a shortcut, cleanup, comment or test gap to repay), `flaky` (timing- or load-dependent result), `spec-gap` (an AC or spec row left unmet), `infra` (CI, loops harness, scanners, machine setup).

## Priority

Take the highest row that matches. Milestones are MASTER_SPEC §73. `loops backlog` queues P0 first and unlabelled issues last, so an issue without a priority is worked last.

| Label | Means | When |
|---|---|---|
| `priority: P0` | Drop other work | A crash or hang; save loss or corruption; a determinism, replay or state-hash break; `main` CI red; a security hole; a fault that fails every lane or PR |
| `priority: P1` | Next | Wrong behaviour in M0–M5 core gameplay; a promised follow-up of a merged change; it blocks another open issue or PR; a tooling fault that fails or doubles every gate run with no workaround |
| `priority: P2` | Planned | M6–M8 systems and the UI they need; a player-visible bug with a workaround; a spec gap in a shipped feature; tooling that slows or misleads runs but has a workaround |
| `priority: P3` | When convenient | Polish, cosmetic, comments, docs, test-only gaps, rare edge cases, optional features, milestone roll-ups |

Then adjust: a duplicate takes the priority of the oldest issue it repeats; a `loops`-area issue (fixed in ai-loops, not here) is at most P2 unless it fails every lane; "possibly already fixed" is P3 until checked; an issue blocking an open P0 or P1 goes up one step. Never lower a priority because the fix is big.

## Size

The expected change, tests included. When unsure, take the larger.

| Label | Shape |
|---|---|
| `size: XS` | Under an hour: one file, about 20 lines (a comment, a constant, one test case) |
| `size: S` | One package, up to about 150 lines with tests, no new interface |
| `size: M` | Two or three packages or up to about 500 lines; a port, save field or content schema changes |
| `size: L` | Several areas, up to about 1,500 lines; needs a plan with Parallel workers (`PARALLEL_WORK.md` §0) |
| `size: XL` | More than that, or the shape is unknown: split into L-or-smaller issues before anyone starts |

A `spec-backlog` issue is sized for its whole task list. Within one priority, work the smaller sizes and then the older issues first.

## Who sets them

Whoever files the issue sets all three. A loops finding arrives without them (the lane proposes them in its Impact line); the triage pass, by the human or an agent asked to triage, applies the labels before it removes `loops-finding`. Changing a priority or size later takes a one-line comment naming the table row.

## How

1. Search first: `gh issue list --state open --search "<plain key words>"`. On a match, comment there with the new evidence instead of filing; if unsure, file and write `Possible duplicate of #N` under Refs.
2. Write the body to a file and pass it with `--body-file` (`gh issue create`, `gh issue edit`, `loops backlog file`). Never `--body "<text>"`: the shell expands model-written text.
3. Cite the new `#N` in the handoff's `Open:` (`HANDOFF.md`).

## Loops lanes

A `/loops:work` lane in backlog mode never files issues; `loops backlog` files its findings, titled by the note and with the `--ref` file as the body. So each finding is recorded as `loops run event --kind note --name finding --note "<area>: <symptom>" --ref findings/<slug>.md`, where the ref file is this template filled in (front matter removed) in the run dir. `loops backlog` reads note findings before `pr.md` Open follow-ups and drops a repeated title, so the follow-up bullet's first line repeats the note title word for word (detail goes on indented lines below it); a follow-up with no matching note is filed with no body. Reflector harness notes follow the same rule with area `loops`.
