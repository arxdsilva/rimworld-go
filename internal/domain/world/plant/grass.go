package plant

import "rimworld-go/internal/domain/world/terrain"

// grassEvery is how sparse grass is: one soil tile in five.
const grassEvery = 5

// tileHash is a deterministic per-tile hash. It is copied verbatim from the
// ebiten renderer's tileHash (sprites.go); the two MUST match so the map
// looks the same as before grass became simulation state
// (TestGrassMatchesRendererRule pins this).
func tileHash(x, y int) uint64 {
	return uint64(uint32(x))*0x9E3779B97F4A7C15 ^ uint64(uint32(y))*0xBF58476D1CE4E5B9
}

// GrassAt reports whether tile (x, y) of terrain t starts with grass: only
// Soil and RichSoil grow it, on a fixed pseudo-random one-in-five of tiles.
func GrassAt(t terrain.Type, x, y int) bool {
	if t != terrain.Soil && t != terrain.RichSoil {
		return false
	}
	return (tileHash(x, y)>>8)%grassEvery == 0
}
