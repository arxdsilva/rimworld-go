// Package bodytemp is exposure for the simulation (TEMP-3): the ledger of
// every pawn's and animal's hypothermia and heatstroke severities, the
// animals' exposure on their 60-tick decision cadence (Animals, a hook in
// animal_needs.go: never a scan of every creature per tick) and the
// warmth-seeking spot search (Spot). The formulas, stages and frostbite
// rules are the domain's (domain/pawn/exposure, domain/pawn/body
// cond_frostbite.go); the game reaches in through consumer-owned roles
// (.cursor/SOLID_GO.md §7, §10).
package bodytemp
