package exposure

import "testing"

// AIL-1: the held band (wiki: Ailments: heatstroke builds above the
// comfortable maximum + 10 °C and fades at or below the maximum;
// hypothermia mirrors it) is where a severity neither builds nor fades.
func TestHeldIsTheBandWhereNothingHeals(t *testing.T) {
	b, sev := Bands{Min: 50, Max: 292}, Full/100*80
	for _, c := range []struct {
		hypo, heat int32
		t          int
		held       bool
	}{
		{0, sev, 334, true},  // the player's save: 33.4 °C, comfortable to 29.2 °C
		{0, sev, 393, true},  // past the danger line it builds: not healing either
		{0, sev, 292, false}, // at the maximum it fades
		{0, Trivial - 1, 334, false},
		{sev, 0, 40, true},
		{sev, 0, 50, false},
		{0, 0, 600, false},
	} {
		if got := Held(c.hypo, c.heat, Trivial, c.t, b); got != c.held {
			t.Errorf("hypo %d heat %d in %d: held %v", c.hypo, c.heat, c.t, got)
		}
		if h, s := Step(c.hypo, c.heat, c.t, 60, b); c.held && c.t < b.HeatDanger() && c.t > b.ColdDanger() && (h != c.hypo || s != c.heat) {
			t.Errorf("held in %d but moved: %d %d", c.t, h, s)
		}
	}
	if Held(0, Minor-1, Minor, 334, b) || !Held(0, Minor, Minor, 334, b) {
		t.Error("the floor")
	}
	if got := (Bands{Min: 160, Max: 260}).Inset(10); got != (Bands{Min: 170, Max: 250}) {
		t.Errorf("inset %+v", got)
	}
}

func TestNoteSaysWhatAirHeals(t *testing.T) {
	b, sev := Bands{Min: 50, Max: 292}, Full/100*80
	if got := Note(0, sev, 334, b); got != "Not recovering: air 33 °C, needs 29 °C or cooler" {
		t.Errorf("hot: %q", got)
	}
	if got := Note(sev, 0, -55, b); got != "Not recovering: air -6 °C, needs 5 °C or warmer" {
		t.Errorf("cold: %q", got)
	}
	if Note(0, sev, 250, b) != "" {
		t.Error("healing: no note")
	}
}
