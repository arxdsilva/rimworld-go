# Task Handoff

Task ID: LOG-001
Status: complete

## Summary

Every colonist, raider and animal keeps an event log of its last 30 entries. It is recorded inside `Step`, saved, and shown in the inspectors: a Log tab for colonists, and a Recent list plus a Log toggle for animals.

## Files Changed

- `internal/application/game/eventlog.go` (kinds, `logEvent`, `logStrike`, text formatting, export/import) and `eventlog_test.go`.
- Hooks in `combat.go` (`strike`, `hurt`, `die`), `tick.go` (eat, sleep, `down`), `defense.go` (traps), `animals.go` (`killAnimal`, removal cleanup), `raids.go` (raider removal), `medical.go` (tend, rescue), `handling.go` (tame, manhunter), `mental.go`, `infection.go`, `heat.go`.
- UI: `log_ui.go`, the Log tab in `inspector.go`, the Recent/Log section in `animal_inspector.go`.

## Public Contracts Changed

- `PawnView.Log` and `AnimalView.Log` are `[]LogLineView{Tick, Day (1-based), Hour, Text}`, newest first.
- Save gains the optional `Logs` field.

## Invariants / Decisions

- Each entry stores the other party's name as it was at the time, so lines stay readable after that creature is gone.
- Exposure is logged once per episode, when it becomes serious.
- Animal and raider logs are deleted with the creature. A colonist's log stays after death.

## Verification

`eventlog_test.go` covers a predator bite on both sides, hits and misses, a trap, downed and died, eating and sleeping, taming success and failure, the 30-entry cap with newest-first order, save/load, and cleanup on removal. The game package is at 87.2% coverage. Determinism passes.
