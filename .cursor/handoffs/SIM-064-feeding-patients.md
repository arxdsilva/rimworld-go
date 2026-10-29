# SIM-064: feeding patients, downed hunger

Date: 2026-10-29. Programme SIM-061…066; committed with SIM-063…066 (see `SIM-066-mental-breaks.md` for gates and budget).

## What changed (S5, opus)
- **Downed hunger:** downed pawns' food need falls at the awake rate; rest stays frozen. The wiki publishes no downed rate. Downed prisoners too (`prisoner_life.go`).
- **Who gets fed:** `care.Feeding` is a Doctor work giver, after the tends. It feeds a patient who cannot feed itself: a downed colonist, or a guest the colony took in, not being carried. Pawns on bed rest who can stand still get up to eat.
- **When:** the doctor feeds at or below Hungry (2,500 of 10,000, the wiki's 0.25 and the existing `needs.CriticalHunger`). One meal fills the patient almost full. The patient is claimed by its feeder, and a run with no food retries after 1,000 ticks (`FeedRetry`, saved).
- **Shared driver:** the warden's prisoner food run uses the same driver. That also fixed an old bug where a warden gave up after its first re-path.
- **Order:** "Feed <name>" is offered below 80 % food and disabled "no food" when none is reachable.
- **Log lines:** "Fed by <name>" / "Fed <name>" (`logFedBy`/`logFed`).
- **Tests:** whole-game `feed_patient_test.go`, care `feed_test.go`, no-flap `TestFeedDoesNotFlap` and `TestFeedPatientDoesNotFlap`, journey `TestJourneyOrdersFeedPatient`.

## Gaps
Feeding downed animals (MED gap §7) stays open.

## Checklists
Vanilla 11.3 Downed and 11.5 Bed rest notes.
