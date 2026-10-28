package env

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Zoom tiers by on-screen tile size, as the renderer's lod.go draws them
// (its lodLiveMin, lodChunkMin, decalMinTile and visPad must equal these:
// TestEnvTiersMatchLOD).
const (
	// LiveMin is the tile size (px) from which the world draws live.
	LiveMin = 48.0
	// ChunkMin is the tile size from which chunk bakes draw, the map bake
	// below.
	ChunkMin = 16.0
	// DecalMinTile is the tile size ground-cover decals (snow cover,
	// drifts, frost) need; below it the smooth veil alone shows.
	DecalMinTile = 32.0
	// VisPad is how far past the visible tiles the frame's chunk lists
	// reach.
	VisPad = 5
)

// tier is a zoom tier (lod.go's lodTier).
type tier uint8

const (
	tierLive tier = iota
	tierChunk
	tierMap
)

// tierFor is the tier for an on-screen tile size.
func tierFor(tile float64) tier {
	switch {
	case tile >= LiveMin:
		return tierLive
	case tile >= ChunkMin:
		return tierChunk
	}
	return tierMap
}

const (
	// SoilVariants is how many variants each terrain tile has.
	SoilVariants = 4
	// GrassEvery is the hashed grass rule's spacing without simulation
	// grass data.
	GrassEvery = 5
)

// terrainNames indexes the read model's terrain codes.
var terrainNames = game.TerrainNames()

// ArtKey is the sprite key family/name_NN.
func ArtKey(family, name string, variant int) string {
	return family + "/" + name + "_" + sprite.TwoDigits(variant)
}

// TerrainCode is the read model's terrain code at (x, y).
func TerrainCode(w game.WorldView, x, y int) (uint8, bool) {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0, false
	}
	i := y*w.TerrainW + x
	if i >= len(w.Terrain) {
		return 0, false
	}
	return w.Terrain[i], true
}

// TerrainName is the terrain's name at (x, y) ("" off the map).
func TerrainName(w game.WorldView, x, y int) string {
	c, ok := TerrainCode(w, x, y)
	if !ok || int(c) >= len(terrainNames) {
		return ""
	}
	return terrainNames[c]
}
