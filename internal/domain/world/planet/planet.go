// Package planet generates the world: a Goldberg hex sphere with
// elevation, climate, biomes, hills, rivers, lakes, caves, stones,
// faction settlements and roads, built in budgeted steps so the UI can
// show progress (NG-1 §4.1). Deterministic from the seed text.
package planet

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// Frequency is the icosphere subdivision: 10*F*F+2 tiles on the full globe.
const Frequency = 48

// Params are the Create World settings.
type Params struct {
	SeedText string
	Coverage int // 30, 50 or 100 (5 in dev mode)
	Rain     int // 0..4 (very dry..very wet)
	Temp     int // 0..4 (very cold..very warm)
	Pop      int // 0..2 (sparse, normal, crowded)
	Factions []faction.Slot
	Version  uint16
}

// Phase is a generation step.
type Phase uint8

const (
	PhaseShape Phase = iota
	PhaseElevation
	PhaseClimate
	PhaseBiomes
	PhaseRivers
	PhaseFactions
	PhaseRoads
	PhaseDone
)

// Water kinds per tile.
const (
	Land  uint8 = 0
	Ocean uint8 = 1
	Lake  uint8 = 2
)

// Planet is a generated world (struct of arrays indexed by tile).
//
// Geometry: Center is the unit-sphere tile centre. Corners[CornerOff[i]:
// CornerOff[i+1]] is tile i's polygon (5 or 6 unit vectors, counter-
// clockwise seen from outside). Neigh[NeighOff[i]:NeighOff[i+1]] are the
// kept neighbours, also counter-clockwise; for a tile with all its
// neighbours kept (every tile at 100% coverage), corner k lies between
// neighbour k and neighbour k+1, so the edge shared with neighbour k runs
// from corner k-1 to corner k. North is +Z; Lat/Lon come from Center.
//
// Water tiles keep Biome 0 (TemperateForest) except Sea Ice, which is
// frozen ocean stored as land (Water Land, Biome SeaIce, Elev < 0).
type Planet struct {
	Params     Params
	Seed       uint64
	Center     [][3]float32
	Corners    [][3]float32
	CornerOff  []uint32
	Neigh      []int32
	NeighOff   []uint32
	Lat, Lon   []int16 // tenths of a degree
	Elev       []int16 // metres
	Hill       []site.Hills
	Biome      []biome.ID
	Water      []uint8
	Temp       []int16 // tenths of a degree C, yearly mean
	Rain       []int16 // mm per year
	River      []site.RiverSize
	RiverTo    []int32
	Caves      []bool
	Stones     [][3]geology.Stone
	Settlement []int32 // global settlement index (factions in order, capital first), -1 none
	Roads      [][2]int32
	Factions   []faction.Faction

	// CoastDist is the hop distance to the nearest ocean tile (0 on the
	// ocean, capped at 255).
	CoastDist []uint8
	// Pole is the centre of the coverage cap; CapCos its cosine radius
	// (a direction d is covered when dot(d, Pole) >= CapCos).
	Pole   [3]float32
	CapCos float32

	settlements []settlementRef
	radCos      []float32 // cos of the tile's largest centre-corner angle
	buckets     []int32   // TileAt lat/lon grid
	stepCos     float64   // cos of the largest neighbour angle
	landCum     []uint32  // RandomLanding cumulative weights
	landTile    []int32
	hopMark     []uint32 // Hops scratch (not safe for concurrent use)
	hopGen      uint32
}

type settlementRef struct {
	faction, k int
	tile       int32
}

// Builder runs generation in steps of a given number of work units.
type Builder struct {
	p     Params
	phase Phase
	out   *Planet

	w      *Planet
	stage  int
	began  bool
	cursor int
	count  int
	g      gen

	genFactions func([]faction.Slot, *randx.Rand) []faction.Faction
	place       func(faction.Globe, []faction.Faction, int, *randx.Rand)
}

// NewBuilder starts generating a planet.
func NewBuilder(p Params) *Builder {
	return &Builder{p: p, genFactions: faction.Generate, place: faction.PlaceSettlements}
}

// Planet is the finished world (nil until done).
func (b *Builder) Planet() *Planet { return b.out }

// TileCount is the number of generated tiles.
func (p *Planet) TileCount() int { return len(p.Center) }
