package env

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

func swayPose(on bool, present uint64) float64 {
	var e Sky
	e.SetWeather(game.WorldView{WindX: 2, WindY: 0.5})
	v := &render.View{Cam: camera.Camera{Zoom: 1}, Present: present}
	v.Prefs.PlantSway = on
	return e.PlantPose(v, 5, 5, KindGrass, 0, 1).Skew
}

func TestPlantSwayOffFreezesThePhase(t *testing.T) {
	t.Parallel()
	if a, b := swayPose(false, 100), swayPose(false, 4000); a != b {
		t.Fatalf("sway off: pose moved %v -> %v", a, b)
	}
	if a, b := swayPose(true, 100), swayPose(true, 4000); a == b {
		t.Fatalf("sway on: pose stayed %v", a)
	}
}
