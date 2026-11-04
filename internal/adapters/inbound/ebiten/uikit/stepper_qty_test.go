package uikit

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/platform/input"
)

// CTL-018: Ctrl steps a Stepper by 10, Shift by 100, both by 1000, and the
// value stops at its bounds.
func TestStepperStepsByHeldModifiers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mods input.Mod
		add  int
	}{{"plain", 0, 1}, {"ctrl", input.ModCtrl, 10}, {"shift", input.ModShift, 100}, {"both", input.ModCtrl | input.ModShift, 1000}}
	for _, c := range cases {
		k, h := newTestKit()
		v := 0
		ui := func() { Stepper(k, "q", gfx.Rect{X: 0, Y: 0, W: 112, H: 28}, &v, 0, 5000) }
		step(k, h, at(0, 0), ui)
		inc, _ := k.Find("q.inc")
		r := click(int(inc.R.X)+2, int(inc.R.Y)+2)
		r.Mods = c.mods
		step(k, h, r, ui)
		if v != c.add {
			t.Fatalf("%s: + gave %d, want %d", c.name, v, c.add)
		}
	}
}

func TestStepperLargeStepClampsAtBounds(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	v := 95
	ui := func() { Stepper(k, "q", gfx.Rect{X: 0, Y: 0, W: 112, H: 28}, &v, 0, 100) }
	step(k, h, at(0, 0), ui)
	inc, _ := k.Find("q.inc")
	r := click(int(inc.R.X)+2, int(inc.R.Y)+2)
	r.Mods = input.ModCtrl | input.ModShift
	step(k, h, r, ui)
	if v != 100 {
		t.Fatalf("+1000 from 95 stops at the max: %d", v)
	}
	dec, _ := k.Find("q.dec")
	r = click(int(dec.R.X)+2, int(dec.R.Y)+2)
	r.Mods = input.ModCtrl
	step(k, h, r, ui)
	if v != 90 {
		t.Fatalf("-10 from 100: %d", v)
	}
	r.Mods = input.ModCtrl | input.ModShift
	step(k, h, r, ui)
	if v != 0 {
		t.Fatalf("-1000 stops at the min: %d", v)
	}
}
