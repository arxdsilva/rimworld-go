package ebiten

import (
	"rimworld-go/internal/application/game"
)

// Stone, ore and metal presentation (ART-015, SIM-029): the tints, the
// linked keys and the material helpers live in render/sprite, the stone
// cache with the ground tile memo in render/world (world.StoneArt); the
// titles and the Architect's default stone stay here (they read
// capitalize and stockOf).

// defaultStoneStuff is the stone material the Architect offers first: the
// first stone blocks (enum order) with free stock, else granite blocks.
func defaultStoneStuff(w game.WorldView) string {
	for _, s := range stoneNames {
		if s == "" {
			continue
		}
		if k := s + " blocks"; stockOf(w, k) > 0 {
			return k
		}
	}
	return defaultStone + " blocks"
}
