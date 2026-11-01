# TEMP-6: close-out simulation (agent Z)

Wiki fidelity under ENG-003 (temperature). Gap rows: `.cursor/TEMPERATURE_GAP_ANALYSIS.md` §3.0 (TEMP-0 row closed), §3.2, §3.3, §3.5, §3.6.

## What changed
- **Outdoors judgements** (`game/thermo/outdoors.go`, wiki: Rooms). `OutdoorsForWork(r)` is true above 25 % unroofed or above 100 unroofed cells. `PsychOutdoors(r)` is true at 300 or more unroofed cells, or for the edge-open outdoor area with at least 50 % unroofed. Both read cached counts.
  - Surgery (`surgery.go`) takes the outdoor factor in a room that is outdoors for work.
  - Three things go by psychologically outdoors: the 80 % bench speed and the food-poisoning kitchen roll (`craft_host.go` Cleanliness), and "slept outside" (`rest/waking.go`, host `Outside` replaces `UnderSky`).
  - So a small walled yard with no roof no longer counts as outdoors, and a roofed lean-to open to the map edge does.
  - Research speed has no room factor in this game, so OutdoorsForWork has nothing else to drive.
- **Room cap** (`room.MaxCells` 5184, wiki: Rooms). Past the cap, an area is "a non-room indoors space": `RoleOf`/`RoleOfFull` give General, so it gets no room role or room thought. Its air still behaves like a room's.
- **Ignition and burns** (`fire.go` scorch, run after every room step):
  - Flammable cells in air at 235 °C or hotter catch fire (`thermo.IgniteAt`, wiki). The chance is 0.5 % per cell per 120 ticks at 235 °C, rising to 20 % at 1000 °C. Rolls use a hash, as spread does. One spontaneous fire per 250 ticks map-wide (`IgniteCooldown`, saved in `thermo.Save.IgniteAt`). The rate and cooldown are original.
  - A creature in air more than 150 °C over its comfortable maximum is burned (`exposure.BurnMargin`, wiki). Damage is 1 HP, plus 1 per further 40 °C, at most 6, every 480 ticks (original). Pawns take burn wounds; animals take injury.
  - Nothing runs while `thermo.Hottest()` is at or below 150 °C.
  - `burnCreature` is shared with fire damage.
- **Warmth-spot cache:** `thermo.Spot` walks only the cached per-room cell lists of bearable rooms, nearest bounding box first, and stops once no room left can beat the 12th candidate. A randomised test shows it returns the same spot as the old full scan. `bodytemp.Spot` is deleted.
- **Drying rack:** vanilla core has none (the wiki page is a 404), so `item.DryRate` is marked original tuning.
- **N/A rows:** crematorium heat (the game has no crematorium) and insect hunger (hive insects are `NeedsFree`).

## Paid lines
The aging Health-tab text moved from `game/aging.go` to `domain/pawn/body/aging_text.go` (`AgingLabel`, `AgingTip`, `AgingOnsetText`, `AgingHeadline`, `AgingHeads`).

| | Before | After |
|---|---|---|
| Game package lines | 53783 | 53709 |
| `Game` methods | 1906 | 1904 |
| Files | 178 | 178 |

The lead re-pins the budget.

## Tests
- `thermo/outdoors_test.go`: both judgements at their edges, spot equivalence, near-rooms-only, ignite chance, hot rooms and the saved cooldown.
- `room_test.go` TestRoomCapTakesNoRole.
- `exposure_test.go` TestBurnDamage.
- `game/temp6_test.go`:
  - ignition at 300 °C, none at 234 °C, and the cooldown no-flap;
  - burns on a cadence while the air wobbles across the line (no-flap), and a cow burned;
  - TestFreezerKeepsMeatFrozen;
  - the outdoors judgements through the real hosts.
- `roof_test.go`: TestSleptUnderRoofNotOutside became TestSleptOutsideIsPsychologicallyOutdoors. Under the wiki rule, a pavilion open to the map counts as outdoors.
- Determinism gate: goldens unchanged.

## Probes (`scenario/temperature_test.go`, RIMWORLD_PROBE=1, about 2.5 min with -race)
Four colonists, born at 06:00 on the first day, with a wood-fired generator and a battery.

- **Winter, heated** (temperate, days 38-52): all 4 alive. Bedroom 11.1-21.0 °C, outside -27.0 to -1.8 °C. No hypothermia, no frostbite.
- **Winter, bare:** all 4 dead of hypothermia on day 39. Bedroom -21.3 to -3.2 °C, hypothermia peaked at 968 ‰, all frostbitten.
- **Summer, cooled** (desert, days 7-21, heat wave on day 12): all 4 alive. Bedroom 10.2-23.1 °C, outside 4.2-50.8 °C. Heatstroke peaked at 137 ‰ (outdoor chores).
- **Summer, bare:** all 4 alive. Bedroom up to 46.6 °C, heatstroke peaked at 554 ‰.
- **Freezer:** held at -5.0 °C from day 21 with up to 35.8 °C outside. The meat inside stopped rotting; the meat outside spoiled.

## UI
No new view is needed:
- fires already show;
- burns appear as wounds;
- "slept outside" appears in the thoughts;
- bench speed shows in the building pane;
- an oversized area shows role "room".

## Open
- **Power balance (lead):** the wood-fired generator's tank lasts about 0.94 days (15 logs, burn ×1.6). The wiki's is 75 wood at 22 a day, about 3.4 days. Haulers refuel at half, but only in work hours, so a heater on a generator alone goes dark for an hour or two every night. The probes add a battery. Not changed here (PLAY-021 power).
- Weapon-fire ignition sources (vanilla checklist Ignition row).
- The outdoor area is judged as one room (every edge-open cell). Vanilla splits disconnected outdoor areas.
