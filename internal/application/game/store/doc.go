// Package store is the storage buildings' own rules (STO-2): the zones
// shelves, hoppers, graves and sarcophagi lay over their footprint and
// the storage groups they link into (Buildings), the stacks marked to
// haul urgently (Urgent), and the nutrient paste dispenser fed by the
// hoppers beside it (Paste).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game package implements with small adapters beside
// the code they serve (zones.go). It is deterministic: no random
// numbers, no clock, no map iteration.
package store
