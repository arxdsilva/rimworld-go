// Package wealth tracks the colony's wealth for the storyteller (EAI-0
// skeleton): the current value, a history of samples for the graph, and
// their save.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package; the
// full count comes through the Counter role, which the game itself
// implements (Game.Wealth). Each map keeps its own Tracker (built with
// the map's hosts).
//
// Today a Tracker only wraps one full pass: Value counts the whole map
// unless Step counted it already, Step recounts and takes no sample, and
// the history stays empty. Nothing in the tick calls Step yet. It is
// deterministic: time comes only from the caller's tick, and the count
// draws no randomness.
package wealth
