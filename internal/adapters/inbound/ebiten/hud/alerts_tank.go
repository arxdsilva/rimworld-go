package hud

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// "Fish dying" (FISH): a stocked tank is losing its stock to a power cut, the
// air or an empty feeder.

// FishDyingAlertKey is the alert's key.
const FishDyingAlertKey = "fishdying"

// FishDyingAlert lists the stocked tanks that are in trouble. It reads the
// building list the alert pass already holds.
func FishDyingAlert(w game.WorldView) (Alert, bool) {
	var at [][2]int
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.Tank != nil && b.Tank.Stocked && b.Tank.Trouble != "" {
			at = append(at, [2]int{b.X, b.Y})
		}
	}
	switch len(at) {
	case 0:
		return Alert{}, false
	case 1:
		return Alert{Key: FishDyingAlertKey, Label: "Fish dying", Targets: at}, true
	}
	return Alert{Key: FishDyingAlertKey, Label: fmt.Sprintf("Fish dying (%d tanks)", len(at)), Targets: at}, true
}
