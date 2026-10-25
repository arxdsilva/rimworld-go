package ebiten

import "testing"

// MAPGEN-2: a vent hisses once per burst, however many frames draw it.
func TestSteamHissOncePerBurst(t *testing.T) {
	r := &Renderer{}
	x, y := 7, 3
	period := uint64(steamPeriod(x, y))
	start := period - steamPhase(x, y) // a burst begins here
	for now := start; now < start+steamFrameTicks*steamFrames; now++ {
		r.steamHiss(now, x, y)
	}
	if n := len(r.takeCues()); n != 1 {
		t.Fatalf("%d cues in one burst, want 1", n)
	}
	r.steamHiss(start+period, x, y)
	cues := r.takeCues()
	if len(cues) != 1 || cues[0].id != "sfx.steam_vent" || cues[0].x != 8 || cues[0].y != 4 {
		t.Fatalf("next burst cues %+v", cues)
	}
	r.steamHiss(start+period, 20, 20) // another vent hisses on its own
	if len(r.takeCues()) != 1 {
		t.Fatal("each vent has its own burst record")
	}
}
