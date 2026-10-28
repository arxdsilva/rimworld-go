package ebiten

import (
	"strings"

	"rimworld-go/internal/application/game"
)

// Stone, ore and metal presentation (ART-015, SIM-029): the tints, the
// linked keys and the material helpers live in render/sprite, the stone
// cache with the ground tile memo in render/world (world.StoneArt); the
// titles and the Architect's default stone stay here (they read
// capitalize and stockOf).

// stuffAdjective names a non-wood material for titles: "Granite",
// "Steel", "Stone" (legacy); "" for wood.
func stuffAdjective(stuff string) string {
	if s := stuffStone(stuff); s != "" {
		if stuff == "stone" {
			return "Stone"
		}
		return capitalize(s)
	}
	if stuff == "steel" {
		return "Steel"
	}
	return ""
}

// materialTitle is "Granite wall" / "Steel door" / "Wall".
func materialTitle(kind, stuff string) string {
	if adj := stuffAdjective(stuff); adj != "" {
		return adj + " " + kind
	}
	return capitalize(kind)
}

// ruinTitle is "Ruined granite wall" / "Ruined wooden floor".
func ruinTitle(b game.BuildingView) string {
	adj := strings.ToLower(stuffAdjective(b.Stuff))
	if adj == "" {
		adj = "wooden"
	}
	return "Ruined " + adj + " " + b.Kind
}

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
