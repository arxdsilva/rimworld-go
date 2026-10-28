package world

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
)

// StoneArt is the painter's stone cache (sprite.StoneCache) and the
// ground tile memo per terrain code for one catalog (draw_art.go).
type StoneArt struct {
	sprite.StoneCache
	tilesArt *art.Catalog
	tiles    [][SoilVariants]tileKeyEntry
}

type tileKeyEntry struct {
	key  string
	cs   sprite.ColorScale
	done bool
}

// stoneCache is the stone cache with its ground tile memo cleared when
// the catalog changed.
func (b *base) stoneCache() *StoneArt {
	c := b.Stone
	if c.tilesArt != b.art() {
		c.tiles, c.tilesArt = nil, b.art()
	}
	return c
}

// tileKeyOf is the cached ground tile and colour scale of a terrain code
// and variant (TerrainTileKey without the per-tile string building).
func (b *base) tileKeyOf(code uint8, v int) (string, sprite.ColorScale) {
	c := b.stoneCache()
	if c.tiles == nil {
		c.tiles = make([][SoilVariants]tileKeyEntry, len(TerrainNames))
	}
	if int(code) >= len(c.tiles) {
		return b.TerrainTileKey("", v)
	}
	e := &c.tiles[code][v]
	if !e.done {
		e.key, e.cs = b.TerrainTileKey(TerrainNames[code], v)
		e.done = true
	}
	return e.key, e.cs
}
