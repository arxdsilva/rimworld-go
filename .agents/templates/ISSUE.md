# New GitHub issue

Every new issue in this repository, by a person or an agent, uses one shape: a defect, an unmet spec row, debt, a flaky test or a tooling problem alike. The body is `/.github/ISSUE_TEMPLATE/issue.md` without its front matter; the web form offers only that template (`config.yml` turns blank issues off), and agents fill the same file.

## Title

`<area>: <symptom>`, at most 80 characters, no trailing period. The area is lower case and names the package or system: `incident`, `travel`, `render`, `modding`, `contentlint`, `savefmt`, `arch`, `tests`, `ci`, and `loops` for the loops harness itself (not this repo's code). The symptom says what is wrong in plain words, not the fix.

## Body

All eight sections, in order: Summary, Where, Evidence, Expected vs actual, Impact, Suggested fix, Acceptance criteria, Refs. Write "unknown", "n/a" or "Not recorded" rather than dropping one; never invent evidence.
- **Where:** real `path:line` on the base branch (check it), plus the issue or PR it came from.
- **Evidence:** the command and the tail of its output, or the test name; numbers with the machine load when timing matters.
- **Impact:** one severity of `blocker`, `high`, `medium`, `low`.
- **Acceptance criteria:** checkboxes a reviewer can tick.

## Labels

One type label: `bug` (game behaviour is wrong), `debt` (works, but a shortcut, cleanup, comment or test gap to repay), `flaky` (timing- or load-dependent result), `spec-gap` (an AC or spec row left unmet), `infra` (CI, loops harness, scanners, machine setup). Plus one `priority: P0`..`P3`, except on a loops finding, which the human prioritises at triage. Keep labels a tool adds (`loops-finding`, `spec-backlog`).

## How

1. Search first: `gh issue list --state open --search "<plain key words>"`. On a match, comment there with the new evidence instead of filing; if unsure, file and write `Possible duplicate of #N` under Refs.
2. Write the body to a file and pass it with `--body-file` (`gh issue create`, `gh issue edit`, `loops backlog file`). Never `--body "<text>"`: the shell expands model-written text.
3. Cite the new `#N` in the handoff's `Open:` (`HANDOFF.md`).

## Loops lanes

A `/loops:work` lane in backlog mode never files issues; `loops backlog` files its findings, titled by the note and with the `--ref` file as the body. So each finding is recorded as `loops run event --kind note --name finding --note "<area>: <symptom>" --ref findings/<slug>.md`, where the ref file is this template filled in (front matter removed) in the run dir. `loops backlog` reads note findings before `pr.md` Open follow-ups and drops a repeated title, so the follow-up bullet's first line repeats the note title word for word (detail goes on indented lines below it); a follow-up with no matching note is filed with no body. Reflector harness notes follow the same rule with area `loops`.
