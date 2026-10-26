# MED-08: aging and chronic conditions

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the wiki Ailments page under the ENG-003/MED exception.

## Behaviour
- **Conditions** (`body/cond_aging.go`):

  | Condition | Onset age | Effect | Ends when |
  |---|---|---|---|
  | Bad back | 41 | moving −300, manipulation −100 | spinal brace |
  | Cataract (per eye) | 49 | sight −250 | that eye is replaced |
  | Frail | 51 | moving and manipulation −300 | never |
  | Dementia | 69 | consciousness −150, talking and hearing −250; a 4 h confused wander, MTB 5 days | never |
  | Hearing loss (per ear) | 49 | hearing −250 | that ear is replaced |
  | Artery blockage | 21 | +0.7‰/day × 0.5–3; 5 stages, pumping −50…−600; heart attack MTB 300/200/100/60/30 days; kills at 1000 | artificial or new heart |
  | Carcinoma | 23 | starts at 300; grows, holds or remits; pain 100…600; at 1000 destroys its part, or kills if that part is vital | excision |

  - A part's wiki −500 becomes −250 on the capacity, because each capacity averages two parts.
- **Heart attack**: starts at 400 and moves by −400..+600 each hour.
  - Painful stage: consciousness −500.
  - From 600: consciousness at most 100.
  - At 1000: death (`CauseHeartAttack`).
  - A doctor can tend it hourly: each tend has a 65 % × quality chance to cut 300.
  - Untended, about 76 % die.
  - A pump or valve heart blocks both artery blockage and heart attacks.
- **Daily roll**: at 00:00 over sorted living colonists on `streamMedical`.
  - Onset chance (original tuning): min(Cap, Base + Slope × years past onset) per year, spread over 60 days.
  - Growth rates and the carcinoma's grow/stable/remit split come from a hash of the seed, the pawn and the condition, so nothing new is saved.
- **Excise carcinoma** (MED-06's operation) is now allowed when a carcinoma exists. Replacing a part cures that part's condition.
- **Epitaphs**: "of a heart attack", "of a carcinoma", "of an overdose" (lead fix).

## UI
- Health rows: "Bad back", "Cataract (left eye)", "Carcinoma (stomach), growing", "Heart attack (painful)", each with a tooltip saying how it ends.
- Headlines: "Heart attack: needs tending", "Carcinoma".
- Age was already shown in the bio, the pawn pane and the colonist bar.

## Verify
- `go test -race` over the domain and game packages passes; `go vet` and gofmt are clean.
- Tests:
  - `TestNoChronicBeforeOnsetAge`
  - `TestOnsetChanceRisesWithAge`
  - `TestHeartAttackDeterministic`
  - `TestPumpHeartPreventsHeartAttack`
  - `TestExciseCarcinoma`
  - `TestCataractCuredByEyeReplacement`
  - `TestBadBackCuredBySpinalBrace`
  - `TestArteryBlockageProgresses`
  - `TestLegacySaveHasNoAgingConds`
  - `TestAgingIsDeterministic`
  - plus onset, carcinoma, dementia, heart-attack tending, a no-flap check and a save round-trip
- Goldens: unchanged by MED-08. The pinned runs never reach midnight.

## Deviations
- A carcinoma's part-efficiency loss is not modelled; its pain is.
- The wiki's 30 % painless carcinomas and carcinoma tending are left out.
- "Biological aging" beyond these conditions stays `[-]`.
