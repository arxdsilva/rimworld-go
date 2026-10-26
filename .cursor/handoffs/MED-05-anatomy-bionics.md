# MED-05: organs, capacities, bionic tier

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the wiki (Kidney, Heart, Bionic_*, Prosthetic_*) under
the ENG-003/MED exception.

## Behaviour
- **Parts**: KidneyL/R 50 HP (coverage 17 each), Liver 50 HP (25), Spine 40 HP (25). Torso coverage drops 380 → 296, so the total stays 1000.
  - Old v5/v6 bodies load with the new organs at full HP.
  - `body.Inner(p)` marks the internal organs. The picker, `CanOperate` and the UI fallback never offer to amputate them; they are harvested in MED-06.
- **Capacities**:
  - Blood pumping (heart): weighs 0.2 on consciousness and moving, and scales bleeding downward only.
  - Blood filtration (kidneys × liver): weighs 0.1 on consciousness (one kidney −50) and 0.5 on immunity gain (one kidney 750, which feeds the MED-02 race).
  - Digestion (stomach × liver).
  - Spine efficiency multiplies moving.
  - Servo limbs keep their bonus above 100 %.
- **Death**: pumping, filtration, breathing or digestion at 0 is organ failure (`health.CauseOrganFailure`, epitaph "of organ failure").
  - A hit never destroys the last life organ, so death from wounds still comes at MaxHP.
  - The hourly medical phase catches organ failure from anything else.
- **Items**:

  | Item | Efficiency or potency | Value |
  |---|---|---|
  | Nanomed kit | potency 1600, max quality 1300 | 50 |
  | Servo arm, servo leg, optic implant | 1250 | 1030 |
  | Cochlear implant, synth stomach | 1250 | 785 |
  | Spinal brace | 1000 | 1030 |
  | Pump heart | 1250 | 1030 |
  | Valve heart | 800 | 230 |
  | Organs: kidney / lung / heart / liver | – | 900 / 1000 / 1200 / 1200 |

  - Prosthetic arm is fixed to 500 and wooden hand to 600 (wiki).
  - Helpers for later phases: `HeartAttackImmune`, `GutWormImmune`, `SpineBraced`.
  - A fitted artificial heart can't be removed.
- **Research and recipes**:
  - `ServoReplacements` (6000) needs Prosthetics + Fabrication. Servo recipes need Crafting 8.
  - Valve heart needs Crafting 5.
  - Recipes take a single input: components for servo parts, steel 25 for the valve heart.
  - The nanomed kit comes only from vaults (4 % of rolls).
- **Medicine**:
  - The nanomed kit is the top tier: care level "Best available" allows it, and `MaxMedicine(Best)` is the nanomed kit.
  - Severe or infected patients try nanomed, then industrial, then herbal. Others try herbal first.
  - Nanomed kits and organs are hauled to goods stockpiles.

## UI
- New capacity rows and organ part rows in the Health tab.
- The capacity column now has 11 rows, so drug lines can be cut off on short panels (follow-up).

## Verify
- `go test -race` on the domain and game packages and `go vet` pass. The lead's full suite ran after the integration fixes.
- New tests:
  - `TestCoverageSumsTo1000`
  - `TestKidneyLossKillsWhenBoth`
  - `TestOneKidneyLowersImmunity`
  - `TestSpineLossStopsMoving`
  - `TestServoArmAbove100`
  - `TestProstheticArmEfficiencyFixed`
  - `TestLegacyBodyHasNewOrgans`
  - `TestNanomedPotency`
  - recipe/research gating and value/category tests
- Golden re-pins after same-seed double runs. MED-03 and MED-05 were re-pinned together; MED-05 alone produced the same values on a clean HEAD + MED-05 copy:

  | Run | Old | New |
  |---|---|---|
  | 48x40/s42 | `0xb1380f278ba0cb52` | `0x7ae892588e0e1327` |
  | 120x120/s7 | `0x4118850af9407833` | `0xade18d4a6ba193e9` |
  | grown scenario | `0x15aa8af06fa3437b` | `0xde4879ac9bcfeb75` |

## Art
The new item sprites and icons are a separate art pass (ART-026): nanomed kit, servo arm and leg, optic and cochlear implants, spinal brace, pump and valve hearts, synth stomach, kidney, lung, heart and liver.
