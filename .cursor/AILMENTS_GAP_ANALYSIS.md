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
| AIL-001 | Colonists stay downed after a heatwave | Treated by moving to comfortable air | Seen in the player's save (day 76): two colonists at 80 % and 94 % heatstroke in their own beds in 33–34 °C air, comfort maximum 29.2 °C. Severity unchanged over 12 h, because the air is in the held band (`domain/pawn/exposure/exposure.go:139-158`). No one moves them: `rescueEligible` skips a pawn in bed (`game/care/rescue.go:152`), and rescue picks any bed whatever its air (`care/beds.go:81`) | open [AIL-1] |
| AIL-002 | Stand-up check | | `expose` checks standing only when the stage changes (`game/heat.go:95-97`) | open [AIL-1]: every climate interval |
| AIL-003 | Bed choice for an exposure victim | | Any bed (RSC-201) | open [AIL-1]: a bed whose air heals, else the nearest bearable spot (`warmSpot`, `heat.go:157`), with a retry cooldown |
| AIL-004 | The player can't see why | | "Heatstroke 80 %" only | open [AIL-1]: "Not recovering: air 33 °C, needs 29 °C or cooler" on the inspector, and an alert |
| AIL-005 | Audit of every condition | | Infection, flu, plague, malaria, sleeping sickness, gut worms, muscle parasites, food poisoning, toxic buildup, hypothermia, heatstroke, frostbite, malnutrition, blood loss | open [AIL-1]: a table per condition (wiki, ours, file:line, can it hold forever) filled from the audit |

## Proposed phases

- **AIL-1:** the carry to healing air, the stand-up check, the reason on screen, the audit table, and a fix with a no-stuck test for every path the audit finds.

## Checklist cross-reference

Vanilla: 1041 representative disease classes `[-]` (stays), 1113–1114 temperature comfort `[-]`. `RESCUE_GAP_ANALYSIS.md`: RSC-201 (the rescuer's temperature rule) is next to AIL-003 but separate.
