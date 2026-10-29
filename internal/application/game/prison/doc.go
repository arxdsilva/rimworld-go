// Package prison keeps the colony's prisoners: their state, their life
// on the raid roster and the warden's work (ENG-003, M8).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game implements with small adapter types in
// prison_host.go. The game builds one *Prison and keeps one field.
//
// Prison is deterministic: every roll comes from the host (the game's
// prison stream), time only from the host's tick, and it never ranges
// over a map.
package prison
