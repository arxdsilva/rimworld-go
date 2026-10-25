# Task Handoff

Task ID: SIM-055 (PLAY-015 §1)
Status: complete

## Summary
Allowed areas, rebuilt to the user's spec:
- **Domain** (`world/area`): stable ids (never reused), colours from our palette, `Delete`, `Invert` (padding bits untouched); old saves get ids 1..n.
- **Assignment refs:** an area id, Home (-2) or Unrestricted (-1), for colonists and tame non-penned animals. Every area command carries the id.
- **Delete** resets users and posts "N pawns are now unrestricted".
- **Soft rule:** destinations must be inside: work targets, the haul destination, food, beds (with no bed inside, the pawn sleeps on the floor inside), recreation, wander, and hunting (given up after the prey stays out 600 ticks).
- **Paths** may cross outside: +300% per cell, plus the fire avoidance cost. With no area and no fire, the grid query is unchanged.
- **Overrides:** drafted, mental break, downed, forced orders, fleeing, attack, rescue/tend/surgery, warm, hide, and animals following a drafted master.
- **Cleaning, repair and firefighting** need Home and the pawn's area.

## Files
- `game/areas.go`, `area_soft.go`, `area_dest.go`.
- Hooks in `tick.go`, `defense.go` (`pathQuery`), `orders.go`, `occupancy.go`, `food.go`, `idle.go`, `clean.go`, `combat.go`, `zones.go`, `host_commands.go`, `world_state.go` (save v2 with `HuntOut`).

## Notes
- Trained hauling animals don't exist, so there is nothing to restrict there.
- A bill's ingredient outside the area defers the bill.
