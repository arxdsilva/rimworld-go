package planet

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
)

// extData is the planet's derived query tables and the WG (v1) data, kept
// in one field so Planet stays inside its field budget. Everything here
// is written once by the builder (or finish) and read-only afterwards,
// except hops, the scratch behind the non-concurrent Hops.
type extData struct {
	landCum  []uint32 // RandomLanding cumulative weights
	landTile []int32
	roadAt   []bool     // tile touches a road segment
	hops     HopScratch // Hops scratch (not safe for concurrent use)

	roadClass []RoadClass // v1: parallel to Planet.Roads (nil on v0)
	features  []Feature   // v1: named world features
	featureAt []int32     // v1: tile -> primary feature index, -1 none
	name      string      // v1: the planet's name
}

// HopScratch is caller-owned breadth-first search scratch for HopsWith
// and NearbyBasesWith. The zero value is ready; use one per goroutine.
type HopScratch struct {
	mark        []uint32
	gen         uint32
	front, next []int32
}

// start begins a search over n tiles and returns its mark generation.
func (s *HopScratch) start(n int) uint32 {
	if len(s.mark) != n {
		s.mark, s.gen = make([]uint32, n), 0
	}
	s.gen++
	if s.gen == 0 {
		clear(s.mark)
		s.gen = 1
	}
	return s.gen
}

// RoadClass is how a road segment is built (WG-07, wiki: World
// generation). Every class moves caravans at the same speed; only the
// look differs.
type RoadClass uint8

const (
	Path RoadClass = iota
	DirtRoad
	StoneRoad
	AncientAsphaltRoad
	AncientHighway
	NumRoadClasses
)

var roadClassLabels = [NumRoadClasses]string{"Path", "Dirt road", "Stone road", "Ancient asphalt road", "Ancient highway"}

// Label names the class ("Stone road"); unknown values are "".
func (c RoadClass) Label() string {
	if c >= NumRoadClasses {
		return ""
	}
	return roadClassLabels[c]
}

// RoadClassOf is the class of road segment Roads[k]: every v0 road is a
// dirt road; -1 or out of range gives DirtRoad too.
func (p *Planet) RoadClassOf(k int) RoadClass {
	if k < 0 || k >= len(p.ext.roadClass) {
		return DirtRoad
	}
	return p.ext.roadClass[k]
}

// FeatureKind is what a named world feature is (WG-09).
type FeatureKind uint8

const (
	FeatOcean FeatureKind = iota + 1
	FeatSea
	FeatLake
	FeatBay
	FeatMountains
	FeatDesert
	FeatForest
	FeatIceCap
	// FeatSettlement marks a Find hit on a faction settlement.
	FeatSettlement
)

var featureKindLabels = [...]string{FeatOcean: "Ocean", FeatSea: "Sea", FeatLake: "Lake", FeatBay: "Bay",
	FeatMountains: "Mountain range", FeatDesert: "Desert", FeatForest: "Forest", FeatIceCap: "Ice cap", FeatSettlement: "Settlement"}

// Label names the kind ("Mountain range"); unknown values are "".
func (k FeatureKind) Label() string {
	if int(k) >= len(featureKindLabels) {
		return ""
	}
	return featureKindLabels[k]
}

// Feature is a named region of the world: Centre is the member tile
// nearest the region's mean direction, Tiles its members in index order.
type Feature struct {
	Name   string
	Kind   FeatureKind
	Centre int
	Tiles  []int32
}

// Hit is one Find result: a feature (Feature is its index) or a
// settlement (Kind FeatSettlement, Feature -1); Tile is where to look.
type Hit struct {
	Name    string
	Kind    FeatureKind
	Feature int
	Tile    int
}

// Base is a faction settlement near a tile: Faction indexes Planet.
// Factions, Rank is its place in that faction (0 = capital).
type Base struct {
	Tile    int
	Faction int
	Rank    int
	Hops    int
}

// Tri is a yes/no/any filter switch; the zero value is any.
type Tri uint8

const (
	Any Tri = iota
	Yes
	No
)

// ok reports whether have passes the switch.
func (t Tri) ok(have bool) bool { return t == Any || (t == Yes) == have }

// Filter narrows a random landing (WG-11). Every zero field is "any":
// an empty Biomes set or Hills mask, TempMin == TempMax == 0, Stone None.
type Filter struct {
	Biomes           biome.Set
	TempMin, TempMax float64 // yearly mean, °C
	Hills            uint8   // bit per site.Hills value
	River            Tri
	Coast            Tri
	Road             Tri
	Stone            geology.Stone
	Caves            Tri
}
