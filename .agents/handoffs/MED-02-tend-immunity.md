# MED-02: tend quality, healing, scars, infection vs immunity

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the RimWorld wiki (Medical Tend Quality / Doctoring,
Infection, Injury) under the ENG-003/MED exception.

## Behaviour
- **Tend quality** (`body/tend.go`).
  - Base is 200 + 100 per Medicine level to level 8 (1000), then the wiki curve (lvl 18 1500, lvl 20 1550).
  - Factors:
    - manipulation, capped at 1400;
    - sight, weighted 70 %;
    - medicine potency;
    - hospital bed +100;
    - self-tend ×0.7;
    - a roll of 750–1250.
  - The result is clamped to the medicine's cap:

    | Medicine | Potency | Cap |
    |---|---|---|
    | none | 300 | 700 |
    | herbal | 600 | 700 |
    | industrial | 1000 | 1000 |

  - The nanomed hook is ready for MED-05. Rolls use `streamMedical`.
- **Healing**, per wound per day:
  - 8 HP base;
  - tended: +4 + 0.008 × quality;
  - lying: +4 on the ground, +8 in a bed, +14 in a hospital bed.

  This is RimWorld's pace, much slower than the old flat heal, and runs hourly over the patients index.
- **Scars**: odds are 20‰ × clamp((dmg − 4)/10).
- **Infection** is now `CondInfection`, one per wound, on the shared race engine (`infection.StepRace` / `WoundChance` / `Losing` / `HoursToDeath`).
  - Rolled 6–18 h after the wound.
  - Base chance: bite/burn 300‰, other bleeding wounds 150‰, bruises 0.
  - Multiplied by a tend factor (850 → 50‰ with quality; untended counts as 1000) and a room factor (500 in a clean roofed, floored room, else 1000).
  - Severity rises 840/day; tending cuts that by up to 530/day. Once immune, severity falls 700/day. Immunity rises 644/day.
  - Tend every 12 h. Stages at 330/780/870; death at 1000.
  - Raiders and prisoners can be infected.
- **Removed**: `g.wounds`, `tendHealBy`, `doctorInfectionPct`, and the per-pawn infection percentages. `story.go` now uses `infectionChance` in permille.

## UI
- Health tab:
  - a conditions section with a severity bar (red), an immunity bar (green) and "needs tend in Nh" / "now";
  - wound tend quality in part tooltips;
  - the infection headline and death clock.
- Pawn status: "Infection: losing/winning (N%)".
- Alerts: `infection` (warning when the infection is projected to win) and `untended` (≥ 4 h).

## Save
`SaveVersion = 6`. The migration `v < 6` turns each legacy pawn infection into a `CondInfection` on the worst open wound, with 0 immunity. The `Wounds` field is read-only.

## Verify
- `go test -race` on the pawn domain (body 97 %, infection 90 %), game (91 %) and targeted ebiten tests all pass. The lead's full `go test -race ./...` also ran.
- New tests:
  - `TestTendQualityFormula`
  - `TestHigherTendQualityWinsMoreRaces`
  - `TestScarOddsFollowDamage`
  - `TestOldSaveInfectionMigrates`
  - `TestOneTendPerWindow` (no-flap)
  - `TestPrisonerWoundsCanInfect`
  - `TestInfectionAlert`
  - `TestHealthTabConditions`
- Tests rewritten for the new healing and tending model:
  - plain-wound heal at the wiki rate;
  - scar, industrial-care, hospital-bed and rescue-then-tend tests;
  - longer windows for slower healing;
  - permille difficulty hooks.
- Golden re-pins after a same-seed double run:

  | Run | Old | New |
  |---|---|---|
  | 48x40/s1 | `0xd35284b973bbfa13` | `0xe0741d9f0ddd90e8` |
  | 48x40/s42 | `0x2e19907738249db9` | `0xb1380f278ba0cb52` |
  | 120x120/s7 | `0x9b0c8eb684a7186a` | `0x4118850af9407833` |

- Benchmarks were noisy under load: ColonyHour 18.5 → 19.8 ms; TickGrown is flat to faster.

## Original tuning (wiki has no number)
- Infection onset severity 10.
- A room counts as clean from 750‰ roofed.
- A 20-point stage-exit band.
- Animal tend heal `TendHeal × (250 + q) / 500` (until MED-09).

## Not done
UI row 702 (capacity breakdown tooltip) is still open.
