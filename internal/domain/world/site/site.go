// Package site is what a world tile means for the local colony map: biome,
// climate, hills, river, coast, lakes, caves, stones and road (NG-1 §4.2).
// The zero Site is today's map (legacy generation).
package site

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/temperature"
)

// Hills is how rocky the local map is.
type Hills uint8

const (
	HillsDefault Hills = iota // legacy thresholds
	Flat
	SmallHills
	LargeHills
	Mountainous
)

// RiverSize is the river crossing the map.
type RiverSize uint8

const (
	NoRiver RiverSize = iota
	Creek
	River
	LargeRiver
)

// Dir8 is a map edge direction: 0 N, 1 NE, 2 E, ... 7 NW.
type Dir8 uint8

// Dir8Set is a set of edge directions.
type Dir8Set uint8

// Has reports d in s.
func (s Dir8Set) Has(d Dir8) bool { return s&(1<<d) != 0 }

// RiverPath is where a river enters and leaves.
type RiverPath struct {
	Size    RiverSize
	In, Out Dir8
}

// Site is the local map's parameters.
type Site struct {
	Biome    biome.ID
	Climate  temperature.Climate
	Hills    Hills
	River    RiverPath
	Coast    Dir8Set
	Lakes    uint8
	Caves    bool
	Stones   []geology.Stone
	Road     Dir8Set
	Latitude int16 // tenths of a degree; negative = south
}

// Zero reports the legacy (unset) site.
func (s Site) Zero() bool {
	return s.Hills == HillsDefault && s.River.Size == NoRiver && s.Coast == 0 && s.Lakes == 0 &&
		!s.Caves && len(s.Stones) == 0 && s.Road == 0
}

var hillLabels = [...]string{HillsDefault: "Default", Flat: "Flat", SmallHills: "Small hills", LargeHills: "Large hills", Mountainous: "Mountainous"}

// Label names a hills value ("Small hills"); unknown values are "".
func (h Hills) Label() string {
	if int(h) >= len(hillLabels) {
		return ""
	}
	return hillLabels[h]
}

var riverLabels = [...]string{NoRiver: "", Creek: "Creek", River: "River", LargeRiver: "Large river"}

// Label names a river size ("Large river"); NoRiver is "".
func (r RiverSize) Label() string {
	if int(r) >= len(riverLabels) {
		return ""
	}
	return riverLabels[r]
}
