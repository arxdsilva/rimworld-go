# MED-04: diseases and outbreaks

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the RimWorld wiki disease pages under the ENG-003/MED
exception.

## Behaviour
- **Disease conditions** (`body/cond_disease.go`, appended after `CondInfection`) run on the MED-02 race engine in the hourly medical phase (`game/disease.go`). Death cause: `CauseDisease`.

  | Disease | Severity/day (unimmune / immune) | Immunity/day | Tend max | Tend every | Stages |
  |---|---|---|---|---|---|
  | Flu | +249 / −495 | +239 | 77 | 12 h | 666 / 833 |
  | Plague | +666 / −333 | +522 | 363 | 15 h | 600 / 800 / 910 |
  | Malaria | +370 / −730 | +315 | 232 | 12 h | 780 / 910 |
  | Sleeping sickness | +120 / −176 | +110 | 70 | 32 h | 437 / 625 / 875 / 937 |

  - Stages carry the wiki's capacity and pain changes and vomit intervals.
  - Gut worms: pain +200, hunger ×2, vomiting.
  - Muscle parasites: pain +200, rest fall ×2, moving and manipulation −300.
  - The two parasite conditions build no immunity. They are cured when tend quality adds up to 3000, at one tend per 48 h.
  - Untended deaths: flu 99 h, plague 36 h, malaria 66 h, sleeping sickness 198 h. All four are beaten at 70 % tend quality.
- **Outbreaks**: six Nature incidents from day 15.
  - Weight and cooldown are original tuning: flu 6/20, plague 2/40, malaria 4/25 (warm biomes), sleeping sickness 3/40 (tropical only), each parasite 4/25 (not on ice).
  - Weights scale with Disease frequency; 0 means none (`PickAt` / `TotalAt`).
  - Victims are drawn on `streamMedical`: one colonist, or 1 to min(n, 6) for flu, plague and malaria.
  - A synth stomach blocks gut worms.
- **Bed rest** starts at the major stage or when a tend is due, and ends only once the pawn is immune or cured (hysteresis).
- Vomiting leaves a dirt splash. Sick pawns get the existing "sick" thought.

## UI
- Health tab headline: "Flu: losing/winning" or "Gut worms: needs tending".
- Parasite tooltips show tending progress toward 300 %.
- A caution letter names everyone struck and how to treat it.
- New `disease` alert.
- The difficulty screens describe Disease frequency instead of "No diseases yet".

## Verify
- `go test -race` on the domain, game and scenario packages passes; targeted ebiten tests pass; `go vet` is clean.
- New tests:
  - `TestDiseaseOutcomeCurves`
  - `TestParasitesCureByTendSum`
  - `TestDiseaseFreqZeroNoIncidents`
  - `TestDiseaseBedRestDoesNotFlap`
  - `TestSleepingSicknessTropicalOnly`
  - `TestSynthStomachBlocksGutWorms`
  - `TestDiseaseAlert`
  - `TestDiseaseLetter`
  - `TestUntreatedFluKillsColonist`
  - `TestTendedFluIsBeaten`
  - `TestDiseaseWeightsFollowFrequency`
  - `TestDiseaseBiomesAndDays`
- Goldens are unchanged: the pinned runs never reach a day-15 disease roll.

## Deviations and follow-ups
- Parasites use the existing −8 "sick" thought, not the wiki's −5.
- Malaria's extreme stage caps consciousness at 10 %; the wiki marks that as unsure.
- Rates step in whole permille per hour.
- Food poisoning, toxic buildup, and disease while on a caravan are not covered yet.
