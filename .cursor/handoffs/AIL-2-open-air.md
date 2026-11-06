# AIL-2: heatstroke victims are carried into cool open air

The player's autosave (`colony-2026-10-27-1459-autosave.rwsave`) still held two colonists down after AIL-1. Every room in that colony stayed above their comfortable maximum long after the heatwave ended outside. AIL-1's fallback, "the nearest spot whose air heals", went through `thermo.Spot`, and `Spot` searched only enclosed rooms. The cool evening air was never a candidate, so no doctor moved them and they lay for more than 30 simulated hours.

## What changed
- `thermo/outdoors.go`:
  - `Spot` also searches open-air cells when the caller finds `room.Outdoors` bearable.
  - `outdoorSpots` grows Chebyshev rings out from the pawn, up to `outReach` (64 cells). It stops once the twelve nearest candidates are certain, so it never scans the whole map.
  - `Model.Air(r, out)` returns a room's air, or the outdoor air for `room.Outdoors`.
- `heat.go` `warmSpot`: the bearable test reads `g.thermo.Air(r, out)`, so the open air counts.
  - Bed choice is unchanged. A bed in a room whose air heals still wins, and the open air is a spot, not a bed.

## Results
- On the player's autosave, through a temporary diagnostic test that is not committed:
  - At h22 the outdoor air falls to 26.5 °C, and both colonists are carried outside.
  - By h24 both are standing, at 14–16 % heatstroke. By h26 both are under 10 %.
  - Without the fix, neither had moved after 30 h.

## Tests
- `TestHeatstrokeVictimWithNoCoolRoomIsCarriedIntoTheOpenAir` (game): every room is held at 32 °C and the air outside is 22 °C.
  - She is carried out once, stands, and is not carried back for 2 h.
  - It fails without the fix: she is never carried and never stands.
- `TestSpotMatchesFullScan` (thermo): half its trials now count the open air as bearable, and it still matches the full-map scan.
- `TestAirIsTheRoomsOrTheOpenAirs` (thermo).

Player-facing: AIL-1's "Not recovering" note and alert clear once the colonist is outside. No new UI.
