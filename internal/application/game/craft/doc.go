// Package craft is production: bench bills and their work, drying racks,
// the tool-cabinet bonus and the electric benches (PRD-0). Later phases
// add quality rolls, stuff choice, art and deterioration hooks here.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. Everything
// it needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game package implements with small adapter types in
// craft_host.go. The game package builds one *Craft and keeps one field.
//
// Craft is deterministic: it draws no random numbers (a meal's poison is
// rolled by the host), reads no clock, and iterates maps only where the
// order cannot show in the result.
package craft
