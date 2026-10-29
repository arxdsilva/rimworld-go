# BED-1: bed stats, bedrolls, animal beds, bed furniture, forbidding buildings

Date: 2026-10-29. Wave B (committed with PRD-1 and DEF-1). Backlog `.cursor/BEDS_GAP_ANALYSIS.md`.

## What changed
- **New kinds** (B1, opus; `construction.go`, `construction/furniture.go`):
  - bedroll and double bedroll: cloth or leather (`Def.Fabric`), movable/reinstallable;
  - animal sleeping spot, animal bed;
  - end table, dresser (walkable).
  - Costs and work are the wiki's, scaled like the bed's.
  - Art (A1, opus): new Blender family `bedroom` with blanket overlays and architect icons, wired in `render/world/build_keys.go`, `render/creature/pawn.go` and `architect/catalog.go`.
- **Stats** (`rest/stats.go`):
  - comfort, rest effectiveness and beauty from the wiki, by kind × stuff (stone keeps 90 % rest) × quality (Normal until PRD-2);
  - an end table beside the bed head and a dresser within 6 tiles in the same room each add +0.05 comfort.
  - **Sleep gain is the game's base rate × rest effectiveness** (`needs.SleepTickAt`). The game drains rest faster than vanilla, so the wiki's absolute rate isn't used. Ground and spots are 0.8, so ground sleep is slower.
- **Animals** (`rest/animals.go`): pets claim an animal bed when they bed down; penned livestock don't. No-flap test for pets keeping their bed.
- **Forbid:** `Building.Forbidden`, set by `SetForbidden` with a building id; colonists and pets skip a forbidden bed.
- **Budget:** lovin' moved into `rest/lovin.go` to pay for the additions.
- **Read model:** `BuildingView.Comfort`, `RestEffect`, `Beauty`, `Facilities` (bitset `FacilityEndTable`/`FacilityDresser`), `Forbidden`.
- **UI** (U1): the bed stat lines and a Forbid toggle on every building. Lead fixes from the journeys: owner slots now follow the construction kind (bedrolls included); the forbid click is routed.
- **Tests:** `bed_stats_test.go`, rest stats/animals/lovin tests; journeys `TestJourneyBedrollSleep`, `TestJourneyForbidBed`. `coversCell` is now allocation-free for small footprints; the quiet-tick allocation test is back at 179.

## Follow-ups
- Bedrolls are movable and reinstallable (the move order), but can't be picked up and carried: the game has no minify/carried-building item yet.
- Medical animal beds.
- Care, benches and doors don't check Forbidden yet.
- The stuff picker rejects leather.
- AssignOwner for animal beds.
