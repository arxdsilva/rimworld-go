# CBT-4: combat AI

Checklist 17.8 (target selection, cover seeking, threat response, grenade avoidance), 24.3/24.4 raid leftovers and steal, 24.6 hives, 19.4 prison breaks, tantrum.

## What changed
- Raiders: threat-weighted targets, cover seeking that holds its spot, grenade dodging, loot grabbed on retreat, crop burning, siege resupply, a staged assault timer of 10-15 h (`domain/raid` rules). Escaped prisoners fight their way out.
- Mechs: bash walls and shut doors, arrive by drop pod, never retreat (wiki); EMP through `explosion.Adapted`, shared with turrets.
- Colonists: drafted fire at will and threat-response cover steps; threat response Flee, Attack or Ignore (the lead added `SetThreatResponse` and `PawnView.ThreatResponse`). Hunters fetch any non-explosive ranged weapon. Stalking skipped: the wiki page was missing and it is not in core.
- Tantrum: Major tier, 8000-12000 ticks, shown as "tantrum", smashes nearby objects.
- Prison break: rooms join per the wiki rule.
- Hives: construction `HiveMound` ("hive mound"), wiki 130 HP, cap 30, spread within 10 tiles, -17 C.
- Kidnap moved into `game/raid` (`kidnap.go` deleted). Doors are forbiddable through `BuildingForbid`.
- Game about -90 lines, -1 file, -7 Game methods.

## Wiki versus original
Wiki: hive HP, cap, spread radius and temperature; mech cluster rules; prison room-joining rule. Original tuning: prison break timing (60-day mean time, 3-day grace, 10-day calm), hive breed and spread rates, cover numbers, tantrum length.

## UI
Threat response control and door forbid (see CBT-5).

## Tests
`raid_cbt4_test.go`, `TestThreatResponseAndSmokepopCommands`; no-flap tests for cover seeking and threat response. Raid frame p99 HEAD to now: 1.80 to 1.91 ms (250), 2.07 to 2.10 ms (500).

## Gaps
Stalking skipped. Mech voices blocked. Wiki hive dark/warm placement not verified.
