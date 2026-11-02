# MAP-2: attacking settlements and pod modes

## What changed
- `domain/world/mapgen/settlement.go`: original compound generator with loot and, for industrial factions, turrets. `raid.Defend` strategy (`domain/raid/raid.go`, `game/raid/defend.go`): hold, engage within sight, flee at 50 % losses.
- `AttackSettlement`: -200 goodwill and hostile, +20 from the faction's enemies, defeat after 3 hourly census results, destroyed settlements leave `planet.SettlementTiles()` (`maps.Prune` reapplies it after a rebuild), "Settlement destroyed" letter, then the 24 h pack-up. A faction with no base left is defeated. Tests in `maps_settle_test.go` (compound, hold and engage, flee, no-flap, destruction, reasons, same seed and hash).
- Pods: Attack edge and centre, Found, Spot, Drop, DropCentre on any open map (`travel/launch.go`, `maps_pods_test.go`); the "not in this game" refusal is gone.
- Storyteller target: `streamStory`, weighted by colony wealth, no draw with one colony, sites and encounters never picked (`TestThreatTargetsColonies`).

## Gaps
- Settlement turrets are inert (no hostile turret fire).
- Compound doors are held open so defenders can move, so colonists can walk through them too.
- World view: pod range ring and anchored landing menu (edge, centre, attack edge or centre, found, gift, visit, pick spot) in `worldview/pods.go`; Visit is refused without a settlement; `TestJourneyWorldPodModes`, `PodPickSpot`, `PodFound`.
- Known behaviour: a caravan attacks hostile bases only (plan 2.6); pods can attack any human base, with the -200 goodwill rule.
- Open for the user: inert turrets and held-open compound doors.
