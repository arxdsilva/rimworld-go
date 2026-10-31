package inspector

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// Hydroponics basin and sun lamp panes (FOOD-5, FOOD-2 contract): the
// basin's plants and soil, the lamp's hours and reach. They read
// BuildingView.Glow and GlowRadius and the world's plants (a basin cell's
// plant reads fertility 280); the power lines come from PowerLines.

// Basin and lamp kind names (construction names).
const (
	BasinKind   = "hydroponics basin"
	SunLampKind = "sun lamp"
	// SunLampHours is the lamp's lit window (wiki: Sun lamp).
	SunLampHours = "06:00 to 19:12"
	basinFert    = 280
)

// BasinPlants are the plants growing in basin b: the ones on its cells.
func BasinPlants(b game.BuildingView, plants []game.PlantView) []game.PlantView {
	var out []game.PlantView
	for i := range plants {
		p := plants[i]
		if p.X >= b.X && p.X < b.X+max(b.W, 1) && p.Y >= b.Y && p.Y < b.Y+max(b.H, 1) {
			out = append(out, p)
		}
	}
	return out
}

// GrowLines are the pane lines of a hydroponics basin or a sun lamp (nil
// for any other building).
func GrowLines(b game.BuildingView, plants []game.PlantView) []string {
	switch b.Kind {
	case BasinKind:
		ps := BasinPlants(b, plants)
		out := []string{fmt.Sprintf("Soil fertility %d%%: crops grow far faster than in the ground", basinFert)}
		out = append(out, fmt.Sprintf("Plants: %d of %d cells", len(ps), max(b.W, 1)*max(b.H, 1)))
		for _, p := range ps {
			state := fmt.Sprintf("%d%%", p.Growth)
			if p.Harvestable {
				state = "ready"
			}
			out = append(out, fmt.Sprintf("  %s: %s", Capitalize(p.Species), state))
		}
		out = append(out, "Needs power and a sun lamp or sunlight (crops rest at night)")
		return out
	case SunLampKind:
		state := "dark now"
		if b.Glow {
			state = "shining"
		}
		r := b.GlowRadius
		if r == 0 {
			r = 11 // wiki: Sun lamp
		}
		return []string{
			"Lamp: " + state,
			fmt.Sprintf("Shines %s while powered, %d tiles around", SunLampHours, r),
			"Plants in its light grow indoors; they rest at night like outdoor ones",
		}
	}
	return nil
}
