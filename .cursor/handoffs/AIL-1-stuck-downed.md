# AIL-1: no exposure state holds a colonist down forever

After a heatwave in the player's save (`colony-2026-10-27-1459.rwsave`, day 76), Imani (80 %) and Hana (94 %) lay in their own beds with heatstroke. The air there was 33.4 and 34.1 °C, against a comfortable maximum of 29.2 °C. That is the band where heatstroke neither builds nor fades (wiki: Ailments), and no one moved them. Over 12 simulated hours from the save, neither severity changed.

## What was wrong
- Rescue skipped a pawn already in bed (`care/rescue.go` `rescueEligible`), and it picked a bed whatever its air.
- `expose` checked standing only when an exposure stage changed. If a pawn could not stand at that moment, nothing ever checked again, because `updateDowned` stands only `CauseInjury` (AIL-002).

## What changed
- `domain/pawn/exposure`:
  - `Held(hypo, heat, floor, t, b)`: the air doesn't heal the condition (it holds or builds).
  - `Note`: the sim's sentence, in whole °C.
  - `Bands.Inset`.
  - `bodytemp.Sev.Held` and `Sev.Note` wrap them.
- `care`: `Reach` embeds a new `Climate` role with four methods: `AirHeals(patient, x, y, margin)`, `HealingSpot`, `ExposureWait` and `SetExposureWait`. The game's `careReach` serves them in `heat.go`, and the wait is the saved `noComfort` map. No `Care` methods were added, and `Care` stays at 40.
  - `rescueEligible` also takes a pawn lying in a bed whose air won't heal its exposure (`exposureStuck`, from the trivial stage, 4 %).
  - A rescue takes only a bed whose air heals the patient, with `healMargin` = 1 °C to spare (`healBest`).
  - If there is no such bed, the patient goes to the nearest spot whose air heals it (`warmSpot`, via `setOffToSpot` and `laySpot`). The trip keeps `Dest` 0, and the carry itself marks it.
  - A pawn on a floor that doesn't heal it either, with no spot, still goes to any safe bed.
  - With nowhere to go, the pawn waits `ExposureRetry` (1 h).
  - Every exposure move holds the patient for 1 h (`holdExposure`).
  - Prison beds (capture, the warden) ignore the air.
- `forced.go`: "Rescue" isn't disabled for "no bed" when a healing spot exists.
- `heat.go`:
  - A downed pawn checks standing every climate interval (AIL-002).
  - `exposureNoted` fills `PawnView.ExposureNote` for a downed pawn whose exposure isn't healing.
  - `needsComfort` now uses `Held`, with the same rule.
  - Root package: +12 net lines (game.go: the one contract line).
- UI:
  - `inspector/temp_ui.go` `ExposureNoteLine` builds "Not recovering: air 33.4 °C, needs 29.2 °C or cooler" from `Temp` and `ComfortMin`/`ComfortMax`.
  - `hud/alerts_exposure.go` adds a "Not recovering" danger alert, with its explanation in `alerts.go`.
- RSC-201 (the rescuer's own temperature rule) is unchanged; the fix didn't need it.

## Results
- The player's save, through a temporary diagnostic test, deleted afterwards: Imani and Hana are both carried and standing by +4 h. At +4 h their heatstroke is 44 % and 45 %, and at +5 h 14 %. They then go back to sleep in their own hot beds, standing, at 14 %. That is under the 20 % where a colonist goes looking for cooler air, and short of the build line.
- `TestHeatstrokeVictimInHotBedIsCarriedToACoolRoomAndStands`: in a bed at 30, 32 and 34 °C, the pawn is carried once to the cool room and stands after 0.3 h.

## Tests
- `care/exposure_rescue_test.go`:
  - cool bed, spot, waiting, staying put while healing;
  - forced-order reason;
  - prison beds;
  - no-flap: `TestExposureRescueDoesNotFlap`.
- `game/exposure_stuck_test.go`: the scenario above, and `TestExposureDownedStandsOnceItCan` (AIL-002).
- `exposure/held_test.go`, `inspector/temp_note_test.go`, `hud/alerts_exposure_test.go`.
- The `TestGrownGolden` hash is unchanged. The rules differ only for a pawn with 4 % or more hypothermia or heatstroke, or a downed exposure victim.

## Gaps
- AIL-006 (a permanent capacity inside the stand-up band) and AIL-007 (exposure off the map) are open in `AILMENTS_GAP_ANALYSIS.md`.
- The inspector's temperature lines are Celsius only, like the others there: no unit reaches `ExposureLines`.
- A medical bed the patient held for its stay remains held while it lies on a spot. It is given back when the patient stands.
- Prisoners don't get `ExposureNote` (`prisoners.go` is not owned here).
