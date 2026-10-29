// Package rest is sleeping: where a pawn sleeps (its bed, its partner's
// double bed, the floor), who owns which bed, the sleep job driver and
// when a sleeper gets up, and the thoughts it wakes with (BED-0).
//
// The root type is Sleep, not Rest, so it never reads as care.Rest: bed
// rest for patients stays in the care package (care/rest.go). Room stats
// are domain/world/room's, fed by the game's room index; rest turns a
// room's role and impressiveness into thoughts (room_mood.go, BED-2).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. Everything
// it needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game package implements with small adapter types in
// rest_host.go. The game package builds one *Sleep and keeps one field.
//
// Rest is deterministic: it draws no random numbers, reads no wall clock,
// and walks beds in the host's building order.
package rest
