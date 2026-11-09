// Package rage is the animals' fight-or-flight brain: manhunter rage and
// manhunter packs, tame guards, predators' hunts, charges and pack calls,
// fighting back and fleeing (where to run, and whether a foe can be
// reached), and the chase warnings the player sees (PLAY-010, PLAY-017).
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package.
// What it needs from the simulation comes through the consumer-owned roles
// in deps.go, which the game implements with small adapter types at the
// end of game/handling.go (rageOf builds one *Rage on first use and keeps
// it in the wildlife state).
//
// Its state lives in the animals' animal.State and in Memory (memory.go),
// which the game keeps in its wildlife state and saves. It is
// deterministic: every roll comes from the host's main stream in the order
// the game rolled before the extraction, time only from the host's tick,
// and sight queries visit creatures in the host's index order (ties to the
// lower id).
package rage
