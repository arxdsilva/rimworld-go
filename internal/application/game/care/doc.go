// Package care looks after pawns that cannot look after themselves: who
// carries whom, rescue to a safe bed, tending, bed rest, the care policy,
// and the orders a player gives one pawn about another (SIM-061…065).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. Everything
// it needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game package implements with small adapter types in
// care_host.go. The game package builds one *Care and keeps one field.
//
// Care is deterministic: it draws no random numbers (tend quality is
// rolled by the host on the simulation's medical stream), reads time only
// from the host's tick, and iterates maps only in sorted order.
package care
