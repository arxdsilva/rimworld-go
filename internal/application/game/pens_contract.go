package game

import (
	"encoding/json"

	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-015 pens contract (pens.go fills the behaviour).

// PenSpecies is one penned species' acceptance in a pen.
type PenSpecies struct {
	Key                 string // animal species key
	Allow, Male, Female bool
}

// PenSettings is a pen marker's configuration.
type PenSettings struct {
	Species []PenSpecies
	// AutoCut lists plant species keys designated for cutting inside.
	AutoCut []string
}

// EncodePenSettings is the SetPenSettings command's Name payload.
func EncodePenSettings(s PenSettings) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// DecodePenSettings reads EncodePenSettings' output.
func DecodePenSettings(s string) (PenSettings, bool) {
	var out PenSettings
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return PenSettings{}, false
	}
	return out, true
}

// PenView is one pen as the renderer sees it.
type PenView struct {
	ID       entity.EntityID // the pen marker
	X, Y     int             // the marker
	Name     string          // "Pen 1"
	Enclosed bool
	// Bits is the pen's cells (row-major bitset over the map; nil when not
	// enclosed).
	Bits     []uint64
	Settings PenSettings
	Animals  int
	// Food (nutrition per day, our units): what the pen's grazing grows,
	// what its herd eats, and the troughs' stock.
	GrazePerDay, DemandPerDay, TroughStock int
}

// penAnimalSave is one penned animal's unpenned clock (PLAY-015): Since
// is the tick it last lost its pen (roping or penning resets it).
type penAnimalSave struct {
	Animal entity.EntityID
	Since  uint64
}

// penSave is one pen marker's saved settings.
type penSave struct {
	Marker   entity.EntityID
	Settings PenSettings
}
