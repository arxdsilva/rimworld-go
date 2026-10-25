# Task Handoff

Task ID: SIM-056 (PLAY-015 §2)
Status: complete

## Summary
Pens are `construction.PenMarker` buildings (10 wood, Architect → Misc).

**Enclosure:**
- a flood fill from the marker over cells a penned animal can stand on, bounded by fences, closed gates and doors, walls, rock, deep water and movement-blocking buildings;
- the pen is open if the flood reaches the map edge or exceeds 2,500 cells;
- it is recomputed lazily on structure changes;
- an open pen raises the alert "Pen N is open".

**Penned species** (29, `animal.Roster.Penned`): farm animals, tamed grazers and herd birds. They can't pass gates or doors unless a handler ropes them.

**Rope job** (Handling): 60 work to rope; the animal follows the handler through gates and is released in its target pen, the closest reachable enclosed pen that accepts its species and sex.

**Pen settings** (`SetPenSettings`): species allow and sex filters, and an auto-cut plant list (every 2 hours). Food figures: grazing per day vs demand per day, plus trough stock.

**Unpenned livestock:**
- roams (radius 30), with the alert "Unpenned livestock";
- after 3 days unpenned, an animal 40+ cells from Home has a 12% daily chance to wander off;
- roping or penning resets the clock, which is saved in `PenAnimals`.

**Migration:** painted pen zones become markers on load. Scenario set pieces use markers.

**Follow drafted:** an Animals-tab toggle (`SetAnimalFollow`; `animal.State.NoFollow`; zero means it follows as before).

## Files
- `game/pens.go`, `rope.go`, `pens_contract.go`.
- `fences.go`, `handling.go`, `animal_needs.go`, `animal_flee.go`, `trough_feed.go`, `animal_life.go`, `alerts`.
- `scenario/grown.go`, `cmd/game/main.go`.
- `train_animals.go` (follow).

## Notes
- Roaming livestock avoid wander targets toward visible predators (this keeps flee from flapping).
- `DesignatePen` still paints inert zones for old-save compatibility; the Architect tool is gone.
- The Master dropdown lists all colonists, not only capable handlers.
