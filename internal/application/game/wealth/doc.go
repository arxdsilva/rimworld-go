// Package wealth tracks the colony's wealth for the storyteller (EAI-014):
// the current value, a daily history for the graph, and their save.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package; the
// map's things come through the Source role, which the game itself
// implements (Game.WealthOf: stacks, buildings, tame animals, then the
// colonists' gear, in a fixed order). Each map keeps its own Tracker
// (built with the map's hosts).
//
// The count is a chunked sweep, never a full scan on a tick: the game
// calls Step every Stride ticks and each call sums one slice of the parts,
// so one full pass takes an in-game hour (Pass). The value is the last
// finished pass (a full count until the first one finishes); the first
// pass of each day is that day's sample, HistoryDays of them kept. The
// pass in progress (its hour, cursor and partial sum) is saved, so a game
// saved mid-sweep loads into the same state hash and ends the pass alike.
// It is deterministic: time comes only from the caller's tick, and the
// count draws no randomness.
package wealth
