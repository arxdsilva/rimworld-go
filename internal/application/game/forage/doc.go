// Package forage is a tame animal's food search rules (FEED-001): which
// cells it can reach on its movement layer (Reach), the best meal of its
// search with animal feed ahead of meals (Pick), and what it remembers
// between searches (Memory: the spots it gave up on, and whether it is
// starving and searching far).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package and needs
// nothing from it beyond the region labels it is handed (Labels, a
// consumer-owned role the region map satisfies). It is deterministic: no
// random numbers, no clock; maps are only looked up, never iterated for
// a decision.
package forage
