// Package plant is map plants: wild trees and berry bushes, and sown crops.
// Growth is an integer tick counter; a plant is ripe at GrowTicks.
package plant

import "rimworld-go/internal/domain/economy/item"

// Species is a plant kind.
type Species uint8

const (
	BerryBush Species = iota
	Tree
	Amberroot // farm crop
	numSpecies
)

// Def is the static definition of a species.
type Def struct {
	Species     Species
	Name        string
	GrowTicks   int // ticks from sown to ripe at 100% fertility
	HarvestWork int // ticks to harvest when ripe
	Yield       int // food units from a ripe harvest
	Yields      item.Kind
	CutWork     int  // ticks to cut down
	Wood        int  // wood from cutting a fully grown plant
	Regrows     bool // harvest resets growth instead of removing the plant
	RegrowTo    int  // growth percent after a regrowing harvest
	Crop        bool // sown in growing zones
}

var defs = [numSpecies]Def{
	BerryBush: {Species: BerryBush, Name: "berry_bush", GrowTicks: 12000, HarvestWork: 30, Yield: 5, Yields: item.Berries,
		CutWork: 20, Regrows: true, RegrowTo: 35},
	Tree: {Species: Tree, Name: "tree", GrowTicks: 60000, CutWork: 90, Wood: 8},
	Amberroot: {Species: Amberroot, Name: "amberroot", GrowTicks: 15000, HarvestWork: 25, Yield: 8, Yields: item.Amberroot,
		CutWork: 10, Crop: true},
}

// DefOf returns the definition for s.
func DefOf(s Species) Def {
	if s >= numSpecies {
		return Def{}
	}
	return defs[s]
}

func (s Species) String() string { return DefOf(s).Name }

// Parse returns the species for a name.
func Parse(name string) (Species, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Species, true
		}
	}
	return 0, false
}

// Legacy constants kept for older fixtures.
const (
	HarvestWork  = 30
	HarvestYield = 5
)

// Plant is a plant on the map.
type Plant struct {
	Species    Species
	X, Y       int
	Growth     int  // 0..GrowTicks
	Designated bool // marked for harvest
	Cut        bool // marked for cutting
	Zone       bool // sown by a growing zone
}

// Wild is the historical name for Plant.
type Wild = Plant

// Ripe reports the plant can be harvested (or a tree is fully grown).
func (p Plant) Ripe() bool {
	d := DefOf(p.Species)
	return d.GrowTicks > 0 && p.Growth >= d.GrowTicks
}

// Harvestable is true for ripe food plants.
func (p Plant) Harvestable() bool { return p.Ripe() && DefOf(p.Species).Yield > 0 }

// Percent is growth in 0..100.
func (p Plant) Percent() int {
	d := DefOf(p.Species)
	if d.GrowTicks <= 0 {
		return 100
	}
	v := p.Growth * 100 / d.GrowTicks
	if v > 100 {
		return 100
	}
	return v
}

// Stage is the visual/gameplay growth stage 0 (sown) .. 3 (ripe).
func (p Plant) Stage() int {
	switch pc := p.Percent(); {
	case pc >= 100:
		return 3
	case pc >= 55:
		return 2
	case pc >= 20:
		return 1
	default:
		return 0
	}
}

// WoodYield is the wood from cutting p now (scaled by growth).
func (p Plant) WoodYield() int {
	return DefOf(p.Species).Wood * p.Percent() / 100
}

// Grow advances growth by one tick at fertility percent. Growth uses an
// integer accumulator so it stays deterministic.
func (p Plant) Grow(fertility int, tick uint64) Plant {
	d := DefOf(p.Species)
	if fertility <= 0 || p.Growth >= d.GrowTicks {
		return p
	}
	// fertility 100 -> +1 per tick, 140 -> +1.4 per tick on average,
	// using the integer difference of a running total (deterministic).
	if tick == 0 {
		return p
	}
	f := uint64(fertility)
	p.Growth += int(tick*f/100 - (tick-1)*f/100)
	if p.Growth > d.GrowTicks {
		p.Growth = d.GrowTicks
	}
	return p
}
