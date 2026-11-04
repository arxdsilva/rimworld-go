package gameplay

import "testing"

func TestSettingsZeroIsDefaults(t *testing.T) {
	t.Parallel()
	var s Settings
	for _, k := range AllToggles() {
		if s.On(k) != ToggleDefault(k) {
			t.Errorf("toggle %d: zero value says %v", k, s.On(k))
		}
	}
	for l := Limit(0); l < NumLimits; l++ {
		if s.LimitOf(l) != LimitDefault(l) {
			t.Errorf("limit %d: zero value says %d", l, s.LimitOf(l))
		}
	}
}

func TestToggleDefaultsMatchDoc(t *testing.T) {
	t.Parallel()
	off := map[Toggle]bool{OddMeatNormal: true, FlavourIngredients: true, BenchCleanToggle: true,
		CleanBeforeMeal: true, ShelterWander: true, PoliteWander: true}
	for _, k := range AllToggles() {
		if ToggleDefault(k) == off[k] {
			t.Errorf("toggle %d default %v", k, ToggleDefault(k))
		}
	}
}

func TestWithRoundTripsToZero(t *testing.T) {
	t.Parallel()
	var s Settings
	for _, k := range AllToggles() {
		flip := s.With(k, !ToggleDefault(k))
		if flip.On(k) == ToggleDefault(k) || flip == (Settings{}) {
			t.Errorf("toggle %d did not flip", k)
		}
		if back := flip.With(k, ToggleDefault(k)); back != (Settings{}) {
			t.Errorf("toggle %d back to default leaves %+v", k, back)
		}
	}
	if got := s.With(NumToggles, true); got != s {
		t.Errorf("an unknown toggle changed settings: %+v", got)
	}
}

func TestLimitRoundTrip(t *testing.T) {
	t.Parallel()
	var s Settings
	for _, c := range []struct{ in, want int }{{0, 0}, {3, 3}, {50, 50}, {51, 50}, {-1, 0}} {
		if got := s.WithLimit(TidyStation, c.in).LimitOf(TidyStation); got != c.want {
			t.Errorf("limit %d reads %d, want %d", c.in, got, c.want)
		}
	}
	if s.WithLimit(TidyStation, LimitDefault(TidyStation)) != (Settings{}) {
		t.Error("a limit at its default must stay the zero value")
	}
	if s.WithLimit(NumLimits, 3) != s || s.LimitOf(NumLimits) != 0 {
		t.Error("an unknown limit must change nothing")
	}
}
