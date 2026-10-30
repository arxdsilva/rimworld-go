// Package haul is stockpile hauling's own rules (HAUL-002): the load a
// hauler gathers from nearby piles before it delivers (Plan, Gather), the
// room a storage cell has for a load (Cell, Capacity, Toppable) and the
// load split off a pile (SplitLoad).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned role in
// deps.go, which the game package implements with a small adapter beside
// the haul code it serves (tick.go). It is deterministic: no random
// numbers, no clock, no maps.
package haul
