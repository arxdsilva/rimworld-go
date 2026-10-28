package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestTurbineBladesFollowWind: the blade frame advances with the wind on
// the presentation clock, faster in stronger wind, holds still in calm
// air or while the turbine is off, and never jumps when the wind changes.
func TestTurbineBladesFollowWind(t *testing.T) {
	t.Parallel()
	on := game.BuildingView{Kind: WindTurbineKind, PowerRole: "producer", PowerOn: true}
	if TurbineFPS(0, 0, &on) != 0 {
		t.Fatal("calm air turns the blades")
	}
	off := on
	off.PowerOn = false
	if TurbineFPS(2, 0, &off) != 0 {
		t.Fatal("a turbine switched off turns")
	}
	slow, fast := TurbineFPS(0.5, 0, &on), TurbineFPS(-1.5, 2, &on)
	if slow <= 0 || fast <= slow || TurbineFPS(50, 0, &on) > 60 {
		t.Fatalf("speeds: 0.5 → %v, 2.5 → %v, gale capped %v", slow, fast, TurbineFPS(50, 0, &on))
	}

	var p PowerDraw
	f0 := p.BladeFrame(5, 0, 100)
	for now := uint64(101); now < 400; now++ {
		if f := p.BladeFrame(5, 0, now); f != f0 {
			t.Fatalf("no wind: frame moved %d → %d", f0, f)
		}
	}
	frames := map[int]bool{}
	prev := p.BladeFrame(5, slow, 400)
	for now := uint64(401); now < 1400; now++ {
		fps := slow
		if now%37 < 18 { // gusts: the wind changes all the time
			fps = fast
		}
		f := p.BladeFrame(5, fps, now)
		if step := (f - prev + TurbineFrames) % TurbineFrames; step > 1 {
			t.Fatalf("frame jumped %d → %d at %d", prev, f, now)
		}
		frames[f] = true
		prev = f
	}
	if len(frames) != TurbineFrames {
		t.Fatalf("frames seen %v, want all %d", frames, TurbineFrames)
	}
	for _, f := range []int{p.BladeFrame(6, 0, 0), p.BladeFrame(7, 0, 0)} {
		if f < 1 || f > TurbineFrames {
			t.Fatalf("frame %d out of 1..%d", f, TurbineFrames)
		}
	}
	p.Prune(100000)
	if len(p.Spins) != 0 {
		t.Fatalf("stale turbines kept: %d", len(p.Spins))
	}
}
