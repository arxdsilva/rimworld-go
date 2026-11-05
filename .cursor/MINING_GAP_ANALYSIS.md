# Mining gap analysis (MIN-001..)

Scope: how long a tile takes, what it yields, what falls on the miner, what's in the rock. Vanilla core only. Load only for MIN-* tasks. The Mine designation and its place in the work order are done (`WORK_PRIORITIES_GAP_ANALYSIS.md`); built roofs are SIM-018.

## Decision record

| Item | Decision |
|---|---|
| Goal | Mining a mountain costs what the wiki says and carries the wiki's risk, and the player can see that risk before giving the order. |
| Numbers | ENG-003 extended to mining (2026-11-05, user approved). The tables below are published wiki data. |
| Thick cave-ins | Recommended: lethal as published (99,999 damage), shipped only together with the risk preview (MIN-402), so a colonist's death doesn't come as a surprise. To confirm when MIN-2 is planned. |
| Numbers ship together | Recommended: all of MIN-1 in one change. Ore gets about 4× slower and steel doubles; either alone skews the economy. |

## How we can use it

- **Mining becomes a gamble.** A mountain base is the strongest shelter in the game, since thick roof stops pods and mortars. It should cost something: each tile you take from the middle can bring the ceiling down.
- **Show the risk.** A preview on the Mine designator goes beyond vanilla, which gives no advance warning. It is cheap: `roof.Cell` already has `Supported` and the support grid is already computed.
- **Dropping a roof on raiders.** Follows from the rule itself, with no new system: lure them under, remove the last support.
- **A skilled miner matters.** On the wiki's curve level 0 does 4 % of the work of level 8, and level 20 does 244 %. Today the spread is 58 % to 178 %, so anyone will do.

## Sources

| Page | Used for |
|---|---|
| [Mining Speed](https://rimworldwiki.com/wiki/Mining_Speed) | 80 damage per 120 ticks at Mining 8; 4 % + 12 % per level; Manipulation, Sight (stub, verified for 1.2.2900) |
| [Mining Yield](https://rimworldwiki.com/wiki/Mining_Yield) | Table, cap 125 %, Manipulation, Sight (verified for 1.3.3200) |
| [Ore](https://rimworldwiki.com/wiki/Ore) | HP and yield of each ore; Losing is Fun 80 % |
| [Compacted steel](https://rimworldwiki.com/wiki/Compacted_steel), [Silver ore](https://rimworldwiki.com/wiki/Silver_ore), [Gold ore](https://rimworldwiki.com/wiki/Gold_ore), [Uranium ore](https://rimworldwiki.com/wiki/Uranium_ore), [Jade (ore)](https://rimworldwiki.com/wiki/Jade_(ore)), [Compacted plasteel](https://rimworldwiki.com/wiki/Compacted_plasteel), [Compacted machinery](https://rimworldwiki.com/wiki/Compacted_machinery) | Vein size and commonality |
| [Granite](https://rimworldwiki.com/wiki/Granite), [Limestone](https://rimworldwiki.com/wiki/Limestone), [Slate](https://rimworldwiki.com/wiki/Slate), [Marble](https://rimworldwiki.com/wiki/Marble), [Sandstone](https://rimworldwiki.com/wiki/Sandstone) | Rock HP, cover 75 % |
| [Stone chunk](https://rimworldwiki.com/wiki/Stone_chunk) | "~25% chance" per tile, 20 blocks per chunk at 1,600 ticks, HP 300, cover 50 %, masses |
| [Roof](https://rimworldwiki.com/wiki/Roof) | Support range, collapse damage, thin and thick (stub, needs fact check) |
| [Deep drill](https://rimworldwiki.com/wiki/Deep_drill), [Ground-penetrating scanner](https://rimworldwiki.com/wiki/Ground-penetrating_scanner) | Cycle, power, reach, cost, mean and maximum find time |
| [Infestation](https://rimworldwiki.com/wiki/Infestation), [Events](https://rimworldwiki.com/wiki/Events) | Placement and pacing; light and cold reduce the chance |
| [Work](https://rimworldwiki.com/wiki/Work) | Mine and Drill tasks |

**Not on any page read:** the Sandstone figure on Mining Speed (the number is missing; 600 below is derived from the same formula), what a yield under 100 % does (a smaller stack or a chance of nothing), the likelihood formula for infestation, the damage items take in a collapse, the HP of collapsed rock, how commonality turns into a count of veins.

## Numbers

### Work per tile

Ticks at 100 % speed (Mining 8) = HP × 120 / 80. Compacted steel is the page's own example, 2,250.

| Tile | HP | Ticks | Ours |
|---|---|---|---|
| Sandstone | 400 | 600 | 360 |
| Marble | 450 | 675 | 360 |
| Slate | 500 | 750 | 360 |
| Limestone | 700 | 1,050 | 360 |
| Granite | 900 | 1,350 | 360 |
| Steel, silver, gold, jade | 1,500 | 2,250 | 540 |
| Machinery | 2,000 | 3,000 | 540 |
| Uranium | 4,000 | 6,000 | none |
| Plasteel | 8,000 | 12,000 | none |

### Mining Speed by level (100 % Manipulation)

4 % + 12 % per level. Multiply by Manipulation (100 % importance, no maximum) and Sight (50 %, 100 % limit).

| 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 |
|---|---|---|---|---|---|---|---|---|---|---|
| 4 | 16 | 28 | 40 | 52 | 64 | 76 | 88 | 100 | 112 | 124 |

| 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|---|---|---|---|
| 136 | 148 | 160 | 172 | 184 | 196 | 208 | 220 | 232 | 244 |

### Mining Yield by level (100 % Manipulation)

Maximum 125 %. Manipulation 30 %, Sight 20 %, each with a 100 % limit. Applies to wall mining and the deep drill, not the drill's chunk rate.

| 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 |
|---|---|---|---|---|---|---|---|---|---|---|
| 60 | 70 | 80 | 85 | 90 | 92.5 | 95 | 97.5 | 100 | 101 | 102 |

| 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|---|---|---|---|
| 103 | 104 | 105 | 106 | 107 | 108 | 109 | 110 | 112 | 113 |

### Ore

| Ore | Yield | Ours | Vein size | Ours | Commonality | One vein, wiki | One vein, ours |
|---|---|---|---|---|---|---|---|
| Steel | 40 | 20 | 30~40 ("30+ tiles") | 6-14 | 1 | 1,200-1,600 | 120-280 |
| Silver | 40 | 25 | 4~12 | 4-9 | 0.10 | 160-480 | 100-225 |
| Gold | 40 | 8 | 2~8 | 3-6 | 0.07 | 80-320 | 24-48 |
| Machinery | 2 components | 2 | 3~6 | 1-3 | 1.00 | 6-12 | 2-6 |
| Uranium | 40 | surface none | 6~12 | none | 0.12 | 240-480 | none |
| Jade | 40 | surface none | 2~8 | none | 0.065 | 80-320 | none |
| Plasteel | 40 | surface none | 2~8 | none | 0.05 | 80-320 | none |

### Balance note

Per tile of plain rock, blocks halve rather than quartering. The wiki pays 25 % × 20 = 5 blocks at 1,600 ticks a chunk; we pay 1 chunk × `recipe.CutYield` 10 at 300 ticks. Chunks on the ground fall to a quarter, which clears the map too.

A tile of steel takes about 4.2× longer and gives twice the steel, so steel per hour of work is about half. With wiki veins there's far more steel on the map in total. Everything priced against today's numbers (build costs, trader stock, early-game pacing) should be looked at when MIN-1 ships. A short play test on the default difficulty is part of that phase. Vein size is MIN-3, but the per-vein column above means it should follow soon after.

## Rows

State: `done` shipped; `open` owed to a later change. Phase in brackets.

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| MIN-101 | Work per tile | HP × 1.5 | `MineWork` 360, ore ×3/2, `world.go:26`, `:122-127` | open [MIN-1] |
| MIN-102 | Speed by level | The table | `skill.SpeedPermil`, `roster/bio.go:25-30`, `skill/skill.go:122-128`: 58 % at 0, 106 % at 8, 178 % at 20. Plants already has its own curve there | open [MIN-1] |
| MIN-103 | Speed capacities | Manipulation 100 %, Sight 50 % | Manipulation through generic `WorkSpeed`, `body.go:97-99`; Sight absent | open [MIN-1] |
| MIN-104 | Yield by level | The table, cap 125 % | ±4 % a level around 7, 60..140 %, `skills.go:537-543`: 72 % at 0, 104 % at 8, 140 % at 20 | open [MIN-1] |
| MIN-105 | Yield capacities | Manipulation 30 %, Sight 20 % | Absent. `plant.YieldAmount` is the precedent (HRV-008) | open [MIN-1] |
| MIN-106 | Chunk per tile | "~25% chance" | Always 1, `world.go:22`, `:417` | open [MIN-1] |
| MIN-107 | Ore yield | 40 each, 2 components | Steel 20, silver 25, gold 8, `world.go:30` | open [MIN-1] |
| MIN-108 | Blocks per chunk | 20, at 1,600 ticks | `CutYield` 10, `Work: 300`, `recipe/recipe.go:487-492` | open [MIN-1] |
| MIN-109 | Chunk mass | Granite 25, marble 25, limestone 22, sandstone 20, slate 18 kg | All 25, `item/mass.go:46-47` | open [MIN-1] |
| MIN-201 | Mining removes support | A roof "will collapse if the last supporting structure within 6 tiles is removed, whether by mining, deconstruction, or destruction" | `roofOnMined` leaves a natural roof, `roof.go:426-433`; `checkRoofSupport` skips natural, `roof.go:409` | open [MIN-2] |
| MIN-202 | Thick collapse | "99,999 damage with 999% AP"; collapsed rock; the roof stays and it can happen again | None. Comment `roof.go:389`, "nobody is hurt" | open [MIN-2] |
| MIN-203 | Thin and built collapse | "between 15 and 30 damage with 0% AP", to neck and head; rubble filth; "approximately 33.3%" kill [Verify] | The roof goes, a message, nobody hurt, `roof.go:412-421` | open [MIN-2] |
| MIN-204 | Items under a collapse | Damaged, no number | Untouched | open [MIN-2], own number |
| MIN-205 | Removing a roof that holds others | Deletes the unsupported section with no collapse | Any unsupported built roof falls with a message, `roof.go:409-421` | open [MIN-2] |
| MIN-206 | Notice | Notification and pause, no advance warning | `g.message`, no pause, `roof.go:421` | open [MIN-2] |
| MIN-207 | Shape of the range | "roughly circular area 6 tiles around" | Chebyshev, a square, `world/roof/roof.go:34-36` | open [MIN-2]; changes built roofs too |
| MIN-301 | Surface uranium, jade, plasteel | Vein pages | `veinSpecs` has steel, silver, gold, machinery, `geology/generate.go:31-38`. Deep drill finds them, `industry/ore.go:29-36` | open [MIN-3] |
| MIN-302 | Vein size | The table | The table | open [MIN-3], after MIN-1 |
| MIN-303 | Deep drill auto-forbid | Summary only | `Drop` unforbidden, `industry/ops.go:147` | open [MIN-3], re-read first |
| MIN-304 | Scanner under a roof | "do not function under a roof" | Only `Powered` is checked, `industry/ops.go:79`, `:108` | open [MIN-3] |
| MIN-305 | Drill and scanner cost | Drill 100 steel, 2 components; scanner 150 steel, 4 components, 1 advanced | Drill 11 and 2 components, scanner 17, `construction/construction.go:801-804`; no scale found | open [MIN-3], check ARCHITECT first |
| MIN-306 | Infestation placement | Overhead mountain roof, above -17 °C, within 30 tiles of a colony structure, not a 1×1 room; less below -8 °C and in light | `HiveSpot` takes any roof, light under 30 %, above -17 °C, `raid_host.go:239-243`; `hiveInfest` 6 from the rock face, `raid/nest.go:55`. No 30-tile rule | open [MIN-3] |
| MIN-307 | Infestation pacing | At most once in 20 days; none at 30 hives | 1/8 of threat rolls from day 12, `raid/threats.go:40`, `:56`, no spacing. `HiveCap` 30 only limits spreading, `raid/nest.go:36` | open [MIN-3] |
| MIN-308 | Four-way mining | Summary only | Diagonals allowed, `world.go:428` | open [MIN-3], re-read first |
| MIN-401 | Roof overlay | Thin and thick are different things | Hover names both, `render/overlay/roof_draw.go:77-80`; the draw has one natural style, `:133` | open [MIN-4] |
| MIN-402 | Risk preview on Mine | No advance warning (ours goes further) | None. Support is on `roof.Cell` | open [MIN-4], ships with MIN-202 |
| MIN-403 | Roof in tile info | The tile's roof kind | `RoofLabelAt`, `hud/cellinfo.go:80` | done; no change |

`roof.go:27` and `world/roof/roof.go:13-14` say natural roofs can't be removed. Only the thick ones are refused (`roof.go:226`), which matches the wiki, so the comments should change with MIN-2.

## Proposed phases

- **MIN-1, numbers.** `mineWorkAt` reads tile HP; `MiningSpeedPermil` and a yield table beside `plant`'s; capacities; the chunk roll on the mining RNG stream; ore yields, cut yield, cut time, chunk masses. Golden re-pin, balance pass.
- **MIN-2, support and cave-ins.** A mined tile re-checks support; thick and thin behave as above; collapsed rock as a tile; damage through the existing `hurt` path; pause and letter. Package budget: `game` is at its ceiling, so this goes in a sub-package per `SOLID_GO.md` §10. Needs a no-flap test if a colonist retreats from a warning. Ships with MIN-402.
- **MIN-3, what's in the rock.** New surface veins and sizes, drill forbid, scanner roof rule, infestation rules.
- **MIN-4, seeing it.** Overlay and preview, through read models. MIN-402 moves up to go with MIN-2.

Order: MIN-1, then MIN-2 with MIN-402, then MIN-3, then the rest of MIN-4.

## Already matching

Support range 6 tiles. Losing is Fun 80 % (`difficulty.go:130`). Chunk cover 50 % (`cover/cover.go:37`). Drill cycle 14,000 (`industry/ops.go:23`), 200 W (`power/ax.go:29`), reach 21 cells (`industry/ore.go:154`), then stone chunks (`industry/ops.go:149`). Scanner 700 W (`power/ax.go:31`), maximum 360,000, mean 180,000 (`industry/ops.go:24`, `:130`). Scanner HP 200. Thick roof stops drop pods (`roof.go:29`). Built, thin and thick roofs are told apart in the save and the tile info. Mine in the work order (`WORK_PRIORITIES_GAP_ANALYSIS.md` row 12).

## Checklist cross-reference

Vanilla: 260 Mining skill `[-]`; 549 Mine designation `[x]` (stays); 600 Roofs `[-]` "no deterioration or collapse injuries" (MIN-2); 2478 Roof overlay `[-]` (MIN-401); 1791 Ore deposits `[x]` (MIN-3 re-opens the numbers). 1955 `[-]` and 2011 `[ ]` both describe infestation and disagree; MIN-3 should merge them. No row covers cave-ins or the risk preview, so MIN-2 adds them.
