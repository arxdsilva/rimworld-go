package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Stone, ore and metal presentation (ART-015, SIM-029): the tints, the
// linked keys and the material helpers live in render/sprite; the titles,
// the Architect's default stone and the ground tile memo stay here.

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

// stoneArtCache is the painter's stone cache (sprite.StoneCache) and the
// ground tile memo per terrain code for one catalog (draw_art.go).
type stoneArtCache struct {
	sprite.StoneCache
	tilesArt *artCatalog
	tiles    [][soilVariants]tileKeyEntry
}

type tileKeyEntry struct {
	key  string
	cs   colorScale
	done bool
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

// stoneCache is the renderer's stone cache with its ground tile memo
// cleared when the catalog changed.
func (r *Renderer) stoneCache() *stoneArtCache {
	c := &r.stoneArt
	if c.tilesArt != r.art {
		c.tiles, c.tilesArt = nil, r.art
	}
	return c
}
