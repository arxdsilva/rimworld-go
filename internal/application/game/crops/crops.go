// Package crops is the catalogue of what a growing zone can sow, as the
// UI reads it (crop picker, hover). A consumer sub-package of
// internal/application/game (.cursor/SOLID_GO.md §10): pure functions
// over the domain plant and terrain tables, nothing from the game.
package crops

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// View describes a sowable crop for the UI (crop picker, hover).
type View struct {
	Name      string  // species name (zone crop / CropID key)
	Label     string  // produce name
	Days      float64 // days to harvest on plain soil, daylight growth
	GravelDay float64 // days on poor (gravel) soil
	RichDays  float64 // days on rich soil
	Yield     int     // items per plant
	Nutrition int     // per item (a colonist eats about 5000 a day)
	Sens      int     // soil sensitivity, percent
	// FrostKill is the air temperature (tenths of a degree C) that kills
	// the crop; ShelfDays how long the produce keeps at room temperature
	// (0 = forever). Days assume growing weather (10 C or warmer).
	FrostKill int
	ShelfDays float64
	// Food: the produce is edible. Extra and ExtraQty: a by-product per
	// harvest ("straw", 4) (SIM-021).
	Food     bool
	Extra    string
	ExtraQty int
}

// All lists the sowable crops, fastest first.
func All() []View {
	var out []View
	for _, s := range plant.Crops() {
		d := plant.DefOf(s)
		out = append(out, View{
			Name: d.Name, Label: d.Label, Days: plant.GrowDays(s, terrain.Soil.Fertility()),
			GravelDay: plant.GrowDays(s, terrain.Gravel.Fertility()), RichDays: plant.GrowDays(s, terrain.RichSoil.Fertility()),
			Yield: d.Yield, Nutrition: d.Yields.Nutrition(), Sens: d.Sens,
			FrostKill: d.FrostKill, ShelfDays: float64(d.Yields.ShelfTicks()) / clock.TicksPerDay,
			Food: d.Yields.Nutrition() > 0,
		})
		if d.ExtraQty > 0 {
			out[len(out)-1].Extra, out[len(out)-1].ExtraQty = d.Extra.String(), d.ExtraQty
		}
	}
	return out
}

// ZoneSpecies lists what a growing zone can sow: the crops, then trees.
func ZoneSpecies() []View {
	out := All()
	for _, s := range []plant.Species{plant.Tree, plant.Pine, plant.JungleTree} { // tree zones (SIM-023)
		d := plant.DefOf(s)
		out = append(out, View{
			Name: d.Name, Label: "wood", Days: plant.GrowDays(s, terrain.Soil.Fertility()),
			GravelDay: plant.GrowDays(s, terrain.Gravel.Fertility()),
			RichDays:  plant.GrowDays(s, terrain.RichSoil.Fertility()),
			Yield:     d.Wood, Sens: d.Sens,
		})
	}
	return out
}

// ID is the command value (Command.Build) for a crop name.
func ID(name string) (uint8, bool) {
	s, ok := plant.Parse(name)
	if !ok || !plant.DefOf(s).Crop {
		return 0, false
	}
	return uint8(s), true
}

// ZoneID is the DesignateGrowZone command value (Command.Build) for any
// species a growing zone can sow: crops and trees.
func ZoneID(name string) (uint8, bool) {
	s, ok := plant.Parse(name)
	if !ok || !plant.DefOf(s).ZoneSowable() {
		return 0, false
	}
	return uint8(s), true
}
