package health

import "testing"

func TestBodyDownedHysteresis(t *testing.T) {
	for _, c := range []struct {
		name               string
		cons, moving, pain int32
		downs, recovers    bool
	}{
		{"healthy", 1000, 1000, 0, false, true},
		{"just conscious", 300, 1000, 0, false, false}, // in the band: stays as it is
		{"unconscious", 299, 1000, 0, true, false},
		{"conscious again", 350, 1000, 0, false, true},
		{"just moving", 1000, 150, 0, false, false},
		{"cannot move", 1000, 149, 0, true, false},
		{"moving again", 1000, 200, 0, false, true},
		{"pain shock", 1000, 1000, 800, true, false},
		{"pain in the band", 1000, 1000, 760, false, false},
		{"pain eased", 1000, 1000, 750, false, true},
	} {
		if got := BodyDowns(c.cons, c.moving, c.pain, PainShock); got != c.downs {
			t.Errorf("%s: downs %v, want %v", c.name, got, c.downs)
		}
		if got := BodyRecovered(c.cons, c.moving, c.pain, PainShock); got != c.recovers {
			t.Errorf("%s: recovers %v, want %v", c.name, got, c.recovers)
		}
	}
	// a tougher pawn's shock threshold moves both edges
	if BodyDowns(1000, 1000, 850, 880) || !BodyRecovered(1000, 1000, 830, 880) || BodyRecovered(1000, 1000, 831, 880) {
		t.Error("shock threshold not honoured")
	}
}

func TestCauseStrings(t *testing.T) {
	if CauseBloodLoss.String() != "blood loss" || CauseBurns.String() != "burns" || CauseNone.String() != "" {
		t.Error("cause strings")
	}
	for c := CauseNoFood; c <= CauseBloodLoss; c++ {
		if c.String() == "" {
			t.Errorf("cause %d has no text", c)
		}
	}
	if (Record{Injury: 30}).HP() != 70 || (Record{Injury: 130}).HP() != 0 {
		t.Error("HP")
	}
	if Downed.String() != "downed" || Dead.String() != "dead" || Alive.String() != "alive" {
		t.Error("status strings")
	}
}
