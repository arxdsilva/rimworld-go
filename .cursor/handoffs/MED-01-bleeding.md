# MED-01: bleeding, blood loss, pain, downed

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the RimWorld wiki (Injury, Ailments) under the
ENG-003/MED exception.

## Behaviour
- **Bleeding** (`body/bleed.go`).
  - Rates are in ‰ of blood per day:

    | Wound | Bleed |
    |---|---|
    | cut, bite, scratch, surgical | 60‰ per HP |
    | bruise, burn, tended, scar | 0 |
    | heart wound | ×5 |
    | fresh stump (part torn off by a hit) | 2× its MaxHP, until tended |

  - Blood loss is stepped hourly over the sorted patients index. It regenerates 333‰/day, but only while nothing bleeds.
  - Stages enter at 150/300/450/600 and exit 50 lower. They take −100/−200/−400/−400 from consciousness, and consciousness is capped at 100 from 600. Death at 1000 (`CauseBloodLoss`).
- **Pain**: 12.5‰ per HP, open or tended; scars 1‰ per HP. Pain shock is at 800, scaled by pain tolerance.
- **Downed** (`health.BodyDowns` / `BodyRecovered`):
  - Down when consciousness < 300, moving < 150 or pain ≥ shock.
  - Up when consciousness ≥ 350, moving ≥ 200 and pain ≤ shock − 50.
  - Checked in `hurtAs`, `heal`, `doTend`, surgery, the dev hook and the hourly pass.
  - Bodiless pawns (animals) keep the old injury thresholds until MED-09.
- **Raiders and prisoners**: the 4h bleed-out timer is removed, so they bleed for real and get wound records. A downed raider who recovers flees.
- **Doctor**: tends the fastest bleeder first, then the worst hurt, then the nearest.
- **Stains**: drip only while bleeding.
- **Epitaph**: "of blood loss".
- **Phase**: `phaseInfection` is renamed `phaseMedical` (blood, infections, patients sweep).

## UI
- Health tab: a state line (red when bleeding), a blood-loss row with a bar, and the bleed rate in part tooltips.
- Pawn status: "Bleeding (death in Nh)".
- Alert `bleeding` (danger) at ≤ 12 h to death, with its explanation.

## Contract additions
- `Body.BloodStage` (hysteresis memory) and `Body.Fresh` (stumps), both saved via `savedBody`.
- `PartView.Bleed`, `PawnView.BleedDeathHours`.

## Verify
- `go test -race` for the pawn domain, game (166 s) and targeted ebiten tests: all pass. `go vet` is clean.
- New tests: `TestBloodStageTable`, `TestBloodLossDoesNotFlap`, `TestUntendedCutBleedsOut`, `TestTendStopsBleeding`, `TestRaiderBleedsOutLikeColonist`, `TestRaiderBleedsOutInTheRealGame`, `TestBruisedRaiderGetsUpAndFlees`, `TestDownedHysteresis`, `TestPainPerHP`, `TestBleedRate`, `TestStepBlood`, `TestFreshStumpHurtsAndBleedsUntilTended`, `TestHealthViewBleeding`, `TestDoctorTendsTheFastestBleederFirst`, `TestBloodStateSurvivesSave`, `TestBleedingAlert`, `TestHealthTabBleeding`, `TestPawnStatusSaysBleeding`.
- Tests changed for the new rules:
  - pain numbers (`TestTendAndPain`, `TestConsciousnessPain`);
  - the bleedOut test, replaced;
  - `downedAt` → `painShockAt`;
  - `TestInjuryDownsThenHeals` uses a bruise, since a torn-off part now bleeds a lone colonist out;
  - stains fixtures.
- Golden re-pins after a same-seed double run:

  | Run | Old | New |
  |---|---|---|
  | 48x40/s1 | `0xbdffb6d11dde9e6f` | `0xd35284b973bbfa13` |
  | 48x40/s42 | `0x3813609e849f6e9b` | `0x2e19907738249db9` |
  | 120x120/s7 | `0x7a003675fdf36a80` | `0x9b0c8eb684a7186a` |

  The other runs and the biome pins are unchanged.
- Benchmarks:

  | Benchmark | Before | After | Note |
  |---|---|---|---|
  | ColonyHour | 18.3 ms | 18.3 ms | |
  | ColonyRaid | 21.0 ms | 19.2 ms | |
  | TickRaid/500 | 0.278 ms/tick | 0.345 ms/tick | raiders now live longer and flee; the medical code doesn't show in the profile |

## Deferred
- Infection for raiders and prisoners (MED-02).
- The `Wound.Bleed` cache is unused.
