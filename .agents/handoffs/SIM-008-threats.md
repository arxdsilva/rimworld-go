# SIM-008 Threats: raids and winter wolf packs

## What changed
- `internal/application/game/raids.go`: raiders are pawns kept apart from the colony
  (`raiderIDs`, `raiders`, `raidGroups`). Group modes: arrive → assault → flee.
  `thinkRaider` targets the nearest standing colonist within sight range (18),
  fires with a rifle or closes with a club. Badly hurt raiders, or a group that has
  lost half its members, flee to the map edge and leave. Downed raiders bleed out
  after 4 h and drop their weapon; corpses linger half an hour.
- Storyteller (`scheduleThreats` / `phaseThreats` / `fireThreat`): first threat on
  day 5, then every 5–9 days. In winter there is a 50% chance of a 2–3 wolf pack
  instead of a raid. Raid size is `min(5, 1+day/10)`. `Raid(n)` is the dev hook.
- Colonists defend against raiders attacking them (14 tiles), auto-engage standing
  raiders and accept drafted "Shoot/Melee attack raider" orders.
- Save format carries raiders, groups, next threat tick and the on/off switch.
- Adapter: hostiles are rendered but excluded from selection, the colonist bar and
  colonist alerts. A red chevron marks raiders (orange while fleeing). Raiders wear the
  reserved `raider` cloth tint (colonists and beds never draw it). A "Raid: N raiders"
  danger alert is shown.
- `-scenario raid` spawns three raiders at once (stress test).

## Tests
`raids_test.go`: an armed colonist beats a raider; a hurt raider flees; a downed
raider bleeds out and drops its weapon; the storyteller sends the first raid; and
save/load mid-raid stays in lock step.

## Follow-ups
- Pause-on-threat option, damage numbers.
- Raid strategies (siege, sappers), point budget tied to colony wealth.
- Manhunter state for animals hurt by colonists.
