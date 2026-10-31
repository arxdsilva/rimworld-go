package exposure

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
)

const ticksPerHour = 2500

// hoursUntil steps sev in air t from start with interval-tick steps until
// done, and reports the hours it took.
func hoursUntil(t, interval int, b Bands, hypo, heat int32, done func(h, x int32) bool) float64 {
	ticks := 0
	for !done(hypo, heat) && ticks < 100*ticksPerHour {
		hypo, heat = Step(hypo, heat, t, interval, b)
		ticks += interval
	}
	return float64(ticks) / ticksPerHour
}

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if got < want-tol || got > want+tol {
		t.Errorf("%s: %.2f h, want about %.1f h", what, got, want)
	}
}

// TestWikiTimings: about 32 h to die of cold 10 °C past the danger line,
// about 4.1 h at 100 °C below the range, about 24.8 h of heat at +25,
// and about 3.6 h to recover from near 100 %. wiki: Hypothermia,
// Heatstroke.
func TestWikiTimings(t *testing.T) {
	b := Everyday
	for _, interval := range []int{60, 250} {
		dead := func(h, x int32) bool { return h >= Full }
		near(t, "cold death 10 C past the danger line", hoursUntil(b.ColdDanger()-100, interval, b, 0, 0, dead), 32, 0.2)
		near(t, "cold death 100 C below the range", hoursUntil(b.Min-1000, interval, b, 0, 0, dead), 4.1, 0.15)
		hot := func(h, x int32) bool { return x >= Full }
		near(t, "heat death at +25", hoursUntil(b.Max+250, interval, b, 0, 0, hot), 24.8, 0.2)
		gone := func(h, x int32) bool { return h == 0 }
		near(t, "recovery from near 100 %", hoursUntil(200, interval, b, Full-1, 0, gone), 3.6, 0.1)
	}
}

func TestDangerLinesAndMargin(t *testing.T) {
	b := Everyday
	if b.Min != 160 || b.Max != 260 || b.ColdDanger() != 60 || b.HeatDanger() != 360 {
		t.Fatalf("bands %+v danger %d/%d", b, b.ColdDanger(), b.HeatDanger())
	}
	if HypoGain(60, b) != 0 || HypoGain(59, b) != hypoMin || HeatGain(360, b) != 0 || HeatGain(361, b) != heatMin {
		t.Error("exposure builds only past 10 C outside the range, at the floors first")
	}
	if HypoGain(b.Min-1000, b) != 900*tenthK || HeatGain(b.HeatDanger()+900, b) != 560*tenthK {
		t.Error("the wiki slope and heat curve")
	}
	// in the margin it holds; inside the range it recovers
	if h, x := Step(Minor, Minor, 100, 250, b); h != Minor || x >= Minor {
		t.Errorf("margin: %d %d", h, x)
	}
	if h, _ := Step(Minor, 0, 200, 60, b); h != Minor-Recovery(Minor) {
		t.Errorf("recovery %d", h)
	}
	if Recovery(Full) != recMax || Recovery(Full/100) != recMin || Recovery(Full/4) != Full/4/1000*27 {
		t.Error("recovery clamps")
	}
}

func TestParkaBandsSurviveWinter(t *testing.T) {
	parka := Everyday.Wearing(200+50, 0) // parka + tuque (wiki factors on cloth)
	if parka.Min != -90 || parka.ColdDanger() != -190 {
		t.Fatalf("bands = %+v", parka)
	}
	if h, _ := Step(0, 0, -150, 250, parka); h != 0 {
		t.Error("a -15 C night is fine in a parka and tuque")
	}
	if h, _ := Step(0, 0, -150, 250, Everyday); h == 0 {
		t.Error("but dangerous in everyday clothes")
	}
}

func TestThoughtLadders(t *testing.T) {
	b := Everyday
	cases := []struct {
		t    int
		want mood.Kind
		ok   bool
	}{
		{200, 0, false}, {160, 0, false}, {159, mood.Chilly, true}, {61, mood.Chilly, true}, {60, mood.Shivering, true},
		{-40, mood.NumbingCold, true}, {-140, mood.AbsolutelyFreezing, true}, {-900, mood.AbsolutelyFreezing, true},
		{261, mood.Sweaty, true}, {360, mood.Overheated, true}, {460, mood.Baking, true}, {560, mood.BlisteringHot, true},
	}
	for _, c := range cases {
		if k, ok := b.Thought(c.t); ok != c.ok || k != c.want {
			t.Errorf("%d: %v %v, want %v %v", c.t, k, ok, c.want, c.ok)
		}
	}
	if b.Freezing() != 60 || b.Sweltering() != 360 {
		t.Error("urgent dressing follows the second rungs")
	}
}

func TestFrostbiteChance(t *testing.T) {
	from := Full / 100 * 37
	if FrostbiteChance(from-1, from) != 0 || FrostbiteChance(from, from) != frostLo || FrostbiteChance(Full, from) != frostHi {
		t.Error("frostbite chance curve")
	}
	if Permille(Extreme) != 620 || FromPermille(600) != Full/1000*600 || FromPermille(2000) != Full {
		t.Error("permille conversions")
	}
}
