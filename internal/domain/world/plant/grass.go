package plant

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/terrain"
)

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

// GrassGround reports whether terrain t carries grass: in the legacy
// (temperate) map only soil and rich soil, in other biomes any fertile
// ground (moss on tundra gravel, sedge on marsh, tufts on sand). Ice never.
func GrassGround(t terrain.Type, legacy bool) bool {
	if legacy {
		return t == terrain.Soil || t == terrain.RichSoil
	}
	return t.Fertility() > 0
}

// GrassAtBiome reports whether tile (x, y) of terrain t starts with grass in
// a biome that seeds permil of its grass ground (SIM-023); sand holds a
// quarter of that.
func GrassAtBiome(t terrain.Type, x, y, permil int) bool {
	if !GrassGround(t, false) {
		return false
	}
	if t == terrain.Sand {
		permil /= 4
	}
	h := tileHash(x, y) ^ 0x6772617373 // "grass"
	h = (h ^ h>>30) * 0xbf58476d1ce4e5b9
	h = (h ^ h>>27) * 0x94d049bb133111eb
	h ^= h >> 31
	return int(h%1000) < permil
}

// Grass growth (PLAY-010): a grazed or cut tile grows back as shoots and
// fills out over a few days of mild weather. Growth is percent, 0..100.
const (
	// GrassFull is a fully grown grass tile.
	GrassFull = 100
	// GrassShoots is the growth a tile regrows at.
	GrassShoots = 15
	// GrassGrazeMin: grazers pass over grass below this growth while any
	// fuller grass is in reach (hysteresis: shoots are left to fill out).
	GrassGrazeMin = 30
	// GrassMinMeal is the least share of a full meal a grazed tile gives,
	// percent.
	GrassMinMeal = 30
	// GrassFillTicks is how long shoots take to fill out at full growth
	// speed (mild weather).
	GrassFillTicks = 3 * clock.TicksPerDay
)

// GrassGrowSpan is the growth (percent points) grass gains over ticks
// t0+1..t1 at growth speed tempFactor permille (TempFactor), integrated
// as a running total so spans telescope and fractions are never lost.
func GrassGrowSpan(tempFactor int, t0, t1 uint64) int {
	if tempFactor <= 0 || t1 <= t0 {
		return 0
	}
	const den = 1000 * GrassFillTicks
	f := uint64(tempFactor) * (GrassFull - GrassShoots)
	return int(t1*f/den - t0*f/den)
}

// GrassMeal is the food of grazing a tile at growth percent, out of a
// full tile's meal full: in proportion, never under GrassMinMeal percent.
func GrassMeal(full, growth int) int {
	return full * max(growth, GrassMinMeal) / 100
}
