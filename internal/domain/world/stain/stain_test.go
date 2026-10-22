package stain

import "testing"

func TestFadeLinearSevenDays(t *testing.T) {
	var f int
	a := uint16(Full)
	prev := a
	for tick := 0; tick < FadeTicks; tick += 250 {
		a = Fade(a, 250, &f)
		if a > prev {
			t.Fatalf("grew at %d: %d -> %d", tick, prev, a)
		}
		prev = a
		if tick+250 == FadeTicks/2 && a != 500 {
			t.Fatalf("half way = %d, want 500", a)
		}
	}
	if a != 0 {
		t.Fatalf("after 7 days = %d, want 0", a)
	}
	if Fade(0, 250, &f) != 0 || Fade(10, 0, &f) != 10 || Fade(10, -5, &f) != 10 {
		t.Fatal("zero and non-positive steps")
	}
	// one big step equals many small ones
	var g int
	if b := Fade(Full, FadeTicks, &g); b != 0 || g != 0 {
		t.Fatalf("one step = %d frac %d", b, g)
	}
	// overshoot is clamped, never negative
	var h int
	if b := Fade(5, 10*FadeTicks, &h); b != 0 {
		t.Fatalf("overshoot = %d", b)
	}
}

func TestWashRainTwoHoursSnowThree(t *testing.T) {
	for _, c := range []struct {
		snow  bool
		ticks int
	}{{false, RainWash}, {true, SnowWash}} {
		var f int
		a := uint16(Full)
		for i := 0; i < c.ticks; i += 250 {
			a = Wash(a, 250, 1, c.snow, &f)
		}
		if a != 0 {
			t.Errorf("snow=%v: %d left after %d ticks", c.snow, a, c.ticks)
		}
		var h int
		if half := Wash(Full, c.ticks/2, 1, c.snow, &h); half != 500 {
			t.Errorf("snow=%v: half = %d, want 500", c.snow, half)
		}
	}
}

func TestWashScalesWithIntensity(t *testing.T) {
	var f int
	if a := Wash(Full, RainWash, 0.5, false, &f); a != 500 {
		t.Errorf("half intensity = %d, want 500", a)
	}
	f = 0
	if a := Wash(Full, RainWash, 5, false, &f); a != 0 {
		t.Errorf("intensity clamps to 1: %d", a)
	}
	f = 0
	if Wash(300, 250, 0, false, &f) != 300 || Wash(300, 0, 1, false, &f) != 300 || Wash(0, 250, 1, false, &f) != 0 {
		t.Error("no wash at zero intensity, zero ticks or zero stain")
	}
	// wash and fade share the remainder and still reach zero together
	f = 0
	a := uint16(Full)
	for i := 0; i < RainWash; i += 250 {
		a = Fade(a, 250, &f)
		a = Wash(a, 250, 1, false, &f)
	}
	if a != 0 {
		t.Errorf("rain plus fade = %d, want 0", a)
	}
}

func TestAddCaps(t *testing.T) {
	if Add(900, 200) != Full || Add(0, 120) != 120 || Add(10, -50) != 0 {
		t.Fatal("Add caps at Full and floors at 0")
	}
	if Blood.String() != "blood" || Ichor.String() != "ichor" {
		t.Fatal("kind names")
	}
}
