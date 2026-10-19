# SIM-013 Wealth-scaled raids, sieges and sappers

## What changed
- `item.Value` (rough unit values); `Game.Wealth()` = goods + buildings (2x wood +
  work/20, stone x1.5) + tame animals.
- `raidPoints(day)` = wealth/40 + 30 per standing colonist + 2 per day; `raidKit`
  spends them on raiders (club 60, rifle 90 at 55%), at least one, at most 8.
- `raidWith(kit, plan)` replaces the old Raid body; `Raid(n)` (assault), `Siege(n)`,
  `Sappers(n)` dev hooks. The storyteller (`scaledRaid`) picks: sappers (day >= 10,
  colony `enclosed()` from the edge, 50%), siege (day >= 12, 30%), else assault.
- `raidGroup` gains Plan, CampX/Y, Camped, Stormed, Started, Until (saved).
- Siege (`thinkSiege`): walk to a camp 18 tiles from `home()` towards their edge;
  dig in when there (or after 2h): sandbags in front of each besieger, danger
  message; hold and shoot whoever comes in range (never chase); storm after 5h.
- Sappers: `pathQuery` gives them `sapperMap` (doors shut, traps avoided);
  `startBreach` plans through walls, doors and rock (`sapperBreaks`); `doDig` hacks
  rock tiles (`rockDamage`, saved; 90 HP) into mined floor.
- PawnView `RaidPlan`; alert reads "Raid / Siege / Sappers: N raiders"; arrival
  messages per plan. `-scenario siege|sappers`.

## Found by the year probe, fixed here
- Smashed buildings leave a `Rebuild` blueprint (salvage moved aside); haulers and
  builders treat rebuild blueprints as `rebuildFirst` tiles closer.
- A breached room no longer jumps to the outside temperature: its cells keep a
  `residual` temperature (saved) that mixes with the outside air over a few hours
  (`residualLeak` 6%/interval), and a rebuilt room starts from it.
- Animals back off `pathRetry` ticks after a failed path search (manhunters shut out
  by a door were re-searching the whole map every tick); raiders wait `raiderRetry`
  after finding no way in.
- `edgeTile` only picks edge tiles from which the colony can be reached (walls and
  doors may be broken), so raids no longer spawn stuck in a cut-off pocket.
- Raiders give up and go home `raiderGiveUp` (a day) after arriving.
- The year probe now logs messages day by day (`RIMWORLD_PROBE=raiders` also logs
  each raider's state). With all of the above the probe colony lives through the
  year: eight threats including a winter sapper raid, walls smashed and rebuilt.

## Tests
`raid_plans_test.go`: wealth and raid size; siege digs in and storms; sappers tunnel
through a rock ring; sappers plan around traps while ordinary raiders don't.
`TestBreachedRoomCoolsOverHours`; the rebuild blueprint in `TestBuildersRepairDamage`.
