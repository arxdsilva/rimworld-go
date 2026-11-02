package planet

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
)

// Movement difficulty (WG-03, wiki: Caravan): hills add to the biome
// base, winter adds up to MoveWinter-MoveBase between winterStart and
// winterFull (the linear ramp is original tuning), a road halves it.
const (
	winterStart = -17.1 // °C daily mean: winter starts to slow travel
	winterFull  = -22.5 // °C: the full winter penalty
	roadMove    = 0.5
)

var hillMove = [...]float64{site.HillsDefault: 0, site.Flat: 0, site.SmallHills: 0.5, site.LargeHills: 1.5, site.Mountainous: 3} // wiki: Caravan

// Base search bounds (original tuning) and the landing warning radius
// (wiki: Faction base).
const (
	nearbyMaxHops = 60
	nearBaseHops  = 4
)

// MoveDifficulty is tile i's world-map movement difficulty on day of the
// year day (any integer; it wraps), or 0 for water and invalid tiles
// (impassable).
func (p *Planet) MoveDifficulty(i, day int) float64 {
	if !p.Land(i) {
		return 0
	}
	d := biome.DefOf(p.Biome[i])
	v := d.MoveBase + p.hillMoveOf(i)
	day = (day%clock.DaysPerYear + clock.DaysPerYear) % clock.DaysPerYear
	t := float64(p.climate(i).DailyMean(clock.Tick(day*clock.TicksPerDay))) / 10
	v += (d.MoveWinter - d.MoveBase) * clampF((winterStart-t)/(winterStart-winterFull), 0, 1)
	return p.roadMoveOf(i, v)
}

// moveAllYear is land tile i's difficulty without the winter term: what
// Route plans with.
func (p *Planet) moveAllYear(i int) float64 {
	return p.roadMoveOf(i, biome.DefOf(p.Biome[i]).MoveBase+p.hillMoveOf(i))
}

func (p *Planet) hillMoveOf(i int) float64 {
	if h := int(p.Hill[i]); h < len(hillMove) {
		return hillMove[h]
	}
	return 0
}

func (p *Planet) roadMoveOf(i int, v float64) float64 {
	if i < len(p.ext.roadAt) && p.ext.roadAt[i] {
		return v * roadMove
	}
	return v
}

// TimeZone is tile i's hour offset from the prime meridian, round(lon/15)
// (-12..12); 0 for an invalid tile.
func (p *Planet) TimeZone(i int) int {
	if i < 0 || i >= p.TileCount() {
		return 0
	}
	return int(math.Round(float64(p.Lon[i]) / 150))
}

// QuadrumTemps are tile i's mean temperatures (°C) over each 15-day
// quadrum of the year, from the tile's seasonal curve.
func (p *Planet) QuadrumTemps(i int) [4]float64 {
	var out [4]float64
	if i < 0 || i >= p.TileCount() {
		return out
	}
	cl := p.climate(i)
	for q := range out {
		sum := 0
		for d := q * clock.DaysPerQuadrum; d < (q+1)*clock.DaysPerQuadrum; d++ {
			sum += int(cl.DailyMean(clock.Tick(d * clock.TicksPerDay)))
		}
		out[q] = float64(sum) / float64(clock.DaysPerQuadrum) / 10
	}
	return out
}

// NearbyBases are up to n faction settlements nearest tile i (fewest
// hops, then lowest tile), searched up to nearbyMaxHops. It allocates its
// own scratch; NearbyBasesWith reuses a caller's.
func (p *Planet) NearbyBases(i, n int) []Base {
	var s HopScratch
	return p.NearbyBasesWith(&s, i, n)
}

// NearbyBasesWith is NearbyBases over caller-owned scratch s.
func (p *Planet) NearbyBasesWith(s *HopScratch, i, n int) []Base {
	return basesNear(p, s, i, n, nearbyMaxHops)
}

// NearBase is the nearest faction settlement within nearBaseHops of tile
// i, which the landing screen warns about (wiki: Faction base).
func (p *Planet) NearBase(i int) (Base, bool) {
	var s HopScratch
	if b := basesNear(p, &s, i, 1, nearBaseHops); len(b) > 0 {
		return b[0], true
	}
	return Base{}, false
}

// basesNear is a ring-by-ring breadth-first search from i that stops
// after the ring where it has found n settlements (or at limit hops).
func basesNear(p *Planet, s *HopScratch, i, n, limit int) []Base {
	if i < 0 || i >= p.TileCount() || n <= 0 {
		return nil
	}
	var out []Base
	add := func(t int32, d int) {
		if k := p.Settlement[t]; k >= 0 {
			ref := p.settlements[k]
			out = append(out, Base{Tile: int(t), Faction: ref.faction, Rank: ref.k, Hops: d})
		}
	}
	gen := s.start(p.TileCount())
	s.mark[i] = gen
	front := append(s.front[:0], int32(i))
	next := s.next[:0]
	add(int32(i), 0)
	for d := 1; d <= limit && len(out) < n && len(front) > 0; d++ {
		next = next[:0]
		for _, t := range front {
			for _, nb := range p.Neighbours(int(t)) {
				if s.mark[nb] != gen {
					s.mark[nb] = gen
					next = append(next, nb)
					add(nb, d)
				}
			}
		}
		front, next = next, front
	}
	s.front, s.next = front[:0], next[:0]
	slices.SortFunc(out, func(a, b Base) int {
		if a.Hops != b.Hops {
			return a.Hops - b.Hops
		}
		return a.Tile - b.Tile
	})
	return out[:min(len(out), n)]
}

// RandomLandingFiltered is RandomLanding among the tiles that pass f,
// with the same weights and stream (the zero Filter picks exactly what
// RandomLanding picks); -1 when no valid tile passes.
func (p *Planet) RandomLandingFiltered(f Filter, roll int) int {
	cum, tiles := p.ext.landCum, p.ext.landTile
	var total, prev uint32
	for k, t := range tiles {
		if f.passes(p, int(t)) {
			total += cum[k] - prev
		}
		prev = cum[k]
	}
	if total == 0 {
		return -1
	}
	x := uint32(landingRand(p.Seed, roll).IntN(int(total)))
	var acc uint32
	prev = 0
	for k, t := range tiles {
		w := cum[k] - prev
		prev = cum[k]
		if !f.passes(p, int(t)) {
			continue
		}
		if acc += w; acc > x {
			return int(t)
		}
	}
	return -1
}

// passes reports whether tile i meets every set field of f.
func (f Filter) passes(p *Planet, i int) bool {
	if f.Biomes != 0 && !f.Biomes.Has(p.Biome[i]) {
		return false
	}
	if f.TempMin != 0 || f.TempMax != 0 {
		if t := float64(p.Temp[i]) / 10; t < f.TempMin || t > f.TempMax {
			return false
		}
	}
	if f.Hills != 0 && f.Hills&(1<<p.Hill[i]) == 0 {
		return false
	}
	coast := false
	for _, nb := range p.Neighbours(i) {
		coast = coast || p.Water[nb] == Ocean
	}
	if !f.River.ok(p.River[i] != site.NoRiver) || !f.Coast.ok(coast) || !f.Road.ok(p.ext.roadAt[i]) || !f.Caves.ok(p.Caves[i]) {
		return false
	}
	return f.Stone == geology.None || slices.Contains(p.Stones[i][:], f.Stone)
}
