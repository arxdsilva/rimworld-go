// Package travel is the colony's ways off the map and out of time (AX-9,
// ENG-003: the RimWorld wiki's numbers, original names and prose): the
// stasis casket that holds a sleeper with needs, aging and illness
// suspended; transport pods loaded by weight, fuelled at their launchers
// and fired to a world tile (a drop, a gift, a visit); the long-range
// mineral scanner that finds world mining sites; and the caravan hitching
// spot where a forming caravan gathers before it leaves.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the consumer-owned roles in deps.go, which
// the game package implements with a small adapter type (world_state.go
// travelHost). The game keeps one State in its world state (worldSt.ax9,
// nil until first used); Save is its saved form, carried in the world
// save block.
//
// Sleepers and passengers are the game's stashed pawns (world_stash.go):
// off the map, so no per-pawn loop ticks their needs. Passengers who land
// away from home become an ordinary caravan (world_state.go), and the
// pods' goods and animals ride along as its cargo until it comes home.
//
// Nothing here draws a random number: a mineral find's work, tile,
// mineral and size hash the colony seed and the finds so far. Pod groups
// are cached per structure version. No wall clock.
package travel
