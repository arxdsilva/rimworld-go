package hud

import (
	"fmt"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/biome"
)

// Temperature alerts (TEMP-5): "Need warm clothes" (vanilla Alert_NeedWarmClothes:
// a colonist whose comfortable minimum is above the coming cold) and
// hypothermic animals. Both read the views; neither scans the world per
// frame beyond the pawn and animal lists the alert list already walks.

// animalColdAlert is the hypothermia percent at which a tame animal is
// called out (the same 20 % as the colonists' exposureAlert).
const animalColdAlert = 20

// ComingLow is the coldest the coming days are expected to get, tenths of a
// degree C, and whether the cold season is near at all: the year's winter
// low in the fall (time to prepare) and winter, nothing in spring or
// summer (original: the model has no forecast). It depends only on the
// season, so the alert never flaps with the day's swing.
func ComingLow(w game.WorldView) (low int, ok bool) {
	id, found := biome.Parse(w.BiomeKey)
	if !found {
		return 0, false
	}
	winter := game.BiomeInfoOf(id).WinterLow * 10
	if w.Season == "Fall" || w.Season == "Winter" {
		return winter, true
	}
	return 0, false
}

// NeedWarmClothesAlert lists the colonists who would be cold in the coming
// season's low in what they wear now.
func NeedWarmClothesAlert(pawns []game.PawnView, w game.WorldView) (Alert, bool) {
	low, ok := ComingLow(w)
	if !ok {
		return Alert{}, false
	}
	var at [][2]int
	name := ""
	for _, p := range pawns {
		if p.Status == "dead" || p.ComfortMax <= p.ComfortMin || p.ComfortMin <= low {
			continue
		}
		if name == "" {
			name = p.Name
		}
		at = append(at, [2]int{p.X, p.Y})
	}
	switch len(at) {
	case 0:
		return Alert{}, false
	case 1:
		return Alert{Key: "warmclothes", Label: name + " needs warm clothes", Targets: at}, true
	}
	return Alert{Key: "warmclothes", Label: fmt.Sprintf("%d colonists need warm clothes", len(at)), Targets: at}, true
}

// ColdAnimalAlert lists the tame animals with hypothermia.
func ColdAnimalAlert(w game.WorldView) (Alert, bool) {
	var at [][2]int
	for _, an := range w.Animals {
		if an.Tame && an.Hypothermia >= animalColdAlert {
			at = append(at, [2]int{an.X, an.Y})
		}
	}
	switch len(at) {
	case 0:
		return Alert{}, false
	case 1:
		return Alert{Key: "animalcold", Label: "Hypothermic animal", Targets: at}, true
	}
	return Alert{Key: "animalcold", Label: fmt.Sprintf("Hypothermic animals (%d)", len(at)), Targets: at}, true
}
