// Package combat resolves fighting: line of sight and cover, strikes and
// their hit rolls, defending against attackers, spike traps, breaching,
// the sentry turrets (DEF-0), and projectiles in flight with worn armor
// (DEF-1; later explosions and mortars, per
// .cursor/DEFENCE_GAP_ANALYSIS.md).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game implements with small adapter types in
// combat_host.go. The game builds one *Combat and keeps one field.
//
// Applying harm (wounds, death, building damage and rebuild blueprints)
// stays on the game, which every other cluster also calls; combat decides
// who is hit and for how much, and calls Harm. Raids (game/raid, DEF-4)
// reach Strikes, Breach and Turrets through the game.
//
// Combat is deterministic: every roll comes from the host (the game's
// main or defence stream, as before the extraction), time only from the
// host's tick, and it iterates maps only where the result is order-free.
package combat
