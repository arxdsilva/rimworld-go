# SIM-065: visiting the sick, strip

Date: 2026-10-29. Programme SIM-061…066; committed with SIM-063…066 (see `SIM-066-mental-breaks.md` for gates and budget).

## What changed (S6, sonnet)
- **Who visits:** `care.Visits`. An idle, standing colonist with recreation below 85 % visits a friend lying awake in bed. The friend must be on bed rest, downed or needing a tend. Friendship means the visitor's opinion is 20 or more (original tuning, since the wiki publishes none) and the patient's is 0 or more.
- **The visit:** it lasts 1,500 ticks, and both gain 0.216 of the recreation bar, the wiki's number. One social interaction happens midway, and the patient's log reads "Visited by <name>". The patient isn't reserved, so doctors are never blocked.
- **Pair cooldown:** a pair may start a visit only in the first 600 ticks of each 2,400-tick window. That leaves 900 or more ticks between visits, and the no-flap test asserts it. It is stateless because the agent could not add saved state. A per-pair map with save state would read better; it is a follow-up.
- **Partners:** an awake patient in bed, downed ones included, is a valid partner for idle chat and social interactions.
- **Strip:** `care.Stripping`. "Strip <name>" works on any downed pawn, colonists included as in RimWorld, or a prisoner. It takes 90 ticks, and apparel and weapon drop as items. Stripping a downed guest of a faction costs −40 goodwill: the wiki pages disagree, and the decision was −40. `Gear.HasGear` disables the order with "nothing to strip".
- **Tests:** whole-game `visit_strip_test.go`, care `visit_test.go` and `strip_test.go`, journey `TestJourneyOrdersStripPrisoner`.

## Checklists
Vanilla 10.3 Periodic social interaction (note).
