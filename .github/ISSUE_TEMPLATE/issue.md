---
name: Bug, gap or follow-up
about: A defect, an unmet spec row, debt to repay, a flaky test or a tooling problem
title: "<area>: <symptom>"
labels: []
assignees: []
---

<!-- Rules: .agents/templates/ISSUE.md. Keep every section; write "unknown" or "n/a" rather than dropping one. -->

## Summary
One sentence: what is wrong or missing.

## Where
- `path/to/file.go:123` (function or test name)
- Found by: #<issue or PR> (or "by hand")

## Evidence
What was seen: the failing command and its output tail, a test name, a screenshot or a metric. Steps to reproduce, if any.

## Expected vs actual
- Expected: …
- Actual: …

## Impact
Severity: blocker | high | medium | low. What it breaks or blocks, and which issues or PRs.

## Suggested fix
1–3 bullets, or "unknown".

## Acceptance criteria
- [ ] A checkable condition (a test passes, a gate is green, a spec row is ticked)

## Refs
Spec §, gap row, related issues or PRs. `Possible duplicate of #N` when one looks the same.
