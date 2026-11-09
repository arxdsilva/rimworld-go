package game

import "rimworld-go/internal/application/game/pathstage"

// PathStageStats is the current map's path stage counters (benchmarks).
func PathStageStats(g *Game) (runs, requests uint64) { return g.regs.paths.Stats() }

// SetPathPool runs every open map's path stage on p (benchmarks; nil: serial).
func SetPathPool(g *Game, p pathstage.Pool) {
	for _, m := range g.maps {
		m.regs.paths.SetPool(p)
	}
}
