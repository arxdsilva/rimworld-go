# Ailments gap analysis (AIL-*)

Scope: the conditions that build up and fade on their own or with care (exposure, disease, infection, food poisoning, toxins, malnutrition, blood loss), checked for two things: that they match their wiki pages, and that no state can hold a pawn down forever. Load only for AIL-* tasks. Handoffs go in `.cursor/handoffs/AIL-N-*.md`. The numbers already live in `TEMPERATURE_GAP_ANALYSIS.md` (TEMP-3) and `MEDICAL_GAP_ANALYSIS.md` (MED-02, MED-04); this document holds only what is still different.

## Decision record

| Item | Decision |
|---|---|
| Goal | A pawn downed by an ailment always has a way back: care, a move, or the condition running its course. |
| Numbers | ENG-003 extended to AIL-* (2026-11-06). |
| The in-between band | Keep it. Heatstroke builds above the comfort maximum + 10 °C and heals below the maximum; hypothermia is the mirror (wiki: Ailments). In between it holds. |
| How a held pawn gets out | Vanilla treats these "by moving to more comfortable temperatures". A Doctor carries the pawn to air where it heals. |

## Sources

| Page | Used for |
|---|---|
| [Ailments](https://rimworldwiki.com/wiki/Ailments) (Heatstroke and Hypothermia redirect here) | Onset at 10 °C past the range, gain formulas, recovery below the maximum (above the minimum), the stages, treatment by moving |
| Flu, Plague, Malaria, Sleeping sickness, Gut worms, Muscle parasites, Infection | Already used in `MEDICAL_GAP_ANALYSIS.md` §4 |

**Not on any page read:** what rescue does with an exposure victim; whether a held band is meant to last (the page states only when severity rises and when it falls).

## Rows

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| AIL-001 | Colonists stay downed after a heatwave | Treated by moving to comfortable air | Was: two colonists at 80 % and 94 % heatstroke in their own beds in 33–34 °C air (comfort maximum 29.2 °C), held by the in-between band (`domain/pawn/exposure/exposure.go` `Step`), never moved (`rescueEligible` skipped a pawn in bed; rescue took any bed). Now a doctor carries a pawn lying in air that won't heal it (`care/rescue.go` `exposureStuck`, `exposure.Held` at the trivial stage). The player's save: both carried and standing within 4 h | done [AIL-1] |
| AIL-002 | Stand-up check | | `expose` checks standing every climate interval while the pawn is downed (`game/heat.go`), not only on a stage change | done [AIL-1] |
| AIL-003 | Bed choice for an exposure victim | | A rescue takes only a bed whose air heals with 1 °C to spare (`care/beds.go` `healMargin`, `healBest`); else the nearest spot whose air heals (`warmSpot` via `Climate.HealingSpot`, `care/carrybed.go` `setOffToSpot`); else, for a pawn on a floor that doesn't heal it either, any safe bed; else a wait of `ExposureRetry` (1 h). Every exposure move holds the patient 1 h (`noComfort`). No-flap: `TestExposureRescueDoesNotFlap`. Prison beds ignore the air (capture, warden) | done [AIL-1] |
| AIL-004 | The player can't see why | | `PawnView.ExposureNote` (sim, whole °C) for a downed pawn not healing; the inspector writes "Not recovering: air 33.4 °C, needs 29.2 °C or cooler" from `Temp` and the comfort range (`inspector/temp_ui.go`); the "Not recovering" alert (`hud/alerts_exposure.go`). The inspector's status lines are Celsius only, as the other temperature lines there (no unit reaches `ExposureLines`) | done [AIL-1]; unit: open, with the inspector's other lines |
| AIL-005 | Audit of every condition | | Table below | done [AIL-1] |
| AIL-006 | A permanent capacity inside the stand-up band | | `standsUp` needs consciousness ≥ 35 % and moving ≥ 20 %, `downsNow` downs below 30 % and 15 % (`domain/pawn/health`). A pawn left between the two by something permanent (scars, a lost part, a chronic condition) can't stand and isn't downed again; with nothing left to care for it leaves the patients index (`game/patients.go` `needsCare`) and is never checked again. None of the 14 conditions leaves such a residue; not reproduced | open, MEDICAL scope |
| AIL-007 | Exposure off the map | | `expose` skips pawns not on the map (`game/heat.go`), so a downed pawn carried off in a caravan keeps its severity frozen. Not traced whether a downed pawn can leave that way | open, check caravans |

## AIL-005 audit (2026-11-06)

Wiki: rimworldwiki.com pages Infection, Flu, Plague, Malaria, Sleeping sickness, Gut worms, Muscle parasites, Ailments (food poisoning, toxic buildup, hypothermia, heatstroke), Frostbite, Malnutrition, Blood loss. "Holds forever": a living pawn kept downed with no path to change.

| Condition | Wiki | Ours | Where | Holds forever |
|---|---|---|---|---|
| Infection | +0.84/day; immunity +0.6441/day; immune −0.7/day; tends slow it; 100 % kills | The same race, cured at 0, death at 100 % | `domain/pawn/infection/infection.go` `StepRace`; `game/infection.go` `stepConds` | n: it ends in a cure or death; `stepCare` checks standing hourly |
| Flu | +0.2488/day; immunity +0.2388/day | The race | `game/disease.go` `stepDisease`; `body/cond_disease.go` | n |
| Plague | +0.666/day; immunity +0.5224/day; immune −0.333/day | The race | same | n |
| Malaria | +0.3702/day; immunity +0.3145/day; immune −0.7297/day | The race | same | n |
| Sleeping sickness | +0.12/day; immunity +0.11/day; immune −0.176/day | The race (slowest rate 50/day, above the hourly rounding floor) | same | n |
| Gut worms | Never rises or fades; 300 % summed tend quality cures | Flat until the tends sum to 3000 | `game/disease.go` (`TendSum`), `body/cond_disease.go` `TendCureSev` | n: never downs on its own (pain +20 %) |
| Muscle parasites | Same as gut worms | Same | same | n: moving stays ≥ 70 % |
| Food poisoning | 24 h of stages, then gone | Timed stages, removed after 24 h | `body/cond_food.go` `StepFoodPoisoning`; `game/food_poison.go` | n |
| Toxic buildup | +40 %/day exposed; −8 %/day after | Same; zero only at full resistance, which humans never have | `body/cond_toxic.go` `StepToxic`; `game/incidents.go` `stepToxic` | n |
| Hypothermia | Builds 10 °C below the range; fades at or above the minimum; between, nothing is said | Same; between it holds | `domain/pawn/exposure/exposure.go` `Step`; `game/heat.go` `expose` | was y (AIL-001, AIL-002); now n: a doctor moves the pawn, it checks standing every interval |
| Heatstroke | Builds 10 °C above the range; fades at or below the maximum | Same; between it holds | same | was y; now n |
| Frostbite | A wound; heals naturally | A wound on an extremity, healed as one | `body/cond_frostbite.go` `StrikeFrostbite`; `body/tend.go` `HealHour` | n |
| Malnutrition | ±1.51–2.27 %/h; fed it falls | Same; a doctor feeds a downed colonist, the warden a prisoner | `body/cond_malnutrition.go` `StepMalnutrition`; `game/care_host_prison.go` `stepMalnutrition`; `care/feed.go` | n: it ends in feeding or death |
| Blood loss | Recovers 33.3 %/day once nothing bleeds | Same; untended bleeding kills | `body/bleed.go` `StepBlood`; `game/bleed.go` `stepBloodLoss` | n |

Every non-exposure condition downs with `CauseInjury`, which `stepCare` (`game/infection.go`) checks for standing every hour while the pawn is a patient. The exposure causes were the only ones checked on a stage change alone. A tend of quality 0 adds nothing to a parasite's tend sum (`body/tend.go` clamps at 0); it needs a doctor with no manipulation or sight, and parasites never down.

## Proposed phases

- **AIL-1:** the carry to healing air, the stand-up check, the reason on screen, the audit table, and a fix with a no-stuck test for every path the audit finds.

## Checklist cross-reference

Vanilla: 1041 representative disease classes `[-]` (stays), 1113–1114 temperature comfort `[-]`. `RESCUE_GAP_ANALYSIS.md`: RSC-201 (the rescuer's temperature rule) is next to AIL-003 but separate.
