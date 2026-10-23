package planet

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/pkg/randx"
)

type siteClimate = temperature.Climate

var built = map[string]*Planet{}

// world builds (once per test binary) a planet without factions, so the
// result does not depend on settlement placement.
func world(t testing.TB, seed string, coverage int) *Planet {
	t.Helper()
	k := fmt.Sprintf("%s/%d", seed, coverage)
	if p, ok := built[k]; ok {
		return p
	}
	p := Build(Params{SeedText: seed, Coverage: coverage, Rain: 2, Temp: 2, Pop: 1})
	built[k] = p
	return p
}

// fakePlace settles n well-spaced tiles of the largest landmass (the
// lowest land tiles by index that are at least 5 hops apart), standing in
// for faction.PlaceSettlements.
func fakePlace(n int) func(faction.Globe, []faction.Faction, int, *randx.Rand) {
	return func(g faction.Globe, fs []faction.Faction, _ int, _ *randx.Rand) {
		var picked []int
		for i := 0; i < g.TileCount() && len(picked) < n; i++ {
			if !g.Land(i) || g.Biome(i) == biome.SeaIce || g.Biome(i) == biome.IceSheet {
				continue
			}
			ok := true
			for _, q := range picked {
				if g.Hops(i, q, 5) < 5 {
					ok = false
					break
				}
			}
			if ok {
				picked = append(picked, i)
			}
		}
		for k, t := range picked {
			f := &fs[k%len(fs)]
			f.Settlements = append(f.Settlements, t)
		}
	}
}

func fakeFactions(slots []faction.Slot, _ *randx.Rand) []faction.Faction {
	return []faction.Faction{{ID: 1, Name: "the Ashen Test"}, {ID: 2, Name: "Test Clan", SettlementNames: []string{"Thornwick"}}}
}

func buildWith(p Params, step int, n int) *Planet {
	b := NewBuilder(p)
	b.genFactions, b.place = fakeFactions, fakePlace(n)
	for {
		if _, _, done := b.Step(step); done {
			return b.Planet()
		}
	}
}

func hashPlanet(p *Planet) uint64 {
	h := fnv.New64a()
	w := func(v any) { _ = binary.Write(h, binary.LittleEndian, v) }
	w(p.Seed)
	w(p.Lat)
	w(p.Lon)
	w(p.Elev)
	w(p.Temp)
	w(p.Rain)
	w(p.Water)
	w(p.RiverTo)
	w(p.Settlement)
	w(p.CoastDist)
	w(p.NeighOff)
	w(p.Neigh)
	for i := range p.Center {
		w([]uint8{uint8(p.Hill[i]), uint8(p.Biome[i]), uint8(p.River[i]), uint8(p.Stones[i][0]), uint8(p.Stones[i][1]), uint8(p.Stones[i][2])})
		w(p.Caves[i])
	}
	for _, r := range p.Roads {
		w(r)
	}
	return h.Sum64()
}

func TestTileCountFormula(t *testing.T) {
	for _, f := range []int{1, 2, 3, 4, 7, Frequency} {
		m := buildMesh(f)
		if got, want := len(m.pos), 10*f*f+2; got != want {
			t.Errorf("f=%d: %d tiles, want %d", f, got, want)
		}
	}
	if got := world(t, "count", 100).TileCount(); got != 23042 {
		t.Errorf("100%% planet has %d tiles, want 23042", got)
	}
}

func TestPentagonsAre12(t *testing.T) {
	for _, f := range []int{1, 2, 5, Frequency} {
		m := buildMesh(f)
		pent := 0
		for v := range m.deg {
			switch m.deg[v] {
			case 5:
				pent++
			case 6:
			default:
				t.Fatalf("f=%d: vertex %d has %d neighbours", f, v, m.deg[v])
			}
		}
		if pent != 12 {
			t.Errorf("f=%d: %d pentagons, want 12", f, pent)
		}
	}
	p := world(t, "count", 100)
	pent := 0
	for i := 0; i < p.TileCount(); i++ {
		nc, cc := len(p.Neighbours(i)), len(p.TileCorners(i))
		if nc != cc {
			t.Fatalf("tile %d: %d neighbours but %d corners", i, nc, cc)
		}
		if nc == 5 {
			pent++
		}
	}
	if pent != 12 {
		t.Errorf("planet has %d pentagons, want 12", pent)
	}
	defer func() {
		if recover() == nil {
			t.Error("buildMesh(0) should panic")
		}
	}()
	buildMesh(0)
}

func TestNeighboursSymmetric(t *testing.T) {
	for _, cov := range []int{30, 100} {
		p := world(t, "symmetric", cov)
		for i := 0; i < p.TileCount(); i++ {
			nbs := p.Neighbours(i)
			c := f64(p.Center[i])
			for k, nb := range nbs {
				if int(nb) == i {
					t.Fatalf("tile %d lists itself", i)
				}
				back := false
				for _, x := range p.Neighbours(int(nb)) {
					back = back || int(x) == i
				}
				if !back {
					t.Fatalf("cov %d: %d -> %d is one-way", cov, i, nb)
				}
				if len(nbs) == len(p.TileCorners(i)) { // interior: counter-clockwise order
					a := sub64(f64(p.Center[nb]), c)
					b := sub64(f64(p.Center[nbs[(k+1)%len(nbs)]]), c)
					if dot64(cross64(a, b), c) <= 0 {
						t.Fatalf("tile %d neighbours not counter-clockwise at %d", i, k)
					}
				}
			}
			// Corner k lies between neighbour k and k+1.
			if len(nbs) == len(p.TileCorners(i)) {
				for k, cn := range p.TileCorners(i) {
					a, b := f64(p.Center[nbs[k]]), f64(p.Center[nbs[(k+1)%len(nbs)]])
					want := norm64([3]float64{c[0] + a[0] + b[0], c[1] + a[1] + b[1], c[2] + a[2] + b[2]})
					if dot64(want, f64(cn)) < 0.999999 {
						t.Fatalf("tile %d corner %d is not between neighbours %d and %d", i, k, k, k+1)
					}
				}
			}
		}
	}
}

func TestCoverageFraction(t *testing.T) {
	full := 23042.0
	for _, cov := range []int{5, 30, 50, 100} {
		p := world(t, "coverage", cov)
		got := float64(p.TileCount()) / full
		if math.Abs(got-float64(cov)/100) > 0.01 {
			t.Errorf("coverage %d%%: %.3f of the globe", cov, got)
		}
		pole := f64(p.Pole)
		if lat := math.Asin(pole[2]) * 180 / math.Pi; math.Abs(lat) > poleLatMax+0.01 {
			t.Errorf("coverage pole at latitude %.1f", lat)
		}
		for i := 0; i < p.TileCount(); i++ {
			if dot64(f64(p.Center[i]), pole) < float64(p.CapCos)-1e-6 {
				t.Fatalf("coverage %d: tile %d outside the cap", cov, i)
			}
		}
	}
	if CoverageCos(0) != 1-2*0.01 || CoverageCos(250) != -2 {
		t.Error("CoverageCos clamps to 1..100")
	}
}

func TestLandShareInRange(t *testing.T) {
	lo, hi := 1.0, 0.0
	for s := 0; s < 100; s++ {
		p := Build(Params{SeedText: fmt.Sprintf("land-%d", s), Coverage: 30, Rain: s % 5, Temp: (s / 5) % 5, Pop: 1})
		land := 0
		for i := 0; i < p.TileCount(); i++ {
			if p.Elev[i] >= 0 {
				land++
			}
		}
		share := float64(land) / float64(p.TileCount())
		lo, hi = math.Min(lo, share), math.Max(hi, share)
		if share < 0.315 || share > 0.445 {
			t.Errorf("seed %d: land share %.3f outside 38%% ± 6%%", s, share)
		}
	}
	t.Logf("land share over 100 seeds: %.3f .. %.3f", lo, hi)
}

func TestBiomeClassificationTable(t *testing.T) {
	cases := []struct {
		t    float64
		r    int
		h    site.Hills
		e    int
		want biome.ID
	}{
		{-20, 500, site.Flat, 100, biome.IceSheet},
		{-10, 2000, site.Flat, 100, biome.Tundra},
		{0, 1300, site.SmallHills, 100, biome.ColdBog},
		{0, 1300, site.LargeHills, 100, biome.BorealForest},
		{0, 200, site.Flat, 100, biome.BorealForest},
		{10, 1600, site.Flat, 300, biome.TemperateSwamp},
		{10, 1600, site.Flat, 500, biome.TemperateForest},
		{10, 1600, site.LargeHills, 100, biome.TemperateForest},
		{10, 600, site.Mountainous, 2000, biome.TemperateForest},
		{25, 2600, site.Flat, 200, biome.TropicalSwamp},
		{25, 2600, site.Flat, 400, biome.TropicalRainforest},
		{25, 2000, site.Mountainous, 100, biome.TropicalRainforest},
		{19, 2000, site.Flat, 100, biome.TropicalRainforest}, // 18–20 °C closes the table's gap
		{25, 1800, site.Flat, 100, biome.AridShrubland},
		{10, 400, site.Flat, 100, biome.AridShrubland},
		{20, 150, site.Flat, 100, biome.Desert},
		{10, 150, site.Flat, 100, biome.Tundra},
		{30, 50, site.Flat, 100, biome.ExtremeDesert},
		{20, 50, site.Flat, 100, biome.Desert},
	}
	for _, c := range cases {
		if got := Classify(c.t, c.r, c.h, c.e); got != c.want {
			t.Errorf("Classify(%v °C, %d mm, %v, %d m) = %v, want %v", c.t, c.r, c.h, c.e, got, c.want)
		}
	}
	hills := []struct {
		g, e int
		want site.Hills
	}{{0, 0, site.Flat}, {149, 0, site.Flat}, {150, 0, site.SmallHills}, {399, 0, site.SmallHills},
		{400, 0, site.LargeHills}, {899, 0, site.LargeHills}, {900, 0, site.Mountainous}, {10, 3100, site.LargeHills}, {1000, 3100, site.Mountainous}}
	for _, c := range hills {
		if got := HillsFor(c.g, c.e); got != c.want {
			t.Errorf("HillsFor(%d, %d) = %v, want %v", c.g, c.e, got, c.want)
		}
	}
	// Generated land agrees with the table, cold ocean is sea ice.
	p := world(t, "amber-hollow", 100)
	seaIce := 0
	for i := 0; i < p.TileCount(); i++ {
		if p.Biome[i] == biome.SeaIce {
			seaIce++
			if p.Elev[i] >= 0 || p.Temp[i] >= seaIceTempX10 || p.Water[i] != Land {
				t.Fatalf("tile %d: sea ice with elev %d temp %d water %d", i, p.Elev[i], p.Temp[i], p.Water[i])
			}
			continue
		}
		if p.Water[i] != Land {
			continue
		}
		if want := Classify(float64(p.Temp[i])/10, int(p.Rain[i]), p.Hill[i], int(p.Elev[i])); p.Biome[i] != want {
			// a filled river pit may have been raised after classification
			if p.River[i] == site.NoRiver {
				t.Fatalf("tile %d is %v, table says %v", i, p.Biome[i], want)
			}
		}
	}
	if seaIce == 0 {
		t.Error("no sea ice on a full globe")
	}
}

func TestPickStones(t *testing.T) {
	s := pickStones([5]float64{0.1, 0.9, 0.5, 0.85, 0.2})
	if s != [3]geology.Stone{geology.Granite, geology.Slate, geology.None} {
		t.Errorf("two clear winners: %v", s)
	}
	s = pickStones([5]float64{0.7, 0.9, 0.5, 0.75, 0.2})
	if s != [3]geology.Stone{geology.Granite, geology.Slate, geology.Sandstone} {
		t.Errorf("close third: %v", s)
	}
	p := world(t, "amber-hollow", 30)
	for i := 0; i < p.TileCount(); i++ {
		st := p.Stones[i]
		if st[0] == geology.None || st[1] == geology.None || st[0] == st[1] || st[2] == st[0] || st[2] == st[1] {
			t.Fatalf("tile %d stones %v", i, st)
		}
	}
}

func TestRiversFlowDownhillToWater(t *testing.T) {
	for _, seed := range []string{"amber-hollow", "river-1", "river-2"} {
		p := world(t, seed, 100)
		rivers, sizes := 0, map[site.RiverSize]int{}
		for i := 0; i < p.TileCount(); i++ {
			if p.River[i] == site.NoRiver {
				if p.RiverTo[i] >= 0 {
					t.Fatalf("tile %d has RiverTo without a river", i)
				}
				continue
			}
			rivers++
			sizes[p.River[i]]++
			if p.Water[i] != Land {
				t.Fatalf("river on water tile %d", i)
			}
			cur := i
			for steps := 0; ; steps++ {
				if steps > p.TileCount() {
					t.Fatalf("river from %d loops", i)
				}
				if p.riverEnds(cur) {
					break
				}
				next := int(p.RiverTo[cur])
				if next < 0 {
					t.Fatalf("river at %d stops on dry land (from %d)", cur, i)
				}
				adj := false
				for _, nb := range p.Neighbours(cur) {
					adj = adj || int(nb) == next
				}
				if !adj {
					t.Fatalf("river %d -> %d is not a neighbour step", cur, next)
				}
				if p.Elev[next] > p.Elev[cur] {
					t.Fatalf("river climbs %d (%d m) -> %d (%d m)", cur, p.Elev[cur], next, p.Elev[next])
				}
				if p.River[next] != site.NoRiver && p.River[next] < p.River[cur] {
					t.Fatalf("river shrinks downstream %d -> %d", cur, next)
				}
				cur = next
			}
		}
		if rivers == 0 || sizes[site.Creek] == 0 || sizes[site.River] == 0 {
			t.Errorf("%s: %d river tiles, sizes %v", seed, rivers, sizes)
		}
	}
}

func TestRoadsConnectSettlements(t *testing.T) {
	p := buildWith(Params{SeedText: "roads", Coverage: 50}, 1<<20, 8)
	settled := p.SettlementTiles()
	if len(settled) < 2 {
		t.Fatalf("fake placement settled %d tiles", len(settled))
	}
	adj := map[int32][]int32{}
	for _, r := range p.Roads {
		a, b := r[0], r[1]
		if !p.passable(int(a)) || !p.passable(int(b)) {
			t.Fatalf("road %v crosses water or sea ice", r)
		}
		if p.Hops(int(a), int(b), 2) != 1 {
			t.Fatalf("road %v joins non-neighbours", r)
		}
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	reach := func(from int32) map[int32]bool {
		seen := map[int32]bool{from: true}
		q := []int32{from}
		for h := 0; h < len(q); h++ {
			for _, n := range adj[q[h]] {
				if !seen[n] {
					seen[n] = true
					q = append(q, n)
				}
			}
		}
		return seen
	}
	connectedPairs := 0
	for x, a := range settled {
		r := reach(a)
		for _, b := range settled[x+1:] {
			if p.sameLandmass(int(a), int(b)) {
				connectedPairs++
				if !r[b] {
					t.Errorf("settlements %d and %d share a landmass but no road", a, b)
				}
			}
		}
	}
	if connectedPairs == 0 {
		t.Error("no two fake settlements share a landmass")
	}
}

// sameLandmass is a flood fill over passable tiles (test helper).
func (p *Planet) sameLandmass(a, b int) bool {
	seen := map[int]bool{a: true}
	q := []int{a}
	for h := 0; h < len(q); h++ {
		if q[h] == b {
			return true
		}
		for _, n := range p.Neighbours(q[h]) {
			if !seen[int(n)] && p.passable(int(n)) {
				seen[int(n)] = true
				q = append(q, int(n))
			}
		}
	}
	return false
}

func TestRealFactionsPlaced(t *testing.T) {
	p := Build(Params{SeedText: "amber-hollow", Coverage: 30, Pop: 1, Factions: faction.DefaultSlots()})
	if len(p.Factions) == 0 {
		t.Skip("faction.Generate returned nothing (RULES not landed)")
	}
	for k, tile := range p.SettlementTiles() {
		if !p.Land(int(tile)) || p.Settlement[tile] != int32(k) {
			t.Fatalf("settlement %d on tile %d is not recorded", k, tile)
		}
		if f, _ := p.SettlementFaction(int(tile)); f < 0 || p.SettlementName(int(tile)) == "" {
			t.Fatalf("settlement %d has no faction or name", k)
		}
	}
}

// goldenHash pins "amber-hollow" at 30% (no factions, so it does not move
// with settlement placement). Re-pin only for an intended change and
// record why in the handoff.
const goldenHash = 0x60db28b1a028806d

func TestPlanetGoldenHash(t *testing.T) {
	p := world(t, "amber-hollow", 30)
	if got := hashPlanet(p); got != goldenHash {
		t.Errorf("planet hash = %#x, want %#x", got, uint64(goldenHash))
	}
}

func TestStepBudgetDeterministic(t *testing.T) {
	params := Params{SeedText: "steps", Coverage: 30, Rain: 3, Temp: 1, Pop: 2, Version: 3}
	big := buildWith(params, 10000, 6)
	small := buildWith(params, 1, 6)
	odd := buildWith(params, 777, 6)
	if hashPlanet(big) != hashPlanet(small) || hashPlanet(big) != hashPlanet(odd) {
		t.Fatal("step size changed the planet")
	}
	if len(big.Roads) == 0 {
		t.Error("no roads to compare")
	}
	// Progress is monotonic, phases advance in order and finish once.
	b := NewBuilder(params)
	b.genFactions, b.place = fakeFactions, fakePlace(4)
	lastF, lastP := float32(-1), PhaseShape
	for {
		ph, f, done := b.Step(500)
		if f < lastF || ph < lastP || f > 1 {
			t.Fatalf("progress went %v/%v -> %v/%v", lastP, lastF, ph, f)
		}
		if b.Planet() != nil && !done {
			t.Fatal("planet visible before done")
		}
		lastF, lastP = f, ph
		if done {
			break
		}
	}
	if ph, f, done := b.Step(0); ph != PhaseDone || f != 1 || !done || b.Planet() == nil {
		t.Fatal("finished builder must stay done")
	}
	if Build(Params{SeedText: "steps"}).Seed == Build(Params{SeedText: "steps", Version: 1}).Seed {
		t.Error("version must change the seed")
	}
}

func TestSiteFromTile(t *testing.T) {
	p := world(t, "amber-hollow", 100)
	if !p.Site(-1).Zero() || !p.Site(p.TileCount()).Zero() {
		t.Fatal("invalid tile must give the zero site")
	}
	var river, coast, north, south, lake bool
	for i := 0; i < p.TileCount(); i++ {
		if p.Water[i] != Land {
			continue
		}
		s := p.Site(i)
		if s.Zero() || s.Biome != p.Biome[i] || s.Hills != p.Hill[i] || s.Caves != p.Caves[i] || s.Latitude != p.Lat[i] {
			t.Fatalf("tile %d: site %+v does not mirror the tile", i, s)
		}
		c := s.Climate
		if c.YearMean != int(p.Temp[i]) || c.DaySwing < swingWet || c.DaySwing > swingDry ||
			c.Amplitude < ampEquator || c.Amplitude > ampPolar+ampInland {
			t.Fatalf("tile %d climate %+v", i, c)
		}
		if len(s.Stones) < 2 || len(s.Stones) > 3 {
			t.Fatalf("tile %d stones %v", i, s.Stones)
		}
		if p.Lat[i] > 0 {
			north = north || c.PeakDay == peakNorth
			if c.PeakDay != peakNorth {
				t.Fatalf("northern tile %d peak day %d", i, c.PeakDay)
			}
		} else if p.Lat[i] < 0 {
			south = south || c.PeakDay == peakSouth
			if c.PeakDay != peakSouth || c.SeasonAt(clock.Tick(15*clock.TicksPerDay)) == "Summer" {
				t.Fatalf("southern tile %d peak day %d", i, c.PeakDay)
			}
		}
		if p.River[i] != site.NoRiver {
			river = true
			if s.River.Size != p.River[i] || s.River.Out != site.Dir8(p.bearing(i, int(p.RiverTo[i]))) {
				t.Fatalf("tile %d river %+v", i, s.River)
			}
		}
		for _, nb := range p.Neighbours(i) {
			if p.Water[nb] == Ocean {
				coast = true
				if !s.Coast.Has(site.Dir8(p.bearing(i, int(nb)))) {
					t.Fatalf("tile %d misses coast toward %d", i, nb)
				}
			}
			if p.Water[nb] == Lake {
				lake = true
				if s.Lakes == 0 {
					t.Fatalf("tile %d beside a lake has Lakes 0", i)
				}
			}
		}
	}
	if !river || !coast || !north || !south || !lake {
		t.Errorf("coverage of cases: river %v coast %v north %v south %v lake %v", river, coast, north, south, lake)
	}
	// Amplitude grows with latitude; inland adds continentality.
	if a, b := p.climateAt(0, 0, 1000), p.climateAt(600, 0, 1000); b.Amplitude-a.Amplitude != ampPolar-ampEquator {
		t.Errorf("amplitude equator %d pole %d", a.Amplitude, b.Amplitude)
	}
	if a, b := p.climateAt(0, 0, 1000), p.climateAt(0, 10, 1000); b.Amplitude-a.Amplitude != ampInland {
		t.Errorf("inland amplitude %d vs coast %d", b.Amplitude, a.Amplitude)
	}
	if d, w := p.climateAt(0, 0, 100), p.climateAt(0, 0, 3000); d.DaySwing != swingDry || w.DaySwing != swingWet {
		t.Errorf("day swing dry %d wet %d", d.DaySwing, w.DaySwing)
	}
}

// climateAt overrides tile 0's latitude, coast distance and rain to probe
// the climate formula, restoring them afterwards.
func (p *Planet) climateAt(lat int16, coast uint8, rain int16) siteClimate {
	l, c, r := p.Lat[0], p.CoastDist[0], p.Rain[0]
	p.Lat[0], p.CoastDist[0], p.Rain[0] = lat, coast, rain
	out := p.climate(0)
	p.Lat[0], p.CoastDist[0], p.Rain[0] = l, c, r
	return out
}

func TestBearingsAndRoadsInSite(t *testing.T) {
	p := buildWith(Params{SeedText: "roads", Coverage: 50}, 1<<20, 8)
	for i := 0; i < p.TileCount(); i++ {
		for _, nb := range p.Neighbours(i) {
			dLat := float64(p.Lat[nb] - p.Lat[i])
			b := p.bearing(i, int(nb))
			if math.Abs(float64(p.Lat[i])) < 600 && dLat > 8 && b != 0 && b != 1 && b != 7 {
				t.Fatalf("tile %d: northward neighbour %d has bearing %d", i, nb, b)
			}
			if math.Abs(float64(p.Lat[i])) < 600 && dLat < -8 && b != 3 && b != 4 && b != 5 {
				t.Fatalf("tile %d: southward neighbour %d has bearing %d", i, nb, b)
			}
		}
	}
	for _, r := range p.Roads {
		s := p.Site(int(r[0]))
		if !s.Road.Has(site.Dir8(p.bearing(int(r[0]), int(r[1])))) {
			t.Fatalf("road %v missing from the site", r)
		}
	}
	// A pole tile still has a usable frame.
	e, n := (&Planet{Center: [][3]float32{{0, 0, 1}}}).eastNorth(0)
	if dot64(e, n) != 0 || e != ([3]float64{0, 1, 0}) {
		t.Errorf("pole frame %v %v", e, n)
	}
}

func TestValidLandingReasons(t *testing.T) {
	p := buildWith(Params{SeedText: "landing", Coverage: 30}, 1<<20, 4)
	if ok, why := p.ValidLanding(-1); ok || why != ReasonOutside {
		t.Errorf("tile -1: %v %q", ok, why)
	}
	if ok, why := p.ValidLanding(p.TileCount()); ok || why != ReasonOutside {
		t.Errorf("tile past the end: %v %q", ok, why)
	}
	// Opposite the cap's pole there are no tiles.
	pole := p.Pole
	if tile := p.TileAt([3]float32{-pole[0], -pole[1], -pole[2]}); tile != -1 {
		t.Errorf("TileAt opposite the cap = %d", tile)
	}
	var water, occupied, near, fine, ice = -1, -1, -1, -1, -1
	for i := 0; i < p.TileCount(); i++ {
		switch {
		case p.Water[i] != Land:
			water = i
		case p.Settlement[i] >= 0:
			occupied = i
		case p.Biome[i] == biome.SeaIce || p.Biome[i] == biome.IceSheet:
			ice = i
		default:
			close := false
			for _, nb := range p.Neighbours(i) {
				close = close || p.Settlement[nb] >= 0
			}
			if close {
				near = i
			} else {
				fine = i
			}
		}
	}
	if water < 0 || occupied < 0 || near < 0 || fine < 0 {
		t.Fatalf("missing cases: water %d occupied %d near %d fine %d", water, occupied, near, fine)
	}
	if ok, why := p.ValidLanding(water); ok || why != ReasonWater {
		t.Errorf("water: %v %q", ok, why)
	}
	if ok, why := p.ValidLanding(occupied); ok || why != "Occupied by "+p.SettlementName(occupied)+"." {
		t.Errorf("settlement: %v %q", ok, why)
	}
	if ok, why := p.ValidLanding(near); ok || !strings.HasPrefix(why, "Too close to ") || !strings.HasSuffix(why, ".") {
		t.Errorf("adjacent: %v %q", ok, why)
	}
	if ok, why := p.ValidLanding(fine); !ok || why != "" {
		t.Errorf("open land: %v %q", ok, why)
	}
	if ice >= 0 {
		if ok, _ := p.ValidLanding(ice); !ok {
			t.Errorf("ice tile %d should be allowed (warned only)", ice)
		}
	}
	// Names: generated name when the faction has one, derived otherwise.
	names := map[string]bool{}
	for _, tile := range p.SettlementTiles() {
		names[p.SettlementName(int(tile))] = true
	}
	if !names["Thornwick"] || !names["the capital of the Ashen Test"] || p.SettlementName(fine) != "" {
		t.Errorf("settlement names %v", names)
	}
	if f, k := p.SettlementFaction(fine); f != -1 || k != -1 {
		t.Error("open land has no settlement faction")
	}
}

func TestTileAtRoundTrip(t *testing.T) {
	for _, cov := range []int{30, 100} {
		p := world(t, "tileat", cov)
		for i := 0; i < p.TileCount(); i++ {
			c := p.Center[i]
			if got := p.TileAt(c); got != i {
				t.Fatalf("cov %d: TileAt(centre of %d) = %d", cov, i, got)
			}
			// Scaled input and a point most of the way to each corner.
			if got := p.TileAt([3]float32{c[0] * 3, c[1] * 3, c[2] * 3}); got != i {
				t.Fatalf("TileAt(3×centre of %d) = %d", i, got)
			}
			for _, cn := range p.TileCorners(i) {
				q := [3]float32{c[0]*0.3 + cn[0]*0.7, c[1]*0.3 + cn[1]*0.7, c[2]*0.3 + cn[2]*0.7}
				if got := p.TileAt(q); got != i {
					t.Fatalf("cov %d: TileAt(near corner of %d) = %d", cov, i, got)
				}
			}
		}
		if p.TileAt([3]float32{}) != -1 {
			t.Error("zero direction must miss")
		}
	}
	if (&Planet{}).TileAt([3]float32{1, 0, 0}) != -1 {
		t.Error("empty planet must miss")
	}
}

func TestHopsAndNearest(t *testing.T) {
	p := world(t, "hops", 30)
	if p.Hops(5, 5, 10) != 0 || p.Hops(-1, 5, 10) != 10 || p.Hops(1, 2, 0) != 0 {
		t.Fatal("Hops edge cases")
	}
	for i := 0; i < p.TileCount(); i += 97 {
		for _, nb := range p.Neighbours(i) {
			if p.Hops(i, int(nb), 64) != 1 {
				t.Fatalf("neighbours %d,%d not 1 hop", i, nb)
			}
		}
		j := (i * 7919) % p.TileCount()
		d := p.Hops(i, j, 64)
		if d != p.Hops(j, i, 64) {
			t.Fatalf("Hops not symmetric for %d,%d", i, j)
		}
		if d < 64 && p.Hops(i, j, d) != d {
			t.Fatalf("Hops(%d,%d) with limit at the distance", i, j)
		}
		if d > 1 && d < 64 && p.Hops(i, j, d-1) != d-1 {
			t.Fatalf("Hops below the distance must cap")
		}
		// Nearest water from a tile: distance agrees with Hops.
		if w, wd := p.Nearest(i, 64, func(t int) bool { return p.Water[t] != Land }); w >= 0 && p.Hops(i, w, 64) != wd {
			t.Fatalf("Nearest water %d at %d hops, Hops says %d", w, wd, p.Hops(i, w, 64))
		}
	}
	if tile, d := p.Nearest(3, 5, func(int) bool { return false }); tile != -1 || d != 5 {
		t.Error("Nearest without a match")
	}
	if tile, _ := p.Nearest(-4, 5, func(int) bool { return true }); tile != -1 {
		t.Error("Nearest from an invalid tile")
	}
	if tile, d := p.Nearest(3, 5, func(t int) bool { return t == 3 }); tile != 3 || d != 0 {
		t.Error("Nearest matching the start")
	}
}

func TestRandomLanding(t *testing.T) {
	p := buildWith(Params{SeedText: "random", Coverage: 30}, 1<<20, 6)
	nice, seen := 0, map[int]bool{}
	for n := 0; n < 200; n++ {
		tile := p.RandomLanding(n)
		if tile != p.RandomLanding(n) {
			t.Fatalf("press %d not deterministic", n)
		}
		if ok, why := p.ValidLanding(tile); !ok {
			t.Fatalf("press %d picked invalid tile %d: %s", n, tile, why)
		}
		seen[tile] = true
		if biome.DefOf(p.Biome[tile]).Difficulty != "extreme" && p.Hill[tile] <= site.LargeHills && p.GrowingDays(tile) >= landingGrowDays {
			nice++
		}
	}
	if nice < 150 || len(seen) < 100 {
		t.Errorf("200 presses: %d preferred, %d distinct", nice, len(seen))
	}
	if (&Planet{}).RandomLanding(0) != -1 {
		t.Error("empty planet has no landing")
	}
}

func TestGrowingDays(t *testing.T) {
	p := world(t, "amber-hollow", 100)
	hot, cold := false, false
	for i := 0; i < p.TileCount(); i++ {
		g := p.GrowingDays(i)
		if g < 0 || g > 60 {
			t.Fatalf("tile %d grows %d days", i, g)
		}
		if p.Biome[i] == biome.TropicalRainforest && p.Water[i] == Land {
			hot = hot || g == 60
		}
		if p.Biome[i] == biome.IceSheet {
			cold = cold || g == 0
		}
	}
	if !hot || !cold {
		t.Errorf("rainforest year-round %v, ice sheet barren %v", hot, cold)
	}
	if p.GrowingDays(-1) != 0 {
		t.Error("invalid tile grows nothing")
	}
}

func TestParamsKey(t *testing.T) {
	base := Params{SeedText: "a", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()}
	same := base
	same.Factions = faction.DefaultSlots()
	if same.Key() != base.Key() || len(base.Key()) != 16 {
		t.Fatal("key must be a stable 16-hex string")
	}
	variants := []Params{base, base, base, base, base, base, base, base}
	variants[0].SeedText = "b"
	variants[1].Coverage = 50
	variants[2].Rain = 3
	variants[3].Temp = 1
	variants[4].Pop = 2
	variants[5].Version = 1
	variants[6].Factions = []faction.Slot{{Type: faction.Clans, Count: 1}}
	variants[7].Factions = nil
	for k, v := range variants {
		if v.Key() == base.Key() {
			t.Errorf("variant %d shares the base key", k)
		}
	}
}

func TestGlobeView(t *testing.T) {
	p := world(t, "amber-hollow", 30)
	g := p.Globe()
	for i := 0; i < p.TileCount(); i += 101 {
		if g.Land(i) != (p.Water[i] == Land) || g.Biome(i) != p.Biome[i] || g.Hills(i) != uint8(p.Hill[i]) {
			t.Fatalf("globe view disagrees at %d", i)
		}
	}
	if g.TileCount() != p.TileCount() || g.Hops(0, 0, 3) != 0 || p.Land(-1) {
		t.Error("globe view basics")
	}
}

func BenchmarkPlanetBuild100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Build(Params{SeedText: fmt.Sprintf("bench-%d", i), Coverage: 100, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()})
	}
}

func BenchmarkPlanetBuild30(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Build(Params{SeedText: fmt.Sprintf("bench-%d", i), Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()})
	}
}

func BenchmarkTileAt(b *testing.B) {
	p := Build(Params{SeedText: "bench", Coverage: 100})
	for i := 0; i < b.N; i++ {
		c := p.Center[(i*7919)%p.TileCount()]
		p.TileAt(c)
	}
}
