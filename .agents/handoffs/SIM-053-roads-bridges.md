# Task Handoff

Task ID: SIM-053 (PLAY-014, MAPGEN-3 §2–3)
Status: complete

## Summary
**Roads (mapgen):**
- **Bridges:** roads cross river and lake water on straight, 2-wide wooden bridges from dry bank to dry bank, snapped to the river's normal, and never over the sea (`Result.Ocean`). Fords no longer exist for roads.
- **Routing:** river/lake water costs 12 per cell, diagonal water steps 2.2×; vault footprints and the open sea are impassable. Rock 7+ deep is impassable on the first try; the retry cuts at 25.
- **Exports:** `Route` and `Lay` are exported.
- **Validation:** `roadsConnected` re-routes a road a repair broke ("road N re-routed") or removes a road it can't fix. Bridges count as walkable for reachability.

**Bridges (game):**
- **Structure:** a foundation layer (`construction.LayerFoundation`, kind `Bridge`, wood 6, work 110, refund 3), unlocked by the Bridges research.
- **Where it can go:** water, marsh or mud, never the open sea (a saved ocean bitset; legacy maps have none).
- **Walking:** `cellWalkable` is used at every walkable-bit site. A bridge walks at 100% and is never "Soaked"; on water it counts as dry footing; stockpiles are allowed on it, growing zones are not.
- **Load:** light only (`construction.Requires` vs `Provides`). Wooden walls, doors, fences and furniture are fine; stone or steel walls, floors, carpets and graves are refused ("needs firmer ground than a bridge"). On marsh or mud the ground's own strength still counts.
- **Building a span:** over deep water it grows from the shore; haulers skip unreachable bridge frames, which avoided a pick-up/drop loop.
- **Removal and destruction:** deconstruction is refused while in use. A destroyed bridge breaks what needed it (no refund); items on deep water sink, with one message per bridge; pawns and animals move to the nearest walkable cell.
- **Generated maps:** road bridges are laid as finished decks.

**Art and UI:**
- `structure/bridge_00..15`: flat plank decks, with edge beams on the sides open to water; the mask is N1 E2 S4 W8.
- A water shadow on open edges.
- The Architect Bridge tool, the inspector line, and the hover line "Bridge over deep water / marsh / mud".

## Notes
- There is no fire system yet (`domain/world/fire` is a stub), so bridges don't burn.
- The plank direction mismatches where a horizontal and a vertical span meet.
