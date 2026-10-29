// Package mind runs mental breaks: when a colonist's mood gives way it
// stops taking orders and acts out for a while (wanders, binges, hides,
// insults people, goes berserk, hunts one person, or collapses), then
// recovers with catharsis (SIM-066).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game implements with small adapter types in
// mind_host.go. The game builds one *Mind and keeps one field.
//
// The break record itself (kind and end tick) stays on the game, which
// other clusters also write (social fights, dementia); mind keeps only the
// target of a break aimed at someone and its retarget tick.
//
// Mind is deterministic: every roll comes from the host (a game random
// stream), time only from the host's tick, and it iterates no maps.
package mind
