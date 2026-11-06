# PLAY-023: zones placed by RimWorld's rules, dumping stockpiles, storage priority

The user asked when and why zones may be placed, and wanted it to match RimWorld. This uses original values.

## Placement (`zoneRefusal`, with the reason shown in the UI)

| Applies to | Refused when | Reason |
|---|---|---|
| Every kind | on the outermost map ring | "Too close to the map edge" |
| Every kind | fogged | "Unexplored" |
| Every kind | already zoned | "Another zone" |
| Every kind | rock wall | "Rock" |
| Every kind | wall, impassable building or filled grave | "Building" |
| Every kind | blueprint of a blocking building | "Planned building" |
| Growing | water | "Water" |
| Growing | bridge | "Bridge" |
| Growing | terrain other than soil, rich soil, gravel or marsh (`terrain.GrowSoil`) | "Not soil" |
| Growing | built or planned floor | "Floor" |
| Stockpile, dump | deep water (shallow water is now allowed) | "Water" |

- A load dropped in water still floats to dry ground unless a stockpile on that cell takes it.
- Saved zones migrate on the ground rules only, so they keep their edge and fogged cells.

## Changes under a zone
- When a building finishes or terrain changes, zoned cells under it are re-checked and refused ones removed (`RemoveTiles`).
- A cut zone splits: the biggest piece keeps its id, and new ids go in row-major order.
- One plain message per zone, e.g. "Growing zone shrank: floor built".
- Not covered: removing a bridge.

## Dumping stockpile and priority
- `zone.KindDump` (appended): animal corpses and stone chunks only. Painted by `DesignateStockpile` with `Build=1`, from the new "Dumping stockpile" tool (it reuses the stockpile icon).
- Priority Low / Normal (zero) / Preferred / Important / Critical on stockpiles and dumps, set with the new `SetZonePriority` command, from a picker and a pane line.
- Haulers search from the highest priority down. A stored stack moves only to a strictly higher priority with room, after at least 1 in-game hour (the hysteresis; `ZoneMap.StoredAt`, empty while everything is at one priority).

## Other
- Sowing happens only while the growing temperature is between 0 and 50 °C.
- The stockpile and growing tool descriptions are fixed.
- `TestBridgeZones` was testing the wrong kind string; that is fixed.

## Verify
- 14 new tests plus `TestSetZonePriorityIgnoresBadTargets` and `TestLowOnlyStorageStays`, and ebiten tool, tooltip and picker tests.
- Existing tests that used edge cells or sand were updated (listed in the agent report: zones_test, `TestIceNotSowable`, `TestBridgeZones`, four ebiten zone tests).
- The full suite passes under `-race`.
- No golden moved.
