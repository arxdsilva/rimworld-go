// Package research is the colony's research (RSCH-1, ENG-003: the
// RimWorld wiki's numbers, original names and prose): the project the
// colony works on, the research bench job, the wiki research speed
// (0.00825 points a tick x the researcher's research speed x the bench
// factor x the difficulty factor), the tech-level cost multiplier, bench
// tiers and the multi-analyzer link, the research-complete letter, the
// "need a research project" alert and the Research tab's read model.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the consumer-owned roles in deps.go, which
// the game package implements with small adapter types (research.go).
// The game keeps one *Lab; its Prog, Cur, Set and Carry are saved.
//
// Per-bench analyzer links and the research bench list are cached per
// structure version; a working tick reads one bench's room and its
// cleanliness, never the map. No random numbers, no wall clock.
package research
