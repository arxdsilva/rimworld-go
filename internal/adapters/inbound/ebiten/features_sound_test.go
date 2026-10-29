package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"testing"
)

// MAPGEN-2: a vent hisses once per burst, however many frames draw it.
func TestSteamHissOncePerBurst(t *testing.T) {
	r := &Renderer{}
	x, y := 7, 3
	period := uint64(env.SteamPeriod(x, y))
	start := period - env.SteamPhase(x, y) // a burst begins here
	for now := start; now < start+env.SteamFrameTicks*env.SteamFrames; now++ {
		r.pipe().Lb.SteamHiss(now, x, y)
	}
	if n := len(r.cueBus().Take()); n != 1 {
		t.Fatalf("%d cues in one burst, want 1", n)
	}
	r.pipe().Lb.SteamHiss(start+period, x, y)
	cues := r.cueBus().Take()
	if len(cues) != 1 || cues[0].ID != "sfx.steam_vent" || cues[0].X != 8 || cues[0].Y != 4 {
		t.Fatalf("next burst cues %+v", cues)
	}
	r.pipe().Lb.SteamHiss(start+period, 20, 20) // another vent hisses on its own
	if len(r.cueBus().Take()) != 1 {
		t.Fatal("each vent has its own burst record")
	}
}
