// Package ship is the escape ship, the colony's way off the planet (AX-8,
// ENG-003: the RimWorld wiki's numbers, original names and prose).
//
// Six kinds of part make a ship: structural beams, stasis caskets, the
// computer core (built around a mind core), the reactor, engines and the
// sensor cluster. Parts that touch side to side are one ship; a casket
// joins only through a beam (wiki: Ship). The launch report lists what a
// ship still lacks: three engines and one of every other part, the
// reactor started and awake after its fifteen-day countdown, and at least
// one sleeper in a casket. Starting the reactor draws raids, people and
// machines by turns, until the countdown ends or the reactor is gone.
// The launch lifts the ship off the map with everyone asleep in its
// caskets, and the colony's story ends (WorldView.AX.Ending "ship").
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The game
// implements the roles in deps.go with one adapter (game/ship_host.go)
// and keeps one State, built on first use, off its world state. Ships are
// recomputed only when the structure version moves; raids are spaced by
// hashes of the colony seed, never a random draw or the wall clock.
package ship
