# Harvesting gap analysis (HRV-001..)

Scope: who harvests what, how nearby plants are grouped, and what a harvest yields. Vanilla core only. Load only for HRV-* tasks. Growth itself is in `FOOD_GAP_ANALYSIS.md`; the tie-break with other work is in `WORK_PRIORITIES_GAP_ANALYSIS.md`.

## Decision record

| Item | Decision |
|---|---|
| Goal | Harvesting behaves as the wiki describes, and a grower finishes the field in front of it before anything but an emergency. |
| Numbers | ENG-003 extended to harvesting (2026-11-04, user approved). The tables below are published wiki data. |
| Waste chance | Ours: `100 - yield` percent below 100 % yield. The wiki says low yield gives a chance of wasting the whole harvest and gives no formula; its own stub asks whether 99 % means 1 %. |
| Batch size | Ours: every ripe plant within 8 tiles of the pawn, chained (HAUL-003). The wiki says colonists batch harvesting and gives no size. Vanilla's batch is limited; the user asked for the field cleared. |
| Speed | The wiki's Plant Work Speed applies to sowing and harvesting only. The other skills keep `skill.SpeedPermil` and `skillYield`. |

## Why a colonist seemed to haul at once

Three routes carry a fresh pile away:

1. A cook takes it as a bill ingredient. Same as vanilla.
2. Another free colonist hauls it. Same as vanilla.
3. The harvester itself leaves the field for more urgent work. The sweep was only tried in the Haul slot, so Cook at 1 and Haul at 3 never reached it. HRV-001 fixes this one.

## Sources

| Page | Used for |
|---|---|
| [Work](https://rimworldwiki.com/wiki/Work) | "Colonists batch cleaning and harvesting"; Grow harvests crops; plant cutters harvest wild plants; allowed areas |
| [Plants](https://rimworldwiki.com/wiki/Plants) | 65 % harvest age, half yield rising linearly, damage, fertility |
| [Growing zone](https://rimworldwiki.com/wiki/Growing_zone) | Automatic harvest at maturity; sowing and cutting toggles; minimum skill |
| [Plant Harvest Yield](https://rimworldwiki.com/wiki/Plant_Harvest_Yield) | Skill table, Manipulation and Sight factors, waste below 100 %, range 0..150 % (stub, verified for 1.2.2753) |
| [Plant Work Speed](https://rimworldwiki.com/wiki/Plant_Work_Speed) | 8 % + 11.5 % per level, minimum 10 % (stub) |

**Not on any page:** the waste formula, the batch size, where the product lands, whether a wasted harvest removes the plant. The version note "destroying the harvest for one plant" is the nearest. Ours removes a non-regrowing plant, as a successful harvest does.

The page's Sight term is inconsistent: its formula writes `max(100%, ...)`, its worked example `min(100%, ...)`. We use the stated "100 % limit", so bad sight lowers the yield and good sight does not raise it.

## Plant Harvest Yield by Plants level (100 % Manipulation)

| 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 |
|---|---|---|---|---|---|---|---|---|---|---|
| 60 | 70 | 75 | 80 | 85 | 90 | 95 | 97.5 | 100 | 101 | 102 |

| 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|---|---|---|---|
| 103 | 104 | 105 | 106 | 107 | 108 | 109 | 110 | 112 | 113 |

Yield = table × (100 % + (Manipulation − 100 %) × 30 %) × min(100 %, 100 % + (Sight − 100 %) × 20 %), held in 0..150 %. The Difficulty factor stays where it was (`DiffCrop`).

## Rows

State: `planned` being done now; `done` shipped; `open` owed to a later change; `kept` differs on purpose.

| Id | Gap | Wiki | Before | State |
|---|---|---|---|---|
| HRV-001 | Grouping nearby plants | Batched | Sweep tried only in the Haul slot | planned: continues before the priority loop, after Firefight and Doctor |
| HRV-002 | Sweep ends on a plant outside the allowed area | Work stays in the area | The nearest plant was refused and the sweep ended | planned: the search skips plants outside the area |
| HRV-003 | Who harvests wild plants | Plant cut; Grow harvests crops | Grow | planned: outside a zone and not sown is Plant cut, otherwise Grow |
| HRV-004 | Damage | Up to half: 50 % health loses 25 % | 50 % damage lost 50 % | planned |
| HRV-005 | Yield by level | The table above | Straight line, 72 % at 0, 100 % at 7, 140 % at 20 | planned |
| HRV-006 | Yield below 100 % | Chance of wasting everything; no smaller amount described | Smaller amount, and a waste chance only under level 4 | planned |
| HRV-007 | Yield above 100 % | Proportional bonus | Proportional bonus | planned, on the new table |
| HRV-008 | Manipulation and Sight | 30 % and 20 % weight | Ignored | planned |
| HRV-009 | Speed by level | 8 % + 11.5 % per level, minimum 10 %: 10 % at 0, 100 % at 8, 238 % at 20 | 58 % at 0, 100 % at 7, 178 % at 20 | planned |
| HRV-010 | By-product of a wasted harvest | "The entire harvest will be wasted" | Straw still dropped | planned |
| HRV-101 | Plants specialist, Field hand and other offsets | Listed | Absent: no such role or implant | open |
| HRV-102 | Drug crops use another stat | Listed | No drug crops (FOOD scope) | open |

## Already matching

Harvestable from 65 % at half yield, rising linearly to full; a zone marks its plants at full maturity; fertility never touches yield; minimum skill only stops sowing; a zone with sowing off still harvests and cuts; regrowing plants go back to their regrowth point.
