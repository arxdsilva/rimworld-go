package planet

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/pkg/randx"
)

// TileAt grid: 2° latitude × 2° longitude buckets.
const (
	bucketDeg  = 2
	bucketRows = 180 / bucketDeg
	bucketCols = 360 / bucketDeg
)

func bucketOf(latX10, lonX10 int) int {
	row := clampInt((latX10+900)/(bucketDeg*10), 0, bucketRows-1)
	col := clampInt((lonX10+1800)/(bucketDeg*10), 0, bucketCols-1)
	return row*bucketCols + col
}

// buildBuckets records one tile per bucket (the lowest index whose centre
// falls in it); empty buckets borrow the tile of the nearest filled bucket
// (grid breadth-first search, longitude wrapping), so TileAt always starts
// close by.
func (p *Planet) buildBuckets() {
	p.buckets = make([]int32, bucketRows*bucketCols)
	for i := range p.buckets {
		p.buckets[i] = -1
	}
	var queue []int32
	for i := 0; i < p.TileCount(); i++ {
		k := bucketOf(int(p.Lat[i]), int(p.Lon[i]))
		if p.buckets[k] < 0 {
			p.buckets[k] = int32(i)
			queue = append(queue, int32(k))
		}
	}
	for h := 0; h < len(queue); h++ {
		k := int(queue[h])
		row, col := k/bucketCols, k%bucketCols
		for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			r := row + d[0]
			if r < 0 || r >= bucketRows {
				continue
			}
			c := (col + d[1] + bucketCols) % bucketCols
			if nk := r*bucketCols + c; p.buckets[nk] < 0 {
				p.buckets[nk] = p.buckets[k]
				queue = append(queue, int32(nk))
			}
		}
	}
}

// TileAt is the tile under a direction from the planet's centre (any
// length), or -1 when it points outside the coverage. It starts from the
// direction's lat/lon bucket and walks to the neighbour closest to the
// direction until no neighbour is closer.
func (p *Planet) TileAt(dir [3]float32) int {
	if p.TileCount() == 0 {
		return -1
	}
	d := norm64(f64(dir))
	if d == ([3]float64{}) {
		return -1
	}
	// Directions well outside the cap (beyond one tile step) hit nothing.
	if p.CapCos > -1 && math.Acos(clampF(dot64(d, f64(p.Pole)), -1, 1)) >
		math.Acos(float64(p.CapCos))+2*math.Acos(clampF(p.stepCos, -1, 1)) {
		return -1
	}
	lat := int(math.Round(math.Asin(clampF(d[2], -1, 1)) * 1800 / math.Pi))
	lon := int(math.Round(math.Atan2(d[1], d[0]) * 1800 / math.Pi))
	cur := int(p.buckets[bucketOf(lat, lon)])
	best := dot64(f64(p.Center[cur]), d)
	for {
		next := -1
		for _, nb := range p.Neighbours(cur) {
			if v := dot64(f64(p.Center[nb]), d); v > best {
				best, next = v, int(nb)
			}
		}
		if next < 0 {
			break
		}
		cur = next
	}
	if best < float64(p.radCos[cur])-1e-6 {
		return -1
	}
	return cur
}

// Site is the local map's parameters for tile i (NG-1 §4.2). An invalid
// tile gives the zero (legacy) Site.
func (p *Planet) Site(i int) site.Site {
	if i < 0 || i >= p.TileCount() {
		return site.Site{}
	}
	s := site.Site{
		Biome:    p.Biome[i],
		Climate:  p.climate(i),
		Hills:    p.Hill[i],
		Caves:    p.Caves[i],
		Latitude: p.Lat[i],
	}
	if s.Hills == site.HillsDefault {
		s.Hills = site.Flat
	}
	for _, st := range p.Stones[i] {
		if st != geology.None {
			s.Stones = append(s.Stones, st)
		}
	}
	if len(p.Rain) > i {
		s.Rain = p.Rain[i]
	}
	if len(p.CoastDist) > i {
		s.CoastNear = p.CoastDist[i]
	}
	if len(p.Settlement) > i && p.Settlement[i] >= 0 {
		s.Settled = true
	}
	top := int16(0)
	if len(p.Elev) > i {
		top = p.Elev[i]
	}
	lakes := 0
	for _, nb := range p.Neighbours(i) {
		if len(p.Elev) > int(nb) && p.Elev[nb] > top {
			top, s.Uphill, s.HasUphill = p.Elev[nb], site.Dir8(p.bearing(i, int(nb))), true
		}
		if len(p.Settlement) > int(nb) && p.Settlement[nb] >= 0 {
			s.Settled = true
		}
		switch p.Water[nb] {
		case Ocean:
			s.Coast |= 1 << p.bearing(i, int(nb))
		case Lake:
			lakes++
		}
	}
	s.Lakes = uint8(min(lakes, 3))
	if p.River[i] != site.NoRiver {
		s.River.Size = p.River[i]
		out := p.RiverTo[i]
		if out >= 0 {
			s.River.Out = site.Dir8(p.bearing(i, int(out)))
		}
		up := -1
		for _, nb := range p.Neighbours(i) {
			if p.River[nb] != site.NoRiver && p.RiverTo[nb] == int32(i) && (up < 0 || p.River[nb] > p.River[up]) {
				up = int(nb)
			}
		}
		if up >= 0 {
			s.River.In = site.Dir8(p.bearing(i, up))
		} else {
			s.River.In = (s.River.Out + 4) % 8 // a source: enter opposite the exit
		}
	}
	for _, r := range p.Roads {
		switch int32(i) {
		case r[0]:
			s.Road |= 1 << p.bearing(i, int(r[1]))
		case r[1]:
			s.Road |= 1 << p.bearing(i, int(r[0]))
		}
	}
	return s
}

// Site climate numbers (tenths of a degree).
const (
	ampEquator     = 60
	ampPolar       = 220
	ampLatFull     = 600 // tenths of a degree of latitude
	ampInland      = 40
	ampInlandTiles = 6
	swingDry       = 120
	swingWet       = 50
	swingDryRain   = 250
	swingWetRain   = 2000
	peakNorth      = 15
	peakSouth      = 45
)

// climate is tile i's temperature curve: the yearly mean, an amplitude
// growing with latitude and distance from the sea, a day swing that is
// larger in dry places, and the hemisphere's peak day.
func (p *Planet) climate(i int) temperature.Climate {
	lat := absInt(int(p.Lat[i]))
	amp := ampEquator + (ampPolar-ampEquator)*min(lat, ampLatFull)/ampLatFull
	amp += ampInland * min(int(p.CoastDist[i]), ampInlandTiles) / ampInlandTiles
	r := clampInt(int(p.Rain[i]), swingDryRain, swingWetRain)
	swing := swingDry - (swingDry-swingWet)*(r-swingDryRain)/(swingWetRain-swingDryRain)
	peak := peakNorth
	if p.Lat[i] < 0 {
		peak = peakSouth
	}
	return temperature.Climate{YearMean: int(p.Temp[i]), Amplitude: amp, PeakDay: peak, DaySwing: swing}
}

// GrowingDays is the growing period at tile i (of 60): days with a mean
// above 3 °C.
func (p *Planet) GrowingDays(i int) int {
	if i < 0 || i >= p.TileCount() {
		return 0
	}
	return p.climate(i).GrowingDays(30)
}

// Hops is the hop distance between tiles (capped at limit). It is not
// safe for concurrent use (it reuses a scratch buffer).
func (p *Planet) Hops(a, b, limit int) int {
	n := p.TileCount()
	if a < 0 || b < 0 || a >= n || b >= n || limit <= 0 {
		return max(limit, 0)
	}
	if a == b {
		return 0
	}
	// Each hop turns at most acos(stepCos): farther pairs are out of reach.
	ang := math.Acos(clampF(dot64(f64(p.Center[a]), f64(p.Center[b])), -1, 1))
	if ang > float64(limit)*math.Acos(clampF(p.stepCos, -1, 1))+1e-9 {
		return limit
	}
	if len(p.hopMark) != n {
		p.hopMark, p.hopGen = make([]uint32, n), 0
	}
	p.hopGen++
	gen := p.hopGen
	p.hopMark[a] = gen
	front := []int32{int32(a)}
	for d := 1; d < limit; d++ {
		var next []int32
		for _, t := range front {
			for _, nb := range p.Neighbours(int(t)) {
				if p.hopMark[nb] == gen {
					continue
				}
				if int(nb) == b {
					return d
				}
				p.hopMark[nb] = gen
				next = append(next, nb)
			}
		}
		if len(next) == 0 {
			break
		}
		front = next
	}
	return limit
}

// Nearest is the closest tile (by hops, lowest index on ties) within
// limit hops of i that satisfies pred, with its distance; -1 if none.
func (p *Planet) Nearest(i int, limit int, pred func(int) bool) (int, int) {
	n := p.TileCount()
	if i < 0 || i >= n {
		return -1, limit
	}
	if pred(i) {
		return i, 0
	}
	seen := map[int32]bool{int32(i): true}
	front := []int32{int32(i)}
	for d := 1; d <= limit && len(front) > 0; d++ {
		var next []int32
		found := -1
		for _, t := range front {
			for _, nb := range p.Neighbours(int(t)) {
				if seen[nb] {
					continue
				}
				seen[nb] = true
				next = append(next, nb)
				if pred(int(nb)) && (found < 0 || int(nb) < found) {
					found = int(nb)
				}
			}
		}
		if found >= 0 {
			return found, d
		}
		front = next
	}
	return -1, limit
}

// Landing reasons (NG-1 §2.7).
const (
	ReasonWater   = "Can't land on water."
	ReasonOutside = "Not part of this world."
)

// ValidLanding reports whether a colony may land on tile i, and why not.
// Ice Sheet and Sea Ice are allowed (the UI warns about extreme climate).
func (p *Planet) ValidLanding(i int) (bool, string) {
	if i < 0 || i >= p.TileCount() {
		return false, ReasonOutside
	}
	if p.Water[i] != Land {
		return false, ReasonWater
	}
	if p.Settlement[i] >= 0 {
		return false, "Occupied by " + p.SettlementName(i) + "."
	}
	for _, nb := range p.Neighbours(i) {
		if p.Settlement[nb] >= 0 {
			return false, "Too close to " + p.SettlementName(int(nb)) + "."
		}
	}
	return true, ""
}

// SettlementFaction is the index into Factions of the settlement on tile
// i and its rank in that faction (0 = capital), or -1, -1.
func (p *Planet) SettlementFaction(i int) (int, int) {
	if i < 0 || i >= p.TileCount() || p.Settlement[i] < 0 {
		return -1, -1
	}
	s := p.settlements[p.Settlement[i]]
	return s.faction, s.k
}

// SettlementName names the settlement on tile i ("" if none): the
// faction's generated name for it, or one derived from the faction.
func (p *Planet) SettlementName(i int) string {
	f, k := p.SettlementFaction(i)
	if f < 0 {
		return ""
	}
	if names := p.Factions[f].SettlementNames; k < len(names) && names[k] != "" {
		return names[k]
	}
	name := p.Factions[f].Name
	if name == "" {
		name = fmt.Sprintf("faction %d", f+1)
	}
	if k == 0 {
		return "the capital of " + name
	}
	return fmt.Sprintf("%s settlement %d", name, k+1)
}

// SettlementTiles are the settled tiles, in global settlement order.
func (p *Planet) SettlementTiles() []int32 {
	out := make([]int32, len(p.settlements))
	for k, s := range p.settlements {
		out[k] = s.tile
	}
	return out
}

// Random landing weights: the preferred tiles (non-extreme biome, growing
// period ≥ 20 days, flat to large hills) are landingPreferred times as
// likely as any other valid tile.
const (
	landingPreferred = 12
	landingGrowDays  = 20
)

// finish prepares the read-only query tables once generation is done.
func finish(p *Planet) {
	var total uint32
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); !ok {
			continue
		}
		w := uint32(1)
		if biome.DefOf(p.Biome[i]).Difficulty != "extreme" && p.Hill[i] <= site.LargeHills &&
			p.GrowingDays(i) >= landingGrowDays {
			w = landingPreferred
		}
		total += w
		p.landCum = append(p.landCum, total)
		p.landTile = append(p.landTile, int32(i))
	}
}

// RandomLanding picks a valid tile deterministically from the planet seed
// and the press counter n, weighted toward pleasant sites; -1 when no tile
// is valid.
func (p *Planet) RandomLanding(n int) int {
	if len(p.landCum) == 0 {
		return -1
	}
	r := randx.New(mix(p.Seed ^ saltLanding ^ mix(uint64(n))))
	x := uint32(r.IntN(int(p.landCum[len(p.landCum)-1])))
	k := sort.Search(len(p.landCum), func(j int) bool { return p.landCum[j] > x })
	return int(p.landTile[k])
}

// Key hashes the params (a planet is reused when its key matches).
func (p Params) Key() string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%q|%d|%d|%d|%d|%d|", p.SeedText, p.Coverage, p.Rain, p.Temp, p.Pop, p.Version)
	for _, s := range p.Factions {
		fmt.Fprintf(h, "%d:%d,", s.Type, s.Count)
	}
	return fmt.Sprintf("%016x", h.Sum64())
}
