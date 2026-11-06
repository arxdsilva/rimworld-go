package units

import (
	"math"
	"testing"
)

// OPT-007: Kelvin reads like the other units and round-trips.
func TestKelvinFormats(t *testing.T) {
	k := FromUnit(UnitK)
	if !k.K || k.F || k.Name() != "Kelvin" || k.Symbol() != "K" {
		t.Fatalf("FromUnit(K) = %+v %q %q", k, k.Name(), k.Symbol())
	}
	for name, c := range map[string][2]string{
		"Tenths":  {k.Tenths(210), "294 K"},
		"Compact": {k.Compact(-30), "270K"},
		"Whole":   {k.Whole(0), "273K"},
		"Step":    {k.Step(), "1 K"},
		"Target":  {k.Target(210), "294.1 K"},
	} {
		if c[0] != c[1] && !(name == "Target" && c[0] == "294.2 K") { // x.15 rounds either way
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if v := k.Value(20); math.Abs(v-293.15) > 1e-9 {
		t.Errorf("Value(20) = %v", v)
	}
	if c := k.FromDisplay(k.Value(-12.5)); math.Abs(c+12.5) > 1e-9 {
		t.Errorf("round trip %v", c)
	}
	if d := k.Delta(2.5); d != 2.5 {
		t.Errorf("a difference keeps its size: %v", d)
	}
	if got := k.Text("Comfort +2.5 °C and -8 °C"); got != "Comfort +2.5 K and -8 K" {
		t.Errorf("Text = %q", got)
	}
}

// An unknown unit is Celsius; Kelvin and Fahrenheit do not mix.
func TestUnitSettingPicksOneFormatter(t *testing.T) {
	if FromUnit("") != Celsius || FromUnit("x") != Celsius {
		t.Fatal("an unknown unit is Celsius")
	}
	if f := FromUnit(UnitF); !f.F || f.K {
		t.Fatalf("F: %+v", f)
	}
}
