package game

import "rimworld-go/internal/domain/world/planet"

// OpenBenchMaps gives g the test planet and opens n site maps on plain land
// far apart, for BenchmarkMultiMap: how many opened.
func OpenBenchMaps(g *Game, n int) int {
	p := testPlanet()
	w := g.worldS()
	w.planet, w.planetTried = p.Fork(), true
	opened := 0
	for t := 0; t < p.TileCount() && opened < n; t++ {
		if p.Water[t] != planet.Land || p.Settlement[t] >= 0 {
			continue
		}
		if _, ok := DevOpenMap(g, t); !ok {
			return opened
		}
		opened++
		t += 50
	}
	return opened
}
