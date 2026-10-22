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
	Settlement []int32 // index into Factions' settlements, -1 none
	Roads      [][2]int32
	Factions   []faction.Faction
}

// Builder runs generation in steps of a given number of work units.
type Builder struct {
	p     Params
	phase Phase
	out   *Planet
}

// NewBuilder starts generating a planet.
func NewBuilder(p Params) *Builder { return &Builder{p: p} } // NG-1 stub (PLANET)

// Step does up to units of work and reports progress.
func (b *Builder) Step(units int) (Phase, float32, bool) { return PhaseDone, 1, true } // NG-1 stub (PLANET)

// Planet is the finished world (nil until done).
func (b *Builder) Planet() *Planet { return b.out } // NG-1 stub (PLANET)

// TileCount is the number of generated tiles.
func (p *Planet) TileCount() int { return len(p.Center) }

// ValidLanding reports whether a colony may land on tile i, and why not.
func (p *Planet) ValidLanding(i int) (bool, string) { return false, "" } // NG-1 stub (PLANET)

// Site is the local map's parameters for tile i in the given hemisphere season.
func (p *Planet) Site(i int) site.Site { return site.Site{} } // NG-1 stub (PLANET)

// GrowingDays is the growing period at tile i (of 60).
func (p *Planet) GrowingDays(i int) int { return 0 } // NG-1 stub (PLANET)

// TileAt is the tile under a unit direction.
func (p *Planet) TileAt(dir [3]float32) int { return -1 } // NG-1 stub (PLANET)

// RandomLanding picks a valid tile deterministically for the n-th press.
func (p *Planet) RandomLanding(n int) int { return -1 } // NG-1 stub (PLANET)

// Hops is the hop distance between tiles (capped at limit).
func (p *Planet) Hops(a, b, limit int) int { return limit } // NG-1 stub (PLANET)

// Key hashes the params (a planet is reused when its key matches).
func (p Params) Key() string { return "" } // NG-1 stub (PLANET)
