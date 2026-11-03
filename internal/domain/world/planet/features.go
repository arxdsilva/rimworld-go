package planet

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// World feature sizes in tiles (WG-09, original tuning).
const (
	bayMinLand     = 3  // ocean tile with this many land neighbours is a cove
	bayMinTiles    = 4  // a bay is a connected group of coves this big
	bayMaxTiles    = 80 //
	oceanMinTiles  = 600
	oceanChunk     = 2500 // a big ocean splits into parts of about this size
	oceanMaxParts  = 6
	seaMinTiles    = 40
	mountainMin    = 12
	desertMin      = 25
	forestMin      = 40
	iceCapMin      = 25
	findMaxResults = 50
)

// featurePasses run as the stage's items, one region kind each.
const featurePasses = 6

// featState is the feature stage's scratch.
type featState struct {
	mark  []uint32 // flood-fill visit stamps
	gen   uint32
	inBay []bool
	queue []int32
}

func beginFeatures(b *Builder) int {
	if !b.p.v1() {
		return 0
	}
	n := b.w.TileCount()
	b.g.feat = &featState{mark: make([]uint32, n), inBay: make([]bool, n)}
	return featurePasses
}

func itemFeatures(b *Builder, k int) {
	p, fs := b.w, b.g.feat
	land := func(t int) bool { return p.Water[t] == Land }
	switch k {
	case 0: // bays: groups of ocean coves
		cove := func(t int) bool {
			if p.Water[t] != Ocean {
				return false
			}
			n := 0
			for _, nb := range p.Neighbours(t) {
				if land(int(nb)) {
					n++
				}
			}
			return n >= bayMinLand
		}
		for _, r := range fs.regions(p, cove, nil) {
			if len(r) >= bayMinTiles && len(r) <= bayMaxTiles {
				for _, t := range r {
					fs.inBay[t] = true
				}
				addFeature(p, FeatBay, r)
			}
		}
	case 1: // water bodies outside the bays
		water := func(t int) bool { return p.Water[t] != Land && !fs.inBay[t] }
		for _, r := range fs.regions(p, water, nil) {
			lake := true
			for _, t := range r {
				lake = lake && p.Water[t] == Lake
			}
			switch {
			case lake || len(r) < seaMinTiles:
				addFeature(p, FeatLake, r)
			case len(r) < oceanMinTiles:
				addFeature(p, FeatSea, r)
			default:
				for _, part := range fs.split(p, r, water) {
					addFeature(p, FeatOcean, part)
				}
			}
		}
	case 2:
		high := func(t int) bool { return land(t) && p.Hill[t] >= site.LargeHills }
		addRegions(p, fs, FeatMountains, mountainMin, high, nil)
	case 3:
		dry := func(t int) bool {
			return land(t) && (p.Biome[t] == biome.Desert || p.Biome[t] == biome.ExtremeDesert)
		}
		addRegions(p, fs, FeatDesert, desertMin, dry, nil)
	case 4:
		wood := func(t int) bool {
			switch p.Biome[t] {
			case biome.TemperateForest, biome.BorealForest, biome.TropicalRainforest:
				return land(t)
			}
			return false
		}
		addRegions(p, fs, FeatForest, forestMin, wood, func(a, c int) bool { return p.Biome[a] == p.Biome[c] })
	case 5:
		ice := func(t int) bool { return land(t) && (p.Biome[t] == biome.IceSheet || p.Biome[t] == biome.SeaIce) }
		addRegions(p, fs, FeatIceCap, iceCapMin, ice, nil)
	}
}

// featurePriority picks a tile's primary feature: the most specific kind
// wins where regions overlap (a mountain range inside a forest).
var featurePriority = [...]int{FeatMountains: 0, FeatBay: 1, FeatLake: 2, FeatDesert: 3, FeatIceCap: 4,
	FeatForest: 5, FeatSea: 6, FeatOcean: 7}

func endFeatures(b *Builder) {
	if !b.p.v1() {
		return
	}
	p := b.w
	r := randx.New(randx.Mix64(p.Seed ^ saltFeatures))
	used := map[string]bool{}
	for k := range p.ext.features {
		p.ext.features[k].Name = featureName(r, p.ext.features[k].Kind, used)
	}
	p.ext.featureAt = make([]int32, p.TileCount())
	for i := range p.ext.featureAt {
		p.ext.featureAt[i] = -1
	}
	for k, f := range p.ext.features {
		for _, t := range f.Tiles {
			cur := p.ext.featureAt[t]
			if cur < 0 || featurePriority[f.Kind] < featurePriority[p.ext.features[cur].Kind] {
				p.ext.featureAt[t] = int32(k)
			}
		}
	}
	p.ext.name = planetName(randx.New(randx.Mix64(p.Seed ^ saltName)))
	b.g.feat = nil
}

// addRegions records every region of member tiles (joined where same
// holds, any neighbour when nil) of at least minTiles as a kind feature.
func addRegions(p *Planet, fs *featState, kind FeatureKind, minTiles int, member func(int) bool, same func(a, c int) bool) {
	for _, r := range fs.regions(p, member, same) {
		if len(r) >= minTiles {
			addFeature(p, kind, r)
		}
	}
}

// addFeature appends an (as yet unnamed) feature over tiles in index
// order, centred on the member nearest their mean direction.
func addFeature(p *Planet, kind FeatureKind, tiles []int32) {
	slices.Sort(tiles)
	var m [3]float64
	for _, t := range tiles {
		c := f64(p.Center[t])
		m = [3]float64{m[0] + c[0], m[1] + c[1], m[2] + c[2]}
	}
	centre, best := int(tiles[0]), -2.0
	for _, t := range tiles {
		if d := dot64(f64(p.Center[t]), m); d > best {
			centre, best = int(t), d
		}
	}
	p.ext.features = append(p.ext.features, Feature{Kind: kind, Centre: centre, Tiles: tiles})
}

// regions are the connected groups of member tiles, each found from its
// lowest tile, in that order; neighbours join when same holds (nil: all).
func (fs *featState) regions(p *Planet, member func(int) bool, same func(a, c int) bool) [][]int32 {
	fs.gen++
	var out [][]int32
	for i := 0; i < p.TileCount(); i++ {
		if fs.mark[i] == fs.gen || !member(i) {
			continue
		}
		fs.mark[i] = fs.gen
		r := []int32{int32(i)}
		for h := 0; h < len(r); h++ {
			t := int(r[h])
			for _, nb := range p.Neighbours(t) {
				if fs.mark[nb] != fs.gen && member(int(nb)) && (same == nil || same(t, int(nb))) {
					fs.mark[nb] = fs.gen
					r = append(r, nb)
				}
			}
		}
		out = append(out, r)
	}
	return out
}

// split cuts a big ocean region into about len/oceanChunk parts: seeds by
// farthest-point sampling over hops (the first is the lowest tile), then
// every tile joins the seed it is fewest hops from.
func (fs *featState) split(p *Planet, region []int32, member func(int) bool) [][]int32 {
	parts := min(oceanMaxParts, len(region)/oceanChunk)
	if parts < 2 {
		return [][]int32{region}
	}
	low := slices.Min(region)
	seeds := []int32{low}
	owner := map[int32]int{}
	for {
		last := fs.flood(p, seeds, member, owner)
		if len(seeds) == parts {
			break
		}
		seeds = append(seeds, last)
	}
	out := make([][]int32, parts)
	for _, t := range region {
		k := owner[t]
		out[k] = append(out[k], t)
	}
	return out
}

// flood is a multi-source breadth-first search from seeds over member
// tiles, recording each tile's seed in owner; it returns the last tile
// reached (the farthest from every seed).
func (fs *featState) flood(p *Planet, seeds []int32, member func(int) bool, owner map[int32]int) int32 {
	fs.gen++
	q := fs.queue[:0]
	for k, s := range seeds {
		fs.mark[s] = fs.gen
		owner[s] = k
		q = append(q, s)
	}
	for h := 0; h < len(q); h++ {
		t := q[h]
		for _, nb := range p.Neighbours(int(t)) {
			if fs.mark[nb] != fs.gen && member(int(nb)) {
				fs.mark[nb] = fs.gen
				owner[nb] = owner[t]
				q = append(q, nb)
			}
		}
	}
	fs.queue = q
	return q[len(q)-1]
}

// Features are the planet's named regions (nil on a v0 world).
func (p *Planet) Features() []Feature { return p.ext.features }

// FeatureAt is the index into Features of tile i's primary feature, or
// -1 for none (or a v0 world).
func (p *Planet) FeatureAt(i int) int {
	if i < 0 || i >= len(p.ext.featureAt) {
		return -1
	}
	return int(p.ext.featureAt[i])
}

// Name is the planet's generated name ("" on a v0 world).
func (p *Planet) Name() string { return p.ext.name }

// Find searches feature and settlement names for q, ignoring case and a
// leading "the": exact names first, then name prefixes, word prefixes and
// any substring, shorter names first within a rank; at most
// findMaxResults hits, none for a blank query.
func (p *Planet) Find(q string) []Hit {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	type ranked struct {
		Hit
		rank int
	}
	var hits []ranked
	add := func(h Hit) {
		if r := matchRank(strings.ToLower(h.Name), q); r >= 0 {
			hits = append(hits, ranked{h, r})
		}
	}
	for k, f := range p.ext.features {
		add(Hit{Name: f.Name, Kind: f.Kind, Feature: k, Tile: f.Centre})
	}
	for _, t := range p.SettlementTiles() {
		add(Hit{Name: p.SettlementName(int(t)), Kind: FeatSettlement, Feature: -1, Tile: int(t)})
	}
	slices.SortStableFunc(hits, func(a, b ranked) int {
		switch {
		case a.rank != b.rank:
			return a.rank - b.rank
		case len(a.Name) != len(b.Name):
			return len(a.Name) - len(b.Name)
		case a.Name != b.Name:
			return strings.Compare(a.Name, b.Name)
		}
		return a.Tile - b.Tile
	})
	out := make([]Hit, 0, min(len(hits), findMaxResults))
	for _, h := range hits[:min(len(hits), findMaxResults)] {
		out = append(out, h.Hit)
	}
	return out
}

// matchRank is how well lower-case name matches q: 0 exact, 1 prefix,
// 2 word prefix, 3 substring, -1 no match.
func matchRank(name, q string) int {
	bare := strings.TrimPrefix(name, "the ")
	switch {
	case name == q || bare == q:
		return 0
	case strings.HasPrefix(name, q) || strings.HasPrefix(bare, q):
		return 1
	case strings.Contains(" "+name, " "+q):
		return 2
	case strings.Contains(name, q):
		return 3
	}
	return -1
}
