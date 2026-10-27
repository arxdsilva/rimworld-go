package weather

import "testing"

// TestSunFactorTable: each sky's share of the sun (permille) at full
// intensity, half its dimming at none, and clear always full.
func TestSunFactorTable(t *testing.T) {
	t.Parallel()
	full := map[Kind]int32{Clear: 1000, Cloudy: 700, Rain: 450, Storm: 350, Snow: 400, Blizzard: 250}
	for k, want := range full {
		if got := SunFactor(k, 1); got != want {
			t.Errorf("%s at full intensity: %d‰, want %d", k, got, want)
		}
		if got, want := SunFactor(k, 0), 1000-(1000-want)/2; got != want {
			t.Errorf("%s at no intensity: %d‰, want %d", k, got, want)
		}
	}
	if got := SunFactor(Clear, 0.7); got != 1000 {
		t.Errorf("clear at 0.7: %d‰", got)
	}
	if got := SunFactor(Kind(99), 1); got != 1000 {
		t.Errorf("an unknown sky dims the sun: %d‰", got)
	}
}

// TestSunFactorFallsWithIntensity: harder weather never lets more sun
// through, and intensity outside 0..1 is clamped.
func TestSunFactorFallsWithIntensity(t *testing.T) {
	t.Parallel()
	for k := Clear; k <= Blizzard; k++ {
		prev := int32(1001)
		for i := 0; i <= 20; i++ {
			got := SunFactor(k, float64(i)/20)
			if got > prev || got < 0 || got > 1000 {
				t.Fatalf("%s: %d‰ at %.2f after %d‰", k, got, float64(i)/20, prev)
			}
			prev = got
		}
		if SunFactor(k, -1) != SunFactor(k, 0) || SunFactor(k, 3) != SunFactor(k, 1) {
			t.Fatalf("%s: intensity not clamped", k)
		}
	}
}

func TestKindOfRoundTrips(t *testing.T) {
	t.Parallel()
	for k := Clear; k <= Blizzard; k++ {
		if got := KindOf(k.String()); got != k {
			t.Errorf("KindOf(%q) = %s", k.String(), got)
		}
	}
	if KindOf("fog") != Clear {
		t.Error("an unknown name is not clear")
	}
}
