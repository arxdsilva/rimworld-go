// Package industry is the colony's water, ground and deep-ore machinery
// (AX-4, ENG-003: the RimWorld wiki's numbers, original names and
// prose): the river's flow cells and the watermill that turns them into
// power, the deep ore deposits a ground-penetrating scanner finds and a
// deep drill brings up, the moisture pump that dries wet ground back to
// soil, and the wind turbines' clear band (moved here from the game
// package to pay for the rest).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the consumer-owned roles in deps.go, which
// the game package implements with a small adapter type (power_gen.go).
// The game keeps one State (in its bridge state, beside the sea mask);
// Save is its saved form.
//
// Finds draw no random numbers: a find's place, ore and size hash the
// colony seed and the find count. Building lists are cached per
// structure version; a pump dries only the cells in its own radius. No
// wall clock.
package industry
