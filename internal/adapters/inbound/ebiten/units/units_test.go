package units

import "testing"

func TestFormats(t *testing.T) {
	f := FromUnit(UnitF)
	for _, c := range []struct {
		name, got, want string
	}{
		{"C tenths", Celsius.Tenths(210), "21 °C"},
		{"F tenths", f.Tenths(210), "70 °F"},
		{"C compact neg", Celsius.Compact(-25), "-3C"},
		{"F compact", f.Compact(0), "32F"},
		{"C target", Celsius.Target(215), "21.5 °C"},
		{"C target neg", Celsius.Target(-15), "-1.5 °C"},
		{"F target whole", f.Target(100), "50 °F"},
		{"F target frac", f.Target(215), "70.7 °F"},
		{"C whole", Celsius.Whole(21.4), "21°C"},
		{"F degrees", f.Degrees(-40, 0), "-40°F"},
		{"F step", f.Step(), "1.8 °F"},
		{"name", f.Name(), "Fahrenheit"},
	} {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.name, c.got, c.want)
		}
	}
	if FromUnit("").F || FromUnit("x").F {
		t.Error("only F selects Fahrenheit")
	}
	if got := f.FromDisplay(f.Value(12.5)); got < 12.49 || got > 12.51 {
		t.Errorf("round trip %v", got)
	}
}

// Domain sentences state differences: the offset is not added.
func TestTextConvertsDifferences(t *testing.T) {
	f := Temp{F: true}
	for in, want := range map[string]string{
		"-8 °C all year":                "-14.4 °F all year",
		"+6 °C, temperature swing -30%": "+10.8 °F, temperature swing -30%",
		"Comfort +2.5 °C and +1 °C":     "Comfort +4.5 °F and +1.8 °F",
		"no number °C":                  "no number °C",
		"no temperature":                "no temperature",
	} {
		if got := f.Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Celsius.Text("-8 °C"); got != "-8 °C" {
		t.Errorf("Celsius changed the sentence: %q", got)
	}
}
