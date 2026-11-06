# ENG-007: military aid by radio (PLAY-024 phase 4)

Original tuning; the values are in `.cursor/PLAY024_PLAN.md`.

## Behaviour
- **Request:** the comms dialog's "Request aid" (`CommsRequestAid`) is open only for friendly factions. It costs 20 goodwill and has a 3-day cooldown. Each disabled case shows its reason:
  - no fighters;
  - already on the way;
  - already here.
- **Arrival:** 2–4 h later, 3–5 armed fighters walk in from a map edge. An arrival that finds no way in retries an hour later.
- **Allies** (`allies.go`) are visitors flagged `Ally`, non-hostile pawns on `raiderIDs`:
  - They stay off the colonist bar, take no orders, and are never targets for turrets, colonists or drafted pawns.
  - Each standing ally fights the raider or hostile animal it is on (`doCombat`), looks for a new one every 30 ticks through the creature index (sight 20), and otherwise strolls near the colony.
  - Raiders fight them back (`raidTarget` in raids.go).
- **Leaving:** the group leaves after 2 h with no hostile about, or after 1 day at most. Leaving is one-way, so there is no flapping.
- **Letters:** arrival, departure and each fallen ally get a letter. A death costs no goodwill.
- **Save:** the `Aid` field in the trade block, nil when unused, plus the visitor record's `Ally` flag.

## UI
- An "Allies on the map (N)" alert (`alerts_allies.go`) with click targets. A lone group that is leaving says so.
- `PawnView.Ally`: the pawn inspector subtitle reads "Ally" ("Visitor" for other visitors).
- `ReasonNotYet` is removed.

## Verify
- Military aid: `TestRequestAidSpendsGoodwillAndCooldown`, `TestAidArrivesAfterDelay`, `TestAlliesFightRaiders`, `TestRaidersTargetAllies`, `TestTurretsIgnoreAllies`.
- Leaving, letters and save: `TestAlliesLeaveWhenQuiet`, `TestAllyDeathLetter`, `TestAidSurvivesSave`, `TestAidSameSeed`.
- UI: `TestAllyFlagInSnapshot`, `TestAlliesAlert`, `TestPawnSubtitleNamesAllyAndVisitor`.
- The full suite passes under `-race`. No golden moved: legacy colonies have no factions.

## Left open
- No quests and no faction-initiated aid.
